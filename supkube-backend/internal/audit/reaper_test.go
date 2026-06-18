package audit

import (
	"context"
	"testing"
	"time"
)

func TestPruneOnce_DeletesOldAndAuditsItself(t *testing.T) {
	s, _ := openTmp(t)
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	now := time.Now().UTC()
	// 5 条"老"(100 天前)+ 3 条"新"(今天)。
	for i := 0; i < 5; i++ {
		_ = s.Append(ctx, ev("old", "c", PhaseSubmitted, now.Add(-100*24*time.Hour).Add(time.Duration(i)*time.Minute)))
	}
	for i := 0; i < 3; i++ {
		_ = s.Append(ctx, ev("new", "c", PhaseSubmitted, now.Add(time.Duration(i)*time.Minute)))
	}

	n, err := PruneOnce(ctx, s, DefaultRetention) // 90 天
	if err != nil {
		t.Fatalf("PruneOnce: %v", err)
	}
	if n != 5 {
		t.Fatalf("应删 5 条老事件,got %d", n)
	}
	// 剩 3 新 + 1 条 TTL 自审计事件 = 4。
	got, _ := s.List(ctx, ListOpts{})
	if len(got) != 4 {
		t.Fatalf("应剩 4 条(3新+1自审计),got %d", len(got))
	}
	// 最近一条应是 TTL 自审计事件,记录删了 5 条。
	if got[0].ActionType != ActionTTLPrune || got[0].Metadata["pruned"] != "5" {
		t.Fatalf("应有 TTL 自审计事件 pruned=5,got %s pruned=%q", got[0].ActionType, got[0].Metadata["pruned"])
	}
	// 裁剪 + 追加后链仍合法。
	if ok, at, err := s.VerifyChain(ctx); err != nil || !ok {
		t.Fatalf("prune 后链应合法,got ok=%v at=%q err=%v", ok, at, err)
	}
}

func TestPruneOnce_NoOldNoAuditNoise(t *testing.T) {
	s, _ := openTmp(t)
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	_ = s.Append(ctx, ev("new", "c", PhaseSubmitted, time.Now().UTC()))
	n, err := PruneOnce(ctx, s, DefaultRetention)
	if err != nil || n != 0 {
		t.Fatalf("无老事件应删 0,got %d err=%v", n, err)
	}
	// 无删除 → 不写自审计噪声(仍 1 条)。
	if got, _ := s.List(ctx, ListOpts{}); len(got) != 1 {
		t.Fatalf("无删除不应写自审计,应仍 1 条,got %d", len(got))
	}
}

func TestStartTTLReaper_RunsAndStops(t *testing.T) {
	s, _ := openTmp(t)
	defer func() { _ = s.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	_ = s.Append(ctx, ev("old", "c", PhaseSubmitted, time.Now().UTC().Add(-100*24*time.Hour)))
	stop := StartTTLReaper(ctx, s, DefaultRetention, time.Hour) // 立即跑一次
	// 等首轮 prune 生效。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if g, _ := s.List(ctx, ListOpts{TaskID: "ttl-prune"}); len(g) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if g, _ := s.List(ctx, ListOpts{TaskID: "ttl-prune"}); len(g) != 1 {
		t.Fatal("reaper 首轮应已 prune 并写自审计")
	}
	cancel()
	stop() // 应及时返回(goroutine 退出)
}
