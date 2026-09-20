package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// GeoIPRule represents a country-level access rule.
type GeoIPRule struct {
	CountryCode string `json:"country_code"` // 2-letter ISO code e.g. "CN", "RU", "KP"
	Action      string `json:"action"`       // "block" or "allow"
	Reason      string `json:"reason"`
}

// GeoIPFilter manages country-level traffic filtering.
type GeoIPFilter struct {
	rules map[string]GeoIPRule
	mu    sync.RWMutex
}

func NewGeoIPFilter() *GeoIPFilter {
	f := &GeoIPFilter{
		rules: make(map[string]GeoIPRule),
	}
	// Seed sample demo rule
	f.rules["XX"] = GeoIPRule{CountryCode: "XX", Action: "block", Reason: "High-risk anonymous proxy territory"}
	return f
}

func (f *GeoIPFilter) AddRule(code, action, reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	code = strings.ToUpper(strings.TrimSpace(code))
	f.rules[code] = GeoIPRule{
		CountryCode: code,
		Action:      action,
		Reason:      reason,
	}
}

func (f *GeoIPFilter) RemoveRule(code string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.rules, strings.ToUpper(strings.TrimSpace(code)))
}

func (f *GeoIPFilter) ListRules() []GeoIPRule {
	f.mu.RLock()
	defer f.mu.RUnlock()
	list := make([]GeoIPRule, 0, len(f.rules))
	for _, r := range f.rules {
		list = append(list, r)
	}
	return list
}

func (f *GeoIPFilter) ExtractCountry(r *http.Request) string {
	if c := r.Header.Get("X-Country-Code"); c != "" {
		return strings.ToUpper(strings.TrimSpace(c))
	}
	if c := r.Header.Get("CF-IPCountry"); c != "" {
		return strings.ToUpper(strings.TrimSpace(c))
	}
	if c := r.URL.Query().Get("country"); c != "" {
		return strings.ToUpper(strings.TrimSpace(c))
	}
	return "US" // Default fallback country
}

func (f *GeoIPFilter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		country := f.ExtractCountry(r)

		f.mu.RLock()
		rule, exists := f.rules[country]
		f.mu.RUnlock()

		if exists && rule.Action == "block" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-GeoIP-Country", country)
			w.Header().Set("X-GeoIP-Blocked", "true")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":        "access denied by GeoIP firewall",
				"country_code": country,
				"reason":       rule.Reason,
			})
			return
		}

		w.Header().Set("X-GeoIP-Country", country)
		next.ServeHTTP(w, r)
	})
}
