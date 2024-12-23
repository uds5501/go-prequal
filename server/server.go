package server

import (
	"encoding/json"
	"fmt"
	"go-prequel/metrics"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"math/rand"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	rif uint64 // Request in flight counter

	// Store last 1000 RIF-latency pairs
	metricReporter *MetricReporter
	port           string
	logger         *log.Logger
}

type BatchRequest struct {
	Strings []string `json:"strings"`
}

type Response struct {
	Message string `json:"message"`
}

type ProbeResponse struct {
	RIF           uint64        `json:"rif"`
	MedianLatency time.Duration `json:"latency"`
}

func NewServer() *Server {
	return &Server{
		metricReporter: NewMetricReporter(),
	}
}

func (s *Server) incrementRIF() uint64 {
	return atomic.AddUint64(&s.rif, 1)
}

func (s *Server) decrementRIF() uint64 {
	return atomic.AddUint64(&s.rif, ^uint64(0))
}

// getCurrentRIF returns the current RIF value
func (s *Server) getCurrentRIF() uint64 {
	return atomic.LoadUint64(&s.rif)
}

func (s *Server) HandleBatchProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rif := s.incrementRIF()
	metrics.UpdateCurrentRIF(int64(rif))
	start := time.Now()
	defer func() {
		s.decrementRIF()
		duration := time.Since(start)
		s.metricReporter.recordMetric(rif, duration)
		metrics.ObserveRequestLatency("/batch", duration)
	}()

	var req BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Simulate long processing
	baseDuration := 10 * time.Second
	randomOffset := time.Duration(rand.Intn(11)-5) * time.Second // Random duration between -10 and +10 seconds
	time.Sleep(baseDuration + randomOffset)

	json.NewEncoder(w).Encode(Response{
		Message: "Processed batch of " + fmt.Sprint(req.Strings) + " strings",
	})
}

func (s *Server) HandlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rif := s.incrementRIF()
	metrics.UpdateCurrentRIF(int64(rif))
	start := time.Now()
	defer func() {
		s.decrementRIF()
		duration := time.Since(start)
		s.metricReporter.recordMetric(rif, duration)
		metrics.ObserveRequestLatency("/ping", duration)
	}()

	json.NewEncoder(w).Encode(Response{Message: "pong"})
}

func (s *Server) HandleMediumProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rif := s.incrementRIF()
	metrics.UpdateCurrentRIF(int64(rif))
	start := time.Now()
	defer func() {
		s.decrementRIF()
		duration := time.Since(start)
		s.metricReporter.recordMetric(rif, duration)
		metrics.ObserveRequestLatency("/medium", duration)
	}()

	// Simulate medium processing
	baseDuration := 3 * time.Second
	randomOffset := time.Duration(rand.Intn(3)-1) * time.Second // Random duration between -10 and +10 seconds
	time.Sleep(baseDuration + randomOffset)

	json.NewEncoder(w).Encode(Response{Message: "Medium process complete"})
}

// HandleProbe handles probe requests
func (s *Server) HandleProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentRIF := s.getCurrentRIF()
	medianLatency := s.metricReporter.getNearestLatencies(currentRIF)
	s.logger.Printf("Current RIF: %d, Median Latency: %v", currentRIF, medianLatency)
	metrics.UpdateMedianLatency(medianLatency)

	json.NewEncoder(w).Encode(ProbeResponse{
		RIF:           currentRIF,
		MedianLatency: medianLatency,
	})
}

func (s *Server) Start(addr string) error {
	// Initialize logger with address prefix
	prefix := fmt.Sprintf("[Server %s] ", addr)
	s.logger = log.New(os.Stdout, prefix, log.LstdFlags)

	metrics.InitServerMetrics()

	s.logger.Printf("Starting server on %s", addr)

	mux := http.NewServeMux()
	mux.HandleFunc("/batch", s.HandleBatchProcess)
	mux.HandleFunc("/ping", s.HandlePing)
	mux.HandleFunc("/medium", s.HandleMediumProcess)
	mux.HandleFunc("/probe", s.HandleProbe)
	mux.Handle("/metrics", promhttp.Handler())

	return http.ListenAndServe(addr, mux)
}
