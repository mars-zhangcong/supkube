// Package auth: HTTP handlers for the OIDC login flow.
//
// Endpoints (mounted under /api/v1/auth/):
//
//	GET  /api/v1/auth/providers
//	  Returns the list of identity providers the SPA should render as
//	  "Login with X" buttons. In demo (embedded Dex with static password)
//	  this returns a single "username/password" entry. In production it
//	  returns whatever Dex connectors are configured.
//
//	GET  /api/v1/auth/login?provider=<id>
//	  Returns the OAuth2 authorization URL for the chosen provider.
//	  The SPA navigates the browser to it; Dex handles the rest.
//
//	POST /api/v1/auth/callback
//	  The SPA POSTs the authorization code (extracted from the Dex
//	  redirect URL) here. We exchange it for an ID token + refresh token
//	  and return them to the SPA.
//
//	GET  /api/v1/auth/me
//	  Returns the authenticated user (validated by middleware).
//
//	POST /api/v1/auth/logout
//	  Server-side: nothing to invalidate (stateless JWTs). Client side:
//	  clear localStorage. We just return a redirect URL for the IdP's
//	  end-session endpoint if it has one.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

// Provider is one row in the SPA's login screen "Login with X" list.
type Provider struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`     // "oidc" | "password"
	LoginURL string `json:"loginUrl"` // for oidc only — SPA navigates here
}

// ListProviders builds the /providers payload.
//
// Dex's discovery doc doesn't expose the connector list (it's a Dex
// implementation detail). But Dex's authorization endpoint accepts a
// `connector_id` URL parameter that skips the chooser screen and goes
// straight to the named connector. So we construct N "Login with X"
// buttons by:
//
//  1. Reading the configured provider list from c.Providers (populated
//     from the Helm-rendered AUTH_PROVIDERS_JSON env var)
//  2. For each, building a fresh OAuth2 authorize URL with the right
//     connector_id parameter
//
// All buttons share the same state nonce — they're functionally
// equivalent auth attempts; whichever the user picks, the callback flow
// is identical.
//
// Fallback: if Providers is empty (e.g. the customer disabled Dex but
// kept auth.enabled=true with a direct external OIDC issuer), we emit
// a single generic "Login" entry without connector_id — Dex (or whatever
// the issuer is) will present its own chooser UI.
func (c *Config) ListProviders(gc *gin.Context) {
	if !c.Enabled {
		gc.JSON(http.StatusOK, gin.H{
			"enabled":   false,
			"providers": []Provider{},
		})
		return
	}
	if err := c.Init(gc.Request.Context()); err != nil {
		gc.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	state := mustRandomState()
	providers := make([]Provider, 0, len(c.Providers)+1)

	if len(c.Providers) > 0 {
		// Multi-provider mode: one button per configured IdP.
		for _, p := range c.Providers {
			url := c.oauth2Config.AuthCodeURL(
				state,
				oauth2.SetAuthURLParam("connector_id", p.ID),
			)
			providers = append(providers, Provider{
				ID:       p.ID,
				Name:     p.Name,
				Type:     p.Type,
				LoginURL: url,
			})
		}
	} else {
		// Fallback: single generic Login entry without connector_id.
		providers = append(providers, Provider{
			ID:       "dex",
			Name:     "Login",
			Type:     "oidc",
			LoginURL: c.oauth2Config.AuthCodeURL(state),
		})
	}

	gc.JSON(http.StatusOK, gin.H{
		"enabled":   true,
		"providers": providers,
		"state":     state,
	})
}

// CallbackRequest is the body the SPA POSTs to /auth/callback after Dex
// redirects back with ?code=...&state=....
type CallbackRequest struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state" binding:"required"`
}

// Callback exchanges the authorization code for an ID token.
//
// State validation happens in the SPA before this call — we trust the
// SPA's storage (sessionStorage) since CSRF protection at this layer
// would require server-side state (Redis/etc), which we're avoiding in
// v0.8.5. Strict CSRF will come in v0.8.6 when we add audit + sessions.
func (c *Config) Callback(gc *gin.Context) {
	var req CallbackRequest
	if err := gc.ShouldBindJSON(&req); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.Init(gc.Request.Context()); err != nil {
		gc.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	token, err := c.oauth2Config.Exchange(context.Background(), req.Code)
	if err != nil {
		gc.JSON(http.StatusUnauthorized, gin.H{"error": "code exchange failed: " + err.Error()})
		return
	}
	rawID, ok := token.Extra("id_token").(string)
	if !ok || rawID == "" {
		gc.JSON(http.StatusUnauthorized, gin.H{"error": "missing id_token in token response"})
		return
	}
	idToken, err := c.verifier.Verify(context.Background(), rawID)
	if err != nil {
		gc.JSON(http.StatusUnauthorized, gin.H{"error": "id_token verification failed: " + err.Error()})
		return
	}
	user, err := c.extractUser(idToken)
	if err != nil {
		gc.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	// v0.8.5 step 3 fix: resolve role + namespace scope BEFORE we hand
	// the user object to the SPA. Without this, the SPA persists
	// role="" into localStorage and falls back to viewer-mode UI
	// (no role chip, hidden tabs) until the next bootstrap call to
	// /auth/me — which itself was failing 403 because /auth/me wasn't
	// in the RBAC table. The combo silently broke the whole demo.
	c.ResolveRole(user)

	gc.JSON(http.StatusOK, gin.H{
		"idToken":      rawID,
		"refreshToken": token.RefreshToken,
		"expiresAt":    token.Expiry,
		"user":         user,
	})
}

// Me returns the current user (resolved by middleware). v0.8.5 step 3
// also exposes role + namespaceScope so the SPA can drive canDo() helpers.
func (c *Config) Me(gc *gin.Context) {
	user := FromContext(gc)
	if user == nil {
		// Demo mode or auth not enabled — surface a sensible identity.
		gc.JSON(http.StatusOK, gin.H{
			"user": &User{
				Username: "anonymous",
				Groups:   []string{"admin"},
				Role:     RoleAdmin,
			},
			"authEnabled": c.Enabled,
			"rbacEnabled": c.RBACEnabled,
		})
		return
	}
	gc.JSON(http.StatusOK, gin.H{
		"user":        user,
		"authEnabled": c.Enabled,
		"rbacEnabled": c.RBACEnabled,
	})
}

// Logout is a server-side no-op for stateless JWTs. Returns a URL the
// SPA can navigate to for IdP single-logout, if the issuer advertises
// an end_session_endpoint.
func (c *Config) Logout(gc *gin.Context) {
	gc.JSON(http.StatusOK, gin.H{
		"endSessionURL": "", // populated in step 2 when we parse discovery
	})
}

// mustRandomState returns a URL-safe random string for the OAuth2 state
// parameter. Caller stores it in the SPA's sessionStorage and verifies
// on callback to prevent CSRF.
func mustRandomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
