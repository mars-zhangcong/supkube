package confirm

import (
	"testing"
	"time"
)

func TestPutGetDelete(t *testing.T) {
	m := NewMemory()
	id := m.Put(Snapshot{Skill: "trigger_restore", Args: map[string]any{"namespace": "foo"}})
	s, ok := m.Get(id)
	if !ok || s.Skill != "trigger_restore" || s.Args["namespace"] != "foo" {
		t.Fatalf("get: %+v ok=%v", s, ok)
	}
	m.Delete(id)
	if _, ok := m.Get(id); ok {
		t.Fatal("should be gone after delete (one-shot)")
	}
}

func TestExpiry(t *testing.T) {
	m := NewMemory()
	cur := time.Now()
	m.now = func() time.Time { return cur } // controllable clock (DoD#6: 5-min expiry)
	id := m.Put(Snapshot{Skill: "x"})
	if _, ok := m.Get(id); !ok {
		t.Fatal("fresh snapshot should exist")
	}
	cur = cur.Add(6 * time.Minute)
	if _, ok := m.Get(id); ok {
		t.Fatal("expired snapshot should miss")
	}
}

func TestUnknownID(t *testing.T) {
	if _, ok := NewMemory().Get("nope"); ok {
		t.Fatal("unknown id should miss")
	}
}
