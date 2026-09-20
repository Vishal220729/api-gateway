package proxy

import (
	"math/rand"
	"sync"
	"sync/atomic"
)

// CanaryStatus represents live telemetry for A/B canary routing.
type CanaryStatus struct {
	Enabled        bool    `json:"enabled"`
	CanaryWeight   int     `json:"canary_weight"` // 0 - 100%
	StableWeight   int     `json:"stable_weight"` // 100 - canaryWeight
	StableUpstream string  `json:"stable_upstream"`
	CanaryUpstream string  `json:"canary_upstream"`
	StableRequests int64   `json:"stable_requests"`
	CanaryRequests int64   `json:"canary_requests"`
	StableErrors   int64   `json:"stable_errors"`
	CanaryErrors   int64   `json:"canary_errors"`
	CanaryErrorPct float64 `json:"canary_error_pct"`
}

// CanaryRouter manages dynamic A/B canary traffic splitting.
type CanaryRouter struct {
	canaryWeight   int32 // 0 to 100
	stableUpstream string
	canaryUpstream string
	stableRequests int64
	canaryRequests int64
	stableErrors   int64
	canaryErrors   int64
	mu             sync.RWMutex
}

func NewCanaryRouter(stable, canary string, defaultWeight int) *CanaryRouter {
	if defaultWeight < 0 {
		defaultWeight = 0
	}
	if defaultWeight > 100 {
		defaultWeight = 100
	}

	return &CanaryRouter{
		canaryWeight:   int32(defaultWeight),
		stableUpstream: stable,
		canaryUpstream: canary,
	}
}

// PickUpstream selects either stable or canary based on the configured weight.
func (c *CanaryRouter) PickUpstream() (string, bool) {
	weight := int(atomic.LoadInt32(&c.canaryWeight))
	if weight <= 0 {
		atomic.AddInt64(&c.stableRequests, 1)
		return c.stableUpstream, false
	}
	if weight >= 100 {
		atomic.AddInt64(&c.canaryRequests, 1)
		return c.canaryUpstream, true
	}

	roll := rand.Intn(100)
	if roll < weight {
		atomic.AddInt64(&c.canaryRequests, 1)
		return c.canaryUpstream, true
	}

	atomic.AddInt64(&c.stableRequests, 1)
	return c.stableUpstream, false
}

// RecordResult updates error metrics.
func (c *CanaryRouter) RecordResult(isCanary bool, isError bool) {
	if isCanary {
		if isError {
			atomic.AddInt64(&c.canaryErrors, 1)
		}
	} else {
		if isError {
			atomic.AddInt64(&c.stableErrors, 1)
		}
	}
}

// SetWeight updates the canary traffic percentage (0-100).
func (c *CanaryRouter) SetWeight(weight int) {
	if weight < 0 {
		weight = 0
	}
	if weight > 100 {
		weight = 100
	}
	atomic.StoreInt32(&c.canaryWeight, int32(weight))
}

// GetStatus returns the current canary configuration and statistics.
func (c *CanaryRouter) GetStatus() CanaryStatus {
	weight := int(atomic.LoadInt32(&c.canaryWeight))
	cReqs := atomic.LoadInt64(&c.canaryRequests)
	cErrs := atomic.LoadInt64(&c.canaryErrors)

	var errPct float64
	if cReqs > 0 {
		errPct = (float64(cErrs) / float64(cReqs)) * 100.0
	}

	return CanaryStatus{
		Enabled:        weight > 0,
		CanaryWeight:   weight,
		StableWeight:   100 - weight,
		StableUpstream: c.stableUpstream,
		CanaryUpstream: c.canaryUpstream,
		StableRequests: atomic.LoadInt64(&c.stableRequests),
		CanaryRequests: cReqs,
		StableErrors:   atomic.LoadInt64(&c.stableErrors),
		CanaryErrors:   cErrs,
		CanaryErrorPct: errPct,
	}
}
