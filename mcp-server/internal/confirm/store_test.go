package confirm

import (
	"context"
	"testing"
	"time"
)

func TestPutGetDelete(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	id, err := m.Put(ctx, Snapshot{Skill: "trigger_restore", Args: map[string]any{"namespace": "foo"}})
	if err != nil {
		t.Fatal(err)
	}
	s, ok, err := m.Get(ctx, id)
	if err != nil || !ok || s.Skill != "trigger_restore" || s.Args["namespace"] != "foo" {
		t.Fatalf("get: %+v ok=%v err=%v", s, ok, err)
	}
	if err := m.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := m.Get(ctx, id); ok {
		t.Fatal("should be gone after delete (one-shot)")
	}
}

func TestExpiry(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	cur := time.Now()
	m.now = func() time.Time { return cur } // controllable clock (DoD#6: 5-min expiry)
	id, _ := m.Put(ctx, Snapshot{Skill: "x"})
	if _, ok, _ := m.Get(ctx, id); !ok {
		t.Fatal("fresh snapshot should exist")
	}
	cur = cur.Add(6 * time.Minute)
	if _, ok, _ := m.Get(ctx, id); ok {
		t.Fatal("expired snapshot should miss")
	}
}

func TestUnknownID(t *testing.T) {
	if _, ok, err := NewMemory().Get(context.Background(), "nope"); ok || err != nil {
		t.Fatalf("unknown id: ok=%v err=%v", ok, err)
	}
}
