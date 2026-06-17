package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// supKubeCollector is the package collector skeleton. Metric descriptors are
// defined up front so future implementations can add values without changing
// the public factory contract.
type supKubeCollector struct {
	runtimeCli  client.Client
	k8sCli      kubernetes.Interface
	clusterName string

	upDesc *prometheus.Desc
}

// NewSupKubeCollector returns the SupKube custom collector required by the PRD.
// It is safe to register in a private Prometheus registry.
func NewSupKubeCollector(runtimeCli client.Client, k8sCli kubernetes.Interface, clusterName string) prometheus.Collector {
	return &supKubeCollector{
		runtimeCli:  runtimeCli,
		k8sCli:      k8sCli,
		clusterName: clusterName,
		upDesc: prometheus.NewDesc(
			"supkube_up",
			"SupKube exporter availability.",
			[]string{"cluster"},
			nil,
		),
	}
}

// Describe exposes all metric descriptors emitted by this collector.
func (c *supKubeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.upDesc
}

// Collect emits a minimal liveness metric so the collector is functional while
// the rest of the metric set is added incrementally in future work-items.
func (c *supKubeCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(c.upDesc, prometheus.GaugeValue, 1, c.clusterName)
}
