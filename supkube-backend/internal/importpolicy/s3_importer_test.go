// s3_importer_test.go — 覆盖 s3BackupImporter 的 import 路径 + 幂等 +
// label merge + race handling.
package importpolicy

import (
	"context"
	"encoding/json"
	"testing"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// seedManifest 在 fake BSL 写一份合法 Velero Backup manifest JSON.
func seedManifest(t *testing.T, bsl *fakeBSLObjectClient, bslName, backupName, sourceSL string) {
	t.Helper()
	src := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:            backupName,
			Namespace:       "velero",
			ResourceVersion: "12345",                                     // 应被清掉
			UID:             types.UID("11111111-2222-3333-4444-555555"), // 应被清掉
			Labels:          map[string]string{"velero.io/storage-location": sourceSL},
		},
		Spec: velerov1.BackupSpec{
			StorageLocation:    sourceSL,
			IncludedNamespaces: []string{"app-ns"},
		},
	}
	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	key := "backups/" + backupName + "/velero-backup.json"
	bsl.objects[bsl.k(bslName, key)] = raw
}

func TestS3Importer_CreatesCRFromManifest(t *testing.T) {
	bsl := newFakeBSLObjectClient()
	seedManifest(t, bsl, "default", "daily-1", "default")

	sch := newVeleroScheme(t)
	cli := fake.NewClientBuilder().WithScheme(sch).Build()
	im := NewS3BackupImporter(bsl, cli)

	labels := map[string]string{
		LabelImported:          "true",
		LabelSourceCluster:     "src-cluster-1",
		LabelFingerprintStatus: "valid",
	}
	if err := im.ImportBackup(context.Background(), "velero", "default", "daily-1", labels); err != nil {
		t.Fatalf("import: %v", err)
	}
	// 验证 CR 存在 + 字段对齐.
	got := &velerov1.Backup{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: "velero", Name: "daily-1"}, got); err != nil {
		t.Fatalf("get after import: %v", err)
	}
	if got.Spec.StorageLocation != "default" {
		t.Errorf("storageLocation: want default, got %q", got.Spec.StorageLocation)
	}
	if got.Labels[LabelImported] != "true" {
		t.Errorf("missing imported label")
	}
	if got.Labels[LabelFingerprintStatus] != "valid" {
		t.Errorf("missing fingerprint-status label, labels=%v", got.Labels)
	}
	// 源 backup 自己的 label 应保留 (merge 不丢).
	if got.Labels["velero.io/storage-location"] != "default" {
		t.Errorf("expected source label preserved, got %v", got.Labels)
	}
	// resourceVersion / UID 应被清掉 — fake client 会 reset, 但我们也确认
	// IncludedNamespaces 没丢 (说明 Spec 整段被搬过来).
	if len(got.Spec.IncludedNamespaces) != 1 || got.Spec.IncludedNamespaces[0] != "app-ns" {
		t.Errorf("expected spec carried over, got %v", got.Spec.IncludedNamespaces)
	}
}

func TestS3Importer_AlreadyExistsIsIdempotent(t *testing.T) {
	bsl := newFakeBSLObjectClient()
	seedManifest(t, bsl, "default", "race-1", "default")

	sch := newVeleroScheme(t)
	// 先 prep 一个已经存在的 Backup CR (模拟 Velero sync 抢先创建了).
	existing := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "race-1", Namespace: "velero"},
		Spec:       velerov1.BackupSpec{StorageLocation: "default"},
	}
	cli := fake.NewClientBuilder().WithScheme(sch).WithObjects(existing).Build()
	im := NewS3BackupImporter(bsl, cli)

	// Import 应返回 nil (race-safe), 不应 panic 也不应 overwrite.
	labels := map[string]string{LabelImported: "true"}
	if err := im.ImportBackup(context.Background(), "velero", "default", "race-1", labels); err != nil {
		t.Fatalf("expected nil on AlreadyExists, got %v", err)
	}
	// 已存在的 CR 不应被加 SupKube label (我们 race 输了, 不 overwrite).
	got := &velerov1.Backup{}
	_ = cli.Get(context.Background(), types.NamespacedName{Namespace: "velero", Name: "race-1"}, got)
	if got.Labels[LabelImported] == "true" {
		t.Errorf("AlreadyExists path should NOT overwrite labels; got %v", got.Labels)
	}
}

func TestS3Importer_MissingManifestReturnsError(t *testing.T) {
	bsl := newFakeBSLObjectClient() // 啥都没写
	sch := newVeleroScheme(t)
	cli := fake.NewClientBuilder().WithScheme(sch).Build()
	im := NewS3BackupImporter(bsl, cli)
	err := im.ImportBackup(context.Background(), "velero", "default", "ghost", map[string]string{})
	if err == nil {
		t.Fatalf("expected error when manifest missing, got nil")
	}
}

func TestS3Importer_BadManifestReturnsError(t *testing.T) {
	bsl := newFakeBSLObjectClient()
	bsl.objects[bsl.k("default", "backups/broken/velero-backup.json")] = []byte("{not json")
	sch := newVeleroScheme(t)
	cli := fake.NewClientBuilder().WithScheme(sch).Build()
	im := NewS3BackupImporter(bsl, cli)
	if err := im.ImportBackup(context.Background(), "velero", "default", "broken", nil); err == nil {
		t.Fatalf("expected decode error, got nil")
	}
}

func TestS3Importer_NilDepsReturnsError(t *testing.T) {
	im := &s3BackupImporter{}
	if err := im.ImportBackup(context.Background(), "velero", "default", "x", nil); err == nil {
		t.Fatalf("expected error when BSL or Cli is nil")
	}
}

func TestMergeLabels(t *testing.T) {
	// 空 + 空 → nil
	if got := mergeLabels(nil, nil); got != nil {
		t.Errorf("want nil, got %v", got)
	}
	// src only
	if got := mergeLabels(map[string]string{"a": "1"}, nil); got["a"] != "1" {
		t.Errorf("src-only merge lost data: %v", got)
	}
	// overlay (add wins)
	got := mergeLabels(map[string]string{"a": "1", "b": "2"}, map[string]string{"a": "OVERRIDE", "c": "3"})
	if got["a"] != "OVERRIDE" || got["b"] != "2" || got["c"] != "3" {
		t.Errorf("merge incorrect: %v", got)
	}
}
