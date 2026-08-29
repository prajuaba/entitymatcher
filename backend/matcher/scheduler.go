package matcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type SchedulerConfig struct {
	Enabled          bool      `json:"enabled"`
	CronExpression   string    `json:"cron_expression"` // e.g. "0 2 * * *" (Nightly 2 AM)
	WebhookURL       string    `json:"webhook_url"`      // e.g. Slack/Teams/Custom REST URL
	NotifyOnSuccess  bool      `json:"notify_on_success"`
	NotifyOnAnomaly  bool      `json:"notify_on_anomaly"`
	LastRunTimestamp time.Time `json:"last_run_timestamp,omitempty"`
	LastRunStatus    string    `json:"last_run_status,omitempty"`
}

func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		Enabled:         false,
		CronExpression:  "0 2 * * *",
		WebhookURL:      "https://hooks.example.com/services/webhook-endpoint",
		NotifyOnSuccess: true,
		NotifyOnAnomaly: true,
	}
}

type WebhookPayload struct {
	Event             string    `json:"event"` // "RECONCILIATION_COMPLETED" | "ANOMALY_DETECTED"
	BatchID           string    `json:"batch_id"`
	TotalSources      int       `json:"total_sources"`
	TotalDestinations int       `json:"total_destinations"`
	AutoMatchedCount  int64     `json:"auto_matched_count"`
	ReviewNeededCount int64     `json:"review_needed_count"`
	ThroughputPerSec  float64   `json:"throughput_per_sec"`
	Timestamp         time.Time `json:"timestamp"`
	Message           string    `json:"message"`
}

type SchedulerManager struct {
	mu     sync.RWMutex
	config SchedulerConfig
}

func NewSchedulerManager() *SchedulerManager {
	return &SchedulerManager{
		config: DefaultSchedulerConfig(),
	}
}

func (sm *SchedulerManager) GetConfig() SchedulerConfig {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.config
}

func (sm *SchedulerManager) UpdateConfig(cfg SchedulerConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.config = cfg
}

func (sm *SchedulerManager) DispatchWebhook(ctx context.Context, payload WebhookPayload) error {
	sm.mu.RLock()
	url := sm.config.WebhookURL
	enabled := sm.config.Enabled
	sm.mu.RUnlock()

	if !enabled || url == "" {
		return nil
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook HTTP dispatch error: %v", err)
	}
	defer resp.Body.Close()

	return nil
}
