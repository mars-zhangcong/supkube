// Package supkubeclient calls supkube-backend's REST API. MCP skills depend on
// the Client INTERFACE (not the HTTP impl) so they unit-test without a live
// backend. This is the "thin adapter" boundary: the MCP server is a client of
// supkube-backend, a separate process (PRD-004 §4.1 / §4.4 option a).
package supkubeclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Workload is what list_k8s_workloads returns.
//
// HONEST GAP (flagged, not faked): PRD-004 spec says list_k8s_workloads returns
// per-workload name+kind+status+replicas. The current backend only exposes
// per-namespace workload COUNTS (GET /api/v1/applications → ApplicationInfo
// {namespace, workloads:int}). Full per-workload enumeration needs a dedicated
// backend endpoint (Phase-2 backend work). For the Phase-1 PoC this skill
// returns the namespace+count the backend actually provides, and the gap is
// recorded rather than a workload shape invented.
type Workload struct {
	Namespace string `json:"namespace"`
	Workloads int    `json:"workloads"`
}

type Client interface {
	ListWorkloads(ctx context.Context, cluster, namespace string) ([]Workload, error)
	// GetRestoreStatus = the authoritative status fallback SupInsight asked for
	// (#3): reliable wait = SSE fast-path + this poll. Returns a trimmed summary.
	GetRestoreStatus(ctx context.Context, name string) (map[string]any, error)
	// Write ops (#1) — executed only after HitL confirm (ADR-057).
	TriggerRestore(ctx context.Context, args map[string]any) (map[string]any, error)
	TriggerBackup(ctx context.Context, args map[string]any) (map[string]any, error)
}

// HTTP is the real client; bearer = the server→backend service token.
type HTTP struct {
	BaseURL string
	Token   string
	HC      *http.Client
}

func NewHTTP(baseURL, token string) *HTTP {
	return &HTTP{BaseURL: baseURL, Token: token, HC: &http.Client{Timeout: 10 * time.Second}}
}

func (h *HTTP) ListWorkloads(ctx context.Context, cluster, namespace string) ([]Workload, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, h.BaseURL+"/api/v1/applications", nil)
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}
	if cluster != "" {
		req.Header.Set("X-Supkube-Cluster", cluster) // backend's per-request cluster routing
	}
	resp, err := h.HC.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend %d: %s", resp.StatusCode, string(b))
	}
	// Decode tolerantly (don't assume exact field names beyond what's verified).
	var raw struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	out := make([]Workload, 0, len(raw.Items))
	for _, it := range raw.Items {
		ns, _ := it["namespace"].(string)
		if ns == "" {
			ns, _ = it["name"].(string)
		}
		if namespace != "" && ns != namespace {
			continue
		}
		wl := 0
		if f, ok := it["workloads"].(float64); ok {
			wl = int(f)
		}
		out = append(out, Workload{Namespace: ns, Workloads: wl})
	}
	return out, nil
}

// GetRestoreStatus calls GET /api/v1/restores/{name} and returns a trimmed
// summary (≤4KB target per PRD-004 §4.3): name/phase/error info, not raw logs.
func (h *HTTP) GetRestoreStatus(ctx context.Context, name string) (map[string]any, error) {
	var raw map[string]any
	if err := h.getJSON(ctx, "/api/v1/restores/"+name, "", &raw); err != nil {
		return nil, err
	}
	status, _ := raw["status"].(map[string]any)
	phase := ""
	if status != nil {
		phase, _ = status["phase"].(string)
	}
	return map[string]any{"name": name, "phase": phase, "status": status}, nil
}

// TriggerRestore POSTs /api/v1/restores (CreateRestore). Called only post-confirm.
func (h *HTTP) TriggerRestore(ctx context.Context, args map[string]any) (map[string]any, error) {
	return h.postJSON(ctx, "/api/v1/restores", args)
}

// TriggerBackup POSTs /api/v1/backups (CreateBackup). Called only post-confirm.
func (h *HTTP) TriggerBackup(ctx context.Context, args map[string]any) (map[string]any, error) {
	return h.postJSON(ctx, "/api/v1/backups", args)
}

func (h *HTTP) getJSON(ctx context.Context, path, cluster string, out any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, h.BaseURL+path, nil)
	h.authHeaders(req, cluster)
	resp, err := h.HC.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend %d: %s", resp.StatusCode, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (h *HTTP) postJSON(ctx context.Context, path string, body map[string]any) (map[string]any, error) {
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, h.BaseURL+path, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	h.authHeaders(req, "")
	resp, err := h.HC.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("backend %d: %s", resp.StatusCode, string(rb))
	}
	var out map[string]any
	_ = json.Unmarshal(rb, &out)
	return out, nil
}

func (h *HTTP) authHeaders(req *http.Request, cluster string) {
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}
	if cluster != "" {
		req.Header.Set("X-Supkube-Cluster", cluster)
	}
}
