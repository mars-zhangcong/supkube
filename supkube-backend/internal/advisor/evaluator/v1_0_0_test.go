package evaluator

// v1_0_0_test.go — Unit tests for PRD-011 evaluator v1.0.0.
//
// P0 coverage (8 cases from 测试用例.md §19 TC-AI-MVP):
//   - TC-AI-002 — 逐子项分档精确评分 (table-driven, every tier of every subitem)
//   - TC-AI-003 — 确定性可复现 (100× same input, no wall-clock dependency)
//   - TC-AI-004 — 4 档安全级别边界映射 (90/75/60 inclusive)
//   - TC-AI-005 — 无备份封顶 30 (hard threshold 1)
//   - TC-AI-016 — 高分校准 30 (hard threshold 2)
//   - TC-AI-017 — 滑动窗口公式 + Tier1 连失>3 归 0
//   - TC-AI-018 — WORM COMPLIANCE=10 / Governance=6 / 可覆盖=0
//   - TC-AI-019 — 缺失字段走 unable_to_confirm
//
// Determinism contract (TC-AI-003 + Mars Gate 1 rule ③):
//   - Same AppContext fixture → same ScoreResult, including SubitemScore order.
//   - No time.Now() — only ctx.SnapshotAt is read.
//   - Tests run twice over the same fixture must produce reflect.DeepEqual results.

import (
	"reflect"
	"testing"
	"time"
)

// ─── fixture builders (DRY for table-driven tests) ────────────────────────────

// snapshotAt — fixed time used across all tests for determinism.
var snapshotAt = time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)

// confirmed wraps a value as Field[T] with StatusConfirmed.
func confirmed[T any](v T) Field[T] { return Field[T]{Value: v, Status: StatusConfirmed} }

// missing wraps a zero value as Field[T] with StatusMissing.
func missing[T any]() Field[T] {
	var zero T
	return Field[T]{Value: zero, Status: StatusMissing}
}

// timePtr returns a pointer to a time.Time literal (for LastBackupAt).
func timePtr(t time.Time) *time.Time { return &t }

// perfectCtx builds an AppContext that scores exactly 100 (every subitem at max tier).
// Used as the baseline for tests that want to mutate one field at a time.
func perfectCtx() AppContext {
	retainDate := snapshotAt.AddDate(0, 0, 90) // > snapshot+30d, satisfies WORM full credit
	lastBackup := snapshotAt.Add(-1 * time.Hour)
	return AppContext{
		Namespace:  "prod-mysql",
		SnapshotAt: snapshotAt,
		Coverage: CoverageInput{
			Tier:                      confirmed(Tier1),
			HasDifferentiatedPolicies: confirmed(true),
			CoreCoveragePct:           confirmed(100.0),
			ZombieAssetPct:            confirmed(0.0),
			RPOActualSeconds:          confirmed(1800), // 30 min
			RPOTargetSeconds:          confirmed(3600), // 1 h target → actual better than target
			BackupIncludesMetadata:    confirmed(true),
		},
		Resilience: ResilienceInput{
			StorageMediaTypes: confirmed([]string{"s3", "minio"}),
			BSLRegions:        confirmed([]string{"us-east-1", "eu-west-1"}),
			BSLProviders:      confirmed([]string{"aws", "azure"}),
			NetworkIsolation:  confirmed(IsolationAirGap),
		},
		Security: SecurityInput{
			WORM: WORMInput{
				ObjectLockEnabled:  confirmed(true),
				Mode:               confirmed(WORMCompliance),
				RetainUntilDate:    confirmed(retainDate),
				BusinessSafetyDays: 30,
			},
			Encryption: EncryptionInput{
				StaticAES256:           confirmed(true),
				TransitTLS13:           confirmed(true),
				KMSIndependentRotation: confirmed(true),
			},
			AccessControl: AccessControlInput{
				MFAEnabled:              confirmed(true),
				RBACEnabled:             confirmed(true),
				DeleteRequiresApproval:  confirmed(true),
				AuditOffsiteTamperProof: confirmed(true),
			},
		},
		Reliability: ReliabilityInput{
			Last14DaysAttempts:  confirmed(14),
			Last14DaysSuccess:   confirmed(14),
			ConsecutiveFailures: confirmed(0),
			LastBackupAt:        confirmed(timePtr(lastBackup)),
			LastBackupSucceeded: confirmed(true),
			DrillStatus:         confirmed(DrillFullAuto3mo100pct),
		},
	}
}

// findSubitem looks up a SubitemScore by ID in a ScoreResult. Returns nil if not found.
func findSubitem(t *testing.T, r ScoreResult, id string) *SubitemScore {
	t.Helper()
	for _, dim := range []DimensionScore{
		r.Dimensions.BackupCoverage,
		r.Dimensions.Resilience,
		r.Dimensions.ImmutabilityAndSecurity,
		r.Dimensions.Reliability,
	} {
		for i := range dim.Subitems {
			if dim.Subitems[i].ID == id {
				return &dim.Subitems[i]
			}
		}
	}
	t.Fatalf("subitem %q not found in ScoreResult", id)
	return nil
}

// ─── TC-AI-002 [P0] 逐子项分档精确评分 ─────────────────────────────────────────

// TC-AI-002: For every (subitem, tier) cell in the matrix, build a fixture that
// triggers exactly that tier and assert the subitem score == matrix value.
//
// Perfect fixture totals 100 (sanity check); we mutate single fields to land in lower tiers.
func TestTC_AI_002_PerfectCtx_TotalsTo100(t *testing.T) {
	r := Score(perfectCtx())
	if r.TotalScore != 100 {
		t.Errorf("perfect fixture: TotalScore = %d, want 100", r.TotalScore)
	}
	if r.Level != LevelHighResilience {
		t.Errorf("perfect fixture: Level = %q, want high_resilience", r.Level)
	}
	if r.ScoreRulesVersion != "v1.0.0" {
		t.Errorf("ScoreRulesVersion = %q, want v1.0.0", r.ScoreRulesVersion)
	}
	if !r.EvaluatedAt.Equal(snapshotAt) {
		t.Errorf("EvaluatedAt = %v, want = snapshotAt %v", r.EvaluatedAt, snapshotAt)
	}
}

func TestTC_AI_002_Dim1_TierClassification_Tiers(t *testing.T) {
	cases := []struct {
		name       string
		hasDiff    bool
		coreCovPct float64
		zombiePct  float64
		wantScore  int
		wantTier   string
	}{
		{"max: diff + 100% core", true, 100.0, 0.0, 10, "max"},
		{"partial: no-diff + zombie<5", false, 100.0, 4.99, 5, "partial"},
		{"partial: diff but core 80%", true, 80.0, 0.0, 5, "partial"},
		{"zero: no-diff + zombie=5%", false, 100.0, 5.0, 0, "zero"},
		{"zero: no-diff + zombie=10%", false, 90.0, 10.0, 0, "zero"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := perfectCtx()
			ctx.Coverage.HasDifferentiatedPolicies = confirmed(c.hasDiff)
			ctx.Coverage.CoreCoveragePct = confirmed(c.coreCovPct)
			ctx.Coverage.ZombieAssetPct = confirmed(c.zombiePct)
			s := findSubitem(t, Score(ctx), "coverage.tier_classification")
			if s.Score != c.wantScore || s.Tier != c.wantTier {
				t.Errorf("got score=%d tier=%q, want score=%d tier=%q (basis=%q)",
					s.Score, s.Tier, c.wantScore, c.wantTier, s.Basis)
			}
		})
	}
}

func TestTC_AI_002_Dim1_RPOAttainment_Tiers(t *testing.T) {
	cases := []struct {
		name      string
		actual    int
		target    int
		wantScore int
		wantTier  string
	}{
		{"max: actual=target", 3600, 3600, 10, "max"},
		{"max: actual<target", 1800, 3600, 10, "max"},
		{"partial: actual=1.5×target", 5400, 3600, 5, "partial"}, // boundary inclusive
		{"partial: actual just under 1.5×", 5399, 3600, 5, "partial"},
		{"zero: actual=1.51×target", 5436, 3600, 0, "zero"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := perfectCtx()
			ctx.Coverage.RPOActualSeconds = confirmed(c.actual)
			ctx.Coverage.RPOTargetSeconds = confirmed(c.target)
			s := findSubitem(t, Score(ctx), "coverage.rpo_attainment")
			if s.Score != c.wantScore || s.Tier != c.wantTier {
				t.Errorf("got score=%d tier=%q, want score=%d tier=%q",
					s.Score, s.Tier, c.wantScore, c.wantTier)
			}
		})
	}
}

func TestTC_AI_002_Dim1_MetadataBackup_Tiers(t *testing.T) {
	for _, v := range []struct {
		val   bool
		score int
		tier  string
	}{{true, 5, "max"}, {false, 0, "zero"}} {
		ctx := perfectCtx()
		ctx.Coverage.BackupIncludesMetadata = confirmed(v.val)
		s := findSubitem(t, Score(ctx), "coverage.metadata_backup")
		if s.Score != v.score || s.Tier != v.tier {
			t.Errorf("BackupIncludesMetadata=%v: got %d/%q, want %d/%q",
				v.val, s.Score, s.Tier, v.score, v.tier)
		}
	}
}

func TestTC_AI_002_Dim2_MediaDiversity_Tiers(t *testing.T) {
	cases := []struct {
		media     []string
		wantScore int
		wantTier  string
	}{
		{[]string{"s3", "minio", "azure"}, 10, "max"},
		{[]string{"s3", "minio"}, 10, "max"},
		{[]string{"s3"}, 5, "partial"},
		{[]string{}, 0, "zero"},
	}
	for _, c := range cases {
		ctx := perfectCtx()
		ctx.Resilience.StorageMediaTypes = confirmed(c.media)
		s := findSubitem(t, Score(ctx), "resilience.media_diversity")
		if s.Score != c.wantScore || s.Tier != c.wantTier {
			t.Errorf("media=%v: got %d/%q, want %d/%q",
				c.media, s.Score, s.Tier, c.wantScore, c.wantTier)
		}
	}
}

func TestTC_AI_002_Dim2_OffsiteCrossCloud_Tiers(t *testing.T) {
	cases := []struct {
		name      string
		providers []string
		regions   []string
		wantScore int
		wantTier  string
	}{
		{"max: multi-provider", []string{"aws", "azure"}, []string{"us-east-1"}, 10, "max"},
		{"partial: single-provider multi-region", []string{"aws"}, []string{"us-east-1", "eu-west-1"}, 6, "partial"},
		{"zero: single AZ", []string{"aws"}, []string{"us-east-1"}, 0, "zero"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := perfectCtx()
			ctx.Resilience.BSLProviders = confirmed(c.providers)
			ctx.Resilience.BSLRegions = confirmed(c.regions)
			s := findSubitem(t, Score(ctx), "resilience.offsite_cross_cloud")
			if s.Score != c.wantScore || s.Tier != c.wantTier {
				t.Errorf("got %d/%q, want %d/%q", s.Score, s.Tier, c.wantScore, c.wantTier)
			}
		})
	}
}

func TestTC_AI_002_Dim2_AirGapIsolation_Tiers(t *testing.T) {
	cases := []struct {
		mode      NetworkIsolationMode
		wantScore int
		wantTier  string
	}{
		{IsolationAirGap, 15, "max"},
		{IsolationSameADDomain, 8, "partial"},
		{IsolationNone, 0, "zero"},
	}
	for _, c := range cases {
		ctx := perfectCtx()
		ctx.Resilience.NetworkIsolation = confirmed(c.mode)
		s := findSubitem(t, Score(ctx), "resilience.air_gap_isolation")
		if s.Score != c.wantScore || s.Tier != c.wantTier {
			t.Errorf("mode=%v: got %d/%q, want %d/%q", c.mode, s.Score, s.Tier, c.wantScore, c.wantTier)
		}
	}
}

func TestTC_AI_002_Dim3_Encryption_Tiers(t *testing.T) {
	cases := []struct {
		name          string
		aes, tls, kms bool
		wantScore     int
		wantTier      string
	}{
		{"max: all 3", true, true, true, 5, "max"},
		{"partial: aes only", true, false, false, 2, "partial"},
		{"partial: tls only", false, true, false, 2, "partial"},
		{"partial: aes+tls no kms", true, true, false, 2, "partial"},
		{"zero: no encryption", false, false, false, 0, "zero"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := perfectCtx()
			ctx.Security.Encryption = EncryptionInput{
				StaticAES256:           confirmed(c.aes),
				TransitTLS13:           confirmed(c.tls),
				KMSIndependentRotation: confirmed(c.kms),
			}
			s := findSubitem(t, Score(ctx), "security.encryption_credentials")
			if s.Score != c.wantScore || s.Tier != c.wantTier {
				t.Errorf("got %d/%q, want %d/%q", s.Score, s.Tier, c.wantScore, c.wantTier)
			}
		})
	}
}

// TC-AI-002 dim3 access_control — graduated 5/3/1/0 (Mars 2026-06-03 Option A).
//
// Mapping per PRD-011 §4.2.3 决策 ③:
//
//	n=4 (full stack)        → 5  (Tier "max")
//	n=3 (3 of 4 satisfied)  → 3  (Tier "partial-high")
//	n=2 (2 of 4 satisfied)  → 1  (Tier "partial-low")
//	n≤1 (single or none)    → 0  (Tier "zero", false sense of security)
func TestTC_AI_002_Dim3_AccessControl_Graduated(t *testing.T) {
	cases := []struct {
		name                       string
		mfa, rbac, approval, audit bool
		wantScore                  int
		wantTier                   string
	}{
		// n=4 → 5
		{"4/4: all satisfied → 5", true, true, true, true, 5, "max"},

		// n=3 → 3 (4 combos all give 3)
		{"3/4: missing audit → 3", true, true, true, false, 3, "partial-high"},
		{"3/4: missing approval → 3", true, true, false, true, 3, "partial-high"},
		{"3/4: missing rbac → 3", true, false, true, true, 3, "partial-high"},
		{"3/4: missing mfa → 3", false, true, true, true, 3, "partial-high"},

		// n=2 → 1
		{"2/4: mfa+rbac → 1", true, true, false, false, 1, "partial-low"},
		{"2/4: approval+audit → 1", false, false, true, true, 1, "partial-low"},

		// n=1 → 0 (single control = false sense of security)
		{"1/4: mfa only → 0", true, false, false, false, 0, "zero"},
		{"1/4: audit only → 0", false, false, false, true, 0, "zero"},

		// n=0 → 0
		{"0/4: all false → 0", false, false, false, false, 0, "zero"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := perfectCtx()
			ctx.Security.AccessControl = AccessControlInput{
				MFAEnabled:              confirmed(c.mfa),
				RBACEnabled:             confirmed(c.rbac),
				DeleteRequiresApproval:  confirmed(c.approval),
				AuditOffsiteTamperProof: confirmed(c.audit),
			}
			s := findSubitem(t, Score(ctx), "security.access_control")
			if s.Score != c.wantScore || s.Tier != c.wantTier {
				t.Errorf("got %d/%q, want %d/%q (basis=%q)",
					s.Score, s.Tier, c.wantScore, c.wantTier, s.Basis)
			}
		})
	}
}

func TestTC_AI_002_Dim4_DrillPassingRate_Tiers(t *testing.T) {
	cases := []struct {
		status    DrillTier
		wantScore int
	}{
		{DrillFullAuto3mo100pct, 15},
		{DrillPeriodicManual, 10},
		{DrillPartialFile, 5},
		{DrillNone, 0},
	}
	for _, c := range cases {
		ctx := perfectCtx()
		ctx.Reliability.DrillStatus = confirmed(c.status)
		s := findSubitem(t, Score(ctx), "reliability.drill_passing_rate")
		if s.Score != c.wantScore {
			t.Errorf("drill=%v: got score=%d, want %d", c.status, s.Score, c.wantScore)
		}
	}
}

// ─── TC-AI-003 [P0] 确定性可复现 ──────────────────────────────────────────────

// TC-AI-003: Same AppContext → same ScoreResult, deep equal, 100 iterations.
// Also: switching wall-clock doesn't affect output (no time.Now() dependency).
func TestTC_AI_003_Determinism_100Iterations(t *testing.T) {
	ctx := perfectCtx()
	baseline := Score(ctx)
	for i := 0; i < 100; i++ {
		got := Score(ctx)
		if !reflect.DeepEqual(got, baseline) {
			t.Fatalf("iteration %d: result differs from baseline\nbaseline: %+v\ngot: %+v", i, baseline, got)
		}
	}
	if baseline.ScoreRulesVersion != "v1.0.0" {
		t.Errorf("ScoreRulesVersion = %q, want v1.0.0", baseline.ScoreRulesVersion)
	}
}

// TC-AI-003 bonus: wall-clock independence (no time.Now() usage).
// We sleep ~10ms between calls — same input should still produce same output.
// (We can't easily mock time.Now globally without a clock interface,
// but Score()'s public contract guarantees it doesn't read wall clock.)
func TestTC_AI_003_WallClockIndependence(t *testing.T) {
	ctx := perfectCtx()
	first := Score(ctx)
	time.Sleep(10 * time.Millisecond)
	second := Score(ctx)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("results differ across wall-clock time:\nfirst: %+v\nsecond: %+v", first, second)
	}
}

// TC-AI-003 bonus: ScoreRulesVersion is locked.
func TestTC_AI_003_ScoreRulesVersion(t *testing.T) {
	if ScoreRulesVersion != "v1.0.0" {
		t.Errorf("constant ScoreRulesVersion = %q, want v1.0.0", ScoreRulesVersion)
	}
}

// ─── TC-AI-004 [P0] 4 档安全级别边界映射 (90/75/60 inclusive) ─────────────────

// TC-AI-004: classifyLevel respects 90/75/60 inclusive boundaries (upper-tier).
// We test the helper directly since constructing exact total scores via AppContext
// is harder than calling classifyLevel(score, nil).
func TestTC_AI_004_LevelBoundaries(t *testing.T) {
	cases := []struct {
		score int
		want  SecurityLevel
	}{
		{100, LevelHighResilience},
		{90, LevelHighResilience},   // 90 inclusive on upper tier
		{89, LevelCompliantLowRisk}, // 89 → low risk
		{75, LevelCompliantLowRisk}, // 75 inclusive
		{74, LevelFragile},          // 74 → fragile
		{60, LevelFragile},          // 60 inclusive
		{59, LevelCritical},         // 59 → critical
		{30, LevelCritical},
		{0, LevelCritical},
	}
	for _, c := range cases {
		got := classifyLevel(c.score, nil)
		if got != c.want {
			t.Errorf("classifyLevel(%d, nil) = %q, want %q", c.score, got, c.want)
		}
	}
}

// TC-AI-004 bonus: hard threshold override forces Critical regardless of score.
func TestTC_AI_004_HardThresholdForcesCritical(t *testing.T) {
	for _, score := range []int{100, 90, 75, 60, 30, 0} {
		got := classifyLevel(score, []string{"no_backup_cap_30"})
		if got != LevelCritical {
			t.Errorf("classifyLevel(%d, [no_backup_cap_30]) = %q, want critical", score, got)
		}
	}
}

// ─── TC-AI-005 [P0] not_eligible 路径 (Mars 2026-06-03 §4.2.1 替换初版"封顶 30") ──

// TC-AI-005: LastBackupAt == nil → Status: not_eligible_no_runs, NOT scored.
// evaluator 不参评 (没有 TotalScore/Level/Dimensions), 前端显示提示卡 "建议先跑首次备份".
func TestTC_AI_005_NoBackup_NotEligibleNoRuns(t *testing.T) {
	ctx := perfectCtx()
	// Wipe out the last backup record entirely.
	ctx.Reliability.LastBackupAt = confirmed[*time.Time](nil)

	r := Score(ctx)
	if r.Status != EvalStatusNotEligibleNoRuns {
		t.Errorf("LastBackupAt=nil: Status=%q, want %q (Mars 2026-06-03 §4.2.1)",
			r.Status, EvalStatusNotEligibleNoRuns)
	}
	if r.TotalScore != 0 {
		t.Errorf("not_eligible: TotalScore=%d, want 0 (no score computed)", r.TotalScore)
	}
	if r.Level != "" {
		t.Errorf("not_eligible: Level=%q, want empty (no classification)", r.Level)
	}
	if r.NotEligibleReason == "" {
		t.Errorf("not_eligible: NotEligibleReason is empty, must be populated for UI prompt")
	}
	// Calibration list must NOT contain the deprecated "no_backup_cap_30" (Mars removed it).
	if containsString(r.CalibrationApplied, "no_backup_cap_30") {
		t.Errorf("CalibrationApplied contains deprecated no_backup_cap_30: %v (Mars 2026-06-03 移除)",
			r.CalibrationApplied)
	}
	// ScoreRulesVersion still emitted (callers need to know which rule version ran).
	if r.ScoreRulesVersion != "v1.0.0" {
		t.Errorf("not_eligible: ScoreRulesVersion=%q, want v1.0.0", r.ScoreRulesVersion)
	}
	// EvaluatedAt still equals SnapshotAt (determinism).
	if !r.EvaluatedAt.Equal(ctx.SnapshotAt) {
		t.Errorf("not_eligible: EvaluatedAt=%v, want = SnapshotAt %v", r.EvaluatedAt, ctx.SnapshotAt)
	}
}

// TC-AI-005 variant: LastBackupAt unconfirmed (collector failed) → also not_eligible (conservative).
func TestTC_AI_005_LastBackupAtUnconfirmed_NotEligible(t *testing.T) {
	ctx := perfectCtx()
	ctx.Reliability.LastBackupAt = missing[*time.Time]()

	r := Score(ctx)
	if r.Status != EvalStatusNotEligibleNoRuns {
		t.Errorf("LastBackupAt unconfirmed: Status=%q, want not_eligible_no_runs (innocent until proven)",
			r.Status)
	}
}

// ─── TC-AI-016 [P0] 高分校准 30 (hard threshold 2) ───────────────────────────

// TC-AI-016: preliminary ≥ 90 && LastBackupSucceeded == false → force 30 + Critical.
func TestTC_AI_016_HighScoreCalibration30(t *testing.T) {
	ctx := perfectCtx() // would score 100
	ctx.Reliability.LastBackupSucceeded = confirmed(false)

	r := Score(ctx)
	if r.TotalScore != 30 {
		t.Errorf("preliminary 100 + LastBackupSucceeded=false: TotalScore=%d, want 30", r.TotalScore)
	}
	if r.Level != LevelCritical {
		t.Errorf("Level=%q, want critical", r.Level)
	}
	if !containsString(r.CalibrationApplied, "high_score_calibration_30") {
		t.Errorf("CalibrationApplied=%v, want contain high_score_calibration_30", r.CalibrationApplied)
	}
}

// TC-AI-016 bonus: score < 90 should NOT trigger calibration even if last backup failed.
func TestTC_AI_016_BelowThreshold_NoCalibration(t *testing.T) {
	ctx := perfectCtx()
	ctx.Reliability.LastBackupSucceeded = confirmed(false)
	// Bring score down to ~70 by zeroing dim 2 (35 points removed).
	ctx.Resilience.StorageMediaTypes = confirmed([]string{}) // 0
	ctx.Resilience.BSLProviders = confirmed([]string{"aws"})
	ctx.Resilience.BSLRegions = confirmed([]string{"us-east-1"})
	ctx.Resilience.NetworkIsolation = confirmed(IsolationNone)

	r := Score(ctx)
	if containsString(r.CalibrationApplied, "high_score_calibration_30") {
		t.Errorf("preliminary=%d (<90), should NOT calibrate; got %v", r.TotalScore, r.CalibrationApplied)
	}
}

// ─── TC-AI-017 [P0] 滑动窗口公式 + Tier1 连失>3 归 0 ─────────────────────────

func TestTC_AI_017_SlidingWindow_StandardRate(t *testing.T) {
	cases := []struct {
		attempts, success int
		wantScore         int // success/attempts × 5 (int truncation)
	}{
		{14, 14, 5}, // 100% → 5
		{14, 7, 2},  // 50% → 2.5 truncated to 2
		{14, 1, 0},  // 7% → 0.357 truncated to 0
		{10, 10, 5},
		{30, 30, 5}, // 30 attempts (alt window)
	}
	for _, c := range cases {
		ctx := perfectCtx()
		ctx.Reliability.Last14DaysAttempts = confirmed(c.attempts)
		ctx.Reliability.Last14DaysSuccess = confirmed(c.success)
		s := findSubitem(t, Score(ctx), "reliability.success_rate_window")
		if s.Score != c.wantScore {
			t.Errorf("%d/%d: got score=%d, want %d (basis=%q)",
				c.success, c.attempts, s.Score, c.wantScore, s.Basis)
		}
	}
}

func TestTC_AI_017_Tier1_ConsecutiveFailuresOver3_ForcesZero(t *testing.T) {
	ctx := perfectCtx() // Tier1
	ctx.Reliability.Last14DaysAttempts = confirmed(14)
	ctx.Reliability.Last14DaysSuccess = confirmed(10)  // would normally be 3
	ctx.Reliability.ConsecutiveFailures = confirmed(4) // > 3

	s := findSubitem(t, Score(ctx), "reliability.success_rate_window")
	if s.Score != 0 || s.Tier != "zero" {
		t.Errorf("Tier1 + 4 consecutive failures: got %d/%q, want 0/zero", s.Score, s.Tier)
	}
}

// Edge: Tier1 + exactly 3 consecutive failures → NOT penalized (penalty is >3 strict).
func TestTC_AI_017_Tier1_Exactly3ConsecutiveFailures_NoPenalty(t *testing.T) {
	ctx := perfectCtx()
	ctx.Reliability.Last14DaysAttempts = confirmed(14)
	ctx.Reliability.Last14DaysSuccess = confirmed(14)
	ctx.Reliability.ConsecutiveFailures = confirmed(3) // boundary

	s := findSubitem(t, Score(ctx), "reliability.success_rate_window")
	if s.Score != 5 {
		t.Errorf("Tier1 + 3 consecutive failures (boundary): got %d, want 5 (no penalty at =3)", s.Score)
	}
}

// Edge: Tier2 + 5 consecutive failures → NO penalty (only Tier1 triggers it).
func TestTC_AI_017_Tier2_ConsecutiveFailures_NoPenalty(t *testing.T) {
	ctx := perfectCtx()
	ctx.Coverage.Tier = confirmed(Tier2)
	ctx.Reliability.Last14DaysAttempts = confirmed(14)
	ctx.Reliability.Last14DaysSuccess = confirmed(14)
	ctx.Reliability.ConsecutiveFailures = confirmed(5)

	s := findSubitem(t, Score(ctx), "reliability.success_rate_window")
	if s.Score != 5 {
		t.Errorf("Tier2 + 5 consecutive failures: got %d, want 5 (penalty Tier1-only)", s.Score)
	}
}

// Mars Gate 1 rule ①: attempts == 0 → unable_to_confirm, no panic.
func TestTC_AI_017_ZeroAttempts_NoDivideByZero(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Score() panicked on attempts=0: %v", r)
		}
	}()
	ctx := perfectCtx()
	ctx.Reliability.Last14DaysAttempts = confirmed(0)
	ctx.Reliability.Last14DaysSuccess = confirmed(0)

	s := findSubitem(t, Score(ctx), "reliability.success_rate_window")
	if s.Tier != "unable_to_confirm" {
		t.Errorf("attempts=0: got tier=%q, want unable_to_confirm", s.Tier)
	}
	if s.Score != 0 {
		t.Errorf("attempts=0: got score=%d, want 0", s.Score)
	}
}

// Edge: negative success count → unable_to_confirm.
func TestTC_AI_017_InvalidInput_NoDivideByZero(t *testing.T) {
	ctx := perfectCtx()
	ctx.Reliability.Last14DaysAttempts = confirmed(10)
	ctx.Reliability.Last14DaysSuccess = confirmed(-1)
	s := findSubitem(t, Score(ctx), "reliability.success_rate_window")
	if s.Tier != "unable_to_confirm" {
		t.Errorf("invalid input: got tier=%q, want unable_to_confirm", s.Tier)
	}
}

// ─── TC-AI-018 [P0] WORM 10 / 6 / 0 ──────────────────────────────────────────

func TestTC_AI_018_WORM_Compliance_Full10(t *testing.T) {
	ctx := perfectCtx()
	// Already configured for COMPLIANCE + 90d retention in perfectCtx.
	s := findSubitem(t, Score(ctx), "security.immutable_storage")
	if s.Score != 10 || s.Tier != "max" {
		t.Errorf("WORM COMPLIANCE + 90d: got %d/%q, want 10/max", s.Score, s.Tier)
	}
}

func TestTC_AI_018_WORM_Compliance_RetentionTooShort_DownTo6(t *testing.T) {
	ctx := perfectCtx()
	// Retain only 10 days — less than 30d safety margin.
	ctx.Security.WORM.RetainUntilDate = confirmed(snapshotAt.AddDate(0, 0, 10))
	s := findSubitem(t, Score(ctx), "security.immutable_storage")
	if s.Score != 6 || s.Tier != "partial" {
		t.Errorf("COMPLIANCE + 10d retention: got %d/%q, want 6/partial", s.Score, s.Tier)
	}
}

func TestTC_AI_018_WORM_Governance_6(t *testing.T) {
	ctx := perfectCtx()
	ctx.Security.WORM.Mode = confirmed(WORMGovernance)
	s := findSubitem(t, Score(ctx), "security.immutable_storage")
	if s.Score != 6 || s.Tier != "partial" {
		t.Errorf("WORM GOVERNANCE: got %d/%q, want 6/partial", s.Score, s.Tier)
	}
}

func TestTC_AI_018_WORM_None_0(t *testing.T) {
	ctx := perfectCtx()
	ctx.Security.WORM.ObjectLockEnabled = confirmed(false)
	ctx.Security.WORM.Mode = confirmed(WORMNone)
	s := findSubitem(t, Score(ctx), "security.immutable_storage")
	if s.Score != 0 || s.Tier != "zero" {
		t.Errorf("no Object Lock: got %d/%q, want 0/zero", s.Score, s.Tier)
	}
}

// Custom BusinessSafetyDays override (e.g. 60d profile).
func TestTC_AI_018_WORM_CustomSafetyDays(t *testing.T) {
	ctx := perfectCtx()
	ctx.Security.WORM.BusinessSafetyDays = 60
	ctx.Security.WORM.RetainUntilDate = confirmed(snapshotAt.AddDate(0, 0, 45)) // <60d
	s := findSubitem(t, Score(ctx), "security.immutable_storage")
	if s.Score != 6 {
		t.Errorf("60d profile + 45d retention: got %d, want 6 (insufficient margin)", s.Score)
	}
}

// ─── TC-AI-019 [P0] 缺失字段 unable_to_confirm ───────────────────────────────

// TC-AI-019: Missing fields must take the unable_to_confirm path,
// NOT be treated as "confirmed-zero".
func TestTC_AI_019_MissingFields_UnableToConfirm(t *testing.T) {
	cases := []struct {
		name      string
		mutator   func(ctx *AppContext)
		subitemID string
	}{
		{
			name: "missing hasDifferentiatedPolicies",
			mutator: func(c *AppContext) {
				c.Coverage.HasDifferentiatedPolicies = missing[bool]()
			},
			subitemID: "coverage.tier_classification",
		},
		{
			name: "missing rpoTargetSeconds",
			mutator: func(c *AppContext) {
				c.Coverage.RPOTargetSeconds = missing[int]()
			},
			subitemID: "coverage.rpo_attainment",
		},
		{
			name: "missing backupIncludesMetadata",
			mutator: func(c *AppContext) {
				c.Coverage.BackupIncludesMetadata = missing[bool]()
			},
			subitemID: "coverage.metadata_backup",
		},
		{
			name: "missing storageMediaTypes",
			mutator: func(c *AppContext) {
				c.Resilience.StorageMediaTypes = missing[[]string]()
			},
			subitemID: "resilience.media_diversity",
		},
		{
			name: "missing networkIsolation",
			mutator: func(c *AppContext) {
				c.Resilience.NetworkIsolation = missing[NetworkIsolationMode]()
			},
			subitemID: "resilience.air_gap_isolation",
		},
		{
			name: "missing WORM mode",
			mutator: func(c *AppContext) {
				c.Security.WORM.Mode = missing[WORMMode]()
			},
			subitemID: "security.immutable_storage",
		},
		{
			name: "missing drill status",
			mutator: func(c *AppContext) {
				c.Reliability.DrillStatus = missing[DrillTier]()
			},
			subitemID: "reliability.drill_passing_rate",
		},
		{
			name: "missing window attempts",
			mutator: func(c *AppContext) {
				c.Reliability.Last14DaysAttempts = missing[int]()
			},
			subitemID: "reliability.success_rate_window",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := perfectCtx()
			c.mutator(&ctx)
			s := findSubitem(t, Score(ctx), c.subitemID)
			if s.Tier != "unable_to_confirm" {
				t.Errorf("subitem %s: got tier=%q, want unable_to_confirm (Basis: %s)",
					c.subitemID, s.Tier, s.Basis)
			}
			if s.Score != 0 {
				t.Errorf("subitem %s: score=%d (should be 0 with unable_to_confirm)", c.subitemID, s.Score)
			}
		})
	}
}

// TC-AI-019 critical: missing field is NOT zero. Compare:
//   - HasDifferentiatedPolicies = confirmed(false) + zombie<5 → score=5 partial
//   - HasDifferentiatedPolicies = missing[bool]() + zombie<5 → score=0 unable_to_confirm
//
// This proves we don't treat "missing" as confirmed-false.
func TestTC_AI_019_MissingNotEqualConfirmedFalse(t *testing.T) {
	ctx1 := perfectCtx()
	ctx1.Coverage.HasDifferentiatedPolicies = confirmed(false)
	ctx1.Coverage.ZombieAssetPct = confirmed(2.0)
	s1 := findSubitem(t, Score(ctx1), "coverage.tier_classification")

	ctx2 := perfectCtx()
	ctx2.Coverage.HasDifferentiatedPolicies = missing[bool]()
	ctx2.Coverage.ZombieAssetPct = confirmed(2.0)
	s2 := findSubitem(t, Score(ctx2), "coverage.tier_classification")

	if s1.Score == s2.Score {
		t.Errorf("confirmed(false) and missing[bool]() produced same score (%d) — should differ!", s1.Score)
	}
	if s2.Tier != "unable_to_confirm" {
		t.Errorf("missing field: tier=%q, want unable_to_confirm", s2.Tier)
	}
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func containsString(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}
