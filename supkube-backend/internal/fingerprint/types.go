// Package fingerprint implements PRD-007 §4.4 — cryptographic provenance
// stamps written alongside every Velero backup and verified at Import time.
//
// Why this exists
// ───────────────
// Velero backups in a BSL are just object-store blobs; anyone with bucket
// write access (or a malicious admin on the source cluster) can craft a
// "looks-real" Velero backup that the import flow would happily restore.
// SupKube ImportPolicy needs a way to say "this backup came from a cluster
// we trust, and nobody altered it after the source wrote it". HMAC-SHA256
// over the {sourceClusterID|backupName|tarballSHA256|createdAt} tuple with
// a pre-shared 32-byte secret gives us exactly that, at the cost of one
// extra small JSON object per backup.
//
// Shape on disk (BSL: <prefix>/backups/<backup-name>/.supkube-fingerprint.json):
//
//	{
//	  "version": "v1",
//	  "sourceClusterID": "<kube-system uid 32-hex>",
//	  "sourceClusterName": "<control-plane node name or 'Local Cluster'>",
//	  "backupName": "<velero-backup-name>",
//	  "backupNamespace": "velero",
//	  "createdAt": "<RFC3339>",
//	  "tarballSHA256": "<hex64>",
//	  "metadataSHA256": "<hex64>",
//	  "hmacSHA256": "<hex64>",
//	  "algo": "HMAC-SHA256"
//	}
//
// HMAC input = "{sourceClusterID}|{backupName}|{tarballSHA256}|{createdAt}"
// Key = base64-decoded supkube-fingerprint-secret.data.sharedSecret (32B).
//
// v1 trust model limitation
// ─────────────────────────
// We do NOT re-hash the tarball at verify time — tarballs can be GBs and
// the controller would block for tens of seconds per import. Instead, we
// trust the sha256 written by the source AT WRITE TIME and use HMAC to
// detect any later tamper of the fingerprint document itself. If an
// attacker swapped the tarball but kept the fingerprint, they'd produce
// a corrupted restore that Velero would reject; if they swapped both,
// they'd need the shared secret to recompute HMAC. Acceptable for v1;
// v1.1 may add an opt-in on-the-fly verifier for the paranoid mode.
package fingerprint

import (
	"context"
	"errors"
	"time"
)

// Schema constants — keep in lockstep with the shared-contract block in
// the PRD. Changing any of these is a breaking change for every existing
// fingerprint on every BSL; bump the Version field if it ever happens.
const (
	Version         = "v1"
	Algo            = "HMAC-SHA256"
	FilenameInBSL   = ".supkube-fingerprint.json"
	DefaultVeleroNS = "velero"
)

// FingerprintMode controls how strictly the validator enforces fingerprint
// presence/validity. The ImportPolicy CR carries the mode the user picked.
type FingerprintMode string

const (
	// ModeEnforce: missing or invalid fingerprint → import REJECTED.
	// Production default; the right answer once an org has rolled the
	// shared secret out to every source cluster.
	ModeEnforce FingerprintMode = "enforce"

	// ModeWarn: missing fingerprint → allowed but flagged in the result;
	// invalid (HMAC failure) → still rejected (an INVALID stamp is a
	// stronger signal than a MISSING one — someone tried to forge).
	ModeWarn FingerprintMode = "warn"

	// ModeDisabled: skip the check entirely. For day-0 onboarding before
	// any source cluster has been fitted with the writer.
	ModeDisabled FingerprintMode = "disabled"
)

// Fingerprint is the JSON document persisted to BSL. Field order in the
// struct mirrors the on-disk JSON for ease of code-review against the
// shared contract — DO NOT reorder.
type Fingerprint struct {
	Version           string `json:"version"`
	SourceClusterID   string `json:"sourceClusterID"`
	SourceClusterName string `json:"sourceClusterName"`
	BackupName        string `json:"backupName"`
	BackupNamespace   string `json:"backupNamespace"`
	CreatedAt         string `json:"createdAt"` // RFC3339; string (not time.Time) so re-serialisation is byte-identical
	TarballSHA256     string `json:"tarballSHA256"`
	MetadataSHA256    string `json:"metadataSHA256"`
	HMACSHA256        string `json:"hmacSHA256"`
	Algo              string `json:"algo"`
}

// FingerprintResultStatus is the high-level verdict returned to callers
// (ImportPolicy controller, GET /imports/preview, audit log entries).
type FingerprintResultStatus string

const (
	// StatusValid: fingerprint present + HMAC OK + fields consistent.
	StatusValid FingerprintResultStatus = "valid"
	// StatusMissing: no .supkube-fingerprint.json under the backup prefix.
	StatusMissing FingerprintResultStatus = "missing"
	// StatusTampered: fingerprint present but the stamped fields don't
	// match what the caller observed (e.g. tarballSHA256 differs from
	// what the importer recomputed if it bothered to). Reserved for
	// future on-the-fly verification; v1 doesn't emit this.
	StatusTampered FingerprintResultStatus = "tampered"
	// StatusHMACInvalid: HMAC didn't match — either wrong shared secret
	// or someone edited the fingerprint after the source wrote it.
	StatusHMACInvalid FingerprintResultStatus = "hmac-invalid"
	// StatusDisabled: validator was called with mode=disabled.
	StatusDisabled FingerprintResultStatus = "disabled"
)

// FingerprintResult is what Validator returns. The Fingerprint pointer is
// nil for Missing/Disabled (nothing to surface).
type FingerprintResult struct {
	Status      FingerprintResultStatus
	Fingerprint *Fingerprint
	// Reason is a human-readable explanation suitable for the UI tooltip
	// and the audit log. Always set; for Valid it's just confirmation.
	Reason string
}

// Validator is what the ImportPolicy controller (Agent B) depends on.
// Tests can inject a fake; production wires up *validator (lowercase).
type Validator interface {
	ValidateBackup(ctx context.Context, bslName, backupName string, mode FingerprintMode) (*FingerprintResult, error)
}

// Writer is what the post-Backup-completion hook depends on. Same split
// (interface for tests, struct for prod) as Validator.
type WriterInterface interface {
	WriteForBackup(ctx context.Context, bslName, backupName, tarballSHA256, metadataSHA256 string) error
}

// SecretLoader pulls the shared HMAC key. Abstracted so tests can use a
// fixed-bytes fake and production reads the K8s Secret.
type SecretLoader interface {
	GetSharedSecret(ctx context.Context) ([]byte, error)
}

// BSLObjectClient is the minimum BSL surface fingerprint needs: read +
// write a single small JSON object at a known key. Implemented by the
// bsl.go helper in this package. Interface (rather than a concrete type)
// keeps validator/writer testable without spinning up MinIO.
//
// ListPrefix was added (sprint 2026-06-01 Agent G) for the ImportPolicy
// S3 lister — without it, ImportPolicy was bounded by Velero's
// backupSyncPeriod (60-90s default) instead of our 30s continuousInterval
// floor. Keeping it on this interface (vs. a new one) lets the S3 lister
// reuse all the auth/provider plumbing already wired up here.
type BSLObjectClient interface {
	GetObject(ctx context.Context, bslName, key string) ([]byte, error)
	PutObject(ctx context.Context, bslName, key string, body []byte) error
	// Prefix returns the BSL's configured key prefix (Velero's "prefix"
	// field), so callers can build "<prefix>/backups/<name>/..." paths
	// without each one re-fetching the BSL CR.
	Prefix(ctx context.Context, bslName string) (string, error)
	// ListPrefix returns every object key whose name starts with keyPrefix
	// (which the caller has already joined with the BSL's configured
	// prefix — same convention as GetObject's key arg). Pagination is
	// handled by the implementation; callers receive the full list.
	// Order is provider-defined (S3 is ASCII-sorted; Azure is too in
	// practice) but callers MUST NOT rely on it — sort if needed.
	ListPrefix(ctx context.Context, bslName, keyPrefix string) ([]string, error)
}

// Error sentinels — callers use errors.Is to switch behaviour. Names
// match the shared-contract block in the PRD; do not rename without
// also editing the ImportPolicy controller.
var (
	ErrFingerprintMissing     = errors.New("fingerprint missing")
	ErrFingerprintTampered    = errors.New("fingerprint tampered")
	ErrFingerprintHMACInvalid = errors.New("fingerprint HMAC invalid")
	ErrFingerprintRequired    = errors.New("fingerprint required by ImportPolicy enforce mode")
)

// Internal: how long the SecretLoader caches the shared secret in memory
// before re-reading from K8s. 60s matches the BSL size cache TTL —
// short enough that secret rotation propagates quickly, long enough that
// a high-rate signer doesn't hammer the apiserver.
const secretCacheTTL = 60 * time.Second
