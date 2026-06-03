package v1

// ai_explain.go — PRD-011 §4.6 H5 LLM 解释 SSE 端点的桩.
//
// 当前任务 (PRD-011 §4.6) 只接评分 endpoint, LLM 解释器是后续单件.
// 此文件提供 SSE 框架桩, 让前端 SPA 在 LLM 接入前可以:
//
//  1. 提前对接 EventSource / SSE 客户端代码 (不会因 endpoint 不存在 502/404).
//  2. 收到一个 "event: stub" 帧, 知道 LLM 未启用, 显示降级提示文案.
//
// 设计取舍 (Gate 0):
//
//   - 路径: GET /api/v1/ai/explain/:taskId (PRD-011 §4.6 表 + §12 H5).
//     taskId 由 /ai/score 响应里的 explainTaskId 字段交付; 桩态下 taskId 不
//     校验, 任何字符串都允许 (反正下面只回桩帧).
//
//   - 响应 headers (SSE 标准 3 件套):
//     Content-Type: text/event-stream  — 触发 EventSource 解析
//     Cache-Control: no-cache         — 防中间代理缓存
//     X-Accel-Buffering: no           — 关 nginx proxy buffering, 防"全收齐才转发"
//
//   - Payload: 一帧 event=stub + data={message:"..."} + 一帧 event=done, 然后关流.
//     done 帧让前端 EventSource onmessage 收到 close 信号 (per H5 §4.6 stream 协议).
//
//   - 200 OK 不报错 — 不是"接口故障"而是"功能桩态"; UI 层显示"LLM 解释功能待启用"
//     比 503 更友好 (503 会让前端 retry logic 反复打).
//
// LLM 真接入 (Ollama / chroma-go RAG / sanitize layer) 走 PRD-011 §4.6 后续单件;
// 那个单件会替换本文件内容 + 加 ai_explain_test.go LLM-specific TC.

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ExplainStubHandler implements GET /api/v1/ai/explain/:taskId (SSE stub).
//
// Current behavior: writes one "event: stub" + one "event: done" SSE frame
// and returns 200 OK. The real LLM streaming explainer ships in a later
// PRD-011 unit; this stub lets the SPA wire its EventSource code today.
//
// Response shape (per https://html.spec.whatwg.org/multipage/server-sent-events.html):
//
//	HTTP/1.1 200 OK
//	Content-Type: text/event-stream
//	Cache-Control: no-cache
//	X-Accel-Buffering: no
//
//	event: stub
//	data: {"message":"LLM explainer 待 PRD-011 后续单件接入, 当前是桩",
//	       "taskId":"<echo>","scoreRulesVersion":"v1.0.0"}
//
//	event: done
//	data: {"reason":"stub"}
func ExplainStubHandler(c *gin.Context) {
	taskID := c.Param("taskId")

	// SSE headers — must be set BEFORE any body write so gin doesn't auto-pick
	// application/json. Setting them via gin.Context.Header() guarantees they
	// go out in the response status line.
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	// X-Accel-Buffering: no — defeats nginx proxy_buffering so each Write hits
	// the wire immediately. Same trick we use in PRD-005 Log Viewer SSE.
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	// Write the stub frame. SSE protocol: each event = "event: <name>\ndata: <payload>\n\n".
	// Two trailing newlines mark the end of the event.
	// We carefully JSON-encode the data line via fmt.Fprintf with %q on the
	// message — but to avoid pulling json marshal for a fixed-shape literal,
	// hand-encode (the literal has no special chars).
	w := c.Writer

	fmt.Fprintf(w,
		"event: stub\n"+
			"data: {\"message\":\"LLM explainer 待 PRD-011 后续单件接入, 当前是桩\","+
			"\"taskId\":\"%s\","+
			"\"scoreRulesVersion\":\"v1.0.0\"}\n\n",
		escapeJSONString(taskID),
	)

	// Flush after the stub frame so the client EventSource fires its
	// onmessage handler immediately (rather than waiting for the connection
	// to close). gin.ResponseWriter wraps http.Flusher; safe to assert.
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// "done" event lets the client close gracefully — mirrors the real
	// streaming protocol's terminator (PRD-011 §4.6 stream sample).
	fmt.Fprint(w,
		"event: done\n"+
			"data: {\"reason\":\"stub\"}\n\n",
	)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// escapeJSONString does the minimal escaping needed to embed a value in a
// JSON-string-literal we're hand-building. Only \ and " need escaping
// inside a JSON string (we control the rest of the chars).
// Used only for taskId echo — taskId comes from URL path so its alphabet is
// already very restricted, but we be defensive.
func escapeJSONString(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\', '"':
			out = append(out, '\\', c)
		case '\n':
			out = append(out, '\\', 'n')
		case '\r':
			out = append(out, '\\', 'r')
		case '\t':
			out = append(out, '\\', 't')
		default:
			if c < 0x20 {
				// drop control chars; taskId shouldn't contain any
				continue
			}
			out = append(out, c)
		}
	}
	return string(out)
}
