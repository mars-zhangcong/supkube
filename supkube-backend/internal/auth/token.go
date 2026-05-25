// Package auth (v0.8.5 step 6) — static API token authentication.
//
// Why this exists
// ───────────────
// OIDC + Dex is great for humans hitting the SPA, but bad for:
//   - CI/CD pipelines that want to call `kubectl-supkube backup create ...`
//   - Operators / SRE writing shell scripts against /api/v1/...
//   - Terraform / Pulumi providers that drive policy lifecycle declaratively
//
// Forcing those clients through an OAuth2 code-exchange dance is silly:
// they're machines, they don't have a browser. The K8s answer here is
// ServiceAccount tokens, but SupKube's RBAC is its own model (not K8s
// RBAC), so we need a way to map "this token → this SupKube role + ns
// scope" that's independent of the OIDC flow.
//
// Design
// ──────
// Tokens are OPAQUE random strings (NOT JWTs). The admin generates one
// per pipeline / operator, hashes it with SHA-256, and lists the hashes
// in Helm values:
//
//   auth:
//     staticTokens:
//       - name: github-actions-prod
//         hash: <sha256-hex>            # echo -n "<plaintext>" | sha256sum
//         role: editor
//         namespaces: [shop-prod]
//       - name: terraform
//         hash: <sha256-hex>
//         role: admin
//
// The plaintext lives ONLY on the calling pipeline's side (as a CI
// secret, vault entry, etc.). The backend only ever sees the hash. A
// leaked values.yaml / `helm get values` can't impersonate the pipeline.
//
// HTTP usage
// ──────────
// Same `Authorization: Bearer <plaintext>` header as OIDC — the dispatch
// happens in Middleware() based on the token shape (JWT has dots, opaque
// tokens don't). This keeps SDK code identical regardless of auth flavor.
//
// Security notes
// ──────────────
// 1. Constant-time compare on the hash to avoid timing attacks.
// 2. Tokens never appear in audit logs — we record name + role only.
// 3. Hash, not bcrypt: bcrypt is slow on purpose for password use, but
//    these tokens are 256-bit random — there's no dictionary attack to
//    defend against, so SHA-256 is fine and adds zero latency per call.
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"strings"
)

// StaticToken is one row in the configured token table. Loaded from
// AUTH_STATIC_TOKENS_JSON env var (rendered by Helm from auth.staticTokens).
//
// Hash is the lowercase hex-encoded SHA-256 of the plaintext token. We
// compare against it via subtle.ConstantTimeCompare in lookupStaticToken.
type StaticToken struct {
	Name       string   `json:"name"`                 // identifier surfaced in audit logs
	Hash       string   `json:"hash"`                 // sha256(plaintext) hex
	Role       string   `json:"role"`                 // admin | editor | viewer
	Namespaces []string `json:"namespaces,omitempty"` // editor scope; ignored for admin/viewer
}

// loadStaticTokens parses AUTH_STATIC_TOKENS_JSON. Empty / missing env →
// nil slice (feature off). Invalid JSON logs a warning and returns nil —
// we fail OPEN here because the OIDC path may still work, and a misformed
// Helm value shouldn't lock everyone out.
func loadStaticTokens() []StaticToken {
	raw := strings.TrimSpace(os.Getenv("AUTH_STATIC_TOKENS_JSON"))
	if raw == "" {
		return nil
	}
	var tokens []StaticToken
	if err := json.Unmarshal([]byte(raw), &tokens); err != nil {
		log.Printf("[auth] AUTH_STATIC_TOKENS_JSON parse failed: %v", err)
		return nil
	}
	// Normalize hashes to lowercase so callers can paste either case.
	for i := range tokens {
		tokens[i].Hash = strings.ToLower(strings.TrimSpace(tokens[i].Hash))
	}
	return tokens
}

// lookupStaticToken finds the StaticToken whose hash matches sha256(raw).
// Returns (token, true) on match, (zero, false) otherwise. Uses a constant-
// time compare so an attacker can't binary-search hashes via timing.
//
// Caller is expected to have already loaded c.staticTokens at startup —
// this is a hot path (every request when token auth is in use).
func (c *Config) lookupStaticToken(raw string) (StaticToken, bool) {
	if raw == "" || len(c.staticTokens) == 0 {
		return StaticToken{}, false
	}
	sum := sha256.Sum256([]byte(raw))
	hexHash := hex.EncodeToString(sum[:])
	// Iterate full list rather than early-exit on first mismatch; the
	// timing of "found at index N" should not leak via response time.
	var match StaticToken
	found := 0
	for _, t := range c.staticTokens {
		if subtle.ConstantTimeCompare([]byte(t.Hash), []byte(hexHash)) == 1 {
			match = t
			found = 1
			// Don't break — keep loop length constant.
		}
	}
	return match, found == 1
}

// userFromToken turns a matched StaticToken into the same User shape the
// OIDC path produces. The "subject" is synthesized as "token:<name>" so
// the audit log distinguishes machine identities from human emails.
func userFromToken(t StaticToken) *User {
	u := &User{
		Subject:  "token:" + t.Name,
		Username: t.Name,
		Email:    t.Name + "@token.supkube.local",
		Groups:   []string{"supkube:tokens"},
		Role:     t.Role,
	}
	if t.Role == RoleEditor {
		u.NamespaceScope = append([]string(nil), t.Namespaces...)
	}
	return u
}

// looksLikeJWT returns true if the token has the canonical 3-segment
// header.payload.signature shape. We use this to decide whether to dispatch
// to the OIDC verifier (slow, JWKS-backed) vs. the static-token table
// (fast, hash lookup). A token that LOOKS like a JWT but fails verification
// is still rejected — we never fall back from JWT to token after a
// signature mismatch (that would let attackers craft tokens that bypass
// signature checks by faking a hash collision).
func looksLikeJWT(raw string) bool {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return false
	}
	// Each segment must be non-empty and look base64url-ish. We don't
	// fully decode — that's the verifier's job. This is just a cheap
	// dispatch filter.
	for _, p := range parts {
		if p == "" {
			return false
		}
	}
	return true
}
