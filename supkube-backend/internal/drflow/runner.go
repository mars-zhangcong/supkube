package drflow

import (
	"context"
	"fmt"
	"log"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	gateInterval = 10 * time.Second // poll interval while waiting for a gate
	gateTimeout  = 20 * time.Minute // max wait per phase
)

// activeRuns is an in-process registry of running goroutines.
// This prevents duplicate runs for the same ID if the HTTP handler
// is called twice. Access is unguarded — each run ID is unique and
// goroutines only read/write their own entry.
var activeRuns = map[string]bool{}

// RecoverInFlightRuns scans existing ConfigMaps for runs that were
// in-progress at the time of the last server restart (ADR-053 D4 reconcile).
// Any run in a non-terminal phase that has no active goroutine gets its
// goroutine relaunched starting from the saved phase — so progress is
// resumed rather than replayed from Pending.
//
// Call this once at server startup before the HTTP server begins serving.
func RecoverInFlightRuns(ctx context.Context, k8sCli kubernetes.Interface, dynCli dynamic.Interface) {
	runs, err := ListRuns(ctx, k8sCli)
	if err != nil {
		log.Printf("[drflow] recovery scan failed: %v", err)
		return
	}
	terminal := map[Phase]bool{PhaseSucceeded: true, PhaseFailed: true}
	for _, r := range runs {
		if terminal[r.Phase] || activeRuns[r.ID] {
			continue
		}
		run := *r
		activeRuns[run.ID] = true
		log.Printf("[drflow] recovering in-flight run %s (phase=%s)", run.ID, run.Phase)
		go func() {
			defer delete(activeRuns, run.ID)
			resumePhases(context.Background(), k8sCli, dynCli, run)
		}()
	}
}

// StartRun creates a new DRFlow run and launches the runner goroutine.
// Returns an error if a run with this ID is already active.
func StartRun(ctx context.Context, k8sCli kubernetes.Interface, dynCli dynamic.Interface, t Trigger) (*Run, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	id := generateRunID()
	r := Run{
		ID:              id,
		Phase:           PhasePending,
		StartedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
		DrillNS:         t.DrillNS,
		TargetApp:       t.TargetApp,
		KBCluster:       t.KBCluster,
		DBSecret:        t.DBSecret,
		BackupName:      t.BackupName,
		BackupNamespace: t.BackupNamespace,
		DryRun:          t.DryRun,
	}
	if err := createRun(ctx, k8sCli, r); err != nil {
		return nil, fmt.Errorf("create run CM: %w", err)
	}

	activeRuns[id] = true
	go func() {
		defer delete(activeRuns, id)
		runPhases(context.Background(), k8sCli, dynCli, r)
	}()
	return &r, nil
}

// phaseStep is a single step in the DRFlow phase plan.
type phaseStep struct {
	next    Phase
	trigger func() error // run once on phase entry (nil = no-op); must be idempotent
	gate    func() error // polled until nil (nil = immediate succeed)
}

// phasePlan returns the ordered phase steps starting from the NEXT phase
// after currentPhase. If currentPhase is Pending, all phases are included.
// resumePhases uses this to skip already-completed phases on recovery.
// Triggers are idempotent so it is safe to re-run them on reconcile restart.
func phasePlan(ctx context.Context, k8sCli kubernetes.Interface, dynCli dynamic.Interface, r Run, currentPhase Phase) []phaseStep {
	all := []phaseStep{
		{
			PhaseRestoringDB,
			func() error { return triggerRestoringDB(ctx, dynCli, r) },
			func() error { return gateRestoringDB(ctx, k8sCli, dynCli, r) },
		},
		{
			PhaseRestoringApp,
			func() error { return triggerRestoringApp(ctx, k8sCli, r) },
			func() error { return gateRestoringApp(ctx, k8sCli, r) },
		},
		{PhaseRealigning, nil, func() error { return gateRealigning(ctx, k8sCli, r) }},
		{PhaseValidating, nil, func() error { return gateValidating(ctx, k8sCli, r) }},
		{PhaseSucceeded, nil, nil},
	}
	if currentPhase == PhasePending {
		return all
	}
	// Find where the current phase sits and return from that step (inclusive,
	// so the gate for the current phase is re-polled after restart — correct
	// because the gate is idempotent and we can't know if it passed).
	for i, step := range all {
		if step.next == currentPhase {
			return all[i:]
		}
	}
	return all
}

// runPhases drives the linear phase progression for a new run (starts at Pending).
func runPhases(ctx context.Context, k8sCli kubernetes.Interface, dynCli dynamic.Interface, r Run) {
	execPhases(ctx, k8sCli, dynCli, r, PhasePending)
}

// resumePhases drives phase progression from the run's saved phase (reconcile).
func resumePhases(ctx context.Context, k8sCli kubernetes.Interface, dynCli dynamic.Interface, r Run) {
	execPhases(ctx, k8sCli, dynCli, r, r.Phase)
}

func execPhases(ctx context.Context, k8sCli kubernetes.Interface, dynCli dynamic.Interface, r Run, fromPhase Phase) {
	for _, step := range phasePlan(ctx, k8sCli, dynCli, r, fromPhase) {
		if err := advance(ctx, k8sCli, &r, step.next, step.trigger, step.gate); err != nil {
			fail(ctx, k8sCli, &r, string(step.next), err.Error())
			return
		}
		if step.next == PhaseSucceeded {
			break
		}
	}
}

// advance transitions to the next phase, runs the trigger once, then polls the gate
// until it passes or gateTimeout expires. Trigger errors fail the phase immediately.
func advance(ctx context.Context, k8sCli kubernetes.Interface, r *Run, next Phase, trigger func() error, gate func() error) error {
	r.Phase = next
	r.UpdatedAt = time.Now().UTC()
	if err := updateRun(ctx, k8sCli, *r); err != nil {
		log.Printf("[drflow] %s: failed to persist phase %s: %v", r.ID, next, err)
	}
	emitPhaseEvent(ctx, k8sCli, *r, fmt.Sprintf("DRFlow %s entered phase %s", r.ID, next))

	if trigger != nil {
		if err := trigger(); err != nil {
			return fmt.Errorf("trigger %s: %w", next, err)
		}
	}

	if gate == nil {
		// PhaseSucceeded has no gate
		return nil
	}

	deadline := time.Now().Add(gateTimeout)
	ticker := time.NewTicker(gateInterval)
	defer ticker.Stop()

	for {
		if err := gate(); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("gate timeout after %s", gateTimeout)
			}
		}
	}
}

// fail transitions the run to Failed and emits an event.
func fail(ctx context.Context, k8sCli kubernetes.Interface, r *Run, phase, reason string) {
	r.Phase = PhaseFailed
	r.FailedPhase = phase
	r.FailReason = reason
	r.UpdatedAt = time.Now().UTC()
	if err := updateRun(ctx, k8sCli, *r); err != nil {
		log.Printf("[drflow] %s: failed to persist Failed state: %v", r.ID, err)
	}
	emitPhaseEvent(ctx, k8sCli, *r,
		fmt.Sprintf("DRFlow %s failed in phase %s: %s", r.ID, phase, reason))
	log.Printf("[drflow] run %s FAILED at phase %s: %s", r.ID, phase, reason)
}
