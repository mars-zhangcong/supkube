// s3_lister_test.go — 覆盖 s3BackupLister.ListBackups 关键路径.
//
// 测试 strategy: 用 in-memory fake BSLObjectClient (key → []byte map),
// 模拟 ListPrefix 返回固定 key 集合. 不依赖 Velero scheme / runtime
// client 走真 CRD 路径 — 那是 runtime_impl_test.go 的事. 这里只关心
// "marker 文件识别 + dedupe + 排序 + 分页等价语义".
package importpolicy

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// fakeBSLObjectClient 是 fingerprint.BSLObjectClient 的最小实现, 仅覆
// 盖 s3BackupLister/s3BackupImporter 需要的方法. Get/Put 用 map, List
// 用前缀线性扫描 — 50 backup 的测试也只是一次 O(N).
type fakeBSLObjectClient struct {
	mu      sync.Mutex
	objects map[string][]byte // key = "<bslName>|<objectKey>"
	prefix  string
	failGet bool
}

func newFakeBSLObjectClient() *fakeBSLObjectClient {
	return &fakeBSLObjectClient{objects: map[string][]byte{}}
}

func (f *fakeBSLObjectClient) k(bsl, key string) string { return bsl + "|" + key }

func (f *fakeBSLObjectClient) GetObject(_ context.Context, bsl, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failGet {
		return nil, fmt.Errorf("simulated read error")
	}
	v, ok := f.objects[f.k(bsl, key)]
	if !ok {
		return nil, fmt.Errorf("fake BSL: not found: %s", key)
	}
	return append([]byte(nil), v...), nil
}

func (f *fakeBSLObjectClient) PutObject(_ context.Context, bsl, key string, body []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[f.k(bsl, key)] = append([]byte(nil), body...)
	return nil
}

func (f *fakeBSLObjectClient) Prefix(_ context.Context, _ string) (string, error) {
	return f.prefix, nil
}

func (f *fakeBSLObjectClient) ListPrefix(_ context.Context, bsl, keyPrefix string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	bslPrefix := bsl + "|"
	for k := range f.objects {
		if !strings.HasPrefix(k, bslPrefix) {
			continue
		}
		objKey := strings.TrimPrefix(k, bslPrefix)
		if strings.HasPrefix(objKey, keyPrefix) {
			out = append(out, objKey)
		}
	}
	return out, nil
}

// stubVeleroLister 是 BackupLister 的简单 stub, 给 s3BackupLister 做 dedup.
type stubVeleroLister struct {
	known map[string][]string // bsl → names
	err   error
}

func (s *stubVeleroLister) ListBackups(_ context.Context, bsl string) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.known[bsl], nil
}

// seedBackup 在 fake BSL 写一个 velero-backup.json marker (内容随便).
func seedBackup(f *fakeBSLObjectClient, bsl, name string) {
	key := "backups/" + name + "/velero-backup.json"
	f.objects[f.k(bsl, key)] = []byte(`{"kind":"Backup"}`)
}

// seedTarball 在 fake BSL 写一个 .tar.gz (不带 manifest), 用来确认 lister
// 不会把 tarball-only 的 backup 算进去.
func seedTarball(f *fakeBSLObjectClient, bsl, name string) {
	key := "backups/" + name + "/" + name + ".tar.gz"
	f.objects[f.k(bsl, key)] = []byte("fake tar")
}

func TestS3Lister_FindsBackupsByManifest(t *testing.T) {
	bsl := newFakeBSLObjectClient()
	seedBackup(bsl, "default", "alpha")
	seedBackup(bsl, "default", "beta")
	seedBackup(bsl, "default", "gamma")
	// Other BSL — 不应被算进去.
	seedBackup(bsl, "other", "noise")
	// tarball-only — 没 manifest, 应被忽略.
	seedTarball(bsl, "default", "ghost")

	l := NewS3BackupLister(bsl, nil)
	names, err := l.ListBackups(context.Background(), "default")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{"alpha", "beta", "gamma"}
	if len(names) != len(want) {
		t.Fatalf("want %v, got %v", want, names)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("at %d want %s, got %s", i, n, names[i])
		}
	}
}

func TestS3Lister_DedupesAgainstVelero(t *testing.T) {
	bsl := newFakeBSLObjectClient()
	for _, n := range []string{"a", "b", "c", "d"} {
		seedBackup(bsl, "default", n)
	}
	// Velero 已经 sync 了 b 和 c — lister 应只返回 a, d.
	dedupe := &stubVeleroLister{known: map[string][]string{
		"default": {"b", "c"},
	}}
	l := NewS3BackupLister(bsl, dedupe)
	names, err := l.ListBackups(context.Background(), "default")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{"a", "d"}
	if len(names) != len(want) {
		t.Fatalf("want %v, got %v", want, names)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("at %d want %s, got %s", i, n, names[i])
		}
	}
}

func TestS3Lister_DedupeErrorFallsBackToFullList(t *testing.T) {
	// Velero CR list 出错时, 我们仍然返回 BSL 的 candidates — importer 的
	// AlreadyExists 兜底会保证幂等. 不要因为 dedup 故障就丢 RPO.
	bsl := newFakeBSLObjectClient()
	seedBackup(bsl, "default", "x")
	dedupe := &stubVeleroLister{err: fmt.Errorf("velero api down")}
	l := NewS3BackupLister(bsl, dedupe)
	names, err := l.ListBackups(context.Background(), "default")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(names) != 1 || names[0] != "x" {
		t.Fatalf("expected [x], got %v", names)
	}
}

func TestS3Lister_HandlesManyBackups(t *testing.T) {
	// "分页"在我们的设计里被 BSLObjectClient.ListPrefix 内部消化 — 这里
	// 用 50 个 backup 验证 lister 自己的循环没漏数 + 排序稳定.
	bsl := newFakeBSLObjectClient()
	for i := 0; i < 50; i++ {
		seedBackup(bsl, "default", fmt.Sprintf("b-%02d", i))
	}
	// Velero 已知前 10 个 → lister 应返回 40 个.
	knownNames := make([]string, 10)
	for i := 0; i < 10; i++ {
		knownNames[i] = fmt.Sprintf("b-%02d", i)
	}
	dedupe := &stubVeleroLister{known: map[string][]string{"default": knownNames}}
	l := NewS3BackupLister(bsl, dedupe)
	names, err := l.ListBackups(context.Background(), "default")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(names) != 40 {
		t.Fatalf("want 40 backups (50 - 10 known), got %d", len(names))
	}
	// 首个应是 b-10 (按 ASCII 排序; 10 < 11 < ... < 49).
	if names[0] != "b-10" {
		t.Errorf("expected first b-10, got %s", names[0])
	}
	if names[39] != "b-49" {
		t.Errorf("expected last b-49, got %s", names[39])
	}
}

func TestS3Lister_IgnoresStrayManifestFiles(t *testing.T) {
	// 防御性: 一些 BSL 可能有 "backups/loose-manifest.json" 这种异常 key
	// (e.g. admin 手放的文件), 我们的 lister 看 parent path, 应忽略.
	bsl := newFakeBSLObjectClient()
	bsl.objects[bsl.k("default", "backups/velero-backup.json")] = []byte("{}")
	bsl.objects[bsl.k("default", "loose/dir/velero-backup.json")] = []byte("{}")
	seedBackup(bsl, "default", "real")

	l := NewS3BackupLister(bsl, nil)
	names, err := l.ListBackups(context.Background(), "default")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(names) != 1 || names[0] != "real" {
		t.Fatalf("expected [real], got %v", names)
	}
}

func TestS3Lister_NilBSLClientReturnsError(t *testing.T) {
	l := &s3BackupLister{BSL: nil}
	_, err := l.ListBackups(context.Background(), "default")
	if err == nil {
		t.Fatalf("expected error from nil BSL client")
	}
}
