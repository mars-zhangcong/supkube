package fingerprint

import (
	"context"
	"encoding/base64"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestK8sSecretLoader_HappyPath(t *testing.T) {
	raw := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	encoded := base64.StdEncoding.EncodeToString(raw)
	ks := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: secretNamespace},
		Data:       map[string][]byte{secretDataKey: []byte(encoded)},
	})
	l := NewK8sSecretLoader(ks)

	got, err := l.GetSharedSecret(context.Background())
	if err != nil {
		t.Fatalf("GetSharedSecret: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatalf("decoded mismatch: got %q want %q", got, raw)
	}
}

func TestK8sSecretLoader_NotFound(t *testing.T) {
	ks := fake.NewSimpleClientset()
	l := NewK8sSecretLoader(ks)
	if _, err := l.GetSharedSecret(context.Background()); err == nil {
		t.Fatalf("expected NotFound error")
	}
}

func TestK8sSecretLoader_MissingKey(t *testing.T) {
	ks := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: secretNamespace},
		Data:       map[string][]byte{"wrong-key": []byte("x")},
	})
	l := NewK8sSecretLoader(ks)
	if _, err := l.GetSharedSecret(context.Background()); err == nil {
		t.Fatalf("expected error on missing key")
	}
}

func TestK8sSecretLoader_RawFallback(t *testing.T) {
	// Operator pasted raw bytes (not base64) — we should fall back to using them.
	raw := []byte("!!!!!notbase64atallreally!!!!")
	ks := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: secretNamespace},
		Data:       map[string][]byte{secretDataKey: raw},
	})
	l := NewK8sSecretLoader(ks)
	got, err := l.GetSharedSecret(context.Background())
	if err != nil {
		t.Fatalf("GetSharedSecret: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("should have returned something")
	}
}

func TestStaticSecretLoader_Empty(t *testing.T) {
	s := &StaticSecretLoader{}
	if _, err := s.GetSharedSecret(context.Background()); err == nil {
		t.Fatalf("expected error on empty static secret")
	}
}
