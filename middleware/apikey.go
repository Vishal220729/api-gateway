package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	APIKeyContextKey contextKey = "api_key_info"
)

// APIKeyTier defines quota rules for an API key tier.
type APIKeyTier struct {
	Name      string `json:"name"`
	RateLimit int    `json:"rate_limit"`
	WindowSec int    `json:"window_sec"`
}

var DefaultTiers = map[string]APIKeyTier{
	"free": {
		Name:      "free",
		RateLimit: 10,
		WindowSec: 60,
	},
	"pro": {
		Name:      "pro",
		RateLimit: 250,
		WindowSec: 60,
	},
	"enterprise": {
		Name:      "enterprise",
		RateLimit: 2500,
		WindowSec: 60,
	},
}

// APIKey represents a registered API credential.
type APIKey struct {
	Key       string    `json:"key"`
	Owner     string    `json:"owner"`
	Tier      string    `json:"tier"`
	RateLimit int       `json:"rate_limit"`
	WindowSec int       `json:"window_sec"`
	CreatedAt time.Time `json:"created_at"`
}

// APIKeyStore manages in-memory and Redis-persisted API keys.
type APIKeyStore struct {
	keys  map[string]*APIKey
	rdb   *redis.Client
	mu    sync.RWMutex
}

// NewAPIKeyStore initializes the store and pre-seeds standard demo keys.
func NewAPIKeyStore(rdb *redis.Client) *APIKeyStore {
	store := &APIKeyStore{
		keys: make(map[string]*APIKey),
		rdb:  rdb,
	}

	// Seed default demo keys for testing
	store.CreateWithKey("agy_live_free_demo_key", "Developer (Free)", "free")
	store.CreateWithKey("agy_live_pro_demo_key", "Startup (Pro)", "pro")
	store.CreateWithKey("agy_live_enterprise_vip", "Enterprise Corp (VIP)", "enterprise")

	return store
}

func (s *APIKeyStore) Create(owner, tierName string) (*APIKey, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("agy_live_%s_%s", tierName, hex.EncodeToString(b)[:12])
	return s.CreateWithKey(key, owner, tierName)
}

func (s *APIKeyStore) CreateWithKey(key, owner, tierName string) (*APIKey, error) {
	tierName = strings.ToLower(tierName)
	tier, ok := DefaultTiers[tierName]
	if !ok {
		tier = DefaultTiers["free"]
		tierName = "free"
	}

	ak := &APIKey{
		Key:       key,
		Owner:     owner,
		Tier:      tierName,
		RateLimit: tier.RateLimit,
		WindowSec: tier.WindowSec,
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	s.keys[key] = ak
	s.mu.Unlock()

	// Persist to Redis if available
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.rdb.HSet(ctx, "apikeys:"+key, map[string]interface{}{
			"owner":      owner,
			"tier":       tierName,
			"rate_limit": tier.RateLimit,
			"window_sec": tier.WindowSec,
			"created_at": ak.CreatedAt.Unix(),
		}).Err()
	}

	return ak, nil
}

func (s *APIKeyStore) Get(key string) (*APIKey, bool) {
	s.mu.RLock()
	ak, ok := s.keys[key]
	s.mu.RUnlock()
	return ak, ok
}

func (s *APIKeyStore) Revoke(key string) bool {
	s.mu.Lock()
	_, ok := s.keys[key]
	if ok {
		delete(s.keys, key)
	}
	s.mu.Unlock()

	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.rdb.Del(ctx, "apikeys:"+key).Err()
	}
	return ok
}

func (s *APIKeyStore) List() []*APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*APIKey, 0, len(s.keys))
	for _, ak := range s.keys {
		list = append(list, ak)
	}
	return list
}

// APIKeyMiddleware inspects the request for X-API-Key and validates it against the store.
func APIKeyMiddleware(store *APIKeyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKeyHeader := r.Header.Get("X-API-Key")
			if apiKeyHeader == "" {
				// Also check query parameter "api_key"
				apiKeyHeader = r.URL.Query().Get("api_key")
			}

			if apiKeyHeader != "" {
				ak, valid := store.Get(apiKeyHeader)
				if !valid {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"error":"invalid or revoked API key"}`))
					return
				}

				// Attach APIKey to request context
				ctx := context.WithValue(r.Context(), APIKeyContextKey, ak)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}
