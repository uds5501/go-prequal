package server

import (
	"testing"
	"time"
	"fmt"
)

func TestGetNearestLatencies(t *testing.T) {
	reporter := NewMetricReporter()

	// Add some metrics
	reporter.recordMetric(1, 10*time.Millisecond)
	reporter.recordMetric(3, 20*time.Millisecond)
	reporter.recordMetric(9, 30*time.Millisecond)
	reporter.recordMetric(21, 40*time.Millisecond)
	reporter.recordMetric(42, 50*time.Millisecond)
	reporter.recordMetric(1, 60*time.Millisecond)
	reporter.recordMetric(7, 70*time.Millisecond)

	tests := []struct {
		rif      uint64
		expected time.Duration
	}{
		{3, 30 * time.Millisecond},
		{15, 40 * time.Millisecond},
		{1, 30 * time.Millisecond},
		{70, 40 * time.Millisecond},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("Testing for RIF %v", test.rif), func(t *testing.T) {
			latency := reporter.getNearestLatencies(test.rif)
			if latency != test.expected {
				t.Errorf("Expected %v, got %v", test.expected, latency)
			}
		})
	}
}
