// Package v1: restores_preflight.go
//
// Pre-flight conflict detector for Velero Restores (v0.7.12).
//
// Why this exists
// ───────────────
// Cross-namespace / cross-cluster Restores hit *structural* conflicts —
// not edge cases. Things like Service NodePorts, clusterIPs, Ingress hosts,
// PV bindings, and StorageClass names are global-uniqueness fields. When
// Velero tries to restore them verbatim into a new namespace (or onto a
// different cluster), the K8s API refuses, the Restore ends up
// PartiallyFailed, and the user has to dig through download URLs to find
// out *why*.
//
// Pre-flight Check shifts that diagnosis BEFORE the restore. We read the
// backup's artifact list, scan the target cluster for collisions, and
// report each one with a suggested Velero ResourceModifier patch the user
// can apply (v0.8 turns those into a one-click "Apply Suggested Fix"
// button — for v0.7.12 they're read-only so the user at least has the
// exact patch to copy/paste).
//
// Conflict taxonomy
// ─────────────────
// See USER_MANUAL.md §4.2 for the full 10-row matrix. The kinds detected
// here map 1:1 to that table (image-registry probe is opt-in via
// `deepCheck:true` because DNS resolution is slow).
package v1

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/auth"
	"github.com/supkube/supkube-backend/internal/k8s"
	"github.com/supkube/supkube-backend/internal/velerons"
)

// ─── Conflict severity ──────────────────────────────────────────────────
const (
	SeverityBlocker = "blocker" // restore WILL fail unless fixed
	SeverityWarning = "warning" // restore may partially succeed but degraded
	SeverityInfo    = "info"
)

// ─── ConflictKind enum (stable identifiers) ─────────────────────────────
// UI maps these to localized strings via i18n keys
// `restoreDrawer.conflictKind.<value>`. NEVER change a value once shipped
// (only ever add new ones), so historical Pre-flight responses keep
// rendering correctly when the user opens an older drawer.
const (
	ConflictNodePortCollision        = "NodePortCollision"
	ConflictClusterIPCollision       = "ClusterIPCollision"
	ConflictLoadBalancerIPCollision  = "LoadBalancerIPCollision"
	ConflictIngressHostCollision     = "IngressHostCollision"
	ConflictPVBinding                = "PVBinding"
	ConflictStorageClassMissing      = "StorageClassMissing"
	ConflictImageRegistryUnreachable = "ImageRegistryUnreachable"
	ConflictServiceAccountMissing    = "ServiceAccountMissing"
	ConflictNodeSelectorMissing      = "NodeSelectorMissing"
	ConflictPriorityClassMissing     = "PriorityClassMissing"
)

// ─── Data types ─────────────────────────────────────────────────────────

// ArtifactRef points to the backup resource being analyzed.
type ArtifactRef struct {
	Kind      string `json:"kind"`
	Group     string `json:"group,omitempty"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// TransformRule is the suggested patch a Pre-flight Conflict carries. v0.8.4
// supports three variants — only one is populated per Conflict:
//
//  1. JSON Patch (Operation+Path+Value)
//     Best for: replace/add operations where the path is known to exist
//     Used by: IngressHostCollision (replace rule[N].host)
//
//  2. JSON Merge Patch (MergePatch)
//     Best for: "remove if exists" — silently no-ops when the field
//     isn't in the backup tarball (Velero strips many fields at backup
//     time, so JSON Patch `remove` errored out at restore in v0.8.3).
//     Used by: ClusterIPCollision, LoadBalancerIPCollision, PVBinding
//
//  3. Strategic Merge Patch (StrategicPatch)
//     Best for: in-place updates to array elements (e.g. one specific
//     Service port among many). Uses K8s strategic merge semantics —
//     ports are merged by `port`, containers by `name`, etc.
//     Used by: NodePortCollision (with port value for the merge key)
type TransformRule struct {
	// JSON Patch (RFC 6902)
	Operation string      `json:"operation,omitempty"`
	Path      string      `json:"path,omitempty"`
	Value     interface{} `json:"value,omitempty"`

	// JSON Merge Patch (RFC 7396). String form of the JSON.
	MergePatch string `json:"mergePatch,omitempty"`

	// Strategic Merge Patch. String form of the JSON.
	StrategicPatch string `json:"strategicPatch,omitempty"`
}

// Conflict describes a single detected conflict between the backup and
// the target cluster/namespace.
//
// PRD-001 v2 (#104, finding #2 — 2026-05-31): severity is固化 as an enum
// (`blocker` | `warning`) decided by the BACKEND. The frontend MUST treat
// it as read-only and MUST NOT compute or override severity client-side;
// otherwise cross-version drift and security-bypass risks creep in. The
// `matchingTransformSets` slice is likewise authoritative server-side —
// the backend matches the conflict against the TransformSet library and
// hands the names to the UI, which only renders them.
type Conflict struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	// MatchingTransformSets lists names of TransformSets known to address
	// this exact conflict kind+payload. Used by the RestoreDrawer
	// Checklist to render a "→ Go to Transform" CTA per conflict row.
	// Empty slice (not nil) when no match — encoded as `[]` so the
	// frontend can iterate without a null-guard.
	MatchingTransformSets []string       `json:"matchingTransformSets"`
	Artifact              ArtifactRef    `json:"artifact"`
	Field                 string         `json:"field"`
	CurrentValue          interface{}    `json:"currentValue,omitempty"`
	ConflictsWith         string         `json:"conflictsWith,omitempty"`
	Message               string         `json:"message"`
	SuggestedTransform    *TransformRule `json:"suggestedTransform,omitempty"`
}

// PreflightRequest is the body for POST /api/v1/restores/preflight.
type PreflightRequest struct {
	BackupName      string `json:"backupName" binding:"required"`
	TargetNamespace string `json:"targetNamespace" binding:"required"`
	// When true, we exclude resources in TargetNamespace from collision checks
	// because those resources will be deleted before the restore runs (see
	// CreateRestore's CleanupBeforeRestore flow).
	CleanupBeforeRestore bool `json:"cleanupBeforeRestore"`
	// When true, run slow checks (image registry DNS resolution).
	DeepCheck bool `json:"deepCheck"`
}

// PreflightResponse is the JSON returned to the UI.
type PreflightResponse struct {
	BackupName      string     `json:"backupName"`
	TargetNamespace string     `json:"targetNamespace"`
	Conflicts       []Conflict `json:"conflicts"`
	Checks          []string   `json:"checks"`           // detectors that ran
	Errors          []string   `json:"errors,omitempty"` // detectors that failed (non-blocking)
}

// ─── Entry point ────────────────────────────────────────────────────────

// PreflightRestore runs every applicable detector and returns the
// aggregated conflict list. It's a POST (not GET) because:
//   - the request body is non-trivial (targetNamespace, flags)
//   - it triggers external side effects (DNS lookups in deep mode)
//   - clients should not cache the response (cluster state changes)
func PreflightRestore(c *gin.Context) {
	var req PreflightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// v0.8.5 step 3: editor must scope-cover the target ns. Source ns
	// access is checked separately when CreateRestore actually fires.
	if !auth.RequireNamespaceAccess(c, req.TargetNamespace) {
		return
	}

	rcl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	backup := &velerov1.Backup{}
	if err := rcl.Get(context.Background(), client.ObjectKey{Name: req.BackupName, Namespace: velerons.Namespace()}, backup); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Pull artifacts as unstructured objects (not YAML strings — we'll
	// inspect fields directly).
	artifacts, err := loadBackupArtifactObjects(context.Background(), backup)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load artifacts: " + err.Error()})
		return
	}

	resp := PreflightResponse{
		BackupName:      req.BackupName,
		TargetNamespace: req.TargetNamespace,
		Conflicts:       []Conflict{},
		Checks:          []string{},
	}

	// Each detector is independent and best-effort. If one fails (e.g.
	// RBAC denies listing Ingresses), we record the error and continue
	// instead of failing the whole Pre-flight.
	run := func(name string, fn func() ([]Conflict, error)) {
		conflicts, err := fn()
		resp.Checks = append(resp.Checks, name)
		if err != nil {
			resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %s", name, err.Error()))
			return
		}
		// PRD-001 v2 (#104): normalize each conflict so the wire shape
		// matches the schema the frontend renders against:
		//   - severity defaults to "blocker" if a detector forgot to set
		//     it (conservative — forces admin to acknowledge unknown
		//     conflicts rather than silently ignoring)
		//   - matchingTransformSets is an empty slice (not nil) so the
		//     frontend can iterate without a null-guard, and JSON
		//     marshals to `[]` not `null`
		// TransformSet library matching itself lands in a follow-up
		// (PRD-002 §4.4) — for now the slice is always empty here.
		for i := range conflicts {
			if conflicts[i].Severity == "" {
				conflicts[i].Severity = SeverityBlocker
			}
			if conflicts[i].MatchingTransformSets == nil {
				conflicts[i].MatchingTransformSets = []string{}
			}
		}
		resp.Conflicts = append(resp.Conflicts, conflicts...)
	}

	run("service-collisions", func() ([]Conflict, error) {
		return detectServiceCollisions(context.Background(), artifacts, req.TargetNamespace, req.CleanupBeforeRestore)
	})
	run("storage-class-missing", func() ([]Conflict, error) {
		return detectStorageClassMissing(context.Background(), artifacts)
	})
	run("ingress-host-collisions", func() ([]Conflict, error) {
		return detectIngressHostCollisions(context.Background(), artifacts, req.TargetNamespace, req.CleanupBeforeRestore)
	})
	run("pv-binding", func() ([]Conflict, error) {
		return detectPVBinding(artifacts)
	})

	if req.DeepCheck {
		run("image-registry", func() ([]Conflict, error) {
			return detectImageRegistryReachability(artifacts)
		})
	}

	c.JSON(http.StatusOK, resp)
}

// ─── Artifact loader (shared with ListBackupArtifacts) ──────────────────

// loadBackupArtifactObjects returns the same set of unstructured objects
// that ListBackupArtifacts marshals to YAML, but without the YAML step.
// We use this for Pre-flight detectors that need to read typed fields.
//
// Limitation (v0.7.12): same as ListBackupArtifacts — we query the live
// cluster as a stand-in for the backup tarball, so the artifact set is
// approximate. Good enough when restoring shortly after a backup; v0.8
// will switch to BackupStore.GetBackupContents() for accuracy.
func loadBackupArtifactObjects(ctx context.Context, backup *velerov1.Backup) ([]unstructured.Unstructured, error) {
	namespaces := backup.Spec.IncludedNamespaces
	if len(namespaces) == 0 {
		return nil, fmt.Errorf("backup has no includedNamespaces; whole-cluster backups not supported in v0.7.12 Pre-flight")
	}

	dcl, err := k8s.GetDynamicClient()
	if err != nil {
		return nil, err
	}

	listOpts := metav1.ListOptions{}
	if backup.Spec.LabelSelector != nil {
		listOpts.LabelSelector = metav1.FormatLabelSelector(backup.Spec.LabelSelector)
	}

	var out []unstructured.Unstructured
	for _, ns := range namespaces {
		for _, gvr := range restoreArtifactGVRs {
			list, err := dcl.Resource(gvr).Namespace(ns).List(ctx, listOpts)
			if err != nil {
				if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
					continue
				}
				continue
			}
			out = append(out, list.Items...)
		}
	}
	return out, nil
}

// ─── Helpers ────────────────────────────────────────────────────────────

func gvrFromObj(obj *unstructured.Unstructured) schema.GroupVersionResource {
	gvk := obj.GroupVersionKind()
	return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: strings.ToLower(gvk.Kind) + "s"}
}

func artifactRefFromObj(obj *unstructured.Unstructured) ArtifactRef {
	gvk := obj.GroupVersionKind()
	return ArtifactRef{
		Kind:      gvk.Kind,
		Group:     gvk.Group,
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

// ─── Detector 1: Service NodePort / clusterIP / loadBalancerIP ──────────

// detectServiceCollisions checks every Service in the backup against the
// target cluster's existing Services. It excludes Services that live in
// targetNs if cleanupBeforeRestore=true (because those will be deleted
// before the restore creates the new ones, so no real conflict).
func detectServiceCollisions(ctx context.Context, artifacts []unstructured.Unstructured, targetNs string, cleanupBeforeRestore bool) ([]Conflict, error) {
	cl, err := k8s.GetClient()
	if err != nil {
		return nil, err
	}
	allSvcs, err := cl.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Build allocation maps: nodePort → "ns/name", clusterIP → "ns/name",
	// loadBalancerIP → "ns/name".
	nodePortHolder := map[int32]string{}
	clusterIPHolder := map[string]string{}
	lbIPHolder := map[string]string{}
	for _, s := range allSvcs.Items {
		// Resources in target ns that will be cleaned up before restore
		// shouldn't count as occupants.
		if cleanupBeforeRestore && s.Namespace == targetNs {
			continue
		}
		owner := s.Namespace + "/" + s.Name
		for _, p := range s.Spec.Ports {
			if p.NodePort > 0 {
				nodePortHolder[p.NodePort] = owner
			}
		}
		if s.Spec.ClusterIP != "" && s.Spec.ClusterIP != "None" {
			clusterIPHolder[s.Spec.ClusterIP] = owner
		}
		if s.Spec.LoadBalancerIP != "" {
			lbIPHolder[s.Spec.LoadBalancerIP] = owner
		}
	}

	var conflicts []Conflict
	for i := range artifacts {
		obj := artifacts[i]
		if obj.GetKind() != "Service" {
			continue
		}
		// Pull Service fields out of unstructured.
		ports, _, _ := unstructured.NestedSlice(obj.Object, "spec", "ports")
		for idx, p := range ports {
			pm, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			npRaw, found := pm["nodePort"]
			if !found {
				continue
			}
			np, ok := npRaw.(int64)
			if !ok {
				// JSON unmarshaling sometimes leaves this as float64
				if f, ok2 := npRaw.(float64); ok2 {
					np = int64(f)
				} else {
					continue
				}
			}
			if np == 0 {
				continue
			}
			if holder, taken := nodePortHolder[int32(np)]; taken {
				// Self-reference check: if the only holder is the same
				// (ns/name) as the artifact being restored, it's not a
				// real conflict (Velero will skip/update in place).
				artifactOwner := obj.GetNamespace() + "/" + obj.GetName()
				if holder == artifactOwner && obj.GetNamespace() == targetNs {
					continue
				}
				// v0.8.4: use a strategic merge patch that matches the
				// port by its `port` (or `name`) field — K8s knows the
				// merge key for Service ports. This is array-element
				// safe in a way that JSON Patch with /ports/N index
				// (brittle if port order differs) and Merge Patch
				// (replaces the whole array) aren't.
				portValue := pm["port"]
				portName, _ := pm["name"].(string)
				strategic := buildStrategicNodePortPatch(portValue, portName)
				conflicts = append(conflicts, Conflict{
					Kind:          ConflictNodePortCollision,
					Severity:      SeverityBlocker,
					Artifact:      artifactRefFromObj(&obj),
					Field:         fmt.Sprintf("/spec/ports/%d/nodePort", idx),
					CurrentValue:  np,
					ConflictsWith: holder,
					Message:       fmt.Sprintf("NodePort %d is already allocated by Service %s. K8s will reject the restored Service unless the field is stripped or changed.", np, holder),
					SuggestedTransform: &TransformRule{
						StrategicPatch: strategic,
					},
				})
			}
		}
		// clusterIP — v0.8.4: merge patch (silent no-op if Velero already
		// stripped it during backup, which it often does).
		if cip, _, _ := unstructured.NestedString(obj.Object, "spec", "clusterIP"); cip != "" && cip != "None" {
			if holder, taken := clusterIPHolder[cip]; taken {
				artifactOwner := obj.GetNamespace() + "/" + obj.GetName()
				if !(holder == artifactOwner && obj.GetNamespace() == targetNs) {
					conflicts = append(conflicts, Conflict{
						Kind:          ConflictClusterIPCollision,
						Severity:      SeverityBlocker,
						Artifact:      artifactRefFromObj(&obj),
						Field:         "/spec/clusterIP",
						CurrentValue:  cip,
						ConflictsWith: holder,
						Message:       fmt.Sprintf("clusterIP %s is already allocated by Service %s. K8s assigns clusterIPs from a pool; the field should be removed so the restored Service gets a fresh one.", cip, holder),
						SuggestedTransform: &TransformRule{
							MergePatch: `{"spec":{"clusterIP":null,"clusterIPs":null}}`,
						},
					})
				}
			}
		}
		// loadBalancerIP — v0.8.4: merge patch.
		if lb, _, _ := unstructured.NestedString(obj.Object, "spec", "loadBalancerIP"); lb != "" {
			if holder, taken := lbIPHolder[lb]; taken {
				artifactOwner := obj.GetNamespace() + "/" + obj.GetName()
				if !(holder == artifactOwner && obj.GetNamespace() == targetNs) {
					conflicts = append(conflicts, Conflict{
						Kind:          ConflictLoadBalancerIPCollision,
						Severity:      SeverityBlocker,
						Artifact:      artifactRefFromObj(&obj),
						Field:         "/spec/loadBalancerIP",
						CurrentValue:  lb,
						ConflictsWith: holder,
						Message:       fmt.Sprintf("loadBalancerIP %s is already claimed by Service %s. Strip the field to let the cloud provider allocate a new one.", lb, holder),
						SuggestedTransform: &TransformRule{
							MergePatch: `{"spec":{"loadBalancerIP":null}}`,
						},
					})
				}
			}
		}
	}
	return conflicts, nil
}

// buildStrategicNodePortPatch returns the strategic merge patch JSON that
// nulls out the nodePort of ONE specific Service port. Prefers `port`
// number as the merge key (K8s default for Service.spec.ports); falls back
// to `name` if port is unknown.
func buildStrategicNodePortPatch(portValue interface{}, portName string) string {
	// Normalize port value into a JSON-serializable number.
	var portJSON string
	switch v := portValue.(type) {
	case int64:
		portJSON = fmt.Sprintf("%d", v)
	case float64:
		portJSON = fmt.Sprintf("%d", int64(v))
	case int:
		portJSON = fmt.Sprintf("%d", v)
	default:
		portJSON = ""
	}
	if portJSON != "" {
		return fmt.Sprintf(`{"spec":{"ports":[{"port":%s,"nodePort":null}]}}`, portJSON)
	}
	if portName != "" {
		return fmt.Sprintf(`{"spec":{"ports":[{"name":%q,"nodePort":null}]}}`, portName)
	}
	// Last resort: strip from first port. Velero applies as strategic
	// merge which doesn't have an unambiguous match key here, so this
	// path is best-avoided but at least not a crash.
	return `{"spec":{"ports":[{"nodePort":null}]}}`
}

// ─── Detector 2: StorageClass missing ───────────────────────────────────

func detectStorageClassMissing(ctx context.Context, artifacts []unstructured.Unstructured) ([]Conflict, error) {
	cl, err := k8s.GetClient()
	if err != nil {
		return nil, err
	}
	scList, err := cl.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	existing := map[string]struct{}{}
	for _, sc := range scList.Items {
		existing[sc.Name] = struct{}{}
	}

	var conflicts []Conflict
	for i := range artifacts {
		obj := artifacts[i]
		if obj.GetKind() != "PersistentVolumeClaim" {
			continue
		}
		sc, _, _ := unstructured.NestedString(obj.Object, "spec", "storageClassName")
		if sc == "" {
			continue // default SC will be used
		}
		if _, ok := existing[sc]; !ok {
			conflicts = append(conflicts, Conflict{
				Kind:          ConflictStorageClassMissing,
				Severity:      SeverityBlocker,
				Artifact:      artifactRefFromObj(&obj),
				Field:         "/spec/storageClassName",
				CurrentValue:  sc,
				ConflictsWith: "(no such StorageClass in target cluster)",
				Message:       fmt.Sprintf("StorageClass %q does not exist on the target cluster. The PVC will fail to bind unless the field is replaced with an equivalent SC name or removed (to use the cluster default).", sc),
				SuggestedTransform: &TransformRule{
					Operation: "replace",
					Path:      "/spec/storageClassName",
					Value:     "<target-cluster-equivalent-sc>", // user fills in
				},
			})
		}
	}
	return conflicts, nil
}

// ─── Detector 3: Ingress host collisions ────────────────────────────────

func detectIngressHostCollisions(ctx context.Context, artifacts []unstructured.Unstructured, targetNs string, cleanupBeforeRestore bool) ([]Conflict, error) {
	cl, err := k8s.GetClient()
	if err != nil {
		return nil, err
	}
	ingList, err := cl.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	// host → "ns/name" of the Ingress already holding it.
	hostHolder := map[string]string{}
	for _, ing := range ingList.Items {
		if cleanupBeforeRestore && ing.Namespace == targetNs {
			continue
		}
		owner := ing.Namespace + "/" + ing.Name
		for _, rule := range ing.Spec.Rules {
			if rule.Host == "" {
				continue
			}
			hostHolder[rule.Host] = owner
		}
	}

	var conflicts []Conflict
	for i := range artifacts {
		obj := artifacts[i]
		if obj.GetKind() != "Ingress" {
			continue
		}
		rules, _, _ := unstructured.NestedSlice(obj.Object, "spec", "rules")
		for idx, r := range rules {
			rm, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			host, _ := rm["host"].(string)
			if host == "" {
				continue
			}
			if holder, taken := hostHolder[host]; taken {
				artifactOwner := obj.GetNamespace() + "/" + obj.GetName()
				if holder == artifactOwner && obj.GetNamespace() == targetNs {
					continue
				}
				conflicts = append(conflicts, Conflict{
					Kind:          ConflictIngressHostCollision,
					Severity:      SeverityBlocker,
					Artifact:      artifactRefFromObj(&obj),
					Field:         fmt.Sprintf("/spec/rules/%d/host", idx),
					CurrentValue:  host,
					ConflictsWith: holder,
					Message:       fmt.Sprintf("Ingress host %q is already in use by %s. Most Ingress controllers reject duplicate hosts. Rewrite the host (e.g. prefix with the target namespace) to keep both routable.", host, holder),
					SuggestedTransform: &TransformRule{
						Operation: "replace",
						Path:      fmt.Sprintf("/spec/rules/%d/host", idx),
						Value:     fmt.Sprintf("%s-%s", targetNs, host),
					},
				})
			}
		}
	}
	return conflicts, nil
}

// ─── Detector 4: PV binding (spec.volumeName set) ───────────────────────

func detectPVBinding(artifacts []unstructured.Unstructured) ([]Conflict, error) {
	var conflicts []Conflict
	for i := range artifacts {
		obj := artifacts[i]
		if obj.GetKind() != "PersistentVolumeClaim" {
			continue
		}
		vn, _, _ := unstructured.NestedString(obj.Object, "spec", "volumeName")
		if vn == "" {
			continue
		}
		conflicts = append(conflicts, Conflict{
			Kind:          ConflictPVBinding,
			Severity:      SeverityWarning,
			Artifact:      artifactRefFromObj(&obj),
			Field:         "/spec/volumeName",
			CurrentValue:  vn,
			ConflictsWith: "(PV is already bound or absent in the target cluster)",
			Message:       fmt.Sprintf("PVC pins .spec.volumeName=%q. The bound PV either no longer exists in this cluster or is bound to the original PVC. Strip the field so PVC dynamically binds to a fresh PV.", vn),
			// v0.8.4: merge patch — Velero strips volumeName at backup
			// time for PVCs, so JSON Patch `remove` would error at
			// restore on a key that's already gone. Merge patch with
			// null is a no-op when the field is missing.
			SuggestedTransform: &TransformRule{
				MergePatch: `{"spec":{"volumeName":null}}`,
			},
		})
	}
	return conflicts, nil
}

// ─── Detector 5: Image registry reachability (deep, opt-in) ─────────────

// detectImageRegistryReachability resolves the hostname of every container
// image and reports unreachable registries as warnings. Bounded to one
// DNS lookup per unique host so 50 containers from the same registry only
// cost 1 lookup. Capped at 8s total wall time so a hung DNS doesn't stall
// the whole Pre-flight.
func detectImageRegistryReachability(artifacts []unstructured.Unstructured) ([]Conflict, error) {
	// Extract unique hosts from container images.
	hosts := map[string][]ArtifactRef{} // host → artifacts that reference it
	for i := range artifacts {
		obj := artifacts[i]
		var containers [][]interface{}
		// Pods, Deployments, StatefulSets, DaemonSets, Jobs, CronJobs all
		// have containers somewhere under template.spec; collect both
		// containers and initContainers.
		switch obj.GetKind() {
		case "Pod":
			c1, _, _ := unstructured.NestedSlice(obj.Object, "spec", "containers")
			c2, _, _ := unstructured.NestedSlice(obj.Object, "spec", "initContainers")
			containers = append(containers, c1, c2)
		case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet":
			c1, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
			c2, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "initContainers")
			containers = append(containers, c1, c2)
		case "Job":
			c1, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
			c2, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "initContainers")
			containers = append(containers, c1, c2)
		case "CronJob":
			c1, _, _ := unstructured.NestedSlice(obj.Object, "spec", "jobTemplate", "spec", "template", "spec", "containers")
			c2, _, _ := unstructured.NestedSlice(obj.Object, "spec", "jobTemplate", "spec", "template", "spec", "initContainers")
			containers = append(containers, c1, c2)
		default:
			continue
		}
		for _, list := range containers {
			for _, c := range list {
				cm, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				img, _ := cm["image"].(string)
				host := registryHost(img)
				if host == "" {
					continue
				}
				hosts[host] = append(hosts[host], artifactRefFromObj(&obj))
			}
		}
	}

	if len(hosts) == 0 {
		return nil, nil
	}

	// DNS-resolve each unique host with a tight per-lookup timeout.
	resolver := &net.Resolver{}
	dialCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	var conflicts []Conflict
	for host, refs := range hosts {
		ctx, cancelOne := context.WithTimeout(dialCtx, 2*time.Second)
		_, err := resolver.LookupHost(ctx, host)
		cancelOne()
		if err == nil {
			continue
		}
		// Deduplicate refs (one host may be used by many containers).
		seen := map[string]struct{}{}
		var first ArtifactRef
		count := 0
		for _, r := range refs {
			key := r.Namespace + "/" + r.Kind + "/" + r.Name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			if count == 0 {
				first = r
			}
			count++
		}
		msg := fmt.Sprintf("Image registry %q is not resolvable from this cluster (%v). %d workload(s) reference it; the target pods will likely stay in ImagePullBackOff until the registry name is rewritten or DNS is fixed.", host, err, count)
		conflicts = append(conflicts, Conflict{
			Kind:          ConflictImageRegistryUnreachable,
			Severity:      SeverityWarning,
			Artifact:      first,
			Field:         "/spec/.../containers[*]/image",
			CurrentValue:  host,
			ConflictsWith: "(no DNS record from inside the cluster)",
			Message:       msg,
			SuggestedTransform: &TransformRule{
				Operation: "replace",
				Path:      "/spec/template/spec/containers/0/image",
				Value:     "<target-registry-host>/<path>",
			},
		})
	}
	return conflicts, nil
}

// registryHost extracts the hostname portion of an image reference. Returns
// "" for canonical-form images that target the default Docker Hub
// (e.g. "nginx:latest" or "library/postgres:15") since those need no
// registry rewriting in 99% of cases.
//
// Examples:
//
//	"nginx:latest"                              → ""
//	"library/postgres:15"                       → ""
//	"docker.io/library/postgres:15"             → "docker.io"
//	"quay.io/jetstack/cert-manager:v1.12.0"     → "quay.io"
//	"registry.example.com:5000/app/web:abc"     → "registry.example.com:5000"
//	"internal.corp/team/app@sha256:..."         → "internal.corp"
func registryHost(image string) string {
	if image == "" {
		return ""
	}
	// Strip tag/digest first.
	if idx := strings.IndexAny(image, "@"); idx > 0 {
		image = image[:idx]
	}
	// Find the first "/". The portion before it is a registry only if it
	// contains a "." or ":", otherwise it's just an org name on Docker Hub.
	slash := strings.Index(image, "/")
	if slash < 0 {
		return "" // single segment → Docker Hub
	}
	candidate := image[:slash]
	if !strings.ContainsAny(candidate, ".:") && candidate != "localhost" {
		return "" // org/name → Docker Hub
	}
	// Drop port for DNS lookup if present.
	if colon := strings.Index(candidate, ":"); colon > 0 {
		return candidate[:colon]
	}
	return candidate
}
