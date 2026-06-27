package v1

// ai_routes.go — PRD-011 §4.6 AI Backup Advisor 路由注册.
//
// 单独成文件而不是直接写到 cmd/server/server.go, 跟 internal/importpolicy
// 的 RegisterRoutes 模式一致 (handler 跟它的路由声明放在一起, 服务器
// 启动文件只引一行函数调用).
//
// 当前注册了 2 个路由 (PRD-011 §4.6 表中前 2 行):
//
//	POST /api/v1/ai/score                — ScoreHandler   (ai_score.go)
//	GET  /api/v1/ai/explain/:taskId      — ExplainStubHandler (ai_explain.go)
//
// 剩余 4 个 endpoint (/ai/result, /ai/scores, /ai/audit, /ai/settings) 是
// PRD-011 §4.6 表中后续单件; 此函数那时再加, 不在本 task 范围.
//
// RBAC: 这两个端点目前不显式声明 permission — viewer 即可读 (score 是纯计算).
// PRD-011 §4.6 不要求 admin/editor gate, 跟 /backup-advisor (现役) 同级.

import "github.com/gin-gonic/gin"

// RegisterAIRoutes registers PRD-011 §4.6 AI Backup Advisor endpoints on
// the given gin RouterGroup (typically the /api/v1 group from server.go).
//
// Caller:  v1.RegisterAIRoutes(api)  // where api = r.Group("/api/v1")
func RegisterAIRoutes(r *gin.RouterGroup) {
	ai := r.Group("/ai")
	{
		// PRD-011 §4.6 H5: /ai/score — deterministic rule-engine scoring,
		// returns ScoreResult in <5ms. Pure function (no I/O), so no per-
		// request rate limit needed.
		ai.POST("/score", ScoreHandler)

		// PRD-011 §4.6 H5: /ai/explain/:taskId — SSE stub. LLM streaming
		// explainer ships in a later PRD-011 unit; current handler writes
		// one stub frame so SPA can wire EventSource code today.
		ai.GET("/explain/:taskId", ExplainStubHandler)

		// PRD-011 §4.6: /ai/scores — collector-driven cluster-wide DR scoring
		// (Task #115 Phase A, dr_score.go). Walks every app namespace, builds
		// evaluator.AppContext from real K8s + Velero state, scores it via the
		// deterministic rule engine, and attaches actionable backup advice.
		// Read-only (viewer can read; same level as /score).
		ai.GET("/scores", ListDRScores)

		// M3: /ai/chat — Copilot 对话（移植自 SupInsight）。上下文感知（前端把
		// 当前页 + DR 评分塞进 context）+ Azure OpenAI/claude provider。LLM 仅在
		// 此 + ai_explain 出现；评分路径永不调 LLM（PRD-011 铁律）。
		// Feature gate: SUPKUBE_ALLOW_AI=1（见 ai_core.go）。
		ai.POST("/chat", ChatHandler)
	}
}
