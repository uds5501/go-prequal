package server

import (
	"time"
	"sync"
	"container/heap"
	"sort"
)

type Metric struct {
	RIF     uint64
	Latency time.Duration
}

type MetricReporter struct {
	metrics    []Metric
	maxMetrics int
	metricsMu  sync.RWMutex
}

func NewMetricReporter() *MetricReporter {
	return &MetricReporter{
		metrics:    make([]Metric, 0, 1000),
		maxMetrics: 1000,
	}
}

func (m *MetricReporter) recordMetric(rif uint64, latency time.Duration) {
	m.metricsMu.Lock()
	defer m.metricsMu.Unlock()

	metric := Metric{RIF: rif, Latency: latency}

	m.metrics = append(m.metrics, metric)

	// Maintain max size
	if len(m.metrics) > m.maxMetrics {
		m.metrics = m.metrics[1:]
	}
}

func (m *MetricReporter) getNearestLatencies(rif uint64) time.Duration {
	m.metricsMu.RLock()
	defer m.metricsMu.RUnlock()

	if len(m.metrics) == 0 {
		return 0
	}

	h := &MaxHeap{}
	heap.Init(h)

	for _, metric := range m.metrics {
		absDiff := uint64(0)
		if metric.RIF > rif {
			absDiff = metric.RIF - rif
		} else {
			absDiff = rif - metric.RIF
		}
		customMetric := Metric{RIF: absDiff, Latency: metric.Latency}

		heap.Push(h, customMetric)
		if h.Len() > 5 {
			heap.Pop(h)
		}
	}

	latencies := make([]time.Duration, h.Len())
	for i := range latencies {
		latencies[i] = heap.Pop(h).(Metric).Latency
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	return latencies[len(latencies)/2]
}
