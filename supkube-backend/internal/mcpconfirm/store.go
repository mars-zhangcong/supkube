// Package mcpconfirm persists MCP HitL confirmation snapshots (ADR-057 R1) as
// K8s Secrets. Secrets are etcd-backed namespaced objects → genuinely
// cross-replica (DoD#6: kill a pod, another replica still resolves the
// confirm_id), reuse the backend's existing runtime client + corev1 scheme
// (zero new CRD, zero new middleware), and are encrypted at rest when etcd
// encryption is on (snapshots may carry sensitive write-op inputs → Secret, not
// ConfigMap).
//
// The mcp-server stays a pure backend client (PRD-004 §4.1 thin adapter): it
// never touches K8s — it calls this store through the backend's /mcp/confirmations
// API. The protocol (dry-run → confirm_id → second call uses the snapshot,
// ignores 2nd inputs, one-shot, 5-min TTL) lives in the mcp-server; this is only
// the persistence.
package mcpconfirm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultTTL    = 5 * time.Minute
	namePrefix    = "mcp-confirm-"
	managedByKey  = "app.kubernetes.io/managed-by"
	managedByVal  = "supkube-mcp-confirm"
	expiresAtAnno = "supkube.io/expires-at"
	dataKey       = "snapshot"
)

// Snapshot is the persisted confirmation (skill + locked inputs + dry-run).
type Snapshot struct {
	Skill  string         `json:"skill"`
	Args   map[string]any `json:"args"`
	DryRun any            `json:"dryRun,omitempty"`
}

type Store struct {
	cl  client.Client
	ns  string
	ttl time.Duration
	now func() time.Time // injectable for tests
}

func New(cl client.Client, namespace string, ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Store{cl: cl, ns: namespace, ttl: ttl, now: time.Now}
}

// Put creates a Secret holding the snapshot and returns its confirm_id.
func (s *Store) Put(ctx context.Context, snap Snapshot) (string, error) {
	raw, err := json.Marshal(snap)
	if err != nil {
		return "", err
	}
	id := newID()
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        namePrefix + id,
			Namespace:   s.ns,
			Labels:      map[string]string{managedByKey: managedByVal},
			Annotations: map[string]string{expiresAtAnno: s.now().Add(s.ttl).UTC().Format(time.RFC3339)},
		},
		Data: map[string][]byte{dataKey: raw},
	}
	if err := s.cl.Create(ctx, sec); err != nil {
		return "", err
	}
	return id, nil
}

// Get returns the snapshot for id. Expired entries are deleted and reported as
// missing (ok=false), so an expired confirm can never execute.
func (s *Store) Get(ctx context.Context, id string) (Snapshot, bool, error) {
	var sec corev1.Secret
	err := s.cl.Get(ctx, types.NamespacedName{Namespace: s.ns, Name: namePrefix + id}, &sec)
	if apierrors.IsNotFound(err) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, err
	}
	if s.expired(&sec) {
		_ = s.Delete(ctx, id)
		return Snapshot{}, false, nil
	}
	var snap Snapshot
	if err := json.Unmarshal(sec.Data[dataKey], &snap); err != nil {
		return Snapshot{}, false, err
	}
	return snap, true, nil
}

// Delete removes the confirmation (one-shot consume / expiry GC). NotFound is ok.
func (s *Store) Delete(ctx context.Context, id string) error {
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: namePrefix + id, Namespace: s.ns}}
	if err := s.cl.Delete(ctx, sec); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// Reap deletes all expired confirmation Secrets and returns how many it removed.
func (s *Store) Reap(ctx context.Context) (int, error) {
	var list corev1.SecretList
	if err := s.cl.List(ctx, &list, client.InNamespace(s.ns), client.MatchingLabels{managedByKey: managedByVal}); err != nil {
		return 0, err
	}
	n := 0
	for i := range list.Items {
		if s.expired(&list.Items[i]) {
			if err := s.cl.Delete(ctx, &list.Items[i]); err == nil || apierrors.IsNotFound(err) {
				n++
			}
		}
	}
	return n, nil
}

func (s *Store) expired(sec *corev1.Secret) bool {
	exp, err := time.Parse(time.RFC3339, sec.Annotations[expiresAtAnno])
	if err != nil {
		return true // unparseable expiry → treat as expired (fail-safe)
	}
	return s.now().After(exp)
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
