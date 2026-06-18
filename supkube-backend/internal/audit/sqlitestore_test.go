package audit

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTmp(t *testing.T) (*SQLiteStore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit.db")
	s, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	return s, path
}

func ev(task, cluster string, ph Phase, ts time.Time) *ActivityEvent {
	return &ActivityEvent{
		TaskID: task, ClusterID: cluster, ActionType: ActionDeleteBackup,
		Phase: ph, ResourceRef: "backup/rp", Triggered: TriggerUser,
		Metadata: map[string]string{"ns": "prod"}, Timestamp: ts,
	}
}

func TestSQLite_AppendListGet(t *testing.T) {
	s, _ := openTmp(t)
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	base := time.Unix(1000, 0).UTC()
	for i, ph := range []Phase{PhaseSubmitted, PhaseDBRCreated, PhaseCompleted} {
		if err := s.Append(ctx, ev("task-1", "clu-1", ph, base.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	got, err := s.List(ctx, ListOpts{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 events, got %d", len(got))
	}
	// 倒序:最近(Completed)在前。
	if got[0].Phase != PhaseCompleted || got[2].Phase != PhaseSubmitted {
		t.Fatalf("应倒序 got[0]=%s got[2]=%s", got[0].Phase, got[2].Phase)
	}
	// ID/Hash 自动填充。
	if got[0].ID == "" || got[0].Hash == "" {
		t.Fatal("ID/Hash 应自动填充")
	}
	// Get 命中 + miss。
	one, err := s.Get(ctx, got[0].ID)
	if err != nil || one == nil || one.ID != got[0].ID {
		t.Fatalf("Get 命中失败: %v %+v", err, one)
	}
	if miss, err := s.Get(ctx, "no-such"); err != nil || miss != nil {
		t.Fatalf("Get miss 应返回 nil,nil;got %+v %v", miss, err)
	}
}

func TestSQLite_Filters(t *testing.T) {
	s, _ := openTmp(t)
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	base := time.Unix(2000, 0).UTC()
	_ = s.Append(ctx, ev("A", "clu-1", PhaseSubmitted, base))
	_ = s.Append(ctx, ev("B", "clu-2", PhaseSubmitted, base.Add(time.Second)))
	_ = s.Append(ctx, ev("A", "clu-1", PhaseCompleted, base.Add(2*time.Second)))

	if g, _ := s.List(ctx, ListOpts{TaskID: "A"}); len(g) != 2 {
		t.Fatalf("TaskID=A 应 2 条,got %d", len(g))
	}
	if g, _ := s.List(ctx, ListOpts{Cluster: "clu-2"}); len(g) != 1 {
		t.Fatalf("Cluster=clu-2 应 1 条,got %d", len(g))
	}
	if g, _ := s.List(ctx, ListOpts{Limit: 1}); len(g) != 1 {
		t.Fatalf("Limit=1 应 1 条,got %d", len(g))
	}
	// 时间范围 [base+1, base+3):应含 B、第二条 A,共 2。
	if g, _ := s.List(ctx, ListOpts{Since: base.Add(time.Second), Until: base.Add(3 * time.Second)}); len(g) != 2 {
		t.Fatalf("时间范围应 2 条,got %d", len(g))
	}
}

func TestSQLite_VerifyChain_Clean(t *testing.T) {
	s, _ := openTmp(t)
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		if err := s.Append(ctx, ev("t", "c", PhaseSubmitted, time.Unix(int64(i), 0))); err != nil {
			t.Fatal(err)
		}
	}
	ok, at, err := s.VerifyChain(ctx)
	if err != nil || !ok || at != "" {
		t.Fatalf("完好链应过,got ok=%v at=%q err=%v", ok, at, err)
	}
}

func TestSQLite_VerifyChain_TamperDetected(t *testing.T) {
	s, path := openTmp(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_ = s.Append(ctx, ev("t", "c", PhaseSubmitted, time.Unix(int64(i), 0)))
	}
	_ = s.Close()

	// 绕过 store 直接篡改一条 data(模拟落盘后被改),VerifyChain 须检出。
	raw, err := OpenSQLite(path) // 重开同库
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Close() }()
	_, err = raw.db.Exec(`UPDATE activity_event SET data=? WHERE seq=(SELECT min(seq) FROM activity_event)`,
		[]byte(`{"id":"x","task_id":"EVIL","phase":"Submitted","hash":"deadbeef","prev_hash":""}`))
	if err != nil {
		t.Fatal(err)
	}
	ok, at, err := raw.VerifyChain(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ok || at == "" {
		t.Fatalf("篡改应被检出,got ok=%v at=%q", ok, at)
	}
}

func TestSQLite_ReopenPersistsChain(t *testing.T) {
	s, path := openTmp(t)
	ctx := context.Background()
	_ = s.Append(ctx, ev("t", "c", PhaseSubmitted, time.Unix(1, 0)))
	lastBefore := s.lastHash
	_ = s.Close()

	// 重开:lastHash 应从末条恢复,后续 Append 接续不断链。
	s2, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	if s2.lastHash != lastBefore {
		t.Fatalf("重开应恢复链尾 hash,want %q got %q", lastBefore, s2.lastHash)
	}
	if err := s2.Append(ctx, ev("t", "c", PhaseCompleted, time.Unix(2, 0))); err != nil {
		t.Fatal(err)
	}
	ok, at, err := s2.VerifyChain(ctx)
	if err != nil || !ok {
		t.Fatalf("跨重开链应连续,got ok=%v at=%q err=%v", ok, at, err)
	}
}

func TestSQLite_PruneTTL_ChainStillValid(t *testing.T) {
	s, _ := openTmp(t)
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	base := time.Unix(0, 0).UTC()
	for i := 0; i < 10; i++ {
		_ = s.Append(ctx, ev("t", "c", PhaseSubmitted, base.Add(time.Duration(i)*time.Hour)))
	}
	// 删前 5 小时之前的(保留后 5 条)。
	n, err := s.PruneBefore(ctx, base.Add(5*time.Hour))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 5 {
		t.Fatalf("应删 5 条,got %d", n)
	}
	got, _ := s.List(ctx, ListOpts{})
	if len(got) != 5 {
		t.Fatalf("应剩 5 条,got %d", len(got))
	}
	// 裁前缀后链仍校验通过(首条 prev 不强校)。
	ok, at, err := s.VerifyChain(ctx)
	if err != nil || !ok {
		t.Fatalf("TTL 裁前缀后链应仍合法,got ok=%v at=%q err=%v", ok, at, err)
	}
}

// DoD#19 量级 sanity(非严格基准,仅确认接口在万级可用)。
func TestSQLite_TenKSanity(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	s, _ := openTmp(t)
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	t0 := time.Now()
	for i := 0; i < 10000; i++ {
		if err := s.Append(ctx, ev("t", "c", PhaseSubmitted, time.Unix(int64(i), 0))); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("10k append(逐条事务): %v", time.Since(t0))
	t1 := time.Now()
	g, _ := s.List(ctx, ListOpts{Limit: 100})
	t.Logf("list 100 倒序: %v (got %d)", time.Since(t1), len(g))
	if len(g) != 100 {
		t.Fatalf("want 100, got %d", len(g))
	}
}
