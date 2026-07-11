package v1

// dr_score_test.go — M1 collector + recommendation logic (Task #115 Phase A).
// Pure-function tests: no K8s API, deterministic (fixed SnapshotAt).

import (
	"testing"
	"time"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/supkube/supkube-backend/internal/advisor/evaluator"
)

var testBSLs = map[string]velerov1.BackupStorageLocation{
	"default": {
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec: velerov1.BackupStorageLocationSpec{
			Provider: "aws",
			Config:   map[string]string{"region": "ap-southeast-1"},
		},
	},
}

// Never-backed-up namespace: collector must mark LastBackupAt as a CONFIRMED nil
// (not missing) — this is the high-signal "never backed up" case the advisor exists
// for. RPOActual must be missing (no backup → no measurable RPO).
func TestDRCollector_NeverBackedUp(t *testing.T) {
	snap := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	ac := buildDRAppContext("app-nobackup", nil, snap, nil, nil, testBSLs)

	if !ac.Reliability.LastBackupAt.IsConfirmed() {
		t.Fatalf("LastBackupAt should be confirmed (we KNOW it was never backed up), got status=%s", ac.Reliability.LastBackupAt.Status)
	}
	if ac.Reliability.LastBackupAt.Value != nil {
		t.Errorf("LastBackupAt.Value should be nil for never-backed-up app")
	}
	if ac.Coverage.RPOActualSeconds.Status != evaluator.StatusMissing {
		t.Errorf("RPOActualSeconds should be missing (no backup), got %s", ac.Coverage.RPOActualSeconds.Status)
	}
	// Resilience signals must be missing too (no backup → no BSL usage).
	if ac.Resilience.BSLProviders.Status != evaluator.StatusMissing {
		t.Errorf("BSLProviders should be missing when no backups, got %s", ac.Resilience.BSLProviders.Status)
	}
}

// Backed-up namespace: collector must fill confirmed last-backup, compute RPO from
// SnapshotAt − completion, and pull BSL provider/region/media into Resilience.
func TestDRCollector_WithBackup(t *testing.T) {
	snap := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	start := snap.Add(-2 * time.Hour)
	completion := start.Add(5 * time.Minute)
	bk := velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bk1"},
		Spec:       velerov1.BackupSpec{IncludedNamespaces: []string{"app1"}, StorageLocation: "default"},
		Status: velerov1.BackupStatus{
			Phase:               velerov1.BackupPhaseCompleted,
			StartTimestamp:      &metav1.Time{Time: start},
			CompletionTimestamp: &metav1.Time{Time: completion},
		},
	}
	ac := buildDRAppContext("app1", nil, snap, []velerov1.Backup{bk}, nil, testBSLs)

	if !ac.Reliability.LastBackupAt.IsConfirmed() || ac.Reliability.LastBackupAt.Value == nil {
		t.Fatalf("LastBackupAt should be confirmed non-nil, got status=%s value=%v", ac.Reliability.LastBackupAt.Status, ac.Reliability.LastBackupAt.Value)
	}
	if !ac.Reliability.LastBackupSucceeded.Value {
		t.Errorf("LastBackupSucceeded should be true for a Completed backup")
	}
	// RPO = snap − completion = 2h − 5min = 6900s.
	wantRPO := int(snap.Sub(completion).Seconds())
	if ac.Coverage.RPOActualSeconds.Status != evaluator.StatusConfirmed || ac.Coverage.RPOActualSeconds.Value != wantRPO {
		t.Errorf("RPOActualSeconds = (%s,%d), want (confirmed,%d)", ac.Coverage.RPOActualSeconds.Status, ac.Coverage.RPOActualSeconds.Value, wantRPO)
	}
	if ac.Reliability.Last14DaysAttempts.Value != 1 || ac.Reliability.Last14DaysSuccess.Value != 1 {
		t.Errorf("14d window = (%d/%d), want 1/1", ac.Reliability.Last14DaysSuccess.Value, ac.Reliability.Last14DaysAttempts.Value)
	}
	if ac.Resilience.BSLProviders.Status != evaluator.StatusConfirmed || len(ac.Resilience.BSLProviders.Value) != 1 || ac.Resilience.BSLProviders.Value[0] != "aws" {
		t.Errorf("BSLProviders = (%s,%v), want (confirmed,[aws])", ac.Resilience.BSLProviders.Status, ac.Resilience.BSLProviders.Value)
	}
	if ac.Resilience.StorageMediaTypes.Value[0] != "s3" {
		t.Errorf("media for aws should map to s3, got %v", ac.Resilience.StorageMediaTypes.Value)
	}

	// End-to-end: a backed-up app must score (not not_eligible).
	res := evaluator.Score(ac)
	if res.Status != evaluator.EvalStatusScored {
		t.Errorf("a backed-up app should be scored, got status=%s", res.Status)
	}
}

// Recommendation: a data-bearing app (StatefulSet/PVC) that was never backed up
// must yield a P1 NO_BACKUP advice with a create_backup action hint.
func TestDRRecommendations_DataNoBackup(t *testing.T) {
	snap := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	ac := buildDRAppContext("app-data", nil, snap, nil, nil, testBSLs)
	res := evaluator.Score(ac)

	recs := buildDRRecommendations(res, 2 /*workloads*/, 1 /*sts*/, 3 /*pvc*/, false /*protected*/)

	var found *DRRecommendation
	for i := range recs {
		if recs[i].Type == "NO_BACKUP" {
			found = &recs[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected a NO_BACKUP recommendation for data-bearing unbacked app, got %+v", recs)
	}
	if found.Severity != "P1" {
		t.Errorf("data-bearing unbacked app should be P1, got %s", found.Severity)
	}
	if found.Action != "create_backup" {
		t.Errorf("NO_BACKUP action hint should be create_backup, got %s", found.Action)
	}
}

// Recommendation: a stateless protected app with weak resilience scoring should
// surface WEAK_RESILIENCE, and NOT a NO_BACKUP (it IS backed up).
func TestDRRecommendations_ScoredWeakResilience(t *testing.T) {
	res := evaluator.ScoreResult{
		Status: evaluator.EvalStatusScored,
		Level:  evaluator.LevelFragile,
		Dimensions: evaluator.Dimensions{
			BackupCoverage:          evaluator.DimensionScore{Score: 20, MaxScore: 25},
			Resilience:              evaluator.DimensionScore{Score: 5, MaxScore: 35}, // weak: 5/35 < 50%
			ImmutabilityAndSecurity: evaluator.DimensionScore{Score: 15, MaxScore: 20},
			Reliability:             evaluator.DimensionScore{Score: 18, MaxScore: 20},
		},
	}
	recs := buildDRRecommendations(res, 1, 0, 0, true /*protected*/)

	hasWeakResil, hasNoBackup := false, false
	for _, r := range recs {
		if r.Type == "WEAK_RESILIENCE" {
			hasWeakResil = true
		}
		if r.Type == "NO_BACKUP" {
			hasNoBackup = true
		}
	}
	if !hasWeakResil {
		t.Errorf("weak resilience dimension should surface WEAK_RESILIENCE advice, got %+v", recs)
	}
	if hasNoBackup {
		t.Errorf("a protected app must NOT get NO_BACKUP advice")
	}
}
