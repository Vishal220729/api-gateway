package proxy

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vishalrr/api-gateway/config"
)

// Router forwards incoming requests to the configured upstream service
// based on the longest matching path prefix, reusing a cached
// httputil.ReverseProxy per upstream so no proxy is rebuilt per-request.
type Router struct {
	routes   []config.RouteConfig
	proxies  map[string]*httputil.ReverseProxy
	Breakers *CircuitBreakerManager
	Balancer *LoadBalancer
	mu       sync.RWMutex
}

// NewRouter builds a Router and eagerly constructs a reverse proxy for
// every configured upstream, failing fast on invalid upstream URLs.
func NewRouter(routes []config.RouteConfig) (*Router, error) {
	upstreams := make([]string, 0, len(routes))
	for _, rt := range routes {
		upstreams = append(upstreams, rt.Upstream)
	}

	r := &Router{
		routes:   routes,
		proxies:  make(map[string]*httputil.ReverseProxy),
		Breakers: NewCircuitBreakerManager(),
		Balancer: NewLoadBalancer(upstreams),
	}

	for _, rt := range routes {
		if _, err := r.getOrCreateProxy(rt.Upstream); err != nil {
			return nil, err
		}
	}

	return r, nil
}

// AddRoute dynamically inserts or updates a route configuration in memory.
func (r *Router) AddRoute(rt config.RouteConfig) error {
	if _, err := r.getOrCreateProxy(rt.Upstream); err != nil {
		return err
	}

	r.Balancer.AddTarget(rt.Upstream)

	r.mu.Lock()
	defer r.mu.Unlock()

	for i, existing := range r.routes {
		if existing.Path == rt.Path {
			r.routes[i] = rt
			return nil
		}
	}
	r.routes = append(r.routes, rt)
	return nil
}

// RemoveRoute dynamically removes a route by its path prefix.
func (r *Router) RemoveRoute(path string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, existing := range r.routes {
		if existing.Path == path {
			r.routes = append(r.routes[:i], r.routes[i+1:]...)
			return true
		}
	}
	return false
}

// GetRoutes returns a copy of all currently active routes.
func (r *Router) GetRoutes() []config.RouteConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	copied := make([]config.RouteConfig, len(r.routes))
	copy(copied, r.routes)
	return copied
}

func (r *Router) getOrCreateProxy(upstream string) (*httputil.ReverseProxy, error) {
	r.mu.RLock()
	if p, ok := r.proxies[upstream]; ok {
		r.mu.RUnlock()
		return p, nil
	}
	r.mu.RUnlock()

	target, err := url.Parse(upstream)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          10000,
		MaxIdleConnsPerHost:   2000,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
		req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
		req.Header.Set("X-Gateway", "distributed-rate-limiter-gateway")
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		log.Printf("proxy error for %s (upstream %s): %v", req.URL.Path, upstream, err)
		r.Breakers.RecordFailure(upstream)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error":"upstream service unavailable"}`))
	}

	proxy.ModifyResponse = func(res *http.Response) error {
		if res.StatusCode >= 500 {
			r.Breakers.RecordFailure(upstream)
		} else {
			r.Breakers.RecordSuccess(upstream)
		}
		return nil
	}

	r.mu.Lock()
	r.proxies[upstream] = proxy
	r.mu.Unlock()

	return proxy, nil
}

// MatchRoute returns the route configuration whose Path is the longest
// prefix match of the given request path, and whether a match was found.
func (r *Router) MatchRoute(path string) (config.RouteConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var best *config.RouteConfig
	bestLen := -1
	for i := range r.routes {
		rt := r.routes[i]
		if strings.HasPrefix(path, rt.Path) && len(rt.Path) > bestLen {
			best = &r.routes[i]
			bestLen = len(rt.Path)
		}
	}
	if best == nil {
		return config.RouteConfig{}, false
	}
	return *best, true
}

func methodAllowed(methods []string, method string) bool {
	if len(methods) == 0 {
		return true
	}
	for _, m := range methods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

// ServeHTTP implements http.Handler, forwarding the request to the matched
// upstream service via its cached reverse proxy with circuit breaker protection.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	route, ok := r.MatchRoute(req.URL.Path)
	if !ok {
		http.NotFound(w, req)
		return
	}

	if !methodAllowed(route.Methods, req.Method) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error":"method not allowed for this route"}`))
		return
	}

	// Circuit Breaker check before dispatching to upstream
	allowed, state, retryAfter := r.Breakers.Allow(route.Upstream)
	if !allowed {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(fmt.Sprintf(`{"error":"circuit breaker %s: upstream %s is failing","retry_after_seconds":%d}`, state, route.Upstream, retryAfter)))
		return
	}

	proxy, err := r.getOrCreateProxy(route.Upstream)
	if err != nil {
		http.Error(w, "invalid upstream configuration", http.StatusInternalServerError)
		return
	}

	proxy.ServeHTTP(w, req)
}
