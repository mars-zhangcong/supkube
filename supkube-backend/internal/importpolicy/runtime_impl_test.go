// runtime_impl_test.go — 给 BackupLister/Importer 的 Velero 实现做最小覆盖.
//
// 用 controller-runtime 的 fake client (Velero scheme 已注册 — 复用
// internal/backupcopy 的 newScheme 模式).
package importpolicy

import (
	"context"
	"testing"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newVeleroScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := velerov1.AddToScheme(s); err != nil {
		t.Fatalf("add velero scheme: %v", err)
	}
	return s
}

func mkBackup(name, sl string) *velerov1.Backup {
	return &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "velero"},
		Spec:       velerov1.BackupSpec{StorageLocation: sl},
	}
}

func TestVeleroLister_FiltersByBSL(t *testing.T) {
	sch := newVeleroScheme(t)
	cli := fake.NewClientBuilder().WithScheme(sch).WithObjects(
		mkBackup("a", "default"),
		mkBackup("b", "secondary"),
		mkBackup("c", "default"),
		mkBackup("d", ""), // empty = default
	).Build()
	l := NewVeleroBackupLister(cli)
	names, err := l.ListBackups(context.Background(), "default")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{"a", "c", "d"}
	if len(names) != len(want) {
		t.Fatalf("want %v, got %v", want, names)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("at %d want %s, got %s", i, n, names[i])
		}
	}
}

func TestVeleroImporter_CreatesAndPatches(t *testing.T) {
	sch := newVeleroScheme(t)
	cli := fake.NewClientBuilder().WithScheme(sch).Build()
	im := NewVeleroBackupImporter(cli)

	labels := map[string]string{LabelImported: "true", LabelSourceCluster: "src-1"}
	if err := im.ImportBackup(context.Background(), "velero", "default", "new-backup", labels); err != nil {
		t.Fatalf("import (create): %v", err)
	}
	// Verify CR exists with labels.
	got := &velerov1.Backup{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: "velero", Name: "new-backup"}, got); err != nil {
		t.Fatalf("get after create: %v", err)
	}
	if got.Labels[LabelImported] != "true" {
		t.Errorf("missing imported label")
	}
	if got.Spec.StorageLocation != "default" {
		t.Errorf("want storageLocation=default, got %q", got.Spec.StorageLocation)
	}

	// Second call (already exists) should patch labels without error.
	moreLabels := map[string]string{LabelFingerprintStatus: "valid"}
	if err := im.ImportBackup(context.Background(), "velero", "default", "new-backup", moreLabels); err != nil {
		t.Fatalf("import (patch): %v", err)
	}
	got = &velerov1.Backup{}
	_ = cli.Get(context.Background(), types.NamespacedName{Namespace: "velero", Name: "new-backup"}, got)
	if got.Labels[LabelFingerprintStatus] != "valid" {
		t.Errorf("patch should add fingerprint-status label, got %v", got.Labels)
	}
	if got.Labels[LabelImported] != "true" {
		t.Errorf("patch should preserve imported label, got %v", got.Labels)
	}
}

func TestDeepCopy(t *testing.T) {
	p := &ImportPolicy{
		Spec: ImportPolicySpec{
			SourceBSL:       "bsl-A",
			Mode:            ImportModeContinuous,
			SourceClusterID: "cluster-1",
		},
		Status: ImportPolicyStatus{Phase: PhaseActive, ImportedCount: 5},
	}
	p.Name = "deepy"
	cp := p.DeepCopy()
	if cp == p {
		t.Errorf("DeepCopy must return a new pointer")
	}
	if cp.Spec.SourceBSL != "bsl-A" || cp.Status.ImportedCount != 5 {
		t.Errorf("copy lost data: %+v", cp)
	}
	// mutate copy, original should be untouched.
	cp.Spec.SourceBSL = "mutated"
	if p.Spec.SourceBSL != "bsl-A" {
		t.Errorf("mutation of copy leaked to original")
	}

	// DeepCopyObject + List variant — exercises runtime.Object plumbing.
	obj := p.DeepCopyObject()
	if obj == nil {
		t.Errorf("DeepCopyObject returned nil")
	}
	list := &ImportPolicyList{Items: []ImportPolicy{*p}}
	lc := list.DeepCopy()
	if &lc.Items[0] == &list.Items[0] {
		t.Errorf("list deep copy should produce new slice elements")
	}
	lObj := list.DeepCopyObject()
	if lObj == nil {
		t.Errorf("list DeepCopyObject returned nil")
	}

	// nil receivers should not panic.
	var nilPol *ImportPolicy
	if x := nilPol.DeepCopy(); x != nil {
		t.Errorf("nil DeepCopy should return nil")
	}
	var nilList *ImportPolicyList
	if x := nilList.DeepCopy(); x != nil {
		t.Errorf("nil list DeepCopy should return nil")
	}

	// Confirm client.Object compatibility (compile-time only).
	var _ client.Object = &ImportPolicy{}
}
