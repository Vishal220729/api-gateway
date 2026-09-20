package ai

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/vishalrr/api-gateway/middleware"
	"github.com/vishalrr/api-gateway/proxy"
)

// CopilotCommandResult holds the outcome of an AI-interpreted command.
type CopilotCommandResult struct {
	Action      string `json:"action"`
	Target      string `json:"target"`
	Details     string `json:"details"`
	Success     bool   `json:"success"`
	Explanation string `json:"explanation"`
}

// DiagnosticReport holds automated incident root-cause analysis findings.
type DiagnosticReport struct {
	Timestamp      time.Time `json:"timestamp"`
	HealthStatus   string    `json:"health_status"` // "OPTIMAL", "WARNING", "CRITICAL"
	HealthScore    int       `json:"health_score"`  // 0 - 100
	ActiveRPS      float64   `json:"active_rps"`
	ThrottleRatio  float64   `json:"throttle_ratio"` // e.g. 0.05 = 5%
	OpenCircuits   int       `json:"open_circuits"`
	Findings       []string  `json:"findings"`
	Remediations   []string  `json:"remediations"`
	AICopilotNotes string    `json:"ai_copilot_notes"`
}

// AICopilotEngine executes natural language directives and automated diagnostics.
type AICopilotEngine struct{}

func NewAICopilotEngine() *AICopilotEngine {
	return &AICopilotEngine{}
}

// ProcessDirective parses natural language intent and applies gateway configurations.
func (c *AICopilotEngine) ProcessDirective(
	prompt string,
	waf *middleware.WAFEngine,
	geoip *middleware.GeoIPFilter,
	cbm *proxy.CircuitBreakerManager,
) CopilotCommandResult {
	p := strings.ToLower(strings.TrimSpace(prompt))

	// 1. IP Blacklist (e.g., "block ip 192.168.1.5", "blacklist 45.33.32.1 for botting")
	ipRegex := regexp.MustCompile(`(?:block|blacklist|ban)\s+(?:ip\s+)?([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})`)
	if match := ipRegex.FindStringSubmatch(p); len(match) > 1 {
		ip := match[1]
		waf.AddRule(ip, "blacklist", "AI Copilot: Blocked per user natural language command")
		return CopilotCommandResult{
			Action:      "WAF_BLACKLIST",
			Target:      ip,
			Details:     fmt.Sprintf("Added IP %s to WAF Blacklist", ip),
			Success:     true,
			Explanation: fmt.Sprintf("I analyzed your command and instructed the perimeter WAF to drop all traffic originating from IP %s with HTTP 403 Forbidden.", ip),
		}
	}

	// 2. IP Whitelist (e.g., "whitelist ip 10.0.0.1", "allow 172.16.0.4")
	wlRegex := regexp.MustCompile(`(?:whitelist|allow)\s+(?:ip\s+)?([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})`)
	if match := wlRegex.FindStringSubmatch(p); len(match) > 1 {
		ip := match[1]
		waf.AddRule(ip, "whitelist", "AI Copilot: Whitelisted per user natural language command")
		return CopilotCommandResult{
			Action:      "WAF_WHITELIST",
			Target:      ip,
			Details:     fmt.Sprintf("Added IP %s to WAF Whitelist", ip),
			Success:     true,
			Explanation: fmt.Sprintf("I have whitelisted IP %s. Traffic from this IP will now bypass sliding-window rate limiters and pass directly to upstream services.", ip),
		}
	}

	// 3. GeoIP Country Blocking (e.g., "block country CN", "block traffic from KP")
	geoRegex := regexp.MustCompile(`(?:block|restrict)\s+(?:country\s+|traffic\s+from\s+)?([a-z]{2})\b`)
	if match := geoRegex.FindStringSubmatch(p); len(match) > 1 {
		cc := strings.ToUpper(match[1])
		geoip.AddRule(cc, "block", "AI Copilot: Country restricted via natural language directive")
		return CopilotCommandResult{
			Action:      "GEOIP_BLOCK",
			Target:      cc,
			Details:     fmt.Sprintf("Blocked ISO country code %s", cc),
			Success:     true,
			Explanation: fmt.Sprintf("I updated the GeoIP perimeter firewall. Inbound requests with ISO country code '%s' will now be blocked with HTTP 403 Forbidden.", cc),
		}
	}

	// 4. Circuit Breaker Trip (e.g., "trip circuit breaker", "open circuit for 9001")
	if strings.Contains(p, "trip") && (strings.Contains(p, "circuit") || strings.Contains(p, "breaker")) {
		cbm.Trip("http://localhost:9001")
		return CopilotCommandResult{
			Action:      "CIRCUIT_TRIP",
			Target:      "http://localhost:9001",
			Details:     "Tripped circuit breaker to OPEN state",
			Success:     true,
			Explanation: "I triggered a manual fail-fast state on upstream node http://localhost:9001. It will immediately fast-fail with HTTP 503 until cooldown expires.",
		}
	}

	// 5. Circuit Breaker Reset (e.g., "reset circuit breaker", "close circuit")
	if strings.Contains(p, "reset") && (strings.Contains(p, "circuit") || strings.Contains(p, "breaker")) {
		cbm.Reset("http://localhost:9001")
		return CopilotCommandResult{
			Action:      "CIRCUIT_RESET",
			Target:      "http://localhost:9001",
			Details:     "Reset circuit breaker to CLOSED state",
			Success:     true,
			Explanation: "I cleared all failure counts and closed the circuit for http://localhost:9001. Traffic is flowing normally again.",
		}
	}

	// 6. Informational queries
	if strings.Contains(p, "how") || strings.Contains(p, "what") || strings.Contains(p, "help") || strings.Contains(p, "status") {
		return CopilotCommandResult{
			Action:      "ASSISTANT_INFO",
			Target:      "system",
			Details:     "AI Gateway Copilot Knowledge Query",
			Success:     true,
			Explanation: "I am your Gateway AI Copilot! You can speak to me in plain English:\n• 'Block IP 198.51.100.4'\n• 'Whitelist IP 10.0.0.1'\n• 'Block country RU'\n• 'Trip circuit breaker'\n• 'Reset circuit breaker'\n• 'Run incident diagnostics'",
		}
	}

	return CopilotCommandResult{
		Action:      "UNKNOWN_COMMAND",
		Target:      prompt,
		Details:     "Could not map to a deterministic gateway action",
		Success:     false,
		Explanation: fmt.Sprintf("I received: '%s'. Try telling me something like 'Block IP 1.2.3.4', 'Whitelist IP 10.0.0.2', 'Block country KP', or 'Run incident diagnostics'.", prompt),
	}
}

// RunDiagnostics executes root-cause automated diagnosis based on telemetry.
func (c *AICopilotEngine) RunDiagnostics(
	totalReqs int64,
	rateLimited int64,
	rps float64,
	circuits []proxy.CircuitStatus,
	activeBucketsCount int,
) DiagnosticReport {
	findings := make([]string, 0)
	remediations := make([]string, 0)
	score := 100
	status := "OPTIMAL"

	throttleRatio := 0.0
	if totalReqs > 0 {
		throttleRatio = float64(rateLimited) / float64(totalReqs)
	}

	openCircuits := 0
	for _, cs := range circuits {
		if cs.State == proxy.StateOpen {
			openCircuits++
		}
	}

	// 1. Evaluate Throttling
	if throttleRatio > 0.30 {
		score -= 30
		findings = append(findings, fmt.Sprintf("Critical rate-limiting: %.1f%% of all requests are being throttled with HTTP 429.", throttleRatio*100))
		remediations = append(remediations, "Review client tier quotas or consider upgrading high-traffic clients to Enterprise VIP tier (2,500 req/min).")
	} else if throttleRatio > 0.08 {
		score -= 10
		findings = append(findings, fmt.Sprintf("Moderate rate-limiting detected: %.1f%% of traffic throttled.", throttleRatio*100))
		remediations = append(remediations, "Verify if specific IP addresses are triggering quota limits due to polling loops.")
	} else {
		findings = append(findings, "Rate-limiting thresholds are healthy (< 5% throttled).")
	}

	// 2. Evaluate Circuit Breakers
	if openCircuits > 0 {
		score -= 25 * openCircuits
		findings = append(findings, fmt.Sprintf("%d upstream circuit breaker(s) are in OPEN (fail-fast) state.", openCircuits))
		remediations = append(remediations, "Check health of failing upstream microservice nodes on ports 9001-9003 and review upstream timeout settings.")
	} else {
		findings = append(findings, "All upstream circuit breakers are CLOSED and healthy.")
	}

	// 3. Evaluate Traffic Velocity
	if rps > 100.0 {
		findings = append(findings, fmt.Sprintf("High-throughput burst: Gateway is actively processing %.1f requests/sec.", rps))
		remediations = append(remediations, "Ensure Redis connection pool (1000 conns) and keep-alive pools are sufficient.")
	}

	if score < 60 {
		status = "CRITICAL"
	} else if score < 85 {
		status = "WARNING"
	}

	notes := fmt.Sprintf("AI Copilot Analysis completed at %s. Overall system score is %d/100 (%s).",
		time.Now().Format("15:04:05 MST"), score, status)

	return DiagnosticReport{
		Timestamp:      time.Now(),
		HealthStatus:   status,
		HealthScore:    score,
		ActiveRPS:      rps,
		ThrottleRatio:  throttleRatio,
		OpenCircuits:   openCircuits,
		Findings:       findings,
		Remediations:   remediations,
		AICopilotNotes: notes,
	}
}
