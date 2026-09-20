package middleware

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vishalrr/api-gateway/config"
	"github.com/vishalrr/api-gateway/limiter"
	"github.com/vishalrr/api-gateway/metrics"
)

type contextKey string

// ClientIDContextKey is the request-context key under which an
// authenticated client's identifier (from a validated JWT) is stored.
const ClientIDContextKey contextKey = "client_id"

// RateLimiter wraps requests with sliding-window rate limiting, selecting
// the applicable policy by matching the request path against the longest
// configured route prefix.
type RateLimiter struct {
	limiter      *limiter.SlidingWindowLimiter
	routes       []config.RouteConfig
	metrics      *metrics.Metrics
	defaultRoute config.RouteConfig
	mu           sync.RWMutex
}

func NewRateLimiter(l *limiter.SlidingWindowLimiter, routes []config.RouteConfig, m *metrics.Metrics, defaultLimit, defaultWindow int) *RateLimiter {
	return &RateLimiter{
		limiter: l,
		routes:  routes,
		metrics: m,
		defaultRoute: config.RouteConfig{
			Path:      "/",
			RateLimit: defaultLimit,
			WindowSec: defaultWindow,
			KeyType:   "ip",
		},
	}
}

// AddRoute dynamically adds or updates a route configuration in the rate limiter.
func (rl *RateLimiter) AddRoute(rt config.RouteConfig) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for i, existing := range rl.routes {
		if existing.Path == rt.Path {
			rl.routes[i] = rt
			return
		}
	}
	rl.routes = append(rl.routes, rt)
}

// RemoveRoute dynamically removes a route configuration from the rate limiter.
func (rl *RateLimiter) RemoveRoute(path string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for i, existing := range rl.routes {
		if existing.Path == path {
			rl.routes = append(rl.routes[:i], rl.routes[i+1:]...)
			return true
		}
	}
	return false
}

// MatchRoute returns the most specific (longest path prefix) route
// configuration for the given request path, falling back to the default
// policy if nothing matches.
func (rl *RateLimiter) MatchRoute(path string) config.RouteConfig {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	var best *config.RouteConfig
	bestLen := -1
	for i := range rl.routes {
		r := rl.routes[i]
		if strings.HasPrefix(path, r.Path) && len(r.Path) > bestLen {
			best = &rl.routes[i]
			bestLen = len(r.Path)
		}
	}
	if best == nil {
		return rl.defaultRoute
	}
	return *best
}

// clientIdentifier derives the rate-limit bucket key for a request: the
// authenticated client ID when the route is keyed by client_id (falling
// back to an X-Client-Id header), otherwise the caller's IP address.
func clientIdentifier(r *http.Request, keyType string) string {
	if keyType == "client_id" {
		if cid, ok := r.Context().Value(ClientIDContextKey).(string); ok && cid != "" {
			return "client:" + cid
		}
		if cid := r.Header.Get("X-Client-Id"); cid != "" {
			return "client:" + cid
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		host = strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	return "ip:" + host
}

// Middleware enforces the sliding-window rate limit for the matched route
// before delegating to next. On Redis failure it fails open (allows the
// request) rather than taking the entire gateway down, while recording the
// error for observability.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// WAF Whitelist Bypass: Whitelisted IPs bypass rate limits
		if val := r.Context().Value(WAFWhitelistedContextKey); val != nil {
			w.Header().Set("X-RateLimit-Bypass", "WAF-Whitelisted")
			next.ServeHTTP(w, r)
			return
		}

		route := rl.MatchRoute(r.URL.Path)
		limit := route.RateLimit
		window := route.WindowSec
		key := route.Path + ":" + clientIdentifier(r, route.KeyType)

		// API Key Tier Override: If request has a valid API key, enforce tier quota
		if akVal := r.Context().Value(APIKeyContextKey); akVal != nil {
			if ak, ok := akVal.(*APIKey); ok {
				limit = ak.RateLimit
				window = ak.WindowSec
				key = "apikey:" + ak.Key
				w.Header().Set("X-API-Key-Tier", ak.Tier)
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 50*time.Millisecond)
		defer cancel()

		result, err := rl.limiter.Allow(ctx, key, limit, window)
		if err != nil {
			rl.metrics.RecordRateLimiterError()
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))

		if !result.Allowed {
			retrySeconds := int(result.RetryAfter.Seconds()) + 1
			w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
			w.Header().Set("Content-Type", "application/json")
			rl.metrics.RecordRateLimited(route.Path)
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded","retry_after_seconds":` + strconv.Itoa(retrySeconds) + `}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
