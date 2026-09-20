package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// JWTAuth returns middleware that validates a Bearer JWT (HMAC-signed with
// secret). On success, the "client_id" (or "sub") claim is placed into the
// request context under ClientIDContextKey, where the rate limiter and
// downstream handlers can read it. When requireAuth is false, a missing or
// invalid token is ignored rather than rejected — useful for routes that
// accept anonymous traffic but still want to key rate limits by identity
// when a valid token happens to be present.
func JWTAuth(secret string, requireAuth bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			if authHeader == "" {
				if requireAuth {
					writeUnauthorized(w, "missing authorization header")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				if requireAuth {
					writeUnauthorized(w, "invalid authorization header format")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			tokenStr := parts[1]
			claims := jwt.MapClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})

			if err != nil || !token.Valid {
				if requireAuth {
					writeUnauthorized(w, "invalid or expired token")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			clientID := ""
			if sub, ok := claims["client_id"].(string); ok {
				clientID = sub
			} else if sub, ok := claims["sub"].(string); ok {
				clientID = sub
			}

			ctx := context.WithValue(r.Context(), ClientIDContextKey, clientID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"` + msg + `"}`))
}
