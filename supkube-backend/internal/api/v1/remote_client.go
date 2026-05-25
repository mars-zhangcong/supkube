// Package v1: remote_client.go — controller-runtime client for remote
// clusters (v0.9.0 MC3).
//
// Looks up a Cluster CR by name, reads its kubeconfig Secret, and
// returns a fully-typed controller-runtime client that can Create /
// Get / Patch Velero CRs on the remote cluster.
//
// Why a separate file: this helper is used by Restore (MC3), and will
// be used by future MC features (Backup dispatch, cross-cluster
// monitoring, etc.). Keeping it next to api/v1/clusters.go avoids
// circular imports while staying small enough that we don't need an
// internal package for it.
//
// Caching is NOT done here. Each call rebuilds the client from the
// Secret data. Volume is low (cross-cluster ops are user-initiated,
// one per click), and a stale-cache bug here would silently route
// restores to the wrong cluster — way worse than the few-ms cost of
// a fresh client.
package v1

import (
	"context"
	"fmt"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/k8s"
)

// remoteScheme has all the types we might Create on a remote cluster:
//   - core/v1, apps/v1, etc. (via client-go scheme)
//   - Velero v1 (Restore, Backup, Schedule, BSL, ...)
//
// Initialised once at package load. Adding a new resource type to
// dispatch later (e.g. v0.9.10 Distributions) means appending to this
// AddToScheme list.
var remoteScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(remoteScheme))
	utilruntime.Must(velerov1.AddToScheme(remoteScheme))
}

// buildRemoteVeleroClient is the entry point used by handlers. clusterName
// matches a Cluster CR's metadata.name in the supkube namespace.
//
// Errors are intentionally verbose-but-prefixed so the SPA can show the
// raw text to admins without further parsing — most failures here are
// configuration issues the admin needs to see ("Secret not found", "no
// such cluster", "kubeconfig parse error: ...").
func buildRemoteVeleroClient(ctx context.Context, clusterName string) (ctrlclient.Client, error) {
	dynCli, err := k8s.GetDynamicClient()
	if err != nil {
		return nil, fmt.Errorf("local dynamic client: %w", err)
	}
	k8sCli, err := k8s.GetClient()
	if err != nil {
		return nil, fmt.Errorf("local kubernetes client: %w", err)
	}

	// Resolve the Cluster CR.
	cr, err := dynCli.Resource(clusterGVR).Namespace(clustersNamespace).Get(ctx, clusterName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("cluster %q: %w", clusterName, err)
	}

	secretName, _, _ := nestedString(cr.Object, "spec", "kubeconfigSecretRef", "name")
	if secretName == "" {
		return nil, fmt.Errorf("cluster %q: kubeconfigSecretRef.name empty", clusterName)
	}
	secretKey, _, _ := nestedString(cr.Object, "spec", "kubeconfigSecretRef", "key")
	if secretKey == "" {
		secretKey = "kubeconfig"
	}
	contextName, _, _ := nestedString(cr.Object, "spec", "context")

	// Pull the kubeconfig bytes.
	secret, err := k8sCli.CoreV1().Secrets(clustersNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("kubeconfig secret %q: %w", secretName, err)
	}
	kubeconfig, ok := secret.Data[secretKey]
	if !ok || len(kubeconfig) == 0 {
		return nil, fmt.Errorf("kubeconfig secret %q missing key %q", secretName, secretKey)
	}

	// Parse + build rest.Config.
	cfg, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("kubeconfig parse: %w", err)
	}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	rest, err := clientcmd.NewDefaultClientConfig(*cfg, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("kubeconfig build rest: %w", err)
	}

	// controller-runtime client. We pass the local-process scheme so
	// the same Go types (velerov1.Restore etc.) can be created against
	// the remote cluster.
	cli, err := ctrlclient.New(rest, ctrlclient.Options{Scheme: remoteScheme})
	if err != nil {
		return nil, fmt.Errorf("client build: %w", err)
	}
	return cli, nil
}

// buildRemoteTypedClient returns a typed Clientset against the named
// remote cluster. Mirror of buildRemoteVeleroClient but returns the
// client-go typed interface — handlers that need core/apps APIs use this.
func buildRemoteTypedClient(ctx context.Context, clusterName string) (kubernetes.Interface, error) {
	restCfg, err := buildRemoteRestConfig(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(restCfg)
}

// buildRemoteDynamicClient is the dynamic equivalent — for handlers that
// walk CRDs without typed schemes registered (artifact_breakdown,
// resource_yaml, etc.).
func buildRemoteDynamicClient(ctx context.Context, clusterName string) (dynamic.Interface, error) {
	restCfg, err := buildRemoteRestConfig(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(restCfg)
}

// buildRemoteRestConfig is the resolver shared by the three remote-client
// builders above. Lifts the kubeconfig-Secret-→-rest.Config pipeline into
// one place so any future credential rotation logic lands here.
func buildRemoteRestConfig(ctx context.Context, clusterName string) (*rest.Config, error) {
	dynCli, err := k8s.GetDynamicClient()
	if err != nil {
		return nil, fmt.Errorf("local dynamic client: %w", err)
	}
	k8sCli, err := k8s.GetClient()
	if err != nil {
		return nil, fmt.Errorf("local kubernetes client: %w", err)
	}
	cr, err := dynCli.Resource(clusterGVR).Namespace(clustersNamespace).Get(ctx, clusterName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("cluster %q: %w", clusterName, err)
	}
	secretName, _, _ := nestedString(cr.Object, "spec", "kubeconfigSecretRef", "name")
	if secretName == "" {
		return nil, fmt.Errorf("cluster %q: kubeconfigSecretRef.name empty", clusterName)
	}
	secretKey, _, _ := nestedString(cr.Object, "spec", "kubeconfigSecretRef", "key")
	if secretKey == "" {
		secretKey = "kubeconfig"
	}
	contextName, _, _ := nestedString(cr.Object, "spec", "context")

	secret, err := k8sCli.CoreV1().Secrets(clustersNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("kubeconfig secret %q: %w", secretName, err)
	}
	kubeconfig, ok := secret.Data[secretKey]
	if !ok || len(kubeconfig) == 0 {
		return nil, fmt.Errorf("kubeconfig secret %q missing key %q", secretName, secretKey)
	}

	cfg, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("kubeconfig parse: %w", err)
	}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	return clientcmd.NewDefaultClientConfig(*cfg, overrides).ClientConfig()
}
