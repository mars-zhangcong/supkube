package eventwatch

import (
	"testing"

	"github.com/supkube/supkube-backend/internal/events"
)

func types(es []events.Event) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Type+":"+e.Subject)
	}
	return out
}

func TestDiff_SeedNoEmit(t *testing.T) {
	last := map[string]string{}
	// first pass (seeded=false): even terminal items must NOT emit
	got := diffPhases(last, false, []phaseItem{
		{key: "backup/velero/b1", subject: "b1", phase: "Completed", terminalType: events.TypeBackupCompleted},
	})
	if len(got) != 0 {
		t.Fatalf("seed pass must not emit, got %v", types(got))
	}
	if last["backup/velero/b1"] != "Completed" {
		t.Fatalf("seed must record phase")
	}
}

func TestDiff_EmitOnTransitionToTerminal(t *testing.T) {
	last := map[string]string{"backup/velero/b1": "InProgress"}
	got := diffPhases(last, true, []phaseItem{
		{key: "backup/velero/b1", subject: "b1", phase: "Completed", terminalType: events.TypeBackupCompleted},
	})
	if len(got) != 1 || got[0].Type != events.TypeBackupCompleted || got[0].Subject != "b1" {
		t.Fatalf("want one backup.completed:b1, got %v", types(got))
	}
}

func TestDiff_NoEmitOnNonTerminal(t *testing.T) {
	last := map[string]string{"backup/velero/b1": "New"}
	got := diffPhases(last, true, []phaseItem{
		{key: "backup/velero/b1", subject: "b1", phase: "InProgress", terminalType: ""},
	})
	if len(got) != 0 {
		t.Fatalf("non-terminal transition must not emit, got %v", types(got))
	}
}

func TestDiff_NoDuplicateOnSamePhase(t *testing.T) {
	last := map[string]string{"restore/velero/r1": "Completed"}
	got := diffPhases(last, true, []phaseItem{
		{key: "restore/velero/r1", subject: "r1", phase: "Completed", terminalType: events.TypeRestoreCompleted},
	})
	if len(got) != 0 {
		t.Fatalf("unchanged terminal phase must not re-emit, got %v", types(got))
	}
}

func TestDiff_NewItemAlreadyTerminalEmitsOnce(t *testing.T) {
	last := map[string]string{} // seeded run, item appears already-Failed
	got := diffPhases(last, true, []phaseItem{
		{key: "restore/velero/r2", subject: "r2", phase: "Failed", terminalType: events.TypeRestoreFailed},
	})
	if len(got) != 1 || got[0].Type != events.TypeRestoreFailed {
		t.Fatalf("new terminal item should emit once, got %v", types(got))
	}
}
