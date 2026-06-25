package auth

import "testing"

// F1 regression guard: every routed endpoint MUST be in permissionTable, else
// the RBAC middleware fail-closes it to 403 in production. The closed PR#66
// spike shipped /mcp + /events NOT registered → prod-dead 403. This locks in
// /events. (Apply the same guard when new routes land.)
func TestEventsRouteInPermissionTable(t *testing.T) {
	role, ok := requiredRoleFor("GET", "/api/v1/events")
	if !ok {
		t.Fatal("F1 regression: GET /api/v1/events not in permissionTable → would 403 in prod")
	}
	if role != RoleViewer {
		t.Fatalf("GET /api/v1/events should require RoleViewer, got %q", role)
	}
}
