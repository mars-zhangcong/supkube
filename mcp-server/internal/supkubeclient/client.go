// Package supkubeclient calls supkube-backend's REST API. MCP skills depend on
// the Client INTERFACE (not the HTTP impl) so they unit-test without a live
// backend. This is the "thin adapter" boundary: the MCP server is a client of
// supkube-backend, a separate process (PRD-004 §4.1 / §4.4 option a).
package supkubeclient

import (
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
