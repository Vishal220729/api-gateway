package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// RouteConfig describes one proxied API route: where it forwards to, which
// HTTP methods it accepts, and its own independent rate-limit policy.
type RouteConfig struct {
	Path        string   `json:"path"`
	Upstream    string   `json:"upstream"`
	Methods     []string `json:"methods"`
	RateLimit   int      `json:"rate_limit"`
	WindowSec   int      `json:"window_sec"`
	KeyType     string   `json:"key_type"` // "ip" or "client_id"
	RequireAuth bool     `json:"require_auth"`
}

// Config holds all runtime configuration for the gateway, loaded from
// environment variables (optionally combined with a JSON routes file).
type Config struct {
	Port               string
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	JWTSecret          string
	DefaultRateLimit   int
	DefaultWindowSec   int
	CORSAllowedOrigins []string
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	ShutdownTimeout    time.Duration
	Routes             []RouteConfig
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

// Load builds a Config from environment variables. If ROUTES_CONFIG_PATH
// points to a JSON file, routes are loaded from it; otherwise a small set
// of sane default routes is used so the gateway is runnable out of the box.
func Load() (*Config, error) {
	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          getEnvInt("REDIS_DB", 0),
		JWTSecret:        getEnv("JWT_SECRET", "change-me-in-production"),
		DefaultRateLimit: getEnvInt("DEFAULT_RATE_LIMIT", 100),
		DefaultWindowSec: getEnvInt("DEFAULT_WINDOW_SEC", 60),
		ReadTimeout:      getEnvDuration("READ_TIMEOUT", 5*time.Second),
		WriteTimeout:     getEnvDuration("WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:      getEnvDuration("IDLE_TIMEOUT", 120*time.Second),
		ShutdownTimeout:  getEnvDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
	}

	origins := getEnv("CORS_ALLOWED_ORIGINS", "*")
	cfg.CORSAllowedOrigins = strings.Split(origins, ",")

	routesPath := getEnv("ROUTES_CONFIG_PATH", "")
	if routesPath != "" {
		data, err := os.ReadFile(routesPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read routes config %s: %w", routesPath, err)
		}
		var routes []RouteConfig
		if err := json.Unmarshal(data, &routes); err != nil {
			return nil, fmt.Errorf("failed to parse routes config: %w", err)
		}
		cfg.Routes = routes
	} else {
		cfg.Routes = defaultRoutes(cfg)
	}

	return cfg, nil
}

func defaultRoutes(cfg *Config) []RouteConfig {
	return []RouteConfig{
		{
			Path:        "/api/users",
			Upstream:    getEnv("USERS_SERVICE_URL", "http://localhost:9001"),
			Methods:     []string{"GET", "POST", "PUT", "DELETE"},
			RateLimit:   getEnvInt("USERS_RATE_LIMIT", 50),
			WindowSec:   getEnvInt("USERS_WINDOW_SEC", 60),
			KeyType:     "client_id",
			RequireAuth: true,
		},
		{
			Path:        "/api/orders",
			Upstream:    getEnv("ORDERS_SERVICE_URL", "http://localhost:9002"),
			Methods:     []string{"GET", "POST"},
			RateLimit:   getEnvInt("ORDERS_RATE_LIMIT", 100),
			WindowSec:   getEnvInt("ORDERS_WINDOW_SEC", 60),
			KeyType:     "ip",
			RequireAuth: false,
		},
		{
			Path:        "/api/public",
			Upstream:    getEnv("PUBLIC_SERVICE_URL", "http://localhost:9003"),
			Methods:     []string{"GET"},
			RateLimit:   cfg.DefaultRateLimit,
			WindowSec:   cfg.DefaultWindowSec,
			KeyType:     "ip",
			RequireAuth: false,
		},
	}
}
