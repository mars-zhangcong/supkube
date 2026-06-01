// Package v1: clusters.go — Multi-Cluster Manager registry endpoints (v0.9.0 MC1).
//
// Manages clusters.supkube.io CRs. Each Cluster CR represents a remote
// Kubernetes cluster SupKube can dispatch backup/restore operations to.
// The kubeconfig is stored separately in a K8s Secret in the supkube
// namespace; the CR carries only a reference.
//
// API surface:
//
//	GET    /api/v1/clusters             list all
//	GET    /api/v1/clusters/:name       get one (with status)
//	POST   /api/v1/clusters             create (with kubeconfig)
//	DELETE /api/v1/clusters/:name       remove (also deletes secret)
//	POST   /api/v1/clusters/:name/test  one-shot connection check
//
// All endpoints are admin-only — registering a new cluster gives
// SupKube the ability to dispatch into it, which is a cluster-admin-
// level operation by definition (the kubeconfig the customer uploads
// has whatever scope they configured).
//
// MC2 (health check controller) lives in internal/clusterhealth/ and
// poll-updates status.phase every 60s. This file is read/write CRUD only.
package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/supkube/supkube-backend/internal/k8s"
)

const (
	clustersNamespace    = "supkube"
	clustersSecretPrefix = "cluster-kubeconfig-"
	clusterSecretKey     = "kubeconfig"
)

var clusterGVR = schema.GroupVersionResource{
	Group:    "supkube.io",
	Version:  "v1",
	Resource: "clusters",
}

// Cluster name must be DNS-1123 like — Velero / k8s names everywhere
// are constrained the same way and the SPA's URL `?cluster=X` would
// break with anything else.
var clusterNameRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

// ClusterDTO is what the SPA receives. Flatter than the raw CR so the
// list table renders without traversing nested objects.
type ClusterDTO struct {
	Name            string    `json:"name"`
	DisplayName     string    `json:"displayName,omitempty"`
	Type            string    `json:"type"`
	Description     string    `json:"description,omitempty"`
	Context         string    `json:"context,omitempty"`
	Phase           string    `json:"phase"`
	Message         string    `json:"message,omitempty"`
	LastChecked     time.Time `json:"lastChecked,omitempty"`
	K8sVersion      string    `json:"k8sVersion,omitempty"`
	NodeCount       int       `json:"nodeCount,omitempty"`
	Capabilities    []string  `json:"capabilities,omitempty"`
	VeleroInstalled bool      `json:"veleroInstalled"`
	VeleroVersion   string    `json:"veleroVersion,omitempty"`
	CreationTime    time.Time `json:"creationTime,omitempty"`
	IsCurrent       bool      `json:"isCurrent"`
}

// CreateClusterRequest is the SPA's POST body. We accept the kubeconfig
// as a raw string (the SPA reads the file contents and ships them
// verbatim) — base64 would just add UI complexity for zero security
// benefit (HTTPS already protects in transit).
type CreateClusterRequest struct {
	Name        string `json:"name" binding:"required"`
	DisplayName string `json:"displayName,omitempty"`
	Type        string `json:"type,omitempty"` // primary | secondary; default secondary
	Description string `json:"description,omitempty"`
	Kubeconfig  string `json:"kubeconfig" binding:"required"`
	Context     string `json:"context,omitempty"` // empty = use current-context from kubeconfig
}

// ─────────────────────────────────────────────────────────────────────
// List
// ─────────────────────────────────────────────────────────────────────

func ListClusters(c *gin.Context) {
	dynCli, err := k8s.GetDynamicClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	list, err := dynCli.Resource(clusterGVR).Namespace(clustersNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		// CRD not installed yet (fresh cluster pre-v0.9.0 helm upgrade)
		// → return empty list rather than 500 so the SPA doesn't blow
		// up on first load.
		if apierrors.IsNotFound(err) || strings.Contains(err.Error(), "no matches for kind") {
			c.JSON(http.StatusOK, gin.H{"items": []ClusterDTO{}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// v0.9.0.1 fix #4 — always prepend the local cluster as "this-cluster".
	// Previously we only synthesised when items was empty; the moment the
	// admin added their first secondary the local cluster disappeared from
	// the list, breaking the Mode Switcher's "where am I right now" answer.
	// Detection: live Discovery + Nodes count against the in-cluster
	// kubeconfig — same data the MC2 health controller uses for remotes.
	items := make([]ClusterDTO, 0, len(list.Items)+1)
	items = append(items, buildThisClusterDTO(context.Background()))
	for i := range list.Items {
		items = append(items, clusterToDTO(&list.Items[i]))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// buildThisClusterDTO assembles the synthetic "this-cluster" row that
// always sits at the top of the registry. Fail-safe: any probe error
// degrades to "Unknown" phase rather than 500'ing the whole list.
//
// v0.9.x DisplayName 改进 (Mars 2026-05-31 testing 20260531 #1 反馈):
//
//	旧版 DisplayName 写死 "This Cluster" — 在 DR Topology / MCM 列表里看到
//	"This-Cluster" 字眼, 客户的真问题是 "我现在到底在哪个集群?" 死写没回答。
//	新逻辑: 显示**真实可识别的标识**, 按优先级 fallback:
//	  1. control-plane node 名 (e.g. "docker-desktop", "ip-10-0-1-23")
//	  2. 任意第一个 node 名 (managed K8s 像 EKS 拿不到 control-plane)
//	  3. 兜底 "Local Cluster" (无 node? 不太可能但 fail-safe)
//	`Name` (sentinel "this-cluster") 不动, 仍是 cluster_id 用于 routing
//	header (request_client.go L52); 只改 `DisplayName` (UI 渲染).
func buildThisClusterDTO(ctx context.Context) ClusterDTO {
	dto := ClusterDTO{
		Name:         "this-cluster",  // sentinel, 不可改 (routing / header / API)
		DisplayName:  "Local Cluster", // 默认兜底, 下面再 detect override
		Type:         "primary",
		Phase:        "Healthy",
		IsCurrent:    true,
		CreationTime: time.Now(),
	}
	k8sCli, err := k8s.GetClient()
	if err != nil {
		dto.Phase = "Unknown"
		return dto
	}
	if ver, err := k8sCli.Discovery().ServerVersion(); err == nil && ver != nil {
		dto.K8sVersion = ver.GitVersion
	}
	if nl, err := k8sCli.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
		dto.NodeCount = len(nl.Items)
		// Pick a human-recognizable cluster name from node names.
		// Priority: control-plane label → any node → fall back to default.
		var cpName, anyName string
		for _, n := range nl.Items {
			if anyName == "" {
				anyName = n.Name
			}
			// K8s 1.20+ uses node-role.kubernetes.io/control-plane;
			// older clusters used .../master. Accept either.
			if _, ok := n.Labels["node-role.kubernetes.io/control-plane"]; ok {
				cpName = n.Name
				break
			}
			if _, ok := n.Labels["node-role.kubernetes.io/master"]; ok {
				cpName = n.Name
				break
			}
		}
		switch {
		case cpName != "":
			dto.DisplayName = cpName
		case anyName != "":
			dto.DisplayName = anyName
			// (无 node 时保留 "Local Cluster" 默认)
		}
	}
	if _, err := k8sCli.AppsV1().Deployments("velero").Get(ctx, "velero", metav1.GetOptions{}); err == nil {
		dto.VeleroInstalled = true
	}
	return dto
}

// ─────────────────────────────────────────────────────────────────────
// Get
// ─────────────────────────────────────────────────────────────────────

func GetCluster(c *gin.Context) {
	name := c.Param("name")
	if name == "this-cluster" {
		// Synthetic entry — return the same DTO that List would build for
		// "this-cluster" (含 control-plane node name detection, 2026-05-31).
		c.JSON(http.StatusOK, buildThisClusterDTO(context.Background()))
		return
	}
	dynCli, err := k8s.GetDynamicClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	cr, err := dynCli.Resource(clusterGVR).Namespace(clustersNamespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, clusterToDTO(cr))
}

// ─────────────────────────────────────────────────────────────────────
// Create
// ─────────────────────────────────────────────────────────────────────

func CreateCluster(c *gin.Context) {
	var req CreateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Validate name
	if !clusterNameRe.MatchString(req.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name must be DNS-1123 (lowercase letters, digits, '-', '.')"})
		return
	}
	if req.Name == "this-cluster" || req.Name == "_mcm" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name 'this-cluster' and '_mcm' are reserved"})
		return
	}
	if req.Type == "" {
		req.Type = "secondary"
	}
	if req.Type != "primary" && req.Type != "secondary" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'primary' or 'secondary'"})
		return
	}

	// Validate kubeconfig parses
	if _, err := clientcmd.Load([]byte(req.Kubeconfig)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid kubeconfig: " + err.Error()})
		return
	}

	k8sCli, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dynCli, err := k8s.GetDynamicClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx := context.Background()

	// Step 1: write the kubeconfig to a Secret
	secretName := clustersSecretPrefix + req.Name
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: clustersNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "supkube",
				"supkube.io/cluster":           req.Name,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			clusterSecretKey: []byte(req.Kubeconfig),
		},
	}
	if _, err := k8sCli.CoreV1().Secrets(clustersNamespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "cluster '" + req.Name + "' already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write kubeconfig secret: " + err.Error()})
		return
	}

	// Step 2: create the Cluster CR
	cr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "supkube.io/v1",
			"kind":       "Cluster",
			"metadata": map[string]interface{}{
				"name":      req.Name,
				"namespace": clustersNamespace,
				"labels": map[string]interface{}{
					"supkube.io/cluster-type": req.Type,
				},
			},
			"spec": map[string]interface{}{
				"displayName": req.DisplayName,
				"type":        req.Type,
				"description": req.Description,
				"context":     req.Context,
				"kubeconfigSecretRef": map[string]interface{}{
					"name": secretName,
					"key":  clusterSecretKey,
				},
			},
		},
	}
	created, err := dynCli.Resource(clusterGVR).Namespace(clustersNamespace).Create(ctx, cr, metav1.CreateOptions{})
	if err != nil {
		// Rollback: if CR creation failed, delete the orphan Secret so a
		// retry can re-create both cleanly.
		_ = k8sCli.CoreV1().Secrets(clustersNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create cluster CR: " + err.Error()})
		return
	}

	// Step 3: kick off initial health check inline so the SPA gets a
	// useful phase back immediately (rather than waiting up to 60s for
	// the controller to first scan).
	phase, msg, version, nodes, vinstalled, vversion := testClusterConnection([]byte(req.Kubeconfig), req.Context)
	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"phase":       phase,
			"message":     msg,
			"lastChecked": time.Now().UTC().Format(time.RFC3339),
			"k8sVersion":  version,
			"nodeCount":   nodes,
			"velero": map[string]interface{}{
				"installed": vinstalled,
				"version":   vversion,
			},
		},
	}
	patchBytes, _ := jsonMarshal(patch)
	_, _ = dynCli.Resource(clusterGVR).Namespace(clustersNamespace).Patch(ctx, req.Name, "application/merge-patch+json", patchBytes, metav1.PatchOptions{}, "status")

	c.JSON(http.StatusCreated, clusterToDTO(created))
}

// ─────────────────────────────────────────────────────────────────────
// Delete
// ─────────────────────────────────────────────────────────────────────

func DeleteCluster(c *gin.Context) {
	name := c.Param("name")
	if name == "this-cluster" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete the primary cluster"})
		return
	}
	dynCli, err := k8s.GetDynamicClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	k8sCli, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx := context.Background()

	// Delete CR first (idempotent — 404 is fine).
	if err := dynCli.Resource(clusterGVR).Namespace(clustersNamespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Then cleanup the Secret. Independent of CR delete success so an
	// orphan Secret from a half-failed Create can still be reaped by
	// repeating Delete on the cluster name.
	if err := k8sCli.CoreV1().Secrets(clustersNamespace).Delete(ctx, clustersSecretPrefix+name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		// Log + continue — the CR is already gone; the orphan Secret is
		// a tidiness issue not a correctness one.
		fmt.Println("[clusters] orphan secret cleanup:", err)
	}
	c.JSON(http.StatusNoContent, nil)
}

// ─────────────────────────────────────────────────────────────────────
// Test Connection (one-shot)
// ─────────────────────────────────────────────────────────────────────

type TestRequest struct {
	// If kubeconfig is set, test against THAT (used by the Add Cluster
	// wizard's "Test" button before submitting). Otherwise resolve from
	// the existing Cluster CR's Secret.
	Kubeconfig string `json:"kubeconfig,omitempty"`
	Context    string `json:"context,omitempty"`
}

func TestClusterConnection(c *gin.Context) {
	var req TestRequest
	_ = c.ShouldBindJSON(&req)
	name := c.Param("name")

	var kc []byte
	if req.Kubeconfig != "" {
		kc = []byte(req.Kubeconfig)
	} else if name != "" {
		// Resolve from CR's referenced Secret.
		k8sCli, _ := k8s.GetClient()
		dynCli, _ := k8s.GetDynamicClient()
		cr, err := dynCli.Resource(clusterGVR).Namespace(clustersNamespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
			return
		}
		secretName, _, _ := nestedString(cr.Object, "spec", "kubeconfigSecretRef", "name")
		s, err := k8sCli.CoreV1().Secrets(clustersNamespace).Get(context.Background(), secretName, metav1.GetOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read secret: " + err.Error()})
			return
		}
		kc = s.Data[clusterSecretKey]
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kubeconfig or :name required"})
		return
	}

	phase, msg, version, nodes, vinstalled, vversion := testClusterConnection(kc, req.Context)
	c.JSON(http.StatusOK, gin.H{
		"phase":           phase,
		"message":         msg,
		"k8sVersion":      version,
		"nodeCount":       nodes,
		"veleroInstalled": vinstalled,
		"veleroVersion":   vversion,
	})
}

// ─────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────

// testClusterConnection dials the cluster, runs discovery + a node count
// + a velero deployment probe. Returns derived fields used by both the
// inline call from Create and the standalone POST .../test endpoint.
func testClusterConnection(kubeconfig []byte, contextName string) (phase, message, version string, nodes int, veleroInstalled bool, veleroVersion string) {
	cfg, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return "Unauthorized", "kubeconfig parse error: " + err.Error(), "", 0, false, ""
	}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	rest, err := clientcmd.NewDefaultClientConfig(*cfg, overrides).ClientConfig()
	if err != nil {
		return "Unauthorized", "build rest config: " + err.Error(), "", 0, false, ""
	}
	// Tight timeout so a wedged probe doesn't hold the user's "Test" click.
	rest.Timeout = 8 * time.Second

	cli, err := remoteClient(rest)
	if err != nil {
		return "Unreachable", "create client: " + err.Error(), "", 0, false, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	srv, err := cli.Discovery().ServerVersion()
	if err != nil {
		return "Unreachable", "discovery: " + err.Error(), "", 0, false, ""
	}
	version = srv.GitVersion

	if nl, err := cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
		nodes = len(nl.Items)
	}

	if d, err := cli.AppsV1().Deployments("velero").Get(ctx, "velero", metav1.GetOptions{}); err == nil {
		veleroInstalled = true
		// Velero deployments label the image tag onto the container; best-effort.
		if len(d.Spec.Template.Spec.Containers) > 0 {
			img := d.Spec.Template.Spec.Containers[0].Image
			if idx := strings.LastIndex(img, ":"); idx > 0 {
				veleroVersion = img[idx+1:]
			}
		}
	}

	return "Healthy", "", version, nodes, veleroInstalled, veleroVersion
}

// clusterToDTO flattens the unstructured CR into the SPA-facing shape.
func clusterToDTO(u *unstructured.Unstructured) ClusterDTO {
	dto := ClusterDTO{Name: u.GetName(), Phase: "Unknown", CreationTime: u.GetCreationTimestamp().Time}
	if v, ok, _ := nestedString(u.Object, "spec", "displayName"); ok {
		dto.DisplayName = v
	}
	if v, ok, _ := nestedString(u.Object, "spec", "type"); ok {
		dto.Type = v
	}
	if v, ok, _ := nestedString(u.Object, "spec", "description"); ok {
		dto.Description = v
	}
	if v, ok, _ := nestedString(u.Object, "spec", "context"); ok {
		dto.Context = v
	}
	if v, ok, _ := nestedString(u.Object, "status", "phase"); ok {
		dto.Phase = v
	}
	if v, ok, _ := nestedString(u.Object, "status", "message"); ok {
		dto.Message = v
	}
	if v, ok, _ := nestedString(u.Object, "status", "k8sVersion"); ok {
		dto.K8sVersion = v
	}
	if v, ok, _ := nestedString(u.Object, "status", "lastChecked"); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			dto.LastChecked = t
		}
	}
	if v, found, _ := unstructured.NestedInt64(u.Object, "status", "nodeCount"); found {
		dto.NodeCount = int(v)
	}
	if v, ok, _ := unstructured.NestedBool(u.Object, "status", "velero", "installed"); ok {
		dto.VeleroInstalled = v
	}
	if v, ok, _ := nestedString(u.Object, "status", "velero", "version"); ok {
		dto.VeleroVersion = v
	}
	if dto.Type == "primary" {
		dto.IsCurrent = true
	}
	return dto
}

// jsonMarshal — thin wrapper kept as a named seam in case we later need
// to inject structured-merge-friendly encoding (e.g. server-side apply).
func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// remoteClient builds a typed clientset from a rest.Config. Lifted out
// of testClusterConnection so MC2's health-check controller can share
// the same code path. NOT exported — callers should go through the
// MC2 controller's pool when MC2 lands; this is the v0.9.0 MVP path.
func remoteClient(cfg *rest.Config) (kubernetes.Interface, error) {
	return kubernetes.NewForConfig(cfg)
}
