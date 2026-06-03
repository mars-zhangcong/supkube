package evaluator

// v1_0_0.go — AI Backup Advisor scoring engine v1.0.0 implementation.
//
// All numbers, tier definitions, and calibration thresholds come from Mars 2026-06-03
// D-WAIT-002 decision recorded in ADR-043. DO NOT change values without bumping to v1_1_0.go.
//
// Standards alignment (ADR-043 §2):
//   - ISO/IEC 27002:2022 §8.13 Information backup        → dim1 (Backup Coverage & Compliance)
//   - NIST CSF (Protect / Respond)                       → dim2 (Resilience & Redundancy)
//   - NIST SP 1800-26 (Data Integrity)                   → dim3 (Immutability & Security)
//   - NIST SP 800-53 Rev.5 CP-9 (System Backup)          → dim4 (Reliability & Verification)
//
// Determinism guarantees (Mars Gate 1 rule ① + TC-AI-003):
//   - No time.Now() — only ctx.SnapshotAt is read for time math.
//   - No rand.* — no randomness.
//   - No map iteration in output paths — all Subitems are appended in fixed order.
//   - Defensive: attempts==0 → unable_to_confirm (no divide-by-zero).

import "time"

// Score is the public entry point. Deterministic pure function.
//
// Algorithm (high-level):
//  1. Pre-condition gate (Mars 2026-06-03 §4.2.1): if Policy never executed
//     (LastBackupAt unconfirmed or nil), return early with EvalStatusNotEligibleNoRuns
//     — do NOT compute Dimensions/TotalScore. UI prompts customer to deploy + run Policy first.
//  2. Compute each dimension's subitem scores → DimensionScore.
//  3. Sum dimensions → preliminary TotalScore.
//  4. Apply hard threshold (Mars 2026-06-03 修订: only ONE hard threshold remains):
//     - high_score_calibration_30: TotalScore ≥ 90 && LastBackupSucceeded == false
//     → force 30 + Critical. (Initial "no_backup_cap_30" replaced by step 1's not_eligible path.)
//  5. Classify final score into 4-tier SecurityLevel (90/75/60 inclusive boundaries).
//  6. Stamp Status=scored + ScoreRulesVersion + EvaluatedAt (= SnapshotAt, NOT time.Now).
func Score(ctx AppContext) ScoreResult {
	// Step 1: Pre-condition gate — Policy must have executed at least once.
	if status, reason := checkEligibility(ctx); status != EvalStatusScored {
		return ScoreResult{
			Status:            status,
			NotEligibleReason: reason,
			ScoreRulesVersion: ScoreRulesVersion,
			EvaluatedAt:       ctx.SnapshotAt,
			// TotalScore, Level, Dimensions deliberately left as zero values — not meaningful.
		}
	}

	// Step 2-3: Compute 4 dimensions.
	dim1 := scoreCoverage(ctx.Coverage)
	dim2 := scoreResilience(ctx.Resilience)
	dim3 := scoreSecurity(ctx.Security, ctx.SnapshotAt)
	dim4 := scoreReliability(ctx.Reliability, ctx.Coverage.Tier)

	preliminary := dim1.Score + dim2.Score + dim3.Score + dim4.Score

	// Step 4: Apply hard threshold (only one remains after Mars 2026-06-03 revision).
	final, calibrations := applyHardThresholds(preliminary, ctx.Reliability)

	// Step 5-6: Build ScoreResult with Status=scored.
	return ScoreResult{
		Status:            EvalStatusScored,
		ScoreRulesVersion: ScoreRulesVersion,
		TotalScore:        final,
		Level:             classifyLevel(final, calibrations),
		Dimensions: Dimensions{
			BackupCoverage:          dim1,
			Resilience:              dim2,
			ImmutabilityAndSecurity: dim3,
			Reliability:             dim4,
		},
		CalibrationApplied: calibrations,
		EvaluatedAt:        ctx.SnapshotAt,
	}
}

// checkEligibility — Mars 2026-06-03 §4.2.1 evaluator pre-condition gate.
//
// Returns (EvalStatusScored, "") if all pre-conditions pass; else returns a not_eligible status.
//
// Current rules:
//   - LastBackupAt not confirmed OR Value == nil → EvalStatusNotEligibleNoRuns
//     (Policy maybe deployed, but never actually ran → no RP yet → 不参评)
//
// Future extension: when Policy-binding signal is added to AppContext (collector layer),
// add EvalStatusNotEligibleNoPolicy check before this one.
func checkEligibility(ctx AppContext) (EvaluationStatus, string) {
	r := ctx.Reliability
	noBackup := !r.LastBackupAt.IsConfirmed() || r.LastBackupAt.Value == nil
	if noBackup {
		return EvalStatusNotEligibleNoRuns,
			"灾备策略未执行, 无还原点 — 建议客户先定义 + 执行 Policy 后再评分"
	}
	return EvalStatusScored, ""
}

// ─── classifyLevel: 4-tier SecurityLevel from final score ─────────────────────
//
// PRD-011 §4.2 公式 3 / ADR-043 §2 (Mars D-WAIT-002):
//   - 90-100 → high_resilience
//   - 75-89  → compliant_low_risk
//   - 60-74  → fragile
//   - <60    → critical
//
// Boundary rule (TC-AI-004): 90 / 75 / 60 INCLUSIVE on the upper tier (含上界).
//   - 90 → high_resilience (not compliant_low_risk)
//   - 89 → compliant_low_risk
//   - 75 → compliant_low_risk
//   - 74 → fragile
//   - 60 → fragile
//   - 59 → critical
//
// Hard threshold override: if any calibration fired, force LevelCritical regardless of score.
// (PRD-011 §4.2 Q4: hard thresholds always classify as "高危级" = critical.)
func classifyLevel(score int, calibrations []string) SecurityLevel {
	if len(calibrations) > 0 {
		return LevelCritical
	}
	switch {
	case score >= 90:
		return LevelHighResilience
	case score >= 75:
		return LevelCompliantLowRisk
	case score >= 60:
		return LevelFragile
	default:
		return LevelCritical
	}
}

// ─── applyHardThresholds: 1 calibration rule (Mars 2026-06-03 §4.2.5) ─────────
//
// Mars 2026-06-03 修订: the initial "no_backup_cap_30" rule (LastBackupAt==nil → cap 30)
// was removed and replaced by the EvalStatusNotEligibleNoRuns path in checkEligibility().
// Reason: "封顶 30 + 评分" 给客户错误的安全感; 改为"不参评 + 提示先跑首次备份"更诚实.
//
// Remaining rule (high_score_calibration_30):
//
//	IF preliminary >= 90 && LastBackupSucceeded confirmed false (most recent failed) THEN
//	  final = 30; level = Critical
//	Reason: 算分看似完美但最近备份失败 → 配置漂亮但实际备份没落地, 防虚高骗客户.
func applyHardThresholds(preliminary int, r ReliabilityInput) (final int, calibrations []string) {
	final = preliminary

	// High-score calibration: only fires if preliminary >= 90 AND we KNOW the most recent
	// attempt failed (LastBackupSucceeded confirmed false). If status is unable_to_confirm,
	// we don't presume failure (innocent until proven guilty).
	if preliminary >= 90 && r.LastBackupSucceeded.IsConfirmed() && !r.LastBackupSucceeded.Value {
		final = 30
		calibrations = append(calibrations, "high_score_calibration_30")
	}

	return final, calibrations
}

// ─── Dimension 1: Backup Coverage & Compliance (25 max) ───────────────────────
//
// ISO/IEC 27002:2022 §8.13.1.a — Information backup business requirements & RPO compliance.
//
// 3 subitems:
//  1. application_tier_classification (10)
//  2. rpo_attainment (10)
//  3. metadata_backup (5)
func scoreCoverage(c CoverageInput) DimensionScore {
	subitems := []SubitemScore{
		scoreAppTierClassification(c),
		scoreRPOAttainment(c),
		scoreMetadataBackup(c),
	}
	total := 0
	for _, s := range subitems {
		total += s.Score
	}
	return DimensionScore{Score: total, MaxScore: 25, Subitems: subitems}
}

// subitem 1.1 — Application tier classification & core coverage (10).
//
// PRD-011 §4.2 dim1: "Tier 分级差异化+100%核心纳管=10 / 统一未分级或<5%僵尸=5 / 未分级有漏备=0"
//
// Boundary interpretation (deliberate; documented for Gate 1 audit):
//   - 10: HasDifferentiatedPolicies=true AND CoreCoveragePct >= 100
//   - 5:  (HasDifferentiatedPolicies=false AND ZombieAssetPct < 5)          ← 未分级但很少僵尸
//     OR (HasDifferentiatedPolicies=true AND CoreCoveragePct < 100)     ← 分级了但核心未全纳管
//   - 0:  HasDifferentiatedPolicies=false AND ZombieAssetPct >= 5           ← 未分级 + 有漏备
//   - unable_to_confirm: any input field has non-Confirmed status.
//
// "<5% 僵尸" includes 4.999...%, excludes 5.0% (strict <). 5% is the boundary; tested in TC-AI-002.
func scoreAppTierClassification(c CoverageInput) SubitemScore {
	id := "coverage.tier_classification"
	const maxScore = 10

	if !c.HasDifferentiatedPolicies.IsConfirmed() ||
		!c.CoreCoveragePct.IsConfirmed() ||
		!c.ZombieAssetPct.IsConfirmed() {
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "missing inputs: hasDifferentiatedPolicies / coreCoveragePct / zombieAssetPct",
		}
	}

	hasDiff := c.HasDifferentiatedPolicies.Value
	coreCov := c.CoreCoveragePct.Value
	zombie := c.ZombieAssetPct.Value

	if hasDiff && coreCov >= 100 {
		return SubitemScore{
			ID: id, Score: 10, MaxScore: maxScore, Tier: "max",
			Basis: "tier-differentiated policies AND 100% core asset coverage (ISO 27002 §8.13.1.a)",
		}
	}
	// "5 partial" tier: unified-non-tiered AND zombie<5%, OR tiered-but-incomplete-core.
	if (!hasDiff && zombie < 5) || (hasDiff && coreCov < 100) {
		return SubitemScore{
			ID: id, Score: 5, MaxScore: maxScore, Tier: "partial",
			Basis: "unified-non-tiered with <5% zombie OR tier-differentiated but core <100%",
		}
	}
	return SubitemScore{
		ID: id, Score: 0, MaxScore: maxScore, Tier: "zero",
		Basis: "non-tiered AND zombie ≥5% (有漏备)",
	}
}

// subitem 1.2 — RPO attainment (10).
//
// PRD-011 §4.2 dim1: "满足或优于=10 / 基本满足但窗口过大=5 / 不满足=0"
//
// Boundary interpretation (NOTE for Mars Gate 1 audit: "窗口过大" is not numerically defined
// in PRD-011 §4.2; I interpret as "actual RPO between target and 1.5× target = '基本满足但过大'",
// which is the industry-common convention used in SLO design. If Mars wants stricter or
// looser, bump to v1_0_1 with revised constant):
//   - 10: RPOActualSeconds <= RPOTargetSeconds (满足或优于)
//   - 5:  RPOTargetSeconds < RPOActualSeconds <= 1.5 × RPOTargetSeconds (基本满足但过大)
//   - 0:  RPOActualSeconds > 1.5 × RPOTargetSeconds (不满足)
//   - unable_to_confirm: either input field non-confirmed.
func scoreRPOAttainment(c CoverageInput) SubitemScore {
	id := "coverage.rpo_attainment"
	const maxScore = 10

	if !c.RPOActualSeconds.IsConfirmed() || !c.RPOTargetSeconds.IsConfirmed() {
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "missing inputs: rpoActualSeconds / rpoTargetSeconds",
		}
	}
	actual := c.RPOActualSeconds.Value
	target := c.RPOTargetSeconds.Value
	if target <= 0 {
		// target = 0 or negative means business hasn't set a target; can't compute attainment.
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "rpoTargetSeconds <= 0 (no business RPO target set)",
		}
	}
	if actual <= target {
		return SubitemScore{
			ID: id, Score: 10, MaxScore: maxScore, Tier: "max",
			Basis: "actual RPO ≤ target (meets or exceeds SLO)",
		}
	}
	// "1.5× target" boundary is documented interpretation; see function-level comment.
	if actual*2 <= target*3 { // equivalent to actual ≤ 1.5 * target without floats
		return SubitemScore{
			ID: id, Score: 5, MaxScore: maxScore, Tier: "partial",
			Basis: "target < actual ≤ 1.5× target (basically meets but window enlarged)",
		}
	}
	return SubitemScore{
		ID: id, Score: 0, MaxScore: maxScore, Tier: "zero",
		Basis: "actual > 1.5× target (does not meet RPO)",
	}
}

// subitem 1.3 — Metadata backup (5).
//
// PRD-011 §4.2 dim1: "含 Manifest/拓扑/配置=5 / 仅纯数据=0"
//
// Boolean: BackupIncludesMetadata true → 5, false → 0, unconfirmed → unable_to_confirm.
func scoreMetadataBackup(c CoverageInput) SubitemScore {
	id := "coverage.metadata_backup"
	const maxScore = 5
	if !c.BackupIncludesMetadata.IsConfirmed() {
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "missing input: backupIncludesMetadata",
		}
	}
	if c.BackupIncludesMetadata.Value {
		return SubitemScore{
			ID: id, Score: 5, MaxScore: maxScore, Tier: "max",
			Basis: "backups include K8s manifests / topology / configuration",
		}
	}
	return SubitemScore{
		ID: id, Score: 0, MaxScore: maxScore, Tier: "zero",
		Basis: "data-only backup, no metadata captured (cannot one-click rebuild)",
	}
}

// ─── Dimension 2: Resilience & Redundancy (35 max) ────────────────────────────
//
// NIST CSF (Protect / Respond) — 3-2-1-1-0 implementation.
//
// 3 subitems:
//  1. media_diversity (10)    【3-2】
//  2. offsite_cross_cloud (10) 【1·异地】
//  3. air_gap_isolation (15)   【1·隔离/不可变 network layer】
func scoreResilience(r ResilienceInput) DimensionScore {
	subitems := []SubitemScore{
		scoreMediaDiversity(r),
		scoreOffsiteCrossCloud(r),
		scoreAirGapIsolation(r),
	}
	total := 0
	for _, s := range subitems {
		total += s.Score
	}
	return DimensionScore{Score: total, MaxScore: 35, Subitems: subitems}
}

// subitem 2.1 — Media diversity (10).
//
// PRD-011 §4.2 dim2 [3-2]: "≥2 种介质=10 / 单介质=5"
//
// Boundary note: PRD has no "0" tier for media diversity (minimum is 5 if at least 1 media).
// If StorageMediaTypes is empty AND confirmed (i.e. NO backup destination configured at all),
// I interpret as "unable_to_confirm" rather than score=5, since you literally have no media.
// This conservative read assumes "5 = at least 1 media" semantics.
func scoreMediaDiversity(r ResilienceInput) SubitemScore {
	id := "resilience.media_diversity"
	const maxScore = 10
	if !r.StorageMediaTypes.IsConfirmed() {
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "missing input: storageMediaTypes",
		}
	}
	n := uniqueCount(r.StorageMediaTypes.Value)
	if n >= 2 {
		return SubitemScore{
			ID: id, Score: 10, MaxScore: maxScore, Tier: "max",
			Basis: "≥2 distinct storage media (3-2-1 rule satisfied)",
		}
	}
	if n == 1 {
		return SubitemScore{
			ID: id, Score: 5, MaxScore: maxScore, Tier: "partial",
			Basis: "single storage medium (single-point-of-failure on media)",
		}
	}
	return SubitemScore{
		ID: id, Score: 0, MaxScore: maxScore, Tier: "zero",
		Basis: "no storage media configured (no backup destination)",
	}
}

// subitem 2.2 — Offsite / cross-cloud (10).
//
// PRD-011 §4.2 dim2 [1·异地] + H2 finding closure:
//
//	"跨厂商或>100km=10 / 同厂商跨 region=6 / 同 AZ 同源=0"
//
// H2 闭环 (2026-06-02): use BSL region+provider+bucket triple for "异地" judgment.
//   - 10 = MultiProvider (len(providers) > 1)   ← cross-cloud
//   - 10 = SingleProvider && MultiRegion        ← cross-region same-cloud (treat as offsite high)
//     wait, PRD says "同厂商跨 region=6" — so single-provider-multi-region = 6, NOT 10.
//   - 10 = MultiProvider                        ← only multi-provider gets 10
//   - 6  = SingleProvider && MultiRegion
//   - 0  = SingleProvider && SingleRegion
//
// We don't have "100km" distance data, so PRD's "跨厂商 或 >100km" condition is implemented
// as "MultiProvider" only. The ">100km" subclause requires geo-distance data not in AppContext;
// I treat it as syntactic sugar that's subsumed by MultiProvider in practice (cross-cloud
// is almost always >100km).
func scoreOffsiteCrossCloud(r ResilienceInput) SubitemScore {
	id := "resilience.offsite_cross_cloud"
	const maxScore = 10
	if !r.BSLProviders.IsConfirmed() || !r.BSLRegions.IsConfirmed() {
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "missing inputs: bslProviders / bslRegions",
		}
	}
	providers := uniqueCount(r.BSLProviders.Value)
	regions := uniqueCount(r.BSLRegions.Value)

	if providers >= 2 {
		return SubitemScore{
			ID: id, Score: 10, MaxScore: maxScore, Tier: "max",
			Basis: "multi-provider (cross-cloud offsite, >100km implied)",
		}
	}
	if providers == 1 && regions >= 2 {
		return SubitemScore{
			ID: id, Score: 6, MaxScore: maxScore, Tier: "partial",
			Basis: "single-provider multi-region (cross-region but no multi-cloud isolation)",
		}
	}
	return SubitemScore{
		ID: id, Score: 0, MaxScore: maxScore, Tier: "zero",
		Basis: "single AZ or single region with single provider (no offsite redundancy)",
	}
}

// subitem 2.3 — Air-gap & network isolation (15).
//
// PRD-011 §4.2 dim2 [1·隔离]:
//
//	"真 Air-Gap 或专网备完即断=15 / 有隔离但同网络域/同 AD=8 / 无隔离生产可直挂=0"
//
// Straightforward enum mapping from NetworkIsolation field.
func scoreAirGapIsolation(r ResilienceInput) SubitemScore {
	id := "resilience.air_gap_isolation"
	const maxScore = 15
	if !r.NetworkIsolation.IsConfirmed() {
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "missing input: networkIsolation",
		}
	}
	switch r.NetworkIsolation.Value {
	case IsolationAirGap:
		return SubitemScore{
			ID: id, Score: 15, MaxScore: maxScore, Tier: "max",
			Basis: "true air-gap or dedicated network with disconnect-after-backup",
		}
	case IsolationSameADDomain:
		return SubitemScore{
			ID: id, Score: 8, MaxScore: maxScore, Tier: "partial",
			Basis: "network isolation present but same domain/AD (lateral movement risk)",
		}
	case IsolationNone:
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore, Tier: "zero",
			Basis: "no isolation; production can directly mount backup destination",
		}
	default:
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "unknown networkIsolation enum value",
		}
	}
}

// ─── Dimension 3: Immutability & Security (20 max) ────────────────────────────
//
// NIST SP 1800-26 (Data Integrity: Detecting and Responding to Ransomware) +
// ISO/IEC 27002:2022 §8.13.1.c (encryption requirement).
//
// 3 subitems:
//  1. immutable_storage (10) — WORM mode + retention margin
//  2. encryption_credentials (5)
//  3. access_control (5)
func scoreSecurity(s SecurityInput, snapshotAt time.Time) DimensionScore {
	subitems := []SubitemScore{
		scoreImmutableStorage(s.WORM, snapshotAt),
		scoreEncryptionCredentials(s.Encryption),
		scoreAccessControl(s.AccessControl),
	}
	total := 0
	for _, s := range subitems {
		total += s.Score
	}
	return DimensionScore{Score: total, MaxScore: 20, Subitems: subitems}
}

// subitem 3.1 — Immutable storage / WORM (10).
//
// PRD-011 §4.2 dim3 + formula 2:
//
//	"WORM/Object Lock COMPLIANCE 模式=10 / Governance 模式或仅 IAM=6 / 可覆盖删除=0"
//
//	Formula 2: 10 IFF ObjectLockEnabled==true AND Mode==COMPLIANCE AND
//	           RetainUntilDate > SnapshotAt + BusinessSafetyDays (default 30d).
//	Otherwise:
//	  - Mode == GOVERNANCE → 6
//	  - All else (no enable, no mode, retention too short) → 0
//
// TC-AI-018 explicitly tests this 10/6/0 mapping.
func scoreImmutableStorage(w WORMInput, snapshotAt time.Time) SubitemScore {
	id := "security.immutable_storage"
	const maxScore = 10

	// All three fields must be confirmed to make a determination.
	if !w.ObjectLockEnabled.IsConfirmed() || !w.Mode.IsConfirmed() {
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "missing inputs: objectLockEnabled / mode",
		}
	}

	safetyDays := w.BusinessSafetyDays
	if safetyDays <= 0 {
		safetyDays = BusinessSafetyDaysDefault
	}

	// Compliance mode → check retention.
	if w.ObjectLockEnabled.Value && w.Mode.Value == WORMCompliance {
		if !w.RetainUntilDate.IsConfirmed() {
			return SubitemScore{
				ID: id, Score: 0, MaxScore: maxScore,
				Tier:  "unable_to_confirm",
				Basis: "COMPLIANCE mode but retainUntilDate unconfirmed",
			}
		}
		threshold := snapshotAt.AddDate(0, 0, safetyDays)
		if w.RetainUntilDate.Value.After(threshold) {
			return SubitemScore{
				ID: id, Score: 10, MaxScore: maxScore, Tier: "max",
				Basis: "Object Lock COMPLIANCE mode AND retention > snapshot+30d (NIST SP 1800-26)",
			}
		}
		// COMPLIANCE but retention insufficient → still treat as Governance-tier (6).
		// Reasoning: COMPLIANCE mode itself is privilege-resistant, just retention is short.
		return SubitemScore{
			ID: id, Score: 6, MaxScore: maxScore, Tier: "partial",
			Basis: "COMPLIANCE mode but retainUntilDate ≤ snapshot+30d (insufficient margin)",
		}
	}

	// Governance mode → 6.
	if w.ObjectLockEnabled.Value && w.Mode.Value == WORMGovernance {
		return SubitemScore{
			ID: id, Score: 6, MaxScore: maxScore, Tier: "partial",
			Basis: "Object Lock GOVERNANCE mode (privileged accounts can override)",
		}
	}

	// No Object Lock or unknown mode → 0.
	return SubitemScore{
		ID: id, Score: 0, MaxScore: maxScore, Tier: "zero",
		Basis: "no Object Lock or unknown mode; backup data can be overwritten/deleted",
	}
}

// subitem 3.2 — Encryption & credentials (5).
//
// PRD-011 §4.2 dim3:
//
//	"静态 AES-256+传输 TLS1.3+KMS 独立轮换=5 / 有加密但密钥同管=2"
//
// Boundary interpretation:
//   - 5: all 3 (StaticAES256 && TransitTLS13 && KMSIndependentRotation) are true & confirmed.
//   - 2: at least one of (StaticAES256, TransitTLS13) is true & confirmed (some encryption)
//     but KMSIndependentRotation is false or any field is unconfirmed.
//   - 0: all encryption flags are false & confirmed (no encryption at all).
//   - unable_to_confirm: any encryption flag unconfirmed AND no positive evidence.
//
// PRD has no "0" tier explicit but reasonably implied (no encryption ≠ "有加密但密钥同管").
func scoreEncryptionCredentials(e EncryptionInput) SubitemScore {
	id := "security.encryption_credentials"
	const maxScore = 5

	// If all 3 fields unconfirmed, we have nothing to evaluate.
	if !e.StaticAES256.IsConfirmed() && !e.TransitTLS13.IsConfirmed() && !e.KMSIndependentRotation.IsConfirmed() {
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "all encryption fields unconfirmed",
		}
	}

	// Full credit: all 3 confirmed true.
	if e.StaticAES256.IsConfirmed() && e.StaticAES256.Value &&
		e.TransitTLS13.IsConfirmed() && e.TransitTLS13.Value &&
		e.KMSIndependentRotation.IsConfirmed() && e.KMSIndependentRotation.Value {
		return SubitemScore{
			ID: id, Score: 5, MaxScore: maxScore, Tier: "max",
			Basis: "AES-256 at rest + TLS 1.3 in transit + KMS independent rotation",
		}
	}

	// Partial: some encryption confirmed-on, but not all (or KMS not independent).
	hasAnyEncryption := (e.StaticAES256.IsConfirmed() && e.StaticAES256.Value) ||
		(e.TransitTLS13.IsConfirmed() && e.TransitTLS13.Value)
	if hasAnyEncryption {
		return SubitemScore{
			ID: id, Score: 2, MaxScore: maxScore, Tier: "partial",
			Basis: "encryption present but key management not independently rotated",
		}
	}

	// All confirmed flags are false → no encryption at all.
	return SubitemScore{
		ID: id, Score: 0, MaxScore: maxScore, Tier: "zero",
		Basis: "no encryption at rest or in transit",
	}
}

// subitem 3.3 — Access control (5, graduated 5/3/1/0 — Mars 2026-06-03 Option A).
//
// PRD-011 §4.2 dim3 (updated 2026-06-03):
//
//	"MFA+RBAC+删除二次审批+审计异地不可篡改 graduated 5 档:
//	 4 项满足=5 / 3 项=3 / 2 项=1 / ≤1 项=0"
//
// Mars 业务理由: "让客户知道, 自己要进行配置" — 渐进激励改进, 不再 all-or-nothing.
//
// Mapping rationale (Option A, Mars confirmed):
//   - n=4 (full stack)        → 5 (full credit)
//   - n=3 (mostly there)      → 3 (substantial credit)
//   - n=2 (half-built)        → 1 (token credit, significant gaps remain)
//   - n≤1 (single control)    → 0 (single security control = false sense of security,
//     equivalent to "single-password baseline" in attack-surface terms)
//
// Per Mars's Gate 1 rule (only count Confirmed true items):
//   - Status != Confirmed   → counted as "not satisfied" (conservative: missing data = no credit)
//   - Status == Confirmed && Value == false → counted as "not satisfied"
//   - Status == Confirmed && Value == true  → counted as "satisfied" (counts toward n)
//
// Special case: all 4 fields unconfirmed → unable_to_confirm tier (no determination possible).
func scoreAccessControl(a AccessControlInput) SubitemScore {
	id := "security.access_control"
	const maxScore = 5

	// All 4 unconfirmed → can't make any determination.
	if !a.MFAEnabled.IsConfirmed() && !a.RBACEnabled.IsConfirmed() &&
		!a.DeleteRequiresApproval.IsConfirmed() && !a.AuditOffsiteTamperProof.IsConfirmed() {
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "all access-control fields unconfirmed",
		}
	}

	// Count satisfied items: only Confirmed-true counts.
	satisfied := 0
	if a.MFAEnabled.IsConfirmed() && a.MFAEnabled.Value {
		satisfied++
	}
	if a.RBACEnabled.IsConfirmed() && a.RBACEnabled.Value {
		satisfied++
	}
	if a.DeleteRequiresApproval.IsConfirmed() && a.DeleteRequiresApproval.Value {
		satisfied++
	}
	if a.AuditOffsiteTamperProof.IsConfirmed() && a.AuditOffsiteTamperProof.Value {
		satisfied++
	}

	switch satisfied {
	case 4:
		return SubitemScore{
			ID: id, Score: 5, MaxScore: maxScore, Tier: "max",
			Basis: "4/4 satisfied: MFA + RBAC + 2nd-approval + tamper-proof audit (full ISO/NIST stack)",
		}
	case 3:
		return SubitemScore{
			ID: id, Score: 3, MaxScore: maxScore, Tier: "partial-high",
			Basis: "3/4 access controls satisfied (substantial but one gap)",
		}
	case 2:
		return SubitemScore{
			ID: id, Score: 1, MaxScore: maxScore, Tier: "partial-low",
			Basis: "2/4 access controls satisfied (significant gaps; configure remaining items)",
		}
	default: // 0 or 1
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore, Tier: "zero",
			Basis: "≤1/4 access controls satisfied (single-point security = false sense of security)",
		}
	}
}

// ─── Dimension 4: Reliability & Verification (20 max) ─────────────────────────
//
// NIST SP 800-53 Rev.5 CP-9 (System Backup) + ISO/IEC 27002:2022 §8.13.1.b (test).
//
// 2 subitems:
//  1. success_rate_window (5)  — sliding window formula (公式 1)
//  2. drill_passing_rate (15)  — 【0·验证】recovery drill maturity
func scoreReliability(r ReliabilityInput, tier Field[Tier]) DimensionScore {
	subitems := []SubitemScore{
		scoreSuccessRateWindow(r, tier),
		scoreDrillPassingRate(r),
	}
	total := 0
	for _, s := range subitems {
		total += s.Score
	}
	return DimensionScore{Score: total, MaxScore: 20, Subitems: subitems}
}

// subitem 4.1 — Success rate sliding window (5).
//
// Formula 1: success/total × 5 (rounded down to int).
// Penalty: Tier 1 with ConsecutiveFailures > 3 → 0 (regardless of window rate).
//
// Defensive (Mars Gate 1 rule ①): attempts == 0 → unable_to_confirm (NO divide-by-zero panic).
// Unconfirmed inputs also → unable_to_confirm.
func scoreSuccessRateWindow(r ReliabilityInput, tier Field[Tier]) SubitemScore {
	id := "reliability.success_rate_window"
	const maxScore = 5

	if !r.Last14DaysAttempts.IsConfirmed() || !r.Last14DaysSuccess.IsConfirmed() {
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "missing inputs: last14DaysAttempts / last14DaysSuccess",
		}
	}

	attempts := r.Last14DaysAttempts.Value
	success := r.Last14DaysSuccess.Value

	if attempts <= 0 {
		// Mars Gate 1 rule ①: zero attempts → unable_to_confirm, no divide-by-zero.
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "no backup attempts in 14-day window (cannot compute rate)",
		}
	}
	if success < 0 || success > attempts {
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "invalid input: success out of range [0, attempts]",
		}
	}

	// Tier 1 penalty: consecutive failures > 3 → force 0.
	// Applies only when tier is confirmed Tier1 AND ConsecutiveFailures confirmed >3.
	if tier.IsConfirmed() && tier.Value == Tier1 &&
		r.ConsecutiveFailures.IsConfirmed() && r.ConsecutiveFailures.Value > 3 {
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore, Tier: "zero",
			Basis: "Tier 1 core asset with consecutive failures >3 (local data outage risk)",
		}
	}

	// Standard rate formula: success/attempts × 5, integer truncation.
	score := (success * maxScore) / attempts
	return SubitemScore{
		ID:       id,
		Score:    score,
		MaxScore: maxScore,
		Tier:     classifySuccessRateTier(score, maxScore),
		Basis:    formatSuccessRateBasis(success, attempts),
	}
}

// classifySuccessRateTier returns a tier label for the success-rate subitem
// based on the integer score relative to maxScore. Used for human-readable Breakdown.
func classifySuccessRateTier(score, maxScore int) string {
	switch {
	case score == maxScore:
		return "max"
	case score >= maxScore/2:
		return "partial"
	default:
		return "low"
	}
}

// formatSuccessRateBasis returns a stable string for the basis (no time-based content).
func formatSuccessRateBasis(success, attempts int) string {
	return itoa(success) + "/" + itoa(attempts) + " success rate × 5 (sliding 14-day window)"
}

// subitem 4.2 — Recovery drill passing rate (15) — 【0·验证】.
//
// PRD-011 §4.2 dim4:
//
//	"全自动沙箱+近3月100%=15 / 定期人工演练有报告=10 / 仅局部文件恢复测试=5 / 只备不练=0"
//
// Direct enum mapping. Unconfirmed → unable_to_confirm.
func scoreDrillPassingRate(r ReliabilityInput) SubitemScore {
	id := "reliability.drill_passing_rate"
	const maxScore = 15
	if !r.DrillStatus.IsConfirmed() {
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "missing input: drillStatus",
		}
	}
	switch r.DrillStatus.Value {
	case DrillFullAuto3mo100pct:
		return SubitemScore{
			ID: id, Score: 15, MaxScore: maxScore, Tier: "max",
			Basis: "fully automated sandbox recovery with 100% pass rate in last 3 months",
		}
	case DrillPeriodicManual:
		return SubitemScore{
			ID: id, Score: 10, MaxScore: maxScore, Tier: "partial-high",
			Basis: "periodic manual recovery drills with documented reports",
		}
	case DrillPartialFile:
		return SubitemScore{
			ID: id, Score: 5, MaxScore: maxScore, Tier: "partial-low",
			Basis: "only partial file-level recovery tests, no system-level drill",
		}
	case DrillNone:
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore, Tier: "zero",
			Basis: "no recovery drills performed (只备不练 — backup without verification)",
		}
	default:
		return SubitemScore{
			ID: id, Score: 0, MaxScore: maxScore,
			Tier:  "unable_to_confirm",
			Basis: "unknown drillStatus enum value",
		}
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// uniqueCount returns the number of distinct non-empty strings in a slice.
// Deterministic (no map iteration on output path; just a counter).
func uniqueCount(items []string) int {
	if len(items) == 0 {
		return 0
	}
	seen := make(map[string]struct{}, len(items))
	for _, s := range items {
		if s != "" {
			seen[s] = struct{}{}
		}
	}
	return len(seen)
}

// itoa is a small int→ASCII helper (avoids importing strconv just for this).
// Deterministic.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
