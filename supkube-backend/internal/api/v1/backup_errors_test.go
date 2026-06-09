package v1

import (
	"errors"
	"strings"
	"testing"
)

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

// TC-5 (TC-REG-001): when results.gz cannot be fetched, the response must carry
// a fetchError breadcrumb and must NOT present a silent "0 errors" success —
// the v0.8.8.1 anti-pattern the whole fix exists to kill. fetchError surfaces
// in the UI above the (empty) list so the user knows we couldn't check.
func TestMergeBackupResults_FetchFailureNoSilentZero(t *testing.T) {
	resp := &BackupErrorsResponse{Errors: []BackupErrorEntry{}, Warnings: []BackupErrorEntry{}}
	mergeBackupResults(resp, 4 /*Backup.status.Errors*/, nil, errors.New("BSL unreachable: dial tcp timeout"))

	if resp.FetchError == "" {
		t.Fatal("fetch error must set FetchError (else UI shows false '0 errors')")
	}
	if !strings.Contains(resp.FetchError, "results.gz") || !strings.Contains(resp.FetchError, "BSL unreachable") {
		t.Errorf("FetchError should name the source + cause, got %q", resp.FetchError)
	}
	if len(resp.Errors) != 0 {
		t.Errorf("on fetch failure Errors must stay empty (no fabricated detail), got %+v", resp.Errors)
	}
}

// TC-5 cont.: on success, results.gz errors fill Errors only when the CR-level
// scan found none (no double-count); warnings always merge in.
func TestMergeBackupResults_SuccessMerge(t *testing.T) {
	res := &VeleroRestoreResults{
		Errors:   VeleroResultSection{Velero: []string{"e1", "e2"}},
		Warnings: VeleroResultSection{Namespaces: map[string][]string{"ns": {"w1"}}},
	}

	// CR-level scan found nothing → take results.gz errors.
	clean := &BackupErrorsResponse{Errors: []BackupErrorEntry{}, Warnings: []BackupErrorEntry{}}
	mergeBackupResults(clean, 2, res, nil)
	if len(clean.Errors) != 2 || len(clean.Warnings) != 1 {
		t.Fatalf("want 2 errors + 1 warning from results.gz, got %d/%d", len(clean.Errors), len(clean.Warnings))
	}
	if clean.FetchError != "" {
		t.Errorf("success path must not set FetchError, got %q", clean.FetchError)
	}

	// CR-level scan already found a DataUpload error → do NOT double-count
	// errors from results.gz, but still merge warnings.
	withCR := &BackupErrorsResponse{
		Errors:   []BackupErrorEntry{{Source: "DataUpload", Message: "mover failed"}},
		Warnings: []BackupErrorEntry{},
	}
	mergeBackupResults(withCR, 2, res, nil)
	if len(withCR.Errors) != 1 {
		t.Errorf("results.gz errors must not double-count over CR-level errors, got %d", len(withCR.Errors))
	}
	if len(withCR.Warnings) != 1 {
		t.Errorf("warnings should still merge even when errors are skipped, got %d", len(withCR.Warnings))
	}
}
