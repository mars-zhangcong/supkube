// Tests for HMAC computation + verification. These are the integrity
// linchpin — if these regress, every downstream sign/verify is silently
// broken. Run with: go test ./internal/fingerprint/...
package fingerprint

import (
	"strings"
	"testing"
)

// Reference fingerprint used across multiple tests so the expected HMAC
// can be pre-computed once. Any field change here invalidates wantHMAC —
// keep them tied together.
func refFingerprint() *Fingerprint {
	return &Fingerprint{
		Version:           Version,
		SourceClusterID:   "0123456789abcdef0123456789abcdef",
		SourceClusterName: "test-control-plane",
		BackupName:        "daily-2026-06-01",
		BackupNamespace:   DefaultVeleroNS,
		CreatedAt:         "2026-06-01T12:00:00Z",
		TarballSHA256:     strings.Repeat("a", 64),
		MetadataSHA256:    strings.Repeat("b", 64),
		Algo:              Algo,
	}
}

const refSecret = "supkube-test-shared-secret-32by"

func TestHMAC_Deterministic(t *testing.T) {
	fp := refFingerprint()
	h1 := computeHMAC([]byte(refSecret), fp)
	h2 := computeHMAC([]byte(refSecret), fp)
	if h1 != h2 {
		t.Fatalf("HMAC not deterministic: %s vs %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Fatalf("HMAC hex should be 64 chars, got %d (%s)", len(h1), h1)
	}
}

func TestHMAC_DifferentSecret(t *testing.T) {
	fp := refFingerprint()
	h1 := computeHMAC([]byte(refSecret), fp)
	h2 := computeHMAC([]byte("different-secret-of-similar-len"), fp)
	if h1 == h2 {
		t.Fatalf("HMAC should differ with different secret: both %s", h1)
	}
}

func TestHMAC_TamperDetected(t *testing.T) {
	// Verifies: an attacker who edits ANY of the four HMAC-input fields
	// produces a non-matching HMAC, even if they leave the stored
	// HMACSHA256 unchanged.
	fp := refFingerprint()
	fp.HMACSHA256 = computeHMAC([]byte(refSecret), fp)

	if !verifyHMAC([]byte(refSecret), fp) {
		t.Fatalf("baseline verify should pass")
	}

	cases := []struct {
		name   string
		mutate func(*Fingerprint)
	}{
		{"backupName", func(f *Fingerprint) { f.BackupName = "different-backup" }},
		{"tarballSHA256", func(f *Fingerprint) { f.TarballSHA256 = strings.Repeat("c", 64) }},
		{"createdAt", func(f *Fingerprint) { f.CreatedAt = "2099-01-01T00:00:00Z" }},
		{"sourceClusterID", func(f *Fingerprint) { f.SourceClusterID = strings.Repeat("f", 32) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tampered := *fp
			tc.mutate(&tampered)
			if verifyHMAC([]byte(refSecret), &tampered) {
				t.Fatalf("tampered %s should fail HMAC verify but didn't", tc.name)
			}
		})
	}
}

func TestHMAC_WrongLength(t *testing.T) {
	fp := refFingerprint()
	fp.HMACSHA256 = "deadbeef" // 8 chars, not 64
	if verifyHMAC([]byte(refSecret), fp) {
		t.Fatalf("short HMAC should fail length check")
	}
}

func TestHMAC_FieldsOutsideInputDoNotAffect(t *testing.T) {
	// Verifies that metadataSHA256 / sourceClusterName / version are NOT
	// part of the HMAC input — useful for forward-compat (adding a new
	// non-input field doesn't break old signatures).
	fp := refFingerprint()
	h1 := computeHMAC([]byte(refSecret), fp)
	fp.MetadataSHA256 = strings.Repeat("z", 64)
	fp.SourceClusterName = "renamed-cluster"
	fp.Version = "v999"
	h2 := computeHMAC([]byte(refSecret), fp)
	if h1 != h2 {
		t.Fatalf("non-input fields should not affect HMAC: %s vs %s", h1, h2)
	}
}
