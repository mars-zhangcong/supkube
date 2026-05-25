// Package v1: request_client.go — X-Supkube-Cluster header routing (v0.9.0.1).
//
// Every API handler that READS cluster state (backups, restores, schedules,
// namespaces, applications, storage locations) should resolve its
// kubernetes client through one of these helpers instead of calling
// k8s.GetRuntimeClient() / k8s.GetClient() directly.
//
// The helpers honour the `X-Supkube-Cluster` header that the SPA axios
// interceptor (useCluster composable) attaches whenever the user has
// the Mode Switcher set to a remote cluster:
//
//   header absent / "" / "this-cluster" / "_mcm" → local client (no change)
//   header = "<cluster-name>"                     → remote client built
//                                                   from that Cluster CR's
//                                                   kubeconfig Secret
//
// What about WRITE handlers (POST/PUT/DELETE)?
//   The cross-cluster Restore dispatch (MC3) goes through a different
//   path — the request body explicitly carries `targetCluster` rather
//   than relying on the header. That's because a write can land on a
//   different cluster than the user is currently viewing (the canonical
//   "from A's RP, restore into B" flow). For reads, the header IS the
//   intent ("show me B's state"), so routing-by-header is the right model.
//
// Failure modes & UX:
//   - If the header names a cluster that doesn't exist → 404 with a clear
//     message. SPA's useCluster composable can clear the bad selection.
//   - If the kubeconfig Secret is broken → 502 with the underlying error.
//   - If the remote API is unreachable → handler-level error (Velero
//     CRD list will time out → propagated as 500 by the handler).
package v1

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/k8s"
)

const headerCluster = "X-Supkube-Cluster"

// resolveTargetCluster returns the cluster name the request is scoped to,
// or empty string for "this cluster" (local). Centralised so reserved
// sentinels (this-cluster, _mcm) stay in one place.
func resolveTargetCluster(c *gin.Context) string {
	v := strings.TrimSpace(c.GetHeader(headerCluster))
	if v == "" || v == "this-cluster" || v == "_mcm" {
		return ""
	}
	return v
}

// getRequestRuntimeClient returns a controller-runtime Client scoped to
// either the local cluster (default) or the remote cluster named in the
// X-Supkube-Cluster header.
func getRequestRuntimeClient(c *gin.Context) (ctrlclient.Client, error) {
	target := resolveTargetCluster(c)
	if target == "" {
		return k8s.GetRuntimeClient()
	}
	return buildRemoteVeleroClient(context.Background(), target)
}

// getRequestKubernetesClient returns a typed clientset for the request's
// target cluster. Used by handlers that need core/apps APIs (namespaces,
// nodes, deployments) — most Velero CR reads go through getRequestRuntimeClient
// above.
//
// For remote dispatch we build a fresh typed clientset by re-using the
// rest.Config path inside buildRemoteVeleroClient — but since that helper
// returns a ctrlclient and not a rest.Config, we replicate the resolution
// here. Slightly redundant but keeps the helpers independent (callers
// don't need to know which one they want for their handler).
func getRequestKubernetesClient(c *gin.Context) (kubernetes.Interface, error) {
	target := resolveTargetCluster(c)
	if target == "" {
		return k8s.GetClient()
	}
	return buildRemoteTypedClient(context.Background(), target)
}

// getRequestDynamicClient returns a dynamic.Interface for the request's
// target cluster. Used by handlers that walk CRDs we don't have typed
// schemes for (artifact_breakdown, resource_yaml, etc.).
func getRequestDynamicClient(c *gin.Context) (dynamic.Interface, error) {
	target := resolveTargetCluster(c)
	if target == "" {
		return k8s.GetDynamicClient()
	}
	return buildRemoteDynamicClient(context.Background(), target)
}
