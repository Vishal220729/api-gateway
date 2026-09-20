package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type wafContextKey string

// WAFWhitelistedContextKey is placed in request context when the caller's IP is whitelisted.
const WAFWhitelistedContextKey wafContextKey = "waf_whitelisted"

// IPRule represents a single WAF firewall rule.
type IPRule struct {
	IP        string    `json:"ip"`
	Type      string    `json:"type"` // "blacklist" or "whitelist"
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// WAFEngine enforces IP Blacklisting (403 Forbidden) and Whitelisting (Rate-limit bypass).
type WAFEngine struct {
	blacklist map[string]IPRule
	whitelist map[string]IPRule
	mu        sync.RWMutex
}

// NewWAFEngine initializes an in-memory thread-safe WAF engine.
func NewWAFEngine() *WAFEngine {
	return &WAFEngine{
		blacklist: make(map[string]IPRule),
		whitelist: make(map[string]IPRule),
	}
}

// ExtractIP extracts the client IP address from X-Forwarded-For or RemoteAddr.
func ExtractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// AddRule adds an IP to the blacklist or whitelist.
func (w *WAFEngine) AddRule(ip, ruleType, reason string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	ip = strings.TrimSpace(ip)
	if ip == "" {
		return
	}

	rule := IPRule{
		IP:        ip,
		Type:      ruleType,
		Reason:    reason,
		CreatedAt: time.Now(),
	}

	if ruleType == "blacklist" {
		delete(w.whitelist, ip)
		w.blacklist[ip] = rule
	} else if ruleType == "whitelist" {
		delete(w.blacklist, ip)
		w.whitelist[ip] = rule
	}
}

// RemoveRule deletes an IP rule.
func (w *WAFEngine) RemoveRule(ip string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	ip = strings.TrimSpace(ip)
	delete(w.blacklist, ip)
	delete(w.whitelist, ip)
}

// ListRules returns all active blacklist and whitelist rules.
func (w *WAFEngine) ListRules() []IPRule {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var rules []IPRule
	for _, r := range w.blacklist {
		rules = append(rules, r)
	}
	for _, r := range w.whitelist {
		rules = append(rules, r)
	}
	return rules
}

// IsBlacklisted checks if an IP is blocked.
func (w *WAFEngine) IsBlacklisted(ip string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, found := w.blacklist[ip]
	return found
}

// IsWhitelisted checks if an IP is whitelisted.
func (w *WAFEngine) IsWhitelisted(ip string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, found := w.whitelist[ip]
	return found
}

// Middleware returns the HTTP middleware that checks the WAF rules.
func (w *WAFEngine) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		ip := ExtractIP(r)

		if w.IsBlacklisted(ip) {
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusForbidden)
			rw.Write([]byte(`{"error":"forbidden: IP address blacklisted by WAF security rule"}`))
			return
		}

		if w.IsWhitelisted(ip) {
			ctx := context.WithValue(r.Context(), WAFWhitelistedContextKey, true)
			next.ServeHTTP(rw, r.WithContext(ctx))
			return
		}

		next.ServeHTTP(rw, r)
	})
}
