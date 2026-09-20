package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds every Prometheus collector exposed by the gateway.
type Metrics struct {
	RequestsTotal     *prometheus.CounterVec
	RateLimitedTotal  *prometheus.CounterVec
	RequestDuration   *prometheus.HistogramVec
	RateLimiterErrors prometheus.Counter
}

// New registers and returns the gateway's metric collectors. Call this
// exactly once at startup.
func New() *Metrics {
	return &Metrics{
		RequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_requests_total",
			Help: "Total number of requests processed by the gateway, labeled by route, method and status code.",
		}, []string{"route", "method", "status"}),

		RateLimitedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_rate_limited_total",
			Help: "Total number of requests rejected due to rate limiting, labeled by route.",
		}, []string{"route"}),

		RequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gateway_request_duration_seconds",
			Help:    "Request latency distribution in seconds, labeled by route and method.",
			Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		}, []string{"route", "method"}),

		RateLimiterErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_rate_limiter_errors_total",
			Help: "Total number of errors encountered while evaluating rate limits (fail-open events).",
		}),
	}
}

func (m *Metrics) RecordRateLimited(route string) {
	m.RateLimitedTotal.WithLabelValues(route).Inc()
}

func (m *Metrics) RecordRateLimiterError() {
	m.RateLimiterErrors.Inc()
}

// statusRecorder wraps http.ResponseWriter to capture the status code
// written by downstream handlers, since net/http does not expose it.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Instrument wraps next with request-count and latency-histogram
// instrumentation. routeLabelFn should return the matched route's
// configured path prefix (not the raw request path) to keep label
// cardinality bounded.
func (m *Metrics) Instrument(routeLabelFn func(*http.Request) string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		duration := time.Since(start).Seconds()
		route := routeLabelFn(r)

		m.RequestsTotal.WithLabelValues(route, r.Method, strconv.Itoa(rec.status)).Inc()
		m.RequestDuration.WithLabelValues(route, r.Method).Observe(duration)
	})
}

// Handler returns the HTTP handler that exposes collected metrics in the
// Prometheus text exposition format, to be mounted at /metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}
