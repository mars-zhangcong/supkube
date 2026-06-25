package mcpproto

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/supkube/mcp-server/internal/auth"
	"github.com/supkube/mcp-server/internal/confirm"
	"github.com/supkube/mcp-server/internal/skills"
	"github.com/supkube/mcp-server/internal/supkubeclient"
)

type mockClient struct{}

func (mockClient) ListWorkloads(_ context.Context, _, _ string) ([]supkubeclient.Workload, error) {
	return []supkubeclient.Workload{{Namespace: "prod-orders", Workloads: 3}}, nil
}
func (mockClient) GetBackupAdvice(_ context.Context, ns string) (map[string]any, error) {
	return map[string]any{"namespace": ns, "tier": "gold"}, nil
}
func (mockClient) GetBackupStatus(_ context.Context, name string) (map[string]any, error) {
	return map[string]any{"name": name, "phase": "Completed"}, nil
}
func (mockClient) GetRestoreStatus(_ context.Context, name string) (map[string]any, error) {
	return map[string]any{"name": name, "phase": "Completed"}, nil
}
func (mockClient) CreateBackupPolicy(_ context.Context, args map[string]any) (map[string]any, error) {
	return map[string]any{"created": "policy", "name": args["name"]}, nil
}
func (mockClient) RunBackupPolicy(_ context.Context, name string) (map[string]any, error) {
	return map[string]any{"ran": name}, nil
}
func (mockClient) TriggerRestore(_ context.Context, args map[string]any) (map[string]any, error) {
	return map[string]any{"created": "restore", "namespace": args["namespace"]}, nil
}

func newH() *Handler { return New(skills.NewRegistry(mockClient{}, confirm.NewMemory())) }

func call(h http.Handler, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	h.ServeHTTP(w, r)
	return w
}

func TestInitialize(t *testing.T) {
	w := call(newH(), `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	if !strings.Contains(w.Body.String(), "2025-03-26") || !strings.Contains(w.Body.String(), "supkube-mcp") {
		t.Fatalf("initialize: %s", w.Body.String())
	}
}

func TestToolsList(t *testing.T) {
	w := call(newH(), `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	b := w.Body.String()
	// PRD-004 locked five must all be registered (C1 reconciled: get_backup_advice
	// proxies the live backend Advisor endpoint).
	for _, name := range []string{
		"list_k8s_workloads", "get_backup_advice", "get_backup_status",
		"create_backup_policy", "trigger_backup_execution",
	} {
		if !strings.Contains(b, name) {
			t.Fatalf("missing PRD-004 skill %s: %s", name, b)
		}
	}
}

func TestToolsCall(t *testing.T) {
	w := call(newH(), `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_k8s_workloads","arguments":{}}}`)
	if !strings.Contains(w.Body.String(), "prod-orders") {
		t.Fatalf("tools/call: %s", w.Body.String())
	}
}

func TestUnknownMethod(t *testing.T) {
	if w := call(newH(), `{"jsonrpc":"2.0","id":4,"method":"bogus"}`); !strings.Contains(w.Body.String(), "-32601") {
		t.Fatalf("want -32601: %s", w.Body.String())
	}
}

func TestUnknownTool(t *testing.T) {
	w := call(newH(), `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nope","arguments":{}}}`)
	if !strings.Contains(w.Body.String(), "unknown tool") {
		t.Fatalf("want unknown tool: %s", w.Body.String())
	}
}

// TC-AUTH-PATH: /mcp must only be reachable THROUGH the auth middleware.
// (The PR#66 spike's bug: endpoint dead in prod because tests bypassed middleware.)
func TestAuthPath(t *testing.T) {
	protected := auth.Require("secret")(newH())

	// no token → 401, handler never runs
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	protected.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no-token: want 401 got %d", w.Code)
	}

	// wrong token → 401
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	r2.Header.Set("Authorization", "Bearer wrong")
	protected.ServeHTTP(w2, r2)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-token: want 401 got %d", w2.Code)
	}

	// valid token → reaches handler (200 + tools)
	w3 := httptest.NewRecorder()
	r3 := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	r3.Header.Set("Authorization", "Bearer secret")
	protected.ServeHTTP(w3, r3)
	if w3.Code != http.StatusOK || !strings.Contains(w3.Body.String(), "list_k8s_workloads") {
		t.Fatalf("valid-token: want 200+tools got %d %s", w3.Code, w3.Body.String())
	}
}
