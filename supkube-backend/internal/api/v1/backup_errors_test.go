package v1

import "testing"

// flattenResultSection should emit engine → cluster → per-namespace (sorted)
// rows, tagging Source and preserving namespace, so the backup ERRORS panel can
// render results.gz detail the same way it renders DataUpload/PVB failures.
func TestFlattenResultSection(t *testing.T) {
	sec := VeleroResultSection{
		Velero:  []string{"engine boom"},
		Cluster: []string{"cluster-scoped boom"},
		Namespaces: map[string][]string{
			"zzz": {"z1"},
			"aaa": {"a1", "a2"},
		},
	}
	got := flattenResultSection(sec)

	want := []BackupErrorEntry{
		{Source: "Velero", Message: "engine boom"},
		{Source: "Cluster", Message: "cluster-scoped boom"},
		{Source: "Namespace", Namespace: "aaa", Message: "a1"},
		{Source: "Namespace", Namespace: "aaa", Message: "a2"},
		{Source: "Namespace", Namespace: "zzz", Message: "z1"},
	}
	if len(got) != len(want) {
		t.Fatalf("want %d entries, got %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d: want %+v, got %+v", i, want[i], got[i])
		}
	}
}

// Empty section → empty (non-nil) slice, so JSON marshals to [] not null.
func TestFlattenResultSectionEmpty(t *testing.T) {
	got := flattenResultSection(VeleroResultSection{})
	if got == nil {
		t.Fatal("want non-nil empty slice (renders as [] in JSON), got nil")
	}
	if len(got) != 0 {
		t.Fatalf("want 0 entries, got %d", len(got))
	}
}
