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
			// PRD-004 locked five.
			listK8sWorkloads(c),       // read
			getBackupAdvice(c),        // read
			getBackupStatus(c),        // read
			createBackupPolicy(c),     // write — HitL
			triggerBackupExecution(c), // write — HitL
			// SupInsight extras (restore orchestration).
			getRestoreStatus(c), // read — #3 authoritative status fallback
			triggerRestore(c),   // write — HitL
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
		snap, ok, err := r.confirms.Get(ctx, cid)
		if err != nil {
			return "confirm store error: " + err.Error(), true
		}
		if !ok {
			return "confirm_id invalid or expired — re-run for a fresh dry-run", true
		}
		if snap.Skill != name {
			return "confirm_id does not belong to this tool", true
		}
		if err := r.confirms.Delete(ctx, cid); err != nil { // one-shot (anti-replay)
			return "confirm store error: " + err.Error(), true
		}
		return marshal(s.Execute(ctx, snap.Args)) // IGNORE this call's args; use snapshot
	}
	// first call: dry-run + persist + ask for confirmation
	preview, err := s.DryRun(ctx, args)
	if err != nil {
		return err.Error(), true
	}
	id, err := r.confirms.Put(ctx, confirm.Snapshot{Skill: name, Args: normalize(args), DryRun: preview})
	if err != nil {
		return "confirm store error: " + err.Error(), true
	}
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

func getBackupAdvice(c supkubeclient.Client) Skill {
	return Skill{
		Name:        "get_backup_advice",
		Description: "Get the backup Advisor's recommendation for a namespace (score/tier/factors). Proxies the live backend Advisor.",
		InputSchema: obj(map[string]any{"namespace": str("namespace to advise on (required)")}),
		Run: func(ctx context.Context, args map[string]any) (any, error) {
			ns, _ := args["namespace"].(string)
			if ns == "" {
				return nil, errors.New("namespace is required")
			}
			return c.GetBackupAdvice(ctx, ns)
		},
	}
}

func getBackupStatus(c supkubeclient.Client) Skill {
	return Skill{
		Name:        "get_backup_status",
		Description: "Get a backup's current status summary (phase/errors). Structured, ≤4KB (PRD-004 §4.3).",
		InputSchema: obj(map[string]any{"name": str("backup name (required)")}),
		Run: func(ctx context.Context, args map[string]any) (any, error) {
			name, _ := args["name"].(string)
			if name == "" {
				return nil, errors.New("name is required")
			}
			return c.GetBackupStatus(ctx, name)
		},
	}
}

func createBackupPolicy(c supkubeclient.Client) Skill {
	return Skill{
		Name:        "create_backup_policy",
		Write:       true,
		Description: "Create a backup policy (schedule). Returns a dry-run preview + confirm_id first; call again with confirm_id to execute (HitL, ADR-057).",
		InputSchema: obj(map[string]any{
			"name":               str("policy name (required)"),
			"schedule":           str("cron schedule, e.g. '0 2 * * *' (required)"),
			"includedNamespaces": str("namespaces to back up (comma-separated or array; optional)"),
		}),
		DryRun: func(_ context.Context, args map[string]any) (any, error) {
			return map[string]any{
				"action": "create_backup_policy", "name": args["name"], "schedule": args["schedule"],
				"includedNamespaces": args["includedNamespaces"],
				"preview":            "would create a backup schedule; NOT executed until confirmed",
			}, nil
		},
		Execute: func(ctx context.Context, args map[string]any) (any, error) {
			return c.CreateBackupPolicy(ctx, args)
		},
	}
}

func triggerBackupExecution(c supkubeclient.Client) Skill {
	return Skill{
		Name:        "trigger_backup_execution",
		Write:       true,
		Description: "Trigger an existing backup policy to run immediately (run-once). Returns a dry-run preview + confirm_id first; call again with confirm_id to execute (HitL, ADR-057).",
		InputSchema: obj(map[string]any{"policyName": str("name of the existing backup policy/schedule to run now (required)")}),
		DryRun: func(_ context.Context, args map[string]any) (any, error) {
			return map[string]any{
				"action": "run_backup_policy", "policyName": args["policyName"],
				"preview": "would run the policy once immediately; NOT executed until confirmed",
			}, nil
		},
		Execute: func(ctx context.Context, args map[string]any) (any, error) {
			name, _ := args["policyName"].(string)
			if name == "" {
				return nil, errors.New("policyName is required")
			}
			return c.RunBackupPolicy(ctx, name)
		},
	}
}

func obj(props map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": props}
}
func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
