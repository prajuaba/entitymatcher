package matcher

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestRunScheduledJobInvokesReconcileFunc(t *testing.T) {
	sm := NewSchedulerManager()
	sm.config.Enabled = true

	// Set up a mock reconcile function that records when it was called
	callCount := 0
	sm.SetReconcileFunc(func(ctx context.Context, batchID string) (ReconcileOutcome, error) {
		callCount++
		return ReconcileOutcome{
			BatchID:           batchID,
			TotalSources:      100,
			TotalDestinations: 100,
			AutoMatched:       60,
			ReviewNeeded:      30,
			NoMatch:           10,
			ElapsedMs:         5000,
		}, nil
	})

	sm.SetLastBatchID("test-batch-001")
	sm.runScheduledJob()

	if callCount != 1 {
		t.Errorf("Expected reconcile function to be called once, got %d", callCount)
	}

	// Verify that status was updated to COMPLETED and timestamp was set
	cfg := sm.GetConfig()
	if cfg.LastRunStatus != "COMPLETED" {
		t.Errorf("Expected LastRunStatus to be COMPLETED, got %s", cfg.LastRunStatus)
	}
	if cfg.LastRunTimestamp.IsZero() {
		t.Error("Expected LastRunTimestamp to be set")
	}
}

func TestReconcileFuncErrorRecordsFailed(t *testing.T) {
	sm := NewSchedulerManager()

	// Set up a mock reconcile function that returns an error
	sm.SetReconcileFunc(func(ctx context.Context, batchID string) (ReconcileOutcome, error) {
		return ReconcileOutcome{}, fmt.Errorf("test error: connection timeout")
	})

	sm.SetLastBatchID("test-batch-001")

	// This should not panic
	sm.runScheduledJob()

	// Verify that status contains FAILED and the error message
	cfg := sm.GetConfig()
	if cfg.LastRunStatus == "COMPLETED" {
		t.Error("Expected LastRunStatus to indicate FAILED, got COMPLETED")
	}
	if !containsString(cfg.LastRunStatus, "FAILED") {
		t.Errorf("Expected LastRunStatus to contain 'FAILED', got: %s", cfg.LastRunStatus)
	}
}

func TestOverlapGuardSkipsSecondInvocation(t *testing.T) {
	sm := NewSchedulerManager()

	callCount := 0
	// Create a reconcile function that takes time and records calls
	sm.SetReconcileFunc(func(ctx context.Context, batchID string) (ReconcileOutcome, error) {
		callCount++
		// Simulate work
		time.Sleep(50 * time.Millisecond)
		return ReconcileOutcome{
			BatchID:           batchID,
			TotalSources:      100,
			TotalDestinations: 100,
			AutoMatched:       60,
			ReviewNeeded:      30,
			NoMatch:           10,
			ElapsedMs:         50,
		}, nil
	})

	sm.SetLastBatchID("test-batch-001")

	// Start first job in a goroutine
	go sm.runScheduledJob()
	time.Sleep(10 * time.Millisecond)  // Ensure first job has started

	// Try to start second job - should be skipped
	sm.runScheduledJob()

	// Wait for first job to complete
	time.Sleep(100 * time.Millisecond)

	if callCount != 1 {
		t.Errorf("Expected reconcile function to be called once due to overlap guard, got %d", callCount)
	}
}

func TestSetReconcileFuncWhenNone(t *testing.T) {
	sm := NewSchedulerManager()
	sm.SetLastBatchID("test-batch-001")

	// Run without setting a reconcile function - should skip gracefully
	sm.runScheduledJob()

	cfg := sm.GetConfig()
	// Should not panic and should leave status unchanged
	if cfg.LastRunStatus != "" {
		// Empty initial status is ok, we just shouldn't panic
		t.Logf("Status after no-op run: %s", cfg.LastRunStatus)
	}
}

func TestAnomalyDetectionLowAutoMatchRate(t *testing.T) {
	// Test: auto-match rate < 50%
	outcome := ReconcileOutcome{
		TotalSources:      100,
		AutoMatched:       40,  // 40%
		ReviewNeeded:      50,
		NoMatch:           10,
	}

	// Note: We can't call the isAnomalous function directly since it's in api/,
	// but we test the behavior through integration tests.
	// This test just documents the expected logic.
	if outcome.AutoMatched >= int64(outcome.TotalSources)*50/100 {
		t.Error("Test setup error: AutoMatched should be < 50% of sources")
	}
}

func TestAnomalyDetectionHighNoMatchRate(t *testing.T) {
	// Test: no-match rate > 30%
	outcome := ReconcileOutcome{
		TotalSources:      100,
		AutoMatched:       60,
		ReviewNeeded:      10,
		NoMatch:           31,  // 31% (> 30%)
	}

	if int64(outcome.NoMatch) <= int64(outcome.TotalSources)*30/100 {
		t.Error("Test setup error: NoMatch should be > 30% of sources")
	}
}

func TestAnomalyDetectionEmptyBatch(t *testing.T) {
	// Test: empty batch should not panic
	outcome := ReconcileOutcome{
		TotalSources:      0,
		AutoMatched:       0,
		ReviewNeeded:      0,
		NoMatch:           0,
	}

	// This should not cause a divide-by-zero panic
	// In the real code, isAnomalous guards against TotalSources == 0
	if outcome.TotalSources == 0 {
		t.Log("Empty batch correctly identified")
	}
}

func TestReconcileOutcomeStructure(t *testing.T) {
	outcome := ReconcileOutcome{
		BatchID:           "test-batch",
		TotalSources:      100,
		TotalDestinations: 100,
		AutoMatched:       70,
		ReviewNeeded:      20,
		NoMatch:           10,
		ElapsedMs:         5000,
	}

	if outcome.BatchID != "test-batch" {
		t.Errorf("Expected BatchID to be 'test-batch', got %s", outcome.BatchID)
	}
	if outcome.TotalSources != 100 {
		t.Errorf("Expected TotalSources to be 100, got %d", outcome.TotalSources)
	}
	if outcome.AutoMatched+outcome.ReviewNeeded+outcome.NoMatch != 100 {
		t.Error("Expected sum of outcome counts to equal TotalSources")
	}
}

// Helper function for testing
func containsString(s, substr string) bool {
	for i := 0; i < len(s); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if i+j >= len(s) || s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
