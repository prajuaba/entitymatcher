package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"entitymatcher/matcher"
	"entitymatcher/store"
)

// TestSchedulerConfigSurvivesPOSTthenGET tests that scheduler config survives POST then GET.
func TestSchedulerConfigSurvivesPOSTthenGET(t *testing.T) {
	memStore := store.NewStore()
	server := NewServer(memStore)

	// POST a scheduler config
	schedulerCfg := matcher.SchedulerConfig{
		Enabled:         true,
		CronExpression:  "*/5 * * * *",
		WebhookURL:      "https://hooks.slack.com/services/webhook",
		NotifyOnSuccess: true,
		NotifyOnAnomaly: true,
	}

	body, _ := json.Marshal(schedulerCfg)
	req := httptest.NewRequest("POST", "/api/scheduler/config", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleSchedulerConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Now GET the scheduler config
	req = httptest.NewRequest("GET", "/api/scheduler/config", nil)
	w = httptest.NewRecorder()

	server.HandleSchedulerConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var retrievedCfg matcher.SchedulerConfig
	json.Unmarshal(w.Body.Bytes(), &retrievedCfg)

	if retrievedCfg.CronExpression != "*/5 * * * *" {
		t.Fatalf("Expected cron_expression '*/5 * * * *', got %s", retrievedCfg.CronExpression)
	}

	if retrievedCfg.WebhookURL != "https://hooks.slack.com/services/webhook" {
		t.Fatalf("Expected webhook_url set, got %s", retrievedCfg.WebhookURL)
	}

	if !retrievedCfg.Enabled {
		t.Fatal("Expected enabled to be true")
	}
}

// TestSchedulerConfigValidatesCronExpression tests cron validation.
func TestSchedulerConfigValidatesCronExpression(t *testing.T) {
	memStore := store.NewStore()
	server := NewServer(memStore)

	// POST with invalid cron expression
	schedulerCfg := matcher.SchedulerConfig{
		Enabled:        true,
		CronExpression: "invalid cron expression",
	}

	body, _ := json.Marshal(schedulerCfg)
	req := httptest.NewRequest("POST", "/api/scheduler/config", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleSchedulerConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for invalid cron, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSchedulerConfigValidCronExpressions tests various valid cron expressions.
func TestSchedulerConfigValidCronExpressions(t *testing.T) {
	memStore := store.NewStore()
	server := NewServer(memStore)

	validExpressions := []string{
		"0 2 * * *",     // Daily at 2 AM
		"*/5 * * * *",   // Every 5 minutes
		"0 0 * * 0",     // Weekly on Sunday at midnight
		"0 0 1 * *",     // Monthly on the 1st
		"@hourly",       // Hourly
		"@daily",        // Daily
	}

	for _, expr := range validExpressions {
		schedulerCfg := matcher.SchedulerConfig{
			Enabled:        true,
			CronExpression: expr,
		}

		body, _ := json.Marshal(schedulerCfg)
		req := httptest.NewRequest("POST", "/api/scheduler/config", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		server.HandleSchedulerConfig(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200 for cron expression '%s', got %d: %s", expr, w.Code, w.Body.String())
		}
	}
}

// TestSchedulerConfigDisabledNoValidation tests that cron is not validated when disabled.
func TestSchedulerConfigDisabledNoValidation(t *testing.T) {
	memStore := store.NewStore()
	server := NewServer(memStore)

	// POST with invalid cron but disabled=false, should still be validated
	schedulerCfg := matcher.SchedulerConfig{
		Enabled:        false,
		CronExpression: "this is invalid",
	}

	body, _ := json.Marshal(schedulerCfg)
	req := httptest.NewRequest("POST", "/api/scheduler/config", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleSchedulerConfig(w, req)

	// When disabled and cron is invalid, it's not validated
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 when disabled, got %d: %s", w.Code, w.Body.String())
	}
}
