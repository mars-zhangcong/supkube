package events

import (
	"testing"
	"time"
)

func TestBusPubSub(t *testing.T) {
	b := NewBus()
	s := b.Subscribe(4)
	defer s.Close()
	b.Publish(Event{Type: TypeBackupCreated, Subject: "b1"})
	select {
	case e := <-s.C:
		if e.Type != TypeBackupCreated || e.Subject != "b1" {
			t.Fatalf("got %+v", e)
		}
		if e.Time.IsZero() {
			t.Fatal("Time should be auto-set")
		}
	case <-time.After(time.Second):
		t.Fatal("no event delivered")
	}
}

func TestBusDropsWhenFull(t *testing.T) {
	b := NewBus()
	s := b.Subscribe(1)
	defer s.Close()
	for i := 0; i < 50; i++ { // must never block even though buffer=1
		b.Publish(Event{Type: "x"})
	}
	select {
	case <-s.C: // at least one delivered
	default:
		t.Fatal("expected at least one buffered event")
	}
}

func TestUnsubscribe(t *testing.T) {
	b := NewBus()
	s := b.Subscribe(2)
	if b.SubscriberCount() != 1 {
		t.Fatalf("count=%d", b.SubscriberCount())
	}
	s.Close()
	if b.SubscriberCount() != 0 {
		t.Fatalf("after close count=%d", b.SubscriberCount())
	}
}
