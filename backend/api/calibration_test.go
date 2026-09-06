package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"entitymatcher/store"
)

func TestCalibrationFitRBACForbiddenNonAdmin(t *testing.T) {
	server := &Server{}
	st := store.NewStore()
	server.store = st

	handler := RequireAuth(RequireRole(RoleAdmin)(http.HandlerFunc(server.HandleCalibrationFit)))

	// Get token for non-admin user
	token, err := generateToken(sampleUsers["reviewer_sarah"])
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/calibration/fit", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403, got %d", w.Code)
	}
}

func TestCalibrationFitUnauthorizedNoToken(t *testing.T) {
	server := &Server{}
	st := store.NewStore()
	server.store = st

	handler := RequireAuth(RequireRole(RoleAdmin)(http.HandlerFunc(server.HandleCalibrationFit)))

	req := httptest.NewRequest("POST", "/api/calibration/fit", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

func TestCalibrationStatusRBACForbiddenNonAdmin(t *testing.T) {
	server := &Server{}
	st := store.NewStore()
	server.store = st

	handler := RequireAuth(RequireRole(RoleAdmin)(http.HandlerFunc(server.HandleCalibrationStatus)))

	// Get token for non-admin user
	token, err := generateToken(sampleUsers["engineer_alex"])
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/calibration/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403, got %d", w.Code)
	}
}

func TestCalibrationStatusUnauthorizedNoToken(t *testing.T) {
	server := &Server{}
	st := store.NewStore()
	server.store = st

	handler := RequireAuth(RequireRole(RoleAdmin)(http.HandlerFunc(server.HandleCalibrationStatus)))

	req := httptest.NewRequest("GET", "/api/calibration/status", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

func TestCalibrationFitAdminAllowed(t *testing.T) {
	server := &Server{}
	st := store.NewStore()
	server.store = st

	handler := RequireAuth(RequireRole(RoleAdmin)(http.HandlerFunc(server.HandleCalibrationFit)))

	// Get token for admin user
	token, err := generateToken(sampleUsers["admin"])
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/calibration/fit", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}

	if w.Body.Len() == 0 {
		t.Fatal("Expected non-empty error response body")
	}
}

func TestCalibrationFitAndStatusQualityReport(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)

	// Record audit log entries with realistic mix
	now := time.Now()
	for i := 1; i <= 24; i++ {
		confidenceScore := 0.75
		action := "CONFIRM"
		newStatus := "CONFIRMED"
		prevStatus := "REVIEW_NEEDED"

		if i > 7 {
			confidenceScore = 0.65
			action = "REJECT"
			newStatus = "REJECTED"
			prevStatus = "REVIEW_NEEDED"
			if i == 24 {
				prevStatus = "AUTO_MATCHED"
			}
		}

		st.RecordAuditLog(store.AuditLogEntry{
			ID:              fmt.Sprintf("log-%d", i),
			BatchID:         "batch-001",
			SourceID:        fmt.Sprintf("src-%d", i),
			DestinationID:   fmt.Sprintf("dst-%d", i),
			UserID:          "usr-03",
			Action:          action,
			PreviousStatus:  prevStatus,
			NewStatus:       newStatus,
			ConfidenceScore: confidenceScore,
			ReviewComments:  "",
			Timestamp:       now.Add(time.Duration(i) * time.Second),
		})
	}

	// Call HandleCalibrationFit directly
	req := httptest.NewRequest("POST", "/api/calibration/fit", bytes.NewBufferString(`{"batch_id": ""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.HandleCalibrationFit(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var fitResp CalibrationFitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &fitResp); err != nil {
		t.Fatalf("Failed to unmarshal fit response: %v", err)
	}

	// Validate fit response
	if fitResp.Status != "fitted" {
		t.Fatalf("Expected status 'fitted', got '%s'", fitResp.Status)
	}

	if fitResp.ModelID == "" {
		t.Fatal("Expected non-empty ModelID")
	}

	if fitResp.ObservationCount != fitResp.PositiveCount+fitResp.NegativeCount {
		t.Errorf("ObservationCount (%d) != PositiveCount (%d) + NegativeCount (%d)",
			fitResp.ObservationCount, fitResp.PositiveCount, fitResp.NegativeCount)
	}

	if fitResp.PositiveCount <= 0 || fitResp.NegativeCount <= 0 {
		t.Errorf("Expected both positive and negative counts > 0, got %d positive, %d negative",
			fitResp.PositiveCount, fitResp.NegativeCount)
	}

	if fitResp.TrainCount+fitResp.HoldoutCount != fitResp.ObservationCount {
		t.Errorf("TrainCount (%d) + HoldoutCount (%d) != ObservationCount (%d)",
			fitResp.TrainCount, fitResp.HoldoutCount, fitResp.ObservationCount)
	}

	// Sum ByPreviousStatus
	sumByStatus := 0
	for _, count := range fitResp.ByPreviousStatus {
		sumByStatus += count
	}
	if sumByStatus != fitResp.ObservationCount {
		t.Errorf("ByPreviousStatus sum (%d) != ObservationCount (%d)", sumByStatus, fitResp.ObservationCount)
	}

	if !strings.Contains(strings.ToLower(fitResp.Caveat), "review queue") {
		t.Errorf("Caveat does not contain 'review queue': %s", fitResp.Caveat)
	}

	// Pretty-print the fit response
	if prettyJSON, err := json.MarshalIndent(fitResp, "", "  "); err == nil {
		t.Logf("Calibration fit response:\n%s", string(prettyJSON))
	}

	// Now test HandleCalibrationStatus
	reqStatus := httptest.NewRequest("GET", "/api/calibration/status", nil)
	wStatus := httptest.NewRecorder()
	srv.HandleCalibrationStatus(wStatus, reqStatus)

	if wStatus.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", wStatus.Code)
	}

	var statusResp CalibrationStatusResponse
	if err := json.Unmarshal(wStatus.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("Failed to unmarshal status response: %v", err)
	}

	if !statusResp.HasActiveModel {
		t.Fatal("Expected HasActiveModel to be true")
	}

	if statusResp.ActiveModel == nil {
		t.Fatal("Expected non-nil ActiveModel")
	}

	if statusResp.ActiveModel.ID != fitResp.ModelID {
		t.Errorf("ActiveModel.ID '%s' != fit ModelID '%s'", statusResp.ActiveModel.ID, fitResp.ModelID)
	}

	if statusResp.ObservationCount != fitResp.ObservationCount {
		t.Errorf("Status ObservationCount (%d) != fit ObservationCount (%d)",
			statusResp.ObservationCount, fitResp.ObservationCount)
	}

	if !strings.Contains(strings.ToLower(statusResp.Caveat), "review queue") {
		t.Errorf("Caveat does not contain 'review queue': %s", statusResp.Caveat)
	}
}

func TestCalibrationFitTooFewObservationsReturns400(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)

	// Record exactly 19 audit log entries (MinCalibrationObservations - 1)
	now := time.Now()
	for i := 1; i <= 19; i++ {
		action := "REJECT"
		newStatus := "REJECTED"
		if i%2 == 0 {
			action = "CONFIRM"
			newStatus = "CONFIRMED"
		}

		st.RecordAuditLog(store.AuditLogEntry{
			ID:              fmt.Sprintf("log-%d", i),
			BatchID:         "batch-001",
			SourceID:        fmt.Sprintf("src-%d", i),
			DestinationID:   fmt.Sprintf("dst-%d", i),
			UserID:          "usr-03",
			Action:          action,
			PreviousStatus:  "REVIEW_NEEDED",
			NewStatus:       newStatus,
			ConfidenceScore: 0.9 - float64(i)*0.05,
			Timestamp:       now.Add(time.Duration(i) * time.Second),
		})
	}

	req := httptest.NewRequest("POST", "/api/calibration/fit", bytes.NewBufferString(`{"batch_id": ""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.HandleCalibrationFit(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "insufficient calibration observations") {
		t.Errorf("Response body does not contain 'insufficient calibration observations': %s", body)
	}
	if !strings.Contains(body, "20") {
		t.Errorf("Response body does not mention required count 20: %s", body)
	}

	// Assert no model was persisted
	_, hasActive, err := st.GetActiveCalibrationModel()
	if err != nil {
		t.Fatalf("Failed to get active calibration model: %v", err)
	}
	if hasActive {
		t.Fatal("Expected no active model to be persisted")
	}
}

func TestCalibrationFitAtMinimumObservationsSucceeds(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)

	// Record exactly 20 audit log entries (MinCalibrationObservations)
	now := time.Now()
	for i := 1; i <= 20; i++ {
		action := "REJECT"
		newStatus := "REJECTED"
		if i%2 == 0 {
			action = "CONFIRM"
			newStatus = "CONFIRMED"
		}

		st.RecordAuditLog(store.AuditLogEntry{
			ID:              fmt.Sprintf("log-%d", i),
			BatchID:         "batch-001",
			SourceID:        fmt.Sprintf("src-%d", i),
			DestinationID:   fmt.Sprintf("dst-%d", i),
			UserID:          "usr-03",
			Action:          action,
			PreviousStatus:  "REVIEW_NEEDED",
			NewStatus:       newStatus,
			ConfidenceScore: 0.9 - float64(i)*0.05,
			Timestamp:       now.Add(time.Duration(i) * time.Second),
		})
	}

	// Get token for admin user
	token, err := generateToken(sampleUsers["admin"])
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/calibration/fit", bytes.NewBufferString(`{"batch_id": ""}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	handler := RequireAuth(RequireRole(RoleAdmin)(http.HandlerFunc(srv.HandleCalibrationFit)))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Assert a model is now active
	_, hasActive, err := st.GetActiveCalibrationModel()
	if err != nil {
		t.Fatalf("Failed to get active calibration model: %v", err)
	}
	if !hasActive {
		t.Fatal("Expected an active model to be persisted")
	}
}

func TestCalibrationFitAllOneClassReturns400(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)

	// Record multiple audit log entries but all CONFIRM
	now := time.Now()
	for i := 1; i <= 6; i++ {
		st.RecordAuditLog(store.AuditLogEntry{
			ID:              "log-" + string(rune('a'+i-1)),
			BatchID:         "batch-001",
			SourceID:        "src-" + string(rune('1'+i-1)),
			DestinationID:   "dst-" + string(rune('1'+i-1)),
			UserID:          "usr-03",
			Action:          "CONFIRM",
			PreviousStatus:  "REVIEW_NEEDED",
			NewStatus:       "CONFIRMED",
			ConfidenceScore: 0.9 - float64(i)*0.05,
			Timestamp:       now.Add(time.Duration(i) * time.Second),
		})
	}

	req := httptest.NewRequest("POST", "/api/calibration/fit", bytes.NewBufferString(`{"batch_id": ""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.HandleCalibrationFit(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}

	// Assert no model was persisted
	_, hasActive, err := st.GetActiveCalibrationModel()
	if err != nil {
		t.Fatalf("Failed to get active calibration model: %v", err)
	}
	if hasActive {
		t.Fatal("Expected no active model to be persisted")
	}
}
