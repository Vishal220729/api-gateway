package cloud

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// CloudWebhook represents a cloud notification or event bridge destination.
type CloudWebhook struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Provider  string    `json:"provider"` // "slack", "aws_eventbridge", "datadog", "generic"
	Secret    string    `json:"secret"`
	Events    []string  `json:"events"`   // "waf_block", "rate_limit", "circuit_trip", "chaos_alert"
	CreatedAt time.Time `json:"created_at"`
}

// CloudWebhookManager manages registered cloud webhooks and dispatches events with HMAC signatures.
type CloudWebhookManager struct {
	webhooks map[string]*CloudWebhook
	client   *http.Client
	mu       sync.RWMutex
}

func NewCloudWebhookManager() *CloudWebhookManager {
	mgr := &CloudWebhookManager{
		webhooks: make(map[string]*CloudWebhook),
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
	}

	// Seed sample demo webhooks for demonstration
	mgr.AddWebhook(&CloudWebhook{
		ID:        "wh_aws_eventbridge_us1",
		Name:      "AWS EventBridge (Security & Threat Bus)",
		URL:       "https://events.us-east-1.amazonaws.com/v1/gateway-alerts",
		Provider:  "aws_eventbridge",
		Secret:    "aws_sec_demo_key_99",
		Events:    []string{"waf_block", "circuit_trip"},
		CreatedAt: time.Now(),
	})

	mgr.AddWebhook(&CloudWebhook{
		ID:        "wh_slack_devops",
		Name:      "Slack #devops-alerts Channel",
		URL:       "https://hooks.slack.com/services/T00/B00/DEMO",
		Provider:  "slack",
		Secret:    "slack_sec_demo_42",
		Events:    []string{"waf_block", "rate_limit", "circuit_trip"},
		CreatedAt: time.Now(),
	})

	return mgr
}

func (m *CloudWebhookManager) AddWebhook(wh *CloudWebhook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if wh.ID == "" {
		wh.ID = fmt.Sprintf("wh_%d", time.Now().UnixMilli()%1000000)
	}
	if wh.CreatedAt.IsZero() {
		wh.CreatedAt = time.Now()
	}
	m.webhooks[wh.ID] = wh
}

func (m *CloudWebhookManager) RemoveWebhook(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.webhooks[id]
	if exists {
		delete(m.webhooks, id)
	}
	return exists
}

func (m *CloudWebhookManager) ListWebhooks() []*CloudWebhook {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*CloudWebhook, 0, len(m.webhooks))
	for _, wh := range m.webhooks {
		list = append(list, wh)
	}
	return list
}

func (m *CloudWebhookManager) DispatchAlert(event string, payload map[string]interface{}) {
	m.mu.RLock()
	webhooks := make([]*CloudWebhook, 0, len(m.webhooks))
	for _, wh := range m.webhooks {
		for _, e := range wh.Events {
			if e == event || e == "*" {
				webhooks = append(webhooks, wh)
				break
			}
		}
	}
	m.mu.RUnlock()

	for _, wh := range webhooks {
		go m.sendToWebhook(wh, event, payload)
	}
}

func (m *CloudWebhookManager) sendToWebhook(wh *CloudWebhook, event string, payload map[string]interface{}) {
	envelope := map[string]interface{}{
		"event":        event,
		"webhook_id":   wh.ID,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"gateway_id":   "gw-prod-us-east-1",
		"payload_data": payload,
	}

	bodyBytes, err := json.Marshal(envelope)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, wh.URL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Go-Redis-APIGateway-CloudNotifier/2.0")

	// Sign payload with HMAC-SHA256 if secret is set
	if wh.Secret != "" {
		h := hmac.New(sha256.New, []byte(wh.Secret))
		h.Write(bodyBytes)
		sig := hex.EncodeToString(h.Sum(nil))
		req.Header.Set("X-Gateway-Signature", "sha256="+sig)
	}

	// Non-blocking best-effort execution (demo endpoints may return 404 or connection failure)
	resp, _ := m.client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
}

// CloudArchiveBundle represents a batched archive uploaded to AWS S3 / GCS.
type CloudArchiveBundle struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	RecordCount int       `json:"record_count"`
	S3URI       string    `json:"s3_uri"`
	SizeBytes   int64     `json:"size_bytes"`
	Status      string    `json:"status"` // "synced", "pending"
}

// CloudArchiver manages log bundling and cloud storage synchronization.
type CloudArchiver struct {
	archives []*CloudArchiveBundle
	mu       sync.RWMutex
}

func NewCloudArchiver() *CloudArchiver {
	return &CloudArchiver{
		archives: make([]*CloudArchiveBundle, 0),
	}
}

func (a *CloudArchiver) CreateArchive(recordCount int) *CloudArchiveBundle {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	id := fmt.Sprintf("arch_%s_%04d", now.Format("20060102_150405"), len(a.archives)+1)
	s3URI := fmt.Sprintf("s3://api-gateway-traffic-logs/%s/%s.json.gz", now.Format("2006/01/02"), id)

	bundle := &CloudArchiveBundle{
		ID:          id,
		Timestamp:   now,
		RecordCount: recordCount,
		S3URI:       s3URI,
		SizeBytes:   int64(recordCount * 285), // Estimated compressed byte size
		Status:      "synced",
	}

	a.archives = append([]*CloudArchiveBundle{bundle}, a.archives...)
	if len(a.archives) > 20 {
		a.archives = a.archives[:20]
	}
	return bundle
}

func (a *CloudArchiver) ListArchives() []*CloudArchiveBundle {
	a.mu.RLock()
	defer a.mu.RUnlock()
	copied := make([]*CloudArchiveBundle, len(a.archives))
	copy(copied, a.archives)
	return copied
}

// CloudRegion represents an edge region deployment.
type CloudRegion struct {
	Code        string    `json:"code"` // "us-east-1", "eu-west-1", "ap-south-1"
	Name        string    `json:"name"`
	Role        string    `json:"role"` // "primary", "replica", "edge"
	LatencyMs   int64     `json:"latency_ms"`
	Status      string    `json:"status"` // "healthy", "syncing"
	LastSynced  time.Time `json:"last_synced"`
	Replication string    `json:"replication"`
}

// CloudRegionManager manages multi-region edge topology and replication status.
type CloudRegionManager struct {
	regions []CloudRegion
	mu      sync.RWMutex
}

func NewCloudRegionManager() *CloudRegionManager {
	return &CloudRegionManager{
		regions: []CloudRegion{
			{
				Code:        "us-east-1",
				Name:        "US East (N. Virginia)",
				Role:        "primary",
				LatencyMs:   1,
				Status:      "healthy",
				LastSynced:  time.Now(),
				Replication: "Leader (Active)",
			},
			{
				Code:        "eu-west-1",
				Name:        "Europe (Ireland)",
				Role:        "replica",
				LatencyMs:   68,
				Status:      "healthy",
				LastSynced:  time.Now().Add(-2 * time.Second),
				Replication: "Async Redis Stream (0.2s lag)",
			},
			{
				Code:        "ap-south-1",
				Name:        "Asia Pacific (Mumbai)",
				Role:        "edge",
				LatencyMs:   14,
				Status:      "healthy",
				LastSynced:  time.Now().Add(-1 * time.Second),
				Replication: "Edge Read-Through Cache",
			},
		},
	}
}

func (rm *CloudRegionManager) GetRegions() []CloudRegion {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	copied := make([]CloudRegion, len(rm.regions))
	copy(copied, rm.regions)
	return copied
}
