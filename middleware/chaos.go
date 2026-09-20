package middleware

import (
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// ChaosConfig defines parameters for chaos engineering simulations.
type ChaosConfig struct {
	Enabled      bool  `json:"enabled"`
	LatencyMs    int64 `json:"latency_ms"`     // Artificial delay in milliseconds
	FaultRatePct int   `json:"fault_rate_pct"` // Percentage (0-100) of requests to fail with 500
}

// ChaosController manages real-time chaos injection for resilience testing.
type ChaosController struct {
	cfg ChaosConfig
	mu  sync.RWMutex
}

func NewChaosController() *ChaosController {
	return &ChaosController{
		cfg: ChaosConfig{
			Enabled:      false,
			LatencyMs:    0,
			FaultRatePct: 0,
		},
	}
}

func (c *ChaosController) GetConfig() ChaosConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cfg
}

func (c *ChaosController) UpdateConfig(cfg ChaosConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg = cfg
}

// Middleware injects synthetic delay and simulated downstream faults based on configuration.
func (c *ChaosController) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.mu.RLock()
		cfg := c.cfg
		c.mu.RUnlock()

		if !cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Inject artificial latency
		if cfg.LatencyMs > 0 {
			time.Sleep(time.Duration(cfg.LatencyMs) * time.Millisecond)
		}

		// Inject synthetic 500 fault based on probability
		if cfg.FaultRatePct > 0 {
			roll := rand.Intn(100)
			if roll < cfg.FaultRatePct {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Chaos-Injected", "true")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"chaos engineering: synthetic 500 internal server error injected","chaos":true}`))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
