// Package policypair: Plan-B controller — when a dual-policy's
// snapshot-half Backup completes, immediately fire its paired
// export-half Backup so the two RPs reflect data captured ~30s
// apart instead of ~7 minutes.
//
// Why this exists (vs. Velero's native Schedule controller firing both)
// ─────────────────────────────────────────────────────────────────────
// In v0.8.9 a dual L2 policy creates TWO Schedule CRs (snapshot half +
// export half), both with the same cron. Velero's Schedule controller
// fires both at the cron tick — but each Backup CR then runs through
// Velero's backup queue independently. The export-half Backup takes
// its OWN fresh CSI snapshot (`snapshotMoveData=true` always pulls a
// new snapshot; it does NOT re-use the snapshot-half's snapshot CR).
// So in practice the two RPs end up capturing data several minutes
// apart, which violates the customer expectation that "this is one
// policy run, captured at one instant".
//
// Plan B: pause the export-half Schedule permanently, and have this
// controller watch for snapshot-half completion. When a snapshot-half
// Backup transitions to Completed, we immediately create the
// corresponding export-half Backup (with the same timestamp suffix
// in the name + a `policy-run-instant` annotation pointing at the
// snapshot-half's creationTimestamp). Net effect: data gap drops
// from minutes to ~30s (one CSI snapshot operation in cluster).
//
// Plan C handoff (future Velero v1.19+ upgrade)
// ─────────────────────────────────────────────
// We stamp every export-half Backup with annotation
// `velero.io/csi-volumesnapshot-content-retain-policy: retain` so that
// when upstream Velero gains `preserveSnapshotsAfterUpload`, our
// data path already keeps the snapshot CR around — we just flip the
// controller to "single-Backup with both artifacts" mode without
// breaking existing RP records.
//
// Failure handling
// ────────────────
//   - snapshot-half PartiallyFailed / Failed → do NOT fire export.
//     Reason: the snapshot is incomplete by definition; uploading it
//     bakes a known-bad state into BSL. Audit event explains the skip.
//   - export-half creation conflict (peer already exists) → idempotent.
//     We annotate the snapshot-half and move on.
//   - peer Schedule missing (e.g. L1 snapshot-only policy) → snapshot
//     half stays unpaired; controller annotates with a "no-peer" marker
//     so the same backup isn't scanned forever.
package policypair

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ─── Constants (intentionally duplicated from internal/api/v1/policy.go)
//
// These mirror labelPolicyName / labelPolicyRole / roleSnapshot /
// roleExport in the v1 package. They're unexported there, and the v0.8.10
// controller needs them too. Keeping the duplication contained to this
// header so a future refactor can lift them to an internal/labels/ pkg
// in one move.
const (
	labelPolicyName = "supkube.io/policy-name"
	labelPolicyRole = "supkube.io/policy-role"
	roleSnapshot    = "snapshot"
	roleExport      = "export"

	// AnnPolicyRunInstant: shared timestamp between the two halves of one
	// policy run. Frontend reads this and uses it as the unified "Created
	// At" displayed in both rows — so user perception is "one run" even
	// though there are two Backup CRs.
	AnnPolicyRunInstant = "supkube.io/policy-run-instant"

	// AnnDualRPPaired: cross-reference. On the snapshot half it points
	// to the export-half Backup name; on the export half it points to
	// the snapshot-half Backup name. Used by the Action Details drawer
	// to render a "Paired with" link.
	AnnDualRPPaired = "supkube.io/dual-rp-paired"

	// AnnDualRPNoPeer: stamped when scanning a snapshot-half Backup
	// found no peer export-half Schedule (e.g. L1 snapshot-only policy
	// converted from legacy v0.8.8 → no export half exists). We mark
	// the Backup so the controller doesn't re-scan it forever.
	AnnDualRPNoPeer = "supkube.io/dual-rp-no-peer"

	// AnnTriggeredBy mirrors v0.8.9.2's manual_snapshot.go usage. We
	// stamp "dual-pair-controller" on export halves we fire, so audit
	// logs can tell them apart from cron-fired backups.
	AnnTriggeredBy = "supkube.io/triggered-by"

	// AnnCSIVSCRetainPolicy: Velero v1.18 annotation that sets the
	// generated VolumeSnapshotContent's deletionPolicy to Retain. We
	// stamp it on every export-half Backup we create — even though
	// it's not used today, it preserves the storage-backend snapshot
	// for future Plan-C "single snapshot, two RPs" mode.
	AnnCSIVSCRetainPolicy = "velero.io/csi-volumesnapshot-content-retain-policy"

	// settingsConfigMap / settingsNamespace mirror gc package — same CM
	// stores all SupKube background-runner toggles.
	settingsConfigMap = "supkube-settings"
	settingsNamespace = "supkube"
	veleroNamespace   = "velero"

	// pollInterval: how often we scan for newly-completed snapshot-half
	// Backups. 10s is a sweet spot — faster than Velero's own controller
	// loops (which take 30-60s end-to-end for backup processing), and
	// slow enough not to thrash the K8s API. Net effect on data-gap:
	// snapshot completes at T → controller picks it up at most T+10s →
	// export-half Backup CR created at T+10s → Velero starts processing
	// within ~5s → fresh snapshot taken at T+~15s. Gap ≈ 15-30s vs
	// previous ~7 minutes.
	pollInterval = 10 * time.Second
)

// Settings is the parsed view of the supkube-settings CM for this
// runner. Defaults to enabled — Plan B is the v0.8.10 product promise;
// disabling it should be deliberate (e.g. for debugging a stuck pair).
type Settings struct {
	Enabled bool
}

func loadSettings(ctx context.Context, k8sCli kubernetes.Interface) Settings {
	s := Settings{Enabled: true}
	cm, err := k8sCli.CoreV1().ConfigMaps(settingsNamespace).Get(ctx, settingsConfigMap, metav1.GetOptions{})
	if err != nil {
		return s
	}
	if v, ok := cm.Data["policyPair.enabled"]; ok {
		s.Enabled = v == "true" || v == "True" || v == "TRUE"
	}
	return s
}

// Run is the entry point — call this from server.go in a goroutine.
// Blocks until ctx is cancelled. Loops every pollInterval.
//
// Cooperates with the gc package (same CM, same Event-emission style).
// We do NOT use a shared informer cache; the polling List with a label
// selector is cheap and easier to reason about than informer state.
func Run(ctx context.Context, runtimeCli client.Client, k8sCli kubernetes.Interface) {
	// One-time startup migration: pause any legacy v0.8.9 export-half
	// Schedules that were created with paused=false (the old default).
	// Plan B requires them to be paused so this controller has exclusive
	// trigger ownership — otherwise Velero's own Schedule controller will
	// race us at every cron tick.
	if err := migrateExportHalvesToPaused(ctx, runtimeCli); err != nil {
		log.Printf("[policypair] startup migration: %v (continuing — non-fatal)", err)
	}

	tick := time.NewTicker(pollInterval)
	defer tick.Stop()
	log.Printf("[policypair] runner started (poll interval %s)", pollInterval)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[policypair] runner stopped: %v", ctx.Err())
			return
		case <-tick.C:
			s := loadSettings(ctx, k8sCli)
			if !s.Enabled {
				continue
			}
			if err := scanOnce(ctx, runtimeCli, k8sCli); err != nil {
				log.Printf("[policypair] scan error: %v (will retry next tick)", err)
			}
		}
	}
}

// scanOnce runs one pass: list all Backups with policy-role=snapshot,
// for any in Completed phase that isn't already paired/marked-no-peer,
// try to fire the export half.
func scanOnce(ctx context.Context, runtimeCli client.Client, k8sCli kubernetes.Interface) error {
	var backups velerov1.BackupList
	if err := runtimeCli.List(ctx, &backups,
		client.InNamespace(veleroNamespace),
		client.MatchingLabels{labelPolicyRole: roleSnapshot},
	); err != nil {
		return fmt.Errorf("list snapshot-half backups: %w", err)
	}
	for i := range backups.Items {
		b := &backups.Items[i]
		if shouldSkip(b) {
			continue
		}
		if err := firePeerOrMark(ctx, runtimeCli, k8sCli, b); err != nil {
			// Per-backup error: log and continue so one bad pair
			// doesn't block the rest of the scan.
			log.Printf("[policypair] firePeer(%s): %v", b.Name, err)
		}
	}
	return nil
}

// shouldSkip decides if a snapshot-half Backup needs any action this tick.
// Returns true when:
//   - it's not Completed yet (still running, or failed)
//   - it's already been paired (annotation present)
//   - we've already determined it has no peer (no-peer marker)
//   - it's older than 24h (safety: stop scanning ancient backups forever)
//
// Failed / PartiallyFailed phases get the no-peer mark applied separately
// in firePeerOrMark — we don't want to fire export halves for incomplete
// data, but we also don't want to keep re-checking them forever.
func shouldSkip(b *velerov1.Backup) bool {
	if b.Annotations[AnnDualRPPaired] != "" {
		return true
	}
	if b.Annotations[AnnDualRPNoPeer] != "" {
		return true
	}
	// >24h old: stop scanning. If it never got paired by now, it's
	// either a controller that was off, or a bug. Either way, the
	// snapshot RP is still a valid independent RP — leave it alone.
	if !b.CreationTimestamp.IsZero() && time.Since(b.CreationTimestamp.Time) > 24*time.Hour {
		return true
	}
	switch b.Status.Phase {
	case velerov1.BackupPhaseCompleted:
		return false // proceed to fire
	case velerov1.BackupPhasePartiallyFailed, velerov1.BackupPhaseFailed,
		velerov1.BackupPhaseFailedValidation:
		// Will be handled in firePeerOrMark — we want to stamp the
		// no-peer marker so we don't scan again, but emit an audit
		// event explaining the skip. So DON'T skip here.
		return false
	default:
		// New / InProgress / WaitingForPluginOperations / Finalizing —
		// not ready, check again next tick.
		return true
	}
}

// firePeerOrMark is the per-Backup decision tree. Either:
//   - Backup is in a terminal failed state → mark no-peer + emit event
//   - Backup is Completed → find peer Schedule, create export Backup, cross-link
func firePeerOrMark(ctx context.Context, runtimeCli client.Client, k8sCli kubernetes.Interface, snap *velerov1.Backup) error {
	// Failure path: terminal but not Completed — never fire export.
	if snap.Status.Phase != velerov1.BackupPhaseCompleted {
		reason := fmt.Sprintf("snapshot-half phase=%s; skipping export-half fire (incomplete data)", snap.Status.Phase)
		emitEvent(ctx, k8sCli, snap, corev1.EventTypeWarning, "PolicyPairSkipped", reason)
		return annotate(ctx, runtimeCli, snap, map[string]string{
			AnnDualRPNoPeer: fmt.Sprintf("snapshot-half-phase=%s", snap.Status.Phase),
		})
	}

	policyName := snap.Labels[labelPolicyName]
	if policyName == "" {
		// Legacy v0.8.8 backup that somehow has policy-role=snapshot
		// but no policy-name. Mark no-peer and move on.
		return annotate(ctx, runtimeCli, snap, map[string]string{
			AnnDualRPNoPeer: "missing-policy-name-label",
		})
	}

	// Look for paired export-half Schedule. Selector matches BOTH the
	// policy name AND role=export to be unambiguous in case someone
	// has multiple policies with overlapping names.
	var schedules velerov1.ScheduleList
	if err := runtimeCli.List(ctx, &schedules,
		client.InNamespace(veleroNamespace),
		client.MatchingLabels{
			labelPolicyName: policyName,
			labelPolicyRole: roleExport,
		},
	); err != nil {
		return fmt.Errorf("list export schedules for policy %q: %w", policyName, err)
	}
	if len(schedules.Items) == 0 {
		// L1 snapshot-only policy or legacy single-Schedule policy.
		// Stamp no-peer and stop scanning this backup.
		emitEvent(ctx, k8sCli, snap, corev1.EventTypeNormal, "PolicyPairNoExport",
			fmt.Sprintf("policy %q has no export half (snapshot-only); pair-controller inactive for this run", policyName))
		return annotate(ctx, runtimeCli, snap, map[string]string{
			AnnDualRPNoPeer: "no-export-half-schedule",
		})
	}
	exportSched := &schedules.Items[0] // exactly one expected per policy-name

	exportBackup, err := buildExportBackup(snap, exportSched)
	if err != nil {
		return fmt.Errorf("build export Backup spec: %w", err)
	}

	// Idempotent create: if a Backup with this name already exists (e.g.
	// because the controller restarted between create and annotate-back),
	// treat as success and just cross-link.
	if err := runtimeCli.Create(ctx, exportBackup); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			emitEvent(ctx, k8sCli, snap, corev1.EventTypeWarning, "PolicyPairExportFailed",
				fmt.Sprintf("failed to create export-half Backup %q: %v", exportBackup.Name, err))
			return fmt.Errorf("create export Backup %q: %w", exportBackup.Name, err)
		}
		log.Printf("[policypair] export Backup %q already exists; treating as paired", exportBackup.Name)
	}

	// Cross-link: snapshot-half ← name of export-half. The export-half
	// already carries its peer pointer (set by buildExportBackup). We
	// ALSO stamp policy-run-instant on the snapshot half (= its own
	// creationTimestamp) so the UI's "Policy Run At" row renders the
	// SAME value on both halves' Action Details drawer. Without this
	// the snapshot card's drawer would hide the row, the export card
	// would show it — a confusing asymmetry that would make users
	// think the snapshot half "isn't a policy run".
	if err := annotate(ctx, runtimeCli, snap, map[string]string{
		AnnDualRPPaired:     exportBackup.Name,
		AnnPolicyRunInstant: snap.CreationTimestamp.UTC().Format(time.RFC3339),
	}); err != nil {
		return fmt.Errorf("annotate snapshot-half with peer name: %w", err)
	}

	emitEvent(ctx, k8sCli, snap, corev1.EventTypeNormal, "PolicyPairFired",
		fmt.Sprintf("export-half %q triggered ~%ds after snapshot-half completion",
			exportBackup.Name, int(time.Since(getCompletionOrNow(snap)).Seconds())))
	return nil
}

// buildExportBackup constructs the export-half Backup CR from the export
// Schedule's template. Naming + annotation strategy:
//
//	name = <policy>-export-<same-suffix-as-snapshot-half>
//	       The snapshot half is named <policy>-YYYYMMDDhhmmss (Velero's
//	       default for cron-fired backups). We mirror that suffix so the
//	       two halves are alphabetically adjacent and visually obvious as
//	       a pair. The "-export-" infix distinguishes from snapshot half.
//
// annotations:
//
//	policy-run-instant = snapshot-half's creationTimestamp (RFC3339).
//	   Frontend uses this as the unified "Created At" for both rows.
//	dual-rp-paired     = snapshot-half's name (back-reference)
//	triggered-by       = "dual-pair-controller" (audit trail)
//	csi-vsc-retain-policy = "retain" (Plan-C upgrade preserve)
//
// labels mirror the export Schedule's labels so the Backup shows up
// in PolicyAggregate's list-by-label query and the Activity feed.
func buildExportBackup(snap *velerov1.Backup, exportSched *velerov1.Schedule) (*velerov1.Backup, error) {
	suffix := suffixFromSnapshotName(snap.Name)
	if suffix == "" {
		// Fallback: use current UTC stamp. Less aesthetically paired
		// but functionally correct.
		suffix = time.Now().UTC().Format("20060102150405")
	}
	policyName := snap.Labels[labelPolicyName]
	name := fmt.Sprintf("%s-export-%s", policyName, suffix)

	annotations := map[string]string{
		AnnPolicyRunInstant:   snap.CreationTimestamp.UTC().Format(time.RFC3339),
		AnnDualRPPaired:       snap.Name,
		AnnTriggeredBy:        "dual-pair-controller",
		AnnCSIVSCRetainPolicy: "retain",
	}
	// Copy any user-set annotations from the export Schedule's own
	// metadata (e.g. notes the user added when configuring the policy).
	// Velero v1.13's ScheduleSpec.Template is a flat BackupSpec with no
	// nested Metadata — the user-facing Labels/Annotations for spawned
	// Backups live on the Schedule's own ObjectMeta. Don't overwrite
	// our controller-set ones.
	for k, v := range exportSched.Annotations {
		if _, exists := annotations[k]; !exists {
			annotations[k] = v
		}
	}

	labels := map[string]string{
		labelPolicyName: policyName,
		labelPolicyRole: roleExport,
		// Tag with schedule name so the Backup shows up in the
		// "Backups from this Policy" UI query (which still relies on
		// velero.io/schedule-name labels for historical reasons).
		"velero.io/schedule-name": exportSched.Name,
	}
	for k, v := range exportSched.Labels {
		if _, exists := labels[k]; !exists {
			labels[k] = v
		}
	}

	return &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   veleroNamespace,
			Labels:      labels,
			Annotations: annotations,
		},
		// Reuse the export Schedule's BackupSpec verbatim — TTL,
		// snapshotMoveData, storageLocation, includedNamespaces all
		// come from the user's policy config. We DO NOT re-derive
		// these from the snapshot half (which has different settings
		// like snapshotMoveData=false).
		Spec: *exportSched.Spec.Template.DeepCopy(),
	}, nil
}

// suffixFromSnapshotName extracts the timestamp suffix from a snapshot-half
// Backup name. Velero's cron-fired Backups are named "<schedule>-<14digits>"
// (YYYYMMDDhhmmss). We grab the last token after the final "-".
func suffixFromSnapshotName(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '-' {
			candidate := name[i+1:]
			if len(candidate) == 14 && isAllDigits(candidate) {
				return candidate
			}
			return ""
		}
	}
	return ""
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// annotate is a small JSON-patch helper. controller-runtime's Update path
// would also work but is more verbose for "set 1-2 annotations".
func annotate(ctx context.Context, runtimeCli client.Client, b *velerov1.Backup, kv map[string]string) error {
	patch := map[string]any{
		"metadata": map[string]any{
			"annotations": kv,
		},
	}
	raw, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	return runtimeCli.Patch(ctx, b, client.RawPatch(types.MergePatchType, raw))
}

// emitEvent writes a K8s Event in the supkube namespace tagged with
// `supkube.io/activity=true` so the existing Activity page picks it up
// without backend changes. Mirrors the gc package's event-emission style.
func emitEvent(ctx context.Context, k8sCli kubernetes.Interface, related *velerov1.Backup, eventType, reason, msg string) {
	now := time.Now()
	ev := &eventsv1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "policypair-",
			Namespace:    "supkube",
			Labels: map[string]string{
				"supkube.io/activity": "true",
				"supkube.io/source":   "policypair",
			},
			Annotations: map[string]string{
				"supkube.io/related-backup": related.Name,
			},
		},
		EventTime:           metav1.MicroTime{Time: now},
		ReportingController: "supkube.io/policypair",
		ReportingInstance:   "policypair-runner",
		Action:              reason,
		Reason:              reason,
		Note:                msg,
		Type:                eventType,
		Regarding: corev1.ObjectReference{
			APIVersion: "velero.io/v1",
			Kind:       "Backup",
			Name:       related.Name,
			Namespace:  related.Namespace,
			UID:        related.UID,
		},
	}
	if _, err := k8sCli.EventsV1().Events("supkube").Create(ctx, ev, metav1.CreateOptions{}); err != nil {
		log.Printf("[policypair] emit event %q failed: %v (non-fatal)", reason, err)
	}
}

func getCompletionOrNow(b *velerov1.Backup) time.Time {
	if b.Status.CompletionTimestamp != nil {
		return b.Status.CompletionTimestamp.Time
	}
	return time.Now()
}

// migrateExportHalvesToPaused runs once at controller startup. It finds
// all Schedules with role=export and forces spec.paused=true. This is
// a one-time data fix for any v0.8.9-era dual policies whose export
// halves were created with paused=false (legacy behavior). Idempotent —
// already-paused Schedules are a no-op.
//
// We do NOT touch the snapshot half. Its paused field still reflects the
// user's policy toggle (active vs paused-by-user).
func migrateExportHalvesToPaused(ctx context.Context, runtimeCli client.Client) error {
	var schedules velerov1.ScheduleList
	if err := runtimeCli.List(ctx, &schedules,
		client.InNamespace(veleroNamespace),
		client.MatchingLabels{labelPolicyRole: roleExport},
	); err != nil {
		return fmt.Errorf("list export schedules: %w", err)
	}
	migrated := 0
	for i := range schedules.Items {
		s := &schedules.Items[i]
		if s.Spec.Paused {
			continue
		}
		patch := []byte(`{"spec":{"paused":true}}`)
		if err := runtimeCli.Patch(ctx, s, client.RawPatch(types.MergePatchType, patch)); err != nil {
			log.Printf("[policypair] migrate %s: %v", s.Name, err)
			continue
		}
		log.Printf("[policypair] migrated export Schedule %s to paused=true", s.Name)
		migrated++
	}
	if migrated > 0 {
		log.Printf("[policypair] startup migration: %d export-half Schedule(s) paused", migrated)
	}
	return nil
}
