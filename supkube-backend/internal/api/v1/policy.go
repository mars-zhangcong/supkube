// Package v1: Policy = aggregated view over one or two Velero Schedules.
//
// Background (v0.8.9)
// ───────────────────
// Up through v0.8.8, "Policy" in SupKube's UI was 1:1 with a Velero
// Schedule CR. That worked for L1 (snapshot-only) but didn't model L2
// (Snapshot + Export) the way users — and Kasten K10 — expect:
//
//	L2 Policy fires once  ─►  Snapshot RP (cluster-local, short TTL)
//	                       ╲
//	                        ▶  Export RP (BSL, long TTL)
//
// One Velero Backup CR carries EITHER a CSI snapshot OR a Data Mover
// upload, never both (Data Mover deletes the snapshot after upload).
// So to produce both RPs we need TWO Schedules per L2 Policy, both
// labeled with the same `supkube.io/policy-name`. Velero's controller
// fires each independently; the resulting two Backup CRs share the
// label and the (approximate) timestamp, letting the UI group them.
//
// Naming convention:
//
//	Snapshot half: <policy-name>           (legacy-compatible)
//	Export   half: <policy-name>-export    (synthetic suffix)
//
// Labels (on both Schedules and inherited by the Backups they spawn
// via Velero controller-side copy in v1.15+):
//
//	supkube.io/policy-name: <user-chosen>
//	supkube.io/policy-role: snapshot | export
//
// Why metadata.name = <policy-name> for snapshot half:
//   - Legacy single-Schedule policies in pre-v0.8.9 deployments don't
//     have labels. Treating their metadata.name AS the policy name
//     lets them keep working unchanged.
//   - The /schedules/<name> route continues to work for legacy users.
//   - The label is the source of truth for *aggregation*; the name is
//     the source of truth for *URL routing*.
package v1

import (
	"context"
	"fmt"
	"strings"
	"time"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Label keys SupKube uses to tie schedule pairs together.
const (
	labelPolicyName = "supkube.io/policy-name"
	labelPolicyRole = "supkube.io/policy-role"

	roleSnapshot = "snapshot"
	roleExport   = "export"
)

// PolicyRequest is the cross-cutting request shape used by Create and
// Patch. It includes everything either half of a paired Policy might
// want to set. Per-role overrides (snapshot retention vs export retention,
// or which BSL the export goes to) are explicit fields so callers don't
// have to guess what gets applied where.
type PolicyRequest struct {
	Name               string            `json:"name"`
	Schedule           string            `json:"schedule"`
	IncludedNamespaces []string          `json:"includedNamespaces"`
	Paused             bool              `json:"paused"`
	Annotations        map[string]string `json:"annotations"`

	// Snapshot half — applies to local CSI snapshot (the L1 half of L2).
	SnapshotTTL             string `json:"snapshotTtl,omitempty"`
	SnapshotStorageLocation string `json:"snapshotStorageLocation,omitempty"`

	// Export half — applies to the Data Mover / Filesystem upload to BSL.
	// Only consulted when Dual == true.
	ExportTTL             string `json:"exportTtl,omitempty"`
	ExportStorageLocation string `json:"exportStorageLocation,omitempty"`

	// Volume mode flags. Shared across the pair; the snapshot half
	// always uses snapshotMoveData=false, the export half always uses
	// snapshotMoveData=true. The caller picks the underlying CSI vs
	// filesystem mode via these flags applied to BOTH halves.
	SnapshotVolumes          *bool `json:"snapshotVolumes,omitempty"`
	DefaultVolumesToFsBackup *bool `json:"defaultVolumesToFsBackup,omitempty"`

	// Dual signals "L2 mode" — create both halves. False = single
	// schedule, backwards-compatible with legacy L1 policies.
	Dual bool `json:"dual"`
}

// makeTemplate builds a velerov1.BackupSpec for one half of the pair.
// `forExport` toggles snapshotMoveData and picks the right TTL + BSL.
func (r *PolicyRequest) makeTemplate(forExport bool) (velerov1.BackupSpec, error) {
	tpl := velerov1.BackupSpec{
		IncludedNamespaces:       r.IncludedNamespaces,
		SnapshotVolumes:          r.SnapshotVolumes,
		DefaultVolumesToFsBackup: r.DefaultVolumesToFsBackup,
	}
	// snapshotMoveData diverges per half. We force it explicitly rather
	// than reading from the request — the role of each half is the
	// source of truth, not whatever moveData flag the caller sent.
	moveData := forExport
	tpl.SnapshotMoveData = &moveData

	ttl := r.SnapshotTTL
	bsl := r.SnapshotStorageLocation
	if forExport {
		ttl = r.ExportTTL
		bsl = r.ExportStorageLocation
	}
	if ttl != "" {
		d, err := time.ParseDuration(ttl)
		if err != nil {
			return tpl, fmt.Errorf("invalid TTL %q: %w", ttl, err)
		}
		tpl.TTL = metav1.Duration{Duration: d}
	}
	if bsl != "" {
		tpl.StorageLocation = bsl
	}
	return tpl, nil
}

// scheduleNameForRole returns the K8s-level Schedule name for a given
// policy + role. Snapshot half keeps the unadorned name for legacy
// URL compat; export half gets a `-export` suffix.
func scheduleNameForRole(policyName, role string) string {
	if role == roleExport {
		return policyName + "-export"
	}
	return policyName
}

// buildSchedule materializes a Schedule CR for the given role.
func (r *PolicyRequest) buildSchedule(role string) (*velerov1.Schedule, error) {
	tpl, err := r.makeTemplate(role == roleExport)
	if err != nil {
		return nil, err
	}
	labels := map[string]string{
		labelPolicyName: r.Name,
		labelPolicyRole: role,
	}
	// v0.8.10 Plan-B: the export half is ALWAYS created with paused=true.
	// This hands trigger ownership to internal/policypair/, which fires
	// the export half right after the snapshot half completes (data gap
	// shrinks from ~7 min to ~30s). If Velero's Schedule controller were
	// allowed to fire the export half on its own cron tick, we'd race
	// the controller and end up with duplicate export Backups.
	//
	// The snapshot half retains the user's paused setting (so "pause
	// this policy" still works from the UI as expected).
	paused := r.Paused
	if role == roleExport {
		paused = true
	}
	return &velerov1.Schedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:        scheduleNameForRole(r.Name, role),
			Namespace:   "velero",
			Labels:      labels,
			Annotations: r.Annotations,
		},
		Spec: velerov1.ScheduleSpec{
			Schedule: r.Schedule,
			Template: tpl,
			Paused:   paused,
		},
	}, nil
}

// PolicyAggregate is the API response shape for "one logical policy".
// Mode tells the UI which sub-schedules to expect.
//
//	"single" — legacy policies or new L1; only SnapshotSchedule is set.
//	"dual"   — new L2 policies; both fields set.
type PolicyAggregate struct {
	PolicyName       string             `json:"policyName"`
	Mode             string             `json:"mode"` // "single" | "dual"
	SnapshotSchedule *velerov1.Schedule `json:"snapshotSchedule,omitempty"`
	ExportSchedule   *velerov1.Schedule `json:"exportSchedule,omitempty"`

	// v0.8.10.1: total Backups this policy has produced — sum of
	// snapshot-half AND export-half Backups (a dual policy run = 2
	// Backups = 2 Restore Points, both count). Populated by
	// enrichRestorePointCounts() during ListSchedules. The Policies
	// page UI shows this as a clickable count → /backups?policy=<name>
	// pre-filters the Restore Points list to just this policy's RPs.
	//
	// 0 means "no RPs yet" (e.g. policy created but Run Once never
	// triggered + cron hasn't fired). The UI renders 0 in a muted
	// non-clickable style.
	RestorePointCount int `json:"restorePointCount"`
}

// findPolicy resolves a user-typed policy name into one or two Schedules.
// Resolution order:
//
//  1. List by label supkube.io/policy-name=<name>. If we get back one
//     or both halves, that's the new-model policy.
//  2. Fall back to exact name match (legacy single-Schedule policies
//     created before v0.8.9 have no label).
//
// Returns ErrPolicyNotFound when nothing matches. Returns PolicyAggregate
// with mode=single when only the snapshot half exists (could be L1 or
// legacy); mode=dual when both halves found.
func findPolicy(ctx context.Context, cl client.Client, name string) (*PolicyAggregate, error) {
	// New-model lookup by label.
	list := &velerov1.ScheduleList{}
	if err := cl.List(ctx, list,
		client.InNamespace("velero"),
		client.MatchingLabels{labelPolicyName: name},
	); err != nil {
		return nil, err
	}
	agg := &PolicyAggregate{PolicyName: name, Mode: "single"}
	for i := range list.Items {
		s := &list.Items[i]
		switch s.Labels[labelPolicyRole] {
		case roleSnapshot:
			agg.SnapshotSchedule = s
		case roleExport:
			agg.ExportSchedule = s
		}
	}
	if agg.SnapshotSchedule != nil && agg.ExportSchedule != nil {
		agg.Mode = "dual"
		return agg, nil
	}
	if agg.SnapshotSchedule != nil {
		return agg, nil
	}

	// Legacy fallback: exact name match, no label.
	single := &velerov1.Schedule{}
	if err := cl.Get(ctx, client.ObjectKey{Name: name, Namespace: "velero"}, single); err != nil {
		return nil, err
	}
	return &PolicyAggregate{
		PolicyName:       name,
		Mode:             "single",
		SnapshotSchedule: single,
	}, nil
}

// aggregateSchedulesIntoPolicies groups a flat ScheduleList by policy name.
// Used by ListSchedules to collapse the storage-level pair back into one
// UI row.
//
// Schedules without our label fall through to single-mode policies
// keyed by their metadata.name — that's how legacy policies coexist
// with new dual-mode ones in the same list response.
func aggregateSchedulesIntoPolicies(schedules []velerov1.Schedule) []PolicyAggregate {
	// First pass: group by policy-name (or fall back to schedule name).
	groups := map[string]*PolicyAggregate{}
	for i := range schedules {
		s := &schedules[i]
		name := s.Labels[labelPolicyName]
		if name == "" {
			name = s.Name // legacy
		}
		g, ok := groups[name]
		if !ok {
			g = &PolicyAggregate{PolicyName: name, Mode: "single"}
			groups[name] = g
		}
		role := s.Labels[labelPolicyRole]
		switch role {
		case roleExport:
			g.ExportSchedule = s
		default: // snapshot OR legacy unlabeled
			g.SnapshotSchedule = s
		}
	}
	// Second pass: promote to "dual" when both halves present + flatten
	// the map to a slice in stable order (sorted by name).
	out := make([]PolicyAggregate, 0, len(groups))
	for _, g := range groups {
		if g.SnapshotSchedule != nil && g.ExportSchedule != nil {
			g.Mode = "dual"
		}
		out = append(out, *g)
	}
	// Stable order: sort by policy name. Caller (UI) can re-sort by
	// last-run-time or other column.
	sortAggregates(out)
	return out
}

// enrichRestorePointCounts populates the RestorePointCount field on
// every PolicyAggregate by counting Backups whose `velero.io/schedule-name`
// label matches either half (snapshot OR export) of the policy.
//
// Why one bulk Backup list vs N per-policy lookups
// ────────────────────────────────────────────────
// The naive way (per-policy List with a label filter) is N k8s API
// calls for N policies. That's fine for 3-5 policies but grows linearly.
// A single un-filtered Backup list + in-memory groupBy by schedule-name
// is O(1) API calls regardless of policy count — and Backups are
// already cached by Velero's controller, so the round-trip is cheap.
//
// Counted Backups span BOTH halves of a dual policy (a dual run = 2
// Backups = 2 RPs visible in the Restore Points table; the column on
// the Policies page mirrors that user-visible total).
//
// Caller is responsible for passing the SAME Backup list the rest of
// the request will use — but in practice we just call cl.List() here
// since the cost is negligible.
func enrichRestorePointCounts(ctx context.Context, cl client.Client, policies []PolicyAggregate) []PolicyAggregate {
	backupList := &velerov1.BackupList{}
	if err := cl.List(ctx, backupList, client.InNamespace("velero")); err != nil {
		// Non-fatal: leave RestorePointCount at zero. The UI gracefully
		// shows "0" in muted style; we don't want a transient backup
		// API hiccup to break the whole Policies page render.
		return policies
	}
	// Single pass: schedule-name → count.
	bySchedule := make(map[string]int, len(backupList.Items))
	for i := range backupList.Items {
		b := &backupList.Items[i]
		sn := b.Labels["velero.io/schedule-name"]
		if sn != "" {
			bySchedule[sn]++
		}
	}
	// Attach to each policy. Both halves contribute if they exist.
	for i := range policies {
		p := &policies[i]
		if p.SnapshotSchedule != nil {
			p.RestorePointCount += bySchedule[p.SnapshotSchedule.Name]
		}
		if p.ExportSchedule != nil {
			p.RestorePointCount += bySchedule[p.ExportSchedule.Name]
		}
	}
	return policies
}

// sortAggregates: simple lexicographic by PolicyName. Inlined to avoid
// pulling in sort.Slice on a tiny slice — for N < 1000 this is fine.
func sortAggregates(s []PolicyAggregate) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && strings.Compare(s[j-1].PolicyName, s[j].PolicyName) > 0; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
