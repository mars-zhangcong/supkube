package v1

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

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
	Namespace    string            `json:"namespace"`
	Labels       map[string]string `json:"labels"`
	Pods         []ResourceItem    `json:"pods"`
	Services     []ResourceItem    `json:"services"`
	Deployments  []ResourceItem    `json:"deployments"`
	StatefulSets []ResourceItem    `json:"statefulSets"`
	ReplicaSets  []ResourceItem    `json:"replicaSets"`
	PVCs         []ResourceItem    `json:"pvcs"`
	ConfigMaps   []ResourceItem    `json:"configMaps"`
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

	detail := ApplicationDetail{Namespace: ns, Labels: map[string]string{}}

	// Get namespace labels
	nsObj, err := k8sClient.CoreV1().Namespaces().Get(context.Background(), ns, metav1.GetOptions{})
	if err == nil && nsObj.Labels != nil {
		detail.Labels = nsObj.Labels
	}

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

// PVCCapability lists the storage classes used by a namespace's PVCs and
// whether each one supports CSI snapshots. Frontend uses this to gate
// Policy creation: if user picks "CSI" volume mode but the target ns has
// PVCs on a non-CSI-snapshot-capable SC, we surface a clear error instead
// of letting Velero fail at backup time.
type PVCCapability struct {
	PVC          string `json:"pvc"`
	StorageClass string `json:"storageClass"`
	Provisioner  string `json:"provisioner"`
	CSISnapshot  bool   `json:"csiSnapshot"`
	Reason       string `json:"reason,omitempty"`
}

type NamespaceStorageCapability struct {
	Namespace        string          `json:"namespace"`
	PVCs             []PVCCapability `json:"pvcs"`
	AllCSICapable    bool            `json:"allCSICapable"`
	IncompatibleCount int            `json:"incompatibleCount"`
}

// GetNamespaceStorageCapability returns the CSI-snapshot capability per PVC
// in the namespace. A SC is "CSI snapshot capable" iff:
//   1. Its provisioner is in the CSIDriver list (i.e. it's a CSI driver), AND
//   2. There exists a VolumeSnapshotClass whose .driver matches the provisioner
//
// Non-CSI provisioners (like docker.io/hostpath) always fail the check. SCs
// with no VSC available also fail (driver exists but snapshot not configured).
func GetNamespaceStorageCapability(c *gin.Context) {
	ns := c.Param("namespace")
	cap := NamespaceStorageCapability{Namespace: ns, AllCSICapable: true}

	k8sClient, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rtClient, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build CSI-driver set: which provisioners are real CSI drivers.
	csiDriverSet := map[string]bool{}
	csiList, err := k8sClient.StorageV1().CSIDrivers().List(context.Background(), metav1.ListOptions{})
	if err == nil {
		for _, d := range csiList.Items {
			csiDriverSet[d.Name] = true
		}
	}

	// Build set of drivers that have at least one VolumeSnapshotClass.
	// VSC is a CRD; use unstructured to avoid hard dep at compile time.
	snapDriverSet := map[string]bool{}
	vscGVR := schema.GroupVersionResource{
		Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotclasses",
	}
	dyn, dynErr := k8s.GetDynamicClient()
	if dynErr == nil {
		list, lerr := dyn.Resource(vscGVR).List(context.Background(), metav1.ListOptions{})
		if lerr == nil {
			for _, item := range list.Items {
				if driver, found, _ := unstructuredGet(item.Object, "driver"); found {
					if s, ok := driver.(string); ok {
						snapDriverSet[s] = true
					}
				}
			}
		} else if !apierrors.IsNotFound(lerr) {
			// CRDs not installed = no CSI snapshot anywhere; not an error.
		}
	}

	// Cache StorageClass -> provisioner so we only fetch each SC once.
	scCache := map[string]string{}

	pvcs, err := k8sClient.CoreV1().PersistentVolumeClaims(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, pvc := range pvcs.Items {
		sc := ""
		if pvc.Spec.StorageClassName != nil {
			sc = *pvc.Spec.StorageClassName
		}
		row := PVCCapability{PVC: pvc.Name, StorageClass: sc}

		if sc == "" {
			row.Reason = "PVC has no storageClassName"
			cap.PVCs = append(cap.PVCs, row)
			cap.AllCSICapable = false
			cap.IncompatibleCount++
			continue
		}

		provisioner, cached := scCache[sc]
		if !cached {
			scObj, gerr := k8sClient.StorageV1().StorageClasses().Get(context.Background(), sc, metav1.GetOptions{})
			if gerr == nil {
				provisioner = scObj.Provisioner
			}
			scCache[sc] = provisioner
		}
		row.Provisioner = provisioner

		// Use rtClient placeholder to silence import in case future use; keep API similar to other handlers
		_ = rtClient

		if !csiDriverSet[provisioner] {
			row.Reason = fmt.Sprintf("Provisioner %q is not a CSI driver", provisioner)
		} else if !snapDriverSet[provisioner] {
			row.Reason = fmt.Sprintf("No VolumeSnapshotClass configured for driver %q", provisioner)
		} else {
			row.CSISnapshot = true
		}
		if !row.CSISnapshot {
			cap.AllCSICapable = false
			cap.IncompatibleCount++
		}
		cap.PVCs = append(cap.PVCs, row)
	}

	c.JSON(http.StatusOK, cap)
}

// unstructuredGet is a tiny helper to read a nested string field from an
// unstructured map. Returns (value, found, error). Kept inline here to
// avoid pulling in the full unstructured library helpers.
func unstructuredGet(m map[string]interface{}, key string) (interface{}, bool, error) {
	v, ok := m[key]
	return v, ok, nil
}
