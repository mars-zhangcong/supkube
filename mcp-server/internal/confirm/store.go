// Package confirm is the HitL (human-in-the-loop) confirmation store for write
// skills, per ADR-057. A write skill's first call returns a dry-run + confirm_id;
// the snapshot (skill + normalized inputs + dry-run) is persisted; the second
// call presents only the confirm_id and the snapshot's inputs are used (the
// second call's inputs are IGNORED — anti LLM-hallucination/tamper). 5-min TTL.
//
// Store is the swap seam (ADR-057): production = K8s CR (cross-replica via etcd,
// no new infra); this Memory impl is for PoC/tests (single-replica).
package confirm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const DefaultTTL = 5 * time.Minute

type Snapshot struct {
	Skill   string
	Args    map[string]any
	DryRun  any
	Created time.Time
	Expires time.Time
}

// Store persists confirmation snapshots. ctx+error so a network-backed impl
// (BackendStore) can report failures; Memory never errors.
type Store interface {
	Put(ctx context.Context, s Snapshot) (string, error)        // returns confirm_id
	Get(ctx context.Context, id string) (Snapshot, bool, error) // ok=false if missing or expired
	Delete(ctx context.Context, id string) error
}

// Memory is the single-replica PoC/test store. Production uses BackendStore
// (backend-persisted K8s Secret, cross-replica) per ADR-057 R1.
type Memory struct {
	mu  sync.Mutex
	m   map[string]Snapshot
	ttl time.Duration
	now func() time.Time // injectable for tests
}

func NewMemory() *Memory { return &Memory{m: map[string]Snapshot{}, ttl: DefaultTTL, now: time.Now} }

func (s *Memory) Put(_ context.Context, snap Snapshot) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if snap.Created.IsZero() {
		snap.Created = now
	}
	if snap.Expires.IsZero() {
		snap.Expires = now.Add(s.ttl)
	}
	id := newID()
	s.m[id] = snap
	return id, nil
}

func (s *Memory) Get(_ context.Context, id string) (Snapshot, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.m[id]
	if !ok {
		return Snapshot{}, false, nil
	}
	if s.now().After(snap.Expires) { // expired → GC + miss
		delete(s.m, id)
		return Snapshot{}, false, nil
	}
	return snap, true, nil
}

func (s *Memory) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
