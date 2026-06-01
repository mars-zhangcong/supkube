// Tests for Writer. End-to-end: Writer.WriteForBackup → fake BSL → Validator
// recovers a Valid result. This is the round-trip integration test that
// catches any sign/verify drift.
package fingerprint

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func TestWriter_RoundTripWithValidator(t *testing.T) {
	bsl := newFakeBSL()
	loader := &StaticSecretLoader{Secret: []byte(refSecret)}

	w := NewWriter(bsl, loader, strings.Repeat("c", 32), "src-cp-node")
	if err := w.WriteForBackup(context.Background(), "bsl-x", "daily-1", strings.Repeat("7", 64), strings.Repeat("8", 64)); err != nil {
		t.Fatalf("WriteForBackup: %v", err)
	}

	// Same secret, same Validator → Valid.
	v := NewValidator(bsl, loader)
	res, err := v.ValidateBackup(context.Background(), "bsl-x", "daily-1", ModeEnforce)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if res.Status != StatusValid {
		t.Fatalf("expected StatusValid, got %s (reason=%s)", res.Status, res.Reason)
	}

	// Inspect the stored bytes — verify shape matches contract.
	raw, _ := bsl.GetObject(context.Background(), "bsl-x", fingerprintKey("daily-1"))
	fp := &Fingerprint{}
	if err := json.Unmarshal(raw, fp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if fp.Version != Version {
		t.Errorf("version: got %q want %q", fp.Version, Version)
	}
	if fp.Algo != Algo {
		t.Errorf("algo: got %q want %q", fp.Algo, Algo)
	}
	if fp.BackupName != "daily-1" {
		t.Errorf("backupName: got %q", fp.BackupName)
	}
	if fp.BackupNamespace != DefaultVeleroNS {
		t.Errorf("backupNamespace: got %q", fp.BackupNamespace)
	}
	if len(fp.HMACSHA256) != 64 {
		t.Errorf("HMAC hex length: got %d", len(fp.HMACSHA256))
	}
	if fp.CreatedAt == "" {
		t.Errorf("CreatedAt should be populated")
	}
}

func TestWriter_MissingTarballSHA_Refused(t *testing.T) {
	bsl := newFakeBSL()
	w := NewWriter(bsl, &StaticSecretLoader{Secret: []byte(refSecret)}, strings.Repeat("a", 32), "x")
	if err := w.WriteForBackup(context.Background(), "bsl-x", "n", "", ""); err == nil {
		t.Fatalf("expected error on empty tarballSHA256")
	}
}

func TestWriter_EmptyClusterID_Refused(t *testing.T) {
	bsl := newFakeBSL()
	w := NewWriter(bsl, &StaticSecretLoader{Secret: []byte(refSecret)}, "", "x")
	if err := w.WriteForBackup(context.Background(), "bsl-x", "n", strings.Repeat("a", 64), ""); err == nil {
		t.Fatalf("expected error on empty cluster ID")
	}
}

func TestWriter_ShortSecret_Refused(t *testing.T) {
	bsl := newFakeBSL()
	w := NewWriter(bsl, &StaticSecretLoader{Secret: []byte("short")}, strings.Repeat("a", 32), "x")
	if err := w.WriteForBackup(context.Background(), "bsl-x", "n", strings.Repeat("a", 64), ""); err == nil {
		t.Fatalf("expected error on too-short shared secret")
	}
}

func TestStripDashes(t *testing.T) {
	if got := stripDashes("12345678-90ab-cdef-1234-567890abcdef"); got != "1234567890abcdef1234567890abcdef" {
		t.Fatalf("stripDashes: got %q", got)
	}
}

func TestIsBackupSignedAndMark(t *testing.T) {
	if IsBackupSigned(nil) {
		t.Fatal("nil should not be signed")
	}
	a := MarkBackupSigned(nil)
	if !IsBackupSigned(a) {
		t.Fatal("after MarkBackupSigned, should report signed")
	}
}

func TestResolveClusterIdentity_HappyPath(t *testing.T) {
	ks := fake.NewSimpleClientset(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kube-system",
				UID:  k8stypes.UID("12345678-90ab-cdef-1234-567890abcdef"),
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "cp-1",
				Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
			},
		},
	)
	id, name := ResolveClusterIdentity(context.Background(), ks)
	if id != "1234567890abcdef1234567890abcdef" {
		t.Errorf("expected dashes stripped, got %q", id)
	}
	if name != "cp-1" {
		t.Errorf("expected control-plane node name, got %q", name)
	}
}

func TestResolveClusterIdentity_NoNodes(t *testing.T) {
	ks := fake.NewSimpleClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-system", UID: k8stypes.UID("abc")},
	})
	_, name := ResolveClusterIdentity(context.Background(), ks)
	if name != "Local Cluster" {
		t.Errorf("expected fallback, got %q", name)
	}
}

func TestResolveClusterIdentity_NilClient(t *testing.T) {
	id, name := ResolveClusterIdentity(context.Background(), nil)
	if id != "" {
		t.Errorf("expected empty id, got %q", id)
	}
	if name != "Local Cluster" {
		t.Errorf("expected fallback name, got %q", name)
	}
}

func TestSourceCorev1Secret(t *testing.T) {
	s := SourceCorev1Secret("ns", "n")
	if s.Name != "n" || s.Namespace != "ns" {
		t.Errorf("wrong meta: %+v", s.ObjectMeta)
	}
	if s.Type != corev1.SecretTypeOpaque {
		t.Errorf("expected Opaque type, got %v", s.Type)
	}
}

func TestFingerprintKey_Layout(t *testing.T) {
	got := fingerprintKey("my-backup")
	want := "backups/my-backup/" + FilenameInBSL
	if got != want {
		t.Fatalf("fingerprintKey: got %q want %q", got, want)
	}
}
