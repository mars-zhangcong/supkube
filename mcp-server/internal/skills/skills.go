// Package skills is the MCP skill registry. PRD-004 locks 5 skills; per the
// independent PRR conditions only a subset is registered now:
//   - C1: get_backup_advice deferred (depends on PRD-003 Advisor Engine).
//   - C3: write skills (create_backup_policy / trigger_backup_execution) deferred
//     until the HitL-confirm-storage ADR is accepted.
//
// Phase-1 PoC (C5) registers list_k8s_workloads only.
package skills

import (
	"context"
	"encoding/json"

	"github.com/supkube/mcp-server/internal/supkubeclient"
)

type Skill struct {
	Name        string
	Description string
	InputSchema map[string]any
	Run         func(ctx context.Context, args map[string]any) (any, error)
}

type Registry struct{ skills []Skill }

func NewRegistry(c supkubeclient.Client) *Registry {
	return &Registry{skills: []Skill{listK8sWorkloads(c)}}
}

func (r *Registry) ToolDefs() []map[string]any {
	out := make([]map[string]any, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, map[string]any{
			"name": s.Name, "description": s.Description, "inputSchema": s.InputSchema,
		})
	}
	return out
}

// Call runs a skill. Returns (text, isError) per MCP tools/call result shape.
func (r *Registry) Call(ctx context.Context, name string, args map[string]any) (string, bool) {
	for _, s := range r.skills {
		if s.Name == name {
			res, err := s.Run(ctx, args)
			if err != nil {
				return err.Error(), true
			}
			b, _ := json.MarshalIndent(res, "", "  ")
			return string(b), false
		}
	}
	return "unknown tool: " + name, true
}

func listK8sWorkloads(c supkubeclient.Client) Skill {
	return Skill{
		Name: "list_k8s_workloads",
		Description: "List Kubernetes namespaces with workload counts and backup protection status in a cluster. " +
			"(PoC: returns per-namespace workload counts from supkube-backend; full per-workload name/kind/status/replicas needs a dedicated backend endpoint — flagged for Phase 2.)",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cluster":   map[string]any{"type": "string", "description": "target cluster id (optional)"},
				"namespace": map[string]any{"type": "string", "description": "filter to one namespace (optional)"},
			},
		},
		Run: func(ctx context.Context, args map[string]any) (any, error) {
			cluster, _ := args["cluster"].(string)
			ns, _ := args["namespace"].(string)
			return c.ListWorkloads(ctx, cluster, ns)
		},
	}
}
