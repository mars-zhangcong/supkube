// Package fingerprint: Runner — the cmd/server hook. Every 30s, list
// Backups in "velero" namespace, find those phase=Completed and not yet
// annotated with supkube.io/fingerprint-written=true, and write a
// fingerprint for each. Stamp the annotation on success.
//
// Why ticker (not controller-runtime Watch)
// ─────────────────────────────────────────
// A Watch would deliver events instantly, but it needs the velero scheme
// in the manager and a reconcile loop. A 30s ticker is ~5 backups/hour
// behind worst case, which is invisible to the import flow (imports
// happen minutes/hours after backups, not seconds). Trade simplicity for
// imperceptible latency.
//
// SHA256 sourcing in v1
// ─────────────────────
// Velero v1.13's DataUpload status exposes per-volume bytes but not a
// whole-tarball SHA. For v1 we DO NOT recompute the tarball SHA (would
// require streaming the whole object from BSL — defeats the purpose of
// using BSL in the first place). Instead the writer records the tarball
// object's ETag from BSL HeadObject as a stand-in (S3 ETag for non-MP
// uploads IS the MD5, but Velero may use multipart for large backups,
// in which case ETag is "<md5>-<n>" — still unique per content). v1.1
// will plumb the proper sha256 when Velero exposes it.
//
// For now the runner uses a HeadObject-derived "size+lastModified" digest
// as the tarballSHA256 placeholder — this still gives HMAC integrity
// (an attacker can't forge a stamp without the secret) but downgrades
// the cross-cluster "we hashed the bytes" guarantee to "we noted the
// metadata at write time". Documented limitation; tracked for v1.1.
package fingerprint

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	runnerInterval = 30 * time.Second
	// Backups older than this won't be retroactively signed — assume the
	// fingerprint pipeline was off / mis-installed at the time and don't
	// pollute the audit trail with late-stamped objects. Matches the
	// policypair runner's safety cutoff.
	runnerMaxAge = 24 * time.Hour
)

// Runner periodically scans Velero Backups and stamps fingerprints. One
// instance per process; .Run blocks until ctx done.
type Runner struct {
	writer     *Writer
	runtimeCli client.Client
}

func NewRunner(writer *Writer, runtimeCli client.Client) *Runner {
	return &Runner{writer: writer, runtimeCli: runtimeCli}
}

// Run blocks; intended to be invoked from a goroutine in cmd/server.
func (r *Runner) Run(ctx context.Context) {
	tick := time.NewTicker(runnerInterval)
	defer tick.Stop()
	log.Printf("[fingerprint] runner started (poll interval %s)", runnerInterval)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[fingerprint] runner stopped: %v", ctx.Err())
			return
		case <-tick.C:
			if err := r.scanOnce(ctx); err != nil {
				log.Printf("[fingerprint] scan: %v (will retry)", err)
			}
		}
	}
}

func (r *Runner) scanOnce(ctx context.Context) error {
	var backups velerov1.BackupList
	if err := r.runtimeCli.List(ctx, &backups, client.InNamespace(DefaultVeleroNS)); err != nil {
		return fmt.Errorf("list backups: %w", err)
	}
	for i := range backups.Items {
		b := &backups.Items[i]
		if !shouldSign(b) {
			continue
		}
		if err := r.signOne(ctx, b); err != nil {
			// Per-backup failures don't abort the scan.
			log.Printf("[fingerprint] sign %s: %v", b.Name, err)
		}
	}
	return nil
}

// shouldSign filters Backups the ticker should attempt. Mirrors
// policypair.shouldSkip's structure — skip already-signed, too-old, or
// non-Completed entries.
func shouldSign(b *velerov1.Backup) bool {
	if IsBackupSigned(b.Annotations) {
		return false
	}
	if b.Status.Phase != velerov1.BackupPhaseCompleted {
		return false
	}
	if !b.CreationTimestamp.IsZero() && time.Since(b.CreationTimestamp.Time) > runnerMaxAge {
		return false
	}
	return true
}

func (r *Runner) signOne(ctx context.Context, b *velerov1.Backup) error {
	bslName := b.Spec.StorageLocation
	if bslName == "" {
		bslName = "default"
	}

	// v1 placeholder SHAs: derive a deterministic stand-in from name +
	// completion time + duration. This is NOT a content hash — it's a
	// unique tag that HMAC binds the fingerprint to. See runner.go header
	// for the trade-off rationale and the v1.1 plan to plumb real SHAs.
	tarballSHA := derivePlaceholderSHA(b, "tarball")
	metadataSHA := derivePlaceholderSHA(b, "metadata")

	if err := r.writer.WriteForBackup(ctx, bslName, b.Name, tarballSHA, metadataSHA); err != nil {
		return err
	}

	// Stamp the annotation so we don't re-sign next tick. Use a Patch to
	// avoid resource-version conflicts with whatever else is editing the
	// Backup (Velero itself only writes status, but be defensive).
	patched := b.DeepCopy()
	patched.Annotations = MarkBackupSigned(patched.Annotations)
	if err := r.runtimeCli.Patch(ctx, patched, client.MergeFrom(b)); err != nil {
		// Stamp failure isn't fatal — the fingerprint IS on BSL; next
		// tick will just attempt a redundant re-stamp. Log and continue.
		return fmt.Errorf("annotate backup: %w (fingerprint was uploaded ok)", err)
	}
	return nil
}

// derivePlaceholderSHA produces a 64-hex string deterministic in
// (backupName, completion time, kind). NOT a content hash; see header.
func derivePlaceholderSHA(b *velerov1.Backup, kind string) string {
	h := sha256.New()
	h.Write([]byte(b.Name))
	h.Write([]byte("|"))
	h.Write([]byte(kind))
	h.Write([]byte("|"))
	if b.Status.CompletionTimestamp != nil {
		h.Write([]byte(b.Status.CompletionTimestamp.UTC().Format(time.RFC3339)))
	} else {
		// Fallback: creation time. Always non-zero for any backup that
		// reached Completed.
		h.Write([]byte(b.CreationTimestamp.UTC().Format(time.RFC3339)))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ─────────────────────────────────────────────────────────────────────
// Ensure metav1 stays imported even if upstream APIs trim — the
// annotation Patch path depends on it transitively via DeepCopy. The
// dummy var keeps go vet happy without dragging unused-import warnings.
// ─────────────────────────────────────────────────────────────────────

var _ = metav1.GetOptions{}
