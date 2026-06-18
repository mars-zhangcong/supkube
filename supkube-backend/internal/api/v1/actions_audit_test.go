package v1

import (
	"testing"
	"time"

	"github.com/supkube/supkube-backend/internal/audit"
)

func ae(backup string, action audit.ActionType, ph audit.Phase, ts time.Time) audit.ActivityEvent {
	return audit.ActivityEvent{
		TaskID: "t", ActionType: action, Phase: ph,
		ResourceRef: "backup/" + backup, Timestamp: ts,
	}
}

func TestAuditHistoryActions_OnlyDeadBackups(t *testing.T) {
	now := time.Now().UTC()
	// 倒序(最近优先):rp-gone 已删两次(取最近 Completed)、rp-live 删过但 CR 还在、一条 restore 噪声。
	events := []audit.ActivityEvent{
		ae("rp-gone", audit.ActionForceStripFinalizer, audit.PhaseCompleted, now),
		ae("rp-live", audit.ActionDeleteBackup, audit.PhaseDBRCreated, now.Add(-time.Minute)),
		ae("rp-gone", audit.ActionDeleteBackup, audit.PhaseSubmitted, now.Add(-2*time.Minute)), // 同 backup 旧事件→应被去重
		{TaskID: "t", ActionType: "Restore", Phase: audit.PhaseCompleted, ResourceRef: "restore/x", Timestamp: now},
	}
	live := map[string]bool{"Backup/rp-live": true} // rp-live 的活 CR 仍在

	got := auditHistoryActions(events, live)
	if len(got) != 1 {
		t.Fatalf("应只补 1 张(rp-gone),got %d: %+v", len(got), got)
	}
	a := got[0]
	if a.ID != "rp-gone" || a.Type != ActionTypeBackup {
		t.Fatalf("应为 rp-gone Backup,got %s/%s", a.ID, a.Type)
	}
	if a.Status != ActionStatusCompleted {
		t.Fatalf("Completed 阶段应映射 completed,got %s", a.Status)
	}
	if a.Origin != "audit-history" {
		t.Fatalf("应标记来源 audit-history,got %s", a.Origin)
	}
	if a.StartTime == nil {
		t.Fatal("应带 StartTime")
	}
}

func TestAuditHistoryActions_EmptyWhenAllLive(t *testing.T) {
	now := time.Now().UTC()
	events := []audit.ActivityEvent{ae("rp-1", audit.ActionDeleteBackup, audit.PhaseCompleted, now)}
	live := map[string]bool{"Backup/rp-1": true}
	if got := auditHistoryActions(events, live); len(got) != 0 {
		t.Fatalf("活 CR 仍在应不补卡,got %d", len(got))
	}
}

func TestAuditPhaseToStatus(t *testing.T) {
	cases := map[audit.Phase]string{
		audit.PhaseCompleted:  ActionStatusCompleted,
		audit.PhaseCRRemoved:  ActionStatusCompleted,
		audit.PhaseFailed:     ActionStatusFailed,
		audit.PhaseSubmitted:  ActionStatusRunning,
		audit.PhaseDBRCreated: ActionStatusRunning,
	}
	for ph, want := range cases {
		if got := auditPhaseToStatus(ph); got != want {
			t.Fatalf("phase %s → want %s got %s", ph, want, got)
		}
	}
}
