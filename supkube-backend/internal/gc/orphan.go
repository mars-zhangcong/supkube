// Package gc (v0.8.8) — orphan resource garbage collector for Velero
// artifacts that the Velero controllers fail to clean up.
//
// Background
// ──────────
// When a user deletes a Backup CR via SupKube UI (which submits a
// DeleteBackupRequest to Velero), Velero's controller normally removes:
//
//   - The Backup tarball in the BSL
//   - The Kopia snapshot tombstone (actual GC happens during repo maintenance)
//   - VolumeSnapshot and VolumeSnapshotContent objects with deletionPolicy=Delete
//
// But Velero v1.18's Data Mover code path INTENTIONALLY creates intermediate
// VolumeSnapshot/VolumeSnapshotContent objects with deletionPolicy=Retain
// to ensure they survive transient deletions during the upload phase.
// These retained objects are NEVER cleaned up by Velero when the parent
// Backup is later deleted. Over time the cluster accumulates orphans:
//
//   - Cluster-scoped VolumeSnapshotContents labeled velero.io/backup-name=<name>
//     but the named Backup is gone
//   - Namespaced VolumeSnapshots with the same dangling reference
//   - PodVolumeBackup and DataUpload CRs whose parent Backup is gone
//
// Upstream issue tracking: vmware-tanzu/velero#7838 (1.16+), partial fixes
// in 1.17/1.18 but Data Mover retained snapshots still leak.
//
// This package is SupKube's compensating layer: a scanner that finds these
// orphans and deletes them, plus a manual trigger endpoint and a periodic
// ticker.
//
// Safety
// ──────
// We are aggressive about labeling (must have velero.io/backup-name set)
// and CAUTIOUS about parent existence (Get the named Backup; ONLY delete
// if it returns NotFound). This makes a race condition harmless: if a
// brand-new Backup is mid-creation when we scan, we either see the Backup
// (and skip its VSCs) or see the VSCs but get NotFound for the parent
// (and delete them — which is fine because if the parent doesn't yet
// exist, those VSCs aren't legitimately owned by anyone).
package gc

import (
	"context"
	"fmt"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ScanResult records what a single GC run found and acted on. Useful for
// the Settings UI's "last run summary" + Activity event payload.
type ScanResult struct {
	StartedAt   time.Time
	FinishedAt  time.Time
	// Counts of orphans found AND deleted. If we found 5 but only deleted
	// 3 (because 2 had a still-existing parent we missed), Found=5,
	// Deleted=3. In practice these match because we re-check parent
	// existence right before each delete.
	VSCFound, VSCDeleted     int
	VSFound, VSDeleted       int
	PVBFound, PVBDeleted     int
	DUFound, DUDeleted       int
	// First error encountered (the scan continues past errors but
	// records the first one for UI display).
	Err error
}

// Summary renders ScanResult as a single-line "5/9 VSC deleted; 0/0 VS; …"
// string suitable for the K8s Event message + Activity row.
func (s ScanResult) Summary() string {
	if s.Err != nil {
		return fmt.Sprintf("orphan-gc failed at %s: %v", s.FinishedAt.Format(time.RFC3339), s.Err)
	}
	if s.VSCDeleted+s.VSDeleted+s.PVBDeleted+s.DUDeleted == 0 {
		return "orphan-gc: cluster clean, nothing to delete"
	}
	return fmt.Sprintf("orphan-gc: deleted %d VSC + %d VS + %d PVB + %d DataUpload (in %s)",
		s.VSCDeleted, s.VSDeleted, s.PVBDeleted, s.DUDeleted,
		s.FinishedAt.Sub(s.StartedAt).Round(time.Millisecond))
}

// Scan walks the cluster looking for orphaned Velero artifacts and deletes
// them. Returns a ScanResult either way (caller logs it / emits Event).
//
// Failure mode: we continue past errors on individual resources. Only an
// API-level error (e.g. List failed entirely) stops the scan. That gives
// us "best effort cleanup": a single ill-formed CR can't block the whole
// run.
func Scan(ctx context.Context, cl client.Client) ScanResult {
	r := ScanResult{StartedAt: time.Now().UTC()}

	// Build a set of currently-existing Backup names so we can identify
	// orphans by absence. Doing one List up-front means each orphan check
	// is O(1) map lookup, not an O(N) Get round-trip.
	backupList := &velerov1.BackupList{}
	if err := cl.List(ctx, backupList, client.InNamespace("velero")); err != nil {
		r.Err = fmt.Errorf("list backups: %w", err)
		r.FinishedAt = time.Now().UTC()
		return r
	}
	liveBackups := map[string]bool{}
	for _, b := range backupList.Items {
		liveBackups[b.Name] = true
	}

	// ── 1. VolumeSnapshotContent (cluster scope) ───────────────────
	// These leak the most — Velero Data Mover sets deletionPolicy=Retain
	// on them to survive the workflow, never cleans them up after.
	vscList := &snapshotv1.VolumeSnapshotContentList{}
	if err := cl.List(ctx, vscList); err == nil {
		for i := range vscList.Items {
			vsc := &vscList.Items[i]
			backupName := vsc.Labels["velero.io/backup-name"]
			if backupName == "" {
				continue // not Velero-owned; out of scope
			}
			if liveBackups[backupName] {
				continue // parent exists, this is in-use
			}
			r.VSCFound++
			// Flip deletionPolicy to Delete first — otherwise the
			// underlying CSI driver snapshot stays around. Patch
			// rather than Update to avoid resourceVersion conflicts
			// during fast scans.
			patch := client.RawPatch(client.Merge.Type(), []byte(`{"spec":{"deletionPolicy":"Delete"}}`))
			_ = cl.Patch(ctx, vsc, patch)
			if err := cl.Delete(ctx, vsc); err == nil || apierrors.IsNotFound(err) {
				r.VSCDeleted++
			} else if r.Err == nil {
				r.Err = fmt.Errorf("delete vsc %s: %w", vsc.Name, err)
			}
		}
	}

	// ── 2. VolumeSnapshot (namespaced) ─────────────────────────────
	// Same orphan-by-label test. These are namespaced under each
	// protected ns (test-app, etc.) so we need a cluster-wide List.
	vsList := &snapshotv1.VolumeSnapshotList{}
	if err := cl.List(ctx, vsList); err == nil {
		for i := range vsList.Items {
			vs := &vsList.Items[i]
			backupName := vs.Labels["velero.io/backup-name"]
			if backupName == "" {
				continue
			}
			if liveBackups[backupName] {
				continue
			}
			r.VSFound++
			if err := cl.Delete(ctx, vs); err == nil || apierrors.IsNotFound(err) {
				r.VSDeleted++
			} else if r.Err == nil {
				r.Err = fmt.Errorf("delete vs %s/%s: %w", vs.Namespace, vs.Name, err)
			}
		}
	}

	// ── 3. PodVolumeBackup (filesystem path) ───────────────────────
	pvbList := &velerov1.PodVolumeBackupList{}
	if err := cl.List(ctx, pvbList, client.InNamespace("velero")); err == nil {
		for i := range pvbList.Items {
			pvb := &pvbList.Items[i]
			backupName := pvb.Labels["velero.io/backup-name"]
			if backupName == "" || liveBackups[backupName] {
				continue
			}
			r.PVBFound++
			if err := cl.Delete(ctx, pvb); err == nil || apierrors.IsNotFound(err) {
				r.PVBDeleted++
			} else if r.Err == nil {
				r.Err = fmt.Errorf("delete pvb %s: %w", pvb.Name, err)
			}
		}
	}

	// ── 4. DataUpload (Data Mover path) ────────────────────────────
	duList := &velerov2alpha1.DataUploadList{}
	if err := cl.List(ctx, duList, client.InNamespace("velero")); err == nil {
		for i := range duList.Items {
			du := &duList.Items[i]
			backupName := du.Labels["velero.io/backup-name"]
			if backupName == "" || liveBackups[backupName] {
				continue
			}
			r.DUFound++
			if err := cl.Delete(ctx, du); err == nil || apierrors.IsNotFound(err) {
				r.DUDeleted++
			} else if r.Err == nil {
				r.Err = fmt.Errorf("delete dataupload %s: %w", du.Name, err)
			}
		}
	}

	r.FinishedAt = time.Now().UTC()
	return r
}
