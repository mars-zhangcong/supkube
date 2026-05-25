// Package v1: resource_yaml.go — v0.8.10.3 lazy YAML fetch for the
// per-artifact </> icon in the Action/Application/Restore Point drawers.
//
// Why a separate endpoint vs embedding YAML in the breakdown response
// ─────────────────────────────────────────────────────────────────
// The artifact-breakdown response can list 30-50 items per drawer. If we
// embedded each item's full YAML up front, a single backup would push
// 100-500 KB across the wire for content the user views for AT MOST one
// item at a time. So we lazy-fetch: drawer renders the </> button, user
// clicks → one round trip for one resource, sub-100ms.
//
// Same GVR resolution as ListBackupArtifacts (storage.go) — dynamic
// client + a kind→GVR map for the kinds we know we'll see in our own
// breakdown. Unknown kinds return 400 with an explicit error so the
// frontend can show "this resource type isn't supported yet" rather
// than rendering broken HTML.
package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	sigyaml "sigs.k8s.io/yaml"

	"github.com/supkube/supkube-backend/internal/k8s"
)

// kindToGVR maps the Kind strings the breakdown endpoint emits onto
// the GVRs the dynamic client expects. Mirrors the GVR set in
// artifact_breakdown.go.breakdownGVRs — keeping these in sync is the
// only maintenance burden of the lazy-fetch approach.
var kindToGVR = map[string]schema.GroupVersionResource{
	// Workloads
	"Deployment":         {Group: "apps", Version: "v1", Resource: "deployments"},
	"StatefulSet":        {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"DaemonSet":          {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"ReplicaSet":         {Group: "apps", Version: "v1", Resource: "replicasets"},
	"ControllerRevision": {Group: "apps", Version: "v1", Resource: "controllerrevisions"},
	"Pod":                {Group: "", Version: "v1", Resource: "pods"},
	"Job":                {Group: "batch", Version: "v1", Resource: "jobs"},
	"CronJob":            {Group: "batch", Version: "v1", Resource: "cronjobs"},
	// Config
	"ConfigMap": {Group: "", Version: "v1", Resource: "configmaps"},
	"Secret":    {Group: "", Version: "v1", Resource: "secrets"},
	// Networking
	"Service":        {Group: "", Version: "v1", Resource: "services"},
	"Endpoints":      {Group: "", Version: "v1", Resource: "endpoints"},
	"EndpointSlice":  {Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"},
	"Ingress":        {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	"NetworkPolicy":  {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
	// Storage
	"PersistentVolumeClaim": {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	// RBAC
	"ServiceAccount": {Group: "", Version: "v1", Resource: "serviceaccounts"},
	"Role":           {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
	"RoleBinding":    {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
	// Autoscaling
	"HorizontalPodAutoscaler": {Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
	"ResourceQuota":           {Group: "", Version: "v1", Resource: "resourcequotas"},
	"LimitRange":              {Group: "", Version: "v1", Resource: "limitranges"},
	"PodDisruptionBudget":     {Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"},
	// CSI snapshot
	"VolumeSnapshot": {Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"},
	// Events — Velero captures these so we expose them via the drawer too
	"Event": {Group: "", Version: "v1", Resource: "events"},
}

// GetResourceYAML returns the YAML rendering of a single namespaced
// resource. Query params:
//
//	?namespace=<ns>     required
//	?kind=<Kind>        required, MUST match a key in kindToGVR
//	?name=<name>        required
//
// Output (200):
//
//	{ "kind": "...", "name": "...", "namespace": "...", "yaml": "..." }
//
// The YAML has server-side metadata stripped (resourceVersion, uid,
// managedFields, etc.) so what the user sees in the drawer matches
// what `kubectl apply` would recreate — useful for support tickets.
func GetResourceYAML(c *gin.Context) {
	ns := c.Query("namespace")
	kind := c.Query("kind")
	name := c.Query("name")
	if ns == "" || kind == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace, kind, name are all required"})
		return
	}
	gvr, ok := kindToGVR[kind]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "unsupported kind: " + kind + " (supported set is the same as /backups/:name/artifact-breakdown)",
		})
		return
	}

	dcl, err := k8s.GetDynamicClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	obj, err := dcl.Resource(gvr).Namespace(ns).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "resource not found in live cluster (may have been deleted since the backup was taken)"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Same field-stripping helper as ListBackupArtifacts uses, so the
	// rendered YAML is identical to the "Spec" preview in the Restore
	// drawer. One stripper, one source of truth.
	cleaned := stripServerFields(obj.Object)
	y, err := sigyaml.Marshal(cleaned)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "yaml marshal failed: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"kind":      kind,
		"name":      name,
		"namespace": ns,
		"yaml":      string(y),
	})
}
