package skills

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/supkube/mcp-server/internal/confirm"
	"github.com/supkube/mcp-server/internal/supkubeclient"
)

type mock struct{ lastRestoreNS string }

func (m *mock) ListWorkloads(context.Context, string, string) ([]supkubeclient.Workload, error) {
	return nil, nil
}
func (m *mock) GetRestoreStatus(_ context.Context, name string) (map[string]any, error) {
	return map[string]any{"name": name, "phase": "Completed"}, nil
}
func (m *mock) TriggerRestore(_ context.Context, args map[string]any) (map[string]any, error) {
	m.lastRestoreNS, _ = args["namespace"].(string)
	return map[string]any{"created": "restore", "namespace": args["namespace"]}, nil
}
func (m *mock) TriggerBackup(_ context.Context, args map[string]any) (map[string]any, error) {
	return map[string]any{"created": "backup", "name": args["name"]}, nil
}

func confirmID(s string) string {
	var m map[string]any
	_ = json.Unmarshal([]byte(s), &m)
	id, _ := m["confirm_id"].(string)
	return id
}

// HitL: first call = dry-run (no execute) + confirm_id; second call executes
// using the SNAPSHOT inputs, IGNORING the second call's inputs (ADR-057 / DoD#6).
func TestHitL_DryRunThenConfirm_IgnoresSecondInputs(t *testing.T) {
	mc := &mock{}
	r := NewRegistry(mc, confirm.NewMemory())

	out, isErr := r.Call(context.Background(), "trigger_restore",
		map[string]any{"backupName": "b1", "namespace": "foo"})
	if isErr || !strings.Contains(out, "requires_confirmation") || !strings.Contains(out, "confirm_id") {
		t.Fatalf("first call should dry-run: %s", out)
	}
	if mc.lastRestoreNS != "" {
		t.Fatal("dry-run must NOT execute")
	}

	cid := confirmID(out)
	out2, isErr2 := r.Call(context.Background(), "trigger_restore",
		map[string]any{"confirm_id": cid, "namespace": "bar"}) // tamper attempt: bar
	if isErr2 {
		t.Fatalf("confirm failed: %s", out2)
	}
	if mc.lastRestoreNS != "foo" {
		t.Fatalf("must execute snapshot ns=foo and ignore 2nd-call ns=bar; got %q", mc.lastRestoreNS)
	}
}

func TestHitL_OneShot_NoReplay(t *testing.T) {
	r := NewRegistry(&mock{}, confirm.NewMemory())
	out, _ := r.Call(context.Background(), "trigger_backup",
		map[string]any{"name": "bk", "namespace": "foo"})
	cid := confirmID(out)
	if _, isErr := r.Call(context.Background(), "trigger_backup", map[string]any{"confirm_id": cid}); isErr {
		t.Fatal("first confirm should succeed")
	}
	out3, isErr := r.Call(context.Background(), "trigger_backup", map[string]any{"confirm_id": cid})
	if !isErr || !strings.Contains(out3, "invalid or expired") {
		t.Fatalf("replay must fail (one-shot): %s", out3)
	}
}

func TestHitL_UnknownConfirm(t *testing.T) {
	r := NewRegistry(&mock{}, confirm.NewMemory())
	out, isErr := r.Call(context.Background(), "trigger_restore", map[string]any{"confirm_id": "nope"})
	if !isErr || !strings.Contains(out, "invalid or expired") {
		t.Fatalf("unknown confirm_id must fail: %s", out)
	}
}

func TestReadSkill_GetRestoreStatus(t *testing.T) {
	r := NewRegistry(&mock{}, confirm.NewMemory())
	out, isErr := r.Call(context.Background(), "get_restore_status", map[string]any{"name": "r1"})
	if isErr || !strings.Contains(out, "Completed") {
		t.Fatalf("get_restore_status: %s", out)
	}
}

func TestReadSkill_MissingArg(t *testing.T) {
	r := NewRegistry(&mock{}, confirm.NewMemory())
	out, isErr := r.Call(context.Background(), "get_restore_status", map[string]any{})
	if !isErr || !strings.Contains(out, "required") {
		t.Fatalf("missing name should error: %s", out)
	}
}
