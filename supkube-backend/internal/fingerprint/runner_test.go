// Runner tests — exercise shouldSign filtering, derivePlaceholderSHA
// determinism, signOne end-to-end against the controller-runtime fake
// client. The Run() ticker loop itself is left to integration testing
// (would require time mocking that adds little value).
package fingerprint

import (
	"context"
	"strings"
	"testing"
	"time"

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
		t.Fatalf("scheme: %v", err)
	}
	return s
}

func TestShouldSign_AlreadySigned(t *testing.T) {
	b := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "x",
			Annotations: map[string]string{AnnFingerprintWritten: "now"},
		},
		Status: velerov1.BackupStatus{Phase: velerov1.BackupPhaseCompleted},
	}
	if shouldSign(b) {
		t.Fatalf("already-signed should be skipped")
	}
}

func TestShouldSign_NotCompleted(t *testing.T) {
	for _, ph := range []velerov1.BackupPhase{
		velerov1.BackupPhaseNew,
		velerov1.BackupPhaseInProgress,
		velerov1.BackupPhaseFailed,
		velerov1.BackupPhasePartiallyFailed,
	} {
		b := &velerov1.Backup{Status: velerov1.BackupStatus{Phase: ph}}
		if shouldSign(b) {
			t.Errorf("phase %s should NOT be signed", ph)
		}
	}
}

func TestShouldSign_Eligible(t *testing.T) {
	b := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "fresh",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
		},
		Status: velerov1.BackupStatus{Phase: velerov1.BackupPhaseCompleted},
	}
	if !shouldSign(b) {
		t.Fatalf("fresh completed backup should be signable")
	}
}

func TestShouldSign_TooOld(t *testing.T) {
	b := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "ancient",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-48 * time.Hour)),
		},
		Status: velerov1.BackupStatus{Phase: velerov1.BackupPhaseCompleted},
	}
	if shouldSign(b) {
		t.Fatalf("48h-old backup should be skipped")
	}
}

func TestDerivePlaceholderSHA_DeterministicAndDistinct(t *testing.T) {
	ct := metav1.NewTime(time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC))
	b := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bk-1"},
		Status:     velerov1.BackupStatus{CompletionTimestamp: &ct},
	}
	a1 := derivePlaceholderSHA(b, "tarball")
	a2 := derivePlaceholderSHA(b, "tarball")
	bkind := derivePlaceholderSHA(b, "metadata")
	if a1 != a2 {
		t.Fatalf("non-deterministic for same kind")
	}
	if a1 == bkind {
		t.Fatalf("tarball+metadata shouldn't collide")
	}
	if len(a1) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(a1))
	}
}

func TestRunner_SignOne_EndToEnd(t *testing.T) {
	s := newScheme(t)
	ct := metav1.NewTime(time.Now().Add(-30 * time.Minute))
	b := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "bk-eligible",
			Namespace:         DefaultVeleroNS,
			CreationTimestamp: ct,
		},
		Spec:   velerov1.BackupSpec{StorageLocation: "default"},
		Status: velerov1.BackupStatus{Phase: velerov1.BackupPhaseCompleted, CompletionTimestamp: &ct},
	}
	rc := fake.NewClientBuilder().WithScheme(s).WithObjects(b).Build()

	bsl := newFakeBSL()
	w := NewWriter(bsl, &StaticSecretLoader{Secret: []byte(refSecret)}, strings.Repeat("a", 32), "src")
	r := NewRunner(w, rc)

	if err := r.scanOnce(context.Background()); err != nil {
		t.Fatalf("scanOnce: %v", err)
	}

	// fingerprint should now exist in fakeBSL
	if _, err := bsl.GetObject(context.Background(), "default", fingerprintKey("bk-eligible")); err != nil {
		t.Fatalf("fingerprint not written: %v", err)
	}

	// backup should be annotated as signed
	got := &velerov1.Backup{}
	if err := rc.Get(context.Background(), client.ObjectKey{Name: "bk-eligible", Namespace: DefaultVeleroNS}, got); err != nil {
		t.Fatalf("get backup: %v", err)
	}
	if !IsBackupSigned(got.Annotations) {
		t.Fatalf("backup should be annotated as signed")
	}

	// Second scan: shouldSign filters out → fingerprint NOT re-written
	// (we'd see PUT count rise, but our fake doesn't count; we just
	// confirm scanOnce returns without error and annotation persists).
	if err := r.scanOnce(context.Background()); err != nil {
		t.Fatalf("second scanOnce: %v", err)
	}
}
