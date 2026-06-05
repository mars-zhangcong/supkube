// Package v1: cross_cluster_bsl.go — MC4 (v0.9.0).
//
// Before a cross-cluster Restore CR is created on the target cluster,
// the target cluster's Velero must be able to read the source backup.
// Velero's BackupSyncController only syncs backups from BSLs *configured
// on that cluster*. So we need to:
//
//  1. Inspect the source Backup CR to find which BSL it lives in.
//  2. Refuse if the BSL is in-cluster local-only (supkube-local-store) —
//     the target cluster physically can't reach it.
//  3. Copy the BSL CR + its credential Secret to the target cluster (only
//     when not already present — never overwrite admin-configured BSLs).
//  4. Wait (with a 90s ceiling) for Velero's BackupSyncController on the
//     target to surface the Backup CR; only then is it safe to apply
//     the Restore CR.
//
// What we don't do (out of scope for MVP):
//   - Sync VolumeSnapshotLocations (target cluster uses its own).
//   - Validate the target cluster's Velero version compatibility — the
//     v1.18 wire format works back to v1.13, and we ship v1.18, so this
//     is "vanishingly unlikely to bite" territory for now.
//   - Cross-cluster restore from a Backup that itself failed/partial —
//     Velero will reject it; we surface the rejection rather than
//     pre-validating here.
package v1

import (
	"context"
	"errors"
	"fmt"
	"time"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/k8s"
	"github.com/supkube/supkube-backend/internal/velerons"
)

// veleroNS is the Velero install namespace, resolved from VELERO_NAMESPACE.
var veleroNS = velerons.Namespace()

const (
	// backupSyncTimeout caps how long we wait for the target cluster's
	// Velero to surface our source Backup. Velero defaults to a 60s
	// backupSyncPeriod, so 90s gives us a healthy headroom + accounts
	// for an outbound API server round-trip on slower networks.
	backupSyncTimeout = 90 * time.Second
	backupSyncPoll    = 5 * time.Second
)

// errLocalBSLCantCrossCluster — sentinel so the SPA can surface a
// distinctive message rather than the generic "remote error".
var errLocalBSLCantCrossCluster = errors.New(
	"source backup lives in the in-cluster Local BSL (supkube-local-store), " +
		"which is not reachable from another cluster. " +
		"Use a backup that exists in a Cloud BSL for cross-cluster restore.",
)

// prepareTargetForCrossClusterRestore implements steps 1-4 above. Called
// from CreateRestore right before dispatching the Restore CR remotely.
//
// Returns nil when the target is ready: BSL configured, credential Secret
// present, source Backup synced and visible on the target.
//
// On error the Restore CR is never applied — the caller surfaces the error
// to the SPA and the user sees a precise reason.
func prepareTargetForCrossClusterRestore(
	ctx context.Context,
	targetCli ctrlclient.Client,
	backupName string,
) error {
	// Local source-cluster client (for reading Backup, BSL, Secret).
	localCli, err := k8s.GetRuntimeClient()
	if err != nil {
		return fmt.Errorf("local client: %w", err)
	}
	localTyped, err := k8s.GetClient()
	if err != nil {
		return fmt.Errorf("local typed client: %w", err)
	}

	// ── Step 1: read the source Backup to find its BSL.
	sourceBackup := &velerov1.Backup{}
	if err := localCli.Get(ctx, ctrlclient.ObjectKey{Namespace: veleroNS, Name: backupName}, sourceBackup); err != nil {
		return fmt.Errorf("read source backup %q: %w", backupName, err)
	}
	bslName := sourceBackup.Spec.StorageLocation
	if bslName == "" {
		return fmt.Errorf("source backup %q has no storageLocation", backupName)
	}

	// ── Step 2: read the source BSL CR.
	sourceBSL := &velerov1.BackupStorageLocation{}
	if err := localCli.Get(ctx, ctrlclient.ObjectKey{Namespace: veleroNS, Name: bslName}, sourceBSL); err != nil {
		return fmt.Errorf("read source BSL %q: %w", bslName, err)
	}
	// Local BSL guard — supkube.io/bsl-role=local means the BSL is the
	// in-cluster MinIO our own Helm chart provisioned. Any other cluster
	// reaching `supkube-local-store.supkube.svc:9000` would resolve to
	// ITS OWN local store, not the source's. So we reject early.
	if sourceBSL.Labels["supkube.io/bsl-role"] == "local" {
		return errLocalBSLCantCrossCluster
	}

	// ── Step 3: ensure BSL + credential Secret exist on target.
	if err := ensureBSLOnTarget(ctx, targetCli, sourceBSL); err != nil {
		return fmt.Errorf("apply BSL on target: %w", err)
	}
	if sourceBSL.Spec.Credential != nil && sourceBSL.Spec.Credential.Name != "" {
		secretName := sourceBSL.Spec.Credential.Name
		srcSecret, err := localTyped.CoreV1().Secrets(veleroNS).Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("read source credential secret %q: %w", secretName, err)
		}
		if err := ensureSecretOnTarget(ctx, targetCli, srcSecret); err != nil {
			return fmt.Errorf("apply credential secret on target: %w", err)
		}
	}

	// ── Step 4: wait for Velero on target to sync the Backup CR.
	if err := waitForBackupOnTarget(ctx, targetCli, backupName); err != nil {
		return fmt.Errorf("waiting for target Velero to sync backup metadata: %w", err)
	}
	return nil
}

// ensureBSLOnTarget creates a copy of the source BSL on the target — but
// only if no BSL with the same name already exists. Existing BSLs are left
// alone (the admin may have intentionally configured a different
// credential / region for the same name).
func ensureBSLOnTarget(ctx context.Context, target ctrlclient.Client, source *velerov1.BackupStorageLocation) error {
	existing := &velerov1.BackupStorageLocation{}
	err := target.Get(ctx, ctrlclient.ObjectKey{Namespace: veleroNS, Name: source.Name}, existing)
	if err == nil {
		// Already present. Don't touch — admin may have customised it.
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	clone := &velerov1.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      source.Name,
			Namespace: veleroNS,
			Labels:    copyMap(source.Labels),
			Annotations: mergeAnnotations(source.Annotations, map[string]string{
				"supkube.io/synced-from": "cross-cluster-restore",
			}),
		},
		Spec: *source.Spec.DeepCopy(),
	}
	// Never carry over "default: true" — the target might already have
	// its own default BSL and shouldn't have ours stomp it.
	clone.Spec.Default = false

	return target.Create(ctx, clone)
}

// ensureSecretOnTarget mirrors the credential Secret. Same "don't overwrite"
// rule as BSL — if the target already has a Secret with this name (because
// it's also using this BSL for its own backups, perhaps), we leave it.
func ensureSecretOnTarget(ctx context.Context, target ctrlclient.Client, source *corev1.Secret) error {
	existing := &corev1.Secret{}
	err := target.Get(ctx, ctrlclient.ObjectKey{Namespace: veleroNS, Name: source.Name}, existing)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	clone := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      source.Name,
			Namespace: veleroNS,
			Labels:    copyMap(source.Labels),
			Annotations: mergeAnnotations(source.Annotations, map[string]string{
				"supkube.io/synced-from": "cross-cluster-restore",
			}),
		},
		Type: source.Type,
		Data: copyByteMap(source.Data),
	}
	return target.Create(ctx, clone)
}

// waitForBackupOnTarget polls until the target cluster's BackupSyncController
// (a Velero built-in) materialises a Backup CR with the matching name in the
// target's velero namespace. Returns nil on success, or a timeout error.
func waitForBackupOnTarget(ctx context.Context, target ctrlclient.Client, backupName string) error {
	deadline := time.Now().Add(backupSyncTimeout)
	for {
		got := &velerov1.Backup{}
		err := target.Get(ctx, ctrlclient.ObjectKey{Namespace: veleroNS, Name: backupName}, got)
		if err == nil {
			// Sanity: BackupSyncController only writes when it could
			// successfully read the manifest from the BSL. So a
			// surfaced Backup means restore can proceed.
			return nil
		}
		if !apierrors.IsNotFound(err) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf(
				"backup %q did not appear on target within %s — verify the BSL is reachable from the target cluster and credentials are correct",
				backupName, backupSyncTimeout,
			)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backupSyncPoll):
		}
	}
}

// ─── tiny helpers ─────────────────────────────────────────────────────

func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copyByteMap(m map[string][]byte) map[string][]byte {
	if m == nil {
		return nil
	}
	out := make(map[string][]byte, len(m))
	for k, v := range m {
		dup := make([]byte, len(v))
		copy(dup, v)
		out[k] = dup
	}
	return out
}

func mergeAnnotations(base, extra map[string]string) map[string]string {
	out := copyMap(base)
	if out == nil {
		out = map[string]string{}
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
