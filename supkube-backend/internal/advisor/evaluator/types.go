// Package evaluator implements the AI Backup Advisor deterministic scoring engine v1.0.0.
//
// # Overview
//
// This package provides the new scoring path for PRD-011 AI Backup Advisor MVP:
// a pure function Score(AppContext) → ScoreResult scoring a Kubernetes application's
// data resilience on a 100-point scale across 4 dimensions per ADR-043 (Mars D-WAIT-002).
//
//   - Deterministic: no time.Now(), no rand. "Freshness" / sliding windows read SnapshotAt
//     from AppContext, NOT wall-clock time. Same input → same output.
//   - Versioned: every ScoreResult carries scoreRulesVersion="v1.0.0" (PRD-011 §12 H1).
//     When rules change, bump to v1_1_0.go (multi-version coexistence; old scores reproducible).
//   - Pure: no I/O, no goroutines, no maps with non-deterministic iteration affecting output.
//     LLM is NEVER in this code path (PRD-011 finding #2; LLM only explains, never scores).
//
// # Two-Scorer Tech Debt (2026-06-03, Mars D-WAIT-002 Gate 0)
//
// SupKube currently has TWO scoring paths running in parallel:
//
//  1. LEGACY: internal/api/v1/applications.go scoreNamespaceForAdvisor() →
//     AdvisorRecommendation. Serves GET /applications list & detail UI.
//     Uses heuristic K8s-API-based scoring (+40 HasPVC / +30 critical tier / -30
//     stateless-no-svc). NOT aligned with ADR-043 4-dimension matrix.
//
//  2. NEW (this package): evaluator.Score() → ScoreResult. Pure function, deterministic,
//     versioned. Implements ADR-043 (4 dimensions × subitems × tiers).
//     Future endpoint: /ai/score (PRD-011 §4.6, H5 闭环).
//
// # Convergence Point (Tracked: Task #115 v0.10.x-AI-ADVISOR Phase A)
//
// In v0.10.x Phase A, scoreNamespaceForAdvisor() should be migrated to call evaluator.Score()
// via a K8s-API → AppContext adapter (collector layer, M1-M5 per PRD-011 §4.1). At that point
// the two paths collapse into one. This is OUT OF SCOPE for v0.9.x (current sprint, PRD-011 MVP).
//
// Until then, two scorers coexist serving different endpoints:
//   - applications.go.scoreNamespaceForAdvisor() — GET /applications list (existing UI)
//   - evaluator.Score() — POST /ai/score (PRD-011 MVP endpoint, to be wired up later)
//
// # References
//
//   - PRD-011 §4.2: Scoring matrix definition (authoritative source for tiers & numbers)
//   - ADR-043: AI Backup Advisor 评分细则 v1.0.0 (4 dimensions + Mars D-WAIT-002 numbers
//     25/35/20/20; hard thresholds; 4-tier security level 90/75/60)
//   - ADR-037: Canonical DSL fact/observation/inferred/evidence. This package uses Field[T]
//     as a simplified `fact` carrier (value + status only; full evidence_refs deferred to
//     collector layer in v0.10.x).
//   - 测试用例.md §19 TC-AI-MVP: P0 test cases this package must pass.
//   - D-WAIT-002 2026-06-03: Mars-locked 4-dim weights & hard thresholds.
//   - D-WAIT-005 2026-06-03: "应尽则尽" execution discipline (this package follows it).
package evaluator

import "time"

// ScoreRulesVersion is the version string emitted on every ScoreResult.
// Bump when rules change (will require new file v1_1_0.go to coexist for old-score replay).
const ScoreRulesVersion = "v1.0.0"

// ─── EvaluationStatus (Mars 2026-06-03 拍板) ──────────────────────────────────
//
// not_eligible 路径替代初版"无备份封顶 30"硬阈值:
//   - 配置看似完善但 Policy 从未执行 → 不参评 (返回 Status != scored, 不算 TotalScore)
//   - 前端不显示"30 分", 而显示提示卡"建议客户先定义 + 执行 Policy"
// 详 PRD-011 §4.2.1 + ADR-043 §2.

// EvaluationStatus signals whether the scoring went through normal 4-dim flow,
// or hit a pre-condition that disqualifies evaluation.
type EvaluationStatus string

const (
	// EvalStatusScored — normal 4-dim scoring completed; TotalScore / Level / Dimensions are meaningful.
	EvalStatusScored EvaluationStatus = "scored"
	// EvalStatusNotEligibleNoPolicy — application has no backup Policy bound yet.
	// UI should prompt: "未配置备份 Policy, 请先在 Policies 页创建"
	EvalStatusNotEligibleNoPolicy EvaluationStatus = "not_eligible_no_policy"
	// EvalStatusNotEligibleNoRuns — Policy deployed but never executed (no RP generated).
	// UI should prompt: "Policy 未执行, 请先跑首次备份"
	// Mars 业务语义: 灾备策略未执行, 无还原点 → 不评分, 给提示
	EvalStatusNotEligibleNoRuns EvaluationStatus = "not_eligible_no_runs"
)

// BusinessSafetyDaysDefault is the default minimum WORM RetainUntilDate margin (PRD-011 §4.2
// formula 2: RetainUntilDate > SnapshotAt + this many days → satisfies WORM full-score).
// Mars 2026-06-03 D-WAIT-002 referenced "如 30 天"; clients can override via AppContext.
const BusinessSafetyDaysDefault = 30

// ─── Status enum (ADR-037 Canonical DSL §3) ───────────────────────────────────
//
// "Missing field" handling rule (PRD-011 §4.1 + TC-AI-019):
// A field that wasn't collected MUST be modeled with a non-Confirmed Status,
// NOT a zero value. Zero-value-as-no-data is forbidden ("0 冒充已确认无").

// Status is the field's confirmation status per ADR-037 Canonical DSL.
type Status string

const (
	// StatusConfirmed — value is real and reliable (came from primary source).
	StatusConfirmed Status = "confirmed"
	// StatusOptional — collector skipped this optional field.
	StatusOptional Status = "optional"
	// StatusMissing — field exists in contract but not in source (e.g. no annotation set).
	StatusMissing Status = "missing"
	// StatusUnableToConfirm — collector failed (API timeout, parse error, permission denied).
	StatusUnableToConfirm Status = "unable_to_confirm"
	// StatusSourceConflict — multiple sources disagreed.
	StatusSourceConflict Status = "source_conflict"
)

// ─── Field[T] generic wrapper (Go 1.21+ generics, go.mod 1.25.0 confirmed) ────

// Field wraps a value with its Status. Use Field[T] in AppContext instead of
// bare T to force callers to declare collection status (防"0 冒充已确认无").
//
// Convention:
//   - Status == StatusConfirmed → Value is trusted; evaluator uses it.
//   - Status != StatusConfirmed → evaluator treats Value as not collected
//     and routes to the "unable_to_confirm" tier of the rule (typically lowest score
//     OR a flagged 0 in Breakdown so UI can show "data missing, please configure").
type Field[T any] struct {
	Value  T      `json:"value"`
	Status Status `json:"status"`
}

// IsConfirmed returns true iff Status == StatusConfirmed.
// Helper used everywhere in v1_0_0.go to gate on collection status.
func (f Field[T]) IsConfirmed() bool { return f.Status == StatusConfirmed }

// ─── Enums: SecurityLevel, WORMMode, NetworkIsolationMode, DrillTier ─────────

// SecurityLevel is the 4-tier classification (ADR-043 §2 formula 3).
// Boundary rule (TC-AI-004): 90/75/60 inclusive on the upper tier (含).
type SecurityLevel string

const (
	LevelHighResilience   SecurityLevel = "high_resilience"    // 90-100
	LevelCompliantLowRisk SecurityLevel = "compliant_low_risk" // 75-89
	LevelFragile          SecurityLevel = "fragile"            // 60-74
	LevelCritical         SecurityLevel = "critical"           // <60
)

// WORMMode is the Object Lock retention mode (PRD-011 §4.2 dim3 + TC-AI-018).
type WORMMode string

const (
	// WORMCompliance → satisfies hard rule (公式 2), gives 10 IFF Enabled && RetainUntilDate > SnapshotAt+30d.
	WORMCompliance WORMMode = "COMPLIANCE"
	// WORMGovernance → privileged accounts can override; partial credit 6.
	WORMGovernance WORMMode = "GOVERNANCE"
	// WORMNone → no Object Lock at all; 0.
	WORMNone WORMMode = ""
)

// NetworkIsolationMode is dim2 subitem 3 (空气隔离).
type NetworkIsolationMode string

const (
	IsolationAirGap       NetworkIsolationMode = "air_gap"        // 真 Air-Gap 或备完即断 → 15
	IsolationSameADDomain NetworkIsolationMode = "same_ad_domain" // 同网络域/同 AD → 8
	IsolationNone         NetworkIsolationMode = "none"           // 无隔离生产可直挂 → 0
)

// DrillTier is dim4 subitem 2 (恢复演练通过率, 0 of 3-2-1-1-0).
type DrillTier string

const (
	DrillFullAuto3mo100pct DrillTier = "full_auto_3mo_100pct" // → 15
	DrillPeriodicManual    DrillTier = "periodic_manual"      // → 10
	DrillPartialFile       DrillTier = "partial_file"         // → 5
	DrillNone              DrillTier = "none"                 // → 0
)

// Tier is the application business importance class (Mars frame shift: customer-labeled
// via Application annotation `supkube.io/business-tier`).
type Tier string

const (
	Tier1 Tier = "tier1" // core / critical
	Tier2 Tier = "tier2"
	Tier3 Tier = "tier3"
)

// ─── AppContext: scoring input (minimal subset; full M1-M5 collector is a later unit) ─

// AppContext is the deterministic input to Score(). All time-based logic must read
// SnapshotAt instead of time.Now() to preserve determinism (TC-AI-003 铁律).
//
// Field grouping mirrors the 4 scoring dimensions (PRD-011 §4.2) to make the
// data → score mapping obvious.
type AppContext struct {
	// Namespace this score applies to (per-app scoring).
	Namespace string `json:"namespace"`

	// SnapshotAt is the time-of-collection stamp. ALL time-based rules (sliding window,
	// WORM RetainUntilDate margin, freshness) compare against this, NEVER time.Now().
	SnapshotAt time.Time `json:"snapshotAt"`

	// Coverage — dimension 1 input (Backup Coverage & Compliance, 25 max).
	Coverage CoverageInput `json:"coverage"`

	// Resilience — dimension 2 input (3-2-1-1-0 redundancy, 35 max).
	Resilience ResilienceInput `json:"resilience"`

	// Security — dimension 3 input (Immutability & Security, 20 max).
	Security SecurityInput `json:"security"`

	// Reliability — dimension 4 input (Success rate & verification, 20 max).
	Reliability ReliabilityInput `json:"reliability"`
}

// CoverageInput — dim1 inputs.
type CoverageInput struct {
	// Tier — application's business importance (annotation-driven, optional).
	Tier Field[Tier] `json:"tier"`
	// HasDifferentiatedPolicies — true if Tier 1/2/3 have differentiated backup policies.
	HasDifferentiatedPolicies Field[bool] `json:"hasDifferentiatedPolicies"`
	// CoreCoveragePct — % of core (Tier1) assets with a backup policy (0-100).
	CoreCoveragePct Field[float64] `json:"coreCoveragePct"`
	// ZombieAssetPct — % of unmanaged/zombie assets (0-100).
	ZombieAssetPct Field[float64] `json:"zombieAssetPct"`
	// RPOActualSeconds — measured RPO from BackupHistory (LastBackupAt → SnapshotAt).
	RPOActualSeconds Field[int] `json:"rpoActualSeconds"`
	// RPOTargetSeconds — business RPO target (annotation `supkube.io/business-rpo`).
	RPOTargetSeconds Field[int] `json:"rpoTargetSeconds"`
	// BackupIncludesMetadata — whether backups include K8s manifests / topology / config.
	BackupIncludesMetadata Field[bool] `json:"backupIncludesMetadata"`
}

// ResilienceInput — dim2 inputs.
type ResilienceInput struct {
	// StorageMediaTypes — distinct media types across all BSLs (e.g. ["s3","minio","azureblob"]).
	StorageMediaTypes Field[[]string] `json:"storageMediaTypes"`
	// BSLRegions — distinct regions across all BSLs.
	BSLRegions Field[[]string] `json:"bslRegions"`
	// BSLProviders — distinct cloud providers (e.g. ["aws","azure"]).
	BSLProviders Field[[]string] `json:"bslProviders"`
	// NetworkIsolation — air-gap level of backup network.
	NetworkIsolation Field[NetworkIsolationMode] `json:"networkIsolation"`
}

// SecurityInput — dim3 inputs.
type SecurityInput struct {
	WORM          WORMInput          `json:"worm"`
	Encryption    EncryptionInput    `json:"encryption"`
	AccessControl AccessControlInput `json:"accessControl"`
}

// WORMInput — dim3 subitem 1.
type WORMInput struct {
	ObjectLockEnabled  Field[bool]      `json:"objectLockEnabled"`
	Mode               Field[WORMMode]  `json:"mode"`
	RetainUntilDate    Field[time.Time] `json:"retainUntilDate"`
	BusinessSafetyDays int              `json:"businessSafetyDays"` // 0 → use BusinessSafetyDaysDefault
}

// EncryptionInput — dim3 subitem 2.
type EncryptionInput struct {
	StaticAES256           Field[bool] `json:"staticAes256"`
	TransitTLS13           Field[bool] `json:"transitTls13"`
	KMSIndependentRotation Field[bool] `json:"kmsIndependentRotation"`
}

// AccessControlInput — dim3 subitem 3.
type AccessControlInput struct {
	MFAEnabled              Field[bool] `json:"mfaEnabled"`
	RBACEnabled             Field[bool] `json:"rbacEnabled"`
	DeleteRequiresApproval  Field[bool] `json:"deleteRequiresApproval"`
	AuditOffsiteTamperProof Field[bool] `json:"auditOffsiteTamperProof"`
}

// ReliabilityInput — dim4 inputs.
type ReliabilityInput struct {
	// Last14DaysAttempts — total backup attempts in the rolling window (sliding window denominator).
	// MUST be ≥ 0; 0 → unable_to_confirm (NO division by zero, Mars Gate 1 rule ①).
	Last14DaysAttempts Field[int] `json:"last14DaysAttempts"`
	// Last14DaysSuccess — successful attempts in the rolling window (numerator).
	Last14DaysSuccess Field[int] `json:"last14DaysSuccess"`
	// ConsecutiveFailures — current consecutive-failure streak (for Tier1 penalty).
	ConsecutiveFailures Field[int] `json:"consecutiveFailures"`
	// LastBackupAt — nil → never backed up (triggers hard threshold 1: cap 30).
	LastBackupAt Field[*time.Time] `json:"lastBackupAt"`
	// LastBackupSucceeded — false → triggers hard threshold 2 if total ≥ 90.
	LastBackupSucceeded Field[bool] `json:"lastBackupSucceeded"`
	// DrillStatus — recovery drill maturity tier.
	DrillStatus Field[DrillTier] `json:"drillStatus"`
}

// ─── ScoreResult: scoring output ─────────────────────────────────────────────

// ScoreResult is the deterministic output. Same AppContext → exact same ScoreResult
// (including SubitemScore order, basis strings, calibrationApplied list).
//
// Status semantics (Mars 2026-06-03 拍板):
//   - Status == EvalStatusScored → 4-dim flow completed; TotalScore/Level/Dimensions all meaningful.
//   - Status != EvalStatusScored → not_eligible path; TotalScore=0, Level="" (empty), Dimensions zero;
//     UI should display the NotEligibleReason prompt, NOT a score number.
type ScoreResult struct {
	// Status — scored vs not_eligible (Mars 2026-06-03 §4.2.1).
	// Default "scored" for backward compat; not_eligible_* set when pre-condition fails.
	Status EvaluationStatus `json:"status"`

	// NotEligibleReason — human-readable reason for not_eligible status.
	// Empty when Status == scored.
	NotEligibleReason string `json:"notEligibleReason,omitempty"`

	// ScoreRulesVersion is always "v1.0.0" for this package. Bump on rules change.
	// Always populated regardless of Status (so callers know which version of pre-condition logic ran).
	ScoreRulesVersion string `json:"scoreRulesVersion"`

	// TotalScore — final score 0-100. **0 if Status != scored** (no meaningful score computed).
	// Otherwise may be lowered to 30 by high_score_calibration_30.
	TotalScore int `json:"totalScore"`

	// Level — 4-tier classification (only meaningful if Status == scored).
	Level SecurityLevel `json:"level"`

	// Dimensions — 4-dimension breakdown (only populated if Status == scored).
	Dimensions Dimensions `json:"dimensions"`

	// CalibrationApplied — list of hard thresholds that fired (empty if normal scoring).
	// Mars 2026-06-03 修订: removed "no_backup_cap_30" (now handled via not_eligible_no_runs).
	// Possible value: "high_score_calibration_30".
	CalibrationApplied []string `json:"calibrationApplied,omitempty"`

	// EvaluatedAt — = AppContext.SnapshotAt (NOT time.Now(); determinism guarantee).
	EvaluatedAt time.Time `json:"evaluatedAt"`
}

// Dimensions groups the 4 dimension scores. Order is fixed: BackupCoverage, Resilience,
// ImmutabilityAndSecurity, Reliability (matches PRD-011 §4.2 dimension numbering).
type Dimensions struct {
	BackupCoverage          DimensionScore `json:"backupCoverage"`          // dim1, max 25
	Resilience              DimensionScore `json:"resilience"`              // dim2, max 35
	ImmutabilityAndSecurity DimensionScore `json:"immutabilityAndSecurity"` // dim3, max 20
	Reliability             DimensionScore `json:"reliability"`             // dim4, max 20
}

// DimensionScore is a single dimension's subtotal + subitems.
type DimensionScore struct {
	Score    int            `json:"score"`    // sum of subitem scores
	MaxScore int            `json:"maxScore"` // dim weight (25/35/20/20)
	Subitems []SubitemScore `json:"subitems"`
}

// SubitemScore is one row in the matrix.
// ID format = "<dim>.<subitem>" (e.g. "coverage.tier_classification") for stable test addressing.
// Tier captures which matrix tier the input landed in ("max"/"partial"/"low"/"zero"/"unable_to_confirm").
type SubitemScore struct {
	ID       string `json:"id"`       // e.g. "coverage.tier_classification"
	Score    int    `json:"score"`    // actual score
	MaxScore int    `json:"maxScore"` // subitem max (e.g. 10/15/5)
	Tier     string `json:"tier"`     // matrix tier hit
	Basis    string `json:"basis"`    // one-line judgment basis (deterministic, no time-based)
}
