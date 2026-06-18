package auditwatch

import (
	"context"
	"path/filepath"
	"testing"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/supkube/supkube-backend/internal/audit"
	"github.com/supkube/supkube-backend/internal/velerons"
)

func dbrEvent(task, backup string, ph audit.Phase) audit.ActivityEvent {
	return audit.ActivityEvent{
		TaskID: task, ActionType: audit.ActionDeleteBackup, Phase: ph,
		ResourceRef: "backup/" + backup,
	}
}

func TestInflightDeletes(t *testing.T) {
	events := []audit.ActivityEvent{
		dbrEvent("A", "rp-a", audit.PhaseSubmitted),
		dbrEvent("A", "rp-a", audit.PhaseDBRCreated), // A: 在飞(DBR-Created,无终态)
		dbrEvent("B", "rp-b", audit.PhaseDBRCreated),
		dbrEvent("B", "rp-b", audit.PhaseCompleted), // B: 已终态 → 排除
		dbrEvent("C", "rp-c", audit.PhaseSubmitted), // C: 无 DBR-Created → 排除(还没创 DBR)
		{TaskID: "D", ActionType: audit.ActionForceStripFinalizer, Phase: audit.PhaseDBRCreated, ResourceRef: "backup/rp-d"}, // 非 DeleteBackup → 排除
	}
	got := InflightDeletes(events)
	if len(got) != 1 || got[0].TaskID != "A" || got[0].Backup != "rp-a" {
		t.Fatalf("应只挑出在飞的 A/rp-a,got %+v", got)
	}
}

func TestRunOnce_BackupGone_EmitsTerminal(t *testing.T) {
	// 真 SQLite store 承载审计流。
	store, err := audit.OpenSQLite(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	audit.SetDefault(store)
	defer audit.SetDefault(nil)
	ctx := context.Background()

	// 两个在飞删除:rp-gone(CR 已删)、rp-live(CR 还在)。
	for _, bk := range []string{"rp-gone", "rp-live"} {
		audit.EmitDeleteTask(ctx, "task-"+bk, bk, audit.ActionDeleteBackup, audit.PhaseSubmitted, "", nil)
		audit.EmitDeleteTask(ctx, "task-"+bk, bk, audit.ActionDeleteBackup, audit.PhaseDBRCreated, "", nil)
	}

	// fake client 仅含 rp-live 的 Backup CR(rp-gone 已消失)。
	s := runtime.NewScheme()
	if err := velerov1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(&velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "rp-live", Namespace: velerons.Namespace()},
	}).Build()

	RunOnce(ctx, cl)

	// rp-gone 应补到 Completed;rp-live 仍停在 DBR-Created(CR 还在)。
	goneEvents, _ := store.List(ctx, audit.ListOpts{TaskID: "task-rp-gone"})
	if !hasPhase(goneEvents, audit.PhaseCompleted) || !hasPhase(goneEvents, audit.PhaseCRRemoved) {
		t.Fatalf("rp-gone 应补 CR-Removed+Completed,got phases %v", phases(goneEvents))
	}
	liveEvents, _ := store.List(ctx, audit.ListOpts{TaskID: "task-rp-live"})
	if hasPhase(liveEvents, audit.PhaseCompleted) {
		t.Fatalf("rp-live CR 仍在,不应补终态,got phases %v", phases(liveEvents))
	}

	// 幂等:再跑一次,rp-gone 不应重复补(已终态被排除)。
	before := len(goneEvents)
	RunOnce(ctx, cl)
	after, _ := store.List(ctx, audit.ListOpts{TaskID: "task-rp-gone"})
	if len(after) != before {
		t.Fatalf("幂等:rp-gone 不应再补,before=%d after=%d", before, len(after))
	}
}

func hasPhase(evs []audit.ActivityEvent, ph audit.Phase) bool {
	for _, e := range evs {
		if e.Phase == ph {
			return true
		}
	}
	return false
}

func phases(evs []audit.ActivityEvent) []audit.Phase {
	var out []audit.Phase
	for _, e := range evs {
		out = append(out, e.Phase)
	}
	return out
}
