package v1

// ai_score_test.go — TC-AI-020 + TC-AI-021 (PRD-011 §4.6 evaluator endpoint).
//
// Test numbers (Mars 预分配, 不去 LEDGER 取号):
//   - TC-AI-020 — POST /ai/score happy path (perfect ctx → TotalScore 100)
//   - TC-AI-021 — POST /ai/score not_eligible 路径 (LastBackupAt 缺 → not_eligible_no_runs)
//                 + bad body (空 body / 非法 JSON → 400 + ERR_AI_SCORE_INVALID_BODY)
//
// Determinism contract reuse: evaluator package owns the determinism test
// (TC-AI-003 over 100 iterations); this file only verifies the HTTP shim.
// We do NOT re-test the score correctness — just the HTTP wiring (status
// codes, response shape, content-type).

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/supkube/supkube-backend/internal/advisor/evaluator"
)

// ─── fixture builders (replicates evaluator/v1_0_0_test.go perfectCtx; that
//     helper is unexported so we re-build here per Rule C — single-import,
//     no copy of the score logic, only the input fixture) ───────────────────

func confirmed[T any](v T) evaluator.Field[T] {
	return evaluator.Field[T]{Value: v, Status: evaluator.StatusConfirmed}
}

func missing[T any]() evaluator.Field[T] {
	var zero T
	return evaluator.Field[T]{Value: zero, Status: evaluator.StatusMissing}
}

func timePtr(t time.Time) *time.Time { return &t }

// snapshotAt — fixed time used across HTTP shim tests (determinism rule).
var apiTestSnapshotAt = time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)

// buildPerfectCtx returns an AppContext fixture that scores exactly 100.
// Mirrors evaluator/v1_0_0_test.go.perfectCtx() but lives here because that
// helper is package-private. NEVER duplicate scoring logic — only fixture.
func buildPerfectCtx() evaluator.AppContext {
	retainDate := apiTestSnapshotAt.AddDate(0, 0, 90)
	lastBackup := apiTestSnapshotAt.Add(-1 * time.Hour)
	return evaluator.AppContext{
		Namespace:  "prod-mysql",
		SnapshotAt: apiTestSnapshotAt,
		Coverage: evaluator.CoverageInput{
			Tier:                      confirmed(evaluator.Tier1),
			HasDifferentiatedPolicies: confirmed(true),
			CoreCoveragePct:           confirmed(100.0),
			ZombieAssetPct:            confirmed(0.0),
			RPOActualSeconds:          confirmed(1800),
			RPOTargetSeconds:          confirmed(3600),
			BackupIncludesMetadata:    confirmed(true),
		},
		Resilience: evaluator.ResilienceInput{
			StorageMediaTypes: confirmed([]string{"s3", "minio"}),
			BSLRegions:        confirmed([]string{"us-east-1", "eu-west-1"}),
			BSLProviders:      confirmed([]string{"aws", "azure"}),
			NetworkIsolation:  confirmed(evaluator.IsolationAirGap),
		},
		Security: evaluator.SecurityInput{
			WORM: evaluator.WORMInput{
				ObjectLockEnabled:  confirmed(true),
				Mode:               confirmed(evaluator.WORMCompliance),
				RetainUntilDate:    confirmed(retainDate),
				BusinessSafetyDays: 30,
			},
			Encryption: evaluator.EncryptionInput{
				StaticAES256:           confirmed(true),
				TransitTLS13:           confirmed(true),
				KMSIndependentRotation: confirmed(true),
			},
			AccessControl: evaluator.AccessControlInput{
				MFAEnabled:              confirmed(true),
				RBACEnabled:             confirmed(true),
				DeleteRequiresApproval:  confirmed(true),
				AuditOffsiteTamperProof: confirmed(true),
			},
		},
		Reliability: evaluator.ReliabilityInput{
			Last14DaysAttempts:  confirmed(14),
			Last14DaysSuccess:   confirmed(14),
			ConsecutiveFailures: confirmed(0),
			LastBackupAt:        confirmed(timePtr(lastBackup)),
			LastBackupSucceeded: confirmed(true),
			DrillStatus:         confirmed(evaluator.DrillFullAuto3mo100pct),
		},
	}
}

// newAITestRouter mounts /api/v1/ai/* with the two handlers under test.
func newAITestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	RegisterAIRoutes(api)
	return r
}

// doScoreRequest fires a POST /api/v1/ai/score with the given body and
// returns the recorder.
func doScoreRequest(t *testing.T, r *gin.Engine, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/score", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ─── TC-AI-020 [P0] POST /ai/score happy path ────────────────────────────

// TC-AI-020: perfectCtx ⇒ HTTP 200 + ScoreResult{Status: scored, TotalScore: 100,
// Level: high_resilience, ScoreRulesVersion: v1.0.0}.
//
// This proves the HTTP shim end-to-end: JSON-encode AppContext → handler
// decodes → calls evaluator.Score → JSON-encodes ScoreResult → client decodes.
func TestTC_AI_020_ScoreHandler_HappyPath(t *testing.T) {
	r := newAITestRouter()

	body, err := json.Marshal(buildPerfectCtx())
	if err != nil {
		t.Fatalf("marshal perfect ctx: %v", err)
	}

	w := doScoreRequest(t, r, body)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json prefix", ct)
	}

	var got evaluator.ScoreResult
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v; body = %s", err, w.Body.String())
	}

	if got.Status != evaluator.EvalStatusScored {
		t.Errorf("Status = %q, want %q", got.Status, evaluator.EvalStatusScored)
	}
	if got.TotalScore != 100 {
		t.Errorf("TotalScore = %d, want 100", got.TotalScore)
	}
	if got.Level != evaluator.LevelHighResilience {
		t.Errorf("Level = %q, want %q", got.Level, evaluator.LevelHighResilience)
	}
	if got.ScoreRulesVersion != evaluator.ScoreRulesVersion {
		t.Errorf("ScoreRulesVersion = %q, want %q", got.ScoreRulesVersion, evaluator.ScoreRulesVersion)
	}
	if !got.EvaluatedAt.Equal(apiTestSnapshotAt) {
		t.Errorf("EvaluatedAt = %v, want = snapshotAt %v (determinism: must NOT be time.Now())",
			got.EvaluatedAt, apiTestSnapshotAt)
	}
}

// ─── TC-AI-021 [P0] not_eligible 路径 + bad body ─────────────────────────

// TC-AI-021a: AppContext with LastBackupAt status=missing → Score returns
// EvalStatusNotEligibleNoRuns; the HTTP shim must return 200 + that status
// (NOT 400 — not_eligible is a successful evaluation, just no score computed).
func TestTC_AI_021a_ScoreHandler_NotEligibleNoRuns(t *testing.T) {
	r := newAITestRouter()

	ctx := buildPerfectCtx()
	// Wipe LastBackupAt to trigger checkEligibility's not_eligible_no_runs path.
	ctx.Reliability.LastBackupAt = missing[*time.Time]()

	body, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	w := doScoreRequest(t, r, body)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (not_eligible is success, not error); body = %s",
			w.Code, w.Body.String())
	}

	var got evaluator.ScoreResult
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Status != evaluator.EvalStatusNotEligibleNoRuns {
		t.Errorf("Status = %q, want %q", got.Status, evaluator.EvalStatusNotEligibleNoRuns)
	}
	if got.TotalScore != 0 {
		t.Errorf("not_eligible: TotalScore = %d, want 0 (no meaningful score)", got.TotalScore)
	}
	if got.Level != "" {
		t.Errorf("not_eligible: Level = %q, want empty", got.Level)
	}
	if got.NotEligibleReason == "" {
		t.Errorf("not_eligible: NotEligibleReason empty, want non-empty prompt for UI")
	}
	if got.ScoreRulesVersion != evaluator.ScoreRulesVersion {
		t.Errorf("ScoreRulesVersion = %q, want %q", got.ScoreRulesVersion, evaluator.ScoreRulesVersion)
	}
}

// TC-AI-021b: empty body → 400 + ERR_AI_SCORE_INVALID_BODY.
func TestTC_AI_021b_ScoreHandler_EmptyBody(t *testing.T) {
	r := newAITestRouter()

	w := doScoreRequest(t, r, nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty body: status = %d, want 400", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error response: %v; body = %s", err, w.Body.String())
	}
	if resp["code"] != errCodeInvalidBody {
		t.Errorf("error code = %q, want %q", resp["code"], errCodeInvalidBody)
	}
	if resp["error"] == "" {
		t.Errorf("error message empty")
	}
}

// TC-AI-021c: malformed JSON → 400 + ERR_AI_SCORE_INVALID_BODY.
func TestTC_AI_021c_ScoreHandler_MalformedJSON(t *testing.T) {
	r := newAITestRouter()

	w := doScoreRequest(t, r, []byte("{not valid json"))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("malformed json: status = %d, want 400; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["code"] != errCodeInvalidBody {
		t.Errorf("error code = %q, want %q", resp["code"], errCodeInvalidBody)
	}
}

// TC-AI-021d (determinism via HTTP): twice marshal+POST same fixture → byte-
// equal response bodies. Backstops the determinism contract at the HTTP layer
// (evaluator/v1_0_0_test.go owns the 100× pure-function determinism test).
func TestTC_AI_021d_ScoreHandler_DeterminismAcrossRequests(t *testing.T) {
	r := newAITestRouter()
	body, err := json.Marshal(buildPerfectCtx())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	w1 := doScoreRequest(t, r, body)
	w2 := doScoreRequest(t, r, body)

	if w1.Code != http.StatusOK || w2.Code != http.StatusOK {
		t.Fatalf("status: w1=%d w2=%d", w1.Code, w2.Code)
	}
	if !bytes.Equal(w1.Body.Bytes(), w2.Body.Bytes()) {
		t.Errorf("determinism violated: two identical requests produced different bodies\n"+
			"w1 = %s\nw2 = %s", w1.Body.String(), w2.Body.String())
	}
}
