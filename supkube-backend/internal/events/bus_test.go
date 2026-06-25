package events

import (
	"testing"
	"time"
)

func TestBusPubSub(t *testing.T) {
	b := NewBus()
	s := b.Subscribe(4)
	defer s.Close()
	b.Publish(Event{Type: TypeBackupCompleted, Subject: "b1"})
	select {
	case e := <-s.C:
		if e.Type != TypeBackupCompleted || e.Subject != "b1" {
			t.Fatalf("got %+v", e)
		}
		if e.Time.IsZero() {
			t.Fatal("Time should auto-set")
		}
	case <-time.After(time.Second):
		t.Fatal("no event")
	}
}

func TestBusDropsWhenFull(t *testing.T) {
	b := NewBus()
	s := b.Subscribe(1)
	defer s.Close()
	for i := 0; i < 50; i++ {
		b.Publish(Event{Type: "x"}) // must never block at buffer=1
	}
	select {
	case <-s.C:
	default:
		t.Fatal("expected at least one buffered")
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
		t.Fatalf("after close=%d", b.SubscriberCount())
	}
}
