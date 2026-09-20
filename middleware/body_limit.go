package middleware

import (
	"encoding/json"
	"net/http"
)

// BodySizeLimitMiddleware protects upstream services against payload exhaustion attacks.
func BodySizeLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = 1024 * 1024 // 1 MB default
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":       "payload too large",
					"max_bytes":   maxBytes,
					"actual_size": r.ContentLength,
				})
				return
			}

			// Wrap body in MaxBytesReader to catch chunked transfers exceeding max
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
