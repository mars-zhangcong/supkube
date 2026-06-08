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

// StartRun creates a new DRFlow run and launches the runner goroutine.
// Returns an error if a run with this ID is already active.
func StartRun(ctx context.Context, k8sCli kubernetes.Interface, dynCli dynamic.Interface, t Trigger) (*Run, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	id := generateRunID()
	r := Run{
		ID:        id,
		Phase:     PhasePending,
		StartedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		DrillNS:   t.DrillNS,
		TargetApp: t.TargetApp,
		KBCluster: t.KBCluster,
		DBSecret:  t.DBSecret,
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

// runPhases drives the linear phase progression for one run.
// Each phase polls its gate function every gateInterval until the gate
// passes or gateTimeout is exceeded, then advances to the next phase.
func runPhases(ctx context.Context, k8sCli kubernetes.Interface, dynCli dynamic.Interface, r Run) {
	phases := []struct {
		next Phase
		gate func() error
	}{
		{PhaseRestoringDB, func() error { return gateRestoringDB(ctx, k8sCli, dynCli, r) }},
		{PhaseRestoringApp, func() error { return gateRestoringApp(ctx, k8sCli, r) }},
		{PhaseRealigning, func() error { return gateRealigning(ctx, r) }},
		{PhaseValidating, func() error { return gateValidating(ctx, k8sCli, r) }},
		{PhaseSucceeded, nil},
	}

	for _, step := range phases {
		if err := advance(ctx, k8sCli, &r, step.next, step.gate); err != nil {
			fail(ctx, k8sCli, &r, string(step.next), err.Error())
			return
		}
		if step.next == PhaseSucceeded {
			break
		}
	}
}

// advance transitions to the next phase and polls the gate until it passes
// or gateTimeout expires.
func advance(ctx context.Context, k8sCli kubernetes.Interface, r *Run, next Phase, gate func() error) error {
	r.Phase = next
	r.UpdatedAt = time.Now().UTC()
	if err := updateRun(ctx, k8sCli, *r); err != nil {
		log.Printf("[drflow] %s: failed to persist phase %s: %v", r.ID, next, err)
	}
	emitPhaseEvent(ctx, k8sCli, *r, fmt.Sprintf("DRFlow %s entered phase %s", r.ID, next))

	if gate == nil {
		// Succeeded has no gate
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
