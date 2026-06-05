package v1

// ai_score.go — PRD-011 §4.6 H5 闭环, /ai/score endpoint.
//
// 把 evaluator (轨道 A 纯函数评分器) 通过 HTTP API 暴露给前端 / SPA / CLI:
//
//	POST /api/v1/ai/score
//	  Request:  evaluator.AppContext (JSON, schema 见 internal/advisor/evaluator/types.go)
//	  Response: evaluator.ScoreResult (JSON)
//	  Time:     < 5ms (规则引擎纯函数, 不调 LLM)
//
// 设计取舍 (Mars 拍板 D-WAIT-002 + 本任务 Gate 0):
//
//  1. DTO = evaluator.AppContext 直接作为 request body
//     理由: 零转换 / 零漂移 / JSON tag 已在 evaluator/types.go 就绪 (单一来源).
//     若加 Wrapper 一层, 评分契约改动需双地点同步 — 违反 Rule C 共享契约锁.
//
//  2. 错误码只有一个: ERR_AI_SCORE_INVALID_BODY (400) for JSON decode failure
//     理由: evaluator.Score() 是纯函数, 无 panic / 无 I/O / 无网络;
//     缺字段不报错, 走 unable_to_confirm 子项 tier (TC-AI-019);
//     LastBackupAt 缺失走 EvalStatusNotEligibleNoRuns (200 + Status: not_eligible_*).
//     所以正常路径 200, 异常路径只可能是请求 body 解析失败.
//
//  3. LLM 永不在此 code path (PRD-011 finding #2 铁律).
//     LLM 走 /ai/explain SSE, 见 ai_explain.go.
//
// 错误码体系: 参考 ADR-035 (ERR_<DOMAIN>_<REASON> prefix).
//
// 取号: TC-AI-020 (happy path) + TC-AI-021 (not_eligible + bad body), Mars 预分配, 见 ai_score_test.go.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/supkube/supkube-backend/internal/advisor/evaluator"
)

// errCodeInvalidBody is the only error code emitted by this handler.
// JSON parse failure or empty body → 400 + this code.
const errCodeInvalidBody = "ERR_AI_SCORE_INVALID_BODY"

// ScoreHandler implements POST /api/v1/ai/score.
//
// Reads evaluator.AppContext from request body, calls evaluator.Score(ctx),
// returns ScoreResult as JSON. Deterministic — same input always produces
// the same output (TC-AI-003 contract, verified by upstream evaluator tests).
//
// HTTP status semantics:
//   - 200 + ScoreResult{Status: scored, ...}           — normal scoring complete
//   - 200 + ScoreResult{Status: not_eligible_no_runs}  — Policy never executed; not_eligible path
//   - 200 + ScoreResult{Status: not_eligible_no_policy} — no Policy bound
//   - 400 + {error, code: ERR_AI_SCORE_INVALID_BODY}   — request body invalid / unparseable
func ScoreHandler(c *gin.Context) {
	// Drain the body up front so we can distinguish empty body (caller forgot
	// to send anything) from parse error (caller sent malformed JSON). Both
	// fall under ERR_AI_SCORE_INVALID_BODY but the message differs.
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "failed to read request body: " + err.Error(),
			"code":  errCodeInvalidBody,
		})
		return
	}
	if len(raw) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "request body is empty; expected JSON-encoded evaluator.AppContext",
			"code":  errCodeInvalidBody,
		})
		return
	}

	var ctx evaluator.AppContext
	// Use gin's BindJSON-equivalent via the standard decoder so we can
	// re-feed the buffered bytes. gin.Context.ShouldBindJSON consumes
	// c.Request.Body which we've already drained.
	if err := bindJSONBytes(raw, &ctx); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid JSON for evaluator.AppContext: " + err.Error(),
			"code":  errCodeInvalidBody,
		})
		return
	}

	// evaluator.Score is a pure function — no I/O, no error return. The
	// "score might fail" failure modes are all expressed via ScoreResult.Status
	// (not_eligible_*) or via per-subitem unable_to_confirm tiers.
	result := evaluator.Score(ctx)

	c.JSON(http.StatusOK, result)
}

// bindJSONBytes decodes a JSON byte slice into v. Extracted to a helper
// (instead of using gin's c.ShouldBindJSON) because we've already read the
// body via io.ReadAll above to discriminate empty-body from parse-error.
func bindJSONBytes(raw []byte, v interface{}) error {
	if len(raw) == 0 {
		return errors.New("empty body")
	}
	return json.Unmarshal(raw, v)
}
