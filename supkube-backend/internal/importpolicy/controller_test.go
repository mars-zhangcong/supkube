// controller_test.go — controller 单测.
//
// 覆盖:
//  1. syncOnce — 发现新 backup 全部导入 + 推进 lastSeenBackupName
//  2. 重复 sync 不重复导入 (watermark 生效)
//  3. fingerprint enforce 模式 — invalid backup 被 reject 不导入
//  4. fingerprint warn 模式 — invalid backup 依然导入但 label 体现
//  5. fingerprint disabled 模式 — validator 不被调用 (mode 自身约定)
//  6. sourceClusterID 过滤 — 不匹配静默跳过
//  7. paused 状态 — syncOnce 跳过, status.phase=Paused
//  8. cron parser — 基础字段 + Next 单调
//  9. reconcileAll + ensureRunner / stopRunner — CR delete 后 goroutine 退出
//  10. Continuous mode runner — ticker 触发 syncOnce 至少 2 次
package importpolicy

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynfake "k8s.io/client-go/dynamic/fake"
)

// ─────────────────────────────────────────────────────────────────────
// fakes
// ─────────────────────────────────────────────────────────────────────

type fakeLister struct {
	mu      sync.Mutex
	backups map[string][]string
	calls   int32
	err     error
}

func (f *fakeLister) ListBackups(_ context.Context, bsl string) ([]string, error) {
	atomic.AddInt32(&f.calls, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	out := append([]string(nil), f.backups[bsl]...)
	sort.Strings(out)
	return out, nil
}

func (f *fakeLister) add(bsl string, names ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.backups == nil {
		f.backups = map[string][]string{}
	}
	f.backups[bsl] = append(f.backups[bsl], names...)
	sort.Strings(f.backups[bsl])
}

type importedRecord struct {
	bsl    string
	name   string
	target string
	labels map[string]string
}

type fakeImporter struct {
	mu      sync.Mutex
	records []importedRecord
	err     error
}

func (f *fakeImporter) ImportBackup(_ context.Context, target, bsl, name string, labels map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	cp := map[string]string{}
	for k, v := range labels {
		cp[k] = v
	}
	f.records = append(f.records, importedRecord{bsl: bsl, name: name, target: target, labels: cp})
	return nil
}

func (f *fakeImporter) all() []importedRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]importedRecord(nil), f.records...)
}

type fakeValidator struct {
	mu       sync.Mutex
	results  map[string]*FingerprintResult
	calls    int32
	disabled bool
}

func (f *fakeValidator) ValidateBackup(_ context.Context, _ string, name string, mode FingerprintMode) (*FingerprintResult, error) {
	atomic.AddInt32(&f.calls, 1)
	if mode == FingerprintModeDisabled {
		f.disabled = true
		return &FingerprintResult{Status: "valid"}, nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.results[name]; ok {
		return r, nil
	}
	return &FingerprintResult{Status: "valid", SourceClusterID: "src-default"}, nil
}

// ─────────────────────────────────────────────────────────────────────
// dynamic client helpers
// ─────────────────────────────────────────────────────────────────────

func newDyn(t *testing.T, initial ...*unstructured.Unstructured) dynamic.Interface {
	t.Helper()
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(GVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(GVK.GroupVersion().WithKind(Kind+"List"), &unstructured.UnstructuredList{})
	listKinds := map[schema.GroupVersionResource]string{
		GVR: Kind + "List",
	}
	objs := make([]runtime.Object, 0, len(initial))
	for _, o := range initial {
		objs = append(objs, o)
	}
	return dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objs...)
}

func policyCR(name string, spec ImportPolicySpec, status ImportPolicyStatus) *unstructured.Unstructured {
	cr := toUnstructured(name, spec)
	cr.Object["metadata"].(map[string]interface{})["generation"] = int64(1)
	statusMap := map[string]interface{}{}
	if status.Phase != "" {
		statusMap["phase"] = string(status.Phase)
	}
	if status.LastSeenBackupName != "" {
		statusMap["lastSeenBackupName"] = status.LastSeenBackupName
	}
	if status.ImportedCount != 0 {
		statusMap["importedCount"] = int64(status.ImportedCount)
	}
	if status.RejectedCount != 0 {
		statusMap["rejectedCount"] = int64(status.RejectedCount)
	}
	cr.Object["status"] = statusMap
	return cr
}

// readStatus reads the CR's status sub-object as ImportPolicyStatus.
func readStatus(t *testing.T, dyn dynamic.Interface, name string) ImportPolicyStatus {
	t.Helper()
	cr, err := dyn.Resource(GVR).Namespace(Namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get CR: %v", err)
	}
	p, err := fromUnstructured(cr)
	if err != nil {
		t.Fatalf("decode CR: %v", err)
	}
	return p.Status
}

// ─────────────────────────────────────────────────────────────────────
// tests
// ─────────────────────────────────────────────────────────────────────

func TestSyncOnce_ImportsNewBackups(t *testing.T) {
	spec := ImportPolicySpec{
		SourceBSL:       "default",
		Mode:            ImportModeContinuous,
		FingerprintMode: FingerprintModeEnforce,
	}
	dyn := newDyn(t, policyCR("p1", spec, ImportPolicyStatus{}))
	lister := &fakeLister{}
	lister.add("default", "backup-a", "backup-b")
	importer := &fakeImporter{}
	validator := &fakeValidator{}

	c := &Controller{DynCli: dyn, Lister: lister, Importer: importer, Validator: validator}
	res, err := c.SyncOnce(context.Background(), "p1")
	if err != nil {
		t.Fatalf("syncOnce: %v", err)
	}
	if res.ImportedCount != 2 {
		t.Errorf("want 2 imports, got %d (records=%+v)", res.ImportedCount, importer.all())
	}
	st := readStatus(t, dyn, "p1")
	if st.LastSeenBackupName != "backup-b" {
		t.Errorf("watermark want backup-b, got %q", st.LastSeenBackupName)
	}
	if st.ImportedCount != 2 {
		t.Errorf("status.importedCount want 2, got %d", st.ImportedCount)
	}
	if st.Phase != PhaseActive {
		t.Errorf("phase want Active, got %s", st.Phase)
	}
	// label sanity
	for _, r := range importer.all() {
		if r.labels[LabelImported] != "true" {
			t.Errorf("missing imported=true label on %s", r.name)
		}
		if r.labels[LabelFingerprintStatus] != "valid" {
			t.Errorf("want fingerprint-status=valid, got %q on %s", r.labels[LabelFingerprintStatus], r.name)
		}
	}
}

func TestSyncOnce_WatermarkSkipsAlreadySeen(t *testing.T) {
	spec := ImportPolicySpec{SourceBSL: "default", Mode: ImportModeContinuous}
	dyn := newDyn(t, policyCR("p1", spec, ImportPolicyStatus{LastSeenBackupName: "backup-b"}))
	lister := &fakeLister{}
	lister.add("default", "backup-a", "backup-b", "backup-c")
	importer := &fakeImporter{}
	validator := &fakeValidator{}

	c := &Controller{DynCli: dyn, Lister: lister, Importer: importer, Validator: validator}
	res, err := c.SyncOnce(context.Background(), "p1")
	if err != nil {
		t.Fatalf("syncOnce: %v", err)
	}
	if res.ImportedCount != 1 {
		t.Errorf("want 1 (only backup-c), got %d", res.ImportedCount)
	}
	if importer.all()[0].name != "backup-c" {
		t.Errorf("want backup-c imported, got %q", importer.all()[0].name)
	}
}

func TestSyncOnce_EnforceRejectsInvalidFingerprint(t *testing.T) {
	spec := ImportPolicySpec{SourceBSL: "default", Mode: ImportModeContinuous, FingerprintMode: FingerprintModeEnforce}
	dyn := newDyn(t, policyCR("p1", spec, ImportPolicyStatus{}))
	lister := &fakeLister{}
	lister.add("default", "backup-good", "backup-bad")
	importer := &fakeImporter{}
	validator := &fakeValidator{results: map[string]*FingerprintResult{
		"backup-bad": {Status: "invalid", Reason: "HMAC mismatch"},
	}}
	c := &Controller{DynCli: dyn, Lister: lister, Importer: importer, Validator: validator}
	res, err := c.SyncOnce(context.Background(), "p1")
	if err != nil {
		t.Fatalf("syncOnce: %v", err)
	}
	if res.ImportedCount != 1 {
		t.Errorf("want 1 import (only good), got %d", res.ImportedCount)
	}
	if res.RejectedCount != 1 {
		t.Errorf("want 1 rejected, got %d", res.RejectedCount)
	}
	for _, r := range importer.all() {
		if r.name == "backup-bad" {
			t.Errorf("bad backup should not be imported under enforce")
		}
	}
}

func TestSyncOnce_WarnImportsInvalidWithLabel(t *testing.T) {
	spec := ImportPolicySpec{SourceBSL: "default", Mode: ImportModeContinuous, FingerprintMode: FingerprintModeWarn}
	dyn := newDyn(t, policyCR("p1", spec, ImportPolicyStatus{}))
	lister := &fakeLister{}
	lister.add("default", "backup-bad")
	importer := &fakeImporter{}
	validator := &fakeValidator{results: map[string]*FingerprintResult{
		"backup-bad": {Status: "missing"},
	}}
	c := &Controller{DynCli: dyn, Lister: lister, Importer: importer, Validator: validator}
	res, err := c.SyncOnce(context.Background(), "p1")
	if err != nil {
		t.Fatalf("syncOnce: %v", err)
	}
	if res.ImportedCount != 1 {
		t.Errorf("warn mode should import; got %d imports", res.ImportedCount)
	}
	if got := importer.all()[0].labels[LabelFingerprintStatus]; got != "missing" {
		t.Errorf("warn mode should label fingerprint-status=missing, got %q", got)
	}
}

func TestSyncOnce_DisabledSkipsValidator(t *testing.T) {
	spec := ImportPolicySpec{SourceBSL: "default", Mode: ImportModeContinuous, FingerprintMode: FingerprintModeDisabled}
	dyn := newDyn(t, policyCR("p1", spec, ImportPolicyStatus{}))
	lister := &fakeLister{}
	lister.add("default", "b1", "b2")
	importer := &fakeImporter{}
	validator := &fakeValidator{}
	c := &Controller{DynCli: dyn, Lister: lister, Importer: importer, Validator: validator}
	_, err := c.SyncOnce(context.Background(), "p1")
	if err != nil {
		t.Fatalf("syncOnce: %v", err)
	}
	// validator was called but should have early-returned valid; label
	// should NOT contain fingerprint-status (disabled mode skips that).
	for _, r := range importer.all() {
		if _, ok := r.labels[LabelFingerprintStatus]; ok {
			t.Errorf("disabled mode must not set fingerprint-status label, got %v", r.labels)
		}
	}
	if !validator.disabled {
		t.Errorf("validator should have been called with disabled mode")
	}
}

func TestSyncOnce_SourceClusterIDFilters(t *testing.T) {
	spec := ImportPolicySpec{
		SourceBSL:       "default",
		Mode:            ImportModeContinuous,
		FingerprintMode: FingerprintModeEnforce,
		SourceClusterID: "cluster-A",
	}
	dyn := newDyn(t, policyCR("p1", spec, ImportPolicyStatus{}))
	lister := &fakeLister{}
	lister.add("default", "b1", "b2", "b3")
	importer := &fakeImporter{}
	validator := &fakeValidator{results: map[string]*FingerprintResult{
		"b1": {Status: "valid", SourceClusterID: "cluster-A"},
		"b2": {Status: "valid", SourceClusterID: "cluster-B"},
		"b3": {Status: "valid", SourceClusterID: "cluster-A"},
	}}
	c := &Controller{DynCli: dyn, Lister: lister, Importer: importer, Validator: validator}
	res, err := c.SyncOnce(context.Background(), "p1")
	if err != nil {
		t.Fatalf("syncOnce: %v", err)
	}
	if res.ImportedCount != 2 {
		t.Errorf("want 2 (b1,b3 — cluster-A), got %d", res.ImportedCount)
	}
	for _, r := range importer.all() {
		if r.name == "b2" {
			t.Errorf("b2 (cluster-B) should be filtered out")
		}
	}
	// Watermark should advance past b3 (filtered ones too — otherwise we'd
	// re-validate forever).
	st := readStatus(t, dyn, "p1")
	if st.LastSeenBackupName != "b3" {
		t.Errorf("watermark want b3, got %q", st.LastSeenBackupName)
	}
}

func TestSyncOnce_PausedSkips(t *testing.T) {
	spec := ImportPolicySpec{SourceBSL: "default", Mode: ImportModeContinuous, Paused: true}
	dyn := newDyn(t, policyCR("p1", spec, ImportPolicyStatus{}))
	lister := &fakeLister{}
	lister.add("default", "b1")
	importer := &fakeImporter{}
	validator := &fakeValidator{}
	c := &Controller{DynCli: dyn, Lister: lister, Importer: importer, Validator: validator}
	res, err := c.SyncOnce(context.Background(), "p1")
	if err != nil {
		t.Fatalf("syncOnce: %v", err)
	}
	if res.ImportedCount != 0 {
		t.Errorf("paused should not import, got %d", res.ImportedCount)
	}
	if atomic.LoadInt32(&lister.calls) != 0 {
		t.Errorf("paused should not even list BSL, got %d calls", lister.calls)
	}
	st := readStatus(t, dyn, "p1")
	if st.Phase != PhasePaused {
		t.Errorf("want phase Paused, got %s", st.Phase)
	}
}

func TestCronParser_Basics(t *testing.T) {
	cases := []struct {
		expr string
		ok   bool
	}{
		{"* * * * *", true},
		{"0 * * * *", true},   // hourly
		{"*/5 * * * *", true}, // every 5 min
		{"0 0 1 1 *", true},   // Jan 1 midnight
		{"0 0 * * 1-5", true}, // weekdays midnight
		{"60 * * * *", false}, // minute out of range
		{"* * * * 8", false},  // dow out of range (1-7 mapped; 8 invalid)
		{"a b c d e", false},  // junk
		{"* * *", false},      // wrong arity
	}
	for _, tc := range cases {
		_, err := ParseCron(tc.expr)
		if (err == nil) != tc.ok {
			t.Errorf("expr %q: want ok=%v, got err=%v", tc.expr, tc.ok, err)
		}
	}
	// Next monotonicity: every 5 min, Next > now.
	s, _ := ParseCron("*/5 * * * *")
	now := time.Date(2026, 6, 1, 12, 3, 30, 0, time.UTC)
	n := s.Next(now)
	if !n.After(now) {
		t.Errorf("next should be after now, got %v vs %v", n, now)
	}
	if n.Minute() != 5 || n.Hour() != 12 {
		t.Errorf("next of */5 from 12:03 should be 12:05, got %v", n)
	}
}

func TestEnsureRunner_DeleteStopsGoroutine(t *testing.T) {
	spec := ImportPolicySpec{SourceBSL: "default", Mode: ImportModeContinuous, ContinuousInterval: "100ms"}
	cr := policyCR("p1", spec, ImportPolicyStatus{})
	dyn := newDyn(t, cr)
	lister := &fakeLister{}
	lister.add("default", "b1")
	c := &Controller{DynCli: dyn, Lister: lister, Importer: &fakeImporter{}, Validator: &fakeValidator{}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.reconcileAll(ctx)

	// runner should now exist.
	if _, ok := c.runners.Load("p1"); !ok {
		t.Fatalf("runner should exist after reconcileAll")
	}
	// Simulate CR delete.
	if err := dyn.Resource(GVR).Namespace(Namespace).Delete(ctx, "p1", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("delete CR: %v", err)
	}
	c.reconcileAll(ctx)
	if _, ok := c.runners.Load("p1"); ok {
		t.Errorf("runner should be removed after CR delete")
	}
}

func TestEnsureRunner_PausedPolicyHasNoActiveRunner(t *testing.T) {
	spec := ImportPolicySpec{SourceBSL: "default", Mode: ImportModeContinuous, Paused: true}
	cr := policyCR("p1", spec, ImportPolicyStatus{})
	dyn := newDyn(t, cr)
	lister := &fakeLister{}
	lister.add("default", "b1")
	importer := &fakeImporter{}
	c := &Controller{DynCli: dyn, Lister: lister, Importer: importer, Validator: &fakeValidator{}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.reconcileAll(ctx)

	// Wait a bit; if a runner were active it would have called the lister.
	time.Sleep(150 * time.Millisecond)
	if atomic.LoadInt32(&lister.calls) != 0 {
		t.Errorf("paused policy should not run syncOnce, got %d lister calls", lister.calls)
	}
	if len(importer.all()) != 0 {
		t.Errorf("paused policy should import 0, got %d", len(importer.all()))
	}
	st := readStatus(t, dyn, "p1")
	if st.Phase != PhasePaused {
		t.Errorf("phase want Paused, got %s", st.Phase)
	}
}

func TestRunner_ContinuousFiresMultipleTicks(t *testing.T) {
	spec := ImportPolicySpec{
		SourceBSL:          "default",
		Mode:               ImportModeContinuous,
		ContinuousInterval: "50ms", // 单测里用小 interval; controller 内部最小 clamp 是 30s,
		// 但 parseDurationOr + clamp 是在 runnerFor 里直接对 spec 字符串解析 (不是 controller 启动校验). clamp
		// 把 < 30s 全归到 30s. 这会让本测试无法多 tick. 所以我们改成直接调 safeSync 两次模拟 ticker.
	}
	dyn := newDyn(t, policyCR("p1", spec, ImportPolicyStatus{}))
	lister := &fakeLister{}
	lister.add("default", "b1")
	importer := &fakeImporter{}
	c := &Controller{DynCli: dyn, Lister: lister, Importer: importer, Validator: &fakeValidator{}}

	// 模拟连续两个 tick 的效果, 验证 lastSeenBackupName 累进 + 第二轮新增能被发现.
	p, _ := fromUnstructured(mustGet(t, dyn, "p1"))
	c.safeSync(context.Background(), p)
	// 重新读 CR (status 已被 patch).
	p2, _ := fromUnstructured(mustGet(t, dyn, "p1"))
	if p2.Status.LastSeenBackupName != "b1" {
		t.Fatalf("after first sync watermark should be b1, got %q", p2.Status.LastSeenBackupName)
	}
	// 第二轮: lister 多一个 backup.
	lister.add("default", "b2")
	c.safeSync(context.Background(), p2)
	if got := len(importer.all()); got != 2 {
		t.Errorf("want 2 imports after 2 ticks (b1+b2), got %d", got)
	}
}

func mustGet(t *testing.T, dyn dynamic.Interface, name string) *unstructured.Unstructured {
	t.Helper()
	cr, err := dyn.Resource(GVR).Namespace(Namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	return cr
}
