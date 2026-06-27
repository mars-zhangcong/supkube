package server

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	v1 "github.com/supkube/supkube-backend/internal/api/v1"
	"github.com/supkube/supkube-backend/internal/audit"
	"github.com/supkube/supkube-backend/internal/auditwatch"
	"github.com/supkube/supkube-backend/internal/auth"
	"github.com/supkube/supkube-backend/internal/clusterhealth"
	"github.com/supkube/supkube-backend/internal/csi"
	"github.com/supkube/supkube-backend/internal/drflow"
	"github.com/supkube/supkube-backend/internal/eventwatch"
	"github.com/supkube/supkube-backend/internal/fingerprint"
	"github.com/supkube/supkube-backend/internal/gc"
	"github.com/supkube/supkube-backend/internal/importpolicy"
	"github.com/supkube/supkube-backend/internal/k8s"
	"github.com/supkube/supkube-backend/internal/mcpconfirm"
	"github.com/supkube/supkube-backend/internal/metrics"
	"github.com/supkube/supkube-backend/internal/policypair"
	"github.com/supkube/supkube-backend/internal/velerons"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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

	// Terminal-event watcher: poll Velero Backup/Restore CRs and emit
	// *.completed/*.failed on the event bus (consumed by GET /api/v1/events).
	// Fills SupInsight's #2 gap — the thin-adapter "trigger → wait for
	// completion" needs terminal events to wait on.
	go func() {
		runtimeCli, err := k8s.GetRuntimeClient()
		if err != nil {
			log.Printf("[eventwatch] runtime client unavailable; terminal events disabled: %v", err)
			return
		}
		log.Printf("[eventwatch] terminal-event watcher started")
		eventwatch.Run(context.Background(), runtimeCli)
	}()

	// MCP HitL confirmation reaper (ADR-057 R1): periodically GC expired
	// confirmation Secrets. Get-time expiry already prevents stale confirms from
	// executing; this just keeps orphaned Secrets from accumulating.
	go func() {
		cli, err := k8s.GetRuntimeClient()
		if err != nil {
			log.Printf("[mcpconfirm] runtime client unavailable; confirmation reaper disabled: %v", err)
			return
		}
		st := mcpconfirm.New(cli, v1.ConfirmNamespace(), mcpconfirm.DefaultTTL)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if n, err := st.Reap(context.Background()); err != nil {
				log.Printf("[mcpconfirm] reap error: %v", err)
			} else if n > 0 {
				log.Printf("[mcpconfirm] reaped %d expired confirmation(s)", n)
			}
		}
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

	// v0.9.x #84: CSI auto-config controller. On startup (and every 10
	// min thereafter) walks installed CSIDrivers and creates a Velero-
	// tagged VolumeSnapshotClass for each known driver that doesn't
	// already have one. Fixes the #1 demo-blocker: Backup completes
	// "successfully" but with no PV data because no VSC was wired up.
	// See internal/csi/autoconfig.go for the known-driver table + the
	// "we don't override user-installed VSCs" rule.
	go func() {
		k8sCli, err := k8s.GetClient()
		if err != nil {
			log.Printf("[csi-autoconfig] kubernetes client unavailable; auto-config disabled: %v", err)
			return
		}
		dynCli, err := k8s.GetDynamicClient()
		if err != nil {
			log.Printf("[csi-autoconfig] dynamic client unavailable; auto-config disabled: %v", err)
			return
		}
		csi.Run(context.Background(), k8sCli, dynCli, 10*time.Minute)
	}()

	// PRD-007 §4.4 ImportPolicy fingerprint — source-side signer + dest-
	// side verifier + truststore. Source: ticker scans Velero Backups
	// every 30s, stamps a .supkube-fingerprint.json into BSL alongside
	// each Completed backup. Dest: ImportPolicy controller (Agent B)
	// calls Validator before any import. Truststore + SecretLoader are
	// shared. Failure to boot any one of these does NOT abort the whole
	// server — fingerprint enforcement degrades gracefully to "missing"
	// which the Validator then treats per the ImportPolicy mode.
	go func() {
		k8sCli, err := k8s.GetClient()
		if err != nil {
			log.Printf("[fingerprint] kubernetes client unavailable; fingerprint pipeline disabled: %v", err)
			return
		}
		runtimeCli, err := k8s.GetRuntimeClient()
		if err != nil {
			log.Printf("[fingerprint] runtime client unavailable; fingerprint pipeline disabled: %v", err)
			return
		}
		clusterID, clusterName := fingerprint.ResolveClusterIdentity(context.Background(), k8sCli)
		if clusterID == "" {
			log.Printf("[fingerprint] could not resolve source cluster ID (kube-system UID); writer disabled, validator still available")
		} else {
			log.Printf("[fingerprint] source cluster: %s (%s)", clusterName, clusterID)
		}
		secretLoader := fingerprint.NewK8sSecretLoader(k8sCli)
		bslCli := fingerprint.NewBSLClient(runtimeCli)
		writer := fingerprint.NewWriter(bslCli, secretLoader, clusterID, clusterName)
		// Validator + TrustStore are constructed here and made available
		// to the v1 API package via package-level setters (so the
		// ImportPolicy controller / handlers can pick them up without
		// reaching back into cmd/server).
		validator := fingerprint.NewValidator(bslCli, secretLoader)
		trustStore := fingerprint.NewTrustStore(k8sCli)
		v1.SetFingerprintDeps(validator, trustStore)
		if clusterID != "" {
			fingerprint.NewRunner(writer, runtimeCli).Run(context.Background())
		} else {
			// Block forever; validator side still works via SetFingerprintDeps.
			<-context.Background().Done()
		}
	}()

	// PRD-007 §4.4 ImportPolicy controller (Agent B). Per-CR goroutine
	// drives Continuous/Scheduled sync of BSL tarballs → Velero Backup
	// CR in this cluster. Wires:
	//   - BackupLister  → list BSL backups (v1: query Velero CR by storageLocation)
	//   - BackupImporter→ create/patch Velero Backup CR with import labels
	//   - Validator     → fingerprint validator (Agent C), via adapter
	//   - bslExists     → handler-side BSL existence check (POST/PUT validation)
	go func() {
		runtimeCli, err := k8s.GetRuntimeClient()
		if err != nil {
			log.Printf("[importpolicy] runtime client unavailable; controller disabled: %v", err)
			return
		}
		dynCli, err := k8s.GetDynamicClient()
		if err != nil {
			log.Printf("[importpolicy] dynamic client unavailable; controller disabled: %v", err)
			return
		}
		k8sCli, err := k8s.GetClient()
		if err != nil {
			log.Printf("[importpolicy] kubernetes client unavailable; controller disabled: %v", err)
			return
		}
		// sprint 2026-06-01 Agent G: 替换 lister/importer 为真 S3-driven 实现.
		// 旧的 NewVeleroBackupLister/Importer 仍 exported 作为回退 (见函数注释
		// 的 deprecation 标记), 出 S3 polling 问题时一行可换回去.
		// 真正驱动 RPO 的是这个: 我们自己 LIST BSL, 不再受 Velero
		// backupSyncPeriod (60-90s) 限制.
		bslCli := fingerprint.NewBSLClient(runtimeCli)
		// dedupe 用旧 veleroBackupLister wrap — 它查 Velero CR ns 内已有的
		// Backup, 是 "race-safe" 的兜底.
		dedupeSource := importpolicy.NewVeleroBackupLister(runtimeCli)
		lister := importpolicy.NewS3BackupLister(bslCli, dedupeSource)
		importer := importpolicy.NewS3BackupImporter(bslCli, runtimeCli)
		validator := &fingerprintAdapter{inner: fingerprint.NewValidator(
			fingerprint.NewBSLClient(runtimeCli),
			fingerprint.NewK8sSecretLoader(k8sCli),
		)}
		bslCheck := func(ctx context.Context, name string) (bool, error) {
			bsl := &velerov1.BackupStorageLocation{}
			err := runtimeCli.Get(ctx, types.NamespacedName{Namespace: velerons.Namespace(), Name: name}, bsl)
			switch {
			case err == nil:
				return true, nil
			case apierrors.IsNotFound(err):
				return false, nil
			default:
				return false, err
			}
		}
		ctrl := &importpolicy.Controller{
			DynCli:    dynCli,
			Lister:    lister,
			Importer:  importer,
			Validator: validator,
		}
		importpolicy.RegisterController(ctrl, dynCli, bslCheck)
		log.Printf("[importpolicy] controller started")
		ctrl.Run(context.Background())
	}()

	// ADR-053 DRFlowRunner clients. Initialized once here so the handler
	// registration and recovery goroutine below can reference them.
	// Failure to get a client disables DRFlow rather than aborting the server.
	var drflowK8s kubernetes.Interface
	var drflowDyn dynamic.Interface
	if k8sC, err := k8s.GetClient(); err != nil {
		log.Printf("[drflow] kubernetes client unavailable; DRFlow routes disabled: %v", err)
	} else if dynC, err := k8s.GetDynamicClient(); err != nil {
		log.Printf("[drflow] dynamic client unavailable; DRFlow routes disabled: %v", err)
	} else {
		drflowK8s = k8sC
		drflowDyn = dynC
		// ADR-053 D4 reconcile: resume any runs that were in-flight before
		// the last restart (goroutines die on restart; ConfigMaps survive).
		go drflow.RecoverInFlightRuns(context.Background(), drflowK8s, drflowDyn)
	}

	// v0.8.5: Authentication. AuthCfg holds the OIDC verifier + OAuth2
	// client; the actual discovery doc fetch is lazy (first request)
	// so backend boot doesn't depend on Dex being ready.
	authCfg := auth.LoadConfigFromEnv()

	// PRD-008 / ADR-039: Activity 审计持久层(嵌入式 SQLite on PV)。
	// feature gate(#24):SUPKUBE_AUDIT_DISABLED=1 一键关。fail-soft:打不开就跳过——审计是观测面,
	// 绝不能因它拖垮数据保护控制面;未接线时 audit.Emit 自动成无操作。
	if os.Getenv("SUPKUBE_AUDIT_DISABLED") != "1" {
		dbPath := os.Getenv("SUPKUBE_AUDIT_DB")
		if dbPath == "" {
			dbPath = "/data/audit/audit.db"
		}
		if id := os.Getenv("SUPKUBE_CLUSTER_ID"); id != "" {
			audit.DefaultClusterID = id
		}
		if mkErr := os.MkdirAll(filepath.Dir(dbPath), 0o755); mkErr != nil {
			log.Printf("[audit] 建目录失败,审计禁用: %v", mkErr)
		} else if store, oErr := audit.OpenSQLite(dbPath); oErr != nil {
			log.Printf("[audit] 打开 %s 失败,审计禁用: %v", dbPath, oErr)
		} else {
			audit.SetDefault(store)
			audit.StartTTLReaper(context.Background(), store, audit.DefaultRetention, 24*time.Hour)
			log.Printf("[audit] Activity 持久层就绪: %s (保留 %s)", dbPath, audit.DefaultRetention)
			// PRD-008 Phase 2b:异步删除(DBR 级联)的终态补写——Backup CR 消失即判完成。
			go func() {
				cli, cerr := k8s.GetRuntimeClient()
				if cerr != nil {
					log.Printf("[auditwatch] runtime client 不可用,删除终态补写禁用: %v", cerr)
					return
				}
				auditwatch.Run(context.Background(), cli)
			}()
		}
	}

	// #59 WI-024: /metrics 根引擎暴露所需的 client + 集群名(供下方 r.GET("/metrics") 路由)。
	runtimeCli, err := k8s.GetRuntimeClient()
	if err != nil {
		log.Printf("[metrics] runtime client unavailable; /metrics will expose partial data: %v", err)
	}
	k8sCli, err := k8s.GetClient()
	if err != nil {
		log.Printf("[metrics] kubernetes client unavailable; /metrics will expose partial data: %v", err)
	}
	_, clusterName := fingerprint.ResolveClusterIdentity(context.Background(), k8sCli)

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
	r.GET("/metrics", gin.WrapH(metrics.NewHandler(runtimeCli, k8sCli, clusterName)))

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
		// SSE event stream — agents (SupInsight / MCP server) subscribe to
		// react to Supkube state changes (incl. terminal *.completed/*.failed
		// from the eventwatch runner) without polling. Registered in RBAC table.
		api.GET("/events", v1.EventsHandler)
		api.GET("/namespaces", v1.ListNamespaces)
		api.POST("/namespaces", v1.CreateNamespace)
		api.GET("/dashboard/summary", v1.GetDashboardSummary)
		// PRD-029/N2 (Kopia 仓库服务): 列 Velero BackupRepository CR(去重可见的第一扇窗)。
		api.GET("/backup-repositories", v1.ListBackupRepositories)
		// DR posture export — canonical backup/DR snapshot for SupInsight ingest
		// (Velero + DRFlowRunner → engine-agnostic shape). See dr_posture.go.
		api.GET("/dr-posture", v1.GetDRPosture)
		api.GET("/applications", v1.ListApplications)
		api.GET("/applications/:namespace/details", v1.GetApplicationDetails)
		api.GET("/applications/:namespace/storage-capability", v1.GetNamespaceStorageCapability)
		// v0.9.x #84: status of the CSI auto-config controller. UI uses this
		// to show "X VSCs auto-created, Y drivers unknown" on Settings →
		// Storage tab. Read-only, viewer role is fine.
		api.GET("/storage/csi-autoconfig", v1.GetCSIAutoConfigStatus)

		// v0.9.x #79: Log Viewer — kills the "open a kubectl shell to see
		// what broke" loop. See internal/api/v1/logs.go for the scope of
		// v1 (read-only, 5s client poll, no live tail framework).
		api.GET("/logs/components", v1.GetLogComponents)
		api.GET("/logs", v1.GetLogs)
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
		// v0.9.x #68: emergency escape for a Backup CR whose finalizer is
		// wedged or whose DBR cascade has stalled. Strips finalizers +
		// direct-delete + schedules orphan GC. See ForceDeleteBackup doc
		// for what's bypassed vs. the normal DELETE path.
		api.POST("/backups/:name/force-delete", v1.ForceDeleteBackup)

		// Restores
		api.GET("/restores", v1.ListRestores)
		api.POST("/restores", v1.CreateRestore)
		api.POST("/restores/preflight", v1.PreflightRestore) // v0.7.12: conflict detection
		api.GET("/restores/:name", v1.GetRestore)
		api.GET("/restores/:name/results", v1.GetRestoreResults)
		api.DELETE("/restores/:name", v1.DeleteRestore)

		// MCP HitL confirmation store (ADR-057 R1) — internal, supkube-mcp client.
		api.POST("/mcp/confirmations", v1.CreateMCPConfirmation)
		api.GET("/mcp/confirmations/:id", v1.GetMCPConfirmation)
		api.DELETE("/mcp/confirmations/:id", v1.DeleteMCPConfirmation)

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
		// PRD-002 v1.3: atomic Transform CRUD (the layer below TransformSet).
		// TransformSets reference Transforms by name; deleting a Transform
		// that any TransformSet still references is refused with 409.
		api.GET("/transforms", v1.ListTransforms)
		api.POST("/transforms", v1.CreateTransform)
		api.GET("/transforms/:name", v1.GetTransform)
		api.PUT("/transforms/:name", v1.UpdateTransform)
		api.DELETE("/transforms/:name", v1.DeleteTransform)
		// v0.8.3: batched "Apply Suggested Fix" from Pre-flight. Frontend
		// sends the FULL list of currently-applied fixes every time so the
		// backend can merge into a single consolidated Transform Set
		// (Velero only honors one resourceModifierRef per Restore). The
		// ConfigMap name is stable per restoreName so re-clicks update
		// in place.
		api.POST("/transform-sets/apply-conflict-fixes", v1.ApplyConflictFixes)
		// PRD-002 v1.3 §4.3 (T1): dry-run preview of the compiled Velero
		// resourceModifierRef CM for a given TransformSet + params combo.
		// Doesn't create any CM — just returns the effective rules.yaml,
		// the derived CM name, and the hash. Used by RestoreDrawer to let
		// the user audit "what will actually be applied" before they
		// trigger the Restore. Viewer-role (read-only computation).
		api.POST("/transform-sets/:name/preview-resolution", v1.PreviewTransformSetResolution)

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

		// PRD-007 §4.3 Layer 4 Backup Copy (#111). Phase 1: Preflight only
		// (returns the {eligible, rejected} split with ERR_LAYER4_SNAPSHOT_UNSUPPORTED
		// for CSI-snapshot-only backups whose volume data lives in the cloud
		// provider's regional snapshot store, not the BSL). Phase 2 will
		// implement the actual rclone-driven Transfer; for now POST /backup-copy
		// returns 501. RBAC: Preflight = Editor, Transfer = Admin.
		api.POST("/backup-copy/preflight", v1.BackupCopyPreflight)
		api.POST("/backup-copy", v1.CreateBackupCopy)

		// PRD-007 §4.4 Import Policy (Agent B). REST + RBAC entries
		// (RBAC table extended via auth.RegisterPermissions inside
		// RegisterRoutes — keeps importpolicy's permission rules
		// co-located with handlers).
		importpolicy.RegisterRoutes(api)

		// PRD-011 §4.6 AI Backup Advisor: POST /ai/score (evaluator) +
		// GET /ai/explain/:taskId (SSE stub for the upcoming LLM
		// explainer). See internal/api/v1/ai_routes.go.
		v1.RegisterAIRoutes(api)

		// ADR-053 DRFlowRunner: POST /drflow (trigger), GET /drflow (list),
		// GET /drflow/:id (detail). Clients injected here so the handler
		// package stays client-free and testable without a real cluster.
		if drflowK8s != nil && drflowDyn != nil {
			drflow.RegisterRoutes(api, drflowK8s, drflowDyn)
		}
	}

	return r.Run(":8080")
}

// fingerprintAdapter bridges internal/fingerprint's Validator (Agent C)
// to importpolicy's smaller Validator interface (Agent B). The two
// FingerprintResult types differ in shape (fingerprint pkg carries the
// full *Fingerprint pointer; importpolicy only needs status + cluster
// id/name + reason for label + status updates). The adapter performs the
// shape conversion plus normalises the status enum strings.
type fingerprintAdapter struct {
	inner fingerprint.Validator
}

func (a *fingerprintAdapter) ValidateBackup(ctx context.Context, bsl, name string, mode importpolicy.FingerprintMode) (*importpolicy.FingerprintResult, error) {
	res, err := a.inner.ValidateBackup(ctx, bsl, name, fingerprint.FingerprintMode(mode))
	if err != nil {
		return nil, err
	}
	out := &importpolicy.FingerprintResult{
		Status: string(res.Status),
		Reason: res.Reason,
	}
	if res.Fingerprint != nil {
		out.SourceClusterID = res.Fingerprint.SourceClusterID
		out.SourceClusterName = res.Fingerprint.SourceClusterName
	}
	// importpolicy expects "valid"/"missing"/"invalid"; fingerprint pkg
	// adds "hmac-invalid"/"tampered" — collapse them to "invalid" for the
	// label so the SPA doesn't have to know about every sub-variant.
	switch out.Status {
	case "hmac-invalid", "tampered":
		out.Status = "invalid"
	case "disabled":
		out.Status = "valid"
	}
	return out, nil
}
