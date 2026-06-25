package mcpconfirm

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	return New(cl, "supkube-system", time.Minute)
}

func TestPutGetDelete(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	id, err := st.Put(ctx, Snapshot{Skill: "trigger_restore", Args: map[string]any{"namespace": "foo"}})
	if err != nil {
		t.Fatal(err)
	}
	snap, ok, err := st.Get(ctx, id)
	if err != nil || !ok || snap.Skill != "trigger_restore" || snap.Args["namespace"] != "foo" {
		t.Fatalf("get: %+v ok=%v err=%v", snap, ok, err)
	}
	if err := st.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := st.Get(ctx, id); ok {
		t.Fatal("should be gone after delete (one-shot)")
	}
}

func TestExpiry(t *testing.T) {
	st := newStore(t)
	cur := time.Now()
	st.now = func() time.Time { return cur } // ttl = 1 min
	ctx := context.Background()
	id, err := st.Put(ctx, Snapshot{Skill: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := st.Get(ctx, id); !ok {
		t.Fatal("fresh should exist")
	}
	cur = cur.Add(2 * time.Minute)
	if _, ok, _ := st.Get(ctx, id); ok {
		t.Fatal("expired should miss (and be GC'd)")
	}
}

func TestReap(t *testing.T) {
	st := newStore(t)
	cur := time.Now()
	st.now = func() time.Time { return cur }
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := st.Put(ctx, Snapshot{Skill: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	cur = cur.Add(2 * time.Minute) // all expired
	n, err := st.Reap(ctx)
	if err != nil || n != 3 {
		t.Fatalf("reap: n=%d err=%v (want 3)", n, err)
	}
}

func TestUnknownID(t *testing.T) {
	if _, ok, err := newStore(t).Get(context.Background(), "nope"); ok || err != nil {
		t.Fatalf("unknown id: ok=%v err=%v", ok, err)
	}
}
