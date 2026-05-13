package v1

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/supkube/supkube-backend/internal/k8s"
)

// ResourceItem represents a generic K8S resource
type ResourceItem struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Status    string `json:"status,omitempty"`
	Ready     string `json:"ready,omitempty"`
	Restarts  int32  `json:"restarts,omitempty"`
	Age       string `json:"age,omitempty"`
	Type      string `json:"type,omitempty"`
	ClusterIP string `json:"clusterIP,omitempty"`
	Ports     string `json:"ports,omitempty"`
	Capacity  string `json:"capacity,omitempty"`
	StorageClass string `json:"storageClass,omitempty"`
	UpToDate  int32  `json:"upToDate,omitempty"`
	Available int32  `json:"available,omitempty"`
}

// ApplicationDetail represents detailed resource info for a namespace
type ApplicationDetail struct {
	Namespace    string         `json:"namespace"`
	Pods         []ResourceItem `json:"pods"`
	Services     []ResourceItem `json:"services"`
	Deployments  []ResourceItem `json:"deployments"`
	StatefulSets []ResourceItem `json:"statefulSets"`
	ReplicaSets  []ResourceItem `json:"replicaSets"`
	PVCs         []ResourceItem `json:"pvcs"`
	ConfigMaps   []ResourceItem `json:"configMaps"`
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	if d.Hours() >= 24 {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
	if d.Hours() >= 1 {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	if d.Minutes() >= 1 {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// GetApplicationDetails returns detailed resource inventory for a namespace
func GetApplicationDetails(c *gin.Context) {
	ns := c.Param("namespace")

	k8sClient, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	detail := ApplicationDetail{Namespace: ns}

	// Get Pods
	pods, err := k8sClient.CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{})
	if err == nil {
		for _, p := range pods.Items {
			readyCount := int32(0)
			totalCount := int32(len(p.Spec.Containers))
			restarts := int32(0)
			for _, cs := range p.Status.ContainerStatuses {
				if cs.Ready {
					readyCount++
				}
				restarts += cs.RestartCount
			}
			detail.Pods = append(detail.Pods, ResourceItem{
				Kind:     "Pod",
				Name:     p.Name,
				Status:   string(p.Status.Phase),
				Ready:    fmt.Sprintf("%d/%d", readyCount, totalCount),
				Restarts: restarts,
				Age:      formatAge(p.CreationTimestamp.Time),
			})
		}
	}

	// Get Services
	services, err := k8sClient.CoreV1().Services(ns).List(context.Background(), metav1.ListOptions{})
	if err == nil {
		for _, s := range services.Items {
			ports := ""
			for i, p := range s.Spec.Ports {
				if i > 0 {
					ports += ", "
				}
				if p.NodePort > 0 {
					ports += fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol)
				} else {
					ports += fmt.Sprintf("%d/%s", p.Port, p.Protocol)
				}
			}
			detail.Services = append(detail.Services, ResourceItem{
				Kind:      "Service",
				Name:      s.Name,
				Type:      string(s.Spec.Type),
				ClusterIP: s.Spec.ClusterIP,
				Ports:     ports,
				Age:       formatAge(s.CreationTimestamp.Time),
			})
		}
	}

	// Get Deployments
	deployments, err := k8sClient.AppsV1().Deployments(ns).List(context.Background(), metav1.ListOptions{})
	if err == nil {
		for _, d := range deployments.Items {
			detail.Deployments = append(detail.Deployments, ResourceItem{
				Kind:      "Deployment",
				Name:      d.Name,
				Ready:     fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, *d.Spec.Replicas),
				UpToDate:  d.Status.UpdatedReplicas,
				Available: d.Status.AvailableReplicas,
				Age:       formatAge(d.CreationTimestamp.Time),
			})
		}
	}

	// Get StatefulSets
	statefulsets, err := k8sClient.AppsV1().StatefulSets(ns).List(context.Background(), metav1.ListOptions{})
	if err == nil {
		for _, s := range statefulsets.Items {
			detail.StatefulSets = append(detail.StatefulSets, ResourceItem{
				Kind:  "StatefulSet",
				Name:  s.Name,
				Ready: fmt.Sprintf("%d/%d", s.Status.ReadyReplicas, *s.Spec.Replicas),
				Age:   formatAge(s.CreationTimestamp.Time),
			})
		}
	}

	// Get ReplicaSets
	replicasets, err := k8sClient.AppsV1().ReplicaSets(ns).List(context.Background(), metav1.ListOptions{})
	if err == nil {
		for _, r := range replicasets.Items {
			if r.Status.Replicas > 0 { // Only show active ones
				detail.ReplicaSets = append(detail.ReplicaSets, ResourceItem{
					Kind:  "ReplicaSet",
					Name:  r.Name,
					Ready: fmt.Sprintf("%d/%d", r.Status.ReadyReplicas, *r.Spec.Replicas),
					Age:   formatAge(r.CreationTimestamp.Time),
				})
			}
		}
	}

	// Get PVCs
	pvcs, err := k8sClient.CoreV1().PersistentVolumeClaims(ns).List(context.Background(), metav1.ListOptions{})
	if err == nil {
		for _, pvc := range pvcs.Items {
			capacity := ""
			if qty, ok := pvc.Status.Capacity["storage"]; ok {
				capacity = qty.String()
			}
			sc := ""
			if pvc.Spec.StorageClassName != nil {
				sc = *pvc.Spec.StorageClassName
			}
			detail.PVCs = append(detail.PVCs, ResourceItem{
				Kind:         "PersistentVolumeClaim",
				Name:         pvc.Name,
				Status:       string(pvc.Status.Phase),
				Capacity:     capacity,
				StorageClass: sc,
				Age:          formatAge(pvc.CreationTimestamp.Time),
			})
		}
	}

	// Get ConfigMaps
	configmaps, err := k8sClient.CoreV1().ConfigMaps(ns).List(context.Background(), metav1.ListOptions{})
	if err == nil {
		for _, cm := range configmaps.Items {
			detail.ConfigMaps = append(detail.ConfigMaps, ResourceItem{
				Kind: "ConfigMap",
				Name: cm.Name,
				Age:  formatAge(cm.CreationTimestamp.Time),
			})
		}
	}

	c.JSON(http.StatusOK, detail)
}
