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

// TestConfigMergePutsUnchangedFields tests that partial PUT preserves existing weights.
func TestConfigMergePutsUnchangedFields(t *testing.T) {
	memStore := store.NewStore()
	server := NewServer(memStore)

	// First, set initial config with weights
	initialCfg := matcher.Config{
		AutoMatchThreshold:  0.90,
		ReviewThreshold:     0.70,
		DateToleranceDays:   30,
		Weights: matcher.MatchWeights{
			NameWeight: 0.7,
			DateWeight: 0.3,
		},
		WorkerCount:        4,
		MaxCandidatesPerSrc: 50,
	}
	memStore.UpdateConfig(initialCfg)

	// Now do a partial PUT that changes only auto_match_threshold
	partialUpdate := map[string]interface{}{
		"auto_match_threshold": 0.1,
		"review_threshold":     0.05,
		"date_tolerance_days":  30,
	}

	body, _ := json.Marshal(partialUpdate)
	req := httptest.NewRequest("PUT", "/api/config", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check that weights were preserved
	var respCfg matcher.Config
	json.Unmarshal(w.Body.Bytes(), &respCfg)

	if respCfg.Weights.NameWeight != 0.7 {
		t.Fatalf("Expected name_weight 0.7, got %f", respCfg.Weights.NameWeight)
	}

	if respCfg.Weights.DateWeight != 0.3 {
		t.Fatalf("Expected date_weight 0.3, got %f", respCfg.Weights.DateWeight)
	}

	// Verify that new values were applied
	if respCfg.AutoMatchThreshold != 0.1 {
		t.Fatalf("Expected auto_match_threshold 0.1, got %f", respCfg.AutoMatchThreshold)
	}
}

// TestConfigValidationThresholdRange tests threshold range validation.
func TestConfigValidationThresholdRange(t *testing.T) {
	memStore := store.NewStore()
	server := NewServer(memStore)

	// Try to set auto_match_threshold > 1
	partialUpdate := map[string]interface{}{
		"auto_match_threshold": 1.5,
	}

	body, _ := json.Marshal(partialUpdate)
	req := httptest.NewRequest("PUT", "/api/config", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}
}

// TestConfigValidationReviewThreshold tests review_threshold <= auto_match_threshold.
func TestConfigValidationReviewThreshold(t *testing.T) {
	memStore := store.NewStore()
	server := NewServer(memStore)

	// Set auto_match_threshold to 0.5
	memStore.UpdateConfig(matcher.Config{
		AutoMatchThreshold:  0.5,
		ReviewThreshold:     0.3,
		DateToleranceDays:   30,
		Weights:             matcher.DefaultWeights,
		WorkerCount:         4,
		MaxCandidatesPerSrc: 50,
	})

	// Try to set review_threshold > auto_match_threshold
	partialUpdate := map[string]interface{}{
		"review_threshold": 0.8,
	}

	body, _ := json.Marshal(partialUpdate)
	req := httptest.NewRequest("PUT", "/api/config", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestConfigValidationWeights tests that weights must be > 0.
func TestConfigValidationWeights(t *testing.T) {
	memStore := store.NewStore()
	server := NewServer(memStore)

	// Try to set name_weight to 0
	partialUpdate := map[string]interface{}{
		"weights": map[string]interface{}{
			"name_weight": 0.0,
			"date_weight": 0.3,
		},
	}

	body, _ := json.Marshal(partialUpdate)
	req := httptest.NewRequest("PUT", "/api/config", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestConfigValidationWorkerCount tests worker_count range.
func TestConfigValidationWorkerCount(t *testing.T) {
	memStore := store.NewStore()
	server := NewServer(memStore)

	// Try to set worker_count to 0
	partialUpdate := map[string]interface{}{
		"worker_count": 0,
	}

	body, _ := json.Marshal(partialUpdate)
	req := httptest.NewRequest("PUT", "/api/config", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// Try to set worker_count > 256
	partialUpdate = map[string]interface{}{
		"worker_count": 300,
	}

	body, _ = json.Marshal(partialUpdate)
	req = httptest.NewRequest("PUT", "/api/config", bytes.NewBuffer(body))
	w = httptest.NewRecorder()

	server.HandleConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestConfigValidationMaxCandidates tests max_candidates_per_src range.
func TestConfigValidationMaxCandidates(t *testing.T) {
	memStore := store.NewStore()
	server := NewServer(memStore)

	// Try to set max_candidates_per_src > 1000
	partialUpdate := map[string]interface{}{
		"max_candidates_per_src": 2000,
	}

	body, _ := json.Marshal(partialUpdate)
	req := httptest.NewRequest("PUT", "/api/config", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestConfigValidationAssignmentStrategy tests assignment_strategy values.
func TestConfigValidationAssignmentStrategy(t *testing.T) {
	memStore := store.NewStore()
	server := NewServer(memStore)

	// Try to set invalid assignment_strategy
	partialUpdate := map[string]interface{}{
		"assignment_strategy": "INVALID_STRATEGY",
	}

	body, _ := json.Marshal(partialUpdate)
	req := httptest.NewRequest("PUT", "/api/config", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// Valid values should work
	validStrategies := []string{"GREEDY_1_1", "TOP_1", "ALL_CANDIDATES"}
	for _, strategy := range validStrategies {
		partialUpdate = map[string]interface{}{
			"assignment_strategy": strategy,
		}

		body, _ = json.Marshal(partialUpdate)
		req = httptest.NewRequest("PUT", "/api/config", bytes.NewBuffer(body))
		w = httptest.NewRecorder()

		server.HandleConfig(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200 for strategy %s, got %d", strategy, w.Code)
		}
	}
}
