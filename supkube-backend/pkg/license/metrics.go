package license

import "github.com/prometheus/client_golang/prometheus"

// collector exports the current license status as Prometheus gauges. It reads
// the shared Snapshot() on every scrape, so it needs no clients and stays
// consistent with /api/v1/license/status and the write gate.
type collector struct {
	valid     *prometheus.Desc
	daysLeft  *prometheus.Desc
	nodesUsed *prometheus.Desc
	nodesMax  *prometheus.Desc
	state     *prometheus.Desc
}

// NewCollector returns the license Prometheus collector for registration in the
// server's private registry (internal/metrics.NewRegistry).
func NewCollector() prometheus.Collector {
	return &collector{
		valid:     prometheus.NewDesc("supkube_license_valid", "1 if a valid (licensed or grace) license is loaded, else 0.", []string{"id"}, nil),
		daysLeft:  prometheus.NewDesc("supkube_license_days_left", "Whole days until the license expires (negative if expired).", []string{"id"}, nil),
		nodesUsed: prometheus.NewDesc("supkube_license_nodes_used", "Billable worker node count (excludes control-plane / tainted infra nodes).", nil, nil),
		nodesMax:  prometheus.NewDesc("supkube_license_nodes_max", "Licensed maximum node count (0 if no license).", nil, nil),
		state:     prometheus.NewDesc("supkube_license_state", "License lifecycle state; 1 on the active state label.", []string{"state"}, nil),
	}
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.valid
	ch <- c.daysLeft
	ch <- c.nodesUsed
	ch <- c.nodesMax
	ch <- c.state
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	s := Snapshot()
	id, max := "", 0
	if s.License != nil {
		id = s.License.ID
		max = s.License.Restrictions.Nodes
	}

	valid := 0.0
	if s.State == StateLicensed || s.State == StateGrace {
		valid = 1
	}
	ch <- prometheus.MustNewConstMetric(c.valid, prometheus.GaugeValue, valid, id)
	ch <- prometheus.MustNewConstMetric(c.daysLeft, prometheus.GaugeValue, float64(s.DaysLeft()), id)
	ch <- prometheus.MustNewConstMetric(c.nodesUsed, prometheus.GaugeValue, float64(s.NodeCount))
	ch <- prometheus.MustNewConstMetric(c.nodesMax, prometheus.GaugeValue, float64(max))

	for _, st := range []string{StateLicensed, StateGrace, StateDegraded, StateMissing, StateInvalid} {
		v := 0.0
		if st == s.State {
			v = 1
		}
		ch <- prometheus.MustNewConstMetric(c.state, prometheus.GaugeValue, v, st)
	}
}
