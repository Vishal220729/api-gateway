package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type CachedResponse struct {
	StatusCode  int               `json:"status_code"`
	Headers     map[string]string `json:"headers"`
	Body        []byte            `json:"body"`
	ContentType string            `json:"content_type"`
}

type HTTPCache struct {
	rdb        *redis.Client
	defaultTTL time.Duration
}

func NewHTTPCache(rdb *redis.Client, defaultTTL time.Duration) *HTTPCache {
	if defaultTTL <= 0 {
		defaultTTL = 30 * time.Second
	}
	return &HTTPCache{
		rdb:        rdb,
		defaultTTL: defaultTTL,
	}
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (c *HTTPCache) cacheKey(r *http.Request) string {
	return "cache:http:" + r.Method + ":" + r.URL.Path
}

// Middleware checks Redis for a cached GET response before delegating downstream.
func (c *HTTPCache) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only cache GET requests on non-management paths
		if r.Method != http.MethodGet || strings.HasPrefix(r.URL.Path, "/api/dashboard") || strings.HasPrefix(r.URL.Path, "/metrics") || strings.HasPrefix(r.URL.Path, "/healthz") {
			next.ServeHTTP(w, r)
			return
		}

		if c.rdb == nil {
			next.ServeHTTP(w, r)
			return
		}

		key := c.cacheKey(r)
		ctx, cancel := context.WithTimeout(r.Context(), 50*time.Millisecond)
		defer cancel()

		cachedData, err := c.rdb.Get(ctx, key).Bytes()
		if err == nil && len(cachedData) > 0 {
			var cached CachedResponse
			if jsonErr := json.Unmarshal(cachedData, &cached); jsonErr == nil {
				w.Header().Set("X-Cache", "HIT")
				if cached.ContentType != "" {
					w.Header().Set("Content-Type", cached.ContentType)
				}
				for k, v := range cached.Headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(cached.StatusCode)
				w.Write(cached.Body)
				return
			}
		}

		// Cache Miss: Record downstream response
		w.Header().Set("X-Cache", "MISS")
		rec := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(rec, r)

		// Store in cache only for successful 200 OK responses
		if rec.statusCode == http.StatusOK && rec.body.Len() > 0 {
			cached := CachedResponse{
				StatusCode:  rec.statusCode,
				ContentType: rec.Header().Get("Content-Type"),
				Body:        rec.body.Bytes(),
				Headers: map[string]string{
					"X-Served-By": "Redis-HTTP-Cache",
				},
			}
			if encoded, err := json.Marshal(cached); err == nil {
				saveCtx, saveCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer saveCancel()
				_ = c.rdb.Set(saveCtx, key, encoded, c.defaultTTL).Err()
			}
		}
	})
}

// Purge removes a specific cached path.
func (c *HTTPCache) Purge(path string) error {
	if c.rdb == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	key := "cache:http:GET:" + path
	return c.rdb.Del(ctx, key).Err()
}

// FlushAll removes all cached HTTP responses.
func (c *HTTPCache) FlushAll() (int64, error) {
	if c.rdb == nil {
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	keys, err := c.rdb.Keys(ctx, "cache:http:*").Result()
	if err != nil || len(keys) == 0 {
		return 0, err
	}
	deleted, err := c.rdb.Del(ctx, keys...).Result()
	return deleted, err
}
