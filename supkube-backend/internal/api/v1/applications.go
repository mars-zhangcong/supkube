package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// Beyond this hardcoded set, any namespace carrying the label
// `supkube.io/exclude=true` is also hidden (see ListApplications).
var systemNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
	"velero":          true,
	"supkube":         true,
	"minio":           true,
	"restored-ns":     true,
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
