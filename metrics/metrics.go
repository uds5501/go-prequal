package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	serverChosen = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "server_chosen_total",
			Help: "Total number of times a server was chosen for a query",
		},
		[]string{"server", "job"},
	)
	maxRIF = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_max_rif",
		Help: "Current maximum Request In Flight (RIF) value among all probes",
	})
	normalizedRIF = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "probe_normalized_rif",
		Help: "Normalized RIF value for each server",
	}, []string{"server_id"})
	probeReuseCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "probe_reuse_count",
		Help: "Number of times each probe has been reused",
	}, []string{"server_id"})
	staleProbes = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "probe_stale_total",
		Help: "Total number of probes considered stale due to age",
	})
)

func InitClientMetrics() {
	prometheus.MustRegister(serverChosen)
	prometheus.MustRegister(maxRIF)
	prometheus.MustRegister(normalizedRIF)
	prometheus.MustRegister(probeReuseCount)
	prometheus.MustRegister(staleProbes)
}

// IncrementServerChosen increments the counter for the chosen server
func IncrementServerChosen(server, job string) {
	serverChosen.With(prometheus.Labels{"server": server, "job": job}).Inc()
}

// UpdateMaxRIF updates the maximum RIF metric
func UpdateMaxRIF(value uint64) {
	maxRIF.Set(float64(value))
}

// UpdateNormalizedRIF updates the normalized RIF metric for a server
func UpdateNormalizedRIF(serverID string, value float64) {
	normalizedRIF.With(prometheus.Labels{
		"server_id": serverID,
	}).Set(value)
}

// IncrementProbeReuse increments the probe reuse counter for a server
func IncrementProbeReuse(serverID string) {
	probeReuseCount.With(prometheus.Labels{
		"server_id": serverID,
	}).Inc()
}

// AddStaleProbes increments the stale probes counter
func AddStaleProbes(count int) {
	staleProbes.Add(float64(count))
}

// StartMetricsServer starts an HTTP server for exposing Prometheus metrics
func StartMetricsServer(addr string) {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			panic(err)
		}
	}()
}
