package v1

import (
	"context"
	"os"
	"testing"
)

// TestLiveBackupResults exercises the real fetchBackupDetailedResults path
// against a live cluster (current kubeconfig context) for a named Backup. It
// creates a transient, self-cleaning Velero DownloadRequest — the exact path
// the product runs on every "动作详情 → ERRORS" view — and asserts that the
// results.gz detail for a PartiallyFailed backup is actually retrievable.
//
// Gated by SUPKUBE_LIVE_BACKUP so normal `go test` / CI runs skip it.
//
// Run (DEFECT-001 verification against aks-jumborca-dev):
//
//	VELERO_NAMESPACE=supkube SUPKUBE_LIVE_BACKUP=test-app-policy-20260608223156 \
//	  go test ./internal/api/v1 -run TestLiveBackupResults -v
func TestLiveBackupResults(t *testing.T) {
	name := os.Getenv("SUPKUBE_LIVE_BACKUP")
	if name == "" {
		t.Skip("set SUPKUBE_LIVE_BACKUP=<backup-name> to run against a real cluster")
	}
	res, err := fetchBackupDetailedResults(context.Background(), name)
	if err != nil {
		t.Fatalf("fetchBackupDetailedResults(%s): %v", name, err)
	}
	errs := flattenResultSection(res.Errors)
	warns := flattenResultSection(res.Warnings)
	t.Logf("backup %q → %d error(s), %d warning(s) from results.gz", name, len(errs), len(warns))
	for i, e := range errs {
		t.Logf("  err[%d] %s · ns=%s · %s", i, e.Source, e.Namespace, e.Message)
	}
	for i, w := range warns {
		t.Logf("  warn[%d] %s · ns=%s · %s", i, w.Source, w.Namespace, w.Message)
	}
	if len(errs) == 0 {
		t.Fatalf("expected ≥1 error from results.gz for %s, got 0 (the bug)", name)
	}
}
