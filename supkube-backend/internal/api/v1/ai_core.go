package v1

// ai_core.go — M3 Copilot LLM 接入（移植自 SupInsight internal/api/ai.go）。
//
// Provider（aiProvider() 自动选）:
//   - "azure": Azure OpenAI chat-completions（HTTP API，pod 内可用）— 首选。
//   - "claude": shell 出本地 `claude` CLI（订阅）— 仅本地 dev。
//
// 所有密钥走 env（k8s Secret 注入），永不进源码。
// LLM 只出现在此 + ai_chat.go；评分路径（dr_score.go / evaluator）**永不调 LLM**
// （PRD-011 finding #2 铁律：规则引擎评分，LLM 只解释/对话）。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// aiAllow gates the Copilot endpoints (default off). Set SUPKUBE_ALLOW_AI=1.
var aiAllow = os.Getenv("SUPKUBE_ALLOW_AI") == "1"

func aiEnabled() bool { return aiAllow }

func aiProvider() string {
	if p := os.Getenv("AI_PROVIDER"); p != "" {
		return p
	}
	if os.Getenv("AZURE_OPENAI_ENDPOINT") != "" && os.Getenv("AZURE_OPENAI_KEY") != "" {
		return "azure"
	}
	return "claude"
}

// aiSystemPrompt — SupKube DR/备份域系统提示（取代 SupInsight 的 DRPM 文案）。
const aiSystemPrompt = "You are SupKube Copilot, the AI assistant inside SupKube — a " +
	"Kubernetes backup & disaster-recovery platform (Velero-based). You help users understand " +
	"and improve their DR posture: per-application backup health scores, backup coverage & RPO, " +
	"immutability (WORM object-lock), 3-2-1 storage redundancy, recovery drills, and actionable " +
	"backup recommendations. The context may include DR health scores (per-namespace " +
	"totalScore/level/status/recommendations from /ai/scores) and cluster/backup state. " +
	"Answer concisely using ONLY the provided context; if it lacks the answer, say so briefly. " +
	"Plain text, no markdown headers. 中文提问就用中文回答。 " +
	"When the user explicitly asks to back up / snapshot / protect a specific namespace, " +
	"call the create_backup tool (do not just describe how) — the user will confirm before it runs."

// callAzureOpenAI hits the Azure OpenAI chat-completions API (LHF cloud path).
func callAzureOpenAI(ctx context.Context, system, user string) (string, error) {
	endpoint := strings.TrimRight(os.Getenv("AZURE_OPENAI_ENDPOINT"), "/")
	deployment := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	apiVer := os.Getenv("AZURE_OPENAI_API_VERSION")
	if apiVer == "" {
		apiVer = "2024-06-01"
	}
	key := os.Getenv("AZURE_OPENAI_KEY")
	if endpoint == "" || deployment == "" || key == "" {
		return "", fmt.Errorf("azure openai not configured (need AZURE_OPENAI_ENDPOINT/DEPLOYMENT/KEY)")
	}
	url := endpoint + "/openai/deployments/" + deployment + "/chat/completions?api-version=" + apiVer
	body, _ := json.Marshal(map[string]any{
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		// GPT-5 family: max_tokens rejected (use max_completion_tokens); non-default
		// temperature rejected by reasoning models — omit it. Generous budget covers
		// reasoning + output.
		"max_completion_tokens": 4000,
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("azure openai status %d: %s", resp.StatusCode, aiTrunc(string(data), 200))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("parse azure response: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("azure openai: no choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// callClaudeCLI shells out to the local `claude` CLI (subscription). Local dev only.
// Strips ANTHROPIC_* env so it uses the interactive subscription login, not an API key.
func callClaudeCLI(ctx context.Context, full, model string) (string, error) {
	if model != "haiku" && model != "sonnet" && model != "opus" {
		model = "haiku"
	}
	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", model)
	cmd.Stdin = strings.NewReader(full)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "ANTHROPIC_API_KEY=") ||
			strings.HasPrefix(e, "ANTHROPIC_AUTH_TOKEN=") ||
			strings.HasPrefix(e, "ANTHROPIC_BASE_URL=") {
			continue
		}
		cmd.Env = append(cmd.Env, e)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func aiTrunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
