package v1

// ai_explain_test.go — GET /api/v1/ai/explain/:taskId SSE 桩态测试.
//
// 当前 stub 行为 (LLM 接入前):
//   - 200 OK
//   - Content-Type: text/event-stream
//   - Cache-Control: no-cache
//   - X-Accel-Buffering: no
//   - body 含 "event: stub" 帧 + "event: done" 帧
//   - "data:" 行的 JSON 含 message + 回显的 taskId + scoreRulesVersion

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// doExplainRequest fires a GET /api/v1/ai/explain/:taskId.
func doExplainRequest(t *testing.T, taskID string) *httptest.ResponseRecorder {
	t.Helper()
	r := newAITestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/explain/"+taskID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TC-AI-021e (paired with /ai/score TC-AI-021): SSE stub 必须返回 200 +
// text/event-stream + 含 stub 帧, 这样前端 EventSource 客户端代码不会因
// 端点缺失而报 404/502, 也能正确解析降级提示.
func TestTC_AI_021e_ExplainStub_BasicShape(t *testing.T) {
	w := doExplainRequest(t, "task-abc-123")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream prefix", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
	if xa := w.Header().Get("X-Accel-Buffering"); xa != "no" {
		t.Errorf("X-Accel-Buffering = %q, want no (defeats nginx proxy_buffering)", xa)
	}

	body := w.Body.String()
	// SSE protocol shape — "event: stub" + "data: {...}" then blank line.
	if !strings.Contains(body, "event: stub\n") {
		t.Errorf("body missing 'event: stub' frame; got:\n%s", body)
	}
	if !strings.Contains(body, "event: done\n") {
		t.Errorf("body missing 'event: done' terminator; got:\n%s", body)
	}
	// data line must carry the stub message + echo taskId.
	if !strings.Contains(body, `"message":"LLM explainer 待 PRD-011 后续单件接入, 当前是桩"`) {
		t.Errorf("body missing stub message; got:\n%s", body)
	}
	if !strings.Contains(body, `"taskId":"task-abc-123"`) {
		t.Errorf("body missing echoed taskId; got:\n%s", body)
	}
	if !strings.Contains(body, `"scoreRulesVersion":"v1.0.0"`) {
		t.Errorf("body missing scoreRulesVersion; got:\n%s", body)
	}
}

// Defensive: taskId with chars that need JSON escaping should NOT break the
// stub frame (we hand-encode the JSON so we must escape correctly).
func TestTC_AI_021f_ExplainStub_TaskIDEscaping(t *testing.T) {
	// Try a taskID with chars that would break unescaped JSON: backslash + quote.
	w := doExplainRequest(t, `weird"id\`+`with`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	// Both frames must still be present.
	if !strings.Contains(body, "event: stub\n") || !strings.Contains(body, "event: done\n") {
		t.Errorf("escaping broke frame structure; body =\n%s", body)
	}
}
