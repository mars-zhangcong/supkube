package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewRegistry builds a private registry per ADR-056 D1. It intentionally does
// not use the global default registerer.
func NewRegistry(runtimeCli client.Client, k8sCli kubernetes.Interface, clusterName string) *prometheus.Registry {
	registry := prometheus.NewRegistry()

	registry.MustRegister(
		NewSupKubeCollector(runtimeCli, k8sCli, clusterName),
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return registry
}
