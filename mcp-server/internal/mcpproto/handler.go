// Package mcpproto is the MCP JSON-RPC 2.0 handler over Streamable HTTP
// (single POST /mcp). Protocol version 2025-03-26 per ADR-034 (Streamable HTTP,
// not the deprecated 2024-11-05 HTTP+SSE that the PR#66 spike wrongly used).
package mcpproto

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/supkube/mcp-server/internal/skills"
)

const ProtocolVersion = "2025-03-26"

type rpcReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcErr         `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Handler struct{ Reg *skills.Registry }

func New(reg *skills.Registry) *Handler { return &Handler{Reg: reg} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only (Streamable HTTP single endpoint)", http.StatusMethodNotAllowed)
		return
	}
	var req rpcReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		write(w, rpcResp{JSONRPC: "2.0", Error: &rpcErr{-32700, "parse error: " + err.Error()}})
		return
	}
	// Notifications (no id, "notifications/*") expect no response.
	if len(req.ID) == 0 && strings.HasPrefix(req.Method, "notifications") {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	switch req.Method {
	case "initialize":
		write(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": ProtocolVersion,
			"serverInfo":      map[string]any{"name": "supkube-mcp", "version": "0.1.0"},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		}})
	case "tools/list":
		write(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": h.Reg.ToolDefs()}})
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Name == "" {
			write(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Error: &rpcErr{-32602, "invalid params: need tool name"}})
			return
		}
		text, isErr := h.Reg.Call(r.Context(), p.Name, p.Arguments)
		write(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"isError": isErr,
			"content": []map[string]any{{"type": "text", "text": text}},
		}})
	default:
		write(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Error: &rpcErr{-32601, "method not found: " + req.Method}})
	}
}

func write(w http.ResponseWriter, r rpcResp) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(r)
}
