// Package v1: local_store.go — Local BSL provisioning status (v0.8.12).
//
// SupKube-managed in-cluster MinIO acts as the "fast tier" of every
// L1/L2 backup policy (see 架构设计.md ADR-029). The Helm chart deploys
// the MinIO Deployment + PVC + Service + BSL CR + bucket-init Job when
// values.yaml has localStore.enabled=true; this endpoint surfaces the
// resulting runtime state to the SPA so the Storage Locations page can
// render an accurate health card.
//
// We do NOT mutate K8s state from this handler — provisioning is owned by
// Helm. The frontend's "Enable Local Backup Store" button (v0.8.12-LBS3)
// will issue a `helm upgrade --set localStore.enabled=true` via a separate
// privileged plumbing layer, not via this read-only status API.
//
// RBAC: viewer-or-above (every Storage Locations page reader needs it).
package v1

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/supkube/supkube-backend/internal/k8s"
)

const (
	localStoreBSLName        = "supkube-local-store"
	localStoreDeploymentName = "supkube-local-store"
	localStoreServiceName    = "supkube-local-store"
	localStorePVCName        = "supkube-local-store-data"
)

// LocalStoreStatus is what /api/v1/local-store/status returns. Fields are
// deliberately shallow: the SPA can render a single card from this shape
// without traversing nested structures.
type LocalStoreStatus struct {
	// Enabled is true when the Helm chart has provisioned a local MinIO
	// Deployment (i.e. localStore.enabled=true was set). When false, the
	// rest of the fields are zero values — the UI should show a "Not
	// Enabled" empty state with an "Enable" CTA.
	Enabled bool `json:"enabled"`

	// Phase summarises the readiness of the entire stack:
	//   "Pending"      — Deployment exists but not Ready (helm upgrade
	//                    just ran, MinIO booting)
	//   "Ready"        — Deployment 1/1 AND BSL phase Available
	//   "Degraded"     — Deployment Ready but BSL not Available (e.g.
	//                    bucket-init Job hasn't run, credentials wrong)
	//   "Failed"       — Deployment has CrashLoopBackoff replica
	Phase string `json:"phase"`

	// Bucket / Endpoint mirror what's in the BSL CR; populated for the
	// UI to show a "kubectl port-forward" hint and copy-paste curl example.
	Bucket   string `json:"bucket,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`

	// ObjectLockEnabled + ObjectLockMode reflect the supkube.io/object-lock*
	// annotations on the BSL. Falls back to false / "" when annotations
	// are missing (legacy BSLs added by hand).
	ObjectLockEnabled bool   `json:"objectLockEnabled"`
	ObjectLockMode    string `json:"objectLockMode,omitempty"`

	// CapacityBytes / UsedBytes are best-effort. We query the PVC's spec
	// for Capacity (always available) but UsedBytes requires hitting MinIO
	// admin info — not implemented in v0.8.12-LBS1. Set to 0 for now; LBS3
	// will populate them via mc admin info.
	CapacityBytes int64 `json:"capacityBytes,omitempty"`
	UsedBytes     int64 `json:"usedBytes,omitempty"`

	// Message surfaces the most recent error from the BSL CR's status
	// (which Velero updates with validation errors). Empty when healthy.
	Message string `json:"message,omitempty"`
}

// GetLocalStoreStatus is GET /api/v1/local-store/status.
//
// Reads three sources and folds them into one response:
//   1. Velero BSL "supkube-local-store" — bucket, endpoint, phase, message
//   2. SupKube Deployment "supkube-local-store" — readiness
//   3. PVC "supkube-local-store-data" — capacity
//
// All three sources can be absent independently:
//   - BSL missing  → localStore.enabled was false at helm time
//   - Deploy ready, BSL missing → admin manually deleted the BSL
//   - Deploy missing, BSL present → admin manually scaled down
// We treat (BSL missing) as the canonical "not enabled" signal because
// that's what the Helm chart guards on.
func GetLocalStoreStatus(c *gin.Context) {
	// Two clients: typed (Deployment + PVC) and dynamic (BSL CRD).
	// Matches the established pattern in branding.go and policy.go —
	// see those for the same idiom.
	k8sCli, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "k8s client: " + err.Error()})
		return
	}
	dynCli, err := k8s.GetDynamicClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "dynamic client: " + err.Error()})
		return
	}

	// Velero namespace is hardcoded to "velero" across the codebase
	// (handlers.go does the same). When we eventually parameterize this,
	// it'll be one find/replace — for now consistency beats DRY.
	veleroNS := "velero"
	supkubeNS := "supkube"

	ctx := context.Background()
	resp := LocalStoreStatus{}

	// ---- Step 1: BSL --------------------------------------------------
	// Velero BSL is a CRD; we hit it via dynamic client to avoid an extra
	// scheme dependency. The shape we need (bucket, config.s3Url, phase,
	// message) is shallow enough that dynamic typing is fine here.
	bslGVR := schema.GroupVersionResource{
		Group:    "velero.io",
		Version:  "v1",
		Resource: "backupstoragelocations",
	}
	bsl, err := dynCli.Resource(bslGVR).Namespace(veleroNS).Get(ctx, localStoreBSLName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// BSL absent → feature disabled. Return enabled=false with no
			// further detail; UI shows the "Enable" empty state.
			c.JSON(http.StatusOK, resp)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read BSL: " + err.Error()})
		return
	}

	resp.Enabled = true

	// Pull bucket + endpoint from BSL spec. Errors here are odd (BSL
	// should always have these) but we tolerate them — the UI just shows
	// empty cells.
	if bucket, found, _ := nestedString(bsl.Object, "spec", "objectStorage", "bucket"); found {
		resp.Bucket = bucket
	}
	if s3url, found, _ := nestedString(bsl.Object, "spec", "config", "s3Url"); found {
		resp.Endpoint = s3url
	}

	// Phase + message from BSL status — Velero sets these as it validates.
	bslPhase, _, _ := nestedString(bsl.Object, "status", "phase")
	bslMessage, _, _ := nestedString(bsl.Object, "status", "message")

	// Object Lock metadata lives in our own annotations (Velero doesn't
	// know about them — they're a SupKube convention).
	if anns, found, _ := nestedMap(bsl.Object, "metadata", "annotations"); found {
		if v, ok := anns["supkube.io/object-lock"].(string); ok {
			resp.ObjectLockEnabled = v == "true"
		}
		if v, ok := anns["supkube.io/object-lock-mode"].(string); ok {
			resp.ObjectLockMode = v
		}
	}

	// ---- Step 2: Deployment ------------------------------------------
	// readiness signal: ReadyReplicas == 1 (single-binary mode).
	deployReady := false
	if dep, err := k8sCli.AppsV1().Deployments(supkubeNS).Get(ctx, localStoreDeploymentName, metav1.GetOptions{}); err == nil {
		deployReady = dep.Status.ReadyReplicas >= 1
	}

	// ---- Step 3: PVC capacity ----------------------------------------
	if pvc, err := k8sCli.CoreV1().PersistentVolumeClaims(supkubeNS).Get(ctx, localStorePVCName, metav1.GetOptions{}); err == nil {
		if cap, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			resp.CapacityBytes = cap.Value()
		} else if req, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			// PVC still Pending → fall back to requested size.
			resp.CapacityBytes = req.Value()
		}
	}

	// ---- Fold into Phase --------------------------------------------
	// Priority: BSL Available + Deploy Ready = Ready
	//           Deploy not Ready                = Pending
	//           Deploy Ready but BSL unavailable = Degraded
	//           Anything else                    = Failed
	switch {
	case deployReady && strings.EqualFold(bslPhase, "Available"):
		resp.Phase = "Ready"
	case !deployReady:
		resp.Phase = "Pending"
		if bslMessage != "" {
			resp.Message = bslMessage
		}
	case deployReady && !strings.EqualFold(bslPhase, "Available"):
		resp.Phase = "Degraded"
		resp.Message = bslMessage
	default:
		resp.Phase = "Failed"
		resp.Message = bslMessage
	}

	c.JSON(http.StatusOK, resp)
}

// nestedString / nestedMap: tiny helpers around unstructured.NestedString
// without pulling the full apimachinery/unstructured dependency. They
// return (value, found, err) matching that package's signature so callers
// can swap to it later without touching call sites.
func nestedString(obj map[string]interface{}, path ...string) (string, bool, error) {
	val, found, err := nestedField(obj, path...)
	if err != nil || !found {
		return "", found, err
	}
	s, ok := val.(string)
	if !ok {
		return "", false, nil
	}
	return s, true, nil
}

func nestedMap(obj map[string]interface{}, path ...string) (map[string]interface{}, bool, error) {
	val, found, err := nestedField(obj, path...)
	if err != nil || !found {
		return nil, found, err
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		return nil, false, nil
	}
	return m, true, nil
}

func nestedField(obj map[string]interface{}, path ...string) (interface{}, bool, error) {
	cur := interface{}(obj)
	for _, p := range path {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false, nil
		}
		next, found := m[p]
		if !found {
			return nil, false, nil
		}
		cur = next
	}
	return cur, true, nil
}
