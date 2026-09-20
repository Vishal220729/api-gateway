package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

const (
	CorrelationIDHeader     = "X-Correlation-ID"
	CorrelationIDContextKey = contextKey("correlation_id")
	TraceTimingContextKey   = contextKey("trace_timing")
)

// TraceTiming holds granular latency milestones across the gateway pipeline.
type TraceTiming struct {
	StartTime  time.Time `json:"start_time"`
	WAFMs      int64     `json:"waf_ms"`
	LimiterMs  int64     `json:"limiter_ms"`
	UpstreamMs int64     `json:"upstream_ms"`
	TotalMs    int64     `json:"total_ms"`
}

// GenerateCorrelationID creates a unique request correlation ID.
func GenerateCorrelationID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("req_%d_%s", time.Now().UnixMilli()%1000000, hex.EncodeToString(b)[:8])
}

// TracingMiddleware injects or propagates X-Correlation-ID and tracks pipeline timings.
func TracingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			corrID := r.Header.Get(CorrelationIDHeader)
			if corrID == "" {
				corrID = GenerateCorrelationID()
			}

			w.Header().Set(CorrelationIDHeader, corrID)

			timing := &TraceTiming{
				StartTime: time.Now(),
			}

			ctx := context.WithValue(r.Context(), CorrelationIDContextKey, corrID)
			ctx = context.WithValue(ctx, TraceTimingContextKey, timing)

			next.ServeHTTP(w, r.WithContext(ctx))

			timing.TotalMs = time.Since(timing.StartTime).Milliseconds()
		})
	}
}
