package v1

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/k8s"
)

// ApplicationInfo represents a namespace with its workload and protection info.
// ComplianceStatus is the derived state for the UI status badge; it is computed
// from Workloads count and the latest backup's phase. See computeCompliance().
type ApplicationInfo struct {
	Namespace        string            `json:"namespace"`
	Workloads        int               `json:"workloads"`
	Protected        bool              `json:"protected"`
	ComplianceStatus string            `json:"complianceStatus"`
	Labels           map[string]string `json:"labels,omitempty"`
	LastBackupTime   string            `json:"lastBackupTime,omitempty"`
	LastBackupName   string            `json:"lastBackupName,omitempty"`
	LastBackupPhase  string            `json:"lastBackupPhase,omitempty"`
}

// systemNamespaces that should be excluded from the applications list.
// We only hide the three K8S kernel-internal namespaces — operator/tooling
// namespaces like velero, supkube, minio are shown so users can see and
// protect them too. Hide individual namespaces with the supkube.io/exclude
// label if needed.
var systemNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
}

const excludeLabel = "supkube.io/exclude"

// computeCompliance derives the application's compliance status from workload
// count and the latest backup phase. Values mirror the UI badges.
//   - Empty:        no workloads in the namespace
//   - Unmanaged:    has workloads but no backup ever ran
//   - Compliant:    latest backup completed successfully
//   - NonCompliant: latest backup failed (or partially failed / validation failed)
//   - InProgress:   latest backup is still running (treated as transient)
func computeCompliance(workloads int, latestPhase string) string {
	if workloads == 0 {
		return "Empty"
	}
	switch latestPhase {
	case "":
		return "Unmanaged"
	case string(velerov1.BackupPhaseCompleted):
		return "Compliant"
	case string(velerov1.BackupPhaseFailed),
		string(velerov1.BackupPhasePartiallyFailed),
		string(velerov1.BackupPhaseFailedValidation):
		return "NonCompliant"
	default:
		return "InProgress"
	}
}

// ListApplications returns namespace-level application info with workload counts and protection status
func ListApplications(c *gin.Context) {
	k8sClient, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get all namespaces
	nsList, err := k8sClient.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get all backups to determine protection status
	runtimeClient, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	backupList := &velerov1.BackupList{}
	if err := runtimeClient.List(context.Background(), backupList, client.InNamespace("velero")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build a map of namespace -> latest backup info (any phase, not just Completed).
	// Tracking the latest phase lets the UI distinguish Compliant vs NonCompliant
	// when a backup failed instead of silently showing Unmanaged.
	type backupInfo struct {
		name  string
		time  time.Time
		phase string
	}
	nsBackupMap := make(map[string]*backupInfo)

	recordBackup := func(ns string, b velerov1.Backup) {
		existing, ok := nsBackupMap[ns]
		if !ok || b.CreationTimestamp.Time.After(existing.time) {
			nsBackupMap[ns] = &backupInfo{
				name:  b.Name,
				time:  b.CreationTimestamp.Time,
				phase: string(b.Status.Phase),
			}
		}
	}

	for _, b := range backupList.Items {
		for _, ns := range b.Spec.IncludedNamespaces {
			recordBackup(ns, b)
		}
		// If no included namespaces specified, backup covers all non-system namespaces
		if len(b.Spec.IncludedNamespaces) == 0 {
			for _, nsItem := range nsList.Items {
				if systemNamespaces[nsItem.Name] {
					continue
				}
				if nsItem.Labels[excludeLabel] == "true" {
					continue
				}
				recordBackup(nsItem.Name, b)
			}
		}
	}

	// Build application list
	apps := make([]ApplicationInfo, 0)
	for _, nsItem := range nsList.Items {
		ns := nsItem.Name
		if systemNamespaces[ns] {
			continue
		}
		if nsItem.Labels[excludeLabel] == "true" {
			continue
		}

		// Count workloads (deployments + statefulsets + daemonsets)
		workloadCount := 0

		deployments, err := k8sClient.AppsV1().Deployments(ns).List(context.Background(), metav1.ListOptions{})
		if err == nil {
			workloadCount += len(deployments.Items)
		}

		statefulsets, err := k8sClient.AppsV1().StatefulSets(ns).List(context.Background(), metav1.ListOptions{})
		if err == nil {
			workloadCount += len(statefulsets.Items)
		}

		daemonsets, err := k8sClient.AppsV1().DaemonSets(ns).List(context.Background(), metav1.ListOptions{})
		if err == nil {
			workloadCount += len(daemonsets.Items)
		}

		app := ApplicationInfo{
			Namespace: ns,
			Workloads: workloadCount,
			Labels:    nsItem.Labels,
		}

		latestPhase := ""
		if bi, ok := nsBackupMap[ns]; ok {
			app.LastBackupName = bi.name
			app.LastBackupTime = bi.time.Format(time.RFC3339)
			app.LastBackupPhase = bi.phase
			latestPhase = bi.phase
			// "Protected" historically meant "has at least one Completed backup";
			// keep that semantics for backwards compatibility with the Dashboard
			// summary stat, separate from the richer ComplianceStatus.
			if bi.phase == string(velerov1.BackupPhaseCompleted) {
				app.Protected = true
			}
		}
		app.ComplianceStatus = computeCompliance(workloadCount, latestPhase)

		apps = append(apps, app)
	}

	c.JSON(http.StatusOK, gin.H{"items": apps, "total": len(apps)})
}

// ----- Backup Advisor (v0.7.5 MVP) -------------------------------------
//
// "Should this app be backed up, and how often?" — answered from K8s API
// data alone (no Prometheus, no audit-log analysis; those land in v0.9).
//
// Scoring is a transparent linear sum of signals; tier is bucketed off the
// raw score. We deliberately keep the rules small and easy to read so a
// customer can sanity-check why a namespace landed in High vs Skip.
//
// Tier thresholds (matches ROADMAP v0.7.5 chapter):
//   >= 70  -> High  (every 6h, 30d retention)
//   40-69  -> Medium (daily, 60d)
//   10-39  -> Low   (weekly, 90d)
//   < 10   -> Skip Recommended

type AdvisorFactor struct {
	Reason string `json:"reason"`
	Delta  int    `json:"delta"` // signed contribution; negative = penalty
}

type AdvisorRecommendation struct {
	Namespace           string          `json:"namespace"`
	Score               int             `json:"score"`
	Tier                string          `json:"tier"`
	RecommendedSchedule string          `json:"recommendedSchedule"`
	RecommendedTTL      string          `json:"recommendedTTL"`
	Factors             []AdvisorFactor `json:"factors"`
	Workloads           int             `json:"workloads"`
	HasPVC              bool            `json:"hasPVC"`
	UserTier            string          `json:"userTier,omitempty"`
}

func classifyAdvisorTier(score int) (string, string, string) {
	switch {
	case score >= 70:
		return "High", "0 */6 * * *", "720h"
	case score >= 40:
		return "Medium", "0 0 * * *", "1440h"
	case score >= 10:
		return "Low", "0 0 * * 0", "2160h"
	default:
		return "Skip", "", ""
	}
}

// advisorSkippedNamespaces are namespaces the Advisor never scores — backing
// them up creates a circular dependency (you can't restore Velero with
// Velero, and SupKube backing up its own metadata storage is meaningless).
// User-facing namespaces like minio/restored-ns are still scored; users can
// label them `supkube.io/exclude=true` to opt out.
var advisorSkippedNamespaces = map[string]bool{
	"velero":  true,
	"supkube": true,
}

// GetBackupAdvisor returns recommendations for every non-system namespace.
func GetBackupAdvisor(c *gin.Context) {
	k8sClient, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	nsList, err := k8sClient.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	recs := make([]AdvisorRecommendation, 0)
	for _, nsItem := range nsList.Items {
		ns := nsItem.Name
		if systemNamespaces[ns] || advisorSkippedNamespaces[ns] {
			continue
		}
		if nsItem.Labels[excludeLabel] == "true" {
			continue
		}
		recs = append(recs, scoreNamespaceForAdvisor(k8sClient, ns, nsItem.Labels))
	}
	c.JSON(http.StatusOK, gin.H{"items": recs, "total": len(recs)})
}

// GetBackupAdvisorForNamespace evaluates a single namespace (used by the
// Apply Recommendation re-check flow).
func GetBackupAdvisorForNamespace(c *gin.Context) {
	ns := c.Param("namespace")
	k8sClient, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	nsObj, err := k8sClient.CoreV1().Namespaces().Get(context.Background(), ns, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, scoreNamespaceForAdvisor(k8sClient, ns, nsObj.Labels))
}

func scoreNamespaceForAdvisor(k8sClient kubernetes.Interface, ns string, nsLabels map[string]string) AdvisorRecommendation {
	rec := AdvisorRecommendation{Namespace: ns, Factors: []AdvisorFactor{}}

	deployments, _ := k8sClient.AppsV1().Deployments(ns).List(context.Background(), metav1.ListOptions{})
	statefulsets, _ := k8sClient.AppsV1().StatefulSets(ns).List(context.Background(), metav1.ListOptions{})
	daemonsets, _ := k8sClient.AppsV1().DaemonSets(ns).List(context.Background(), metav1.ListOptions{})
	pvcs, _ := k8sClient.CoreV1().PersistentVolumeClaims(ns).List(context.Background(), metav1.ListOptions{})
	services, _ := k8sClient.CoreV1().Services(ns).List(context.Background(), metav1.ListOptions{})

	wl := 0
	if deployments != nil {
		wl += len(deployments.Items)
	}
	if statefulsets != nil {
		wl += len(statefulsets.Items)
	}
	if daemonsets != nil {
		wl += len(daemonsets.Items)
	}
	rec.Workloads = wl

	if pvcs != nil && len(pvcs.Items) > 0 {
		rec.HasPVC = true
		rec.Score += 40
		rec.Factors = append(rec.Factors, AdvisorFactor{
			Reason: fmt.Sprintf("Has %d PersistentVolumeClaim(s) — stateful data lives here", len(pvcs.Items)),
			Delta:  40,
		})
	}
	if statefulsets != nil && len(statefulsets.Items) > 0 {
		rec.Score += 20
		rec.Factors = append(rec.Factors, AdvisorFactor{
			Reason: fmt.Sprintf("Has %d StatefulSet(s) — typically database or queue", len(statefulsets.Items)),
			Delta:  20,
		})
	}
	if tier := nsLabels["supkube.io/tier"]; tier != "" {
		rec.UserTier = tier
		var bonus int
		switch tier {
		case "core", "critical":
			bonus = 30
		case "business":
			bonus = 15
		case "dev", "test":
			bonus = -10
		}
		if bonus != 0 {
			rec.Score += bonus
			rec.Factors = append(rec.Factors, AdvisorFactor{
				Reason: fmt.Sprintf("User-marked tier %q", tier),
				Delta:  bonus,
			})
		}
	}
	if ns == "default" && wl > 0 {
		rec.Score += 10
		rec.Factors = append(rec.Factors, AdvisorFactor{
			Reason: "Workloads in default namespace are often ad-hoc — protect at least lightly",
			Delta:  10,
		})
	}
	// Penalty: stateless and not exposed by any Service -> easy to rebuild.
	if !rec.HasPVC && wl > 0 && services != nil && len(services.Items) == 0 {
		rec.Score -= 30
		rec.Factors = append(rec.Factors, AdvisorFactor{
			Reason: "Stateless workload with no Service — rebuildable from manifests",
			Delta:  -30,
		})
	}
	if wl == 0 && !rec.HasPVC {
		rec.Score -= 20
		rec.Factors = append(rec.Factors, AdvisorFactor{
			Reason: "Namespace currently empty",
			Delta:  -20,
		})
	}

	rec.Tier, rec.RecommendedSchedule, rec.RecommendedTTL = classifyAdvisorTier(rec.Score)
	return rec
}
