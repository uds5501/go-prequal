package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-prequel/metrics"
	"net/http"
	"sync"
	"time"
)

// ProbeInfo represents a single probe response
type ProbeInfo struct {
	RIF           uint64 // Request in flight counter from server
	Latency       time.Duration
	ServerID      string
	Timestamp     time.Time
	UseCount      int     // Number of times this probe has been reused
	NormalizedRIF float64 // Normalized RIF value for this server
}

// Config holds client configuration
type Config struct {
	MaxProbePoolSize int           `json:"max_probe_pool_size"` // M in the spec (default 16)
	NumReplicas      int           `json:"num_replicas"`        // N in the spec
	ProbeRate        float64       `json:"probe_rate"`          // r_probe
	QRIFThreshold    float64       `json:"q_rif_threshold"`     // Q_RIF threshold to determine hot/cold
	DeltaReuse       float64       `json:"delta_reuse"`         // delta for b_reuse calculation
	MaxProbeAge      time.Duration `json:"max_probe_age"`       // Maximum age of a probe before considered stale
	MaxProbeUse      int           `json:"max_probe_use"`       // Maximum number of times a probe can be reused (calculated from bReuse)
	Servers          []string      `json:"servers"`
}

// ServerPool represents a pool of available servers
type ServerPool struct {
	Servers []string
	mu      sync.RWMutex
}

// Client manages server selection and probing
type Client struct {
	config Config
	probes []ProbeInfo
	mu     sync.RWMutex

	// Server pool
	pool ServerPool

	// Channel to control probe rate
	probeTicker *time.Ticker
	done        chan struct{}

	// Track maximum RIF seen across all servers
	maxRIF uint64
}

// NewClient creates a new client with the given configuration and server addresses
func NewClient(config Config, servers []string) *Client {
	if config.MaxProbePoolSize == 0 {
		config.MaxProbePoolSize = 16
	}
	if config.DeltaReuse == 0 {
		config.DeltaReuse = 0.1
	}
	if config.MaxProbeAge == 0 {
		config.MaxProbeAge = 5 * time.Second
	}
	config.MaxProbeUse = calculateBReuse(config)

	// Ensure we have at most 5 servers
	if len(servers) > 5 {
		servers = servers[:5]
	}

	c := &Client{
		config: config,
		probes: make([]ProbeInfo, 0, config.MaxProbePoolSize),
		pool: ServerPool{
			Servers: servers,
		},
		done:   make(chan struct{}),
		maxRIF: 0, // Initialize maxRIF
	}

	// Start probe ticker based on probe rate
	interval := time.Duration(float64(time.Second) / config.ProbeRate)
	c.probeTicker = time.NewTicker(interval)

	go c.probeLoop()
	return c
}

// calculateBReuse calculates the reuse factor
func calculateBReuse(config Config) int {
	rRemove := 1.0 / float64(config.MaxProbeAge.Seconds())
	denominator := (1.0 - float64(config.MaxProbePoolSize)/float64(config.NumReplicas)*config.ProbeRate) - rRemove
	bReuse := (1.0 + config.DeltaReuse) / denominator
	if bReuse < 1.0 {
		return 1
	}
	return int(bReuse)
}

func (c *Client) updateRIFDistribution(probe *ProbeInfo) {
	if c.maxRIF > 0 {
		probe.NormalizedRIF = float64(probe.RIF) / float64(c.maxRIF)
	} else {
		probe.NormalizedRIF = 1
	}
}

// isProbeHot determines if a probe represents a hot server
func (c *Client) isProbeHot(probe ProbeInfo) bool {
	// Compare the estimated RIF distribution with QRIFThreshold
	return probe.NormalizedRIF >= c.config.QRIFThreshold
}

// SelectReplica selects the best replica based on the HCL algorithm
func (c *Client) SelectReplica(job string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.probes) == 0 {
		return "", fmt.Errorf("no probes available")
	}

	// Find if we have any cold replicas
	var coldProbes, hotProbes []ProbeInfo
	for i := range c.probes {
		if c.isProbeHot(c.probes[i]) {
			hotProbes = append(hotProbes, c.probes[i])
		} else {
			coldProbes = append(coldProbes, c.probes[i])
		}
	}

	// Select probe and increment its use count
	var selected *ProbeInfo
	if len(coldProbes) > 0 {
		selected = &coldProbes[0]
		for i := range coldProbes {
			if coldProbes[i].RIF < selected.RIF {
				selected = &coldProbes[i]
			}
		}
	} else {
		selected = &hotProbes[0]
		for i := range hotProbes {
			if hotProbes[i].Latency < selected.Latency {
				selected = &hotProbes[i]
			}
		}
	}

	// Find and increment the use count of the selected probe
	for i := range c.probes {
		if c.probes[i].ServerID == selected.ServerID {
			c.probes[i].UseCount++
			metrics.IncrementProbeReuse(selected.ServerID)
			break
		}
	}

	metrics.IncrementServerChosen(selected.ServerID, job)

	return selected.ServerID, nil
}

// probeLoop continuously probes servers at the configured rate
func (c *Client) probeLoop() {
	for {
		select {
		case <-c.done:
			return
		case <-c.probeTicker.C:
			c.Probe()
		}
	}
}

// Stop stops the client's probing
func (c *Client) Stop() {
	close(c.done)
	c.probeTicker.Stop()
}

// Probe implements the probing logic
func (c *Client) Probe() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.removeStaleAndOverusedProbes()

	if len(c.probes) >= c.config.MaxProbePoolSize {
		c.removeProbe()
	}

	// Probe all servers in the pool
	c.pool.mu.RLock()
	for _, server := range c.pool.Servers {
		probeInfo, err := c.ProbeServer(server)
		if err != nil {
			continue
		}

		// Update maxRIF if we see a higher value
		if probeInfo.RIF > c.maxRIF {
			c.maxRIF = probeInfo.RIF
			metrics.UpdateMaxRIF(c.maxRIF)
		}

		c.probes = append(c.probes, *probeInfo)
	}

	// Update normalized RIF for all existing probes
	for i := range c.probes {
		c.updateRIFDistribution(&c.probes[i])
		metrics.UpdateNormalizedRIF(c.probes[i].ServerID, c.probes[i].NormalizedRIF)
	}
	c.pool.mu.RUnlock()
}

// removeStaleAndOverusedProbes removes probes that are too old or have been used too many times
func (c *Client) removeStaleAndOverusedProbes() {
	now := time.Now()
	fresh := make([]ProbeInfo, 0, len(c.probes))
	staleCount := 0

	for _, probe := range c.probes {
		if now.Sub(probe.Timestamp) < c.config.MaxProbeAge &&
			probe.UseCount < c.config.MaxProbeUse {
			fresh = append(fresh, probe)
		} else {
			staleCount++
		}
	}

	if staleCount > 0 {
		metrics.AddStaleProbes(staleCount)
	}

	c.probes = fresh
}

// removeProbe implements the probe removal strategy
func (c *Client) removeProbe() {
	if len(c.probes) == 0 {
		return
	}

	// Find hot probes
	var hotProbes []ProbeInfo
	for _, probe := range c.probes {
		if c.isProbeHot(probe) {
			hotProbes = append(hotProbes, probe)
		}
	}

	// If we have hot probes, remove the one with highest RIF
	if len(hotProbes) > 0 {
		maxRIFIndex := 0
		for i, probe := range hotProbes {
			if probe.RIF > hotProbes[maxRIFIndex].RIF {
				maxRIFIndex = i
			}
		}
		c.removeProbeByServerID(hotProbes[maxRIFIndex].ServerID)
		return
	}

	// If all probes are cold, remove the one with highest latency
	maxLatencyIndex := 0
	for i, probe := range c.probes {
		if probe.Latency > c.probes[maxLatencyIndex].Latency {
			maxLatencyIndex = i
		}
	}
	c.removeProbeByServerID(c.probes[maxLatencyIndex].ServerID)
}

// removeProbeByServerID removes a probe with the given server ID
func (c *Client) removeProbeByServerID(serverID string) {
	for i, probe := range c.probes {
		if probe.ServerID == serverID {
			c.probes = append(c.probes[:i], c.probes[i+1:]...)
			return
		}
	}
}

type ServerResponse struct {
	Message string `json:"message"`
	RIF     uint64 `json:"rif"`
}

// ProbeServer probes a server and returns its RIF
func (c *Client) ProbeServer(serverAddr string) (*ProbeInfo, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s/probe", serverAddr))
	if err != nil {
		return nil, fmt.Errorf("probe failed: %w", err)
	}
	defer resp.Body.Close()

	var probeResp struct {
		RIF     uint64        `json:"rif"`
		Latency time.Duration `json:"latency"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&probeResp); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	return &ProbeInfo{
		RIF:       probeResp.RIF,
		Latency:   probeResp.Latency,
		ServerID:  serverAddr,
		Timestamp: time.Now(),
		UseCount:  0,
	}, nil
}

// BatchProcess sends a batch processing request
func (c *Client) BatchProcess(strings []string) error {
	serverAddr, err := c.SelectReplica("batch")
	if err != nil {
		return fmt.Errorf("no replica available: %w", err)
	}
	reqBody, err := json.Marshal(map[string][]string{
		"strings": strings,
	})
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/batch", serverAddr), "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error: %s", resp.Status)
	}

	return nil
}

// Ping sends a ping request
func (c *Client) Ping() error {
	serverAddr, err := c.SelectReplica("ping")
	if err != nil {
		return fmt.Errorf("no replica available: %w", err)
	}
	resp, err := http.Get(fmt.Sprintf("http://%s/ping", serverAddr))
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error: %s", resp.Status)
	}

	return nil
}

// MediumProcess sends a medium processing request
func (c *Client) MediumProcess() error {
	serverAddr, err := c.SelectReplica("medium")
	if err != nil {
		return fmt.Errorf("no replica available: %w", err)
	}
	resp, err := http.Post(fmt.Sprintf("http://%s/medium", serverAddr), "application/json", nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error: %s", resp.Status)
	}

	return nil
}
