package server

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	v1 "github.com/supkube/supkube-backend/internal/api/v1"
	"github.com/supkube/supkube-backend/internal/auth"
	"github.com/supkube/supkube-backend/internal/gc"
	"github.com/supkube/supkube-backend/internal/k8s"
	"github.com/supkube/supkube-backend/internal/clusterhealth"
	"github.com/supkube/supkube-backend/internal/policypair"
)

func Run() error {
	// v0.8.2: Pre-seed built-in Transform Sets (strip-nodeport / clusterip /
	// loadbalancer-ip / pv-binding). Idempotent + non-blocking — server
	// starts immediately, seeding happens in a goroutine.
	v1.SeedBuiltinTransformSets()

	// v0.8.8: Orphan-GC background runner. Reads settings from the
	// supkube-settings ConfigMap each tick; if enabled (default), runs
	// gc.Scan every intervalHours (default 6). Velero v1.18's Data
	// Mover leaks VolumeSnapshotContents on backup deletion (upstream
	// issue #7838); this runner is SupKube's compensating layer. See
	// ADR-024 for the design rationale.
	go func() {
		runtimeCli, err := k8s.GetRuntimeClient()
		if err != nil {
			log.Printf("[gc] runtime client unavailable; orphan GC disabled: %v", err)
			return
		}
		k8sCli, err := k8s.GetClient()
		if err != nil {
			log.Printf("[gc] kubernetes client unavailable; orphan GC disabled: %v", err)
			return
		}
		log.Printf("[gc] orphan GC runner started")
		gc.Run(context.Background(), runtimeCli, k8sCli)
	}()

	// v0.8.10 Plan-B: dual-policy pair controller. Watches snapshot-half
	// Backup completion → immediately fires the paired export-half. This
	// collapses the inter-half data gap from ~7 minutes (cron-driven) to
	// ~30 seconds (event-driven). See internal/policypair/policypair.go
	// for the full design rationale, including Plan-C forward compat.
	go func() {
		runtimeCli, err := k8s.GetRuntimeClient()
		if err != nil {
			log.Printf("[policypair] runtime client unavailable; controller disabled: %v", err)
			return
		}
		k8sCli, err := k8s.GetClient()
		if err != nil {
			log.Printf("[policypair] kubernetes client unavailable; controller disabled: %v", err)
			return
		}
		policypair.Run(context.Background(), runtimeCli, k8sCli)
	}()

	// v0.9.0 MC2: cluster health-check controller. Polls registered
	// Cluster CRs every 60s — dials each remote cluster's API server
	// and patches .status (phase/k8sVersion/nodeCount/velero version).
	// Independent of policypair; failure to start one doesn't affect
	// the other. See internal/clusterhealth/clusterhealth.go.
	go func() {
		dynCli, err := k8s.GetDynamicClient()
		if err != nil {
			log.Printf("[clusterhealth] dynamic client unavailable; controller disabled: %v", err)
			return
		}
		k8sCli, err := k8s.GetClient()
		if err != nil {
			log.Printf("[clusterhealth] kubernetes client unavailable; controller disabled: %v", err)
			return
		}
		clusterhealth.Run(context.Background(), dynCli, k8sCli)
	}()

	// v0.8.5: Authentication. AuthCfg holds the OIDC verifier + OAuth2
	// client; the actual discovery doc fetch is lazy (first request)
	// so backend boot doesn't depend on Dex being ready.
	authCfg := auth.LoadConfigFromEnv()

	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// API v1 routes
	api := r.Group("/api/v1")
	// Middleware order matters:
	//
	// 1. AUDIT first — registers a deferred post-handler that ALWAYS runs
	//    (even when downstream middleware aborts with 401/403). This lets
	//    us record denied attempts, which is the most interesting thing
	//    for compliance audit.
	//
	// 2. AUTH next — validates Bearer JWT, injects user into context.
	//    Aborts 401 if missing/invalid.
	//
	// 3. RBAC last — checks user.Role against the central permission table.
	//    Aborts 403 if insufficient role.
	//
	// All three execute their post-handler code (after gc.Next()) on the
	// way back up the stack regardless of aborts — that's how audit sees
	// the final status code even when auth/rbac abort.
	api.Use(authCfg.AuditMiddleware())
	api.Use(authCfg.Middleware())
	api.Use(authCfg.RBACMiddleware())
	{
		// v0.8.5 Auth endpoints
		api.GET("/auth/providers", authCfg.ListProviders)
		api.POST("/auth/callback", authCfg.Callback)
		api.GET("/auth/me", authCfg.Me)
		api.POST("/auth/logout", authCfg.Logout)
		// v0.8.5 step 3.5: read-only RBAC bindings (admin-only).
		api.GET("/auth/rbac/bindings", authCfg.ListRBACBindings)
		// v0.8.5 step 4: audit log query (admin-only).
		api.GET("/audit-logs", auth.ListAuditLogs)

		api.GET("/status", v1.GetStatus)
		api.GET("/namespaces", v1.ListNamespaces)
		api.POST("/namespaces", v1.CreateNamespace)
		api.GET("/dashboard/summary", v1.GetDashboardSummary)
		api.GET("/applications", v1.ListApplications)
		api.GET("/applications/:namespace/details", v1.GetApplicationDetails)
		api.GET("/applications/:namespace/storage-capability", v1.GetNamespaceStorageCapability)
		// v0.8.9.2: one-click Snapshot button on the Applications page.
		// Distinct from POST /backups because it has zero config — pure
		// cluster-local CSI snapshot with hard-coded sensible defaults.
		// See internal/api/v1/manual_snapshot.go header for rationale.
		api.POST("/applications/:namespace/snapshot", v1.CreateManualSnapshot)

		// Backup Advisor (v0.7.5) — recommendations only, no automatic action
		api.GET("/backup-advisor", v1.GetBackupAdvisor)
		api.GET("/backup-advisor/:namespace", v1.GetBackupAdvisorForNamespace)

		// Backups
		api.GET("/backups", v1.ListBackups)
		api.POST("/backups", v1.CreateBackup)
		api.GET("/backups/:name", v1.GetBackup)
		api.GET("/backups/:name/resources", v1.GetBackupResources)
		api.GET("/backups/:name/artifacts", v1.ListBackupArtifacts)
		// v0.8.8.1: aggregated error messages for a Backup (DataUpload +
		// PodVolumeBackup status.message). Counterpart to GetRestoreResults
		// for Restores; closes the §11.3 silent-error gap that v0.8.3
		// TODO comment flagged for Backups.
		api.GET("/backups/:name/errors", v1.GetBackupErrors)
		// v0.8.10.1: grouped artifact breakdown for the Kasten-style
		// Action Details drawer. Lighter than /artifacts (no YAML) and
		// classifies items into "application" vs "infrastructure" so
		// the drawer can show "Application Items: 25, Infrastructure: 13".
		// Same data path powers the new Application Items column on
		// the Restore Points page (v0.8.10.1 task #24).
		api.GET("/backups/:name/artifact-breakdown", v1.GetBackupArtifactBreakdown)
		// v0.8.10.3: lazy YAML fetch for the per-artifact </> button in
		// the Action/Application/Restore Point drawers. Single resource
		// at a time — avoids the 100-500 KB cost of embedding YAML for
		// every item in the breakdown response.
		api.GET("/resources/yaml", v1.GetResourceYAML)
		api.DELETE("/backups/:name", v1.DeleteBackup)

		// Restores
		api.GET("/restores", v1.ListRestores)
		api.POST("/restores", v1.CreateRestore)
		api.POST("/restores/preflight", v1.PreflightRestore) // v0.7.12: conflict detection
		api.GET("/restores/:name", v1.GetRestore)
		api.GET("/restores/:name/results", v1.GetRestoreResults)
		api.DELETE("/restores/:name", v1.DeleteRestore)

		// v0.8.0 Activity / Actions — unified stream replacing the
		// per-CRD pages. Aggregates Backups + Restores into Action shape.
		api.GET("/actions", v1.ListActions)
		api.GET("/actions/:id", v1.GetAction)

		// v0.8.2 Transform Sets — Velero ResourceModifier ConfigMaps with
		// SupKube-specific labels for managed CRUD. Used during Restore
		// to apply JSONPath patches that fix cross-ns / cross-cluster
		// conflicts (NodePort, clusterIP, image registry, etc.).
		api.GET("/transform-sets", v1.ListTransformSets)
		api.POST("/transform-sets", v1.CreateTransformSet)
		api.GET("/transform-sets/:name", v1.GetTransformSet)
		api.PUT("/transform-sets/:name", v1.UpdateTransformSet)
		api.DELETE("/transform-sets/:name", v1.DeleteTransformSet)
		// v0.8.3: batched "Apply Suggested Fix" from Pre-flight. Frontend
		// sends the FULL list of currently-applied fixes every time so the
		// backend can merge into a single consolidated Transform Set
		// (Velero only honors one resourceModifierRef per Restore). The
		// ConfigMap name is stable per restoreName so re-clicks update
		// in place.
		api.POST("/transform-sets/apply-conflict-fixes", v1.ApplyConflictFixes)

		// Schedules
		api.GET("/schedules", v1.ListSchedules)
		api.POST("/schedules", v1.CreateSchedule)
		api.GET("/schedules/:name", v1.GetSchedule)
		api.PATCH("/schedules/:name", v1.PatchSchedule)
		api.POST("/schedules/:name/run-once", v1.RunScheduleOnce)
		api.DELETE("/schedules/:name", v1.DeleteSchedule)

		// Storage locations
		api.GET("/storage-locations", v1.ListStorageLocations)
		api.POST("/storage-locations", v1.CreateStorageLocationWithSecret)
		api.GET("/storage-locations/:name", v1.GetStorageLocation)
		api.PUT("/storage-locations/:name", v1.UpdateStorageLocation)
		api.DELETE("/storage-locations/:name", v1.DeleteStorageLocation)
		api.POST("/storage-locations/:name/verify", v1.VerifyStorageLocation)

		// Volume snapshot locations (CSI / cloud-native snapshots)
		api.GET("/volume-snapshot-locations", v1.ListVolumeSnapshotLocations)
		api.POST("/volume-snapshot-locations", v1.CreateVolumeSnapshotLocation)
		api.GET("/volume-snapshot-locations/:name", v1.GetVolumeSnapshotLocation)
		api.DELETE("/volume-snapshot-locations/:name", v1.DeleteVolumeSnapshotLocation)

		// v0.8.8 Cluster Hygiene — Settings + manual orphan cleanup.
		// All admin-only (enforced by the RBAC table). Settings GET
		// is wider in case we later let non-admin see last-run health.
		api.GET("/settings/cleanup", v1.GetCleanupSettings)
		api.PUT("/settings/cleanup", v1.UpdateCleanupSettings)
		api.POST("/admin/cleanup/orphans", v1.RunOrphanCleanup)
		// v0.8.11: white-label branding (sidebar logo + product name +
		// favicon). GET is viewer-or-above so EVERY page-load can render
		// the header; PUT is admin-only (rebrand is a privileged action).
		api.GET("/settings/branding", v1.GetBranding)
		api.PUT("/settings/branding", v1.UpdateBranding)
		// v0.8.12 LBS1: local backup store (in-cluster MinIO) status.
		// Read-only — provisioning is owned by Helm. UI uses this to
		// render the Storage Locations "Local Backup Store" card.
		api.GET("/local-store/status", v1.GetLocalStoreStatus)
		// v0.8.12.5 DR Topology Dashboard. Aggregates clusters +
		// BSLs + flows + 3-2-1-1-0 score into one payload so the
		// above-the-fold Dashboard card renders in one round-trip.
		api.GET("/dashboard/topology", v1.GetTopology)
		// v0.8.13 HC4: optional component (plugin) status for the
		// Settings → Plugins tab. Returns per-plugin install state +
		// the helm upgrade command the admin pastes to toggle each.
		api.GET("/plugins/status", v1.GetPluginsStatus)
		// v0.9.0 MC1: Multi-Cluster Manager registry. All admin-only
		// (registering a cluster bestows write access on it).
		api.GET("/clusters", v1.ListClusters)
		api.GET("/clusters/:name", v1.GetCluster)
		api.POST("/clusters", v1.CreateCluster)
		api.DELETE("/clusters/:name", v1.DeleteCluster)
		api.POST("/clusters/:name/test", v1.TestClusterConnection)
		api.POST("/clusters/test", v1.TestClusterConnection)
		// v0.9.0.2 MCM Dashboard aggregator. Backend parallel fan-out
		// + 5s timeout per remote; one round-trip for the SPA.
		api.GET("/multicluster/summary", v1.GetMultiClusterSummary)
	}

	return r.Run(":8080")
}
