# Distributed Rate Limiter & API Gateway

A production-grade API gateway written in Go, backed by Redis for atomic,
cluster-safe rate limiting via the **sliding window counter** algorithm.
Designed for high-throughput (10,000+ req/s) with sub-millisecond limiter overhead.

- 🖥️ **Real-Time Web Dashboard**: Built-in dark & neon light green web application served on `http://localhost:8080/`.
- 📋 **Complete Technical Specifications**: See [SPECIFICATIONS.md](SPECIFICATIONS.md) for math formulas, transport pooling specs, and concurrency models.

## Architecture

```
                                   ┌─────────────────────┐
                                   │       Clients        │
                                   └──────────┬───────────┘
                                              │ HTTPS
                                              ▼
                        ┌──────────────────────────────────────────┐
                        │              API GATEWAY (Go)             │
                        │                                            │
                        │   ┌────────────┐   ┌────────────────┐     │
   /metrics  ◄───────── │   │   CORS     │──▶│   JWT Auth      │     │
   (Prometheus)         │   │ Middleware │   │  (per-route)    │     │
                        │   └────────────┘   └────────┬────────┘     │
                        │                              ▼              │
                        │                    ┌──────────────────┐    │
                        │                    │  Rate Limiter    │    │
                        │                    │  Middleware      │    │
                        │                    └────────┬─────────┘    │
                        │                              │ EVALSHA      │
                        │                              ▼              │
                        │                    ┌──────────────────┐    │
                        │                    │ Prometheus       │    │
                        │                    │ Instrumentation  │    │
                        │                    └────────┬─────────┘    │
                        │                              ▼              │
                        │                    ┌──────────────────┐    │
                        │                    │  Reverse Proxy    │    │
                        │                    │  (httputil)       │    │
                        │                    └────────┬─────────┘    │
                        └──────────────────────────────┼──────────────┘
                                              │
                    ┌─────────────────────────┼─────────────────────────┐
                    ▼                         ▼                         ▼
           ┌────────────────┐       ┌────────────────┐       ┌────────────────┐
           │  Redis          │       │  users-service  │       │  orders-service │
           │  (Lua script:   │       │  (downstream)   │       │  (downstream)   │
           │  sliding window)│       └────────────────┘       └────────────────┘
           └────────────────┘
                    ▲
                    │ scrape /metrics
           ┌────────────────┐
           │   Prometheus    │
           └────────────────┘
```

### Request flow

1. **CORS** — handles preflight `OPTIONS` requests and sets access-control headers.
2. **JWT Auth** — validates `Authorization: Bearer <token>` per the matched route's `require_auth` flag; extracts `client_id` into request context.
3. **Rate Limiter** — matches the longest configured route prefix, derives a bucket key (`client_id` or IP), and runs the sliding-window Lua script against Redis in a single round trip. Fails open on Redis errors.
4. **Metrics** — wraps the handler to record request counts and a latency histogram, labeled by route/method/status.
5. **Reverse Proxy** — forwards to the configured upstream using a cached `httputil.ReverseProxy`.

## Sliding Window Counter Algorithm

Rather than storing a timestamp per request (sliding log — memory-heavy) or
allowing bursts at window boundaries (fixed window), the sliding window
counter blends two fixed counters:

```
estimated_count = previous_window_count * overlap_weight + current_window_count
overlap_weight   = (window_size - elapsed_in_current_window) / window_size
```

This is computed and enforced **atomically** inside a Redis Lua script
(`limiter/sliding_window.go`), so concurrent requests across gateway
instances never race on a read-then-increment. Both keys for a bucket use a
Redis hash tag (`{key}`) so they always land on the same cluster slot,
making the limiter safe to run against Redis Cluster for horizontal scale.

## Project Layout

```
.
├── main.go                     # Server init, middleware chain, graceful shutdown
├── config/config.go            # Env-based config + per-route JSON policy loader
├── limiter/sliding_window.go   # Redis Lua sliding window counter
├── middleware/ratelimit.go     # Rate limit middleware + route matching
├── middleware/auth.go          # JWT validation middleware
├── middleware/cors.go          # CORS middleware
├── proxy/reverse_proxy.go      # Dynamic reverse proxy / request forwarding
├── metrics/prometheus.go       # Prometheus counters + histogram
├── routes.json                 # Sample per-route rate limit / upstream config
├── prometheus.yml              # Prometheus scrape config
├── docker-compose.yml          # Gateway + Redis + Prometheus + demo backends
├── Dockerfile                  # Multi-stage build
├── go.mod
└── scripts/load_test.js        # k6 benchmark script
```

## Configuration (environment variables)

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | `` | Redis password |
| `REDIS_DB` | `0` | Redis logical DB |
| `JWT_SECRET` | `change-me-in-production` | HMAC secret for JWT validation |
| `DEFAULT_RATE_LIMIT` | `100` | Fallback limit for unmatched routes |
| `DEFAULT_WINDOW_SEC` | `60` | Fallback window (seconds) |
| `CORS_ALLOWED_ORIGINS` | `*` | Comma-separated allow-list, or `*` |
| `ROUTES_CONFIG_PATH` | `` | Path to a `routes.json` (see format below); if unset, built-in defaults are used |
| `READ_TIMEOUT` / `WRITE_TIMEOUT` / `IDLE_TIMEOUT` | `5s` / `10s` / `120s` | Server timeouts |
| `SHUTDOWN_TIMEOUT` | `15s` | Grace period for in-flight requests on shutdown |

Per-route policy (`routes.json`):

```json
[
  {
    "path": "/api/users",
    "upstream": "http://users-service:80",
    "methods": ["GET", "POST", "PUT", "DELETE"],
    "rate_limit": 50,
    "window_sec": 60,
    "key_type": "client_id",
    "require_auth": true
  }
]
```

## Running Locally

```bash
docker compose up --build
```

This starts: the gateway (`:8080`), Redis (`:6379`), Prometheus (`:9090`),
and two `whoami` demo backends standing in for real microservices.

Try it:

```bash
# Unauthenticated route
curl -i http://localhost:8080/api/orders

# Authenticated route — mint a token with the demo secret first, e.g. via
# jwt.io with HS256 and secret "super-secret-change-me", claim {"client_id":"abc"}
curl -i http://localhost:8080/api/users \
  -H "Authorization: Bearer <token>"

# Metrics
curl http://localhost:8080/metrics

# Health check
curl http://localhost:8080/healthz
```

Watch `X-RateLimit-Limit` / `X-RateLimit-Remaining` on each response, and a
`429` with `Retry-After` once a bucket is exhausted.

## Running Without Docker

```bash
go mod tidy          # resolves go.sum for the pinned dependencies
redis-server &        # or point REDIS_ADDR at an existing instance
go run .
```

## Benchmarking with k6

`scripts/load_test.js` drives a constant 10,000 requests/sec for 30 seconds
against `/api/orders` and asserts p95 latency stays under 5ms:

```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    high_throughput: {
      executor: 'constant-arrival-rate',
      rate: 10000,
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 500,
      maxVUs: 2000,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<5', 'p(99)<15'],
    http_req_failed: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  const res = http.get(`${BASE_URL}/api/orders`, {
    headers: { 'X-Client-Id': `client-${__VU}` },
  });

  check(res, {
    'status is 200 or 429': (r) => r.status === 200 || r.status === 429,
    'has rate limit headers': (r) => r.headers['X-Ratelimit-Limit'] !== undefined,
  });

  sleep(0.001);
}
```

Run it:

```bash
k6 run scripts/load_test.js
# or against a remote deployment:
BASE_URL=https://gateway.example.com k6 run scripts/load_test.js
```

Realistic p95 overhead added by the limiter itself (excluding upstream
latency) is typically well under 5ms on a warm Redis connection pool,
since each check is a single pipelined `EVALSHA` round trip.

## Scaling Notes

- **Horizontal gateway scaling**: the gateway is stateless — run N replicas
  behind a load balancer; all rate-limit state lives in Redis.
- **Redis scaling**: for very high request volume, point `REDIS_ADDR` at a
  Redis Cluster endpoint. The `{key}` hash-tagging in the Lua script keeps
  both counters for a bucket on the same slot, so no code changes are
  needed beyond switching to a cluster-aware client if desired.
- **Fail-open by design**: if Redis is briefly unreachable, requests are
  allowed through rather than rejected, trading strict enforcement for
  availability — flip this in `middleware/ratelimit.go` if strict
  enforcement is preferred for your use case.
