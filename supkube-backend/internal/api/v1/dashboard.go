package v1

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/supkube/supkube-backend/internal/velerons"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Dashboard summary cache
var (
	summaryCache     *DashboardSummary
	summaryCacheMu   sync.RWMutex
	summaryCacheTime time.Time
	summaryCacheTTL  = 30 * time.Second
)

// DashboardSummary holds aggregated cluster data
type DashboardSummary struct {
	Cluster          ClusterInfo        `json:"cluster"`
	BackupSummary    BackupSummaryInfo  `json:"backupSummary"`
	RecentBackups    []RecentBackupInfo `json:"recentBackups"`
	StorageLocations int                `json:"storageLocations"`
}

type ClusterInfo struct {
	Nodes      int `json:"nodes"`
	Namespaces int `json:"namespaces"`
}

type BackupSummaryInfo struct {
	Total      int `json:"total"`
	Completed  int `json:"completed"`
	Failed     int `json:"failed"`
	InProgress int `json:"inProgress"`
	Deleting   int `json:"deleting"`
}

type RecentBackupInfo struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Phase       string `json:"phase"`
	CreatedAt   string `json:"createdAt"`
	CompletedAt string `json:"completedAt,omitempty"`
}

// GetDashboardSummary returns aggregated dashboard data with 30s cache
func GetDashboardSummary(c *gin.Context) {
	// Check cache
	summaryCacheMu.RLock()
	if summaryCache != nil && time.Since(summaryCacheTime) < summaryCacheTTL {
		cached := *summaryCache
		summaryCacheMu.RUnlock()
		c.JSON(http.StatusOK, cached)
		return
	}
	summaryCacheMu.RUnlock()

	// Build fresh summary
	summary := &DashboardSummary{}

	// Get cluster info (nodes + namespaces).
	// v0.9.0.1 fix #3: route via the per-request helper so the Mode
	// Switcher's selection actually changes what the user sees.
	k8sClient, err := getRequestKubernetesClient(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	nodeList, err := k8sClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err == nil {
		summary.Cluster.Nodes = len(nodeList.Items)
	}

	nsList, err := k8sClient.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err == nil {
		summary.Cluster.Namespaces = len(nsList.Items)
	}

	// Get backup summary (also remote-routed).
	runtimeClient, err := getRequestRuntimeClient(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	backupList := &velerov1.BackupList{}
	if err := runtimeClient.List(context.Background(), backupList, client.InNamespace(velerons.Namespace())); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	summary.BackupSummary.Total = len(backupList.Items)
	for _, b := range backupList.Items {
		if b.DeletionTimestamp != nil {
			summary.BackupSummary.Deleting++
		}
		switch b.Status.Phase {
		case velerov1.BackupPhaseCompleted:
			summary.BackupSummary.Completed++
		case velerov1.BackupPhaseFailed, velerov1.BackupPhasePartiallyFailed:
			summary.BackupSummary.Failed++
		case velerov1.BackupPhaseInProgress:
			summary.BackupSummary.InProgress++
		}
	}

	// Recent backups (last 10, sorted by creation time desc)
	sort.Slice(backupList.Items, func(i, j int) bool {
		return backupList.Items[i].CreationTimestamp.After(backupList.Items[j].CreationTimestamp.Time)
	})
	limit := 10
	if len(backupList.Items) < limit {
		limit = len(backupList.Items)
	}
	summary.RecentBackups = make([]RecentBackupInfo, 0, limit)
	for i := 0; i < limit; i++ {
		b := backupList.Items[i]
		info := RecentBackupInfo{
			Name:      b.Name,
			Namespace: b.Namespace,
			Phase:     string(b.Status.Phase),
			CreatedAt: b.CreationTimestamp.Format(time.RFC3339),
		}
		if b.Status.CompletionTimestamp != nil {
			info.CompletedAt = b.Status.CompletionTimestamp.Format(time.RFC3339)
		}
		summary.RecentBackups = append(summary.RecentBackups, info)
	}

	// Storage locations count
	bslList := &velerov1.BackupStorageLocationList{}
	if err := runtimeClient.List(context.Background(), bslList, client.InNamespace(velerons.Namespace())); err == nil {
		summary.StorageLocations = len(bslList.Items)
	}

	// Update cache
	summaryCacheMu.Lock()
	summaryCache = summary
	summaryCacheTime = time.Now()
	summaryCacheMu.Unlock()

	c.JSON(http.StatusOK, summary)
}

// InvalidateDashboardCache clears the dashboard cache (call after write operations)
func InvalidateDashboardCache() {
	summaryCacheMu.Lock()
	summaryCache = nil
	summaryCacheMu.Unlock()
}
