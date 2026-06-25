package auth

import "testing"

// F1 regression guard (same lesson as /events): the MCP confirmation endpoints
// must be in permissionTable or RBAC fail-closes them to 403 in prod.
func TestMCPConfirmRoutesInPermissionTable(t *testing.T) {
	cases := []struct{ method, path string }{
		{"POST", "/api/v1/mcp/confirmations"},
		{"GET", "/api/v1/mcp/confirmations/:id"},
		{"DELETE", "/api/v1/mcp/confirmations/:id"},
	}
	for _, c := range cases {
		role, ok := requiredRoleFor(c.method, c.path)
		if !ok {
			t.Fatalf("F1 regression: %s %s not in permissionTable → would 403 in prod", c.method, c.path)
		}
		if role != RoleEditor {
			t.Fatalf("%s %s should require RoleEditor, got %q", c.method, c.path, role)
		}
	}
}
