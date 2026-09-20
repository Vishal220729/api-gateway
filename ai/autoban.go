package ai

import (
	"sync"
	"time"
)

// AutoBanEntry represents an active AI-triggered IP ban.
type AutoBanEntry struct {
	IP             string    `json:"ip"`
	Reason         string    `json:"reason"`
	BannedAt       time.Time `json:"banned_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	TTLSeconds     int       `json:"ttl_seconds"`
	TriggerAnomaly string    `json:"trigger_anomaly"`
	RiskScore      int       `json:"risk_score"`
}

type ipActivity struct {
	timestamps []time.Time
	errorCount int
}

// AIAnomalyDetector monitors request velocities and automatically bans abusive clients.
type AIAnomalyDetector struct {
	activity map[string]*ipActivity
	banned   map[string]AutoBanEntry
	mu       sync.RWMutex
	banTTL   time.Duration
}

func NewAIAnomalyDetector() *AIAnomalyDetector {
	detector := &AIAnomalyDetector{
		activity: make(map[string]*ipActivity),
		banned:   make(map[string]AutoBanEntry),
		banTTL:   5 * time.Minute,
	}

	// Seed one sample demo auto-ban
	now := time.Now()
	detector.banned["203.0.113.88"] = AutoBanEntry{
		IP:             "203.0.113.88",
		Reason:         "Credential stuffing & burst velocity anomaly (>60 req/s)",
		BannedAt:       now.Add(-1 * time.Minute),
		ExpiresAt:      now.Add(4 * time.Minute),
		TTLSeconds:     240,
		TriggerAnomaly: "HIGH_FREQUENCY_BURST_PROBING",
		RiskScore:      98,
	}

	return detector
}

// RecordRequest tracks an inbound request and returns whether the IP has triggered an auto-ban.
func (d *AIAnomalyDetector) RecordRequest(ip string, status int) (bool, *AutoBanEntry) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()

	// Check if already banned
	if entry, exists := d.banned[ip]; exists {
		if now.Before(entry.ExpiresAt) {
			entry.TTLSeconds = int(entry.ExpiresAt.Sub(now).Seconds())
			return true, &entry
		}
		delete(d.banned, ip)
	}

	act, exists := d.activity[ip]
	if !exists {
		act = &ipActivity{timestamps: make([]time.Time, 0, 30)}
		d.activity[ip] = act
	}

	// Keep only timestamps within the last 10 seconds
	cutoff := now.Add(-10 * time.Second)
	valid := make([]time.Time, 0, len(act.timestamps)+1)
	for _, t := range act.timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	valid = append(valid, now)
	act.timestamps = valid

	if status >= 400 {
		act.errorCount++
	} else if act.errorCount > 0 {
		act.errorCount--
	}

	// Anomaly Detection Heuristics:
	// 1. More than 25 requests in 10 seconds
	// 2. More than 8 consecutive 4xx errors
	var triggerReason string
	var riskScore int

	if len(act.timestamps) >= 25 {
		triggerReason = "Velocity Anomaly: Exceeded 25 req/10s threshold"
		riskScore = 92
	} else if act.errorCount >= 8 {
		triggerReason = "Error Burst Anomaly: 8+ consecutive 4xx probe failures"
		riskScore = 88
	}

	if triggerReason != "" {
		banEntry := AutoBanEntry{
			IP:             ip,
			Reason:         triggerReason,
			BannedAt:       now,
			ExpiresAt:      now.Add(d.banTTL),
			TTLSeconds:     int(d.banTTL.Seconds()),
			TriggerAnomaly: "AUTONOMOUS_HEURISTIC_TRIGGER",
			RiskScore:      riskScore,
		}
		d.banned[ip] = banEntry
		delete(d.activity, ip)
		return true, &banEntry
	}

	return false, nil
}

// IsBanned returns true if the IP is currently in the active auto-ban list.
func (d *AIAnomalyDetector) IsBanned(ip string) (bool, int) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if entry, exists := d.banned[ip]; exists {
		remaining := int(entry.ExpiresAt.Sub(time.Now()).Seconds())
		if remaining > 0 {
			return true, remaining
		}
	}
	return false, 0
}

// Unban removes an IP from the auto-ban list.
func (d *AIAnomalyDetector) Unban(ip string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.banned[ip]; exists {
		delete(d.banned, ip)
		delete(d.activity, ip)
		return true
	}
	return false
}

// ListBanned returns all currently active auto-banned IPs.
func (d *AIAnomalyDetector) ListBanned() []AutoBanEntry {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	list := make([]AutoBanEntry, 0, len(d.banned))

	for ip, entry := range d.banned {
		if now.Before(entry.ExpiresAt) {
			entry.TTLSeconds = int(entry.ExpiresAt.Sub(now).Seconds())
			list = append(list, entry)
		} else {
			delete(d.banned, ip)
		}
	}
	return list
}
