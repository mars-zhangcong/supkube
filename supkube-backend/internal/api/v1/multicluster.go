// Package v1: multicluster.go — Multi-Cluster Manager aggregate endpoint (v0.9.0.2).
//
// Powers the /multicluster dashboard page. Returns a single payload with
// per-cluster summaries + roll-up totals so the SPA renders in one round-
// trip. Per-cluster data is fetched in parallel goroutines with a 5s
// timeout each — a single unreachable cluster doesn't block the page.
//
// Why not extend GetClusters: that endpoint already serves the Settings →
// Clusters list which is meant to be fast (just the registry CRs + the
// status the health controller has cached). Fanning out to N remote APIs
// for every list call would balloon Settings load times. Multi-cluster
// dashboard is a heavier, opt-in page; deserves its own endpoint.
//
// Why not just have the SPA call N existing endpoints with X-Supkube-Cluster
// per cluster: works, but produces N parallel HTTP requests with the
// overhead of going through axios/nginx for each. One backend call with
// goroutine fan-out is ~3x faster on a 5-cluster fleet.
package v1

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/k8s"
)

const perClusterProbeTimeout = 5 * time.Second

// MultiClusterSummary is the entire payload the MCM page consumes.
type MultiClusterSummary struct {
	Clusters []ClusterSummary `json:"clusters"`
	Totals   MCTotals         `json:"totals"`
}

// ClusterSummary is one cluster's snapshot. Fields are picked to populate
// the cluster-row cards on the MCM dashboard without further drill-down.
type ClusterSummary struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Type        string `json:"type"`  // primary | secondary
	Phase       string `json:"phase"` // Healthy | Unreachable | Unauthorized | Unknown
	Message     string `json:"message,omitempty"`
	IsCurrent   bool   `json:"isCurrent"`
	K8sVersion  string `json:"k8sVersion,omitempty"`
	NodeCount   int    `json:"nodeCount,omitempty"`
	// Live-fetched per-cluster counts. Zero when probe failed.
	AppCount    int `json:"appCount"`
	PolicyCount int `json:"policyCount"`
	RPCount     int `json:"rpCount"`
	// LastBackupAt = most recent Backup CR's creationTimestamp on that
	// cluster. Empty when no backups exist (or probe failed).
	LastBackupAt string `json:"lastBackupAt,omitempty"`
	// VeleroInstalled comes from the Cluster CR status (set by the
	// health controller); included so the UI can disable the "Restore
	// to here" action for clusters without Velero.
	VeleroInstalled bool `json:"veleroInstalled"`
}

// MCTotals is the headline roll-up. Headers on the MCM page render straight
// from these — no client-side arithmetic.
type MCTotals struct {
	ClusterCount   int `json:"clusterCount"`
	AppCount       int `json:"appCount"`
	PolicyCount    int `json:"policyCount"`
	RPCount        int `json:"rpCount"`
	HealthyCount   int `json:"healthyCount"`
	UnhealthyCount int `json:"unhealthyCount"`
}

// GetMultiClusterSummary is GET /api/v1/multicluster/summary.
func GetMultiClusterSummary(c *gin.Context) {
	ctx := context.Background()

	// Step 1 — pull the cluster registry (Cluster CRs + the synthetic
	// "this-cluster" entry). We re-use the same DTO-builder used by
	// ListClusters so the row data is consistent across the two pages.
	dynCli, err := k8s.GetDynamicClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// "this-cluster" always first.
	bases := []ClusterDTO{buildThisClusterDTO(ctx)}

	list, err := dynCli.Resource(clusterGVR).Namespace(clustersNamespace).List(ctx, metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		// Fall through with just the local cluster — never 500 the whole
		// page because the CRD is missing.
		bases = append(bases, []ClusterDTO{}...)
	} else if list != nil {
		for i := range list.Items {
			bases = append(bases, clusterToDTO(&list.Items[i]))
		}
	}

	// Step 2 — fan out per-cluster probes. Goroutine per cluster, each
	// bounded by perClusterProbeTimeout. The slice index is preserved
	// via direct assignment (no append from concurrent goroutines).
	results := make([]ClusterSummary, len(bases))
	var wg sync.WaitGroup
	for i := range bases {
		i := i
		base := bases[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = probeClusterForSummary(ctx, base)
		}()
	}
	wg.Wait()

	// Step 3 — sort: this-cluster first, then alphabetical by name.
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].IsCurrent {
			return true
		}
		if results[j].IsCurrent {
			return false
		}
		return results[i].Name < results[j].Name
	})

	// Step 4 — totals
	totals := MCTotals{ClusterCount: len(results)}
	for _, r := range results {
		totals.AppCount += r.AppCount
		totals.PolicyCount += r.PolicyCount
		totals.RPCount += r.RPCount
		if r.Phase == "Healthy" {
			totals.HealthyCount++
		} else {
			totals.UnhealthyCount++
		}
	}

	c.JSON(http.StatusOK, MultiClusterSummary{Clusters: results, Totals: totals})
}

// probeClusterForSummary connects to one cluster and tallies the counts
// the MCM dashboard cards display. Defensive: any error → all-zero return
// with the original Phase preserved (so the UI shows a degraded chip).
func probeClusterForSummary(ctx context.Context, base ClusterDTO) ClusterSummary {
	out := ClusterSummary{
		Name:            base.Name,
		DisplayName:     base.DisplayName,
		Type:            base.Type,
		Phase:           base.Phase,
		Message:         base.Message,
		IsCurrent:       base.IsCurrent,
		K8sVersion:      base.K8sVersion,
		NodeCount:       base.NodeCount,
		VeleroInstalled: base.VeleroInstalled,
	}

	// Per-cluster timeout — we want the slowest cluster to drag the
	// whole response no more than perClusterProbeTimeout.
	probeCtx, cancel := context.WithTimeout(ctx, perClusterProbeTimeout)
	defer cancel()

	// Resolve the right Velero client (local for this-cluster, remote
	// otherwise). buildRemoteVeleroClient is the right helper — it
	// returns a controller-runtime client wired with the velero scheme.
	var cli client.Client
	var err error
	if base.IsCurrent || base.Name == "this-cluster" {
		cli, err = k8s.GetRuntimeClient()
	} else {
		cli, err = buildRemoteVeleroClient(probeCtx, base.Name)
	}
	if err != nil {
		// Mark unreachable but keep the row visible (degraded state).
		if out.Phase == "Healthy" {
			out.Phase = "Unreachable"
		}
		if out.Message == "" {
			out.Message = err.Error()
		}
		return out
	}

	// AppCount: namespaces minus the system ones. We use the same
	// exclusion list as the Applications page so totals match.
	if base.IsCurrent || base.Name == "this-cluster" {
		k8sCli, err := k8s.GetClient()
		if err == nil {
			if nsList, err := k8sCli.CoreV1().Namespaces().List(probeCtx, metav1.ListOptions{}); err == nil {
				for _, ns := range nsList.Items {
					if !isSystemNamespace(ns.Name) {
						out.AppCount++
					}
				}
			}
		}
	} else {
		// Remote path — typed client needed for Namespaces.List.
		if remoteK8s, err := buildRemoteTypedClient(probeCtx, base.Name); err == nil {
			if nsList, err := remoteK8s.CoreV1().Namespaces().List(probeCtx, metav1.ListOptions{}); err == nil {
				for _, ns := range nsList.Items {
					if !isSystemNamespace(ns.Name) {
						out.AppCount++
					}
				}
			}
		}
	}

	// PolicyCount: Velero Schedules. We don't de-duplicate dual-half
	// pairs here — the MCM page is a count summary, not an authoritative
	// policy list. (The Policies page already de-dups for its own table.)
	scheduleList := &velerov1.ScheduleList{}
	if err := cli.List(probeCtx, scheduleList, client.InNamespace("velero")); err == nil {
		out.PolicyCount = len(scheduleList.Items)
	}

	// RPCount + LastBackupAt: Velero Backup CRs.
	backupList := &velerov1.BackupList{}
	if err := cli.List(probeCtx, backupList, client.InNamespace("velero")); err == nil {
		out.RPCount = len(backupList.Items)
		var latest time.Time
		for i := range backupList.Items {
			if t := backupList.Items[i].CreationTimestamp.Time; t.After(latest) {
				latest = t
			}
		}
		if !latest.IsZero() {
			out.LastBackupAt = latest.Format(time.RFC3339)
		}
	}

	return out
}

// isSystemNamespace mirrors the exclusion list on the Applications page —
// keep these in lockstep so the MCM "apps" total matches what the user
// sees on the per-cluster Applications page.
func isSystemNamespace(name string) bool {
	switch name {
	case "kube-system", "kube-public", "kube-node-lease",
		"velero", "supkube", "minio", "default":
		return true
	}
	return false
}
