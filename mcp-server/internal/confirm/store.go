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

type Store interface {
	Put(s Snapshot) string          // returns confirm_id
	Get(id string) (Snapshot, bool) // false if missing or expired
	Delete(id string)
}

// Memory is the PoC/test store. Production swaps in a CR-backed impl (ADR-057).
type Memory struct {
	mu  sync.Mutex
	m   map[string]Snapshot
	ttl time.Duration
	now func() time.Time // injectable for tests
}

func NewMemory() *Memory { return &Memory{m: map[string]Snapshot{}, ttl: DefaultTTL, now: time.Now} }

func (s *Memory) Put(snap Snapshot) string {
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
	return id
}

func (s *Memory) Get(id string) (Snapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.m[id]
	if !ok {
		return Snapshot{}, false
	}
	if s.now().After(snap.Expires) { // expired → GC + miss
		delete(s.m, id)
		return Snapshot{}, false
	}
	return snap, true
}

func (s *Memory) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
