package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	"github.com/vishalrr/api-gateway/ai"
	"github.com/vishalrr/api-gateway/cloud"
	"github.com/vishalrr/api-gateway/config"
	"github.com/vishalrr/api-gateway/limiter"
	"github.com/vishalrr/api-gateway/metrics"
	appmiddleware "github.com/vishalrr/api-gateway/middleware"
	"github.com/vishalrr/api-gateway/proxy"
	"github.com/vishalrr/api-gateway/realtime"
)

type ActiveBucket struct {
	Key        string `json:"key"`
	Count      int64  `json:"count"`
	TTLSeconds int64  `json:"ttl_seconds"`
}

type DashboardStats struct {
	RedisConnected     bool                 `json:"redis_connected"`
	TotalRequests      uint64               `json:"total_requests"`
	TotalRateLimited   uint64               `json:"total_rate_limited"`
	AvgLatency         string               `json:"avg_latency"`
	ActiveBucketsCount int                  `json:"active_buckets_count"`
	ActiveBuckets      []ActiveBucket       `json:"active_buckets"`
	Routes             []config.RouteConfig `json:"routes"`
}

type TrafficLog struct {
	ID            int64     `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	CorrelationID string    `json:"correlation_id"`
	Method        string    `json:"method"`
	Path          string    `json:"path"`
	ClientIP      string    `json:"client_ip"`
	Status        int       `json:"status"`
	DurationMs    int64     `json:"duration_ms"`
	Limit         int       `json:"limit"`
	Remaining     int       `json:"remaining"`
	CacheStatus   string    `json:"cache_status"`
	Tier          string    `json:"tier"`
}

type TrafficBuffer struct {
	logs []TrafficLog
	cap  int
	seq  int64
	mu   sync.RWMutex
}

func NewTrafficBuffer(capacity int) *TrafficBuffer {
	return &TrafficBuffer{
		logs: make([]TrafficLog, 0, capacity),
		cap:  capacity,
	}
}

func (tb *TrafficBuffer) Add(log TrafficLog) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.seq++
	log.ID = tb.seq
	if len(tb.logs) >= tb.cap {
		tb.logs = tb.logs[1:]
	}
	tb.logs = append(tb.logs, log)
}

func (tb *TrafficBuffer) List() []TrafficLog {
	tb.mu.RLock()
	defer tb.mu.RUnlock()
	out := make([]TrafficLog, len(tb.logs))
	for i := range tb.logs {
		out[len(tb.logs)-1-i] = tb.logs[i] // Newest first
	}
	return out
}

type SSEBroker struct {
	clients map[chan string]bool
	mu      sync.RWMutex
}

func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients: make(map[chan string]bool),
	}
}

func (b *SSEBroker) AddClient() chan string {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan string, 20)
	b.clients[ch] = true
	return ch
}

func (b *SSEBroker) RemoveClient(ch chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, ch)
	close(ch)
}

func (b *SSEBroker) Broadcast(msg string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (b *SSEBroker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

var (
	totalRequestsCounter    atomic.Uint64
	totalRateLimitedCounter atomic.Uint64
)

type statsRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statsRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		PoolSize:     1000,
		MinIdleConns: 100,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  100 * time.Millisecond,
		WriteTimeout: 100 * time.Millisecond,
	})

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		log.Fatalf("failed to connect to redis at %s: %v", cfg.RedisAddr, err)
	}
	log.Printf("connected to redis at %s", cfg.RedisAddr)

	m := metrics.New()
	swLimiter := limiter.NewSlidingWindowLimiter(redisClient)
	rateLimiter := appmiddleware.NewRateLimiter(swLimiter, cfg.Routes, m, cfg.DefaultRateLimit, cfg.DefaultWindowSec)

	router, err := proxy.NewRouter(cfg.Routes)
	if err != nil {
		log.Fatalf("failed to initialize router: %v", err)
	}

	wafEngine := appmiddleware.NewWAFEngine()
	// Seed sample rules for demonstration
	wafEngine.AddRule("198.51.100.1", "blacklist", "Known bot/crawler IP")
	wafEngine.AddRule("10.0.0.1", "whitelist", "Internal VIP Service (Rate-limit bypass)")

	trafficBuffer := NewTrafficBuffer(50)
	sseBroker := NewSSEBroker()

	apiKeyStore := appmiddleware.NewAPIKeyStore(redisClient)
	httpCache := appmiddleware.NewHTTPCache(redisClient, 30*time.Second)
	chaosController := appmiddleware.NewChaosController()
	geoIPFilter := appmiddleware.NewGeoIPFilter()
	bodyLimitMiddleware := appmiddleware.BodySizeLimitMiddleware(1024 * 1024)

	cloudWebhookMgr := cloud.NewCloudWebhookManager()
	cloudArchiver := cloud.NewCloudArchiver()
	cloudRegionMgr := cloud.NewCloudRegionManager()

	aiCopilot := ai.NewAICopilotEngine()
	aiGatewayProxy := ai.NewAIGatewayProxy(redisClient)
	aiAnomalyDetector := ai.NewAIAnomalyDetector()
	canaryRouter := proxy.NewCanaryRouter("http://localhost:9001", "http://localhost:9003", 20)

	routeLabelFn := func(r *http.Request) string {
		return rateLimiter.MatchRoute(r.URL.Path).Path
	}

	// Middleware chain: Tracing -> Security -> BodyLimit -> GeoIP -> WAF -> CORS -> APIKey -> RateLimit -> perRouteAuth -> HTTPCache -> Chaos -> Prometheus -> ReverseProxy
	var handler http.Handler = router
	handler = m.Instrument(routeLabelFn, handler)
	handler = chaosController.Middleware(handler)
	handler = httpCache.Middleware(handler)
	handler = perRouteAuth(cfg, rateLimiter, handler)
	handler = rateLimiter.Middleware(handler)
	handler = appmiddleware.APIKeyMiddleware(apiKeyStore)(handler)
	handler = appmiddleware.CORS(cfg.CORSAllowedOrigins)(handler)
	handler = wafEngine.Middleware(handler)
	handler = geoIPFilter.Middleware(handler)
	handler = bodyLimitMiddleware(handler)
	handler = appmiddleware.SecurityHeadersMiddleware()(handler)
	handler = appmiddleware.TracingMiddleware()(handler)

	// Autonomous Real-Time Traffic Simulator
	var simulator *realtime.TrafficSimulator
	simulator = realtime.NewTrafficSimulator(http.HandlerFunc(func(sw http.ResponseWriter, sr *http.Request) {
		rec := &statsRecorder{ResponseWriter: sw, status: http.StatusOK}
		handler.ServeHTTP(rec, sr)
		totalRequestsCounter.Add(1)
		if rec.status == http.StatusTooManyRequests {
			totalRateLimitedCounter.Add(1)
		}
		lim, _ := strconv.Atoi(rec.Header().Get("X-RateLimit-Limit"))
		rem, _ := strconv.Atoi(rec.Header().Get("X-RateLimit-Remaining"))
		trafficLog := TrafficLog{
			Timestamp:     time.Now(),
			CorrelationID: fmt.Sprintf("sim-%d", time.Now().UnixNano()),
			Method:        sr.Method,
			Path:          sr.URL.Path,
			ClientIP:      appmiddleware.ExtractIP(sr),
			Status:        rec.status,
			DurationMs:    1,
			Limit:         lim,
			Remaining:     rem,
			CacheStatus:   rec.Header().Get("X-Cache"),
			Tier:          rec.Header().Get("X-API-Key-Tier"),
		}
		trafficBuffer.Add(trafficLog)
		if logBytes, err := json.Marshal(trafficLog); err == nil {
			sseBroker.Broadcast(fmt.Sprintf("event: traffic\ndata: %s\n\n", string(logBytes)))
		}
	}), func(ping realtime.ThreatPing) {
		if pingBytes, err := json.Marshal(ping); err == nil {
			sseBroker.Broadcast(fmt.Sprintf("event: threat\ndata: %s\n\n", string(pingBytes)))
		}
	})
	simulator.Start()

	// 500ms Sub-Second SSE Telemetry Broadcaster
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		var lastTotal uint64
		for range ticker.C {
			currTotal := totalRequestsCounter.Load()
			diff := currTotal - lastTotal
			lastTotal = currTotal
			rps := float64(diff) * 2.0 // 500ms interval -> RPS

			statsPayload := map[string]interface{}{
				"timestamp":          time.Now().UnixMilli(),
				"rps":                rps,
				"total_requests":     currTotal,
				"total_rate_limited": totalRateLimitedCounter.Load(),
				"connected_clients":  sseBroker.ClientCount(),
				"simulator":          simulator.GetStatus(),
				"canary":             canaryRouter.GetStatus(),
			}
			if b, err := json.Marshal(statsPayload); err == nil {
				sseBroker.Broadcast(fmt.Sprintf("event: stats\ndata: %s\n\n", string(b)))
			}
		}
	}()

	mux := http.NewServeMux()

	// Web Dashboard Static Assets
	fileServer := http.FileServer(http.Dir("web"))

	// Dashboard Live Stats API
	mux.HandleFunc("/api/dashboard/stats", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var buckets []ActiveBucket
		keys, err := redisClient.Keys(ctx, "ratelimit:*").Result()
		if err == nil {
			for _, k := range keys {
				count, _ := redisClient.Get(ctx, k).Int64()
				ttl, _ := redisClient.TTL(ctx, k).Result()
				ttlSec := int64(ttl.Seconds())
				if ttlSec < 0 {
					ttlSec = 0
				}
				buckets = append(buckets, ActiveBucket{
					Key:        k,
					Count:      count,
					TTLSeconds: ttlSec,
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DashboardStats{
			RedisConnected:     redisClient.Ping(ctx).Err() == nil,
			TotalRequests:      totalRequestsCounter.Load(),
			TotalRateLimited:   totalRateLimitedCounter.Load(),
			AvgLatency:         "< 1 ms",
			ActiveBucketsCount: len(buckets),
			ActiveBuckets:      buckets,
			Routes:             router.GetRoutes(),
		})
	})

	// 1-Click Rate Limit Reset: Flushes all Redis buckets
	mux.HandleFunc("/api/dashboard/reset-buckets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		keys, _ := redisClient.Keys(ctx, "ratelimit:*").Result()
		deleted := 0
		if len(keys) > 0 {
			redisClient.Del(ctx, keys...)
			deleted = len(keys)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"deleted": deleted,
			"message": "All Redis rate-limiting buckets flushed successfully",
		})
	})

	// Live Traffic Stream API
	mux.HandleFunc("/api/dashboard/traffic", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(trafficBuffer.List())
	})

	// WAF Management API
	mux.HandleFunc("/api/dashboard/waf", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(wafEngine.ListRules())
		case http.MethodPost:
			var body struct {
				IP     string `json:"ip"`
				Type   string `json:"type"`
				Reason string `json:"reason"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.IP == "" {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			if body.Type != "blacklist" && body.Type != "whitelist" {
				body.Type = "blacklist"
			}
			wafEngine.AddRule(body.IP, body.Type, body.Reason)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "rule": body})
		case http.MethodDelete:
			ip := r.URL.Query().Get("ip")
			if ip == "" {
				http.Error(w, "ip parameter required", http.StatusBadRequest)
				return
			}
			wafEngine.RemoveRule(ip)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "removed": ip})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Dynamic Route Management API
	mux.HandleFunc("/api/dashboard/routes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(router.GetRoutes())
		case http.MethodPost:
			var newRoute config.RouteConfig
			if err := json.NewDecoder(r.Body).Decode(&newRoute); err != nil || newRoute.Path == "" || newRoute.Upstream == "" {
				http.Error(w, "invalid route body", http.StatusBadRequest)
				return
			}
			if newRoute.RateLimit <= 0 {
				newRoute.RateLimit = 100
			}
			if newRoute.WindowSec <= 0 {
				newRoute.WindowSec = 60
			}
			if newRoute.KeyType == "" {
				newRoute.KeyType = "ip"
			}
			if len(newRoute.Methods) == 0 {
				newRoute.Methods = []string{"GET", "POST"}
			}

			if err := router.AddRoute(newRoute); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			rateLimiter.AddRoute(newRoute)

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "route": newRoute})
		case http.MethodDelete:
			path := r.URL.Query().Get("path")
			if path == "" {
				http.Error(w, "path parameter required", http.StatusBadRequest)
				return
			}
			router.RemoveRoute(path)
			rateLimiter.RemoveRoute(path)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "removed": path})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Circuit Breakers Status & Management API
	mux.HandleFunc("/api/dashboard/circuits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(router.Breakers.ListStatuses())
	})
	mux.HandleFunc("/api/dashboard/circuits/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		upstream := r.URL.Query().Get("upstream")
		if upstream != "" {
			router.Breakers.Reset(upstream)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "reset": upstream})
	})
	mux.HandleFunc("/api/dashboard/circuits/trip", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		upstream := r.URL.Query().Get("upstream")
		if upstream != "" {
			router.Breakers.Trip(upstream)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "tripped": upstream})
	})

	// Dashboard JWT Token Generator API
	mux.HandleFunc("/api/dashboard/token", func(w http.ResponseWriter, r *http.Request) {
		clientID := "client-vip-42"
		var reqBody struct {
			ClientID string `json:"client_id"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&reqBody)
			if reqBody.ClientID != "" {
				clientID = reqBody.ClientID
			}
		}
		if q := r.URL.Query().Get("client_id"); q != "" {
			clientID = q
		}

		claims := jwt.MapClaims{
			"client_id": clientID,
			"sub":       clientID,
			"exp":       time.Now().Add(24 * time.Hour).Unix(),
			"iat":       time.Now().Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString([]byte(cfg.JWTSecret))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token":     signed,
			"client_id": clientID,
		})
	})

	// Upstream Load Balancer Nodes Health & Latency API
	mux.HandleFunc("/api/dashboard/upstreams", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(router.Balancer.GetNodeStatuses())
	})

	// API Keys Management API
	mux.HandleFunc("/api/dashboard/apikeys", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(apiKeyStore.List())
		case http.MethodPost:
			var body struct {
				Owner string `json:"owner"`
				Tier  string `json:"tier"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Owner == "" {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			ak, err := apiKeyStore.Create(body.Owner, body.Tier)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(ak)
		case http.MethodDelete:
			key := r.URL.Query().Get("key")
			if key == "" {
				http.Error(w, "key parameter required", http.StatusBadRequest)
				return
			}
			apiKeyStore.Revoke(key)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "revoked": key})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Redis HTTP Cache Purge API
	mux.HandleFunc("/api/dashboard/cache/purge", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Query().Get("path")
		var count int64
		var err error
		if path != "" {
			err = httpCache.Purge(path)
			count = 1
		} else {
			count, err = httpCache.FlushAll()
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "purged": count})
	})

	// Server-Sent Events (SSE) Real-Time Stream API
	mux.HandleFunc("/api/dashboard/stream", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		ch := sseBroker.AddClient()
		defer sseBroker.RemoveClient(ch)

		fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
		flusher.Flush()

		notify := r.Context().Done()
		for {
			select {
			case <-notify:
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprint(w, msg)
				flusher.Flush()
			}
		}
	})

	// Chaos Engineering Management API
	mux.HandleFunc("/api/dashboard/chaos", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(chaosController.GetConfig())
		case http.MethodPost:
			var body appmiddleware.ChaosConfig
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			chaosController.UpdateConfig(body)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "config": body})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// GeoIP Country Rules Management API
	mux.HandleFunc("/api/dashboard/geoip", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(geoIPFilter.ListRules())
		case http.MethodPost:
			var body struct {
				CountryCode string `json:"country_code"`
				Action      string `json:"action"`
				Reason      string `json:"reason"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CountryCode == "" {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			if body.Action != "block" && body.Action != "allow" {
				body.Action = "block"
			}
			geoIPFilter.AddRule(body.CountryCode, body.Action, body.Reason)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "rule": body})
		case http.MethodDelete:
			country := r.URL.Query().Get("country")
			if country == "" {
				http.Error(w, "country parameter required", http.StatusBadRequest)
				return
			}
			geoIPFilter.RemoveRule(country)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "removed": country})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Cloud Integration APIs
	mux.HandleFunc("/api/dashboard/cloud/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"regions":        cloudRegionMgr.GetRegions(),
			"archives":       cloudArchiver.ListArchives(),
			"webhooks":       cloudWebhookMgr.ListWebhooks(),
			"webhooks_count": len(cloudWebhookMgr.ListWebhooks()),
		})
	})

	mux.HandleFunc("/api/dashboard/cloud/webhooks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(cloudWebhookMgr.ListWebhooks())
		case http.MethodPost:
			var wh cloud.CloudWebhook
			if err := json.NewDecoder(r.Body).Decode(&wh); err != nil || wh.URL == "" {
				http.Error(w, "invalid webhook payload", http.StatusBadRequest)
				return
			}
			if wh.Name == "" {
				wh.Name = "Custom Cloud Webhook"
			}
			if len(wh.Events) == 0 {
				wh.Events = []string{"*"}
			}
			if wh.Secret == "" {
				wh.Secret = fmt.Sprintf("whsec_%d", time.Now().UnixNano())
			}
			cloudWebhookMgr.AddWebhook(&wh)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(wh)
		case http.MethodDelete:
			id := r.URL.Query().Get("id")
			if id == "" {
				http.Error(w, "id parameter required", http.StatusBadRequest)
				return
			}
			cloudWebhookMgr.RemoveWebhook(id)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "removed": id})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/dashboard/cloud/test-webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		dispatched := len(cloudWebhookMgr.ListWebhooks())
		cloudWebhookMgr.DispatchAlert("rate_limit_spike", map[string]interface{}{
			"message": "Manual test alert from API Gateway Cloud Integration Hub",
			"status":  "healthy",
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"message":    "Test alert dispatched to registered cloud webhooks",
			"dispatched": dispatched,
		})
	})

	mux.HandleFunc("/api/dashboard/cloud/archive", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		logs := trafficBuffer.List()
		bundle := cloudArchiver.CreateArchive(len(logs))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      true,
			"archive":      bundle,
			"bucket":       "api-gateway-traffic-logs",
			"file_name":    bundle.ID + ".json.gz",
			"record_count": bundle.RecordCount,
			"size_bytes":   bundle.SizeBytes,
			"compression":  "GZIP (JSONL)",
			"s3_uri":       bundle.S3URI,
		})
	})

	// AI LLM Proxy & Semantic Gateway API
	mux.HandleFunc("/api/ai/chat", aiGatewayProxy.HandleChat)

	// AI Copilot Directive & Incident Diagnostics API
	mux.HandleFunc("/api/dashboard/ai/copilot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Prompt) == "" {
			http.Error(w, "prompt cannot be empty", http.StatusBadRequest)
			return
		}
		res := aiCopilot.ProcessDirective(req.Prompt, wafEngine, geoIPFilter, router.Breakers)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	})

	mux.HandleFunc("/api/dashboard/ai/diagnose", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		keys, _ := redisClient.Keys(ctx, "ratelimit:*").Result()
		diag := aiCopilot.RunDiagnostics(
			int64(totalRequestsCounter.Load()),
			int64(totalRateLimitedCounter.Load()),
			float64(simulator.GetStatus().TargetRPS),
			router.Breakers.ListStatuses(),
			len(keys),
		)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(diag)
	})

	// AI Auto-Ban Anomaly Detection API
	mux.HandleFunc("/api/dashboard/security/autoban", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(aiAnomalyDetector.ListBanned())
		case http.MethodPost:
			ip := r.URL.Query().Get("ip")
			if ip == "" {
				var body struct {
					IP string `json:"ip"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				ip = body.IP
			}
			if ip == "" {
				http.Error(w, "ip parameter required", http.StatusBadRequest)
				return
			}
			success := aiAnomalyDetector.Unban(ip)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": success, "unbanned_ip": ip})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Canary A/B Traffic Routing API
	mux.HandleFunc("/api/dashboard/routes/canary", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(canaryRouter.GetStatus())
		case http.MethodPost:
			var body struct {
				Weight int `json:"weight"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			canaryRouter.SetWeight(body.Weight)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(canaryRouter.GetStatus())
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Autonomous Real-Time Traffic Simulator Control API
	mux.HandleFunc("/api/dashboard/simulator", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(simulator.GetStatus())
		case http.MethodPost:
			var body struct {
				Action string `json:"action"` // "pause", "resume", "set_rps", "burst"
				RPS    int    `json:"rps,omitempty"`
				Count  int    `json:"count,omitempty"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			switch body.Action {
			case "pause":
				simulator.Pause()
			case "resume":
				simulator.Resume()
			case "set_rps":
				simulator.SetRPS(body.RPS)
			case "burst":
				simulator.TriggerBurst(body.Count)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(simulator.GetStatus())
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Root Handler: Serves Web UI for all dashboard HTML pages, otherwise dispatches to Gateway Proxy Chain
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" || strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".svg") || strings.HasSuffix(path, ".ico") || strings.HasSuffix(path, ".webp") || strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".webmanifest") {
			fileServer.ServeHTTP(w, r)
			return
		}

		clientIP := appmiddleware.ExtractIP(r)
		if isBanned, ttlSec := aiAnomalyDetector.IsBanned(clientIP); isBanned {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-AI-Anomaly", "AUTO_BANNED")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":             "IP_AUTO_BANNED",
				"status":            403,
				"reason":            "AI Anomaly Detector: IP quarantined for abusive traffic pattern",
				"ttl_remaining_sec": ttlSec,
			})
			return
		}

		start := time.Now()
		totalRequestsCounter.Add(1)
		rec := &statsRecorder{ResponseWriter: w, status: http.StatusOK}
		handler.ServeHTTP(rec, r)
		if rec.status == http.StatusTooManyRequests {
			totalRateLimitedCounter.Add(1)
		}

		// AI Anomaly Tracking
		if newlyBanned, banEntry := aiAnomalyDetector.RecordRequest(clientIP, rec.status); newlyBanned && banEntry != nil {
			if banBytes, err := json.Marshal(banEntry); err == nil {
				sseBroker.Broadcast(fmt.Sprintf("event: alert\ndata: %s\n\n", string(banBytes)))
			}
		}

		durationMs := time.Since(start).Milliseconds()
		lim, _ := strconv.Atoi(rec.Header().Get("X-RateLimit-Limit"))
		rem, _ := strconv.Atoi(rec.Header().Get("X-RateLimit-Remaining"))
		corrID := rec.Header().Get(appmiddleware.CorrelationIDHeader)
		cacheStatus := rec.Header().Get("X-Cache")
		tier := rec.Header().Get("X-API-Key-Tier")

		trafficLog := TrafficLog{
			Timestamp:     start,
			CorrelationID: corrID,
			Method:        r.Method,
			Path:          r.URL.Path,
			ClientIP:      clientIP,
			Status:        rec.status,
			DurationMs:    durationMs,
			Limit:         lim,
			Remaining:     rem,
			CacheStatus:   cacheStatus,
			Tier:          tier,
		}

		trafficBuffer.Add(trafficLog)

		// Broadcast to SSE clients instantly
		if logBytes, err := json.Marshal(trafficLog); err == nil {
			sseBroker.Broadcast(fmt.Sprintf("event: traffic\ndata: %s\n\n", string(logBytes)))
		}
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		log.Printf("api gateway listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutdown signal received, draining connections...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	if err := redisClient.Close(); err != nil {
		log.Printf("error closing redis client: %v", err)
	}
	log.Println("server stopped cleanly")
}

func perRouteAuth(cfg *config.Config, rl *appmiddleware.RateLimiter, next http.Handler) http.Handler {
	requiredAuth := appmiddleware.JWTAuth(cfg.JWTSecret, true)(next)
	optionalAuth := appmiddleware.JWTAuth(cfg.JWTSecret, false)(next)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := rl.MatchRoute(r.URL.Path)
		if route.RequireAuth {
			requiredAuth.ServeHTTP(w, r)
			return
		}
		optionalAuth.ServeHTTP(w, r)
	})
}
