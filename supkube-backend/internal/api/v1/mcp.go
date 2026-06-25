package v1

// mcp.go — Supkube MCP Server (PRD-004 "Supkube Skills") · MVP
// -----------------------------------------------------------------------------
// Streamable-HTTP MCP endpoint: a single POST /api/v1/mcp speaking JSON-RPC 2.0
// (initialize / tools/list / tools/call). Lets any external AI agent (OpenClaw,
// customer agents, LHF) drive Supkube as MCP tools instead of bespoke REST glue.
//
// MVP scope = READ-ONLY skills (query state). Destructive skills (trigger
// backup/restore/delete) are deliberately NOT exposed yet — they must come
// behind the Four-Eyes/approval design (PRD-013) before going over MCP.
//
// Reuses the same per-request client helpers (getRequestRuntimeClient /
// getRequestKubernetesClient) so cluster routing (X-Supkube-Cluster header) and
// auth apply identically to MCP callers.

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/supkube/supkube-backend/internal/velerons"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const mcpProtocolVersion = "2024-11-05"

// ---- JSON-RPC 2.0 envelopes ----

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type mcpResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *mcpError   `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// emptyObjectSchema = a JSON Schema accepting an optional {namespace} string.
func nsInputSchema() interface{} {
	return gin.H{
		"type": "object",
		"properties": gin.H{
			"namespace": gin.H{"type": "string", "description": "Velero namespace (default: server config)"},
		},
	}
}

// skill = one MCP tool implementation. Returns a JSON-serialisable result.
type skill struct {
	tool mcpTool
	run  func(c *gin.Context, args map[string]interface{}) (interface{}, error)
}

func mcpSkills() []skill {
	return []skill{
		{
			tool: mcpTool{Name: "supkube_dashboard_summary",
				Description: "Backup posture summary: total backups + counts by phase (Completed/Failed/InProgress).",
				InputSchema: nsInputSchema()},
			run: skillDashboardSummary,
		},
		{
			tool: mcpTool{Name: "supkube_list_backups",
				Description: "List Velero backups (name, namespace, phase, createdAt).",
				InputSchema: nsInputSchema()},
			run: skillListBackups,
		},
		{
			tool: mcpTool{Name: "supkube_list_restores",
				Description: "List Velero restores (name, backup, phase).",
				InputSchema: nsInputSchema()},
			run: skillListRestores,
		},
		{
			tool: mcpTool{Name: "supkube_list_storage_locations",
				Description: "List Velero BackupStorageLocations (name, provider, phase).",
				InputSchema: nsInputSchema()},
			run: skillListStorageLocations,
		},
		{
			tool: mcpTool{Name: "supkube_list_namespaces",
				Description: "List Kubernetes namespaces in the (header-routed) target cluster.",
				InputSchema: gin.H{"type": "object", "properties": gin.H{}}},
			run: skillListNamespaces,
		},
	}
}

// MCPHandler is the single Streamable-HTTP MCP endpoint (POST /api/v1/mcp).
func MCPHandler(c *gin.Context) {
	var req mcpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, mcpResponse{JSONRPC: "2.0", ID: nil,
			Error: &mcpError{Code: -32700, Message: "parse error: " + err.Error()}})
		return
	}

	// Notifications (no id, "notifications/*") expect no result.
	if req.ID == nil && len(req.Method) >= 13 && req.Method[:13] == "notifications" {
		c.Status(http.StatusAccepted)
		return
	}

	switch req.Method {
	case "initialize":
		c.JSON(http.StatusOK, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: gin.H{
			"protocolVersion": mcpProtocolVersion,
			"serverInfo":      gin.H{"name": "supkube", "version": "0.1.0"},
			"capabilities":    gin.H{"tools": gin.H{}},
		}})

	case "tools/list":
		tools := []mcpTool{}
		for _, s := range mcpSkills() {
			tools = append(tools, s.tool)
		}
		c.JSON(http.StatusOK, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: gin.H{"tools": tools}})

	case "tools/call":
		var p struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &p)
		var impl *skill
		for i := range mcpSkillsCache {
			if mcpSkillsCache[i].tool.Name == p.Name {
				impl = &mcpSkillsCache[i]
				break
			}
		}
		if impl == nil {
			c.JSON(http.StatusOK, mcpResponse{JSONRPC: "2.0", ID: req.ID,
				Error: &mcpError{Code: -32602, Message: "unknown tool: " + p.Name}})
			return
		}
		out, err := impl.run(c, p.Arguments)
		if err != nil {
			// MCP convention: tool errors surface as isError content, not protocol error.
			c.JSON(http.StatusOK, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: gin.H{
				"isError": true,
				"content": []gin.H{{"type": "text", "text": err.Error()}},
			}})
			return
		}
		blob, _ := json.MarshalIndent(out, "", "  ")
		c.JSON(http.StatusOK, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: gin.H{
			"content": []gin.H{{"type": "text", "text": string(blob)}},
		}})

	default:
		c.JSON(http.StatusOK, mcpResponse{JSONRPC: "2.0", ID: req.ID,
			Error: &mcpError{Code: -32601, Message: "method not found: " + req.Method}})
	}
}

var mcpSkillsCache = mcpSkills()

// ---- helpers ----

func argNamespace(args map[string]interface{}) string {
	if args != nil {
		if v, ok := args["namespace"].(string); ok && v != "" {
			return v
		}
	}
	return velerons.Namespace()
}

// ---- skill implementations (read-only) ----

func skillListBackups(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	cl, err := getRequestRuntimeClient(c)
	if err != nil {
		return nil, err
	}
	list := &velerov1.BackupList{}
	if err := cl.List(context.Background(), list, client.InNamespace(argNamespace(args))); err != nil {
		return nil, err
	}
	rows := make([]gin.H, 0, len(list.Items))
	for _, b := range list.Items {
		rows = append(rows, gin.H{
			"name": b.Name, "namespace": b.Namespace,
			"phase": string(b.Status.Phase), "createdAt": b.CreationTimestamp.Time,
		})
	}
	return gin.H{"total": len(rows), "backups": rows}, nil
}

func skillListRestores(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	cl, err := getRequestRuntimeClient(c)
	if err != nil {
		return nil, err
	}
	list := &velerov1.RestoreList{}
	if err := cl.List(context.Background(), list, client.InNamespace(argNamespace(args))); err != nil {
		return nil, err
	}
	rows := make([]gin.H, 0, len(list.Items))
	for _, r := range list.Items {
		rows = append(rows, gin.H{
			"name": r.Name, "backup": r.Spec.BackupName, "phase": string(r.Status.Phase),
		})
	}
	return gin.H{"total": len(rows), "restores": rows}, nil
}

func skillListStorageLocations(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	cl, err := getRequestRuntimeClient(c)
	if err != nil {
		return nil, err
	}
	list := &velerov1.BackupStorageLocationList{}
	if err := cl.List(context.Background(), list, client.InNamespace(argNamespace(args))); err != nil {
		return nil, err
	}
	rows := make([]gin.H, 0, len(list.Items))
	for _, b := range list.Items {
		rows = append(rows, gin.H{
			"name": b.Name, "provider": b.Spec.Provider, "phase": string(b.Status.Phase),
		})
	}
	return gin.H{"total": len(rows), "storageLocations": rows}, nil
}

func skillDashboardSummary(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	cl, err := getRequestRuntimeClient(c)
	if err != nil {
		return nil, err
	}
	list := &velerov1.BackupList{}
	if err := cl.List(context.Background(), list, client.InNamespace(argNamespace(args))); err != nil {
		return nil, err
	}
	var completed, failed, inProgress int
	for _, b := range list.Items {
		switch b.Status.Phase {
		case velerov1.BackupPhaseCompleted:
			completed++
		case velerov1.BackupPhaseFailed, velerov1.BackupPhasePartiallyFailed:
			failed++
		case velerov1.BackupPhaseInProgress:
			inProgress++
		}
	}
	return gin.H{
		"totalBackups": len(list.Items),
		"completed":    completed,
		"failed":       failed,
		"inProgress":   inProgress,
	}, nil
}

func skillListNamespaces(c *gin.Context, _ map[string]interface{}) (interface{}, error) {
	k8s, err := getRequestKubernetesClient(c)
	if err != nil {
		return nil, err
	}
	nsList, err := k8s.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(nsList.Items))
	for _, n := range nsList.Items {
		names = append(names, n.Name)
	}
	return gin.H{"total": len(names), "namespaces": names}, nil
}
