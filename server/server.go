package server

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
	"fmt"
	"log"
)

type Server struct {
	rif uint64 // Request in flight counter

	// Store last 1000 RIF-latency pairs
	metricReporter *MetricReporter
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
	start := time.Now()
	defer func() {
		s.decrementRIF()
		s.metricReporter.recordMetric(rif, time.Since(start))
	}()

	var req BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Simulate long processing
	time.Sleep(10 * time.Second)

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
	start := time.Now()
	defer func() {
		s.decrementRIF()
		s.metricReporter.recordMetric(rif, time.Since(start))
	}()

	json.NewEncoder(w).Encode(Response{Message: "pong"})
}

func (s *Server) HandleMediumProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rif := s.incrementRIF()
	start := time.Now()
	defer func() {
		s.decrementRIF()
		s.metricReporter.recordMetric(rif, time.Since(start))
	}()

	// Simulate medium processing
	time.Sleep(2 * time.Second)

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

	json.NewEncoder(w).Encode(ProbeResponse{
		RIF:           currentRIF,
		MedianLatency: medianLatency,
	})
}

func (s *Server) Start(addr string) error {
	log.Printf("Starting server on %s", addr)
	mux := http.NewServeMux()
	mux.HandleFunc("/batch", s.HandleBatchProcess)
	mux.HandleFunc("/ping", s.HandlePing)
	mux.HandleFunc("/medium", s.HandleMediumProcess)
	mux.HandleFunc("/probe", s.HandleProbe)

	return http.ListenAndServe(addr, mux)
}
