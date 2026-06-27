package v1

// ai_chat.go — M3 Copilot 对话端点。POST /api/v1/ai/chat（移植自 SupInsight handleAI）。
//
// 上下文感知：前端把"当前页 + DR 评分（/ai/scores 结果）"塞进 context 字段，
// 后端拼进 prompt 让 LLM 据此回答（"为什么 kb-cloud 危险？怎么修？"）。
// KB4AI RAG 暂未接（M3.1 再加 internal/kb）。LLM 调用见 ai_core.go。
//
// 鉴权：走 /api/v1 组的鉴权（需登录）+ feature gate SUPKUBE_ALLOW_AI=1。

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type aiChatRequest struct {
	Prompt  string `json:"prompt"`
	Context string `json:"context"`
	Model   string `json:"model"`
	System  string `json:"system"`
	UseKB   bool   `json:"useKB"`   // 预留：M3.1 KB RAG
	KBQuery string `json:"kbQuery"` // 预留
	KBTopK  int    `json:"kbTopK"`  // 预留
}

type aiChatResponse struct {
	Answer   string `json:"answer"`
	Provider string `json:"provider"`
	KBUsed   bool   `json:"kbUsed"`
}

// ChatHandler implements POST /api/v1/ai/chat.
func ChatHandler(c *gin.Context) {
	if !aiEnabled() {
		c.JSON(http.StatusForbidden, gin.H{"error": "AI disabled (set SUPKUBE_ALLOW_AI=1)"})
		return
	}
	var in aiChatRequest
	if err := c.ShouldBindJSON(&in); err != nil || strings.TrimSpace(in.Prompt) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty or invalid prompt"})
		return
	}

	sys := in.System
	if sys == "" {
		sys = aiSystemPrompt
	}
	ctxv := in.Context
	if ctxv == "" {
		ctxv = "(none)"
	}
	// KB4AI RAG 暂未接（M3.1）；UseKB/KBQuery/KBTopK 字段预留，当前忽略。
	kbUsed := false
	if len(ctxv) > 9000 {
		ctxv = ctxv[:9000] + "…(truncated)"
	}
	userMsg := "--- CONTEXT ---\n" + ctxv + "\n\n--- USER ---\n" + in.Prompt

	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()

	provider := aiProvider()
	var answer string
	var err error
	switch provider {
	case "azure":
		answer, err = callAzureOpenAI(ctx, sys, userMsg)
	default:
		answer, err = callClaudeCLI(ctx, sys+"\n\n"+userMsg, in.Model)
	}
	if err != nil {
		msg := "AI call failed"
		if ctx.Err() == context.DeadlineExceeded {
			msg = "AI call timed out"
		}
		log.Printf("[ai] provider=%s %s: %v", provider, msg, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, aiChatResponse{Answer: answer, Provider: provider, KBUsed: kbUsed})
}
