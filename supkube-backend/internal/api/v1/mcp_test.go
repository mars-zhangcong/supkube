package v1

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func callMCP(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mcp", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	MCPHandler(c)
	return w
}

func TestMCPInitialize(t *testing.T) {
	w := callMCP(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"protocolVersion"`) || !strings.Contains(w.Body.String(), `"supkube"`) {
		t.Fatalf("initialize missing fields: %s", w.Body.String())
	}
}

func TestMCPToolsList(t *testing.T) {
	w := callMCP(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	body := w.Body.String()
	for _, name := range []string{
		"supkube_dashboard_summary", "supkube_list_backups", "supkube_list_restores",
		"supkube_list_storage_locations", "supkube_list_namespaces",
	} {
		if !strings.Contains(body, name) {
			t.Fatalf("tools/list missing %s: %s", name, body)
		}
	}
}

func TestMCPUnknownMethod(t *testing.T) {
	w := callMCP(t, `{"jsonrpc":"2.0","id":3,"method":"bogus"}`)
	if !strings.Contains(w.Body.String(), "-32601") {
		t.Fatalf("expected method-not-found: %s", w.Body.String())
	}
}

func TestMCPUnknownTool(t *testing.T) {
	w := callMCP(t, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"nope","arguments":{}}}`)
	if !strings.Contains(w.Body.String(), "unknown tool") {
		t.Fatalf("expected unknown tool: %s", w.Body.String())
	}
}
