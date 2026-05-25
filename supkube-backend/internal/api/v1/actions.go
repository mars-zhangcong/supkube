// Package v1: actions.go
//
// The Action abstraction (v0.8.0)
// ───────────────────────────────
// SupKube prior to v0.8 had separate pages per Velero CRD: Restore Points
// (Backup CR), Restores (Restore CR), Policies (Schedule CR). That's
// implementation-oriented — users had to learn Velero's CRD taxonomy to
// find "what happened recently". Kasten solves this by unifying everything
// into an Action stream: PolicyRun / Backup / Restore / Export are all
// instances of the same shape, distinguished by Type.
//
// This file lifts SupKube to that model:
//   - Backend exposes /api/v1/actions returning unified Action records
//   - Each Action carries Phases (Kasten-style ordered ✓ checklist
//     derived from Velero status fields)
//   - Each Action carries the relevant refs (Policy, Protected Object,
//     Restore Point, Target Namespace) so the UI can render context-rich
//     cards without per-row extra fetches
//
// Why we don't synthesize a separate "PolicyRun" Action wrapping a Backup:
// Velero doesn't have a PolicyRun CRD. A Schedule produces Backup CRs
// directly. We surface the Backup with a Policy ref instead, which matches
// reality. (Kasten synthesizes a PolicyRun because their internal model
// has explicit policy-execution objects; we don't.)
package v1

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/k8s"
)

// ─── Action types ───────────────────────────────────────────────────────
// Stable string values — UI maps them to localized labels via i18n keys
// (action.type.<value>). Never change a value once shipped; only ever add
// new ones, so old data continues to render correctly.
const (
	ActionTypeBackup  = "Backup"
	ActionTypeRestore = "Restore"
	// v0.9 (Hub-Spoke): Export = "Export" — when a Backup is shipped to a
	// remote BSL/cluster as its own discrete Action. Velero today fuses
	// Snapshot+Upload into one Backup; we'll split it visually in v0.9.
)

// Action status — also Kasten-aligned. Maps from Velero phase strings.
const (
	ActionStatusRunning   = "running"
	ActionStatusCompleted = "completed"
	ActionStatusFailed    = "failed"
	ActionStatusPartial   = "partial" // PartiallyFailed / FinalizingPartiallyFailed
	ActionStatusSkipped   = "skipped"
	ActionStatusUnknown   = "unknown"
)

// Phase status — for each row in the phase checklist.
const (
	PhaseStatusPending   = "pending"   // not started
	PhaseStatusRunning   = "running"   // currently executing (animated spinner)
	PhaseStatusCompleted = "completed" // ✓
	PhaseStatusFailed    = "failed"    // ✗
	PhaseStatusSkipped   = "skipped"
)

// ─── Data shapes ────────────────────────────────────────────────────────

type Ref struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type Phase struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	// DetailLines lets us nest sub-bullets without making a recursive tree.
	// Used e.g. for "Snapshotting Application Components" → list of workloads.
	DetailLines []string `json:"detailLines,omitempty"`
}

// ArtifactStats aggregates resource counts captured by this Action.
// Total = sum of all resources, Counts breaks it down by kind so the UI
// can show "14 specs, 2 PVCs" style chips.
type ArtifactStats struct {
	Total  int            `json:"total"`
	Counts map[string]int `json:"counts,omitempty"`
}

// Action is the unified shape consumed by the Activity page. We deliberately
// keep this flat (no nested polymorphism) so JSON encoding is trivial and
// the Vue side can render with v-if on type without TypeScript discriminated
// unions.
type Action struct {
	ID              string         `json:"id"`              // CR name (unique within type+namespace)
	Type            string         `json:"type"`            // Backup / Restore / Export
	Status          string         `json:"status"`          // running / completed / failed / partial / skipped
	Phases          []Phase        `json:"phases"`
	StartTime       *time.Time     `json:"startTime,omitempty"`
	EndTime         *time.Time     `json:"endTime,omitempty"`
	DurationSeconds int            `json:"durationSeconds,omitempty"`

	// Related entities — only the fields that apply to the Type are populated.
	Policy          *Ref `json:"policy,omitempty"`          // Backup → Schedule, if scheduled
	ProtectedObject *Ref `json:"protectedObject,omitempty"` // Backup → namespace
	RestorePoint    *Ref `json:"restorePoint,omitempty"`    // Restore → source Backup
	TargetNamespace string `json:"targetNamespace,omitempty"` // Restore → mapping target

	// Counters from .status.
	Artifacts *ArtifactStats `json:"artifacts,omitempty"`
	Errors    int            `json:"errors"`
	Warnings  int            `json:"warnings"`

	// Raw CR pointers — let the drawer fetch full YAML on demand.
	RawKind string `json:"rawKind"` // "Backup" | "Restore"
	RawName string `json:"rawName"`

	// Source surface: who originated this Action.
	//   manual    — user clicked Create in SupKube UI
	//   policy    — Velero Schedule fired it
	//   imported  — synced from another cluster's BSL
	Origin string `json:"origin"`

	// v0.8.10 Plan-B fields. A dual policy produces two Backups (snapshot
	// half + export half) — these two annotations let the UI render a
	// "Paired with" cross-link and group both halves under a single
	// "Policy Run Instant" timestamp.
	//
	// PairedAction: peer Backup's Name (snapshot ↔ export). Set on both
	// halves once the policypair controller fires the export half.
	// Frontend Action Details drawer renders this as a clickable ref.
	//
	// PolicyRunInstant: unified RFC3339 timestamp that both halves share.
	// Equals the snapshot half's creationTimestamp. Frontend uses this
	// instead of per-Backup creationTimestamp when rendering "this is
	// one policy run, captured at one instant" semantics.
	PairedAction     *Ref       `json:"pairedAction,omitempty"`
	PolicyRunInstant *time.Time `json:"policyRunInstant,omitempty"`

	// Role: "local" / "cloud" / "" (none). v0.8.12 LBS2 renamed from
	// "snapshot" / "export" — the BSL-pair model replaces the broken
	// VSC-as-snapshot concept (see ADR-029). Internal annotations on
	// existing Backup CRs still say "snapshot"/"export" — we translate
	// at this API boundary so the frontend only knows the new names and
	// old data stays valid (no risky kubectl-relabel migration needed).
	//
	// Mirrors the supkube.io/policy-role Schedule label that the parent
	// Schedule
	// carried. Lets the UI distinguish "this is the snapshot half" vs
	// "this is the export half" without re-deriving from spec fields.
	Role string `json:"role,omitempty"`
}

// ActionStats powers the KPI strip at the top of the Activity page.
// Numbers cover the *filtered* result set so the strip is consistent with
// what the user sees scrolling.
type ActionStats struct {
	Total            int `json:"total"`
	Running          int `json:"running"`
	Completed        int `json:"completed"`
	Failed           int `json:"failed"`
	Partial          int `json:"partial"`
	Skipped          int `json:"skipped"`
	AvgDurationSec   int `json:"avgDurationSec"`
	LiveArtifacts    int `json:"liveArtifacts"`
	RetiredArtifacts int `json:"retiredArtifacts"` // future: deleted-but-still-tracked
}

// DurationBucket is one bar on the Activity page's duration chart. We emit
// one bucket per Action with a known duration; the UI bucketizes by time
// for the X axis.
type DurationBucket struct {
	Time            time.Time `json:"time"`
	DurationSeconds int       `json:"durationSeconds"`
	Status          string    `json:"status"`
	Type            string    `json:"type"`
	ID              string    `json:"id"`
}

type ActionsResponse struct {
	Items           []Action         `json:"items"`
	Stats           ActionStats      `json:"stats"`
	DurationBuckets []DurationBucket `json:"durationBuckets"`
}

// ─── HTTP handler ───────────────────────────────────────────────────────

// ListActions returns the unified Action stream + KPI stats + duration
// buckets. Query params:
//
//	?type=Backup|Restore (default: all)
//	?status=running|completed|failed|partial (default: all)
//	?limit=N (default 200)
//
// The list is sorted by StartTime descending so the newest Action is at the
// top — the user's most common question is "what just happened?".
func ListActions(c *gin.Context) {
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	typeFilter := strings.TrimSpace(c.DefaultQuery("type", ""))
	statusFilter := strings.TrimSpace(c.DefaultQuery("status", ""))

	var actions []Action

	// 1. Backups → Action
	if typeFilter == "" || typeFilter == ActionTypeBackup {
		backupList := &velerov1.BackupList{}
		if err := cl.List(context.Background(), backupList, client.InNamespace("velero")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "list backups: " + err.Error()})
			return
		}
		for i := range backupList.Items {
			actions = append(actions, backupToAction(&backupList.Items[i]))
		}
	}

	// 2. Restores → Action
	if typeFilter == "" || typeFilter == ActionTypeRestore {
		restoreList := &velerov1.RestoreList{}
		if err := cl.List(context.Background(), restoreList, client.InNamespace("velero")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "list restores: " + err.Error()})
			return
		}
		for i := range restoreList.Items {
			actions = append(actions, restoreToAction(&restoreList.Items[i]))
		}
	}

	// Status filter.
	if statusFilter != "" {
		filtered := actions[:0]
		for _, a := range actions {
			if a.Status == statusFilter {
				filtered = append(filtered, a)
			}
		}
		actions = filtered
	}

	// Sort newest first; nil StartTime sorts last.
	sort.SliceStable(actions, func(i, j int) bool {
		ai, aj := actions[i].StartTime, actions[j].StartTime
		switch {
		case ai == nil && aj == nil:
			return actions[i].ID < actions[j].ID
		case ai == nil:
			return false
		case aj == nil:
			return true
		default:
			return ai.After(*aj)
		}
	})

	// Stats over the (filtered) result set.
	stats := computeActionStats(actions)

	// Duration buckets: only Actions that have a known duration (i.e.
	// terminal status with EndTime, OR running with StartTime). We keep
	// at most 30 recent buckets so the chart isn't crowded.
	buckets := actionsToDurationBuckets(actions, 30)

	c.JSON(http.StatusOK, ActionsResponse{
		Items:           actions,
		Stats:           stats,
		DurationBuckets: buckets,
	})
}

// GetAction returns full details for a single Action by id+type. Used by
// the detail drawer for "View Action Details" — we don't bloat ListActions
// with this because the cards only need a summary.
func GetAction(c *gin.Context) {
	id := c.Param("id")
	actionType := c.Query("type")
	if id == "" || actionType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id and type are required"})
		return
	}

	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	switch actionType {
	case ActionTypeBackup:
		b := &velerov1.Backup{}
		if err := cl.Get(context.Background(), client.ObjectKey{Name: id, Namespace: "velero"}, b); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"action": backupToAction(b),
			"raw":    b,
		})
	case ActionTypeRestore:
		r := &velerov1.Restore{}
		if err := cl.Get(context.Background(), client.ObjectKey{Name: id, Namespace: "velero"}, r); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"action": restoreToAction(r),
			"raw":    r,
		})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown action type: " + actionType})
	}
}

// ─── Backup CR → Action ─────────────────────────────────────────────────

func backupToAction(b *velerov1.Backup) Action {
	a := Action{
		ID:      b.Name,
		Type:    ActionTypeBackup,
		Status:  mapVeleroPhaseToStatus(string(b.Status.Phase)),
		Phases:  derivePhasesForBackup(b),
		Errors:  b.Status.Errors,
		Warnings: b.Status.Warnings,
		RawKind: "Backup",
		RawName: b.Name,
		Origin:  detectBackupOrigin(b),
	}

	// Timing — Velero populates StartTimestamp once the controller picks
	// up the Backup, and CompletionTimestamp when it reaches a terminal
	// phase. We expose duration only when both are present (or running).
	//
	// v0.8.3 fix: Actions that fail validation never get StartTimestamp
	// populated. Fall back to metadata.creationTimestamp so the list
	// sorts them next to other recent actions instead of dumping them at
	// the bottom (or worse, treating them as 1970-01-01).
	if b.Status.StartTimestamp != nil {
		t := b.Status.StartTimestamp.Time
		a.StartTime = &t
	} else if !b.CreationTimestamp.IsZero() {
		t := b.CreationTimestamp.Time
		a.StartTime = &t
	}
	if b.Status.CompletionTimestamp != nil {
		t := b.Status.CompletionTimestamp.Time
		a.EndTime = &t
	}
	a.DurationSeconds = computeDuration(a.StartTime, a.EndTime)
	// Suppress duration display when StartTime came from creationTimestamp
	// fallback (the Backup never actually ran, so any duration we compute
	// is the time-since-creation, which is misleading).
	if b.Status.StartTimestamp == nil {
		a.DurationSeconds = 0
	}

	// Refs.
	if b.Labels != nil {
		if schedule := b.Labels["velero.io/schedule-name"]; schedule != "" {
			a.Policy = &Ref{Kind: "Schedule", Name: schedule}
		}
	}
	if len(b.Spec.IncludedNamespaces) > 0 {
		a.ProtectedObject = &Ref{Kind: "Namespace", Name: strings.Join(b.Spec.IncludedNamespaces, ", ")}
	}

	// Artifact stats from progress.
	if b.Status.Progress != nil {
		a.Artifacts = &ArtifactStats{Total: int(b.Status.Progress.ItemsBackedUp)}
	}

	// v0.8.10 Plan-B paired-with + policy-run-instant surfacing.
	// Annotations are stamped by internal/policypair when the export half
	// is fired (both halves carry mirrored values). UI uses them to:
	//   - render a "Paired with" link in the Action Details drawer
	//   - render a unified "Policy Run At" timestamp grouping the pair
	// Absence is fine — most non-policy Actions never had a peer.
	if b.Annotations != nil {
		if peer := b.Annotations["supkube.io/dual-rp-paired"]; peer != "" {
			a.PairedAction = &Ref{Kind: "Backup", Name: peer}
		}
		if ts := b.Annotations["supkube.io/policy-run-instant"]; ts != "" {
			if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
				a.PolicyRunInstant = &parsed
			}
		}
	}
	if b.Labels != nil {
		// Role mirrors the parent Schedule's supkube.io/policy-role label.
		// Velero v1.15+ copies Schedule labels onto spawned Backups; for
		// older Velero / manually-created Backups the label may be missing
		// → leave Role empty.
		//
		// v0.8.12 LBS2: translate internal annotation values
		// (snapshot|export) to user-facing names (local|cloud) at this
		// API boundary. Also accept native (local|cloud) so the eventual
		// annotation migration is a no-op for this layer.
		switch r := b.Labels["supkube.io/policy-role"]; r {
		case "snapshot", "local":
			a.Role = "local"
		case "export", "cloud":
			a.Role = "cloud"
		}
	}

	return a
}

func detectBackupOrigin(b *velerov1.Backup) string {
	if b.Labels != nil && b.Labels["velero.io/schedule-name"] != "" {
		return "policy"
	}
	// Imported detection happens client-side via cluster fingerprint
	// (see Backups.vue setLocalFingerprint). We don't have the baseline
	// here, so default to "manual"; UI can override based on fingerprint
	// when rendering.
	return "manual"
}

// derivePhasesForBackup turns Velero's flat phase string into Kasten-style
// progressive checklist. The mapping is intentionally lossy — Velero has
// many internal phases (`WaitingForPluginOperations`, `Uploading`, etc.)
// that we collapse into ~5 user-visible steps:
//
//	1. Validating
//	2. Snapshotting Application Components
//	3. Uploading to BSL
//	4. Finalizing
//	5. Complete (terminal)
//
// Each step gets one of: pending (not yet) / running (now) / completed (✓)
// / failed (✗) based on where the CR sits.
func derivePhasesForBackup(b *velerov1.Backup) []Phase {
	phase := string(b.Status.Phase)
	progress := b.Status.Progress

	// Pre-build the 4 progress phases; we'll mutate statuses below.
	phases := []Phase{
		{Name: "Validating", Status: PhaseStatusPending},
		{Name: "Snapshotting Application Components", Status: PhaseStatusPending},
		{Name: "Uploading to Storage Profile", Status: PhaseStatusPending},
		{Name: "Finalizing", Status: PhaseStatusPending},
	}

	switch phase {
	case "", "New":
		phases[0].Status = PhaseStatusRunning
	case "FailedValidation":
		phases[0].Status = PhaseStatusFailed
	case "InProgress":
		phases[0].Status = PhaseStatusCompleted
		phases[1].Status = PhaseStatusRunning
		if progress != nil && progress.TotalItems > 0 {
			phases[1].DetailLines = []string{
				fmt.Sprintf("%d / %d items processed", progress.ItemsBackedUp, progress.TotalItems),
			}
		}
	case "WaitingForPluginOperations":
		phases[0].Status = PhaseStatusCompleted
		phases[1].Status = PhaseStatusCompleted
		phases[2].Status = PhaseStatusRunning
		phases[2].DetailLines = []string{"Waiting for CSI snapshot / async operations to finish"}
	case "WaitingForPluginOperationsPartiallyFailed":
		phases[0].Status = PhaseStatusCompleted
		phases[1].Status = PhaseStatusCompleted
		phases[2].Status = PhaseStatusRunning
		phases[2].DetailLines = []string{"Some plugin operations failed but others continue"}
	case "Finalizing":
		phases[0].Status = PhaseStatusCompleted
		phases[1].Status = PhaseStatusCompleted
		phases[2].Status = PhaseStatusCompleted
		phases[3].Status = PhaseStatusRunning
	case "FinalizingPartiallyFailed":
		phases[0].Status = PhaseStatusCompleted
		phases[1].Status = PhaseStatusCompleted
		phases[2].Status = PhaseStatusCompleted
		phases[3].Status = PhaseStatusRunning
		phases[3].DetailLines = []string{"Finalizing — partial failures detected"}
	case "Completed":
		for i := range phases {
			phases[i].Status = PhaseStatusCompleted
		}
		phases = append(phases, Phase{Name: "All phases completed successfully.", Status: PhaseStatusCompleted})
	case "PartiallyFailed":
		for i := range phases {
			phases[i].Status = PhaseStatusCompleted
		}
		phases = append(phases, Phase{Name: fmt.Sprintf("Completed with %d error(s), %d warning(s)", b.Status.Errors, b.Status.Warnings), Status: PhaseStatusFailed})
	case "Failed":
		// Mark up to the first non-completed phase as failed.
		failed := false
		for i := range phases {
			if !failed && phases[i].Status != PhaseStatusCompleted {
				phases[i].Status = PhaseStatusFailed
				failed = true
			}
		}
		if !failed {
			phases = append(phases, Phase{Name: "Failed", Status: PhaseStatusFailed})
		}
		if b.Status.FailureReason != "" {
			phases[len(phases)-1].DetailLines = []string{b.Status.FailureReason}
		}
	case "Deleting":
		phases = []Phase{{Name: "Deleting Backup", Status: PhaseStatusRunning}}
	}

	return phases
}

// ─── Restore CR → Action ────────────────────────────────────────────────

func restoreToAction(r *velerov1.Restore) Action {
	a := Action{
		ID:      r.Name,
		Type:    ActionTypeRestore,
		Status:  mapVeleroPhaseToStatus(string(r.Status.Phase)),
		Phases:  derivePhasesForRestore(r),
		Errors:  r.Status.Errors,
		Warnings: r.Status.Warnings,
		RawKind: "Restore",
		RawName: r.Name,
		Origin:  "manual",
	}
	// v0.8.3: same StartTime fallback as backupToAction — failed-validation
	// Restores never get StartTimestamp.
	if r.Status.StartTimestamp != nil {
		t := r.Status.StartTimestamp.Time
		a.StartTime = &t
	} else if !r.CreationTimestamp.IsZero() {
		t := r.CreationTimestamp.Time
		a.StartTime = &t
	}
	if r.Status.CompletionTimestamp != nil {
		t := r.Status.CompletionTimestamp.Time
		a.EndTime = &t
	}
	a.DurationSeconds = computeDuration(a.StartTime, a.EndTime)
	if r.Status.StartTimestamp == nil {
		a.DurationSeconds = 0
	}

	if r.Spec.BackupName != "" {
		a.RestorePoint = &Ref{Kind: "Backup", Name: r.Spec.BackupName}
	}
	// Target namespace = first value of NamespaceMapping, else first
	// IncludedNamespace, else "(unspecified)".
	if len(r.Spec.NamespaceMapping) > 0 {
		for _, v := range r.Spec.NamespaceMapping {
			a.TargetNamespace = v
			break
		}
	}
	if a.TargetNamespace == "" && len(r.Spec.IncludedNamespaces) > 0 {
		a.TargetNamespace = r.Spec.IncludedNamespaces[0]
	}

	if r.Status.Progress != nil {
		a.Artifacts = &ArtifactStats{Total: int(r.Status.Progress.ItemsRestored)}
	}

	return a
}

func derivePhasesForRestore(r *velerov1.Restore) []Phase {
	phase := string(r.Status.Phase)
	progress := r.Status.Progress

	phases := []Phase{
		{Name: "Validating", Status: PhaseStatusPending},
		{Name: "Restoring Application Components", Status: PhaseStatusPending},
		{Name: "Finalizing", Status: PhaseStatusPending},
	}

	switch phase {
	case "", "New":
		phases[0].Status = PhaseStatusRunning
	case "FailedValidation":
		phases[0].Status = PhaseStatusFailed
		if len(r.Status.ValidationErrors) > 0 {
			phases[0].DetailLines = r.Status.ValidationErrors
		}
	case "InProgress":
		phases[0].Status = PhaseStatusCompleted
		phases[1].Status = PhaseStatusRunning
		if progress != nil && progress.TotalItems > 0 {
			phases[1].DetailLines = []string{
				fmt.Sprintf("%d / %d items restored", progress.ItemsRestored, progress.TotalItems),
			}
		}
	case "WaitingForPluginOperations":
		phases[0].Status = PhaseStatusCompleted
		phases[1].Status = PhaseStatusRunning
		phases[1].DetailLines = []string{"Waiting for plugin operations (volume restores)"}
	case "WaitingForPluginOperationsPartiallyFailed":
		phases[0].Status = PhaseStatusCompleted
		phases[1].Status = PhaseStatusRunning
		phases[1].DetailLines = []string{"Some plugin operations failed but others continue"}
	case "Finalizing":
		phases[0].Status = PhaseStatusCompleted
		phases[1].Status = PhaseStatusCompleted
		phases[2].Status = PhaseStatusRunning
	case "FinalizingPartiallyFailed":
		phases[0].Status = PhaseStatusCompleted
		phases[1].Status = PhaseStatusCompleted
		phases[2].Status = PhaseStatusRunning
		phases[2].DetailLines = []string{"Finalizing — partial failures detected"}
	case "Completed":
		for i := range phases {
			phases[i].Status = PhaseStatusCompleted
		}
		phases = append(phases, Phase{Name: "All phases completed successfully.", Status: PhaseStatusCompleted})
	case "PartiallyFailed":
		for i := range phases {
			phases[i].Status = PhaseStatusCompleted
		}
		phases = append(phases, Phase{Name: fmt.Sprintf("Completed with %d error(s), %d warning(s)", r.Status.Errors, r.Status.Warnings), Status: PhaseStatusFailed})
	case "Failed":
		failed := false
		for i := range phases {
			if !failed && phases[i].Status != PhaseStatusCompleted {
				phases[i].Status = PhaseStatusFailed
				failed = true
			}
		}
		if !failed {
			phases = append(phases, Phase{Name: "Failed", Status: PhaseStatusFailed})
		}
		if r.Status.FailureReason != "" {
			phases[len(phases)-1].DetailLines = []string{r.Status.FailureReason}
		}
	}

	return phases
}

// ─── Helpers ────────────────────────────────────────────────────────────

func mapVeleroPhaseToStatus(phase string) string {
	switch phase {
	case "Completed":
		return ActionStatusCompleted
	case "Failed", "FailedValidation":
		return ActionStatusFailed
	case "PartiallyFailed", "FinalizingPartiallyFailed", "WaitingForPluginOperationsPartiallyFailed":
		return ActionStatusPartial
	case "New", "InProgress", "Uploading", "UploadingPartialFailure", "Finalizing",
		"WaitingForPluginOperations", "Deleting":
		return ActionStatusRunning
	case "":
		// v0.8.9.1: empty phase = Backup CR was just created, Velero
		// controller hasn't started processing yet. Almost always
		// transient (<1 sec). Pre-v0.8.9.1 we showed "UNKNOWN" here,
		// which made the Activity page render gray UNKNOWN badge for
		// the 1-second window between "Run Once" click and Velero's
		// first reconcile. Treat empty as running — same semantics
		// as "New" (which is the next phase anyway).
		//
		// Edge case: Velero deployment entirely down → backup stuck
		// in empty phase forever, shows as "running" indefinitely.
		// That's a Velero-side problem; SupKube's verify-cluster
		// script catches it separately. Better than showing UNKNOWN
		// for the 99% of cases where Velero is fine.
		return ActionStatusRunning
	default:
		return ActionStatusUnknown
	}
}

func computeDuration(start, end *time.Time) int {
	if start == nil {
		return 0
	}
	endT := time.Now()
	if end != nil {
		endT = *end
	}
	d := endT.Sub(*start)
	if d < 0 {
		return 0
	}
	return int(d.Seconds())
}

func computeActionStats(actions []Action) ActionStats {
	stats := ActionStats{Total: len(actions)}
	var durSum int
	var durCount int
	for _, a := range actions {
		switch a.Status {
		case ActionStatusRunning:
			stats.Running++
		case ActionStatusCompleted:
			stats.Completed++
		case ActionStatusFailed:
			stats.Failed++
		case ActionStatusPartial:
			stats.Partial++
		case ActionStatusSkipped:
			stats.Skipped++
		}
		if a.DurationSeconds > 0 && a.Status != ActionStatusRunning {
			durSum += a.DurationSeconds
			durCount++
		}
		if a.Artifacts != nil {
			stats.LiveArtifacts += a.Artifacts.Total
		}
	}
	if durCount > 0 {
		stats.AvgDurationSec = durSum / durCount
	}
	return stats
}

func actionsToDurationBuckets(actions []Action, max int) []DurationBucket {
	out := make([]DurationBucket, 0, max)
	for _, a := range actions {
		if a.StartTime == nil || a.DurationSeconds <= 0 {
			continue
		}
		out = append(out, DurationBucket{
			Time:            *a.StartTime,
			DurationSeconds: a.DurationSeconds,
			Status:          a.Status,
			Type:            a.Type,
			ID:              a.ID,
		})
		if len(out) >= max {
			break
		}
	}
	// Buckets need to be chronological for the chart (oldest left, newest
	// right); ListActions sorted descending so reverse.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// nameForLabel is kept for future use when we render workload-level phase
// detail lines from itemOperations. Defined here so future callers don't
// have to thread a metav1 import.
var _ = metav1.ObjectMeta{}
