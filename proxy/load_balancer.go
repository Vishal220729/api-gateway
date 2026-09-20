package proxy

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// UpstreamTarget represents a single backend server instance in a load-balanced pool.
type UpstreamTarget struct {
	URL         string        `json:"url"`
	Healthy     bool          `json:"healthy"`
	ActiveConns int64         `json:"active_conns"`
	LatencyMs   int64         `json:"latency_ms"`
	LastChecked time.Time     `json:"last_checked"`
	Fails       int           `json:"fails"`
	ParsedURL   *url.URL      `json:"-"`
	mu          sync.RWMutex  `json:"-"`
}

func (t *UpstreamTarget) IsHealthy() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Healthy
}

func (t *UpstreamTarget) SetHealthy(healthy bool, latencyMs int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Healthy = healthy
	t.LatencyMs = latencyMs
	t.LastChecked = time.Now()
	if healthy {
		t.Fails = 0
	} else {
		t.Fails++
	}
}

// LoadBalancer manages a pool of upstream backend nodes using Round-Robin with active health checks.
type LoadBalancer struct {
	targets []*UpstreamTarget
	rrIndex atomic.Uint64
	client  *http.Client
	stopCh  chan struct{}
	mu      sync.RWMutex
}

// NewLoadBalancer initializes the load balancer with the provided upstream URLs and starts active health checking.
func NewLoadBalancer(upstreamURLs []string) *LoadBalancer {
	lb := &LoadBalancer{
		targets: make([]*UpstreamTarget, 0, len(upstreamURLs)),
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
		stopCh: make(chan struct{}),
	}

	for _, rawURL := range upstreamURLs {
		parsed, err := url.Parse(rawURL)
		if err == nil {
			lb.targets = append(lb.targets, &UpstreamTarget{
				URL:         rawURL,
				Healthy:     true,
				ParsedURL:   parsed,
				LastChecked: time.Now(),
			})
		}
	}

	// Start active health check loop every 5 seconds
	go lb.healthCheckLoop()

	return lb
}

// NextTarget returns the next healthy upstream target using Round-Robin.
// If all targets are unhealthy, it fails open to the first available target as a best effort.
func (lb *LoadBalancer) NextTarget() *UpstreamTarget {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	n := len(lb.targets)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return lb.targets[0]
	}

	// Try round-robin over targets up to n attempts to find a healthy one
	startIdx := lb.rrIndex.Add(1)
	for i := 0; i < n; i++ {
		idx := int((startIdx + uint64(i)) % uint64(n))
		target := lb.targets[idx]
		if target.IsHealthy() {
			return target
		}
	}

	// Fallback to first target if none are marked healthy
	return lb.targets[0]
}

// AddTarget adds a new upstream target URL to the pool if it does not already exist.
func (lb *LoadBalancer) AddTarget(rawURL string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for _, t := range lb.targets {
		if t.URL == rawURL {
			return
		}
	}

	parsed, err := url.Parse(rawURL)
	if err == nil {
		lb.targets = append(lb.targets, &UpstreamTarget{
			URL:         rawURL,
			Healthy:     true,
			ParsedURL:   parsed,
			LastChecked: time.Now(),
		})
	}
}

// GetNodeStatuses returns a snapshot of all upstream nodes and their health metrics.
func (lb *LoadBalancer) GetNodeStatuses() []UpstreamTarget {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	result := make([]UpstreamTarget, len(lb.targets))
	for i, t := range lb.targets {
		t.mu.RLock()
		result[i] = UpstreamTarget{
			URL:         t.URL,
			Healthy:     t.Healthy,
			ActiveConns: atomic.LoadInt64(&t.ActiveConns),
			LatencyMs:   t.LatencyMs,
			LastChecked: t.LastChecked,
			Fails:       t.Fails,
		}
		t.mu.RUnlock()
	}
	return result
}

// healthCheckLoop periodically pings /healthz on each upstream target.
func (lb *LoadBalancer) healthCheckLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lb.mu.RLock()
			targets := make([]*UpstreamTarget, len(lb.targets))
			copy(targets, lb.targets)
			lb.mu.RUnlock()

			for _, t := range targets {
				go lb.checkTarget(t)
			}
		case <-lb.stopCh:
			return
		}
	}
}

func (lb *LoadBalancer) checkTarget(t *UpstreamTarget) {
	healthURL := t.URL + "/healthz"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		t.SetHealthy(false, 0)
		return
	}

	start := time.Now()
	resp, err := lb.client.Do(req)
	latency := time.Since(start).Milliseconds()

	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 400 {
		t.SetHealthy(false, latency)
		if resp != nil {
			resp.Body.Close()
		}
		return
	}
	resp.Body.Close()
	t.SetHealthy(true, latency)
}

// Stop stops the background health checker.
func (lb *LoadBalancer) Stop() {
	close(lb.stopCh)
}
