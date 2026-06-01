// Package v1: fingerprint dependency injection point.
//
// cmd/server/server.go constructs the Validator + TrustStore at boot and
// hands them off via SetFingerprintDeps. The ImportPolicy controller +
// future admin/audit handlers in this package can then read them via the
// FingerprintValidator() / FingerprintTrustStore() getters.
//
// Why package-level state (vs. plumbing through every handler signature)
// ─────────────────────────────────────────────────────────────────────
// Gin handlers are registered as func(*gin.Context) — there's no clean
// way to inject extra deps without wrapping each call site. This package
// already uses the same pattern for the SeedBuiltinTransformSets path,
// so it's consistent. The setters are write-once at boot; the getters
// are read-only at request time — race-free without locks.
package v1

import (
	"github.com/supkube/supkube-backend/internal/fingerprint"
)

var (
	fpValidator  fingerprint.Validator
	fpTrustStore *fingerprint.TrustStore
)

// SetFingerprintDeps is called from cmd/server/server.go at boot. Safe to
// call with nil values when the fingerprint pipeline failed to initialize —
// downstream code must nil-check.
func SetFingerprintDeps(v fingerprint.Validator, ts *fingerprint.TrustStore) {
	fpValidator = v
	fpTrustStore = ts
}

// FingerprintValidator returns the active validator, or nil if not wired
// (e.g. K8s client unavailable at boot). Callers MUST nil-check.
func FingerprintValidator() fingerprint.Validator { return fpValidator }

// FingerprintTrustStore returns the active TrustStore, or nil. Callers
// MUST nil-check.
func FingerprintTrustStore() *fingerprint.TrustStore { return fpTrustStore }
