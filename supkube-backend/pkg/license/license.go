// Package license is the SupKube License consumer (Phase 4): it verifies a
// license signed by the upstream Licensor platform, enforces node quota, and
// reports status — read + verify + enforce, no UI.
//
// The verification MUST stay byte-for-byte symmetric with how the Licensor
// signs (Go + gopkg.in/yaml.v3): sign clears the signature field, marshals the
// struct, Ed25519-signs those bytes, then writes the base64 signature back.
// So the License struct's FIELD ORDER + yaml tags here must match the signer's
// exactly, and we must use yaml.v3 (v2 marshals differently and would fail).
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// publicKeyB64 is the Licensor's Ed25519 public key, embedded into the binary
// at build time. It is deliberately NOT overridable via config/ConfigMap — if
// a customer could swap the key they could bypass signature verification, so
// the embedded key IS the security boundary.
//
//go:embed public.key
var publicKeyB64 string

// License mirrors the Licensor's schema exactly (field order matters for the
// yaml.Marshal payload the signature covers — do not reorder).
type License struct {
	ID           string       `yaml:"id"`
	Product      string       `yaml:"product"`
	Customer     string       `yaml:"customer"`
	Edition      string       `yaml:"edition"`
	DateStart    time.Time    `yaml:"dateStart"`
	DateEnd      time.Time    `yaml:"dateEnd"`
	Features     []string     `yaml:"features,omitempty"`
	Restrictions Restrictions `yaml:"restrictions"`
	ClusterID    string       `yaml:"clusterId,omitempty"` // Phase 5: cluster binding (field kept, not enforced)
	Version      string       `yaml:"version"`
	Signature    string       `yaml:"signature,omitempty"` // empty while signing; base64 after
}

// Restrictions is the quota block. This Phase only enforces Nodes.
type Restrictions struct {
	Nodes    int `yaml:"nodes"`
	Clusters int `yaml:"clusters,omitempty"`
}

var (
	// ErrNoSignature is returned when the license carries no signature.
	ErrNoSignature = errors.New("license: signature is empty")
	// ErrBadSignature is returned when Ed25519 verification fails (tampered
	// or wrong key). Callers treat this as "invalid".
	ErrBadSignature = errors.New("license: signature invalid")
)

// Verify parses a raw license (the plaintext YAML the customer put in the
// Secret) and validates its Ed25519 signature against the embedded public key.
// It performs NO time/quota checks — those are policy the controller applies on
// top of a cryptographically-valid license.
func Verify(licBytes []byte) (*License, error) {
	var lic License
	if err := yaml.Unmarshal(licBytes, &lic); err != nil {
		return nil, fmt.Errorf("license: parse yaml: %w", err)
	}
	if strings.TrimSpace(lic.Signature) == "" {
		return nil, ErrNoSignature
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lic.Signature))
	if err != nil {
		return nil, fmt.Errorf("license: decode signature: %w", err)
	}

	// Reproduce the exact bytes the signer covered: the struct with an empty
	// signature field, marshalled by yaml.v3.
	signedCopy := lic
	signedCopy.Signature = ""
	payload, err := yaml.Marshal(&signedCopy)
	if err != nil {
		return nil, fmt.Errorf("license: re-marshal payload: %w", err)
	}

	if !ed25519.Verify(PublicKey(), payload, sig) {
		return nil, ErrBadSignature
	}
	return &lic, nil
}

var cachedPubKey ed25519.PublicKey

// PublicKey returns the embedded Licensor public key (decoded once). It panics
// on a malformed embedded key — that is a build-time defect, not a runtime
// input, so failing loud at first use is correct.
func PublicKey() ed25519.PublicKey {
	if cachedPubKey != nil {
		return cachedPubKey
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(publicKeyB64))
	if err != nil {
		panic("license: embedded public.key is not valid base64: " + err.Error())
	}
	if len(raw) != ed25519.PublicKeySize {
		panic(fmt.Sprintf("license: embedded public key size %d, want %d", len(raw), ed25519.PublicKeySize))
	}
	cachedPubKey = ed25519.PublicKey(raw)
	return cachedPubKey
}
