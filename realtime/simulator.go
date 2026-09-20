package realtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"
)

// GlobalOrigin represents a geographic traffic source.
type GlobalOrigin struct {
	Region    string  `json:"region"`
	Country   string  `json:"country"`
	IP        string  `json:"ip"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ThreatPing represents a live threat or request pinpoint for the Threat Radar map.
type ThreatPing struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	IP        string    `json:"ip"`
	Region    string    `json:"region"`
	Country   string    `json:"country"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int       `json:"status"` // 200, 429, 403, 500
	LatencyMs int64     `json:"latency_ms"`
	IsThreat  bool      `json:"is_threat"`
	Reason    string    `json:"reason,omitempty"`
}

// SimulatorStatus represents the current state of the live traffic engine.
type SimulatorStatus struct {
	Active     bool  `json:"active"`
	TargetRPS  int   `json:"target_rps"`
	TotalSent  int64 `json:"total_sent"`
	TotalBurst int64 `json:"total_burst"`
}

// TrafficSimulator generates realistic global enterprise traffic continuously.
type TrafficSimulator struct {
	active     atomic.Bool
	targetRPS  atomic.Int32
	totalSent  atomic.Int64
	totalBurst atomic.Int64
	origins    []GlobalOrigin
	endpoints  []string
	handler    http.Handler
	onPing     func(ThreatPing)
}

func NewTrafficSimulator(handler http.Handler, onPing func(ThreatPing)) *TrafficSimulator {
	s := &TrafficSimulator{
		handler: handler,
		onPing:  onPing,
		origins: []GlobalOrigin{
			{Region: "us-east-1 (N. Virginia)", Country: "US", IP: "198.51.100.24", Latitude: 37.77, Longitude: -122.41},
			{Region: "eu-west-1 (Ireland)", Country: "GB", IP: "185.190.140.8", Latitude: 51.50, Longitude: -0.12},
			{Region: "ap-south-1 (Mumbai)", Country: "IN", IP: "103.21.244.18", Latitude: 19.07, Longitude: 72.87},
			{Region: "ap-northeast-1 (Tokyo)", Country: "JP", IP: "133.242.18.9", Latitude: 35.67, Longitude: 139.65},
			{Region: "sa-east-1 (São Paulo)", Country: "BR", IP: "177.185.32.14", Latitude: -23.55, Longitude: -46.63},
			{Region: "ap-southeast-1 (Singapore)", Country: "SG", IP: "103.247.10.12", Latitude: 1.35, Longitude: 103.81},
		},
		endpoints: []string{
			"/api/orders",
			"/api/users",
			"/api/products",
			"/api/checkout",
			"/api/ai/chat",
		},
	}

	s.active.Store(true)
	s.targetRPS.Store(12) // Default 12 req/sec

	return s
}

// Start launches the background simulation loop.
func (s *TrafficSimulator) Start() {
	go func() {
		for {
			if !s.active.Load() {
				time.Sleep(200 * time.Millisecond)
				continue
			}

			rps := int(s.targetRPS.Load())
			if rps <= 0 {
				time.Sleep(200 * time.Millisecond)
				continue
			}

			interval := time.Second / time.Duration(rps)
			s.sendOneSimulatedRequest()
			time.Sleep(interval)
		}
	}()
}

func (s *TrafficSimulator) sendOneSimulatedRequest() {
	origin := s.origins[rand.Intn(len(s.origins))]
	endpoint := s.endpoints[rand.Intn(len(s.endpoints))]

	method := "GET"
	var body []byte

	if endpoint == "/api/checkout" || endpoint == "/api/orders" {
		if rand.Float32() < 0.4 {
			method = "POST"
			body = []byte(`{"item_id":"prod-99","quantity":1,"currency":"USD"}`)
		}
	} else if endpoint == "/api/ai/chat" {
		method = "POST"
		prompts := []string{
			"What is the sliding window rate limit algorithm?",
			"Explain WAF perimeter defense and OWASP headers",
			"How does the upstream circuit breaker work?",
			"Describe multi-region edge mesh topology",
		}
		prompt := prompts[rand.Intn(len(prompts))]
		body, _ = json.Marshal(map[string]string{"prompt": prompt, "model": "gemini-1.5-flash"})
	}

	req := httptest.NewRequest(method, endpoint, bytes.NewReader(body))
	req.Header.Set("X-Forwarded-For", origin.IP)
	req.Header.Set("X-Country-Code", origin.Country)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Enterprise-Client/4.2; +https://gateway.internal)")
	req.Header.Set("Content-Type", "application/json")

	// 15% of requests use API key
	if rand.Float32() < 0.15 {
		req.Header.Set("X-API-Key", "ak_live_pro_vip_778899")
	}

	start := time.Now()
	rec := httptest.NewRecorder()

	s.handler.ServeHTTP(rec, req)
	durationMs := time.Since(start).Milliseconds()

	s.totalSent.Add(1)

	isThreat := rec.Code == http.StatusForbidden || rec.Code == http.StatusTooManyRequests
	reason := ""
	if rec.Code == http.StatusForbidden {
		reason = "WAF / GeoIP Perimeter Rule Triggered"
	} else if rec.Code == http.StatusTooManyRequests {
		reason = "Rate Limit Sliding-Window Quota Exceeded"
	}

	if s.onPing != nil {
		s.onPing(ThreatPing{
			ID:        fmt.Sprintf("ping_%d", time.Now().UnixNano()),
			Timestamp: start,
			IP:        origin.IP,
			Region:    origin.Region,
			Country:   origin.Country,
			Method:    method,
			Path:      endpoint,
			Status:    rec.Code,
			LatencyMs: durationMs,
			IsThreat:  isThreat,
			Reason:    reason,
		})
	}
}

// TriggerBurst sends an instant burst of requests concurrently.
func (s *TrafficSimulator) TriggerBurst(count int) {
	if count <= 0 {
		count = 50
	}
	if count > 200 {
		count = 200
	}

	s.totalBurst.Add(int64(count))

	go func() {
		for i := 0; i < count; i++ {
			go s.sendOneSimulatedRequest()
			if i%10 == 0 {
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()
}

// Pause stops sending simulated requests.
func (s *TrafficSimulator) Pause() {
	s.active.Store(false)
}

// Resume starts sending simulated requests again.
func (s *TrafficSimulator) Resume() {
	s.active.Store(true)
}

// SetRPS sets target requests per second.
func (s *TrafficSimulator) SetRPS(rps int) {
	if rps < 0 {
		rps = 0
	}
	if rps > 100 {
		rps = 100
	}
	s.targetRPS.Store(int32(rps))
}

// GetStatus returns the current simulation status.
func (s *TrafficSimulator) GetStatus() SimulatorStatus {
	return SimulatorStatus{
		Active:     s.active.Load(),
		TargetRPS:  int(s.targetRPS.Load()),
		TotalSent:  s.totalSent.Load(),
		TotalBurst: s.totalBurst.Load(),
	}
}
