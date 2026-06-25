// Package events is Supkube's in-process pub/sub bus (no broker — see ADR/
// handoff rationale: SSE + in-process bus is enough; broker only on real
// durable-fanout need; this Publish/Subscribe surface is the swap seam).
//
// Producers: REST create handlers (*.created) + eventwatch (terminal
// *.completed/*.failed from Velero CR transitions). Consumers: the SSE stream
// (GET /api/v1/events) that SupInsight / the MCP server subscribe to.
package events

import (
	"sync"
	"time"
)

// Event types — the contract agents subscribe to. Keep stable.
const (
	TypeBackupCreated    = "backup.created"
	TypeBackupCompleted  = "backup.completed"
	TypeBackupFailed     = "backup.failed"
	TypeRestoreCreated   = "restore.created"
	TypeRestoreCompleted = "restore.completed"
	TypeRestoreFailed    = "restore.failed"
	TypeRunProgress      = "run.progress"
	TypePostureChanged   = "posture.changed"
)

type Event struct {
	Type    string      `json:"type"`
	Cluster string      `json:"cluster,omitempty"`
	Subject string      `json:"subject,omitempty"`
	Time    time.Time   `json:"time"`
	Data    interface{} `json:"data,omitempty"`
}

type Subscription struct {
	C   chan Event
	bus *Bus
}

// Bus fans out to all subscribers, non-blocking (slow subscriber drops rather
// than stalling producers; reliable delivery = consumer polls a status skill).
type Bus struct {
	mu   sync.RWMutex
	subs map[*Subscription]struct{}
}

func NewBus() *Bus { return &Bus{subs: make(map[*Subscription]struct{})} }

var Default = NewBus()

func (b *Bus) Subscribe(buffer int) *Subscription {
	if buffer <= 0 {
		buffer = 64
	}
	s := &Subscription{C: make(chan Event, buffer), bus: b}
	b.mu.Lock()
	b.subs[s] = struct{}{}
	b.mu.Unlock()
	return s
}

func (s *Subscription) Close() {
	b := s.bus
	b.mu.Lock()
	if _, ok := b.subs[s]; ok {
		delete(b.subs, s)
		close(s.C)
	}
	b.mu.Unlock()
}

func (b *Bus) Publish(e Event) {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for s := range b.subs {
		select {
		case s.C <- e:
		default: // drop on full (advisory)
		}
	}
}

func (b *Bus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

func Publish(e Event) { Default.Publish(e) }
