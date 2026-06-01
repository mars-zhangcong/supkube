// Package auth: rbac.go — central permission table + role-check middleware.
//
// Why a central table instead of per-handler annotations
// ───────────────────────────────────────────────────────
// Spreading permission checks across handlers makes audit hard ("which
// endpoints can an editor hit?" → grep all 30 handlers). A single table
// here is the source of truth — auditors and devs both look in one place.
//
// The check happens AFTER the auth middleware (which sets c.user), as
// the second middleware in the /api/v1/* chain. If the user's resolved
// role doesn't meet the path+method's minimum, we abort with 403.
//
// Namespace-scoped checks (editor restricted to specific ns) are NOT done
// here — they happen inside each mutating handler where we know which
// namespace the operation targets. Reason: the namespace might be in the
// URL path (DELETE /backups/:name → ns lookup needed), the request body
// (POST /backups), or both (POST /restores with mapping). Centralizing
// would require per-endpoint extraction config, which is more code than
// just calling CanAccessNamespace() in 6 handlers.
package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// permEntry says "<method> <pattern>" requires at least <minRole>.
type permEntry struct {
	method  string
	pattern string // gin path pattern, e.g. "/backups/:name"
	minRole string
}

// PermEntry is the exported version used by RegisterPermissions so
// downstream packages (e.g. internal/importpolicy) can extend the
// permission table at init without creating an import cycle through
// this package's private types.
type PermEntry struct {
	Method  string
	Pattern string
	MinRole string
}

// RegisterPermissions appends entries to permissionTable. Intended to
// be called from a sibling package's RegisterRoutes() so feature owners
// keep their permissions co-located with their handlers. Safe to call
// multiple times; duplicates (same method+pattern) are silently ignored
// so re-init in tests doesn't grow the table unbounded.
func RegisterPermissions(entries ...PermEntry) {
	for _, e := range entries {
		dup := false
		for _, existing := range permissionTable {
			if existing.method == e.Method && existing.pattern == e.Pattern {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		permissionTable = append(permissionTable, permEntry{
			method:  e.Method,
			pattern: e.Pattern,
			minRole: e.MinRole,
		})
	}
}

// permissionTable is the WHOLE authz surface in one place.
//
// Rules:
//   - viewer: read everything except audit + RBAC (audit comes in step 4)
//   - editor: writes on Backup/Restore/Schedule (with ns scope check in handlers)
//   - admin:  everything (BSL/VSL CRUD, Transform Sets, NS create/delete)
//
// Endpoints that need NO role check (the auth flow itself, /status, demo
// mode): handled by isAuthBypassPath in auth.go, never reach this table.
var permissionTable = []permEntry{
	// ─── Read-only — viewer + above ─────────────────────────────────
	{"GET", "/api/v1/status", RoleViewer},
	{"GET", "/api/v1/dashboard/summary", RoleViewer},
	{"GET", "/api/v1/applications", RoleViewer},
	{"GET", "/api/v1/applications/:namespace/details", RoleViewer},
	{"GET", "/api/v1/applications/:namespace/storage-capability", RoleViewer},
	{"GET", "/api/v1/backup-advisor", RoleViewer},
	{"GET", "/api/v1/backup-advisor/:namespace", RoleViewer},
	{"GET", "/api/v1/namespaces", RoleViewer},
	{"GET", "/api/v1/backups", RoleViewer},
	{"GET", "/api/v1/backups/:name", RoleViewer},
	{"GET", "/api/v1/backups/:name/resources", RoleViewer},
	{"GET", "/api/v1/backups/:name/artifacts", RoleViewer},
	{"GET", "/api/v1/backups/:name/errors", RoleViewer},
	{"GET", "/api/v1/backups/:name/artifact-breakdown", RoleViewer},
	{"GET", "/api/v1/resources/yaml", RoleViewer},
	{"GET", "/api/v1/restores", RoleViewer},
	{"GET", "/api/v1/restores/:name", RoleViewer},
	{"GET", "/api/v1/restores/:name/results", RoleViewer},
	{"GET", "/api/v1/actions", RoleViewer},
	{"GET", "/api/v1/actions/:id", RoleViewer},
	{"GET", "/api/v1/transform-sets", RoleViewer},
	{"GET", "/api/v1/transform-sets/:name", RoleViewer},
	// PRD-002 v1.3: atomic Transform CRUD (the layer below TransformSet).
	// Same role bands as TransformSets — read viewer, write admin.
	{"GET", "/api/v1/transforms", RoleViewer},
	{"GET", "/api/v1/transforms/:name", RoleViewer},
	{"GET", "/api/v1/schedules", RoleViewer},
	{"GET", "/api/v1/schedules/:name", RoleViewer},
	{"GET", "/api/v1/storage-locations", RoleViewer},
	{"GET", "/api/v1/storage-locations/:name", RoleViewer},
	{"GET", "/api/v1/volume-snapshot-locations", RoleViewer},
	{"GET", "/api/v1/volume-snapshot-locations/:name", RoleViewer},

	// ─── Editor writes (with in-handler ns scope check) ─────────────
	{"POST", "/api/v1/backups", RoleEditor},
	{"DELETE", "/api/v1/backups/:name", RoleEditor},
	// v0.8.9.2: Applications-page one-click Snapshot. Editor with
	// ns-scope check (handler calls RequireNamespaceAccess on the URL
	// :namespace). Functionally equivalent to POST /backups in terms of
	// "creates a Backup CR" — same min role.
	{"POST", "/api/v1/applications/:namespace/snapshot", RoleEditor},
	{"POST", "/api/v1/restores", RoleEditor},
	{"POST", "/api/v1/restores/preflight", RoleEditor},
	{"DELETE", "/api/v1/restores/:name", RoleEditor},
	{"POST", "/api/v1/schedules", RoleEditor},
	{"PATCH", "/api/v1/schedules/:name", RoleEditor},
	{"POST", "/api/v1/schedules/:name/run-once", RoleEditor},
	{"DELETE", "/api/v1/schedules/:name", RoleEditor},
	// Transform Set ops the editor needs to drive Pre-flight Apply Fix
	{"POST", "/api/v1/transform-sets/apply-conflict-fixes", RoleEditor},
	// PRD-002 v1.3 §4.3 (T1): preview-resolution is read-only (no CM
	// create; dry-run compile). Viewer can call it to audit what a
	// TransformSet+params combo will compile to before the editor
	// triggers the Restore. Same role as GET /transform-sets/:name.
	{"POST", "/api/v1/transform-sets/:name/preview-resolution", RoleViewer},

	// /auth/me — every authenticated user reads their OWN identity
	// (the canonical "who am I"). Must be viewer-or-above (i.e. any
	// successful auth). Without this entry RBAC's fail-closed default
	// kills the SPA's bootstrap probe with 403, which silently leaves
	// stale user data in localStorage and hides RBAC-driven UI bits
	// like the role chip and My Access / Audit Log tabs.
	{"GET", "/api/v1/auth/me", RoleViewer},

	// ─── Admin only ─────────────────────────────────────────────────
	// v0.8.5 step 3.5: read-only RBAC bindings audit
	{"GET", "/api/v1/auth/rbac/bindings", RoleAdmin},
	// v0.8.5 step 4: audit log query
	{"GET", "/api/v1/audit-logs", RoleAdmin},
	// v0.8.8 Cluster Hygiene. GET is admin too because the last-run
	// summary contains counts of orphans (info we don't want surfacing
	// to non-admins). Manual cleanup + settings mutation are strictly
	// admin.
	{"GET", "/api/v1/settings/cleanup", RoleAdmin},
	{"PUT", "/api/v1/settings/cleanup", RoleAdmin},
	{"POST", "/api/v1/admin/cleanup/orphans", RoleAdmin},
	// v0.8.11: white-label branding. GET viewer-or-above so the
	// sidebar/header can render on every page load; PUT admin-only.
	{"GET", "/api/v1/settings/branding", RoleViewer},
	{"PUT", "/api/v1/settings/branding", RoleAdmin},
	// v0.8.12 LBS1: local backup store status (read-only). Viewer-or-above
	// so the Storage Locations page render works without admin role.
	{"GET", "/api/v1/local-store/status", RoleViewer},
	// v0.8.12.5: DR Topology aggregator for the Dashboard. Viewer is the
	// right level — Dashboard is the first page every authenticated user
	// sees; gating this admin-only would hide the visualization for
	// editors/viewers which contradicts the page's purpose.
	{"GET", "/api/v1/dashboard/topology", RoleViewer},
	// v0.8.13: Plugin status (Settings → Plugins tab). Viewer can see
	// plugin install state; mutation is via `helm upgrade` from the
	// admin's workstation, not via API, so no PUT/POST exists.
	{"GET", "/api/v1/plugins/status", RoleViewer},
	// v0.9.0 MC1: Cluster registry. Read is viewer (everyone needs to
	// see the cluster list to switch context). Write is admin —
	// registering a cluster effectively gives SupKube write access to
	// the remote cluster via the uploaded kubeconfig.
	{"GET", "/api/v1/clusters", RoleViewer},
	{"GET", "/api/v1/clusters/:name", RoleViewer},
	{"POST", "/api/v1/clusters", RoleAdmin},
	{"DELETE", "/api/v1/clusters/:name", RoleAdmin},
	{"POST", "/api/v1/clusters/:name/test", RoleAdmin},
	{"POST", "/api/v1/clusters/test", RoleAdmin},
	// v0.9.0.2 MCM Dashboard aggregator (read-only). Viewer-or-above
	// so non-admins can see the aggregated cross-cluster picture.
	{"GET", "/api/v1/multicluster/summary", RoleViewer},
	// Namespace lifecycle (cluster-wide)
	{"POST", "/api/v1/namespaces", RoleAdmin},
	// Transform Set management
	{"POST", "/api/v1/transform-sets", RoleAdmin},
	{"PUT", "/api/v1/transform-sets/:name", RoleAdmin},
	{"DELETE", "/api/v1/transform-sets/:name", RoleAdmin},
	// PRD-002 v1.3: atomic Transform write CRUD.
	{"POST", "/api/v1/transforms", RoleAdmin},
	{"PUT", "/api/v1/transforms/:name", RoleAdmin},
	{"DELETE", "/api/v1/transforms/:name", RoleAdmin},
	// Storage Profile management
	{"POST", "/api/v1/storage-locations", RoleAdmin},
	{"PUT", "/api/v1/storage-locations/:name", RoleAdmin},
	{"DELETE", "/api/v1/storage-locations/:name", RoleAdmin},
	{"POST", "/api/v1/storage-locations/:name/verify", RoleAdmin},
	// Snapshot Profile management
	{"POST", "/api/v1/volume-snapshot-locations", RoleAdmin},
	{"DELETE", "/api/v1/volume-snapshot-locations/:name", RoleAdmin},

	// v0.9.x #79 Log Viewer. Every logged-in user (viewer+) needs logs
	// to diagnose their own backups/restores; we don't want to gate
	// "open Observability → Logs" behind admin. The actual K8s log
	// retrieval RBAC is enforced by the backend ServiceAccount, not by
	// SupKube role — so a viewer can't see logs the SA can't read either.
	{"GET", "/api/v1/logs/components", RoleViewer},
	{"GET", "/api/v1/logs", RoleViewer},

	// v0.9.x #84 CSI auto-config status (read-only).  Viewer because
	// the Settings → Storage tab surfaces "X drivers auto-configured"
	// for everyone; the underlying VSC create happens in a startup
	// controller (no API surface for that yet).
	{"GET", "/api/v1/storage/csi-autoconfig", RoleViewer},

	// v0.9.x #68 Force-delete: same role as DELETE /backups/:name
	// (Editor with in-handler ns scope check; whole-cluster backup
	// upgraded to admin inside ForceDeleteBackup).
	{"POST", "/api/v1/backups/:name/force-delete", RoleEditor},

	// PRD-007 §4.3 Layer 4 Backup Copy (#111).
	// Preflight is read-only diagnostics (lists which backups are eligible
	// for BSL→BSL copy vs which are CSI-snapshot-only and rejected with
	// ERR_LAYER4_SNAPSHOT_UNSUPPORTED) — Editor matches Restore preflight.
	// The actual Transfer (Phase 2) is admin-only because it moves bytes
	// across BSLs and can incur cloud egress charges.
	{"POST", "/api/v1/backup-copy/preflight", RoleEditor},
	{"POST", "/api/v1/backup-copy", RoleAdmin},
}

// RBACMiddleware returns a Gin handler that enforces the permission
// table. Must be placed AFTER the auth middleware (which sets user).
//
// When RBAC is disabled (RBAC_ENABLED=false), this middleware is a no-op —
// the user.Role was set to "admin" by ResolveRole and IsAtLeast always
// returns true. So you can install this middleware unconditionally; the
// behavior degrades gracefully.
func (c *Config) RBACMiddleware() gin.HandlerFunc {
	return func(gc *gin.Context) {
		// Auth-bypass paths skip role check too (they're not in our table).
		if isAuthBypassPath(gc.Request.URL.Path) {
			gc.Next()
			return
		}
		user := FromContext(gc)
		if user == nil {
			// Defense in depth: auth middleware should have set this.
			gc.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}
		minRole, found := requiredRoleFor(gc.Request.Method, gc.FullPath())
		if !found {
			// Endpoint not in table — fail closed. Forces us to keep
			// the table in sync when new endpoints land.
			gc.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "endpoint not in RBAC table (this is a SupKube bug — please report)",
			})
			return
		}
		if !user.IsAtLeast(minRole) {
			gc.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":    "insufficient role",
				"required": minRole,
				"current":  user.Role,
			})
			return
		}
		gc.Next()
	}
}

// requiredRoleFor looks up the minimum role for a (method, fullPath) pair.
// fullPath uses Gin's pattern form (e.g. "/api/v1/backups/:name"), not the
// rendered URL.
func requiredRoleFor(method, fullPath string) (string, bool) {
	for _, e := range permissionTable {
		if strings.EqualFold(e.method, method) && e.pattern == fullPath {
			return e.minRole, true
		}
	}
	return "", false
}

// RequireNamespaceAccess is a helper for mutating handlers that operate
// on a specific namespace (Backup with includedNamespaces, Restore with
// namespaceMapping, Schedule, etc). Call it AFTER parsing the request,
// passing every ns the operation will touch.
//
// Behavior:
//   - admin / viewer: always allowed (cluster-wide)
//   - editor: every ns must be in user.NamespaceScope
//
// On rejection it writes 403 and returns false; caller should `return`.
func RequireNamespaceAccess(gc *gin.Context, namespaces ...string) bool {
	user := FromContext(gc)
	if user == nil {
		gc.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return false
	}
	if user.Role == RoleAdmin || user.Role == RoleViewer {
		return true
	}
	// editor — every namespace must be in scope.
	for _, ns := range namespaces {
		if !user.CanAccessNamespace(ns) {
			gc.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":     "namespace not in your scope",
				"namespace": ns,
				"scope":     user.NamespaceScope,
			})
			return false
		}
	}
	return true
}
