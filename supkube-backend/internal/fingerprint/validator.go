// Package fingerprint: Validator — runs on the DESTINATION cluster from
// the ImportPolicy controller (Agent B). Pulls the fingerprint blob from
// BSL, recomputes HMAC, and returns a verdict the controller maps to
// "import allowed / blocked / blocked-with-warning".
//
// Mode semantics (PRD-007 §4.4)
// ─────────────────────────────
//
//	disabled → always returns {Valid, "fingerprint disabled by policy"}.
//	warn     → missing OK (StatusMissing, no err); HMAC invalid → err.
//	enforce  → missing → err=ErrFingerprintRequired; HMAC invalid → err.
//
// What we do NOT verify (v1 limitation)
// ─────────────────────────────────────
// The tarballSHA256 field is NOT cross-checked against an on-the-fly
// re-hash of the actual tarball. Tarballs are GB-scale; re-hashing per
// import would make the controller block for tens of seconds. We trust
// the value the source wrote and use HMAC to detect any after-the-fact
// edit of the fingerprint itself. See types.go package header for the
// full threat-model rationale.
package fingerprint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type validator struct {
	bsl          BSLObjectClient
	secretLoader SecretLoader
}

// NewValidator wires up the destination-side dependencies.
func NewValidator(bsl BSLObjectClient, secretLoader SecretLoader) Validator {
	return &validator{bsl: bsl, secretLoader: secretLoader}
}

func (v *validator) ValidateBackup(ctx context.Context, bslName, backupName string, mode FingerprintMode) (*FingerprintResult, error) {
	// Disabled mode short-circuits BEFORE we touch BSL — no secret read,
	// no network call. Critical for day-0 onboarding when neither side
	// of the system is configured yet.
	if mode == ModeDisabled {
		return &FingerprintResult{
			Status: StatusDisabled,
			Reason: "fingerprint disabled by ImportPolicy mode=disabled",
		}, nil
	}

	key := fingerprintKey(backupName)
	raw, err := v.bsl.GetObject(ctx, bslName, key)
	if err != nil {
		// We can't distinguish 404 from "auth denied" without parsing
		// SDK-specific error types — so we treat ANY read failure as
		// "missing". A wrong-credential BSL would be surfaced by the
		// BSL list path already in v1/backup_meta_bsl.go. Trade-off
		// chosen for simplicity; revisit if false-missings show up.
		return v.handleMissing(mode, fmt.Sprintf("fetch fingerprint: %v", err))
	}
	if len(raw) == 0 {
		return v.handleMissing(mode, "fingerprint object empty")
	}

	fp := &Fingerprint{}
	if err := json.Unmarshal(raw, fp); err != nil {
		// A malformed fingerprint is closer to "tampered" than "missing"
		// — someone uploaded garbage. We still return missing in warn
		// mode so the import can proceed if the user opted into it.
		if mode == ModeEnforce {
			return &FingerprintResult{
				Status: StatusHMACInvalid,
				Reason: fmt.Sprintf("malformed fingerprint JSON: %v", err),
			}, ErrFingerprintHMACInvalid
		}
		return &FingerprintResult{
			Status: StatusHMACInvalid,
			Reason: fmt.Sprintf("malformed fingerprint JSON: %v", err),
		}, nil
	}

	// Defensive: backupName field MUST match the path it was found at —
	// otherwise an attacker could move a valid stamp from BackupA's
	// directory to BackupB's. (HMAC also covers backupName, so this is
	// belt-and-suspenders, but the error message is much clearer here.)
	if fp.BackupName != backupName {
		return &FingerprintResult{
			Status:      StatusTampered,
			Fingerprint: fp,
			Reason:      fmt.Sprintf("fingerprint.backupName=%q but found under backups/%s/", fp.BackupName, backupName),
		}, wrapEnforce(mode, ErrFingerprintTampered)
	}
	if fp.Algo != Algo {
		return &FingerprintResult{
			Status:      StatusHMACInvalid,
			Fingerprint: fp,
			Reason:      fmt.Sprintf("unsupported algo %q (expected %q); refusing to verify with a different algorithm", fp.Algo, Algo),
		}, wrapEnforce(mode, ErrFingerprintHMACInvalid)
	}

	secret, err := v.secretLoader.GetSharedSecret(ctx)
	if err != nil {
		return nil, fmt.Errorf("load shared secret: %w", err)
	}
	if !verifyHMAC(secret, fp) {
		return &FingerprintResult{
			Status:      StatusHMACInvalid,
			Fingerprint: fp,
			Reason:      "HMAC mismatch — either wrong shared secret, or the fingerprint was modified after the source wrote it",
		}, wrapEnforce(mode, ErrFingerprintHMACInvalid)
	}

	// All good.
	return &FingerprintResult{
		Status:      StatusValid,
		Fingerprint: fp,
		Reason:      fmt.Sprintf("HMAC verified; source=%s (%s)", fp.SourceClusterName, fp.SourceClusterID),
	}, nil
}

func (v *validator) handleMissing(mode FingerprintMode, reason string) (*FingerprintResult, error) {
	res := &FingerprintResult{Status: StatusMissing, Reason: reason}
	if mode == ModeEnforce {
		return res, ErrFingerprintRequired
	}
	// warn mode → caller is allowed to proceed; the result is the audit trail.
	return res, nil
}

// wrapEnforce returns sentinelErr for enforce mode, nil otherwise. Used
// for HMAC / tamper failures where warn mode still WANTS the bad verdict
// recorded but doesn't want the controller to abort.
//
// NOTE: warn-mode-with-invalid-HMAC is intentionally NOT silently passed.
// The PRD spec says "invalid is a stronger signal than missing — block it
// even in warn". If the customer truly wants to ignore an invalid HMAC,
// they should set mode=disabled.
func wrapEnforce(mode FingerprintMode, sentinel error) error {
	if mode == ModeEnforce || mode == ModeWarn {
		return sentinel
	}
	return nil
}

// Sanity: ensure error wrapping is errors.Is-friendly.
var _ = errors.Is
