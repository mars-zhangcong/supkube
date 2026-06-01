package k8s

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}
	// Fall back to kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

// GetClient returns a standard Kubernetes client
func GetClient() (*kubernetes.Clientset, error) {
	config, err := getConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

// GetRuntimeClient returns a controller-runtime client with Velero scheme
func GetRuntimeClient() (client.Client, error) {
	config, err := getConfig()
	if err != nil {
		return nil, err
	}
	scheme := runtime.NewScheme()
	_ = velerov1.AddToScheme(scheme)
	// v0.8.6: register snapshot.storage.k8s.io so VolumeSnapshotContent
	// reads work via the typed client (used by backup composition meta
	// to sum restoreSize per backup). The earlier "dynamic-only" comment
	// below is outdated for VSC specifically; other one-field CRDs still
	// go through the dynamic client.
	_ = snapshotv1.AddToScheme(scheme)
	// v0.8.6: also register core/v1 (Secret, ConfigMap, Namespace, ...)
	// because the BSL credential fallback path reads the velero-owned
	// `cloud-credentials` Secret via the typed client. Without this,
	// cl.Get(... Secret ...) errors with "no kind is registered for v1.Secret"
	// and the tarball-size feature silently degrades to "no creds → IMDS lookup".
	_ = corev1.AddToScheme(scheme)
	// v0.8.7.3: Velero v1.13+ moved DataUpload / DataDownload (Data Mover
	// CRDs) into the v2alpha1 API group. Backup composition meta needs
	// to read DataUpload.status.progress.totalBytes to surface how much
	// data was actually shipped via Data Mover. Without this group
	// registered, list calls error with "no kind is registered" and the
	// backup detail shows volumeBytes=0 even when GB went up to Azure.
	_ = velerov2alpha1.AddToScheme(scheme)

	return client.New(config, client.Options{Scheme: scheme})
}

// GetDynamicClient returns a dynamic client for accessing CRDs whose types
// we don't link at compile time — e.g. snapshot.storage.k8s.io/VolumeSnapshotClass
// (we don't want to add the external-snapshotter Go module dependency just to
// read one field of one CRD).
func GetDynamicClient() (dynamic.Interface, error) {
	config, err := getConfig()
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(config)
}
