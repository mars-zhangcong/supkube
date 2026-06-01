// Tests for Validator — the security-critical destination-side surface.
// Covers all three modes × {valid, missing, tampered, hmac-invalid}.
package fingerprint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// fakeBSL is an in-memory BSLObjectClient. Map key = "<bslName>/<key>".
type fakeBSL struct {
	mu      sync.Mutex
	objects map[string][]byte
	prefix  string // optional fixed prefix for all BSLs
	failGet bool
}

func newFakeBSL() *fakeBSL {
	return &fakeBSL{objects: map[string][]byte{}}
}

func (f *fakeBSL) k(bslName, key string) string { return bslName + "/" + key }

func (f *fakeBSL) GetObject(ctx context.Context, bslName, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failGet {
		return nil, fmt.Errorf("fakeBSL: simulated read failure")
	}
	v, ok := f.objects[f.k(bslName, key)]
	if !ok {
		return nil, fmt.Errorf("fakeBSL: not found")
	}
	return append([]byte(nil), v...), nil
}

func (f *fakeBSL) PutObject(ctx context.Context, bslName, key string, body []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[f.k(bslName, key)] = append([]byte(nil), body...)
	return nil
}

func (f *fakeBSL) Prefix(ctx context.Context, bslName string) (string, error) {
	return f.prefix, nil
}

// ListPrefix satisfies the BSLObjectClient interface (added 2026-06-01
// for Agent G's S3 lister). Validator/Writer tests don't exercise List,
// so a minimal implementation that walks the in-memory map is enough.
func (f *fakeBSL) ListPrefix(ctx context.Context, bslName, keyPrefix string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var keys []string
	bslKeyPrefix := f.k(bslName, keyPrefix)
	for k := range f.objects {
		if strings.HasPrefix(k, bslKeyPrefix) {
			keys = append(keys, strings.TrimPrefix(k, bslName+"/"))
		}
	}
	return keys, nil
}

func seedValidFingerprint(t *testing.T, bsl *fakeBSL, bslName, backupName, secret string) *Fingerprint {
	t.Helper()
	fp := &Fingerprint{
		Version:           Version,
		SourceClusterID:   strings.Repeat("a", 32),
		SourceClusterName: "src-cluster",
		BackupName:        backupName,
		BackupNamespace:   DefaultVeleroNS,
		CreatedAt:         "2026-06-01T10:00:00Z",
		TarballSHA256:     strings.Repeat("1", 64),
		MetadataSHA256:    strings.Repeat("2", 64),
		Algo:              Algo,
	}
	fp.HMACSHA256 = computeHMAC([]byte(secret), fp)
	body, _ := json.Marshal(fp)
	if err := bsl.PutObject(context.Background(), bslName, fingerprintKey(backupName), body); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return fp
}

func TestValidator_ModeDisabled_ShortCircuits(t *testing.T) {
	bsl := newFakeBSL()
	bsl.failGet = true // ensures we DIDN'T hit BSL
	v := NewValidator(bsl, &StaticSecretLoader{Secret: []byte("ignored")})

	res, err := v.ValidateBackup(context.Background(), "bsl-x", "any-backup", ModeDisabled)
	if err != nil {
		t.Fatalf("disabled mode should not error: %v", err)
	}
	if res.Status != StatusDisabled {
		t.Fatalf("expected StatusDisabled, got %s", res.Status)
	}
}

func TestValidator_ModeEnforce_MissingReturnsRequired(t *testing.T) {
	bsl := newFakeBSL()
	v := NewValidator(bsl, &StaticSecretLoader{Secret: []byte(refSecret)})

	res, err := v.ValidateBackup(context.Background(), "bsl-x", "no-such-backup", ModeEnforce)
	if !errors.Is(err, ErrFingerprintRequired) {
		t.Fatalf("expected ErrFingerprintRequired, got %v", err)
	}
	if res.Status != StatusMissing {
		t.Fatalf("expected StatusMissing, got %s", res.Status)
	}
}

func TestValidator_ModeWarn_MissingNoError(t *testing.T) {
	bsl := newFakeBSL()
	v := NewValidator(bsl, &StaticSecretLoader{Secret: []byte(refSecret)})

	res, err := v.ValidateBackup(context.Background(), "bsl-x", "no-such-backup", ModeWarn)
	if err != nil {
		t.Fatalf("warn mode + missing should NOT error: %v", err)
	}
	if res.Status != StatusMissing {
		t.Fatalf("expected StatusMissing, got %s", res.Status)
	}
}

func TestValidator_ValidFingerprint_Passes(t *testing.T) {
	bsl := newFakeBSL()
	seedValidFingerprint(t, bsl, "bsl-x", "bk-1", refSecret)
	v := NewValidator(bsl, &StaticSecretLoader{Secret: []byte(refSecret)})

	for _, mode := range []FingerprintMode{ModeEnforce, ModeWarn} {
		t.Run(string(mode), func(t *testing.T) {
			res, err := v.ValidateBackup(context.Background(), "bsl-x", "bk-1", mode)
			if err != nil {
				t.Fatalf("valid fingerprint should not error: %v", err)
			}
			if res.Status != StatusValid {
				t.Fatalf("expected StatusValid, got %s (reason=%s)", res.Status, res.Reason)
			}
			if res.Fingerprint == nil || res.Fingerprint.BackupName != "bk-1" {
				t.Fatalf("expected returned fingerprint, got %+v", res.Fingerprint)
			}
		})
	}
}

func TestValidator_TamperedHMAC_FailsEvenInWarn(t *testing.T) {
	// PRD: HMAC failure is "stronger signal than missing" — block in warn too.
	bsl := newFakeBSL()
	fp := seedValidFingerprint(t, bsl, "bsl-x", "bk-tamp", refSecret)
	// Mutate stored bytes: change tarballSHA256 without recomputing HMAC.
	fp.TarballSHA256 = strings.Repeat("9", 64)
	body, _ := json.Marshal(fp)
	_ = bsl.PutObject(context.Background(), "bsl-x", fingerprintKey("bk-tamp"), body)

	v := NewValidator(bsl, &StaticSecretLoader{Secret: []byte(refSecret)})

	for _, mode := range []FingerprintMode{ModeEnforce, ModeWarn} {
		t.Run(string(mode), func(t *testing.T) {
			res, err := v.ValidateBackup(context.Background(), "bsl-x", "bk-tamp", mode)
			if !errors.Is(err, ErrFingerprintHMACInvalid) {
				t.Fatalf("expected ErrFingerprintHMACInvalid in mode=%s, got %v (status=%s)", mode, err, res.Status)
			}
			if res.Status != StatusHMACInvalid {
				t.Fatalf("expected StatusHMACInvalid, got %s", res.Status)
			}
		})
	}
}

func TestValidator_WrongSecret_FailsHMAC(t *testing.T) {
	bsl := newFakeBSL()
	seedValidFingerprint(t, bsl, "bsl-x", "bk-x", refSecret)
	// Validator runs with a different secret — same effect as if BSL
	// was carrying a fingerprint forged by a different cluster.
	v := NewValidator(bsl, &StaticSecretLoader{Secret: []byte("a-completely-different-key-32by")})

	res, err := v.ValidateBackup(context.Background(), "bsl-x", "bk-x", ModeEnforce)
	if !errors.Is(err, ErrFingerprintHMACInvalid) {
		t.Fatalf("expected ErrFingerprintHMACInvalid, got %v", err)
	}
	if res.Status != StatusHMACInvalid {
		t.Fatalf("expected StatusHMACInvalid, got %s", res.Status)
	}
}

func TestValidator_BackupNameMismatch_Tampered(t *testing.T) {
	// Stamp says BackupName=foo but it was found under bar/ → reject.
	bsl := newFakeBSL()
	seedValidFingerprint(t, bsl, "bsl-x", "real-name", refSecret)
	// Copy the bytes to a different path (simulating an attacker who
	// moved a valid stamp into another backup's directory).
	raw, _ := bsl.GetObject(context.Background(), "bsl-x", fingerprintKey("real-name"))
	_ = bsl.PutObject(context.Background(), "bsl-x", fingerprintKey("moved-name"), raw)

	v := NewValidator(bsl, &StaticSecretLoader{Secret: []byte(refSecret)})

	res, err := v.ValidateBackup(context.Background(), "bsl-x", "moved-name", ModeEnforce)
	if !errors.Is(err, ErrFingerprintTampered) {
		t.Fatalf("expected ErrFingerprintTampered, got %v", err)
	}
	if res.Status != StatusTampered {
		t.Fatalf("expected StatusTampered, got %s", res.Status)
	}
}

func TestValidator_UnsupportedAlgo_Rejected(t *testing.T) {
	bsl := newFakeBSL()
	fp := seedValidFingerprint(t, bsl, "bsl-x", "bk-algo", refSecret)
	fp.Algo = "RSA-SHA256"
	body, _ := json.Marshal(fp)
	_ = bsl.PutObject(context.Background(), "bsl-x", fingerprintKey("bk-algo"), body)

	v := NewValidator(bsl, &StaticSecretLoader{Secret: []byte(refSecret)})

	_, err := v.ValidateBackup(context.Background(), "bsl-x", "bk-algo", ModeEnforce)
	if !errors.Is(err, ErrFingerprintHMACInvalid) {
		t.Fatalf("expected ErrFingerprintHMACInvalid for bad algo, got %v", err)
	}
}

func TestValidator_MalformedJSON_HMACInvalid(t *testing.T) {
	bsl := newFakeBSL()
	_ = bsl.PutObject(context.Background(), "bsl-x", fingerprintKey("bk-junk"), []byte("not valid json {{"))

	v := NewValidator(bsl, &StaticSecretLoader{Secret: []byte(refSecret)})
	_, err := v.ValidateBackup(context.Background(), "bsl-x", "bk-junk", ModeEnforce)
	if !errors.Is(err, ErrFingerprintHMACInvalid) {
		t.Fatalf("expected ErrFingerprintHMACInvalid, got %v", err)
	}
}
