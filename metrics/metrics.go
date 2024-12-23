package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var (
	serverChosen = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "server_chosen_total",
			Help: "Total number of times a server was chosen for a query",
		},
		[]string{"server"},
	)
)

func init() {
	prometheus.MustRegister(serverChosen)
}

// IncrementServerChosen increments the counter for the chosen server
func IncrementServerChosen(server string) {
	serverChosen.With(prometheus.Labels{"server": server}).Inc()
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
