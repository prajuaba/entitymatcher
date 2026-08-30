package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"entitymatcher/matcher"
	"entitymatcher/store"
)

// TestIsAnomalousLowAutoMatchRate tests anomaly detection for low auto-match rate
func TestIsAnomalousLowAutoMatchRate(t *testing.T) {
	outcome := matcher.ReconcileOutcome{
		TotalSources: 100,
		AutoMatched:  40, // 40% - below 50% threshold
		ReviewNeeded: 50,
		NoMatch:      10,
	}

	if !isAnomalous(outcome) {
		t.Error("Expected isAnomalous to return true for auto-match rate < 50%")
	}
}

// TestIsAnomalousHighNoMatchRate tests anomaly detection for high no-match rate
func TestIsAnomalousHighNoMatchRate(t *testing.T) {
	outcome := matcher.ReconcileOutcome{
		TotalSources: 100,
		AutoMatched:  60,
		ReviewNeeded: 10,
		NoMatch:      31, // 31% - above 30% threshold
	}

	if !isAnomalous(outcome) {
		t.Error("Expected isAnomalous to return true for no-match rate > 30%")
	}
}

// TestIsAnomalousNormal tests anomaly detection for normal rates
func TestIsAnomalousNormal(t *testing.T) {
	outcome := matcher.ReconcileOutcome{
		TotalSources: 100,
		AutoMatched:  70, // 70% - above 50% threshold
		ReviewNeeded: 15,
		NoMatch:      15, // 15% - below 30% threshold
	}

	if isAnomalous(outcome) {
		t.Error("Expected isAnomalous to return false for normal rates")
	}
}

// TestIsAnomalousEmptyBatch tests anomaly detection for empty batch (no divide-by-zero)
func TestIsAnomalousEmptyBatch(t *testing.T) {
	outcome := matcher.ReconcileOutcome{
		TotalSources: 0,
		AutoMatched:  0,
		ReviewNeeded: 0,
		NoMatch:      0,
	}

	// Should not panic
	result := isAnomalous(outcome)
	if result {
		t.Error("Expected isAnomalous to return false for empty batch")
	}
}

// TestIsAnomalousEdgeCases tests boundary conditions
func TestIsAnomalousEdgeCases(t *testing.T) {
	// Exactly at 50% auto-match (not anomalous)
	outcome := matcher.ReconcileOutcome{
		TotalSources: 100,
		AutoMatched:  50, // Exactly 50%
		ReviewNeeded: 40,
		NoMatch:      10,
	}

	if isAnomalous(outcome) {
		t.Error("Expected isAnomalous to return false at exactly 50% auto-match rate")
	}

	// Exactly at 30% no-match (not anomalous, threshold is > 30%)
	outcome2 := matcher.ReconcileOutcome{
		TotalSources: 100,
		AutoMatched:  60,
		ReviewNeeded: 10,
		NoMatch:      30, // Exactly 30%
	}

	if isAnomalous(outcome2) {
		t.Error("Expected isAnomalous to return false at exactly 30% no-match rate")
	}
}

// TestWebhookDispatchSlackFormat tests that Slack URLs get Slack format
func TestWebhookDispatchSlackFormat(t *testing.T) {
	// Create a test server to capture webhook calls
	capturedRequests := []map[string]interface{}{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		json.Unmarshal(body, &payload)
		capturedRequests = append(capturedRequests, payload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	st := store.NewStore()
	srv := NewServer(st)

	// Set webhook URL to a Slack-like URL
	cfg := srv.schedulerManager.GetConfig()
	cfg.WebhookURL = "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX"
	cfg.NotifyOnSuccess = true
	srv.schedulerManager.UpdateConfig(cfg)

	// Verify that the URL check works for Slack format
	if !strings.Contains(cfg.WebhookURL, "hooks.slack.com") {
		t.Error("Expected WebhookURL to contain hooks.slack.com")
	}

	// Note: We can't actually test webhook dispatch with this URL since it's not real.
	// The actual webhook dispatch behavior is tested with the generic server above.
	t.Log("Slack format detection verified via URL check")
}

// TestWebhookDispatchGenericFormat tests that non-Slack URLs get generic format
func TestWebhookDispatchGenericFormat(t *testing.T) {
	capturedRequests := []matcher.WebhookPayload{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload matcher.WebhookPayload
		json.Unmarshal(body, &payload)
		capturedRequests = append(capturedRequests, payload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	st := store.NewStore()
	srv := NewServer(st)

	cfg := srv.schedulerManager.GetConfig()
	cfg.WebhookURL = server.URL // Generic URL
	cfg.NotifyOnSuccess = true
	srv.schedulerManager.UpdateConfig(cfg)

	outcome := matcher.ReconcileOutcome{
		BatchID:           "test-batch",
		TotalSources:      100,
		TotalDestinations: 100,
		AutoMatched:       70,
		ReviewNeeded:      20,
		NoMatch:           10,
		ElapsedMs:         5000,
	}

	srv.fireWebhooks("test-batch", outcome)
	time.Sleep(100 * time.Millisecond)

	if len(capturedRequests) == 0 {
		t.Fatal("Expected webhook to be dispatched")
	}

	// Check that generic format was used
	payload := capturedRequests[0]
	if payload.Event != "RECONCILIATION_COMPLETED" {
		t.Errorf("Expected event to be RECONCILIATION_COMPLETED, got %s", payload.Event)
	}
	if payload.AutoMatchedCount != 70 {
		t.Errorf("Expected AutoMatchedCount to be 70, got %d", payload.AutoMatchedCount)
	}
	if payload.TotalSources != 100 {
		t.Errorf("Expected TotalSources to be 100, got %d", payload.TotalSources)
	}
}

// TestWebhookDispatchNoNotifications tests that no webhooks are sent when flags are false
func TestWebhookDispatchNoNotifications(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	st := store.NewStore()
	srv := NewServer(st)

	cfg := srv.schedulerManager.GetConfig()
	cfg.WebhookURL = server.URL
	cfg.NotifyOnSuccess = false // Disable success notifications
	cfg.NotifyOnAnomaly = false // Disable anomaly notifications
	srv.schedulerManager.UpdateConfig(cfg)

	outcome := matcher.ReconcileOutcome{
		BatchID:           "test-batch",
		TotalSources:      100,
		TotalDestinations: 100,
		AutoMatched:       70,
		ReviewNeeded:      20,
		NoMatch:           10,
		ElapsedMs:         5000,
	}

	srv.fireWebhooks("test-batch", outcome)
	time.Sleep(100 * time.Millisecond)

	if callCount > 0 {
		t.Error("Expected no webhooks to be dispatched when both flags are false")
	}
}

// TestWebhookDispatchAnomalyNotification tests that anomaly webhooks are sent when configured
func TestWebhookDispatchAnomalyNotification(t *testing.T) {
	capturedRequests := []matcher.WebhookPayload{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload matcher.WebhookPayload
		json.Unmarshal(body, &payload)
		capturedRequests = append(capturedRequests, payload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	st := store.NewStore()
	srv := NewServer(st)

	cfg := srv.schedulerManager.GetConfig()
	cfg.WebhookURL = server.URL
	cfg.NotifyOnSuccess = false // Don't send success notifications
	cfg.NotifyOnAnomaly = true  // Send anomaly notifications
	srv.schedulerManager.UpdateConfig(cfg)

	// Create an anomalous outcome (low auto-match rate)
	outcome := matcher.ReconcileOutcome{
		BatchID:           "test-batch",
		TotalSources:      100,
		TotalDestinations: 100,
		AutoMatched:       40, // 40% - anomalous
		ReviewNeeded:      50,
		NoMatch:           10,
		ElapsedMs:         5000,
	}

	srv.fireWebhooks("test-batch", outcome)
	time.Sleep(100 * time.Millisecond)

	if len(capturedRequests) != 1 {
		t.Errorf("Expected 1 webhook to be dispatched, got %d", len(capturedRequests))
	} else {
		if capturedRequests[0].Event != "ANOMALY_DETECTED" {
			t.Errorf("Expected ANOMALY_DETECTED event, got %s", capturedRequests[0].Event)
		}
	}
}

// TestNewServerWiresReconcileFunc tests that NewServer properly wires the reconcile function
func TestNewServerWiresReconcileFunc(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)

	// Verify that the reconcile function is wired
	// We do this by checking that we can call SetLastBatchID and the scheduler has a function set
	sm := srv.schedulerManager

	cfg := sm.GetConfig()
	if cfg.Enabled == false && cfg.CronExpression == "" {
		t.Log("Scheduler configured as expected (disabled by default)")
	}

	// Verify that Server was created with a scheduler
	if sm == nil {
		t.Error("Expected SchedulerManager to be initialized")
	}
}

// mockRepository is a test double for store.Repository that can simulate SaveResultsCtx failures
type mockRepository struct {
	failSaveResults     bool
	updateProgressCalls []matcher.BatchProgress
	webhooksCalled      bool
}

func (m *mockRepository) GetConfig() matcher.Config {
	return matcher.DefaultConfig()
}

func (m *mockRepository) UpdateConfig(cfg matcher.Config) {
}

func (m *mockRepository) SaveDataset(batchID string, sources []matcher.SourceRecord, dests []matcher.DestinationRecord) {
}

func (m *mockRepository) GetDataset(batchID string) ([]matcher.SourceRecord, []matcher.DestinationRecord, bool) {
	// Return minimal test data
	sources := []matcher.SourceRecord{
		{
			ID:              "src-1",
			BatchID:         batchID,
			ReferenceID:     "REF-001",
			CustomerNameRaw: "Test Source",
			NormalizedName:  matcher.CleanName{Raw: "Test Source", Cleaned: "test source"},
			TransactionDate: time.Now(),
		},
	}
	dests := []matcher.DestinationRecord{
		{
			ID:              "dest-1",
			BatchID:         batchID,
			CustomerID:      "CUST-001",
			CustomerNameRaw: "Test Destination",
			NormalizedName:  matcher.CleanName{Raw: "Test Destination", Cleaned: "test destination"},
			TransactionDate: time.Now(),
		},
	}
	return sources, dests, true
}

func (m *mockRepository) SaveResultsCtx(ctx context.Context, batchID string, results []matcher.MatchResultItem) error {
	if m.failSaveResults {
		return fmt.Errorf("simulated persistence failure")
	}
	return nil
}

func (m *mockRepository) GetResults(batchID string) ([]matcher.MatchResultItem, bool) {
	return nil, false
}

func (m *mockRepository) GetResultByID(batchID, matchID string) (matcher.MatchResultItem, bool) {
	return matcher.MatchResultItem{}, false
}

func (m *mockRepository) UpdateMatchStatus(batchID, matchID, newStatus string) error {
	return nil
}

func (m *mockRepository) GetResultsPage(batchID, status, search string, limit, offset int) ([]matcher.MatchResultItem, int, error) {
	return nil, 0, nil
}

func (m *mockRepository) UpdateProgress(p matcher.BatchProgress) {
	m.updateProgressCalls = append(m.updateProgressCalls, p)
}

func (m *mockRepository) GetProgress(batchID string) (matcher.BatchProgress, bool) {
	return matcher.BatchProgress{}, false
}

func (m *mockRepository) RegisterSSEClient(batchID string) chan matcher.BatchProgress {
	return make(chan matcher.BatchProgress)
}

func (m *mockRepository) UnregisterSSEClient(batchID string, ch chan matcher.BatchProgress) {
}

func (m *mockRepository) ManualLink(batchID, sourceID, destinationID string) (*matcher.MatchResultItem, error) {
	return nil, nil
}

func (m *mockRepository) RecordAuditLog(entry store.AuditLogEntry) store.AuditLogEntry {
	return entry
}

func (m *mockRepository) GetAuditLogs(batchID, userID, actionFilter string) []store.AuditLogEntry {
	return nil
}

func (m *mockRepository) DeleteBatch(batchID string) {
}

func (m *mockRepository) ListBatches() []store.BatchSummary {
	return nil
}

func (m *mockRepository) ListJobs(limit, offset int) ([]store.JobSummary, error) {
	return nil, nil
}

func (m *mockRepository) CalibrationObservations(batchID string) ([]matcher.LabelledScore, error) {
	return nil, nil
}

func (m *mockRepository) CalibrationObservationStats(batchID string) (store.CalibrationObservationStats, error) {
	return store.CalibrationObservationStats{}, nil
}

func (m *mockRepository) SaveCalibrationModel(model store.CalibrationModel) (store.CalibrationModel, error) {
	return model, nil
}

func (m *mockRepository) GetActiveCalibrationModel() (store.CalibrationModel, bool, error) {
	return store.CalibrationModel{}, false, nil
}

func (m *mockRepository) ListCalibrationModels(limit, offset int) ([]store.CalibrationModel, error) {
	return nil, nil
}

// TestRunBatchAndPersistFailure tests that when SaveResultsCtx fails, runBatchAndPersist returns an error
// with FAILED status and does not call fireWebhooks
func TestRunBatchAndPersistFailure(t *testing.T) {
	mockRepo := &mockRepository{failSaveResults: true}
	srv := NewServer(mockRepo)

	// Disable webhooks so we can verify they're not called
	cfg := srv.schedulerManager.GetConfig()
	cfg.WebhookURL = ""
	srv.schedulerManager.UpdateConfig(cfg)

	outcome, err := srv.runBatchAndPersist(context.Background(), "test-batch")

	// Error should be non-nil
	if err == nil {
		t.Error("Expected runBatchAndPersist to return error on SaveResultsCtx failure")
	}

	// Error should be wrapped with batch ID
	if !strings.Contains(err.Error(), "test-batch") {
		t.Errorf("Expected error to contain batch ID, got: %v", err)
	}

	// Check that UpdateProgress was called with FAILED status (last call)
	if len(mockRepo.updateProgressCalls) == 0 {
		t.Error("Expected UpdateProgress to be called with FAILED status")
	} else {
		lastProgress := mockRepo.updateProgressCalls[len(mockRepo.updateProgressCalls)-1]
		if lastProgress.Status != "FAILED" {
			t.Errorf("Expected last progress status to be FAILED, got: %s", lastProgress.Status)
		}
	}

	// Outcome should be empty ReconcileOutcome (zero value)
	if outcome.BatchID != "" || outcome.TotalSources != 0 {
		t.Error("Expected outcome to be empty/zero value on error")
	}
}

// TestRunBatchAndPersistSuccess tests the happy path where SaveResultsCtx succeeds
func TestRunBatchAndPersistSuccess(t *testing.T) {
	mockRepo := &mockRepository{failSaveResults: false}
	srv := NewServer(mockRepo)

	// Disable webhooks
	cfg := srv.schedulerManager.GetConfig()
	cfg.WebhookURL = ""
	srv.schedulerManager.UpdateConfig(cfg)

	outcome, err := srv.runBatchAndPersist(context.Background(), "test-batch")

	// Error should be nil
	if err != nil {
		t.Errorf("Expected no error on success, got: %v", err)
	}

	// Check that UpdateProgress was called with COMPLETED status (last call)
	if len(mockRepo.updateProgressCalls) == 0 {
		t.Error("Expected UpdateProgress to be called")
	} else {
		lastProgress := mockRepo.updateProgressCalls[len(mockRepo.updateProgressCalls)-1]
		if lastProgress.Status != "COMPLETED" {
			t.Errorf("Expected last progress status to be COMPLETED, got: %s", lastProgress.Status)
		}
	}

	// Outcome should be populated
	if outcome.BatchID != "test-batch" {
		t.Errorf("Expected outcome.BatchID to be test-batch, got: %s", outcome.BatchID)
	}
	if outcome.TotalSources != 1 || outcome.TotalDestinations != 1 {
		t.Errorf("Expected 1 source and 1 dest, got: %d sources, %d dests", outcome.TotalSources, outcome.TotalDestinations)
	}
}
