package proxy

import (
	"sync"
	"time"
)

// CircuitState represents the current state of a circuit breaker.
type CircuitState string

const (
	StateClosed   CircuitState = "CLOSED"
	StateOpen     CircuitState = "OPEN"
	StateHalfOpen CircuitState = "HALF-OPEN"
)

// CircuitStatus provides telemetry for an upstream's circuit breaker.
type CircuitStatus struct {
	Upstream        string       `json:"upstream"`
	State           CircuitState `json:"state"`
	Failures        int          `json:"consecutive_failures"`
	LastFailureTime *time.Time   `json:"last_failure_time,omitempty"`
	RetryAfterSec   int          `json:"retry_after_sec"`
}

type circuit struct {
	upstream        string
	state           CircuitState
	failures        int
	threshold       int
	cooldown        time.Duration
	openedAt        time.Time
	lastFailureTime time.Time
	mu              sync.Mutex
}

// CircuitBreakerManager manages circuit breakers across all upstream services.
type CircuitBreakerManager struct {
	circuits map[string]*circuit
	mu       sync.RWMutex
}

// NewCircuitBreakerManager creates a new circuit breaker manager.
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		circuits: make(map[string]*circuit),
	}
}

func (m *CircuitBreakerManager) getOrCreate(upstream string) *circuit {
	m.mu.RLock()
	c, ok := m.circuits[upstream]
	m.mu.RUnlock()
	if ok {
		return c
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.circuits[upstream]; ok {
		return c
	}

	c = &circuit{
		upstream:  upstream,
		state:     StateClosed,
		threshold: 5,
		cooldown:  10 * time.Second,
	}
	m.circuits[upstream] = c
	return c
}

// Allow evaluates if a request is permitted to proceed to the upstream.
func (m *CircuitBreakerManager) Allow(upstream string) (bool, CircuitState, int) {
	c := m.getOrCreate(upstream)
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if c.state == StateOpen {
		elapsed := now.Sub(c.openedAt)
		if elapsed >= c.cooldown {
			// Cooldown expired: probe recovery with 1 trial request
			c.state = StateHalfOpen
			return true, StateHalfOpen, 0
		}
		retryAfter := int((c.cooldown - elapsed).Seconds()) + 1
		return false, StateOpen, retryAfter
	}

	return true, c.state, 0
}

// RecordSuccess records a successful upstream response.
func (m *CircuitBreakerManager) RecordSuccess(upstream string) {
	c := m.getOrCreate(upstream)
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failures = 0
	if c.state == StateHalfOpen {
		c.state = StateClosed
	}
}

// RecordFailure records an upstream connection or 5xx failure.
func (m *CircuitBreakerManager) RecordFailure(upstream string) {
	c := m.getOrCreate(upstream)
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failures++
	c.lastFailureTime = time.Now()

	if c.state == StateHalfOpen || c.failures >= c.threshold {
		c.state = StateOpen
		c.openedAt = time.Now()
	}
}

// Reset manually resets a circuit back to CLOSED.
func (m *CircuitBreakerManager) Reset(upstream string) {
	c := m.getOrCreate(upstream)
	c.mu.Lock()
	defer c.mu.Unlock()

	c.state = StateClosed
	c.failures = 0
}

// Trip manually trips a circuit to OPEN (for testing/maintenance).
func (m *CircuitBreakerManager) Trip(upstream string) {
	c := m.getOrCreate(upstream)
	c.mu.Lock()
	defer c.mu.Unlock()

	c.state = StateOpen
	c.openedAt = time.Now()
}

// ListStatuses returns the status of all tracked circuit breakers.
func (m *CircuitBreakerManager) ListStatuses() []CircuitStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var statuses []CircuitStatus
	now := time.Now()
	for _, c := range m.circuits {
		c.mu.Lock()
		retryAfter := 0
		if c.state == StateOpen {
			elapsed := now.Sub(c.openedAt)
			if elapsed < c.cooldown {
				retryAfter = int((c.cooldown - elapsed).Seconds()) + 1
			}
		}

		var lastFail *time.Time
		if !c.lastFailureTime.IsZero() {
			t := c.lastFailureTime
			lastFail = &t
		}

		statuses = append(statuses, CircuitStatus{
			Upstream:        c.upstream,
			State:           c.state,
			Failures:        c.failures,
			LastFailureTime: lastFail,
			RetryAfterSec:   retryAfter,
		})
		c.mu.Unlock()
	}
	return statuses
}
