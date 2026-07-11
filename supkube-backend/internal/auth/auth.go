// Package auth (v0.8.5) — OIDC authentication for SupKube.
//
// Architecture
// ────────────
// SupKube's backend speaks ONE auth protocol: OIDC. Whether the user is
// coming from Keycloak, Okta, Azure AD, GitHub, or LDAP, the token reaching
// our middleware is always a Dex-signed (or external-IdP-signed) JWT. We
// validate signature + issuer + audience + expiry, extract username +
// groups from claims, and inject the result into Gin context for handlers.
//
// Why we don't authenticate users ourselves (no /login form here):
//   - Login UX is the SPA's job; the backend only validates tokens.
//   - OIDC redirect flow happens entirely in the browser (Dex handles
//     redirects, code exchange, token issuance).
//   - The SPA hits /api/v1/auth/callback ONLY after Dex redirects back —
//     and that handler is a thin shim that swaps code for token using
//     our OIDC client credentials and returns it to the SPA.
//
// Config (from environment, set by Helm in backend-deployment.yaml):
//
//	AUTH_ENABLED=true|false        master switch — false skips ALL checks
//	OIDC_ISSUER_URL=<url>           e.g. http://localhost:30888/dex
//	OIDC_CLIENT_ID=supkube
//	OIDC_CLIENT_SECRET=<secret>
//	OIDC_USERNAME_CLAIM=email
//	OIDC_GROUPS_CLAIM=groups
//	AUTH_PUBLIC_URL=<url>           e.g. http://localhost:30888
//	AUTH_DEX_ENABLED=true|false     whether embedded Dex is on (affects /providers)
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

// User is what handlers see after middleware passes. Stored in the Gin
// context under the key "user" — retrieve via FromContext(c).
//
// v0.8.5 step 3: Role + NamespaceScope are resolved from the OIDC groups
// claim via the Helm-configured RoleBinding list. See ResolveRole().
type User struct {
	Subject  string   `json:"sub"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Groups   []string `json:"groups"`
	// v0.8.5 step 3 RBAC fields:
	Role           string   `json:"role"`           // "admin" | "editor" | "viewer" | "" (no binding → reject)
	NamespaceScope []string `json:"namespaceScope"` // only for editor; empty for admin (cluster-wide) and viewer (read everything)
}

// Role constants. Order matters semantically: admin > editor > viewer.
const (
	RoleAdmin  = "admin"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

// RoleBinding maps an OIDC groups claim value (or a specific user email)
// to a SupKube role. Populated from Helm via RBAC_BINDINGS_JSON.
//
// Either Group OR User must be set (not both). If both are set, Group wins.
// Namespaces is meaningful only when Role == editor — admin gets cluster
// scope, viewer gets cluster-read scope.
type RoleBinding struct {
	Group      string   `json:"group,omitempty"`
	User       string   `json:"user,omitempty"`
	Role       string   `json:"role"`
	Namespaces []string `json:"namespaces,omitempty"`
}

// CanAccessNamespace returns true if the user's role + scope allows
// operating on the given namespace.
//
// Semantics:
//
//	admin  → always (cluster-wide)
//	viewer → always (read-only ops; the role-check middleware blocks writes)
//	editor → only if ns ∈ NamespaceScope
//
// Empty NamespaceScope for editor = NO ns allowed (defensive default;
// the bindings config must explicitly grant namespaces for editor to be useful).
func (u *User) CanAccessNamespace(ns string) bool {
	if u == nil {
		return false
	}
	switch u.Role {
	case RoleAdmin, RoleViewer:
		return true
	case RoleEditor:
		for _, allowed := range u.NamespaceScope {
			if allowed == ns {
				return true
			}
		}
	}
	return false
}

// IsAtLeast checks if the user's role meets the given minimum role.
// Hierarchy: admin (3) > editor (2) > viewer (1) > none (0).
func (u *User) IsAtLeast(minRole string) bool {
	if u == nil {
		return false
	}
	return roleRank(u.Role) >= roleRank(minRole)
}

func roleRank(r string) int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleEditor:
		return 2
	case RoleViewer:
		return 1
	}
	return 0
}

// ProviderMeta is the per-IdP metadata the SPA's login screen needs.
// Populated from AUTH_PROVIDERS_JSON env var (rendered by Helm from the
// auth.dex.connectors + auth.dex.staticPasswords sections of values.yaml).
//
// Why not introspect Dex's running config: parsing Dex's YAML inside the
// backend creates a tight coupling and an RBAC dependency on the Dex
// ConfigMap. Helm renders both Dex's config AND this metadata from the
// SAME source (values.yaml), so they can't drift.
type ProviderMeta struct {
	ID   string `json:"id"`   // dex connector id, e.g. "keycloak" or "local" (password DB)
	Name string `json:"name"` // human label, e.g. "Login with Keycloak"
	Type string `json:"type"` // "oidc" | "password" | "github" | "ldap" | ...
}

// Config holds everything the verifier and the callback handler need.
// Populated from env vars at startup; never mutated after.
//
// The two-URL problem (and why we have IssuerURL AND DiscoveryURL):
//
// Dex's issuer = whatever URL the BROWSER will see (e.g. http://localhost:30888/dex).
// That URL gets baked into every JWT's `iss` claim. But the backend pod
// can't reach localhost:30888 — that's the host's port, not anything
// inside the cluster.
//
// Solution: tell go-oidc "fetch discovery from this in-cluster URL, but
// trust JWTs with iss=<public URL>". That's exactly what
// InsecureIssuerURLContext is for. We feed:
//
//	IssuerURL    = http://localhost:30888/dex   (what's in JWT iss claim)
//	DiscoveryURL = http://supkube-dex:5556/dex  (cluster-internal, reachable)
//
// At validation time, the verifier checks the JWT iss claim equals
// IssuerURL — which it does, because that's what Dex stamps.
type Config struct {
	Enabled       bool
	IssuerURL     string // browser-facing — appears as `iss` claim in JWTs
	DiscoveryURL  string // backend-reachable — used to fetch /.well-known/...
	ClientID      string
	ClientSecret  string
	UsernameClaim string
	GroupsClaim   string
	PublicURL     string // e.g. http://localhost:30888 (for redirect URI)
	DexEnabled    bool

	// v0.8.5 step 2: configured IdP list. Empty → backend falls back to
	// a single generic "Login" button (Dex chooser screen handles routing).
	Providers []ProviderMeta

	// v0.8.5 step 3: RBAC bindings (group/user → role + ns scope).
	// Empty list → RBAC disabled; everyone authenticated is admin.
	// This "RBAC-off-by-default" is for backwards compat with v0.8.5
	// step 1/2 deployments that already shipped without role bindings.
	RBACEnabled  bool
	RoleBindings []RoleBinding
	// DefaultRole applies when a user matches NO binding. For new
	// installations we recommend "viewer" (deny-by-default for writes);
	// to maintain v0.8.5-step-2 backward-compat we default to "admin".
	DefaultRole string

	// Set after Init() runs successfully.
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	initialized  bool
	initErr      error
	initOnce     sync.Once

	// v0.8.5 step 6: fallback auth methods. Loaded eagerly at
	// LoadConfigFromEnv (not lazy in Init) because they don't depend
	// on Dex being ready and we want any parse warnings to surface
	// at boot rather than first-request.
	staticTokens    []StaticToken   // see token.go
	htpasswdEntries []htpasswdEntry // see basic.go
}

// LoadConfigFromEnv reads auth config from environment variables. Returns
// a Config that may not be Initialized — call Init() before use.
func LoadConfigFromEnv() *Config {
	cfg := &Config{
		Enabled:   envBool("AUTH_ENABLED", false),
		IssuerURL: os.Getenv("OIDC_ISSUER_URL"),
		// DiscoveryURL falls back to IssuerURL when not provided —
		// works for the common "browser URL == backend URL" case
		// (single public Ingress reachable from both sides).
		DiscoveryURL:  envOr("OIDC_DISCOVERY_URL", os.Getenv("OIDC_ISSUER_URL")),
		ClientID:      envOr("OIDC_CLIENT_ID", "supkube"),
		ClientSecret:  os.Getenv("OIDC_CLIENT_SECRET"),
		UsernameClaim: envOr("OIDC_USERNAME_CLAIM", "email"),
		GroupsClaim:   envOr("OIDC_GROUPS_CLAIM", "groups"),
		PublicURL:     strings.TrimRight(os.Getenv("AUTH_PUBLIC_URL"), "/"),
		DexEnabled:    envBool("AUTH_DEX_ENABLED", false),
	}
	// v0.8.5 step 2: provider list comes from Helm. Empty/invalid JSON →
	// no providers, the SPA falls back to a single generic "Login" button
	// that hits Dex's chooser screen.
	if raw := os.Getenv("AUTH_PROVIDERS_JSON"); raw != "" {
		var p []ProviderMeta
		if err := json.Unmarshal([]byte(raw), &p); err == nil {
			cfg.Providers = p
		} else {
			log.Printf("[auth] AUTH_PROVIDERS_JSON parse failed (using fallback): %v", err)
		}
	}
	// v0.8.5 step 3: RBAC bindings.
	cfg.RBACEnabled = envBool("RBAC_ENABLED", false)
	cfg.DefaultRole = envOr("RBAC_DEFAULT_ROLE", RoleAdmin)
	if raw := os.Getenv("RBAC_BINDINGS_JSON"); raw != "" {
		var b []RoleBinding
		if err := json.Unmarshal([]byte(raw), &b); err == nil {
			cfg.RoleBindings = b
		} else {
			log.Printf("[auth] RBAC_BINDINGS_JSON parse failed: %v", err)
		}
	}
	// v0.8.5 step 6: fallback auth methods. Both are passive (no
	// initialization beyond parsing config), so we load them here
	// rather than in Init().
	cfg.staticTokens = loadStaticTokens()
	cfg.htpasswdEntries = loadHtpasswd()
	if len(cfg.staticTokens) > 0 {
		log.Printf("[auth] static API tokens loaded: %d entries", len(cfg.staticTokens))
	}
	return cfg
}

// ResolveRole picks the highest role the user qualifies for, given the
// configured bindings.
//
// Resolution order (first wins, but highest role accumulates):
//  1. Group bindings — for every binding.Group that's in user.Groups,
//     we collect its role and (if editor) ns scope
//  2. User bindings — for binding.User == user.Email, same accumulation
//  3. If multiple roles match, keep the highest (admin > editor > viewer)
//  4. If editor matches multiple times, union all namespaces
//  5. If no binding matches AND RBAC enabled, return c.DefaultRole (default admin)
//
// RBAC disabled → everyone is admin regardless of groups (backward compat
// with v0.8.5-step-2 deployments).
func (c *Config) ResolveRole(u *User) {
	if u == nil {
		return
	}
	if !c.RBACEnabled {
		u.Role = RoleAdmin
		return
	}
	bestRole := ""
	var nsUnion []string
	matched := false

	apply := func(b RoleBinding) {
		matched = true
		if roleRank(b.Role) > roleRank(bestRole) {
			bestRole = b.Role
		}
		// Accumulate ns scope; only meaningful if user ends up as editor.
		nsUnion = append(nsUnion, b.Namespaces...)
	}

	groupSet := map[string]struct{}{}
	for _, g := range u.Groups {
		groupSet[g] = struct{}{}
	}
	for _, b := range c.RoleBindings {
		if b.Group != "" {
			if _, ok := groupSet[b.Group]; ok {
				apply(b)
			}
		} else if b.User != "" {
			if b.User == u.Email || b.User == u.Username {
				apply(b)
			}
		}
	}

	if !matched {
		bestRole = c.DefaultRole
	}
	u.Role = bestRole
	if bestRole == RoleEditor {
		u.NamespaceScope = dedupe(nsUnion)
	} else {
		u.NamespaceScope = nil // admin/viewer don't use ns scope
	}
}

func dedupe(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// Init wires up the OIDC verifier by fetching the issuer's discovery doc.
// Called lazily (and at most once) from the middleware so that backend
// startup doesn't fail if Dex isn't ready yet — the first request may
// take longer but subsequent ones are fast.
//
// Lazy + retrying matters in K8s because pods boot in arbitrary order:
// backend may reach steady-state before Dex has registered its DNS name.
func (c *Config) Init(ctx context.Context) error {
	c.initOnce.Do(func() {
		if !c.Enabled {
			c.initialized = true
			return
		}
		if c.IssuerURL == "" {
			c.initErr = errors.New("AUTH_ENABLED=true but OIDC_ISSUER_URL is empty")
			return
		}
		// go-oidc fetches /.well-known/openid-configuration from the
		// DiscoveryURL (which may differ from IssuerURL — see the
		// two-URL problem comment on the Config struct). We tell
		// go-oidc via InsecureIssuerURLContext to trust JWTs with
		// iss=IssuerURL even though we fetched discovery from
		// DiscoveryURL. The name says "Insecure" but in our context
		// it's just acknowledging the URL split is intentional, not
		// "skip all issuer validation".
		discoveryCtx := ctx
		if c.DiscoveryURL != "" && c.DiscoveryURL != c.IssuerURL {
			discoveryCtx = oidc.InsecureIssuerURLContext(ctx, c.IssuerURL)
		}
		discoverURL := c.DiscoveryURL
		if discoverURL == "" {
			discoverURL = c.IssuerURL
		}
		// Retry a few times because Dex may still be starting.
		var provider *oidc.Provider
		var err error
		for attempt := 0; attempt < 10; attempt++ {
			provider, err = oidc.NewProvider(discoveryCtx, discoverURL)
			if err == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if err != nil {
			c.initErr = fmt.Errorf("OIDC discovery failed for %s: %w", discoverURL, err)
			return
		}
		c.provider = provider

		// Three URL classes from discovery, three different destinations:
		//
		//   AuthURL        → BROWSER follows this. Keep as-is (public URL).
		//   TokenURL       → BACKEND calls this server-to-server. Rewrite to cluster-internal.
		//   JWKSURL        → BACKEND fetches public keys to verify JWT signatures. Same — rewrite.
		//
		// provider.Verifier(...) implicitly uses the JWKS URL from
		// discovery (i.e. localhost:30888/dex/keys — unreachable from pod).
		// We swap that out for a custom KeySet pointed at the in-cluster
		// service. Without this, JWT verification fails on every API call
		// AFTER login succeeds, dumping the user back to /login?reason=expired.
		ep := provider.Endpoint()
		if c.DiscoveryURL != "" && c.DiscoveryURL != c.IssuerURL {
			ep.TokenURL = c.DiscoveryURL + "/token"
			// Build a JWKS that fetches from the cluster-internal URL.
			internalJWKS := c.DiscoveryURL + "/keys"
			keySet := oidc.NewRemoteKeySet(ctx, internalJWKS)
			c.verifier = oidc.NewVerifier(c.IssuerURL, keySet, &oidc.Config{ClientID: c.ClientID})
		} else {
			c.verifier = provider.Verifier(&oidc.Config{ClientID: c.ClientID})
		}
		c.oauth2Config = &oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			Endpoint:     ep,
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email", "groups", "offline_access"},
			RedirectURL:  c.PublicURL + "/auth/callback",
		}
		c.initialized = true
	})
	return c.initErr
}

// Middleware returns a Gin handler that validates the Authorization header
// and injects the resolved User into the context. When AUTH_ENABLED=false
// it short-circuits with a synthetic anonymous admin so existing handlers
// keep working in dev/demo mode.
func (c *Config) Middleware() gin.HandlerFunc {
	return func(gc *gin.Context) {
		// Always allow the auth endpoints themselves through — they're how
		// the user OBTAINS a token in the first place.
		if isAuthBypassPath(gc.Request.URL.Path) {
			gc.Next()
			return
		}
		// Demo mode — no real auth, synthesize an admin.
		if !c.Enabled {
			anon := &User{
				Subject:  "anonymous",
				Username: "anonymous",
				Email:    "anonymous@supkube.local",
				Groups:   []string{"admin"},
				Role:     RoleAdmin,
			}
			gc.Set("user", anon)
			gc.Next()
			return
		}
		// v0.8.5 step 6: three auth flavors share this entry point.
		//
		//   1. Authorization: Bearer <opaque>  → static API token lookup
		//   2. Authorization: Bearer <JWT>     → OIDC verify
		//   3. Authorization: Basic <b64>      → htpasswd lookup
		//
		// Dispatch order matters for performance: static tokens are a
		// SHA-256 + map lookup (cheap), JWT validation needs JWKS
		// (cached but heavier), Basic Auth needs bcrypt (slowest, one
		// per attempt). We try cheap-to-expensive.
		h := gc.GetHeader("Authorization")

		var user *User
		switch {
		case strings.HasPrefix(h, "Bearer "):
			raw := strings.TrimPrefix(h, "Bearer ")
			// Step 6a: try static token first if it doesn't LOOK like
			// a JWT. We don't try tokens AFTER a failed JWT verify —
			// that would let an attacker probe the token table by
			// sending crafted JWT-shaped strings.
			if !looksLikeJWT(raw) {
				if t, ok := c.lookupStaticToken(raw); ok {
					user = userFromToken(t)
					// Tokens have role pre-baked; skip RBAC resolution
					// to keep them immutable from the bindings table.
					// (Admins manage token permissions via Helm only.)
					break
				}
				gc.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API token"})
				return
			}
			// Step 6b: JWT — full OIDC verify path. Requires Init().
			if err := c.Init(gc.Request.Context()); err != nil {
				gc.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "auth not ready: " + err.Error()})
				return
			}
			idToken, err := c.verifier.Verify(gc.Request.Context(), raw)
			if err != nil {
				gc.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token: " + err.Error()})
				return
			}
			u, err := c.extractUser(idToken)
			if err != nil {
				gc.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "cannot extract user from token: " + err.Error()})
				return
			}
			c.ResolveRole(u)
			user = u

		case strings.HasPrefix(h, "Basic "):
			// Step 6c: Basic Auth via htpasswd. Disabled by default;
			// only kicks in if AUTH_HTPASSWD_PATH was set and the file
			// loaded at boot.
			u, ok := c.authenticateBasic(h)
			if !ok {
				gc.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid basic credentials"})
				return
			}
			// Basic Auth users go through the same RBAC bindings table
			// (matched by username) so admins can revoke / re-scope
			// without editing the htpasswd file.
			c.ResolveRole(u)
			user = u

		default:
			gc.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed Authorization header"})
			return
		}

		gc.Set("user", user)
		gc.Next()
	}
}

// isAuthBypassPath returns true for endpoints that must work without a
// token — the auth flow itself, and the unauthenticated status probe.
func isAuthBypassPath(p string) bool {
	switch p {
	case "/api/v1/status",
		"/api/v1/auth/providers",
		"/api/v1/auth/callback",
		"/api/v1/auth/logout",
		// DR posture export — read-only canonical backup/DR snapshot consumed by
		// SupInsight's ingest (service-to-service, in-cluster DNS). Exempt by design;
		// the public Ingress is separately gated by nginx. See dr_posture.go.
		"/api/v1/dr-posture":
		return true
	}
	return false
}

// extractUser pulls Username + Groups out of the validated JWT claims.
// Different IdPs put these in different claim names — we let the Helm
// values configure which claim to read (defaults match Dex).
func (c *Config) extractUser(idToken *oidc.IDToken) (*User, error) {
	// Stage 1: pull all claims into a flat map so we can read arbitrary
	// claim names (UsernameClaim / GroupsClaim are user-configured).
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, err
	}
	u := &User{Subject: idToken.Subject}
	if v, ok := claims[c.UsernameClaim].(string); ok {
		u.Username = v
	}
	if v, ok := claims["email"].(string); ok {
		u.Email = v
	}
	// Groups is an array of strings in nearly every IdP.
	if g, ok := claims[c.GroupsClaim].([]interface{}); ok {
		for _, x := range g {
			if s, ok := x.(string); ok {
				u.Groups = append(u.Groups, s)
			}
		}
	}
	// Fallback: if the configured Username claim was empty, use email or sub.
	if u.Username == "" {
		if u.Email != "" {
			u.Username = u.Email
		} else {
			u.Username = u.Subject
		}
	}
	return u, nil
}

// OAuth2Config exposes the configured OAuth2 client to callback/login
// handlers. Returns nil if Init hasn't run successfully.
func (c *Config) OAuth2Config() *oauth2.Config { return c.oauth2Config }

// Verifier exposes the OIDC verifier (for handlers that need to validate
// tokens outside the middleware path, e.g. silent refresh).
func (c *Config) Verifier() *oidc.IDTokenVerifier { return c.verifier }

// FromContext fetches the authenticated User from a Gin context. Panics
// if the middleware never ran (programmer error).
func FromContext(gc *gin.Context) *User {
	v, ok := gc.Get("user")
	if !ok {
		return nil
	}
	u, _ := v.(*User)
	return u
}

// envOr returns the env var or a default.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envBool parses "true"/"false" (case-insensitive). Missing/invalid => def.
func envBool(key string, def bool) bool {
	v := strings.ToLower(os.Getenv(key))
	switch v {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	}
	return def
}
