package matcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// ReconcileFunc runs one reconciliation pass for the given batch and reports what happened.
type ReconcileFunc func(ctx context.Context, batchID string) (ReconcileOutcome, error)

type ReconcileOutcome struct {
	BatchID           string
	TotalSources      int
	TotalDestinations int
	AutoMatched       int64
	ReviewNeeded      int64
	NoMatch           int64
	ElapsedMs         int64
}

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
	mu              sync.RWMutex
	config          SchedulerConfig
	cronScheduler   *cron.Cron
	cronEntry       cron.EntryID
	lastBatchID     string
	runInProgress   bool
	reconcileFunc   ReconcileFunc
}

// JobFunc is the function executed by the cron scheduler.
// It will be provided when setting up the scheduler.
type JobFunc func() error

func NewSchedulerManager() *SchedulerManager {
	return &SchedulerManager{
		config:        DefaultSchedulerConfig(),
		cronScheduler: cron.New(),
		cronEntry:     -1, // Invalid entry ID
		runInProgress: false,
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

	// Remove existing entry if it exists
	if sm.cronEntry != -1 {
		sm.cronScheduler.Remove(sm.cronEntry)
		sm.cronEntry = -1
	}

	// Schedule if enabled
	if cfg.Enabled && cfg.CronExpression != "" {
		entry, err := sm.cronScheduler.AddFunc(cfg.CronExpression, func() {
			sm.runScheduledJob()
		})
		if err != nil {
			log.Printf("Failed to schedule cron job: %v", err)
			return
		}
		sm.cronEntry = entry
		sm.cronScheduler.Start()
	}
}

// ValidateCronExpression validates a cron expression.
func (sm *SchedulerManager) ValidateCronExpression(expr string) error {
	_, err := cron.ParseStandard(expr)
	return err
}

// SetLastBatchID sets the batch ID to run on next schedule.
func (sm *SchedulerManager) SetLastBatchID(batchID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastBatchID = batchID
}

// SetReconcileFunc sets the function called by scheduled jobs.
func (sm *SchedulerManager) SetReconcileFunc(fn ReconcileFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.reconcileFunc = fn
}

// runScheduledJob executes the scheduled matching job.
// It calls the injected ReconcileFunc to perform actual reconciliation.
func (sm *SchedulerManager) runScheduledJob() {
	sm.mu.Lock()
	if sm.runInProgress || sm.lastBatchID == "" {
		sm.mu.Unlock()
		return
	}
	sm.runInProgress = true
	batchID := sm.lastBatchID
	reconcileFunc := sm.reconcileFunc  // Capture under lock
	sm.mu.Unlock()

	if reconcileFunc == nil {
		sm.mu.Lock()
		sm.runInProgress = false
		sm.mu.Unlock()
		log.Printf("Scheduled job skipped: no reconcile function configured for batch %s", batchID)
		return
	}

	// Use a 30-minute timeout for the reconciliation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	outcome, err := reconcileFunc(ctx, batchID)

	sm.mu.Lock()
	defer func() {
		sm.runInProgress = false
		sm.mu.Unlock()
	}()

	if err != nil {
		sm.config.LastRunStatus = fmt.Sprintf("FAILED: %v", err)
		log.Printf("Scheduled job failed for batch %s: %v", batchID, err)
		return
	}

	sm.config.LastRunStatus = "COMPLETED"
	sm.config.LastRunTimestamp = time.Now()
	log.Printf("Scheduled job completed for batch %s: %d auto-matched, %d review needed, %d no-match",
		batchID, outcome.AutoMatched, outcome.ReviewNeeded, outcome.NoMatch)
}

// Stop gracefully stops the cron scheduler.
func (sm *SchedulerManager) Stop() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.cronScheduler != nil {
		sm.cronScheduler.Stop()
	}
}

func (sm *SchedulerManager) DispatchWebhook(ctx context.Context, payload WebhookPayload) error {
	sm.mu.RLock()
	url := sm.config.WebhookURL
	sm.mu.RUnlock()

	// Gate on webhook URL being set, not on Enabled flag
	// (Enabled controls the cron schedule; webhooks dispatch based on NotifyOn* flags in caller)
	if url == "" {
		return nil
	}

	// Determine payload format based on URL
	var bodyBytes []byte
	var contentType string

	if strings.Contains(url, "hooks.slack.com") {
		// Slack format
		slackPayload := map[string]string{
			"text": fmt.Sprintf("Entity Matching %s\nBatch: %s\nAuto-matched: %d\nReview needed: %d\nMessage: %s",
				payload.Event, payload.BatchID, payload.AutoMatchedCount, payload.ReviewNeededCount, payload.Message),
		}
		bodyBytes, _ = json.Marshal(slackPayload)
		contentType = "application/json"
	} else if strings.Contains(url, "webhook.office.com") || strings.Contains(url, "office365") {
		// Teams format (MessageCard)
		teamsPayload := map[string]interface{}{
			"@type":      "MessageCard",
			"@context":   "https://schema.org/extensions",
			"summary":    payload.Event,
			"themeColor": "0078D4",
			"sections": []map[string]interface{}{
				{
					"activityTitle": fmt.Sprintf("Batch %s - %s", payload.BatchID, payload.Event),
					"facts": []map[string]string{
						{"name": "Auto-matched", "value": fmt.Sprintf("%d", payload.AutoMatchedCount)},
						{"name": "Review Needed", "value": fmt.Sprintf("%d", payload.ReviewNeededCount)},
						{"name": "Timestamp", "value": payload.Timestamp.Format(time.RFC3339)},
					},
					"text": payload.Message,
				},
			},
		}
		bodyBytes, _ = json.Marshal(teamsPayload)
		contentType = "application/json"
	} else {
		// Generic format
		bodyBytes, _ = json.Marshal(payload)
		contentType = "application/json"
	}

	// Retry logic: 3 attempts with exponential backoff
	const maxRetries = 3
	const baseDelay = 500 * time.Millisecond
	const maxDuration = 30 * time.Second

	deadline := time.Now().Add(maxDuration)

	for attempt := 0; attempt < maxRetries; attempt++ {
		if time.Now().After(deadline) {
			log.Printf("Webhook dispatch deadline exceeded for %s", url)
			return nil // Don't fail the batch
		}

		// Create request with remaining context deadline
		timeoutCtx, cancel := context.WithDeadline(ctx, deadline)
		req, err := http.NewRequestWithContext(timeoutCtx, "POST", url, bytes.NewBuffer(bodyBytes))
		if err != nil {
			cancel()
			continue
		}
		req.Header.Set("Content-Type", contentType)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		cancel()

		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil // Success
			}
		}

		// Exponential backoff
		if attempt < maxRetries-1 {
			delay := baseDelay * (1 << uint(attempt))
			if delay > 5*time.Second {
				delay = 5 * time.Second
			}
			time.Sleep(delay)
		}
	}

	// Log failure but don't block
	log.Printf("Webhook dispatch failed for %s after %d retries", url, maxRetries)
	return nil // Always return nil to avoid failing the batch
}
