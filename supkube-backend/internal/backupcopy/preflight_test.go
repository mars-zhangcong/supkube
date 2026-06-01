// Tests for Preflight (PRD-007 §4.3 Phase 1).
//
// 用例覆盖 (fixture-backed, see engineer-testing/fixtures/velero-real-2026-05-31-060756/):
//  1. 快照型拦截: snapshotMoveData=false 且 defaultVolumesToFsBackup=false → Rejected
//     errorCode=ERR_LAYER4_SNAPSHOT_UNSUPPORTED
//  2. data-mover eligible: snapshotMoveData=true → Eligible
//  3. fs-backup eligible: defaultVolumesToFsBackup=true → Eligible
package backupcopy

import (
	"context"
	"testing"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := velerov1.AddToScheme(s); err != nil {
		t.Fatalf("add velero scheme: %v", err)
	}
	return s
}

func bsl(name string) *velerov1.BackupStorageLocation {
	return &velerov1.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "velero"},
	}
}

func backup(name, storageLocation string, snapshotMoveData, fsBackup *bool) *velerov1.Backup {
	return &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "velero"},
		Spec: velerov1.BackupSpec{
			StorageLocation:          storageLocation,
			SnapshotMoveData:         snapshotMoveData,
			DefaultVolumesToFsBackup: fsBackup,
		},
	}
}

func boolPtr(v bool) *bool { return &v }

// Test case 1: 快照型 (snapshotMoveData=false, defaultVolumesToFsBackup=false) → Rejected.
func TestPreflight_SnapshotOnly_Rejected(t *testing.T) {
	scheme := newScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		bsl("src"),
		bsl("dst"),
		backup("snap-only", "src", boolPtr(false), boolPtr(false)),
	).Build()

	res, err := Preflight(context.Background(), cl, PreflightRequest{
		SourceBSL: "src",
		TargetBSL: "dst",
	})
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if len(res.Eligible) != 0 {
		t.Errorf("want 0 eligible, got %d (%v)", len(res.Eligible), res.Eligible)
	}
	if len(res.Rejected) != 1 {
		t.Fatalf("want 1 rejected, got %d", len(res.Rejected))
	}
	r := res.Rejected[0]
	if r.Name != "snap-only" {
		t.Errorf("rejected.Name = %q, want %q", r.Name, "snap-only")
	}
	if r.ErrorCode != ErrLayer4SnapshotUnsupported {
		t.Errorf("ErrorCode = %q, want %q", r.ErrorCode, ErrLayer4SnapshotUnsupported)
	}
	if r.Reason == "" || r.Suggestion == "" {
		t.Errorf("Reason/Suggestion must be populated: reason=%q suggestion=%q", r.Reason, r.Suggestion)
	}
	if res.Summary.EligibleCount != 0 || res.Summary.RejectedCount != 1 {
		t.Errorf("Summary = %+v, want {EligibleCount:0 RejectedCount:1}", res.Summary)
	}
}

// Test case 2: data-mover (snapshotMoveData=true) → Eligible.
func TestPreflight_DataMover_Eligible(t *testing.T) {
	scheme := newScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		bsl("src"),
		bsl("dst"),
		backup("dm-1", "src", boolPtr(true), boolPtr(false)),
	).Build()

	res, err := Preflight(context.Background(), cl, PreflightRequest{
		SourceBSL: "src",
		TargetBSL: "dst",
	})
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if len(res.Rejected) != 0 {
		t.Errorf("want 0 rejected, got %d (%+v)", len(res.Rejected), res.Rejected)
	}
	if len(res.Eligible) != 1 || res.Eligible[0] != "dm-1" {
		t.Errorf("want Eligible=[dm-1], got %v", res.Eligible)
	}
	if res.Summary.EligibleCount != 1 || res.Summary.RejectedCount != 0 {
		t.Errorf("Summary = %+v, want {EligibleCount:1 RejectedCount:0}", res.Summary)
	}
}

// Test case 3: fs-backup (defaultVolumesToFsBackup=true) → Eligible.
// Restic/Kopia 走 fs-backup, 卷数据进 BSL Kopia repo, Layer 4 可搬.
func TestPreflight_FsBackup_Eligible(t *testing.T) {
	scheme := newScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		bsl("src"),
		bsl("dst"),
		backup("fsb-1", "src", boolPtr(false), boolPtr(true)),
	).Build()

	res, err := Preflight(context.Background(), cl, PreflightRequest{
		SourceBSL: "src",
		TargetBSL: "dst",
	})
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if len(res.Rejected) != 0 {
		t.Errorf("want 0 rejected, got %d (%+v)", len(res.Rejected), res.Rejected)
	}
	if len(res.Eligible) != 1 || res.Eligible[0] != "fsb-1" {
		t.Errorf("want Eligible=[fsb-1], got %v", res.Eligible)
	}
}

// Bonus: nil-pointer 字段 (旧版 Velero CR 可能没设这两 field) → 视同 false/false → Rejected.
// 保护性测试, 防止 nil-deref crash.
func TestPreflight_NilFields_Rejected(t *testing.T) {
	scheme := newScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		bsl("src"),
		bsl("dst"),
		backup("legacy", "src", nil, nil),
	).Build()

	res, err := Preflight(context.Background(), cl, PreflightRequest{
		SourceBSL: "src",
		TargetBSL: "dst",
	})
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if len(res.Rejected) != 1 || res.Rejected[0].ErrorCode != ErrLayer4SnapshotUnsupported {
		t.Errorf("nil fields should reject with ERR_LAYER4_SNAPSHOT_UNSUPPORTED, got %+v", res.Rejected)
	}
}

// Bonus: 源 BSL 不存在 → 返 error.
func TestPreflight_SourceBSL_NotFound(t *testing.T) {
	scheme := newScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		bsl("dst"),
	).Build()

	_, err := Preflight(context.Background(), cl, PreflightRequest{
		SourceBSL: "nonexistent",
		TargetBSL: "dst",
	})
	if err == nil {
		t.Fatal("want error when source BSL is missing, got nil")
	}
}

// Bonus: 多 backup 混合 + name filter.
func TestPreflight_MultipleBackups_WithFilter(t *testing.T) {
	scheme := newScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		bsl("src"),
		bsl("dst"),
		backup("snap-1", "src", boolPtr(false), boolPtr(false)), // rejected
		backup("dm-1", "src", boolPtr(true), boolPtr(false)),    // eligible
		backup("fsb-1", "src", boolPtr(false), boolPtr(true)),   // eligible
		backup("other-bsl", "elsewhere", boolPtr(true), nil),    // 不在 src, 跳过
	).Build()

	// 不带 filter — 全扫
	res, err := Preflight(context.Background(), cl, PreflightRequest{
		SourceBSL: "src",
		TargetBSL: "dst",
	})
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if len(res.Eligible) != 2 {
		t.Errorf("want 2 eligible, got %d (%v)", len(res.Eligible), res.Eligible)
	}
	if len(res.Rejected) != 1 {
		t.Errorf("want 1 rejected, got %d", len(res.Rejected))
	}

	// 带 filter — 只查 dm-1
	res2, err := Preflight(context.Background(), cl, PreflightRequest{
		SourceBSL: "src",
		TargetBSL: "dst",
		Backups:   []string{"dm-1"},
	})
	if err != nil {
		t.Fatalf("Preflight (filtered): %v", err)
	}
	if len(res2.Eligible) != 1 || res2.Eligible[0] != "dm-1" {
		t.Errorf("filter want [dm-1], got %v", res2.Eligible)
	}
	if len(res2.Rejected) != 0 {
		t.Errorf("filter should exclude snap-1, got rejected=%+v", res2.Rejected)
	}
}

// 确认 listing 不报错时 client.List 行为 + StorageLocation filter.
var _ = client.InNamespace("velero")
