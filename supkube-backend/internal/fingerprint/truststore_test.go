// Tests for TrustStore — backed by a fake K8s clientset (no real apiserver).
// Covers: cold-start (cm doesn't exist), Bind creates+updates, IsBound is
// truthful, repeat Bind is idempotent, List returns sorted entries.
package fingerprint

import (
	"context"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/fake"
)

func TestTrustStore_ColdStartBindAndLookup(t *testing.T) {
	ks := fake.NewSimpleClientset()
	ts := NewTrustStore(ks)
	// 0 TTL so Bind/IsBound see fresh state without timing tricks.
	ts.cacheTTL = 0

	ctx := context.Background()
	if ts.IsBound(ctx, "cluster-a") {
		t.Fatalf("should not be bound before Bind")
	}

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	if err := ts.Bind(ctx, "cluster-a", "Cluster A", now); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if !ts.IsBound(ctx, "cluster-a") {
		t.Fatalf("should be bound after Bind")
	}

	// ConfigMap auto-created in supkube ns
	cm, err := ks.CoreV1().ConfigMaps(truststoreNamespace).Get(ctx, truststoreName, metaGet())
	if err != nil {
		t.Fatalf("cm should exist post-bind: %v", err)
	}
	data := cm.Data[truststoreDataKey]
	if !strings.Contains(data, "cluster-a") || !strings.Contains(data, "Cluster A") {
		t.Fatalf("cm data missing cluster-a entry: %s", data)
	}
}

func TestTrustStore_BindIsIdempotent(t *testing.T) {
	ks := fake.NewSimpleClientset()
	ts := NewTrustStore(ks)
	ts.cacheTTL = 0
	ctx := context.Background()

	t0 := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC)

	if err := ts.Bind(ctx, "cluster-b", "Cluster B", t0); err != nil {
		t.Fatalf("first Bind: %v", err)
	}
	if err := ts.Bind(ctx, "cluster-b", "Cluster B (renamed)", t1); err != nil {
		t.Fatalf("second Bind: %v", err)
	}

	list := ts.List(ctx)
	if len(list) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(list))
	}
	if list[0].SourceClusterName != "Cluster B (renamed)" {
		t.Errorf("name not updated: %s", list[0].SourceClusterName)
	}
	if list[0].BoundAt != t1.Format(time.RFC3339) {
		t.Errorf("boundAt not updated: %s", list[0].BoundAt)
	}
}

func TestTrustStore_ListSortedNewestFirst(t *testing.T) {
	ks := fake.NewSimpleClientset()
	ts := NewTrustStore(ks)
	ts.cacheTTL = 0
	ctx := context.Background()

	_ = ts.Bind(ctx, "cluster-old", "Old", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	_ = ts.Bind(ctx, "cluster-new", "New", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	_ = ts.Bind(ctx, "cluster-mid", "Mid", time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))

	list := ts.List(ctx)
	if len(list) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(list))
	}
	if list[0].SourceClusterID != "cluster-new" || list[2].SourceClusterID != "cluster-old" {
		t.Fatalf("not sorted newest-first: %+v", list)
	}
}

func TestTrustStore_NotBound_FreshCluster(t *testing.T) {
	ks := fake.NewSimpleClientset()
	ts := NewTrustStore(ks)
	ts.cacheTTL = 0
	if ts.IsBound(context.Background(), "never-seen") {
		t.Fatal("should not report bound for unknown cluster")
	}
}
