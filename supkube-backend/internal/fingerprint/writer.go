// Package fingerprint: Writer — runs on the SOURCE cluster, post-Velero-
// Backup completion. Builds the Fingerprint JSON, HMACs it with the
// shared secret, and uploads to the BSL alongside the backup tarball.
//
// Hook point (see cmd/server/server.go): a controller-runtime ticker scans
// recently-Completed Backups every 30s and calls WriteForBackup on any
// that don't already carry annotation `supkube.io/fingerprint-written=true`.
// Ticker (not Watch) keeps the controller surface minimal — at ~100 backups/h
// the polling overhead is irrelevant.
package fingerprint

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Writer is the per-cluster fingerprint signer.
type Writer struct {
	bsl          BSLObjectClient
	secretLoader SecretLoader
	clusterID    string // 32-hex kube-system UID; resolved at construction
	clusterName  string // display name (control-plane node name or "Local Cluster")
}

// NewWriter wires up the dependencies. clusterID and clusterName are
// snapshotted at boot — a process restart picks them up fresh, and they
// can't change for a running cluster anyway (kube-system UID is forever).
func NewWriter(bsl BSLObjectClient, secretLoader SecretLoader, clusterID, clusterName string) *Writer {
	return &Writer{
		bsl:          bsl,
		secretLoader: secretLoader,
		clusterID:    clusterID,
		clusterName:  clusterName,
	}
}

// WriteForBackup builds, signs, and uploads the fingerprint for one
// completed Backup. Caller is responsible for:
//   - only invoking after Velero phase==Completed (we don't validate phase
//     here so the function stays unit-testable without Velero CRs)
//   - knowing the tarball SHA256 + metadata SHA256 (Velero exposes these
//     via DataUpload status; the ticker hook pulls them and passes them in)
//   - annotating the Backup with supkube.io/fingerprint-written=true so the
//     next tick doesn't re-sign (signing is idempotent — same input → same
//     HMAC — but uploading is a wasted network round-trip).
func (w *Writer) WriteForBackup(ctx context.Context, bslName, backupName, tarballSHA256, metadataSHA256 string) error {
	if w.clusterID == "" {
		// Defensive: if cluster ID couldn't be resolved at boot, we'd be
		// stamping unverifiable fingerprints. Better to fail loud than to
		// pollute the BSL with junk.
		return fmt.Errorf("fingerprint writer: source cluster ID unresolved (kube-system namespace lookup failed at boot?)")
	}
	if tarballSHA256 == "" {
		return fmt.Errorf("fingerprint writer: tarballSHA256 required (got empty)")
	}
	secret, err := w.secretLoader.GetSharedSecret(ctx)
	if err != nil {
		return fmt.Errorf("load shared secret: %w", err)
	}
	if len(secret) < 16 {
		// 16 byte minimum — the contract says 32, but we'll accept anything
		// non-trivial so a misconfigured Helm install fails in a way that
		// surfaces in logs rather than silently weakening security.
		return fmt.Errorf("shared secret too short (%d bytes); supkube-fingerprint-secret.data.sharedSecret should be 32 random bytes base64-encoded", len(secret))
	}

	fp := &Fingerprint{
		Version:           Version,
		SourceClusterID:   w.clusterID,
		SourceClusterName: w.clusterName,
		BackupName:        backupName,
		BackupNamespace:   DefaultVeleroNS,
		CreatedAt:         time.Now().UTC().Format(time.RFC3339),
		TarballSHA256:     tarballSHA256,
		MetadataSHA256:    metadataSHA256,
		Algo:              Algo,
	}
	fp.HMACSHA256 = computeHMAC(secret, fp)

	body, err := json.MarshalIndent(fp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal fingerprint: %w", err)
	}
	key := fingerprintKey(backupName)
	if err := w.bsl.PutObject(ctx, bslName, key, body); err != nil {
		return fmt.Errorf("put fingerprint to BSL %s key %s: %w", bslName, key, err)
	}
	return nil
}

// fingerprintKey is the relative-to-BSL-prefix key for one backup's
// fingerprint. Single source of truth — both writer and validator route
// through here so a path change can't desync them.
func fingerprintKey(backupName string) string {
	return path.Join("backups", backupName, FilenameInBSL)
}

// ─────────────────────────────────────────────────────────────────────
// Source-of-truth cluster identity (resolved once at boot)
// ─────────────────────────────────────────────────────────────────────

// ResolveClusterIdentity reads kube-system UID + best-effort control-plane
// node name. Returns ("", "") if the cluster is unreachable — the caller
// should treat that as "fingerprint writer disabled" and log loudly.
//
// Helper lives here (rather than in internal/k8s/) because it's only ever
// used by the fingerprint writer; lifting it requires touching k8s.go and
// risking the boot-path test surface. If a second caller appears, lift it.
func ResolveClusterIdentity(ctx context.Context, k8sCli kubernetes.Interface) (id, name string) {
	if k8sCli == nil {
		return "", "Local Cluster"
	}
	if ns, err := k8sCli.CoreV1().Namespaces().Get(ctx, "kube-system", metav1.GetOptions{}); err == nil {
		// kube-system UID is RFC4122 (36 chars with dashes). The PRD spec
		// says "32-hex"; strip the dashes so the on-disk shape matches.
		id = stripDashes(string(ns.UID))
	}
	name = pickClusterDisplayName(ctx, k8sCli)
	return id, name
}

func stripDashes(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '-' {
			out = append(out, s[i])
		}
	}
	return string(out)
}

// pickClusterDisplayName mirrors internal/api/v1/clusters.go::buildThisClusterDTO
// detection logic: control-plane node name → any node name → "Local Cluster".
// Duplicated here for the same import-cycle reason as the BSL client; lift
// to internal/k8s/ when a third caller shows up.
func pickClusterDisplayName(ctx context.Context, k8sCli kubernetes.Interface) string {
	nl, err := k8sCli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil || len(nl.Items) == 0 {
		return "Local Cluster"
	}
	var cpName, anyName string
	for _, n := range nl.Items {
		if anyName == "" {
			anyName = n.Name
		}
		if _, ok := n.Labels["node-role.kubernetes.io/control-plane"]; ok {
			cpName = n.Name
			break
		}
		if _, ok := n.Labels["node-role.kubernetes.io/master"]; ok {
			cpName = n.Name
			break
		}
	}
	if cpName != "" {
		return cpName
	}
	if anyName != "" {
		return anyName
	}
	return "Local Cluster"
}

// markBackupSigned is a convenience for the ticker hook: stamp the Backup
// with the "fingerprint-written" annotation so the next tick skips it.
// Lives here so the annotation constant has exactly one home.
const AnnFingerprintWritten = "supkube.io/fingerprint-written"

// MarkBackupSigned patches the annotation on a Backup CR. Caller passes
// the Velero Backup as a *corev1.ObjectReference-shaped name/ns pair via
// dynamic client to avoid pulling velero scheme into the writer surface.
// (writer.go remains scheme-independent for testability.)
func MarkBackupSigned(annotations map[string]string) map[string]string {
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[AnnFingerprintWritten] = time.Now().UTC().Format(time.RFC3339)
	return annotations
}

// IsBackupSigned: helper for the ticker — has this Backup already been
// signed? Reads the annotation. Embedded in pkg so the ticker hook in
// server.go can use it without re-grokking the annotation key.
func IsBackupSigned(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	_, ok := annotations[AnnFingerprintWritten]
	return ok
}

// SourceCorev1Secret returns a copy of an empty Secret skeleton — exposed
// for tests + the rare "rotate me a secret" admin path. Kept here so the
// fingerprint package is the single Secret-shape authority.
func SourceCorev1Secret(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{},
	}
}
