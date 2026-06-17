package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewHandler exposes the private registry through promhttp without touching the
// process-wide default Prometheus registry.
func NewHandler(runtimeCli client.Client, k8sCli kubernetes.Interface, clusterName string) http.Handler {
	registry := NewRegistry(runtimeCli, k8sCli, clusterName)
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}
