// Tests for Transport (PRD-007 §4.3 Layer 4 真搬运).
//
// 用例 (TC-COPY-001/002/004/005, fixture-backed minimal fake):
//
//	TC-COPY-001 — BSL→BSL fixture-backed 仓库级 sync (rclone object copy 不是 backup-recreate)
//	TC-COPY-002 — snapshot 类型 BSL 拒绝 (ERR_LAYER4_SNAPSHOT_UNSUPPORTED)
//	TC-COPY-004 — 跨 BSL provider (S3 → Azure Blob)
//	TC-COPY-005 — 不可达 BSL (mock 网络错误) → ERR_LAYER4_BSL_UNREACHABLE
//
// TC-COPY-003 (lifecycle 冲突) 在 lifecycle_test.go.
//
// Fixture 策略 (Gate 0 决策 c): minimal fake (内嵌 BSL CR + fake rclone runner),
// 真 fixture (engineer-testing/fixtures/velero-real-2026-05-31-060756/) 留 Phase 0 E2E task.
package backupcopy

import (
	"context"
	"errors"
	"fmt"
	"testing"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ---- minimal fake BSL builders (匹配 engineer-testing/fixtures/.../backupstoragelocation/) ----

func bslAWS(name, bucket string) *velerov1.BackupStorageLocation {
	return &velerov1.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "velero"},
		Spec: velerov1.BackupStorageLocationSpec{
			Provider: "aws",
			Config: map[string]string{
				"region":           "minio",
				"s3ForcePathStyle": "true",
				"s3Url":            "http://minio.minio.svc:9000",
			},
			StorageType: velerov1.StorageType{
				ObjectStorage: &velerov1.ObjectStorageLocation{Bucket: bucket},
			},
		},
	}
}

func bslAzure(name, bucket string) *velerov1.BackupStorageLocation {
	return &velerov1.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "velero"},
		Spec: velerov1.BackupStorageLocationSpec{
			Provider: "azure",
			Config: map[string]string{
				"resourceGroup":  "DefaultResourceGroup-SEA",
				"storageAccount": "rnd7sa71",
			},
			StorageType: velerov1.StorageType{
				ObjectStorage: &velerov1.ObjectStorageLocation{Bucket: bucket},
			},
		},
	}
}

// bslSnapshot 模拟 "snapshot 类型 BSL" — config 标 volumeDataSource=snapshot.
func bslSnapshot(name, bucket string) *velerov1.BackupStorageLocation {
	bsl := bslAWS(name, bucket)
	bsl.Spec.Config[SnapshotBSLConfigKey] = SnapshotBSLConfigValue
	return bsl
}

// ---- fakeRunner: 不依赖 rclone 二进制, 记录调用 + 可注入失败 ----

type fakeRunner struct {
	calls       []fakeRunnerCall
	failOnSync  bool  // true → 每次 Sync 返 ErrRunnerNetwork
	failGeneric bool  // true → 每次 Sync 返非网络错
	bytesPer    int64 // 每次 Sync 返的 bytes (默认 1024)
	objectsPer  int   // 每次 Sync 返的 objects (默认 4)
	customErr   error
}

type fakeRunnerCall struct {
	src           string
	dst           string
	rateLimitMBps int
}

func (f *fakeRunner) Sync(ctx context.Context, src, dst string, rateLimitMBps int) (int64, int, error) {
	f.calls = append(f.calls, fakeRunnerCall{src: src, dst: dst, rateLimitMBps: rateLimitMBps})
	if f.customErr != nil {
		return 0, 0, f.customErr
	}
	if f.failOnSync {
		return 0, 0, fmt.Errorf("dial tcp: %w", ErrRunnerNetwork)
	}
	if f.failGeneric {
		return 0, 0, errors.New("rclone exit status 5: permission denied")
	}
	b := f.bytesPer
	if b == 0 {
		b = 1024
	}
	o := f.objectsPer
	if o == 0 {
		o = 4
	}
	return b, o, nil
}

// TC-COPY-001: BSL→BSL 仓库级 sync (fixture-backed minimal BSL × 2 → Transport.Copy()
// 断言只 sync 仓库 metadata + Kopia 路径而非 re-backup).
func TestTransport_TC_COPY_001_RepoLevelSync(t *testing.T) {
	src := bslAWS("src-aws", "velero")
	dst := bslAWS("dst-aws", "velero-dr")
	runner := &fakeRunner{bytesPer: 2048, objectsPer: 7}
	tr := NewRcloneRepoSync(runner)

	prog, err := tr.Copy(context.Background(), CopyRequest{
		Source:        src,
		Target:        dst,
		Namespaces:    []string{"test-app"},
		RateLimitMBps: 100,
	})
	if err != nil {
		t.Fatalf("Copy returned error: %v", err)
	}
	if prog == nil {
		t.Fatal("progress is nil")
	}
	if prog.Phase != "Completed" {
		t.Errorf("Phase = %q, want Completed", prog.Phase)
	}

	// 每个 ns 跑 2 个 prefix (kopia + backups) → 1 ns × 2 prefix = 2 Sync 调用
	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 Sync calls (kopia + backups for 1 ns), got %d: %+v", len(runner.calls), runner.calls)
	}

	// 断言走的是 *仓库级* (含 kopia/<ns>/ + backups/), 不是 per-backup
	wantSrcKopia := "src-aws:velero/kopia/test-app/"
	wantSrcMeta := "src-aws:velero/backups/"
	if runner.calls[0].src != wantSrcKopia {
		t.Errorf("calls[0].src = %q, want %q (Kopia repo-level)", runner.calls[0].src, wantSrcKopia)
	}
	if runner.calls[1].src != wantSrcMeta {
		t.Errorf("calls[1].src = %q, want %q (Velero metadata prefix)", runner.calls[1].src, wantSrcMeta)
	}

	// 进度聚合 (2 calls × bytes 2048 = 4096; objects 7 × 2 = 14)
	if prog.BytesTransferred != 4096 {
		t.Errorf("BytesTransferred = %d, want 4096", prog.BytesTransferred)
	}
	if prog.ObjectsCopied != 14 {
		t.Errorf("ObjectsCopied = %d, want 14", prog.ObjectsCopied)
	}
	if len(prog.NamespacesSynced) != 1 || prog.NamespacesSynced[0] != "test-app" {
		t.Errorf("NamespacesSynced = %v, want [test-app]", prog.NamespacesSynced)
	}

	// 速率限制透传
	if runner.calls[0].rateLimitMBps != 100 {
		t.Errorf("rate limit not propagated: %d", runner.calls[0].rateLimitMBps)
	}
}

// TC-COPY-002: snapshot 类型 BSL 拒绝 (ERR_LAYER4_SNAPSHOT_UNSUPPORTED).
// BSL spec.config 含 volumeDataSource=snapshot → Transport.Copy 立即拒, 不调 runner.
func TestTransport_TC_COPY_002_SnapshotBSLRejected(t *testing.T) {
	src := bslSnapshot("src-snap", "velero")
	dst := bslAWS("dst-aws", "velero-dr")
	runner := &fakeRunner{}
	tr := NewRcloneRepoSync(runner)

	_, err := tr.Copy(context.Background(), CopyRequest{
		Source:     src,
		Target:     dst,
		Namespaces: []string{"app1"},
	})
	if err == nil {
		t.Fatal("expected error for snapshot-type source BSL, got nil")
	}
	var te *TransportError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TransportError, got %T: %v", err, err)
	}
	if te.Code != ErrLayer4SnapshotUnsupported {
		t.Errorf("Code = %q, want %q", te.Code, ErrLayer4SnapshotUnsupported)
	}
	if len(runner.calls) != 0 {
		t.Errorf("runner should NOT be called when BSL is snapshot-type, got %d calls", len(runner.calls))
	}

	// 目标 BSL 是 snapshot 也拒
	src2 := bslAWS("src-aws", "velero")
	dst2 := bslSnapshot("dst-snap", "velero-dr")
	_, err2 := tr.Copy(context.Background(), CopyRequest{
		Source:     src2,
		Target:     dst2,
		Namespaces: []string{"app1"},
	})
	if err2 == nil {
		t.Fatal("expected error for snapshot-type target BSL, got nil")
	}
	var te2 *TransportError
	if !errors.As(err2, &te2) || te2.Code != ErrLayer4SnapshotUnsupported {
		t.Errorf("expected ERR_LAYER4_SNAPSHOT_UNSUPPORTED, got %v", err2)
	}
}

// TC-COPY-004: 跨 BSL 提供商 (S3 → Azure Blob) — 走 Transport, 不报错, 返回 progress.
func TestTransport_TC_COPY_004_CrossProvider(t *testing.T) {
	src := bslAWS("src-aws", "velero")
	dst := bslAzure("dst-azure", "velero-dr")
	runner := &fakeRunner{bytesPer: 4096, objectsPer: 10}
	tr := NewRcloneRepoSync(runner)

	prog, err := tr.Copy(context.Background(), CopyRequest{
		Source:        src,
		Target:        dst,
		Namespaces:    []string{"test-app", "kasten-io"},
		RateLimitMBps: 50,
	})
	if err != nil {
		t.Fatalf("cross-provider Copy returned error: %v", err)
	}
	if prog == nil || prog.Phase != "Completed" {
		t.Fatalf("expected Completed progress, got %+v", prog)
	}

	// 2 ns × 2 prefix = 4 calls
	if len(runner.calls) != 4 {
		t.Fatalf("expected 4 Sync calls (2 ns × 2 prefix), got %d", len(runner.calls))
	}

	// 验证一个调用走的是跨 provider (src=aws prefix vs dst=azure prefix)
	if runner.calls[0].src != "src-aws:velero/kopia/test-app/" {
		t.Errorf("calls[0].src = %q", runner.calls[0].src)
	}
	if runner.calls[0].dst != "dst-azure:velero-dr/kopia/test-app/" {
		t.Errorf("calls[0].dst = %q", runner.calls[0].dst)
	}

	if prog.BytesTransferred != 4*4096 {
		t.Errorf("BytesTransferred = %d, want %d", prog.BytesTransferred, 4*4096)
	}
	if len(prog.NamespacesSynced) != 2 {
		t.Errorf("NamespacesSynced = %v, want 2", prog.NamespacesSynced)
	}
}

// TC-COPY-005: 不可达 BSL (mock 网络错误) → ERR_LAYER4_BSL_UNREACHABLE.
func TestTransport_TC_COPY_005_BSLUnreachable(t *testing.T) {
	src := bslAWS("src-aws", "velero")
	dst := bslAWS("dst-aws", "velero-dr")
	runner := &fakeRunner{failOnSync: true}
	tr := NewRcloneRepoSync(runner)

	_, err := tr.Copy(context.Background(), CopyRequest{
		Source:     src,
		Target:     dst,
		Namespaces: []string{"app1"},
	})
	if err == nil {
		t.Fatal("expected error for unreachable BSL, got nil")
	}
	var te *TransportError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TransportError, got %T: %v", err, err)
	}
	if te.Code != ErrLayer4BSLUnreachable {
		t.Errorf("Code = %q, want %q", te.Code, ErrLayer4BSLUnreachable)
	}

	// 网络错应快 fail-fast: 只调 1 次 Sync 就返
	if len(runner.calls) != 1 {
		t.Errorf("expected fail-fast on first Sync, got %d calls", len(runner.calls))
	}
}

// Bonus: 非网络错 (e.g. permission denied) → ERR_LAYER4_TRANSFER_FAILED.
func TestTransport_GenericRunnerError_TransferFailed(t *testing.T) {
	src := bslAWS("src-aws", "velero")
	dst := bslAWS("dst-aws", "velero-dr")
	runner := &fakeRunner{failGeneric: true}
	tr := NewRcloneRepoSync(runner)

	_, err := tr.Copy(context.Background(), CopyRequest{
		Source:     src,
		Target:     dst,
		Namespaces: []string{"app1"},
	})
	if err == nil {
		t.Fatal("expected error for generic runner failure, got nil")
	}
	var te *TransportError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TransportError, got %T: %v", err, err)
	}
	if te.Code != ErrLayer4TransferFailed {
		t.Errorf("Code = %q, want %q", te.Code, ErrLayer4TransferFailed)
	}
}

// Bonus: nil runner → ERR_LAYER4_TRANSFER_FAILED (防呆).
func TestTransport_NilRunner(t *testing.T) {
	tr := NewRcloneRepoSync(nil)
	_, err := tr.Copy(context.Background(), CopyRequest{
		Source:     bslAWS("a", "b"),
		Target:     bslAWS("c", "d"),
		Namespaces: []string{"ns"},
	})
	if err == nil {
		t.Fatal("expected error for nil runner")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != ErrLayer4TransferFailed {
		t.Errorf("expected ERR_LAYER4_TRANSFER_FAILED, got %v", err)
	}
}

// Bonus: 空 namespaces → 拒绝 (Layer 4 必须指定 ns 范围).
func TestTransport_NoNamespaces(t *testing.T) {
	runner := &fakeRunner{}
	tr := NewRcloneRepoSync(runner)
	_, err := tr.Copy(context.Background(), CopyRequest{
		Source:     bslAWS("a", "b"),
		Target:     bslAWS("c", "d"),
		Namespaces: nil,
	})
	if err == nil {
		t.Fatal("expected error for empty namespaces")
	}
	if len(runner.calls) != 0 {
		t.Errorf("runner should not be called, got %d calls", len(runner.calls))
	}
}

// Bonus: BSL 缺 objectStorage.Bucket → 拒绝.
func TestTransport_MissingBucket(t *testing.T) {
	src := bslAWS("src", "")
	src.Spec.ObjectStorage.Bucket = ""
	runner := &fakeRunner{}
	tr := NewRcloneRepoSync(runner)
	_, err := tr.Copy(context.Background(), CopyRequest{
		Source:     src,
		Target:     bslAWS("dst", "ok"),
		Namespaces: []string{"app1"},
	})
	if err == nil {
		t.Fatal("expected error for missing bucket")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != ErrLayer4TransferFailed {
		t.Errorf("expected ERR_LAYER4_TRANSFER_FAILED, got %v", err)
	}
}

// 防回归: TransportError.Error() 包含 code + message.
func TestTransportError_String(t *testing.T) {
	te := &TransportError{Code: "ERR_X", Message: "boom", Cause: errors.New("root")}
	s := te.Error()
	if s == "" {
		t.Fatal("Error() returned empty")
	}
	// 应包含 code
	if !contains(s, "ERR_X") || !contains(s, "boom") {
		t.Errorf("Error() = %q, missing code or message", s)
	}
	// Unwrap 应返 cause
	if errors.Unwrap(te).Error() != "root" {
		t.Errorf("Unwrap mismatch")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > len(sub) && (s[:len(sub)] == sub || s[len(s)-len(sub):] == sub || indexOf(s, sub) >= 0)))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
