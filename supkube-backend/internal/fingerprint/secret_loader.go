// Package fingerprint: SecretLoader — pulls the 32-byte HMAC key from
// the K8s Secret `supkube-fingerprint-secret` in namespace supkube. The
// Helm chart (Agent E) is responsible for creating that Secret with
// `data.sharedSecret` = base64-encoded 32 random bytes.
//
// Caching
// ───────
// 60-second in-memory cache. Trade-off: secret rotation takes up to 60s
// to propagate to a running writer/validator, in exchange for sign and
// verify hot paths not hitting the apiserver. For an org rotating the
// secret, that's well below the import-flow timeout (5 minutes).
package fingerprint

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	secretNamespace = "supkube"
	secretName      = "supkube-fingerprint-secret"
	secretDataKey   = "sharedSecret"
)

// k8sSecretLoader is the production SecretLoader.
type k8sSecretLoader struct {
	k8sCli kubernetes.Interface

	mu     sync.RWMutex
	cached []byte
	at     time.Time
	ttl    time.Duration
}

// NewK8sSecretLoader builds a production SecretLoader. ttl=0 uses the
// package default (60s).
func NewK8sSecretLoader(k8sCli kubernetes.Interface) SecretLoader {
	return &k8sSecretLoader{k8sCli: k8sCli, ttl: secretCacheTTL}
}

func (l *k8sSecretLoader) GetSharedSecret(ctx context.Context) ([]byte, error) {
	l.mu.RLock()
	if l.cached != nil && time.Since(l.at) < l.ttl {
		out := make([]byte, len(l.cached))
		copy(out, l.cached)
		l.mu.RUnlock()
		return out, nil
	}
	l.mu.RUnlock()

	secret, err := l.k8sCli.CoreV1().Secrets(secretNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("fingerprint Secret %s/%s not found — Helm chart must create it before enabling fingerprints", secretNamespace, secretName)
		}
		return nil, fmt.Errorf("get fingerprint Secret: %w", err)
	}
	raw, ok := secret.Data[secretDataKey]
	if !ok {
		return nil, fmt.Errorf("fingerprint Secret missing key %q (expected base64-encoded 32 bytes)", secretDataKey)
	}
	// The value in K8s Secret data IS already base64-decoded by client-go
	// (Data is map[string][]byte; client-go decodes the on-wire base64).
	// But by SHARED-CONTRACT convention the value the operator pastes in
	// is itself base64 of 32 random bytes — so we decode AGAIN to get raw
	// key bytes. If decode fails, fall back to using the bytes as-is so a
	// well-meaning operator who pasted raw 32 bytes also works.
	decoded, derr := base64.StdEncoding.DecodeString(string(raw))
	keyBytes := raw
	if derr == nil && len(decoded) > 0 {
		keyBytes = decoded
	}

	l.mu.Lock()
	l.cached = make([]byte, len(keyBytes))
	copy(l.cached, keyBytes)
	l.at = time.Now()
	l.mu.Unlock()

	out := make([]byte, len(keyBytes))
	copy(out, keyBytes)
	return out, nil
}

// StaticSecretLoader is a fixed-bytes SecretLoader for tests and for the
// rare CLI path that wants to verify a fingerprint with a key from the
// operator's clipboard.
type StaticSecretLoader struct {
	Secret []byte
}

func (s *StaticSecretLoader) GetSharedSecret(ctx context.Context) ([]byte, error) {
	if len(s.Secret) == 0 {
		return nil, fmt.Errorf("StaticSecretLoader: empty secret")
	}
	out := make([]byte, len(s.Secret))
	copy(out, s.Secret)
	return out, nil
}
