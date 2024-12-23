package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	serverChosen = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "server_chosen_total",
			Help: "Total number of times a server was chosen for a query",
		},
		[]string{"server", "path"},
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
	ProbeSelectionCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "probe_selection_total",
			Help: "Total number of times hot/cold probes were selected",
		},
		[]string{"type", "server_id"}, // type will be "hot" or "cold"
	)

	// Current RIF gauge
	CurrentRIF = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "server_current_rif",
		Help: "Current number of requests in flight",
	})

	// Request latency histogram by path
	RequestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "server_request_latency_seconds",
			Help:    "Request latency in seconds by path",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // from 1ms to ~1s
		},
		[]string{"path"},
	)

	// Current median latency gauge
	MedianLatency = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "server_median_latency_seconds",
		Help: "Current median latency across all requests",
	})
)

func InitClientMetrics() {
	prometheus.MustRegister(serverChosen)
	prometheus.MustRegister(maxRIF)
	prometheus.MustRegister(normalizedRIF)
	prometheus.MustRegister(probeReuseCount)
	prometheus.MustRegister(staleProbes)
	prometheus.MustRegister(ProbeSelectionCount)
}

func InitServerMetrics() {
	prometheus.MustRegister(CurrentRIF)
	prometheus.MustRegister(RequestLatency)
	prometheus.MustRegister(MedianLatency)
}

// IncrementServerChosen increments the counter for the chosen server
func IncrementServerChosen(server, job string) {
	serverChosen.With(prometheus.Labels{"server": server, "path": job}).Inc()
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

// Server metric update functions
func UpdateCurrentRIF(value int64) {
	CurrentRIF.Set(float64(value))
}

func ObserveRequestLatency(path string, duration time.Duration) {
	RequestLatency.With(prometheus.Labels{
		"path": path,
	}).Observe(duration.Seconds())
}

func UpdateMedianLatency(value time.Duration) {
	MedianLatency.Set(value.Seconds())
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

func IncrementProbeSelection(probeType string, serverID string) {
	ProbeSelectionCount.With(prometheus.Labels{
		"type":      probeType,
		"server_id": serverID,
	}).Inc()
}
