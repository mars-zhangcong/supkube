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

	// PoC uses the in-memory confirm store; production swaps in the CR-backed
	// store for cross-replica HitL (ADR-057) — same Store interface.
	reg := skills.NewRegistry(supkubeclient.NewHTTP(backend, backendToken), confirm.NewMemory())
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
