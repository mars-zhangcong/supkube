// Package fingerprint: TrustStore — durable record of which source clusters
// have ever produced a valid fingerprint on this destination. Backing store:
// a single ConfigMap `supkube-fingerprint-truststore` in namespace supkube.
//
// Why a ConfigMap (vs. a CRD or external DB)
// ──────────────────────────────────────────
// The truststore is "append + lookup" — no complex queries, no RBAC slicing,
// no per-entry watches. A ConfigMap holds ~1MB of data; at ~150 bytes/entry
// that's ~6000 source clusters before we'd need to shard. We're not going
// to have 6000 sources in v1. Lifting to a CRD is a one-day refactor when
// the truststore admin UI lands in v1.x.
//
// Trust-on-first-use (TOFU) policy
// ────────────────────────────────
// v1 binds automatically on the first valid HMAC verification — same model
// as SSH known_hosts. The admin UI in v1.x will add explicit "rotate
// signing key for cluster X" / "revoke cluster Y" actions; for now,
// kubectl edit cm supkube-fingerprint-truststore is the escape hatch.
//
// Concurrency
// ───────────
// Bind() uses an optimistic Update-with-RetryOnConflict pattern. List/IsBound
// hit the local informer cache (cheap). The whole operation is keyed by
// sourceClusterID so concurrent imports of different sources never contend.
package fingerprint

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	truststoreNamespace = "supkube"
	truststoreName      = "supkube-fingerprint-truststore"
	// Single key whose value is a JSON map[sourceClusterID]TrustEntry.
	// Keeping ONE key (vs. one per cluster) avoids the "ConfigMap is full
	// of stale ghost keys after rebinds" problem and makes the whole
	// state visible in a single `kubectl get cm ... -o yaml`.
	truststoreDataKey = "entries.json"
)

// TrustEntry is one row in the truststore. Time is RFC3339 for human
// readability in `kubectl get cm`.
type TrustEntry struct {
	SourceClusterID   string `json:"sourceClusterID"`
	SourceClusterName string `json:"sourceClusterName"`
	BoundAt           string `json:"boundAt"`
}

// TrustStore is the public surface — see Bind / IsBound / List.
type TrustStore struct {
	k8sCli kubernetes.Interface

	// mu protects the LOCAL read cache only. K8s is still the source
	// of truth; the cache is just a 30s hot-path optimization so a
	// burst of imports doesn't hammer the API server.
	mu       sync.RWMutex
	cache    map[string]TrustEntry
	cacheAt  time.Time
	cacheTTL time.Duration
}

// NewTrustStore constructs a TrustStore over an existing K8s client.
// First call seeds the cache lazily.
func NewTrustStore(k8sCli kubernetes.Interface) *TrustStore {
	return &TrustStore{
		k8sCli:   k8sCli,
		cacheTTL: 30 * time.Second,
	}
}

// Bind records that sourceClusterID has produced a valid fingerprint.
// Idempotent: re-binding an already-bound cluster updates the boundAt
// timestamp (useful for "last seen" telemetry) but doesn't error.
//
// Returns nil on success. Returns an error only on K8s API failures —
// callers should NOT block the import on Bind() failure (log + carry on).
func (t *TrustStore) Bind(ctx context.Context, sourceClusterID, sourceClusterName string, boundAt time.Time) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cm, err := t.fetchOrInit(ctx)
		if err != nil {
			return err
		}
		entries, err := decodeEntries(cm)
		if err != nil {
			return err
		}
		entries[sourceClusterID] = TrustEntry{
			SourceClusterID:   sourceClusterID,
			SourceClusterName: sourceClusterName,
			BoundAt:           boundAt.UTC().Format(time.RFC3339),
		}
		if err := encodeEntries(cm, entries); err != nil {
			return err
		}
		_, err = t.k8sCli.CoreV1().ConfigMaps(truststoreNamespace).Update(ctx, cm, metav1.UpdateOptions{})
		if err == nil {
			t.invalidateCache()
		}
		return err
	})
}

// IsBound: cheap presence check. Hits the local cache when fresh.
func (t *TrustStore) IsBound(ctx context.Context, sourceClusterID string) bool {
	entries := t.read(ctx)
	_, ok := entries[sourceClusterID]
	return ok
}

// List returns all entries sorted by BoundAt (newest first). The admin UI
// in v1.x calls this; the controller's normal hot path uses IsBound.
func (t *TrustStore) List(ctx context.Context) []TrustEntry {
	entries := t.read(ctx)
	out := make([]TrustEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].BoundAt > out[j].BoundAt
	})
	return out
}

// ─────────────────────────────────────────────────────────────────────
// Internals
// ─────────────────────────────────────────────────────────────────────

func (t *TrustStore) fetchOrInit(ctx context.Context) (*corev1.ConfigMap, error) {
	cm, err := t.k8sCli.CoreV1().ConfigMaps(truststoreNamespace).Get(ctx, truststoreName, metav1.GetOptions{})
	if err == nil {
		return cm, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("get truststore cm: %w", err)
	}
	// Create from scratch. Reasonably common in fresh installs.
	cm = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      truststoreName,
			Namespace: truststoreNamespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "supkube-fingerprint"},
		},
		Data: map[string]string{truststoreDataKey: "{}"},
	}
	created, err := t.k8sCli.CoreV1().ConfigMaps(truststoreNamespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		// Race: another replica created it between our Get and Create.
		// Re-Get and return that.
		if apierrors.IsAlreadyExists(err) {
			return t.k8sCli.CoreV1().ConfigMaps(truststoreNamespace).Get(ctx, truststoreName, metav1.GetOptions{})
		}
		return nil, fmt.Errorf("create truststore cm: %w", err)
	}
	return created, nil
}

func decodeEntries(cm *corev1.ConfigMap) (map[string]TrustEntry, error) {
	raw, ok := cm.Data[truststoreDataKey]
	if !ok || raw == "" {
		return map[string]TrustEntry{}, nil
	}
	entries := map[string]TrustEntry{}
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("decode truststore entries: %w", err)
	}
	return entries, nil
}

func encodeEntries(cm *corev1.ConfigMap, entries map[string]TrustEntry) error {
	body, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode truststore entries: %w", err)
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data[truststoreDataKey] = string(body)
	return nil
}

// read returns a snapshot of the entries, using the cache when fresh.
// On cache miss, a stale snapshot is returned if K8s is unreachable —
// IsBound() should fail-open (return true) only when the caller knows
// it's OK; the validator does NOT consult IsBound() in v1's hot path
// (TOFU = bind silently on first valid signature), so cache staleness
// affects only the admin List endpoint.
func (t *TrustStore) read(ctx context.Context) map[string]TrustEntry {
	t.mu.RLock()
	if t.cache != nil && time.Since(t.cacheAt) < t.cacheTTL {
		defer t.mu.RUnlock()
		out := make(map[string]TrustEntry, len(t.cache))
		for k, v := range t.cache {
			out[k] = v
		}
		return out
	}
	t.mu.RUnlock()

	cm, err := t.k8sCli.CoreV1().ConfigMaps(truststoreNamespace).Get(ctx, truststoreName, metav1.GetOptions{})
	if err != nil {
		// Cache miss + K8s unreachable. Return last known (possibly stale).
		t.mu.RLock()
		defer t.mu.RUnlock()
		out := make(map[string]TrustEntry, len(t.cache))
		for k, v := range t.cache {
			out[k] = v
		}
		return out
	}
	entries, err := decodeEntries(cm)
	if err != nil {
		return map[string]TrustEntry{}
	}
	t.mu.Lock()
	t.cache = entries
	t.cacheAt = time.Now()
	t.mu.Unlock()
	out := make(map[string]TrustEntry, len(entries))
	for k, v := range entries {
		out[k] = v
	}
	return out
}

func (t *TrustStore) invalidateCache() {
	t.mu.Lock()
	t.cache = nil
	t.cacheAt = time.Time{}
	t.mu.Unlock()
}
