# Distributed Rate Limiter & API Gateway - Technical Specifications

This document outlines the complete architectural, algorithmic, network, concurrency, security, and performance specifications of the API Gateway.

---

## 1. System Overview & Architecture

```
                                    ┌─────────────────────┐
                                    │    Clients / Apps   │
                                    └──────────┬──────────┘
                                               │ HTTPS / HTTP
                                               ▼
                         ┌──────────────────────────────────────────┐
                         │          API GATEWAY (Go 1.22)           │
                         │                                          │
                         │   ┌────────────┐   ┌────────────────┐    │
    /metrics  ◄───────── │   │   CORS     │──▶│    JWT Auth    │    │
    (Prometheus)         │   │ Middleware │   │  (per-route)   │    │
                         │   └────────────┘   └────────┬───────┘    │
                         │                             ▼            │
                         │                   ┌──────────────────┐   │
    /dashboard ◄──────── │                   │   Rate Limiter   │   │
    (Web UI)             │                   │    Middleware    │   │
                         │                   └────────┬─────────┘   │
                         │                            │ EVALSHA     │
                         │                            ▼             │
                         │                   ┌──────────────────┐   │
                         │                   │ Prometheus Metric│   │
                         │                   │  Instrumentation │   │
                         │                   └────────┬─────────┘   │
                         │                            ▼             │
                         │                   ┌──────────────────┐   │
                         │                   │  Reverse Proxy   │   │
                         │                   │ (Cached Transport│   │
                         │                   └────────┬─────────┘   │
                         └────────────────────────────┼─────────────┘
                                                      │
                      ┌───────────────────────────────┼───────────────────────────────┐
                      ▼                               ▼                               ▼
             ┌────────────────┐              ┌────────────────┐              ┌────────────────┐
             │ Redis Cluster  │              │ users-service  │              │ orders-service │
             │  (Port 6379)   │              │  (Port 9001)   │              │  (Port 9002)   │
             │ Sliding Window │              └────────────────┘              └────────────────┘
             └────────────────┘
```

---

## 2. Sliding Window Counter Algorithm Specification

### Mathematical Formulation
Rather than storing memory-expensive timestamps per request ($O(N)$ space) or permitting boundary burst spikes (Fixed Window), the gateway computes a smooth sliding approximation using two consecutive fixed-window buckets:

$$\text{estimated\_count} = (\text{previous\_count} \times \text{weight}) + \text{current\_count}$$

$$\text{weight} = \max\left(0, \frac{\text{window\_ms} - \text{elapsed\_in\_current\_ms}}{\text{window\_ms}}\right)$$

### Atomic Redis Lua Script Execution
To prevent race conditions across distributed gateway instances, the algorithm executes atomically inside Redis in a single round-trip:

```lua
local current_key = KEYS[1]
local previous_key = KEYS[2]
local limit = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])

local current_window_start = now_ms - (now_ms % window_ms)
local elapsed_in_current = now_ms - current_window_start

local current_count = tonumber(redis.call("GET", current_key)) or 0
local previous_count = tonumber(redis.call("GET", previous_key)) or 0

local weight = (window_ms - elapsed_in_current) / window_ms
if weight < 0 then weight = 0 end

local estimated = (previous_count * weight) + current_count

if estimated >= limit then
    local ttl = redis.call("PTTL", current_key)
    if ttl < 0 then ttl = window_ms end
    return {0, 0, ttl}
end

local new_count = redis.call("INCR", current_key)
if new_count == 1 then
    redis.call("PEXPIRE", current_key, window_ms * 2)
end

local remaining = limit - math.floor(estimated) - 1
if remaining < 0 then remaining = 0 end

return {1, remaining, window_ms - elapsed_in_current}
```

### Redis Cluster Hash-Tag Partitioning
Keys use Redis hash-tags `{key}` to guarantee that both current and previous counters reside on the same Redis cluster slot:
- Current: `ratelimit:{client_id|ip}:<current_window_timestamp>`
- Previous: `ratelimit:{client_id|ip}:<previous_window_timestamp>`

Slot Calculation:
$$\text{Slot} = \text{CRC16}(\text{key}) \pmod{16384}$$

---

## 3. Network & Connection Pooling Specifications

| Component | Setting | Value | Specification Rationale |
|---|---|---|---|
| **Reverse Proxy** | `MaxIdleConns` | `10,000` | Global connection pool across all upstreams. |
| **Reverse Proxy** | `MaxIdleConnsPerHost` | `2,000` | Prevents TCP socket exhaustion on Windows and eliminates socket churn. |
| **Reverse Proxy** | `IdleConnTimeout` | `90s` | Reaps zombie or stale upstream sockets gracefully. |
| **Reverse Proxy** | `KeepAlive` | `60s` | Maintains established TCP sessions with zero handshake overhead. |
| **Redis Client** | `PoolSize` | `1,000` | High-concurrency connection pool for sub-millisecond Redis Lua execution. |
| **Redis Client** | `MinIdleConns` | `100` | Pre-warmed sockets ready to absorb sudden traffic bursts. |
| **Redis Client** | `DialTimeout` | `2s` | Connect timeout threshold. |
| **Redis Client** | `Read / Write Timeout` | `100ms` | Bounded latency ensuring the limiter never stalls caller requests. |
| **Gateway HTTP Server** | `ReadTimeout` | `5s` | Mitigates Slowloris HTTP attacks. |
| **Gateway HTTP Server** | `WriteTimeout` | `10s` | Maximum write window for downstream responses. |
| **Gateway HTTP Server** | `IdleTimeout` | `120s` | Persistent client HTTP keep-alive duration. |
| **Gateway HTTP Server** | `ShutdownTimeout`| `15s` | Grace period for in-flight request draining on termination. |

---

## 4. Security & Middleware Pipeline

### Pipeline Ordering
1. **CORS Middleware**: Evaluates `Origin` against `CORS_ALLOWED_ORIGINS` (`*` or comma-separated allowlist) and handles HTTP `OPTIONS` preflight requests.
2. **JWT Authentication**: Validates Bearer tokens using HMAC-SHA256 (`HS256`).
   - If `require_auth: true`, missing or invalid tokens immediately terminate with `401 Unauthorized`.
   - On success, places `client_id` into `context.Context` under `ClientIDContextKey`.
3. **Sliding Window Rate Limiter**:
   - Resolves key identifier based on route `key_type`:
     - `client_id`: Context `client_id` &rarr; header `X-Client-Id` &rarr; `ratelimit:{client_id}`
     - `ip`: header `X-Forwarded-For` &rarr; `RemoteAddr` &rarr; `ratelimit:{ip}`
   - Sets headers:
     - `X-RateLimit-Limit`: Maximum requests per window.
     - `X-RateLimit-Remaining`: Tokens left in current sliding window.
   - Throttling: If rate exceeded, returns `429 Too Many Requests` with `Retry-After: <seconds>`.
4. **Prometheus Instrumentation**: Records latency observations in a 13-bucket histogram and increments status counters.
5. **Reverse Proxy (httputil)**: Modifies `Host`, adds `X-Forwarded-Host` and `X-Gateway: distributed-rate-limiter-gateway`, and forwards to upstream microservices.

### Fail-Open Reliability Guarantee
If Redis fails, times out, or becomes unreachable:
- Rate limiter catches error and calls `metrics.RecordRateLimiterError()`.
- Increments `gateway_rate_limiter_errors_total`.
- **Fails Open**: Passes the request forward to keep business traffic flowing, avoiding complete system outage.

---

## 5. Verified Performance Benchmarks

Benchmarked using `k6` sustained arrival rate on Windows 11 (Go 1.22 + Redis 5.0):

| Performance Indicator | Measured Result |
|---|---|
| **Peak Throughput** | **10,501.6 requests / second** |
| **Sustained Tested Rate** | **5,000.00 requests / second** |
| **Success Rate** | **100.00% (0.00% drop rate)** |
| **Average Latency** | **124 µs – 326 µs (sub-millisecond)** |
| **p90 Latency** | **515 µs – 555 µs** |
| **p95 Latency** | **541 µs – 877 µs** |
| **p99 Latency** | **3.15 ms – 9.93 ms** |
| **Total Processed Test Traffic** | **> 120,000 requests** |

---

## 6. Prometheus Telemetry Specification

Exposed at `GET /metrics`:

```prometheus
# HELP gateway_requests_total Total number of requests processed by the gateway, labeled by route, method and status code.
# TYPE gateway_requests_total counter
gateway_requests_total{method="GET",route="/api/orders",status="200"} 80005

# HELP gateway_rate_limited_total Total number of requests rejected due to rate limiting, labeled by route.
# TYPE gateway_rate_limited_total counter
gateway_rate_limited_total{route="/api/users"} 1

# HELP gateway_request_duration_seconds Request latency distribution in seconds, labeled by route and method.
# TYPE gateway_request_duration_seconds histogram
gateway_request_duration_seconds_bucket{method="GET",route="/api/orders",le="0.0005"} 41327
gateway_request_duration_seconds_bucket{method="GET",route="/api/orders",le="0.001"} 68210
...
gateway_request_duration_seconds_count{method="GET",route="/api/orders"} 80005

# HELP gateway_rate_limiter_errors_total Total number of errors encountered while evaluating rate limits (fail-open events).
# TYPE gateway_rate_limiter_errors_total counter
gateway_rate_limiter_errors_total 0
```

---

## 7. Web Application Dashboard Specification

- **Frontend Tech Stack**: HTML5, Vanilla CSS3 (Custom Design Tokens), ES6+ JavaScript, Canvas 2D.
- **Theme**: Deep Obsidian Black (`#050806`, `#0a0f0d`) + Radiant Light Green / Mint (`#00ff88`, `#10b981`).
- **Interactive Capabilities**:
  - Live Canvas Throughput & Throttling graph (auto-polled every 2.5s).
  - One-click JWT Bearer Token Minting.
  - Interactive Request Dispatcher with Single, 10x Burst, and 55x Throttling Triggers.
  - Live Redis active bucket table inspector (`ratelimit:*` keys with TTL).
  - Embedded directly into Go binary and served on `http://localhost:8080/`.
