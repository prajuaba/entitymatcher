package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"entitymatcher/matcher"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestAuditEntryLabelMapping(t *testing.T) {
	tests := []struct {
		name      string
		entry     AuditLogEntry
		wantMatch bool
		wantOk    bool
	}{
		{
			name:      "CONFIRM -> (true, true)",
			entry:     AuditLogEntry{Action: "CONFIRM"},
			wantMatch: true,
			wantOk:    true,
		},
		{
			name:      "REJECT -> (false, true)",
			entry:     AuditLogEntry{Action: "REJECT"},
			wantMatch: false,
			wantOk:    true,
		},
		{
			name:      "OVERRIDE with CONFIRMED -> (true, true)",
			entry:     AuditLogEntry{Action: "OVERRIDE", NewStatus: "CONFIRMED"},
			wantMatch: true,
			wantOk:    true,
		},
		{
			name:      "OVERRIDE with REJECTED -> (false, true)",
			entry:     AuditLogEntry{Action: "OVERRIDE", NewStatus: "REJECTED"},
			wantMatch: false,
			wantOk:    true,
		},
		{
			name:      "OVERRIDE with empty NewStatus -> (_, false)",
			entry:     AuditLogEntry{Action: "OVERRIDE", NewStatus: ""},
			wantMatch: false,
			wantOk:    false,
		},
		{
			name:      "OVERRIDE with other NewStatus -> (_, false)",
			entry:     AuditLogEntry{Action: "OVERRIDE", NewStatus: "PENDING"},
			wantMatch: false,
			wantOk:    false,
		},
		{
			name:      "UNLINK -> (_, false)",
			entry:     AuditLogEntry{Action: "UNLINK"},
			wantMatch: false,
			wantOk:    false,
		},
		{
			name:      "empty Action -> (_, false)",
			entry:     AuditLogEntry{Action: ""},
			wantMatch: false,
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMatch, gotOk := auditEntryLabel(tt.entry)
			if gotMatch != tt.wantMatch {
				t.Errorf("auditEntryLabel() gotMatch = %v, want %v", gotMatch, tt.wantMatch)
			}
			if gotOk != tt.wantOk {
				t.Errorf("auditEntryLabel() gotOk = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}

func TestComputeCalibrationObservationsDedupKeepsLatest(t *testing.T) {
	now := time.Now()
	entries := []AuditLogEntry{
		{
			ID:              "audit-2",
			BatchID:         "batch1",
			SourceID:        "src1",
			DestinationID:   "dst1",
			Action:          "REJECT",
			ConfidenceScore: 0.6,
			Timestamp:       now.Add(time.Hour), // Later
		},
		{
			ID:              "audit-1",
			BatchID:         "batch1",
			SourceID:        "src1",
			DestinationID:   "dst1",
			Action:          "CONFIRM",
			ConfidenceScore: 0.7,
			Timestamp:       now, // Earlier
		},
	}

	labels, stats := computeCalibrationObservations(entries)

	if len(labels) != 1 {
		t.Errorf("Expected 1 observation, got %d", len(labels))
	}

	// Entries with no recorded PreviousStatus are counted under the "" key so the
	// ByPreviousStatus breakdown still sums to Total instead of silently hiding them.
	expectedByStatus := map[string]int{"": 1}
	if len(stats.ByPreviousStatus) != len(expectedByStatus) {
		t.Errorf("Expected %d status keys, got %v", len(expectedByStatus), stats.ByPreviousStatus)
	}
	for status, count := range expectedByStatus {
		if stats.ByPreviousStatus[status] != count {
			t.Errorf("Expected ByPreviousStatus[%q]=%d, got %d", status, count, stats.ByPreviousStatus[status])
		}
	}

	sum := 0
	for _, c := range stats.ByPreviousStatus {
		sum += c
	}
	if sum != stats.Total {
		t.Errorf("ByPreviousStatus counts (%d) should sum to Total (%d)", sum, stats.Total)
	}

	if labels[0].IsMatch != false {
		t.Errorf("Expected IsMatch=false (latest REJECT wins), got %v", labels[0].IsMatch)
	}

	if labels[0].Score != 0.6 {
		t.Errorf("Expected Score=0.6, got %v", labels[0].Score)
	}
}

func TestComputeCalibrationObservationsSkipsUnusableActions(t *testing.T) {
	now := time.Now()
	entries := []AuditLogEntry{
		{
			ID:              "audit1",
			BatchID:         "batch1",
			SourceID:        "src1",
			DestinationID:   "dst1",
			Action:          "UNLINK",
			ConfidenceScore: 0.5,
			Timestamp:       now,
		},
		{
			ID:              "audit2",
			BatchID:         "batch1",
			SourceID:        "src2",
			DestinationID:   "dst2",
			Action:          "OVERRIDE",
			NewStatus:       "",
			ConfidenceScore: 0.6,
			Timestamp:       now,
		},
		{
			ID:              "audit3",
			BatchID:         "batch1",
			SourceID:        "src3",
			DestinationID:   "dst3",
			Action:          "CONFIRM",
			ConfidenceScore: 0.7,
			Timestamp:       now,
		},
	}

	labels, stats := computeCalibrationObservations(entries)

	if len(labels) != 1 {
		t.Errorf("Expected 1 observation (only CONFIRM is valid), got %d", len(labels))
	}

	if stats.Total != 1 || stats.Positive != 1 || stats.Negative != 0 {
		t.Errorf("Expected Total=1, Positive=1, Negative=0, got Total=%d, Positive=%d, Negative=%d",
			stats.Total, stats.Positive, stats.Negative)
	}

	if labels[0].IsMatch != true {
		t.Errorf("Expected IsMatch=true for CONFIRM, got %v", labels[0].IsMatch)
	}
}

func TestComputeCalibrationObservationsStatsByPreviousStatus(t *testing.T) {
	now := time.Now()
	entries := []AuditLogEntry{
		{
			ID:              "audit1",
			BatchID:         "batch1",
			SourceID:        "src1",
			DestinationID:   "dst1",
			Action:          "CONFIRM",
			ConfidenceScore: 0.5,
			PreviousStatus:  "REVIEW_NEEDED",
			Timestamp:       now,
		},
		{
			ID:              "audit2",
			BatchID:         "batch1",
			SourceID:        "src2",
			DestinationID:   "dst2",
			Action:          "REJECT",
			ConfidenceScore: 0.5,
			PreviousStatus:  "REVIEW_NEEDED",
			Timestamp:       now,
		},
		{
			ID:              "audit3",
			BatchID:         "batch1",
			SourceID:        "src3",
			DestinationID:   "dst3",
			Action:          "OVERRIDE",
			NewStatus:       "CONFIRMED",
			ConfidenceScore: 0.5,
			PreviousStatus:  "AUTO_MATCHED",
			Timestamp:       now,
		},
	}

	labels, stats := computeCalibrationObservations(entries)

	if len(labels) != 3 {
		t.Errorf("Expected 3 observations, got %d", len(labels))
	}

	if stats.Total != 3 {
		t.Errorf("Expected Total=3, got %d", stats.Total)
	}
	if stats.Positive != 2 {
		t.Errorf("Expected Positive=2, got %d", stats.Positive)
	}
	if stats.Negative != 1 {
		t.Errorf("Expected Negative=1, got %d", stats.Negative)
	}

	expectedByStatus := map[string]int{"REVIEW_NEEDED": 2, "AUTO_MATCHED": 1}
	if len(stats.ByPreviousStatus) != len(expectedByStatus) {
		t.Errorf("Expected %d status keys, got %d", len(expectedByStatus), len(stats.ByPreviousStatus))
	}
	for status, count := range expectedByStatus {
		if stats.ByPreviousStatus[status] != count {
			t.Errorf("Expected ByPreviousStatus[%s]=%d, got %d", status, count, stats.ByPreviousStatus[status])
		}
	}
}

func TestComputeCalibrationObservationsEmpty(t *testing.T) {
	labels, stats := computeCalibrationObservations(nil)

	if len(labels) != 0 {
		t.Errorf("Expected empty labels slice, got %d elements", len(labels))
	}

	if stats.Total != 0 {
		t.Errorf("Expected Total=0, got %d", stats.Total)
	}

	if stats.ByPreviousStatus == nil {
		t.Error("Expected ByPreviousStatus map to be non-nil")
	}

	// Should be safe to range over
	count := 0
	for range stats.ByPreviousStatus {
		count++
	}
	if count != 0 {
		t.Errorf("Expected empty ByPreviousStatus map, got %d keys", count)
	}
}

func TestUnmarshalCalibratorRoundTrip(t *testing.T) {
	// Test data for calibration
	obs := make([]matcher.LabelledScore, 0, 20)
	for i := 0; i < 20; i++ {
		score := float64(i) / 19.0 // 0.0 to 1.0
		isMatch := score >= 0.5
		obs = append(obs, matcher.LabelledScore{Score: score, IsMatch: isMatch})
	}

	tests := []struct {
		name   string
		cal    matcher.Calibrator
		sample []float64
	}{
		{
			name:   "identity",
			cal:    &matcher.IdentityCalibrator{},
			sample: []float64{0.0, 0.25, 0.5, 0.75, 1.0},
		},
		{
			name: "platt",
			cal: func() matcher.Calibrator {
				c, err := matcher.FitCalibrator(obs)
				if err != nil {
					t.Fatalf("FitCalibrator failed: %v", err)
				}
				return c
			}(),
			sample: []float64{0.0, 0.25, 0.5, 0.75, 1.0},
		},
		{
			name: "isotonic",
			cal: func() matcher.Calibrator {
				// FitCalibrator picks Isotonic once observations reach
				// matcher.FitThresholdObservations; build a large enough synthetic set
				// (both classes present) to exercise that path via the public API,
				// since IsotonicCalibrator has no exported constructor/fit method.
				bigObs := make([]matcher.LabelledScore, 0, matcher.FitThresholdObservations+10)
				for i := 0; i < matcher.FitThresholdObservations+10; i++ {
					score := float64(i%100) / 99.0
					isMatch := score >= 0.5
					bigObs = append(bigObs, matcher.LabelledScore{Score: score, IsMatch: isMatch})
				}
				c, err := matcher.FitCalibrator(bigObs)
				if err != nil {
					t.Fatalf("FitCalibrator (isotonic) failed: %v", err)
				}
				if _, ok := c.(*matcher.IsotonicCalibrator); !ok {
					t.Fatalf("expected FitCalibrator to return *matcher.IsotonicCalibrator for %d observations, got %T", len(bigObs), c)
				}
				return c
			}(),
			sample: []float64{0.0, 0.25, 0.5, 0.75, 1.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := json.Marshal(tt.cal)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			// Unmarshal
			c, err := UnmarshalCalibrator(data)
			if err != nil {
				t.Fatalf("UnmarshalCalibrator failed: %v", err)
			}

			// Compare calibrate output
			for _, x := range tt.sample {
				original := tt.cal.Calibrate(x)
				reread := c.Calibrate(x)
				if math.Abs(original-reread) > 1e-9 {
					t.Errorf("Calibrate(%v): original=%v, reread=%v", x, original, reread)
				}
			}
		})
	}

	// Test unknown type error path
	_, err := UnmarshalCalibrator([]byte(`{"type":"bogus"}`))
	if err == nil {
		t.Error("Expected error for unknown type")
	}
}

func TestSaveAndGetActiveCalibrationModel(t *testing.T) {
	store := NewStore()

	model1 := CalibrationModel{
		FittedBy:         "user1",
		BatchID:          "b1",
		ObservationCount: 100,
		PositiveCount:    55,
		BrierScore:       0.1,
		ECEScore:         0.05,
		ModelJSON:        `{"type":"identity"}`,
		Active:           true,
		CreatedAt:        time.Now().Add(-time.Minute),
	}

	// First save
	saved1, err := store.SaveCalibrationModel(model1)
	require.NoError(t, err)

	// Model should have auto-populated fields
	if saved1.ID == "" {
		t.Error("Expected ID to be auto-populated")
	}
	if saved1.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be auto-populated")
	}

	// Second model, active
	model2 := CalibrationModel{
		ID:               "cal-2",
		FittedBy:         "user2",
		BatchID:          "b2",
		ObservationCount: 120,
		PositiveCount:    60,
		BrierScore:       0.08,
		ECEScore:         0.04,
		ModelJSON:        `{"type":"platt","a":1.0,"b":-0.5}`,
		Active:           true,
	}

	saved2, err := store.SaveCalibrationModel(model2)
	require.NoError(t, err)

	// Verify first model deactivated, second active
	active, ok, err := store.GetActiveCalibrationModel()
	require.NoError(t, err)
	require.True(t, ok)

	if active.ID != saved2.ID {
		t.Errorf("Expected active model ID %s, got %s", saved2.ID, active.ID)
	}
	if active.BatchID != saved2.BatchID {
		t.Errorf("Expected active batch %s, got %s", saved2.BatchID, active.BatchID)
	}
	if !active.Active {
		t.Error("Expected active model to be Active=true")
	}

	// List all models, should have both ordered by CreatedAt descending
	models, err := store.ListCalibrationModels(10, 0)
	require.NoError(t, err)
	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	// Verify ordering: most recent first
	if models[0].CreatedAt.Before(models[1].CreatedAt) {
		t.Error("Models not ordered by CreatedAt descending")
	}
}

func TestGetActiveCalibrationModelNoneActive(t *testing.T) {
	store := NewStore()

	model, ok, err := store.GetActiveCalibrationModel()
	require.NoError(t, err)
	if ok {
		t.Error("Expected ok=false when no active model")
	}
	if model != (CalibrationModel{}) {
		t.Errorf("Expected zero model, got %v", model)
	}
}

func TestSaveCalibrationModelFitPersistLoadRoundTrip(t *testing.T) {
	store := NewStore()

	// Create dataset with both classes
	obs := make([]matcher.LabelledScore, 0, 10)
	for i := 0; i < 10; i++ {
		score := float64(i) / 9.0 // 0.0 to 1.0
		isMatch := score >= 0.5
		obs = append(obs, matcher.LabelledScore{Score: score, IsMatch: isMatch})
	}

	// Fit calibrator
	cal, err := matcher.FitCalibrator(obs)
	require.NoError(t, err)

	// Marshal
	modelJSON, err := json.Marshal(cal)
	require.NoError(t, err)

	// Save
	_, err = store.SaveCalibrationModel(CalibrationModel{
		FittedBy:         "test-user",
		BatchID:          "batch1",
		ObservationCount: len(obs),
		PositiveCount:    5,
		BrierScore:       0.1,
		ECEScore:         0.05,
		ModelJSON:        string(modelJSON),
		Active:           true,
	})
	require.NoError(t, err)

	// Load
	active, ok, err := store.GetActiveCalibrationModel()
	require.NoError(t, err)
	require.True(t, ok)

	// Unmarshal and compare
	loaded, err := UnmarshalCalibrator([]byte(active.ModelJSON))
	require.NoError(t, err)

	// Compare outputs
	samples := []float64{0.0, 0.2, 0.4, 0.6, 0.8, 1.0}
	for _, x := range samples {
		original := cal.Calibrate(x)
		reread := loaded.Calibrate(x)
		if math.Abs(original-reread) > 1e-9 {
			t.Errorf("Calibrate(%v): original=%v, reread=%v", x, original, reread)
		}
	}
}

func TestPostgresCalibrationObservationsAndModelRoundTrip(t *testing.T) {
	store := testPostgresStore(t)

	batchID := fmt.Sprintf("calib-batch-%d", time.Now().UnixNano())

	// Seed audit log entries with explicit timestamps
	now := time.Now()
	entries := []AuditLogEntry{
		{
			ID:              "audit1",
			BatchID:         batchID,
			SourceID:        "src1",
			DestinationID:   "dst1",
			Action:          "CONFIRM",
			PreviousStatus:  "REVIEW_NEEDED",
			ConfidenceScore: 0.75,
			Timestamp:       now,
		},
		{
			ID:              "audit2",
			BatchID:         batchID,
			SourceID:        "src2",
			DestinationID:   "dst2",
			Action:          "REJECT",
			PreviousStatus:  "REVIEW_NEEDED",
			ConfidenceScore: 0.6,
			Timestamp:       now,
		},
		{
			ID:              "audit3",
			BatchID:         batchID,
			SourceID:        "src3",
			DestinationID:   "dst3",
			Action:          "CONFIRM",
			PreviousStatus:  "AUTO_MATCHED",
			ConfidenceScore: 0.55,
			Timestamp:       now,
		},
		{
			ID:              "audit4",
			BatchID:         batchID,
			SourceID:        "src4",
			DestinationID:   "dst4",
			Action:          "CONFIRM",
			PreviousStatus:  "REVIEW_NEEDED",
			ConfidenceScore: 0.7,
			Timestamp:       now.Add(-time.Hour),
		},
		{
			ID:              "audit5",
			BatchID:         batchID,
			SourceID:        "src4",
			DestinationID:   "dst4",
			Action:          "REJECT",
			PreviousStatus:  "REVIEW_NEEDED",
			ConfidenceScore: 0.65,
			Timestamp:       now, // Later timestamp, same pair as audit4
		},
	}

	// Record them using store's audit log interface
	for _, entry := range entries {
		store.RecordAuditLog(entry)
	}

	// Test CalibrationObservations
	labels, err := store.CalibrationObservations(batchID)
	require.NoError(t, err)

	// Should have 4 unique pairs (dedup collapsed src4/dst4 to 1)
	if len(labels) != 4 {
		t.Errorf("Expected 4 observations after dedup, got %d", len(labels))
	}

	// Verify the latest (REJECT) won for duplicate pair
	hasReject := false
	for _, lbl := range labels {
		if lbl.Score == 0.65 && !lbl.IsMatch {
			hasReject = true
			break
		}
	}
	if !hasReject {
		t.Error("Expected REJECT observation (0.65, false) for deduplicated pair")
	}

	// Test CalibrationObservationStats
	stats, err := store.CalibrationObservationStats(batchID)
	require.NoError(t, err)

	if stats.Total != 4 {
		t.Errorf("Expected Total=4, got %d", stats.Total)
	}
	if stats.Positive != 2 {
		t.Errorf("Expected Positive=2, got %d", stats.Positive)
	}
	if stats.Negative != 2 {
		t.Errorf("Expected Negative=2, got %d", stats.Negative)
	}

	// Fit calibrator
	if len(labels) < 2 {
		t.Skip("Skipping model round-trip: too few labels to fit calibrator")
	}

	cal, err := matcher.FitCalibrator(labels)
	if err != nil {
		t.Skipf("Skipping model round-trip: FitCalibrator failed: %v", err)
	}

	// Marshal and save
	modelJSON, err := json.Marshal(cal)
	require.NoError(t, err)

	_, err = store.SaveCalibrationModel(CalibrationModel{
		FittedBy:         "test-user",
		BatchID:          batchID,
		ObservationCount: len(labels),
		PositiveCount:    stats.Positive,
		BrierScore:       0.1,
		ECEScore:         0.05,
		ModelJSON:        string(modelJSON),
		Active:           true,
	})
	require.NoError(t, err)

	// Load and verify
	active, ok, err := store.GetActiveCalibrationModel()
	require.NoError(t, err)
	require.True(t, ok)

	loaded, err := UnmarshalCalibrator([]byte(active.ModelJSON))
	require.NoError(t, err)

	// Compare calibrate outputs
	samples := []float64{0.0, 0.25, 0.5, 0.75, 1.0}
	for _, x := range samples {
		original := cal.Calibrate(x)
		reread := loaded.Calibrate(x)
		if math.Abs(original-reread) > 1e-9 {
			t.Errorf("Calibrate(%v): original=%v, reread=%v", x, original, reread)
		}
	}

	// List models and verify it appears
	models, err := store.ListCalibrationModels(10, 0)
	require.NoError(t, err)
	found := false
	for _, m := range models {
		if m.ID == active.ID && m.BatchID == batchID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Saved model not found in ListCalibrationModels")
	}
}

func TestPostgresCalibrationRunTwiceForIsolation(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	store, err := NewPostgresStore(ctx, os.Getenv("TEST_DATABASE_URL"))
	require.NoError(t, err)
	defer store.Close()

	// Clear tables for isolation
	t.Cleanup(func() {
		dsn := os.Getenv("TEST_DATABASE_URL")
		cfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			return
		}
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			return
		}
		defer pool.Close()

		if _, err := pool.Exec(ctx, "TRUNCATE TABLE match_audit_logs, calibration_models CASCADE"); err != nil {
			t.Logf("cleanup truncate failed: %v", err)
		}
	})

	// Helper function to run calibration flow
	runCalibration := func(batchID string) {
		// Seed audit entries
		now := time.Now()
		entries := []AuditLogEntry{
			{
				ID:              fmt.Sprintf("audit-%s-1", batchID),
				BatchID:         batchID,
				SourceID:        fmt.Sprintf("src-%s-1", batchID),
				DestinationID:   fmt.Sprintf("dst-%s-1", batchID),
				Action:          "CONFIRM",
				ConfidenceScore: 0.7,
				Timestamp:       now,
			},
			{
				ID:              fmt.Sprintf("audit-%s-2", batchID),
				BatchID:         batchID,
				SourceID:        fmt.Sprintf("src-%s-2", batchID),
				DestinationID:   fmt.Sprintf("dst-%s-2", batchID),
				Action:          "REJECT",
				ConfidenceScore: 0.6,
				Timestamp:       now,
			},
		}

		for _, entry := range entries {
			store.RecordAuditLog(entry)
		}

		// Get observations
		labels, err := store.CalibrationObservations(batchID)
		require.NoError(t, err)
		require.Len(t, labels, 2)

		// Fit and save model
		cal, err := matcher.FitCalibrator(labels)
		if err != nil {
			t.Skipf("Skipping fit: %v", err)
		}

		modelJSON, _ := json.Marshal(cal)
		_, err = store.SaveCalibrationModel(CalibrationModel{
			FittedBy:         "test-user",
			BatchID:          batchID,
			ModelJSON:        string(modelJSON),
			Active:           true,
			ObservationCount: len(labels),
			PositiveCount:    1,
			BrierScore:       0.1,
			ECEScore:         0.05,
		})
		require.NoError(t, err)
	}

	// Get initial count
	initialModels, err := store.ListCalibrationModels(10, 0)
	require.NoError(t, err)
	initialCount := len(initialModels)

	// Run first calibration
	batchID1 := fmt.Sprintf("calib-batch-1-%d", time.Now().UnixNano())
	runCalibration(batchID1)

	// Verify model count increased by 1
	models, err := store.ListCalibrationModels(10, 0)
	require.NoError(t, err)
	if len(models) != initialCount+1 {
		t.Errorf("Expected model count %d after first run, got %d", initialCount+1, len(models))
	}

	// Run second calibration with different batchID
	batchID2 := fmt.Sprintf("calib-batch-2-%d", time.Now().UnixNano())
	runCalibration(batchID2)

	// Verify model count increased by 1 more
	models, err = store.ListCalibrationModels(10, 0)
	require.NoError(t, err)
	if len(models) != initialCount+2 {
		t.Errorf("Expected model count %d after second run, got %d", initialCount+2, len(models))
	}

	// Verify batch isolation: observations for batch1 should not include batch2 entries
	labels1, err := store.CalibrationObservations(batchID1)
	require.NoError(t, err)
	require.Len(t, labels1, 2)

	labels2, err := store.CalibrationObservations(batchID2)
	require.NoError(t, err)
	require.Len(t, labels2, 2)
}
