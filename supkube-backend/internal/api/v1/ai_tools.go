package v1

// ai_tools.go — M4 Copilot 动手：Azure OpenAI function calling。
//
// 给 LLM 一组工具（M4 先做 create_backup）。LLM 在对话中决定调用 →
// 后端**不直接执行**，把 pendingAction 返回前端，由用户在确认卡片点"确认"
// 后才调真正的写 API（POST /applications/{ns}/snapshot）。这就是 HitL：
// LLM 提议、人确认、才动手。（后端写端点无 confirm-token 机制，HitL 落在
// 前端确认卡片 + 写端点自身的 RBAC editor 作用域。）

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// copilotTools — 暴露给 LLM 的工具（OpenAI function-calling schema）。
// M4 先一个：create_backup（最常用、最低风险的写操作）。后续可加
// create_schedule / trigger_restore。
var copilotTools = []map[string]any{
	{
		"type": "function",
		"function": map[string]any{
			"name":        "create_backup",
			"description": "Create an immediate one-time backup (CSI snapshot) for a Kubernetes application namespace. Call this when the user asks to back up / snapshot / protect a specific namespace.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"namespace": map[string]any{
						"type":        "string",
						"description": "The namespace to back up, e.g. kb-cloud",
					},
					"reason": map[string]any{
						"type":        "string",
						"description": "Short reason for the backup, e.g. 'data-bearing app never backed up'",
					},
				},
				"required": []string{"namespace"},
			},
		},
	},
}

type aiToolCall struct {
	Name string
	Args map[string]any
}

// callAzureOpenAITools is callAzureOpenAI + function-calling. Returns the text
// content AND any tool calls the model wants to make — NOT executed here (HitL:
// the caller turns a tool call into a pendingAction for the user to confirm).
func callAzureOpenAITools(ctx context.Context, system, user string, tools []map[string]any) (string, []aiToolCall, error) {
	endpoint := strings.TrimRight(os.Getenv("AZURE_OPENAI_ENDPOINT"), "/")
	deployment := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	apiVer := os.Getenv("AZURE_OPENAI_API_VERSION")
	if apiVer == "" {
		apiVer = "2024-06-01"
	}
	key := os.Getenv("AZURE_OPENAI_KEY")
	if endpoint == "" || deployment == "" || key == "" {
		return "", nil, fmt.Errorf("azure openai not configured")
	}
	url := endpoint + "/openai/deployments/" + deployment + "/chat/completions?api-version=" + apiVer
	payload := map[string]any{
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"max_completion_tokens": 4000,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
		payload["tool_choice"] = "auto"
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", nil, fmt.Errorf("azure openai status %d: %s", resp.StatusCode, aiTrunc(string(data), 200))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", nil, fmt.Errorf("parse azure response: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", nil, fmt.Errorf("azure openai: no choices")
	}
	msg := out.Choices[0].Message
	var calls []aiToolCall
	for _, tc := range msg.ToolCalls {
		args := map[string]any{}
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		calls = append(calls, aiToolCall{Name: tc.Function.Name, Args: args})
	}
	return strings.TrimSpace(msg.Content), calls, nil
}
