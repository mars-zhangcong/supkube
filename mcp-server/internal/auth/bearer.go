// Package auth enforces the agent→server Bearer token.
//
// Require wraps the /mcp handler so requests without a valid Bearer token get
// 401 BEFORE reaching the handler. Tests MUST exercise the wrapped handler
// (through this middleware) — the closed PR#66 spike passed its tests only
// because they called the handler directly, bypassing middleware, which masked
// a production-dead endpoint (403 in prod). TC-AUTH-PATH guards against that.
package auth

import "net/http"

func Require(validToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if validToken == "" || r.Header.Get("Authorization") != "Bearer "+validToken {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
