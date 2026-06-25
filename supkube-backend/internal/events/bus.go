// Package events is Supkube's in-process pub/sub event bus.
//
// Why in-process (no RabbitMQ/NATS): Supkube is a lightweight DR app; agents
// that want to *subscribe* to Supkube events are served by an SSE stream over
// the existing HTTP backend (see api/v1/events.go). A real message broker is
// only warranted on concrete durable-fanout / cross-service async needs — until
// then a broker is operational weight that fights the "lightweight" posture.
// When that day comes, swap this bus for a broker behind the same Publish/
// Subscribe surface; producers and the SSE handler don't change.
//
// The value is the EVENT CONTRACT (the Type constants below), not the transport.
package events

import (
	"sync"
	"time"
)

// Event types (the contract agents subscribe to). Keep stable.
const (
	TypeBackupCreated   = "backup.created"
	TypeBackupCompleted = "backup.completed"
	TypeRestoreCreated  = "restore.created"
	TypeRestoreFailed   = "restore.failed"
	TypeRunProgress     = "run.progress"    // orchestration engine (P01/P07)
	TypePostureChanged  = "posture.changed" // advisor / 3-2-1-1-0 (P09)
)

// Event is one published occurrence. JSON-serialised onto the SSE wire.
type Event struct {
	Type    string      `json:"type"`
	Cluster string      `json:"cluster,omitempty"`
	Subject string      `json:"subject,omitempty"` // e.g. backup/restore name
	Time    time.Time   `json:"time"`
	Data    interface{} `json:"data,omitempty"`
}

// Subscription is one consumer's delivery channel.
type Subscription struct {
	C   chan Event
	bus *Bus
}

// Bus fans out events to all live subscribers. Non-blocking: a slow/full
// subscriber drops events rather than stalling producers (DR control-plane
// events are advisory; back-pressure on a backup handler would be worse).
type Bus struct {
	mu   sync.RWMutex
	subs map[*Subscription]struct{}
}

func NewBus() *Bus { return &Bus{subs: make(map[*Subscription]struct{})} }

// Default is the process-wide bus.
var Default = NewBus()

// Subscribe registers a consumer. buffer<=0 → 32.
func (b *Bus) Subscribe(buffer int) *Subscription {
	if buffer <= 0 {
		buffer = 32
	}
	s := &Subscription{C: make(chan Event, buffer), bus: b}
	b.mu.Lock()
	b.subs[s] = struct{}{}
	b.mu.Unlock()
	return s
}

// Close unsubscribes and closes the channel. Safe to call once.
func (s *Subscription) Close() {
	b := s.bus
	b.mu.Lock()
	if _, ok := b.subs[s]; ok {
		delete(b.subs, s)
		close(s.C)
	}
	b.mu.Unlock()
}

// Publish fans out to all subscribers, never blocks.
func (b *Bus) Publish(e Event) {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for s := range b.subs {
		select {
		case s.C <- e:
		default: // subscriber full → drop (advisory events)
		}
	}
}

// SubscriberCount is for tests/observability.
func (b *Bus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

// Publish on the default bus.
func Publish(e Event) { Default.Publish(e) }
