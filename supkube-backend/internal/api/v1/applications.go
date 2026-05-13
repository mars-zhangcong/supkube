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

// ApplicationInfo represents a namespace with its workload and protection info
type ApplicationInfo struct {
	Namespace      string `json:"namespace"`
	Workloads      int    `json:"workloads"`
	Protected      bool   `json:"protected"`
	LastBackupTime string `json:"lastBackupTime,omitempty"`
	LastBackupName string `json:"lastBackupName,omitempty"`
}

// systemNamespaces that should be excluded from the applications list
var systemNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
	"velero":          true,
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

	// Build a map of namespace -> latest backup info
	type backupInfo struct {
		name string
		time time.Time
	}
	nsBackupMap := make(map[string]*backupInfo)

	for _, b := range backupList.Items {
		if b.Status.Phase != velerov1.BackupPhaseCompleted {
			continue
		}
		for _, ns := range b.Spec.IncludedNamespaces {
			existing, ok := nsBackupMap[ns]
			if !ok || b.CreationTimestamp.Time.After(existing.time) {
				nsBackupMap[ns] = &backupInfo{
					name: b.Name,
					time: b.CreationTimestamp.Time,
				}
			}
		}
		// If no included namespaces specified, backup covers all namespaces
		if len(b.Spec.IncludedNamespaces) == 0 {
			for _, nsItem := range nsList.Items {
				ns := nsItem.Name
				if systemNamespaces[ns] {
					continue
				}
				existing, ok := nsBackupMap[ns]
				if !ok || b.CreationTimestamp.Time.After(existing.time) {
					nsBackupMap[ns] = &backupInfo{
						name: b.Name,
						time: b.CreationTimestamp.Time,
					}
				}
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
		}

		if bi, ok := nsBackupMap[ns]; ok {
			app.Protected = true
			app.LastBackupName = bi.name
			app.LastBackupTime = bi.time.Format(time.RFC3339)
		}

		apps = append(apps, app)
	}

	c.JSON(http.StatusOK, gin.H{"items": apps, "total": len(apps)})
}
