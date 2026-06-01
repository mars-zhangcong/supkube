// Package clusterhealth — periodic probe of registered Cluster CRs (v0.9.0 MC2).
//
// Polls clusters.supkube.io every 60s; for each Cluster CR it reads the
// referenced kubeconfig Secret, dials the remote cluster (Discovery API +
// Nodes list + Velero Deployment probe), and patches the CR's status
// subresource with:
//
//	.status.phase           Healthy / Unreachable / Unauthorized
//	.status.lastChecked     RFC3339 timestamp
//	.status.message         non-empty when probe fails (useful in UI)
//	.status.k8sVersion      e.g. "v1.32.6"
//	.status.nodeCount
//	.status.velero.{installed,version}
//
// Why a poll loop (not a controller-runtime Reconcile):
//   - State is external to this cluster; informer caches don't help. Each
//     tick must actually dial the remote.
//   - 60s is generous enough that we don't need leader election; if two
//     replicas of supkube-backend ever exist they just both probe — the
//     status patches are idempotent.
//   - Same idiom as internal/policypair — keeps cognitive overhead low.
//
// Why we don't store anything in this package's memory: status is the
// source of truth. The handler in api/v1/clusters.go reads .status when
// it returns the DTO. If this controller crashes, the UI shows stale
// "lastChecked: 10m ago" — not wrong, just stale.
package clusterhealth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	clustersNS   = "supkube"
	pollInterval = 60 * time.Second
	// probeTimeout caps each individual remote dial. With 10 spoke
	// clusters at 8s each we'd block this controller for 80s/cycle and
	// miss the next tick — so 8s is intentionally tight. Customers with
	// pathologically slow clusters will see "Unreachable" until they
	// fix their network; that's the right signal anyway.
	probeTimeout = 8 * time.Second
)

var clusterGVR = schema.GroupVersionResource{
	Group:    "supkube.io",
	Version:  "v1",
	Resource: "clusters",
}

// Run is the controller entry point. Blocks until ctx is cancelled.
func Run(ctx context.Context, dynCli dynamic.Interface, k8sCli kubernetes.Interface) {
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()
	log.Printf("[clusterhealth] runner started (poll interval %s)", pollInterval)

	// Run an immediate scan on boot so the UI doesn't have to wait 60s
	// for the first status update after the backend pod restarts.
	if err := scanOnce(ctx, dynCli, k8sCli); err != nil {
		log.Printf("[clusterhealth] initial scan error: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Printf("[clusterhealth] runner stopped: %v", ctx.Err())
			return
		case <-tick.C:
			if err := scanOnce(ctx, dynCli, k8sCli); err != nil {
				log.Printf("[clusterhealth] scan error: %v (will retry next tick)", err)
			}
		}
	}
}

// scanOnce lists Cluster CRs and probes each in turn. Probes run
// sequentially (not parallel) because (a) total fleet sizes are small
// (typically <10 spokes) and (b) parallel kubeconfig parsing + dial
// would spike CPU + open file descriptors. If fleets grow we can swap
// to a bounded goroutine pool here.
func scanOnce(ctx context.Context, dynCli dynamic.Interface, k8sCli kubernetes.Interface) error {
	list, err := dynCli.Resource(clusterGVR).Namespace(clustersNS).List(ctx, metav1.ListOptions{})
	if err != nil {
		// CRD not installed yet — fresh helm install pre-MC1. Skip
		// silently rather than emit noisy logs every tick.
		if apierrors.IsNotFound(err) || strings.Contains(err.Error(), "no matches for kind") {
			return nil
		}
		return fmt.Errorf("list clusters: %w", err)
	}
	for i := range list.Items {
		probeOne(ctx, dynCli, k8sCli, &list.Items[i])
	}
	return nil
}

// probeOne handles a single Cluster CR. Errors are logged but never
// propagated — we don't want a broken Cluster CR to kill the whole
// scan loop.
func probeOne(ctx context.Context, dynCli dynamic.Interface, k8sCli kubernetes.Interface, cr *unstructured.Unstructured) {
	name := cr.GetName()

	// Step 1: resolve kubeconfig Secret
	secretName, found, _ := unstructured.NestedString(cr.Object, "spec", "kubeconfigSecretRef", "name")
	if !found || secretName == "" {
		// CR is malformed — record as Unauthorized so it shows up red
		// in the UI rather than silently degrading.
		patchStatus(ctx, dynCli, name, statusUpdate{
			Phase: "Unauthorized", Message: "kubeconfigSecretRef.name is empty",
		})
		return
	}
	secretKey, found, _ := unstructured.NestedString(cr.Object, "spec", "kubeconfigSecretRef", "key")
	if !found || secretKey == "" {
		secretKey = "kubeconfig"
	}
	contextName, _, _ := unstructured.NestedString(cr.Object, "spec", "context")

	s, err := k8sCli.CoreV1().Secrets(clustersNS).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		patchStatus(ctx, dynCli, name, statusUpdate{
			Phase: "Unauthorized", Message: fmt.Sprintf("kubeconfig Secret '%s' missing: %v", secretName, err),
		})
		return
	}
	kc, ok := s.Data[secretKey]
	if !ok || len(kc) == 0 {
		patchStatus(ctx, dynCli, name, statusUpdate{
			Phase: "Unauthorized", Message: fmt.Sprintf("Secret '%s' has no key '%s'", secretName, secretKey),
		})
		return
	}

	// Step 2: build a typed clientset against the remote cluster
	cli, err := buildRemoteClient(kc, contextName)
	if err != nil {
		patchStatus(ctx, dynCli, name, statusUpdate{
			Phase: "Unauthorized", Message: "kubeconfig build: " + err.Error(),
		})
		return
	}

	// Step 3: probe (discovery + nodes + velero deploy)
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	upd := probe(probeCtx, cli)

	// Step 4: patch status (idempotent — Patch with merge type)
	patchStatus(ctx, dynCli, name, upd)
}

// probe runs the actual network calls. Pure function of (client, ctx).
func probe(ctx context.Context, cli kubernetes.Interface) statusUpdate {
	upd := statusUpdate{Phase: "Unknown"}

	srv, err := cli.Discovery().ServerVersion()
	if err != nil {
		upd.Phase = classifyDiscoveryError(err)
		upd.Message = "discovery: " + err.Error()
		return upd
	}
	upd.K8sVersion = srv.GitVersion

	if nl, err := cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
		upd.NodeCount = len(nl.Items)
	}

	if d, err := cli.AppsV1().Deployments("velero").Get(ctx, "velero", metav1.GetOptions{}); err == nil {
		upd.VeleroInstalled = true
		if len(d.Spec.Template.Spec.Containers) > 0 {
			img := d.Spec.Template.Spec.Containers[0].Image
			if idx := strings.LastIndex(img, ":"); idx > 0 {
				upd.VeleroVersion = img[idx+1:]
			}
		}
	}

	// Reached here = at least Discovery responded. Healthy.
	upd.Phase = "Healthy"
	upd.Message = ""
	return upd
}

// classifyDiscoveryError separates "wrong creds" (Unauthorized) from
// "can't reach server" (Unreachable) so the UI shows the right hint.
// The strings the apiserver returns are stable enough that a substring
// match is adequate; if a future version changes them we'll just see
// everything as Unreachable, which degrades gracefully.
func classifyDiscoveryError(err error) string {
	if err == nil {
		return "Healthy"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unauthorized"),
		strings.Contains(msg, "forbidden"),
		strings.Contains(msg, "x509"):
		return "Unauthorized"
	default:
		return "Unreachable"
	}
}

// buildRemoteClient parses the kubeconfig + picks the named context,
// applies our probe timeout to the rest.Config, and returns a typed
// clientset. Returns the typed kubernetes.Interface, not the dynamic
// one — probe needs Nodes().List() and Deployments().Get() which
// are easier on the typed client.
func buildRemoteClient(kubeconfig []byte, contextName string) (kubernetes.Interface, error) {
	cfg, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return nil, err
	}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	rest, err := clientcmd.NewDefaultClientConfig(*cfg, overrides).ClientConfig()
	if err != nil {
		return nil, err
	}
	rest.Timeout = probeTimeout
	return kubernetes.NewForConfig(rest)
}

// statusUpdate is the in-memory representation of the patch we'll send.
// Kept separate from the JSON merge struct so we can default-zero fields
// safely (empty-string Velero version etc.).
type statusUpdate struct {
	Phase           string
	Message         string
	K8sVersion      string
	NodeCount       int
	VeleroInstalled bool
	VeleroVersion   string
}

// patchStatus sends a merge-patch to the CR's /status subresource.
// We use merge-patch (not strategic-merge or server-side-apply) because
// the CRD is custom and Kubernetes only supports strategic-merge for
// built-in types. Merge-patch on nested objects fully replaces them
// rather than deep-merging — which is fine here since we always write
// the same set of fields.
func patchStatus(ctx context.Context, dynCli dynamic.Interface, name string, upd statusUpdate) {
	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"phase":       upd.Phase,
			"message":     upd.Message,
			"lastChecked": time.Now().UTC().Format(time.RFC3339),
			"k8sVersion":  upd.K8sVersion,
			"nodeCount":   upd.NodeCount,
			"velero": map[string]interface{}{
				"installed": upd.VeleroInstalled,
				"version":   upd.VeleroVersion,
			},
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		log.Printf("[clusterhealth] %s: marshal patch: %v", name, err)
		return
	}
	if _, err := dynCli.Resource(clusterGVR).Namespace(clustersNS).Patch(
		ctx, name,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
		"status",
	); err != nil {
		// Non-fatal — most likely a transient API-server hiccup; the next
		// 60s tick retries. We log at INFO not ERROR to avoid alert spam.
		log.Printf("[clusterhealth] %s: patch status: %v", name, err)
	}
}

// _corev1 anchor: keep the import linked so a future "include node
// labels in status" change doesn't require re-adding the package.
var _ corev1.SecretType = corev1.SecretTypeOpaque
