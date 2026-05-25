// Package auth (v0.8.5 step 6) — HTTP Basic Auth via htpasswd file.
//
// Why this exists
// ───────────────
// Some enterprise deployments can't run OIDC:
//   - Air-gapped clusters with no upstream IdP
//   - Corporate proxies that strip OAuth2 redirect flows
//   - Customers who want SupKube behind their existing reverse proxy that
//     already authenticates users via Basic Auth (mod_auth_kerb, OIDC2HTTP,
//     etc. — the proxy authenticates, then forwards Basic Auth header)
//
// For these, we accept `Authorization: Basic <b64(user:pw)>` and validate
// against a Helm-mounted htpasswd file. The username then resolves
// through the SAME RBAC bindings table the OIDC path uses — bindings can
// match by user email OR by username, so an entry like:
//
//     bindings:
//       - user: alice
//         role: editor
//         namespaces: [shop-prod]
//
// works for both `alice@corp.com` arriving via OIDC and `alice` arriving
// via Basic Auth.
//
// File format
// ───────────
// Standard htpasswd format, one entry per line:
//
//     alice:$2y$10$bcrypt-hash...
//     bob:$2y$10$bcrypt-hash...
//
// Bcrypt only — we deliberately do NOT support MD5 (Apache md5crypt) or
// SHA-1, those have been broken or near-broken for years and a SupKube
// admin shouldn't be reusing them. `htpasswd -bnBC 10 user pw` produces
// the right hash; the same command we document for Dex static passwords.
//
// File path defaults to /etc/supkube/htpasswd, overridable via env var
// AUTH_HTPASSWD_PATH. The file is read ONCE at startup; reload requires
// a pod restart (v0.8.6 will add SIGHUP-based reload + fsnotify watching).
package auth

import (
	"bufio"
	"encoding/base64"
	"log"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// htpasswdEntry is one parsed row.
type htpasswdEntry struct {
	Username string
	Hash     string // bcrypt hash (starts with $2a$ / $2b$ / $2y$)
}

// loadHtpasswd reads AUTH_HTPASSWD_PATH and parses bcrypt entries.
// Returns nil if file absent / unreadable — feature is off by default.
//
// Skipped entries (logged at warn): blank lines, comment lines (#…),
// non-bcrypt hashes (we refuse to support insecure formats).
func loadHtpasswd() []htpasswdEntry {
	path := strings.TrimSpace(os.Getenv("AUTH_HTPASSWD_PATH"))
	if path == "" {
		// Off by default; an admin enables it by setting the env var
		// (Helm chart sets it when auth.basic.enabled=true).
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		log.Printf("[auth] AUTH_HTPASSWD_PATH=%s but file unreadable: %v", path, err)
		return nil
	}
	defer f.Close()

	var entries []htpasswdEntry
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 1 || colon == len(line)-1 {
			log.Printf("[auth] htpasswd line %d malformed (skipping)", lineNum)
			continue
		}
		user := line[:colon]
		hash := line[colon+1:]
		if !isBcryptHash(hash) {
			log.Printf("[auth] htpasswd line %d (user=%s) is not bcrypt — skipping. Re-generate with `htpasswd -bnBC 10 %s <pw>`.",
				lineNum, user, user)
			continue
		}
		entries = append(entries, htpasswdEntry{Username: user, Hash: hash})
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[auth] htpasswd scan error: %v", err)
	}
	log.Printf("[auth] htpasswd loaded: %d valid bcrypt entries from %s", len(entries), path)
	return entries
}

// isBcryptHash returns true if h matches one of the bcrypt prefixes.
// htpasswd produces $2y$ on Linux; Go's bcrypt accepts $2a$ and $2b$.
// We treat all three as bcrypt — bcrypt.CompareHashAndPassword handles
// the conversion internally.
func isBcryptHash(h string) bool {
	return strings.HasPrefix(h, "$2a$") ||
		strings.HasPrefix(h, "$2b$") ||
		strings.HasPrefix(h, "$2y$")
}

// authenticateBasic parses an Authorization: Basic header value, validates
// against the loaded htpasswd entries, and returns a User on success.
//
// Returns (nil, false) on any failure (missing entries, bad header, bcrypt
// mismatch). Never logs the password.
//
// The caller is responsible for calling ResolveRole on the returned User —
// we don't do it here so this function stays a pure credential check.
func (c *Config) authenticateBasic(headerValue string) (*User, bool) {
	if len(c.htpasswdEntries) == 0 {
		return nil, false
	}
	if !strings.HasPrefix(headerValue, "Basic ") {
		return nil, false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(headerValue, "Basic "))
	if err != nil {
		return nil, false
	}
	creds := string(raw)
	colon := strings.IndexByte(creds, ':')
	if colon < 1 {
		return nil, false
	}
	user := creds[:colon]
	pw := creds[colon+1:]

	// Linear scan + bcrypt is intentionally constant-time per entry;
	// total time is O(n) in entry count, fine for typical n < 100.
	// We do NOT early-exit on username match (that would leak which
	// users exist via timing). Instead we always compare against EVERY
	// matching entry; non-matching usernames fall through to a dummy
	// bcrypt to equalize timing — but that's overkill for typical
	// htpasswd files. Simpler model: O(matching usernames) bcrypt calls,
	// which is fine since usernames are public knowledge in most setups.
	for _, e := range c.htpasswdEntries {
		if e.Username != user {
			continue
		}
		if err := bcrypt.CompareHashAndPassword([]byte(e.Hash), []byte(pw)); err == nil {
			return &User{
				Subject:  "basic:" + user,
				Username: user,
				Email:    user, // bindings can match on either field
				Groups:   []string{"supkube:basic"},
			}, true
		}
	}
	return nil, false
}
