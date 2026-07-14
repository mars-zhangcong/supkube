package license

import (
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"
)

// goldenLicense is a REAL license signed by the upstream Licensor (from the
// Phase-4 handoff prompt). If Verify accepts it against the embedded public
// key, our yaml.v3 marshalling is byte-for-byte symmetric with the signer's —
// the whole feature hinges on this staying green.
const goldenLicense = `id: trial-07a3a3ae-808a-4053-8dd4-0c24bc9fffa6
product: SupKube
customer: cluster-deploy@jumborca.net
edition: trial
dateStart: 2026-07-12T23:13:10Z
dateEnd: 2026-08-11T23:13:10Z
features:
    - core
restrictions:
    nodes: 3
    clusters: 1
version: v1.0.0
signature: e1u/gTzytps7NdaCeH7VPx1djNkKfUAkWIsoUBjBCW0n1bZEtTUuOpFrcxaj9UDyXGqhPzNXacz7NZPjtGzhCw==
`

func TestVerify_GoldenLicense(t *testing.T) {
	lic, err := Verify([]byte(goldenLicense))
	if err != nil {
		t.Fatalf("golden license must verify (proves yaml.v3 symmetry with signer): %v", err)
	}
	if lic.ID != "trial-07a3a3ae-808a-4053-8dd4-0c24bc9fffa6" {
		t.Errorf("id = %q", lic.ID)
	}
	if lic.Product != "SupKube" || lic.Edition != "trial" {
		t.Errorf("product/edition = %q/%q", lic.Product, lic.Edition)
	}
	if lic.Restrictions.Nodes != 3 || lic.Restrictions.Clusters != 1 {
		t.Errorf("restrictions = %+v", lic.Restrictions)
	}
	if len(lic.Features) != 1 || lic.Features[0] != "core" {
		t.Errorf("features = %v", lic.Features)
	}
}

func TestVerify_TamperedNodes(t *testing.T) {
	// Bump the node quota but keep the original signature → must be rejected.
	tampered := strings.Replace(goldenLicense, "nodes: 3", "nodes: 99", 1)
	if _, err := Verify([]byte(tampered)); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("tampered nodes must fail with ErrBadSignature, got %v", err)
	}
}

func TestVerify_TamperedDateEnd(t *testing.T) {
	// Push the expiry out to the year 2100 → signature no longer covers it.
	tampered := strings.Replace(goldenLicense, "2026-08-11T23:13:10Z", "2100-08-11T23:13:10Z", 1)
	if _, err := Verify([]byte(tampered)); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("tampered dateEnd must fail with ErrBadSignature, got %v", err)
	}
}

func TestVerify_EmptySignature(t *testing.T) {
	unsigned := strings.Replace(goldenLicense,
		"signature: e1u/gTzytps7NdaCeH7VPx1djNkKfUAkWIsoUBjBCW0n1bZEtTUuOpFrcxaj9UDyXGqhPzNXacz7NZPjtGzhCw==",
		"signature: \"\"", 1)
	if _, err := Verify([]byte(unsigned)); !errors.Is(err, ErrNoSignature) {
		t.Fatalf("empty signature must fail with ErrNoSignature, got %v", err)
	}
}

func TestVerify_BadBase64Signature(t *testing.T) {
	bad := strings.Replace(goldenLicense,
		"signature: e1u/gTzytps7NdaCeH7VPx1djNkKfUAkWIsoUBjBCW0n1bZEtTUuOpFrcxaj9UDyXGqhPzNXacz7NZPjtGzhCw==",
		"signature: not-valid-base64!!!", 1)
	if _, err := Verify([]byte(bad)); err == nil {
		t.Fatal("invalid base64 signature must error")
	}
}

func TestVerify_GarbageYAML(t *testing.T) {
	if _, err := Verify([]byte("::: not yaml :::")); err == nil {
		t.Fatal("unparseable yaml must error")
	}
}

func TestPublicKey_Loads(t *testing.T) {
	pk := PublicKey()
	if len(pk) != ed25519.PublicKeySize {
		t.Fatalf("public key size = %d, want %d", len(pk), ed25519.PublicKeySize)
	}
}
