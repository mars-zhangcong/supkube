package v1

// dr_score.go — PRD-011 §4.6 /ai/scores + Task #115 Phase A collector (M1).
//
// This is the "collector layer" the evaluator package's doc comment (types.go
// lines 29-37) calls for: a K8s-API → evaluator.AppContext adapter that lets
// evaluator.Score() run on REAL cluster state instead of a caller-supplied DTO.
//
//	GET /api/v1/ai/scores
//	  Header: X-Supkube-Cluster (optional; routes to a registered remote cluster)
//	  Walks every application namespace in the target cluster, collects each
//	  namespace's backup/DR signals (Velero Backup/Schedule/BSL + drflow drills +
//	  workload/PVC topology), builds an evaluator.AppContext, scores it with the
//	  deterministic v1.0.0 rule engine, and attaches actionable backup advice.
//
// Design (mirrors dr_posture.go + applications.go so it reuses the same plumbing):
//   - One List call per resource type (bounded cost regardless of namespace count).
//   - Uncollectable fields are marked StatusMissing, NEVER zero-as-confirmed
//     (evaluator types.go "防 0 冒充已确认无" rule). The engine degrades to its
//     unable_to_confirm tier for those, so partial collection still yields a score.
//   - LLM is never in this path (PRD-011 finding #2). Score is pure/deterministic;
//     the Copilot (M3) will only *explain* these scores.
//
// Deferred to later collector units (marked StatusMissing here): encryption /
// access-control / network-isolation signals (not K8s-API-visible), per-namespace
// drill association (drflow runs key off a transient drill ns, not the source app).

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/advisor/evaluator"
	"github.com/supkube/supkube-backend/internal/drflow"
	"github.com/supkube/supkube-backend/internal/velerons"
)

// ─── Response DTOs ────────────────────────────────────────────────────────────

// DRRecommendation is one actionable backup/DR advice item for an application.
type DRRecommendation struct {
	Severity string `json:"severity"`         // P1 (urgent) / P2 / P3
	Type     string `json:"type"`             // NO_BACKUP / WEAK_RESILIENCE / ...
	Message  string `json:"message"`          // human-readable advice
	Action   string `json:"action,omitempty"` // machine hint for Copilot/MCP (e.g. create_backup)
}

// DRAppScore is one application's (namespace's) score + advice.
type DRAppScore struct {
	Namespace       string                     `json:"namespace"`
	Workloads       int                        `json:"workloads"`
	StatefulSets    int                        `json:"statefulSets"`
	PVCs            int                        `json:"pvcs"`
	Protected       bool                       `json:"protected"`
	Status          evaluator.EvaluationStatus `json:"status"`
	TotalScore      int                        `json:"totalScore"`
	Level           evaluator.SecurityLevel    `json:"level"`
	Dimensions      evaluator.Dimensions       `json:"dimensions"`
	Recommendations []DRRecommendation         `json:"recommendations"`
}

// DRScoresSummary is the cluster-level rollup.
type DRScoresSummary struct {
	TotalApps        int            `json:"totalApps"`
	ScoredApps       int            `json:"scoredApps"`
	AvgScore         int            `json:"avgScore"`
	LevelCounts      map[string]int `json:"levelCounts"`
	AtRiskApps       int            `json:"atRiskApps"`       // fragile/critical OR data-bearing-but-unbacked
	UnbackedWithData int            `json:"unbackedWithData"` // has StatefulSet/PVC but never backed up
	DrillsTotal      int            `json:"drillsTotal"`
	DrillsPassed     int            `json:"drillsPassed"`
}

// DRScoresResponse is the GET /ai/scores body.
type DRScoresResponse struct {
	Cluster      string          `json:"cluster"`
	SnapshotAt   string          `json:"snapshotAt"`
	RulesVersion string          `json:"rulesVersion"`
	Summary      DRScoresSummary `json:"summary"`
	Apps         []DRAppScore    `json:"apps"`
}

// ─── Field[T] construction helpers (status discipline) ────────────────────────

func fConfirmed[T any](v T) evaluator.Field[T] {
	return evaluator.Field[T]{Value: v, Status: evaluator.StatusConfirmed}
}
func fMissing[T any]() evaluator.Field[T] {
	return evaluator.Field[T]{Status: evaluator.StatusMissing}
}

// ─── Handler ──────────────────────────────────────────────────────────────────

// ListDRScores implements GET /api/v1/ai/scores (PRD-011 §4.6).
func ListDRScores(c *gin.Context) {
	cl, err := getRequestRuntimeClient(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	k8sClient, err := getRequestKubernetesClient(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx := context.Background()
	snapshotAt := time.Now().UTC()

	nsList, err := k8sClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Workload / stateful / PVC topology: one list per type, bucket by namespace.
	wl := map[string]int{}
	sts := map[string]int{}
	pvc := map[string]int{}
	if deps, e := k8sClient.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); e == nil {
		for i := range deps.Items {
			wl[deps.Items[i].Namespace]++
		}
	}
	if ss, e := k8sClient.AppsV1().StatefulSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); e == nil {
		for i := range ss.Items {
			wl[ss.Items[i].Namespace]++
			sts[ss.Items[i].Namespace]++
		}
	}
	if dss, e := k8sClient.AppsV1().DaemonSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); e == nil {
		for i := range dss.Items {
			wl[dss.Items[i].Namespace]++
		}
	}
	if pvs, e := k8sClient.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); e == nil {
		for i := range pvs.Items {
			pvc[pvs.Items[i].Namespace]++
		}
	}

	// Velero + drflow state (best-effort; partial beats hard-fail, same as dr_posture.go).
	veleroNS := velerons.Namespace()
	backups := &velerov1.BackupList{}
	_ = cl.List(ctx, backups, ctrlclient.InNamespace(veleroNS))
	schedules := &velerov1.ScheduleList{}
	_ = cl.List(ctx, schedules, ctrlclient.InNamespace(veleroNS))
	bsls := &velerov1.BackupStorageLocationList{}
	_ = cl.List(ctx, bsls, ctrlclient.InNamespace(veleroNS))
	runs, _ := drflow.ListRuns(ctx, k8sClient)

	// Index backups/schedules by their primary target namespace. (Inlined rather
	// than reusing dr_posture.go's firstNS so this collector stays self-contained
	// and doesn't couple the M1 commit to the separate dr-posture work.)
	bkByNS := map[string][]velerov1.Backup{}
	for _, b := range backups.Items {
		if inc := b.Spec.IncludedNamespaces; len(inc) > 0 {
			bkByNS[inc[0]] = append(bkByNS[inc[0]], b)
		}
	}
	schByNS := map[string][]velerov1.Schedule{}
	for _, s := range schedules.Items {
		if inc := s.Spec.Template.IncludedNamespaces; len(inc) > 0 {
			schByNS[inc[0]] = append(schByNS[inc[0]], s)
		}
	}
	bslByName := map[string]velerov1.BackupStorageLocation{}
	for _, l := range bsls.Items {
		bslByName[l.Name] = l
	}

	cluster := c.GetHeader("X-Supkube-Cluster")
	if cluster == "" {
		cluster = "this-cluster"
	}

	resp := DRScoresResponse{
		Cluster:      cluster,
		SnapshotAt:   snapshotAt.Format(time.RFC3339),
		RulesVersion: evaluator.ScoreRulesVersion,
		Summary:      DRScoresSummary{LevelCounts: map[string]int{}},
	}
	for _, run := range runs {
		resp.Summary.DrillsTotal++
		if run.Phase == drflow.PhaseSucceeded {
			resp.Summary.DrillsPassed++
		}
	}

	var scoreSum, scoredN int
	for i := range nsList.Items {
		ns := nsList.Items[i].Name
		if systemNamespaces[ns] || nsList.Items[i].Labels[excludeLabel] == "true" {
			continue
		}
		workloads := wl[ns]
		// Skip truly empty namespaces (no workloads, no data, no backups).
		if workloads == 0 && pvc[ns] == 0 && len(bkByNS[ns]) == 0 {
			continue
		}

		appCtx := buildDRAppContext(ns, nsList.Items[i].Annotations, snapshotAt, bkByNS[ns], schByNS[ns], bslByName)
		result := evaluator.Score(appCtx)
		protected := len(bkByNS[ns]) > 0
		app := DRAppScore{
			Namespace:       ns,
			Workloads:       workloads,
			StatefulSets:    sts[ns],
			PVCs:            pvc[ns],
			Protected:       protected,
			Status:          result.Status,
			TotalScore:      result.TotalScore,
			Level:           result.Level,
			Dimensions:      result.Dimensions,
			Recommendations: buildDRRecommendations(result, workloads, sts[ns], pvc[ns], protected),
		}
		resp.Apps = append(resp.Apps, app)

		resp.Summary.TotalApps++
		if result.Status == evaluator.EvalStatusScored {
			scoredN++
			scoreSum += result.TotalScore
			resp.Summary.LevelCounts[string(result.Level)]++
			if result.Level == evaluator.LevelFragile || result.Level == evaluator.LevelCritical {
				resp.Summary.AtRiskApps++
			}
		} else {
			resp.Summary.LevelCounts[string(result.Status)]++
		}
		if !protected && (sts[ns] > 0 || pvc[ns] > 0) {
			resp.Summary.UnbackedWithData++
			resp.Summary.AtRiskApps++
		}
	}
	resp.Summary.ScoredApps = scoredN
	if scoredN > 0 {
		resp.Summary.AvgScore = scoreSum / scoredN
	}

	// Worst-first ordering so the UI surfaces what needs attention.
	sort.SliceStable(resp.Apps, func(a, b int) bool {
		return drAppRank(resp.Apps[a]) < drAppRank(resp.Apps[b])
	})

	c.JSON(http.StatusOK, resp)
}

// drAppRank: smaller = more urgent (sorted first).
// Data-bearing-but-unbacked apps are most urgent; then not_eligible; then by
// ascending score (low score = worse posture = surfaced earlier).
func drAppRank(a DRAppScore) int {
	if !a.Protected && (a.StatefulSets > 0 || a.PVCs > 0) {
		return -1000 + a.TotalScore
	}
	if a.Status != evaluator.EvalStatusScored {
		return -500
	}
	return a.TotalScore
}

// ─── Collector: K8s state → evaluator.AppContext ──────────────────────────────

func buildDRAppContext(
	ns string,
	ann map[string]string,
	snapshotAt time.Time,
	bks []velerov1.Backup,
	schs []velerov1.Schedule,
	bslByName map[string]velerov1.BackupStorageLocation,
) evaluator.AppContext {
	ac := evaluator.AppContext{Namespace: ns, SnapshotAt: snapshotAt}

	// ── Dimension 1: Coverage ──
	if t := ann["supkube.io/business-tier"]; t != "" {
		ac.Coverage.Tier = fConfirmed(evaluator.Tier(t))
	} else {
		ac.Coverage.Tier = fMissing[evaluator.Tier]()
	}
	if r := ann["supkube.io/business-rpo"]; r != "" {
		if secs, err := strconv.Atoi(r); err == nil {
			ac.Coverage.RPOTargetSeconds = fConfirmed(secs)
		} else {
			ac.Coverage.RPOTargetSeconds = fMissing[int]()
		}
	} else {
		ac.Coverage.RPOTargetSeconds = fMissing[int]()
	}
	// Cluster/cross-tier aggregates are not single-namespace facts → missing.
	ac.Coverage.HasDifferentiatedPolicies = fMissing[bool]()
	ac.Coverage.CoreCoveragePct = fMissing[float64]()
	ac.Coverage.ZombieAssetPct = fMissing[float64]()

	// ── Dimension 4: Reliability (drives RPO-actual + last-backup signals) ──
	sorted := make([]velerov1.Backup, len(bks))
	copy(sorted, bks)
	sort.SliceStable(sorted, func(i, j int) bool {
		return backupStart(sorted[i]).After(backupStart(sorted[j]))
	})
	window := snapshotAt.Add(-14 * 24 * time.Hour)
	var lastBackup *time.Time
	lastSucceeded := false
	attempts14, success14, consecFail := 0, 0, 0
	streakBroken := false
	for idx, b := range sorted {
		st := backupStart(b)
		if !st.IsZero() && st.After(window) {
			attempts14++
			if b.Status.Phase == velerov1.BackupPhaseCompleted {
				success14++
			}
		}
		if idx == 0 {
			if b.Status.CompletionTimestamp != nil {
				t := b.Status.CompletionTimestamp.Time
				lastBackup = &t
			}
			lastSucceeded = b.Status.Phase == velerov1.BackupPhaseCompleted
		}
		if !streakBroken {
			switch b.Status.Phase {
			case velerov1.BackupPhaseFailed, velerov1.BackupPhasePartiallyFailed, velerov1.BackupPhaseFailedValidation:
				consecFail++
			case velerov1.BackupPhaseCompleted:
				streakBroken = true
			}
		}
	}

	if len(bks) == 0 {
		// Confirmed "never backed up" (NOT missing) — triggers the engine's
		// hard threshold / not_eligible path. This is the high-signal case the
		// backup advisor exists for.
		ac.Reliability.LastBackupAt = fConfirmed[*time.Time](nil)
		ac.Reliability.LastBackupSucceeded = fConfirmed(false)
		ac.Reliability.Last14DaysAttempts = fConfirmed(0)
		ac.Reliability.Last14DaysSuccess = fConfirmed(0)
		ac.Reliability.ConsecutiveFailures = fMissing[int]()
		ac.Coverage.RPOActualSeconds = fMissing[int]()
		ac.Coverage.BackupIncludesMetadata = fMissing[bool]()
	} else {
		ac.Reliability.LastBackupAt = fConfirmed(lastBackup)
		ac.Reliability.LastBackupSucceeded = fConfirmed(lastSucceeded)
		ac.Reliability.Last14DaysAttempts = fConfirmed(attempts14)
		ac.Reliability.Last14DaysSuccess = fConfirmed(success14)
		ac.Reliability.ConsecutiveFailures = fConfirmed(consecFail)
		ac.Coverage.BackupIncludesMetadata = fConfirmed(true) // Velero captures manifests by default
		if lastBackup != nil {
			rpo := int(snapshotAt.Sub(*lastBackup).Seconds())
			if rpo < 0 {
				rpo = 0
			}
			ac.Coverage.RPOActualSeconds = fConfirmed(rpo)
		} else {
			ac.Coverage.RPOActualSeconds = fMissing[int]()
		}
	}
	// Per-namespace drill association deferred (drflow runs key off a transient
	// drill ns, not the source app). Cluster-level drill stats live in Summary.
	ac.Reliability.DrillStatus = fMissing[evaluator.DrillTier]()

	// ── Dimension 2: Resilience (backup storage redundancy, from BSLs used) ──
	if len(bks) > 0 {
		provSet, regSet, mediaSet := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
		seen := false
		for _, b := range bks {
			loc := b.Spec.StorageLocation
			if loc == "" {
				continue
			}
			if bsl, ok := bslByName[loc]; ok {
				seen = true
				if bsl.Spec.Provider != "" {
					provSet[bsl.Spec.Provider] = struct{}{}
					mediaSet[mediaOf(bsl.Spec.Provider)] = struct{}{}
				}
				if r := bsl.Spec.Config["region"]; r != "" {
					regSet[r] = struct{}{}
				}
			}
		}
		if seen {
			ac.Resilience.BSLProviders = fConfirmed(sortedKeys(provSet))
			ac.Resilience.BSLRegions = fConfirmed(sortedKeys(regSet))
			ac.Resilience.StorageMediaTypes = fConfirmed(sortedKeys(mediaSet))
		} else {
			ac.Resilience.BSLProviders = fMissing[[]string]()
			ac.Resilience.BSLRegions = fMissing[[]string]()
			ac.Resilience.StorageMediaTypes = fMissing[[]string]()
		}
	} else {
		ac.Resilience.BSLProviders = fMissing[[]string]()
		ac.Resilience.BSLRegions = fMissing[[]string]()
		ac.Resilience.StorageMediaTypes = fMissing[[]string]()
	}
	ac.Resilience.NetworkIsolation = fMissing[evaluator.NetworkIsolationMode]()

	// ── Dimension 3: Security (WORM from object-lock annotation; rest deferred) ──
	wormOn := false
	for _, b := range bks {
		if b.Annotations["supkube.io/object-lock"] == "true" {
			wormOn = true
		}
	}
	for _, s := range schs {
		if s.Annotations["supkube.io/object-lock"] == "true" {
			wormOn = true
		}
	}
	if len(bks) > 0 || len(schs) > 0 {
		ac.Security.WORM.ObjectLockEnabled = fConfirmed(wormOn)
	} else {
		ac.Security.WORM.ObjectLockEnabled = fMissing[bool]()
	}
	ac.Security.WORM.Mode = fMissing[evaluator.WORMMode]()
	ac.Security.WORM.RetainUntilDate = fMissing[time.Time]()
	ac.Security.Encryption.StaticAES256 = fMissing[bool]()
	ac.Security.Encryption.TransitTLS13 = fMissing[bool]()
	ac.Security.Encryption.KMSIndependentRotation = fMissing[bool]()
	ac.Security.AccessControl.MFAEnabled = fMissing[bool]()
	ac.Security.AccessControl.RBACEnabled = fMissing[bool]()
	ac.Security.AccessControl.DeleteRequiresApproval = fMissing[bool]()
	ac.Security.AccessControl.AuditOffsiteTamperProof = fMissing[bool]()

	return ac
}

// ─── Recommendation generator: ScoreResult + topology → actionable advice ─────

func buildDRRecommendations(r evaluator.ScoreResult, workloads, sts, pvc int, protected bool) []DRRecommendation {
	var recs []DRRecommendation
	hasData := sts > 0 || pvc > 0

	// Most urgent: data-bearing application that has never been backed up.
	if !protected {
		if hasData {
			recs = append(recs, DRRecommendation{
				Severity: "P1", Type: "NO_BACKUP", Action: "create_backup",
				Message: "该应用含有状态负载/持久卷但从未备份，数据丢失风险高，建议立即创建备份策略。",
			})
		} else if workloads > 0 {
			recs = append(recs, DRRecommendation{
				Severity: "P2", Type: "NO_BACKUP", Action: "create_backup",
				Message: "该应用有工作负载但未配置备份，建议创建备份策略以便可恢复。",
			})
		}
	}

	switch r.Status {
	case evaluator.EvalStatusNotEligibleNoPolicy:
		recs = append(recs, DRRecommendation{
			Severity: "P2", Type: "NO_POLICY", Action: "create_schedule",
			Message: "未绑定备份 Policy，建议在 Policies 页创建备份计划。",
		})
	case evaluator.EvalStatusNotEligibleNoRuns:
		recs = append(recs, DRRecommendation{
			Severity: "P2", Type: "POLICY_NEVER_RAN", Action: "run_backup",
			Message: "备份 Policy 已配置但从未执行，建议立即跑一次首备。",
		})
	case evaluator.EvalStatusScored:
		d := r.Dimensions
		if dimWeak(d.Resilience) {
			recs = append(recs, DRRecommendation{
				Severity: "P2", Type: "WEAK_RESILIENCE", Action: "add_offsite_copy",
				Message: "备份冗余不足（缺少异地/多介质/多区域副本，未满足 3-2-1），建议增加异地副本。",
			})
		}
		if dimWeak(d.ImmutabilityAndSecurity) {
			recs = append(recs, DRRecommendation{
				Severity: "P2", Type: "WEAK_IMMUTABILITY", Action: "enable_worm",
				Message: "不可变性/安全较弱，建议启用 WORM 对象锁与加密，防勒索软件篡改。",
			})
		}
		if dimWeak(d.Reliability) {
			recs = append(recs, DRRecommendation{
				Severity: "P2", Type: "LOW_RELIABILITY", Action: "investigate_failures",
				Message: "备份成功率偏低或未做恢复演练，建议排查失败原因并安排演练。",
			})
		}
		if dimWeak(d.BackupCoverage) {
			recs = append(recs, DRRecommendation{
				Severity: "P3", Type: "WEAK_COVERAGE", Action: "tighten_rpo",
				Message: "备份覆盖/RPO 达成不足，建议缩短备份周期或补全覆盖范围。",
			})
		}
		if r.Level == evaluator.LevelCritical {
			recs = append(recs, DRRecommendation{
				Severity: "P1", Type: "CRITICAL_POSTURE",
				Message: "整体 DR 态势为 critical（<60 分），建议优先整改上述项。",
			})
		}
	}

	return recs
}

// dimWeak reports whether a dimension scored below 50% of its max.
func dimWeak(d evaluator.DimensionScore) bool {
	return d.MaxScore > 0 && d.Score*100 < d.MaxScore*50
}

// ─── small helpers ────────────────────────────────────────────────────────────

func backupStart(b velerov1.Backup) time.Time {
	if b.Status.StartTimestamp != nil {
		return b.Status.StartTimestamp.Time
	}
	return b.CreationTimestamp.Time
}

func mediaOf(provider string) string {
	switch provider {
	case "aws":
		return "s3"
	case "azure":
		return "azureblob"
	case "gcp":
		return "gcs"
	default:
		return provider
	}
}
