// Package skills is the MCP skill registry.
//
// Read skills run directly. Write skills go through HitL (ADR-057): first call
// returns dry-run + confirm_id; second call (with confirm_id) executes using the
// SNAPSHOT's inputs — the second call's inputs are ignored. Same confirm
// mechanism is intended to serve PRD-013 four-eyes + orchestration approval
// ("build once, use thrice", per SupInsight).
package skills

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/supkube/mcp-server/internal/confirm"
	"github.com/supkube/mcp-server/internal/supkubeclient"
)

type Skill struct {
	Name        string
	Description string
	InputSchema map[string]any
	Write       bool
	Run         func(ctx context.Context, args map[string]any) (any, error) // read skills
	DryRun      func(ctx context.Context, args map[string]any) (any, error) // write: preview, no side-effect
	Execute     func(ctx context.Context, args map[string]any) (any, error) // write: do it
}

type Registry struct {
	skills   []Skill
	confirms confirm.Store
}

func NewRegistry(c supkubeclient.Client, store confirm.Store) *Registry {
	return &Registry{
		confirms: store,
		skills: []Skill{
			listK8sWorkloads(c), // read (Phase-1 PoC)
			getRestoreStatus(c), // read — #3 authoritative status fallback
			triggerRestore(c),   // write — #1 HitL
			triggerBackup(c),    // write — #1 HitL
		},
	}
}

func (r *Registry) ToolDefs() []map[string]any {
	out := make([]map[string]any, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, map[string]any{
			"name": s.Name, "description": s.Description, "inputSchema": s.InputSchema, "write": s.Write,
		})
	}
	return out
}

// Call dispatches a tool. Read → Run. Write → HitL: no confirm_id ⇒ dry-run +
// persist + requires_confirmation; with confirm_id ⇒ execute using the snapshot.
func (r *Registry) Call(ctx context.Context, name string, args map[string]any) (string, bool) {
	s := r.find(name)
	if s == nil {
		return "unknown tool: " + name, true
	}
	if !s.Write {
		return marshal(s.Run(ctx, args))
	}
	if cid, _ := args["confirm_id"].(string); cid != "" {
		snap, ok := r.confirms.Get(cid)
		if !ok {
			return "confirm_id invalid or expired — re-run for a fresh dry-run", true
		}
		if snap.Skill != name {
			return "confirm_id does not belong to this tool", true
		}
		r.confirms.Delete(cid)                    // one-shot (anti-replay)
		return marshal(s.Execute(ctx, snap.Args)) // IGNORE this call's args; use snapshot
	}
	// first call: dry-run + persist + ask for confirmation
	preview, err := s.DryRun(ctx, args)
	if err != nil {
		return err.Error(), true
	}
	id := r.confirms.Put(confirm.Snapshot{
		Skill: name, Args: normalize(args), DryRun: preview,
		Created: time.Now(), Expires: time.Now().Add(confirm.DefaultTTL),
	})
	b, _ := json.MarshalIndent(map[string]any{
		"requires_confirmation": true,
		"confirm_id":            id,
		"expires_in_seconds":    int(confirm.DefaultTTL.Seconds()),
		"dry_run":               preview,
		"note":                  "call again with this confirm_id to execute; the inputs are locked to this dry-run",
	}, "", "  ")
	return string(b), false
}

func (r *Registry) find(name string) *Skill {
	for i := range r.skills {
		if r.skills[i].Name == name {
			return &r.skills[i]
		}
	}
	return nil
}

func marshal(res any, err error) (string, bool) {
	if err != nil {
		return err.Error(), true
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	return string(b), false
}

// normalize drops control args (confirm_id) so the snapshot holds only business inputs.
func normalize(args map[string]any) map[string]any {
	out := make(map[string]any, len(args))
	for k, v := range args {
		if k == "confirm_id" {
			continue
		}
		out[k] = v
	}
	return out
}

// ───────── skills ─────────

func listK8sWorkloads(c supkubeclient.Client) Skill {
	return Skill{
		Name: "list_k8s_workloads",
		Description: "List Kubernetes namespaces with workload counts and backup protection status. " +
			"(PoC: per-namespace counts from supkube-backend; full per-workload detail needs a backend endpoint — flagged.)",
		InputSchema: obj(map[string]any{
			"cluster":   str("target cluster id (optional)"),
			"namespace": str("filter to one namespace (optional)"),
		}),
		Run: func(ctx context.Context, args map[string]any) (any, error) {
			cluster, _ := args["cluster"].(string)
			ns, _ := args["namespace"].(string)
			return c.ListWorkloads(ctx, cluster, ns)
		},
	}
}

func getRestoreStatus(c supkubeclient.Client) Skill {
	return Skill{
		Name:        "get_restore_status",
		Description: "Get a restore's current status summary (phase/errors). Authoritative status for reliable waiting (SSE is lossy fast-path).",
		InputSchema: obj(map[string]any{"name": str("restore name (required)")}),
		Run: func(ctx context.Context, args map[string]any) (any, error) {
			name, _ := args["name"].(string)
			if name == "" {
				return nil, errors.New("name is required")
			}
			return c.GetRestoreStatus(ctx, name)
		},
	}
}

func triggerRestore(c supkubeclient.Client) Skill {
	return Skill{
		Name:        "trigger_restore",
		Write:       true,
		Description: "Trigger a Velero restore from a backup. Returns a dry-run preview + confirm_id first; call again with confirm_id to execute (HitL, ADR-057).",
		InputSchema: obj(map[string]any{
			"backupName": str("backup to restore from (required)"),
			"namespace":  str("target namespace (required)"),
		}),
		DryRun: func(_ context.Context, args map[string]any) (any, error) {
			return map[string]any{
				"action": "restore", "backupName": args["backupName"], "namespace": args["namespace"],
				"preview": "would create a Velero Restore; NOT executed until confirmed",
			}, nil
		},
		Execute: func(ctx context.Context, args map[string]any) (any, error) {
			return c.TriggerRestore(ctx, args)
		},
	}
}

func triggerBackup(c supkubeclient.Client) Skill {
	return Skill{
		Name:        "trigger_backup",
		Write:       true,
		Description: "Trigger an immediate Velero backup. Returns a dry-run preview + confirm_id first; call again with confirm_id to execute (HitL, ADR-057).",
		InputSchema: obj(map[string]any{
			"name":      str("backup name (required)"),
			"namespace": str("namespace to back up (required)"),
		}),
		DryRun: func(_ context.Context, args map[string]any) (any, error) {
			return map[string]any{
				"action": "backup", "name": args["name"], "namespace": args["namespace"],
				"preview": "would create a Velero Backup; NOT executed until confirmed",
			}, nil
		},
		Execute: func(ctx context.Context, args map[string]any) (any, error) {
			return c.TriggerBackup(ctx, args)
		},
	}
}

func obj(props map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": props}
}
func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
