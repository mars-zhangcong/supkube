// supkube-mcp — open-source MCP server (PRD-004 "Supkube Skills").
// Separate process; clients (OpenClaw / Claude Desktop / SupInsight / any MCP
// agent) speak MCP over Streamable HTTP to /mcp; this server calls
// supkube-backend's REST API. Apache-2.0.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/supkube/mcp-server/internal/auth"
	"github.com/supkube/mcp-server/internal/confirm"
	"github.com/supkube/mcp-server/internal/mcpproto"
	"github.com/supkube/mcp-server/internal/skills"
	"github.com/supkube/mcp-server/internal/supkubeclient"
)

func main() {
	backend := getenv("SUPKUBE_BACKEND_URL", "http://supkube-backend")
	backendToken := os.Getenv("SUPKUBE_API_TOKEN") // server → backend
	clientToken := os.Getenv("MCP_BEARER_TOKEN")   // agent → server
	addr := getenv("MCP_ADDR", ":8080")

	if clientToken == "" {
		log.Println("WARN: MCP_BEARER_TOKEN unset — /mcp will reject all requests (fail-closed)")
	}

	// HitL confirm store (ADR-057 R1): when a backend is configured, persist
	// confirmations there (backend-managed K8s Secret = cross-replica). Only fall
	// back to in-memory (single-replica) when there's no backend at all.
	var store confirm.Store
	if backend != "" {
		store = confirm.NewBackendStore(backend, backendToken)
		log.Printf("HitL confirm store: backend Secret-backed (cross-replica) via %s", backend)
	} else {
		store = confirm.NewMemory()
		log.Println("HitL confirm store: in-memory (single-replica; no backend configured)")
	}
	reg := skills.NewRegistry(supkubeclient.NewHTTP(backend, backendToken), store)
	h := mcpproto.New(reg)

	mux := http.NewServeMux()
	// /mcp is wrapped by the auth middleware — never registered bare.
	mux.Handle("/mcp", auth.Require(clientToken)(h))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

	log.Printf("supkube-mcp on %s → backend %s (protocol %s)", addr, backend, mcpproto.ProtocolVersion)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
