package confirm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Fake backend implementing /api/v1/mcp/confirmations, mirroring the real one
// (in-memory map). Lets BackendStore be tested without a cluster.
func fakeBackend() (*httptest.Server, map[string]map[string]any) {
	store := map[string]map[string]any{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/mcp/confirmations", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := "id-" + body["skill"].(string)
		store[id] = body
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"confirmId": id})
	})
	mux.HandleFunc("/api/v1/mcp/confirmations/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/mcp/confirmations/")
		switch r.Method {
		case http.MethodGet:
			snap, ok := store[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(snap)
		case http.MethodDelete:
			delete(store, id)
			w.WriteHeader(http.StatusNoContent)
		}
	})
	return httptest.NewServer(mux), store
}

func TestBackendStore_RoundTrip(t *testing.T) {
	srv, _ := fakeBackend()
	defer srv.Close()
	ctx := context.Background()
	b := NewBackendStore(srv.URL, "tok")

	id, err := b.Put(ctx, Snapshot{Skill: "trigger_restore", Args: map[string]any{"namespace": "foo"}})
	if err != nil || id == "" {
		t.Fatalf("put: id=%q err=%v", id, err)
	}
	snap, ok, err := b.Get(ctx, id)
	if err != nil || !ok || snap.Skill != "trigger_restore" || snap.Args["namespace"] != "foo" {
		t.Fatalf("get: %+v ok=%v err=%v", snap, ok, err)
	}
	if err := b.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := b.Get(ctx, id); ok {
		t.Fatal("should be gone after delete (one-shot, cross-replica)")
	}
}

func TestBackendStore_UnknownIsMiss(t *testing.T) {
	srv, _ := fakeBackend()
	defer srv.Close()
	if _, ok, err := NewBackendStore(srv.URL, "").Get(context.Background(), "nope"); ok || err != nil {
		t.Fatalf("unknown id: ok=%v err=%v", ok, err)
	}
}
