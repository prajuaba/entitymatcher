package api

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"entitymatcher/internal/mockdata"
	"entitymatcher/matcher"
	"entitymatcher/store"
)

type Server struct {
	store            store.Repository
	llmResolver      *matcher.LLMResolver
	schedulerManager *matcher.SchedulerManager
	calibratorMu     sync.RWMutex
	calibrator       matcher.Calibrator
}

func NewServer(st store.Repository) *Server {
	srv := &Server{
		store:            st,
		llmResolver:      matcher.NewLLMResolver(),
		schedulerManager: matcher.NewSchedulerManager(),
	}

	// Wire the reconcile function so scheduled jobs actually run matching
	srv.schedulerManager.SetReconcileFunc(func(ctx context.Context, batchID string) (matcher.ReconcileOutcome, error) {
		return srv.runBatchAndPersist(ctx, batchID)
	})

	return srv
}

// StopScheduler gracefully stops the scheduler during shutdown.
func (s *Server) StopScheduler() {
	if s.schedulerManager != nil {
		s.schedulerManager.Stop()
	}
}

// SetCalibrator installs cal as the calibrator used by subsequent match runs (each run
// constructs a fresh matcher.MatchEngine, so the engine can't hold this itself — the Server
// holds it and wires it into each new engine in runBatchAndPersist). Passing nil clears it.
func (s *Server) SetCalibrator(cal matcher.Calibrator) {
	s.calibratorMu.Lock()
	defer s.calibratorMu.Unlock()
	s.calibrator = cal
}

// GetCalibrator returns the currently installed calibrator, or nil if none has been set.
func (s *Server) GetCalibrator() matcher.Calibrator {
	s.calibratorMu.RLock()
	defer s.calibratorMu.RUnlock()
	return s.calibrator
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

// validateAndMergeConfig merges a partial config update into the existing config.
// Returns the merged config and an error if validation fails.
func validateAndMergeConfig(existing matcher.Config, update map[string]json.RawMessage) (matcher.Config, error) {
	// Start with existing config
	merged := existing

	// Helper to parse a float64 value
	parseFloat := func(raw json.RawMessage) (*float64, error) {
		var v float64
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return &v, nil
	}

	// Helper to parse an int value
	parseInt := func(raw json.RawMessage) (*int, error) {
		var v int
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return &v, nil
	}

	// Helper to parse a string value
	parseString := func(raw json.RawMessage) (*string, error) {
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return &v, nil
	}

	// Helper to parse a bool value
	parseBool := func(raw json.RawMessage) (*bool, error) {
		var v bool
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return &v, nil
	}

	// Helper to parse MatchWeights
	parseWeights := func(raw json.RawMessage) (*matcher.MatchWeights, error) {
		var v matcher.MatchWeights
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return &v, nil
	}

	// Helper to parse AlgorithmToggles
	parseAlgorithms := func(raw json.RawMessage) (*matcher.AlgorithmToggles, error) {
		var v matcher.AlgorithmToggles
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return &v, nil
	}

	// Helper to parse ColumnMapping
	parseColumnMapping := func(raw json.RawMessage) (*matcher.ColumnMapping, error) {
		var v matcher.ColumnMapping
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return &v, nil
	}

	// Process each field that is present in the update
	if raw, exists := update["auto_match_threshold"]; exists {
		val, err := parseFloat(raw)
		if err != nil {
			return merged, fmt.Errorf("invalid auto_match_threshold: %v", err)
		}
		if val != nil {
			if *val < 0 || *val > 1 {
				return merged, fmt.Errorf("auto_match_threshold must be between 0 and 1")
			}
			merged.AutoMatchThreshold = *val
		}
	}

	if raw, exists := update["cross_script_auto_threshold"]; exists {
		val, err := parseFloat(raw)
		if err != nil {
			return merged, fmt.Errorf("invalid cross_script_auto_threshold: %v", err)
		}
		if val != nil {
			if *val < 0 || *val > 1 {
				return merged, fmt.Errorf("cross_script_auto_threshold must be between 0 and 1")
			}
			// 0 is explicitly allowed and means "unset, use auto_match_threshold"
			merged.CrossScriptAutoThreshold = *val
		}
	}

	if raw, exists := update["review_threshold"]; exists {
		val, err := parseFloat(raw)
		if err != nil {
			return merged, fmt.Errorf("invalid review_threshold: %v", err)
		}
		if val != nil {
			if *val < 0 || *val > 1 {
				return merged, fmt.Errorf("review_threshold must be between 0 and 1")
			}
			merged.ReviewThreshold = *val
		}
	}

	if raw, exists := update["margin_threshold"]; exists {
		val, err := parseFloat(raw)
		if err != nil {
			return merged, fmt.Errorf("invalid margin_threshold: %v", err)
		}
		if val != nil {
			if *val < 0 || *val > 1 {
				return merged, fmt.Errorf("margin_threshold must be between 0 and 1")
			}
			merged.MarginThreshold = *val
		}
	}

	if raw, exists := update["no_distinctive_overlap_cap"]; exists {
		val, err := parseFloat(raw)
		if err != nil {
			return merged, fmt.Errorf("invalid no_distinctive_overlap_cap: %v", err)
		}
		if val != nil {
			if *val < 0 || *val > 1 {
				return merged, fmt.Errorf("no_distinctive_overlap_cap must be between 0 and 1")
			}
			merged.NoDistinctiveOverlapCap = *val
		}
	}

	if raw, exists := update["distinctive_overlap_min_weight"]; exists {
		val, err := parseFloat(raw)
		if err != nil {
			return merged, fmt.Errorf("invalid distinctive_overlap_min_weight: %v", err)
		}
		if val != nil {
			if *val < 0 || *val > 1 {
				return merged, fmt.Errorf("distinctive_overlap_min_weight must be between 0 and 1")
			}
			merged.DistinctiveOverlapMinWeight = *val
		}
	}

	if raw, exists := update["date_tolerance_days"]; exists {
		val, err := parseInt(raw)
		if err != nil {
			return merged, fmt.Errorf("invalid date_tolerance_days: %v", err)
		}
		if val != nil {
			if *val < 0 {
				return merged, fmt.Errorf("date_tolerance_days must be >= 0")
			}
			merged.DateToleranceDays = *val
		}
	}

	if raw, exists := update["worker_count"]; exists {
		val, err := parseInt(raw)
		if err != nil {
			return merged, fmt.Errorf("invalid worker_count: %v", err)
		}
		if val != nil {
			if *val < 1 || *val > 256 {
				return merged, fmt.Errorf("worker_count must be between 1 and 256")
			}
			merged.WorkerCount = *val
		}
	}

	if raw, exists := update["max_candidates_per_src"]; exists {
		val, err := parseInt(raw)
		if err != nil {
			return merged, fmt.Errorf("invalid max_candidates_per_src: %v", err)
		}
		if val != nil {
			if *val < 1 || *val > 1000 {
				return merged, fmt.Errorf("max_candidates_per_src must be between 1 and 1000")
			}
			merged.MaxCandidatesPerSrc = *val
		}
	}

	if raw, exists := update["assignment_strategy"]; exists {
		val, err := parseString(raw)
		if err != nil {
			return merged, fmt.Errorf("invalid assignment_strategy: %v", err)
		}
		if val != nil {
			valid := false
			for _, strategy := range []string{"GREEDY_1_1", "TOP_1", "ALL_CANDIDATES"} {
				if *val == strategy {
					valid = true
					break
				}
			}
			if !valid {
				return merged, fmt.Errorf("assignment_strategy must be one of GREEDY_1_1, TOP_1, ALL_CANDIDATES")
			}
			merged.AssignmentStrategy = *val
		}
	}

	if raw, exists := update["emit_unmatched"]; exists {
		val, err := parseBool(raw)
		if err != nil {
			return merged, fmt.Errorf("invalid emit_unmatched: %v", err)
		}
		if val != nil {
			merged.EmitUnmatched = *val
		}
	}

	if raw, exists := update["weights"]; exists {
		val, err := parseWeights(raw)
		if err != nil {
			return merged, fmt.Errorf("invalid weights: %v", err)
		}
		if val != nil {
			if val.NameWeight <= 0 || val.DateWeight <= 0 {
				return merged, fmt.Errorf("name_weight and date_weight must both be > 0")
			}
			merged.Weights = *val
		}
	}

	if raw, exists := update["algorithms"]; exists {
		val, err := parseAlgorithms(raw)
		if err != nil {
			return merged, fmt.Errorf("invalid algorithms: %v", err)
		}
		if val != nil {
			merged.Algorithms = *val
		}
	}

	if raw, exists := update["column_mapping"]; exists {
		val, err := parseColumnMapping(raw)
		if err != nil {
			return merged, fmt.Errorf("invalid column_mapping: %v", err)
		}
		if val != nil {
			merged.ColumnMapping = *val
		}
	}

	// Validate cross-field constraints
	if merged.ReviewThreshold > merged.AutoMatchThreshold {
		return merged, fmt.Errorf("review_threshold must be <= auto_match_threshold")
	}

	// A cap at or above the auto-match bar cannot demote anything, so the rule
	// would silently do nothing. Refuse the combination rather than accept a
	// setting that looks active but is not.
	if merged.NoDistinctiveOverlapCap > 0 && merged.NoDistinctiveOverlapCap >= merged.AutoMatchThreshold {
		return merged, fmt.Errorf("no_distinctive_overlap_cap must be < auto_match_threshold, otherwise it can never demote a match")
	}

	// 0 is explicitly allowed and skips both checks below (means unset, falls back to auto_match_threshold)
	if merged.CrossScriptAutoThreshold != 0 {
		if merged.CrossScriptAutoThreshold > merged.AutoMatchThreshold {
			return merged, fmt.Errorf("cross_script_auto_threshold must be <= auto_match_threshold (it is a lower bar by design)")
		}
		if merged.CrossScriptAutoThreshold < merged.ReviewThreshold {
			return merged, fmt.Errorf("cross_script_auto_threshold must be >= review_threshold (auto-matching below the review bar is incoherent)")
		}
	}

	return merged, nil
}

// isAnomalous checks if a reconciliation run shows anomalous behavior.
func isAnomalous(outcome matcher.ReconcileOutcome) bool {
	if outcome.TotalSources == 0 {
		return false
	}
	autoMatchRate := float64(outcome.AutoMatched) / float64(outcome.TotalSources)
	noMatchRate := float64(outcome.NoMatch) / float64(outcome.TotalSources)
	return autoMatchRate < 0.5 || noMatchRate > 0.3
}

// runBatchAndPersist executes a reconciliation job, saves results, and fires webhooks.
// Called by both the HTTP handler and the scheduled job.
func (s *Server) runBatchAndPersist(ctx context.Context, batchID string) (matcher.ReconcileOutcome, error) {
	sources, dests, ok := s.store.GetDataset(batchID)
	if !ok {
		return matcher.ReconcileOutcome{}, fmt.Errorf("dataset for batch_id not found")
	}

	cfg := s.store.GetConfig()
	engine := matcher.NewMatchEngine(cfg)
	if cal := s.GetCalibrator(); cal != nil {
		engine.SetCalibrator(cal)
	}

	startTime := time.Now()
	results, progress := engine.ExecuteJob(
		ctx,
		batchID,
		sources,
		dests,
		func(p matcher.BatchProgress) {
			s.store.UpdateProgress(p)
		},
	)
	elapsed := time.Since(startTime)
	elapsedMs := elapsed.Milliseconds()

	// Persist results with error handling
	err := s.store.SaveResultsCtx(ctx, batchID, results)
	if err != nil {
		// Create FAILED progress with all fields from engine's progress
		failedProgress := progress
		failedProgress.Status = "FAILED"
		s.store.UpdateProgress(failedProgress)
		return matcher.ReconcileOutcome{}, fmt.Errorf("batch %s: persist results failed: %w", batchID, err)
	}

	s.store.UpdateProgress(progress)

	// Count match statuses
	autoMatched := int64(0)
	reviewNeeded := int64(0)
	noMatch := int64(0)
	for _, r := range results {
		switch r.MatchStatus {
		case "AUTO_MATCHED", "CONFIRMED":
			autoMatched++
		case "REVIEW_NEEDED":
			reviewNeeded++
		case "NO_MATCH":
			noMatch++
		}
	}

	outcome := matcher.ReconcileOutcome{
		BatchID:           batchID,
		TotalSources:      len(sources),
		TotalDestinations: len(dests),
		AutoMatched:       autoMatched,
		ReviewNeeded:      reviewNeeded,
		NoMatch:           noMatch,
		ElapsedMs:         elapsedMs,
	}

	// Fire webhooks asynchronously only after successful persistence
	go s.fireWebhooks(batchID, outcome)

	return outcome, nil
}

// fireWebhooks dispatches webhooks for a completed batch based on config.
func (s *Server) fireWebhooks(batchID string, outcome matcher.ReconcileOutcome) {
	cfg := s.schedulerManager.GetConfig()

	// Gate on webhook URL + NotifyOn* flags, NOT on Enabled
	if cfg.WebhookURL == "" {
		return
	}

	// Create background context for webhook dispatch
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Dispatch RECONCILIATION_COMPLETED if NotifyOnSuccess is set
	if cfg.NotifyOnSuccess {
		payload := matcher.WebhookPayload{
			Event:             "RECONCILIATION_COMPLETED",
			BatchID:           batchID,
			TotalSources:      outcome.TotalSources,
			TotalDestinations: outcome.TotalDestinations,
			AutoMatchedCount:  outcome.AutoMatched,
			ReviewNeededCount: outcome.ReviewNeeded,
			Timestamp:         time.Now(),
			Message:           fmt.Sprintf("Batch %s reconciliation completed with %d auto-matched results", batchID, outcome.AutoMatched),
		}
		if outcome.ElapsedMs > 0 {
			payload.ThroughputPerSec = float64(outcome.TotalSources) / (float64(outcome.ElapsedMs) / 1000.0)
		}
		if err := s.schedulerManager.DispatchWebhook(ctx, payload); err != nil {
			log.Printf("Failed to dispatch success webhook: %v", err)
		}
	}

	// Dispatch ANOMALY_DETECTED if NotifyOnAnomaly is set AND run is anomalous
	if cfg.NotifyOnAnomaly && isAnomalous(outcome) {
		payload := matcher.WebhookPayload{
			Event:             "ANOMALY_DETECTED",
			BatchID:           batchID,
			TotalSources:      outcome.TotalSources,
			TotalDestinations: outcome.TotalDestinations,
			AutoMatchedCount:  outcome.AutoMatched,
			ReviewNeededCount: outcome.ReviewNeeded,
			Timestamp:         time.Now(),
			Message: fmt.Sprintf("Anomaly detected in batch %s: auto-match rate %.1f%%, no-match rate %.1f%%",
				batchID,
				(float64(outcome.AutoMatched)/float64(outcome.TotalSources))*100,
				(float64(outcome.NoMatch)/float64(outcome.TotalSources))*100),
		}
		if outcome.ElapsedMs > 0 {
			payload.ThroughputPerSec = float64(outcome.TotalSources) / (float64(outcome.ElapsedMs) / 1000.0)
		}
		if err := s.schedulerManager.DispatchWebhook(ctx, payload); err != nil {
			log.Printf("Failed to dispatch anomaly webhook: %v", err)
		}
	}
}

func (s *Server) HandleConfig(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	if r.Method == "GET" {
		cfg := s.store.GetConfig()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
		return
	}

	if r.Method == "PUT" || r.Method == "POST" {
		// Decode into map[string]json.RawMessage for selective merge
		var updates map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			http.Error(w, "Invalid request JSON", http.StatusBadRequest)
			return
		}

		// Get existing config and merge with updates
		existing := s.store.GetConfig()
		merged, err := validateAndMergeConfig(existing, updates)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Store the merged config
		s.store.UpdateConfig(merged)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(merged)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

type DatasetPayload struct {
	BatchID       string                   `json:"batch_id"`
	ColumnMapping *matcher.ColumnMapping   `json:"column_mapping,omitempty"`
	Sources       []map[string]interface{} `json:"sources"`
	Destinations  []map[string]interface{} `json:"destinations"`
}

// buildSourceRecords converts raw source rows into matcher.SourceRecord values using cfg's
// column mapping. Extracted verbatim from HandleUpload's inline loop (no behavior change).
func buildSourceRecords(raw []map[string]interface{}, batchID string, cfg matcher.Config) []matcher.SourceRecord {
	sources := make([]matcher.SourceRecord, 0, len(raw))
	for i, rawMap := range raw {
		refID := matcher.ExtractFieldValue(rawMap, cfg.ColumnMapping.RefIDSrc)
		if refID == "" {
			refID = matcher.ExtractFieldValue(rawMap, "reference_id")
		}
		if refID == "" {
			refID = fmt.Sprintf("SRC-%04d", i+1)
		}

		nameStr := matcher.ExtractCompositeName(rawMap, cfg.ColumnMapping.NameFieldsSrc)

		dateStr := matcher.ExtractFieldValue(rawMap, cfg.ColumnMapping.DateFieldSrc)
		// ParseFlexibleDate, not time.Parse: the rigid "2006-01-02" layout silently
		// turned every DD/MM/YYYY, Buddhist-era and Thai-digit date into time.Now(),
		// which made the date component of the score meaningless.
		// Leave the zero value when there is no parseable date: the scorer needs
		// to tell "no date supplied" apart from a real one, and a fabricated
		// time.Now() is indistinguishable from a genuine same-day transaction.
		txDate, _ := matcher.ParseFlexibleDateInCalendar(dateStr, cfg.ColumnMapping.DateCalendarSrc)

		txType := matcher.ExtractFieldValue(rawMap, "transaction_type")

		sources = append(sources, matcher.SourceRecord{
			ID:              fmt.Sprintf("src-%d", i+1),
			BatchID:         batchID,
			ReferenceID:     refID,
			CustomerNameRaw: nameStr,
			NormalizedName:  matcher.Normalize(nameStr),
			TransactionDate: txDate,
			TransactionType: txType,
			Attributes:      rawMap,
		})
	}
	return sources
}

// buildDestinationRecords converts raw destination rows into matcher.DestinationRecord values
// using cfg's column mapping. Extracted verbatim from HandleUpload's inline loop (no behavior change).
func buildDestinationRecords(raw []map[string]interface{}, batchID string, cfg matcher.Config) []matcher.DestinationRecord {
	destinations := make([]matcher.DestinationRecord, 0, len(raw))
	for i, rawMap := range raw {
		custID := matcher.ExtractFieldValue(rawMap, cfg.ColumnMapping.RefIDDest)
		if custID == "" {
			custID = matcher.ExtractFieldValue(rawMap, "customer_id")
		}
		if custID == "" {
			custID = fmt.Sprintf("DEST-%04d", i+1)
		}

		nameStr := matcher.ExtractCompositeName(rawMap, cfg.ColumnMapping.NameFieldsDest)

		dateStr := matcher.ExtractFieldValue(rawMap, cfg.ColumnMapping.DateFieldDest)
		// ParseFlexibleDate, not time.Parse: the rigid "2006-01-02" layout silently
		// turned every DD/MM/YYYY, Buddhist-era and Thai-digit date into time.Now(),
		// which made the date component of the score meaningless.
		// Leave the zero value when there is no parseable date: the scorer needs
		// to tell "no date supplied" apart from a real one, and a fabricated
		// time.Now() is indistinguishable from a genuine same-day transaction.
		txDate, _ := matcher.ParseFlexibleDateInCalendar(dateStr, cfg.ColumnMapping.DateCalendarDest)

		destinations = append(destinations, matcher.DestinationRecord{
			ID:              fmt.Sprintf("dest-%d", i+1),
			BatchID:         batchID,
			CustomerID:      custID,
			CustomerNameRaw: nameStr,
			NormalizedName:  matcher.Normalize(nameStr),
			TransactionDate: txDate,
			Attributes:      rawMap,
		})
	}
	return destinations
}

func (s *Server) HandleUpload(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload DatasetPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Failed to parse JSON payload", http.StatusBadRequest)
		return
	}

	if payload.BatchID == "" {
		payload.BatchID = fmt.Sprintf("batch-%d", time.Now().UnixNano())
	}

	cfg := s.store.GetConfig()
	if payload.ColumnMapping != nil {
		cfg.ColumnMapping = *payload.ColumnMapping
		s.store.UpdateConfig(cfg)
	}

	sources := buildSourceRecords(payload.Sources, payload.BatchID, cfg)
	destinations := buildDestinationRecords(payload.Destinations, payload.BatchID, cfg)

	if err := s.store.SaveDataset(payload.BatchID, sources, destinations); err != nil {
		http.Error(w, "Failed to persist dataset: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Record the batch as the scheduler's current target
	s.schedulerManager.SetLastBatchID(payload.BatchID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":            "success",
		"batch_id":          payload.BatchID,
		"source_count":      len(sources),
		"destination_count": len(destinations),
		"column_mapping":    cfg.ColumnMapping,
	})
}

// MaxUploadBytes caps a single multipart ingestion request.
const MaxUploadBytes = 50 << 20 // 50 MiB

// MaxIngestRecordsEnv names the environment variable that overrides the ingest cap.
const MaxIngestRecordsEnv = "MAX_INGEST_RECORDS"

// defaultMaxIngestRecords is the cap applied when MAX_INGEST_RECORDS is unset.
const defaultMaxIngestRecords = 500000

// MaxIngestRecords bounds how many rows one ingest will read from a single
// source. It is a ceiling on memory: every row is held in memory and persisted
// as JSONB, so raising it trades RAM for completeness. A file that hits the cap
// is reported as truncated rather than silently short.
var MaxIngestRecords = resolveMaxIngestRecords()

func resolveMaxIngestRecords() int {
	raw := strings.TrimSpace(os.Getenv(MaxIngestRecordsEnv))
	if raw == "" {
		return defaultMaxIngestRecords
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		log.Printf("%s=%q is not a positive integer; using default %d", MaxIngestRecordsEnv, raw, defaultMaxIngestRecords)
		return defaultMaxIngestRecords
	}
	return n
}

// IngestPageSize is how many rows each FetchRecords call requests. Ingestion
// pages rather than asking for the whole table at once so a large source does
// not have to materialise in one driver round trip.
const IngestPageSize = 5000

// fetchAllRecords pages a connector to exhaustion, stopping at maxRecords.
// The bool reports whether rows remained beyond the cap: it is determined by
// asking for one more row, not by guessing from a full final page, so a source
// holding exactly maxRecords rows is not falsely reported as truncated.
//
// Paging is only meaningful over a stable order; the database connectors
// resolve a deterministic ORDER BY before fetching, which is what makes reading
// to exhaustion safe here.
func fetchAllRecords(ctx context.Context, conn matcher.DataConnector, maxRecords int) ([]map[string]interface{}, bool, error) {
	rows := make([]map[string]interface{}, 0)
	offset := 0

	for len(rows) < maxRecords {
		pageSize := IngestPageSize
		if maxRecords-len(rows) < pageSize {
			pageSize = maxRecords - len(rows)
		}
		page, err := conn.FetchRecords(ctx, pageSize, offset)
		if err != nil {
			return nil, false, err
		}
		if len(page) == 0 {
			break
		}
		rows = append(rows, page...)
		offset += len(page)
		if len(page) < pageSize {
			break
		}
	}

	truncated := false
	if len(rows) >= maxRecords {
		extra, err := conn.FetchRecords(ctx, 1, offset)
		if err != nil {
			return nil, false, err
		}
		truncated = len(extra) > 0
	}

	return rows, truncated, nil
}

// ConnectorFileRootEnv names the environment variable that confines
// caller-supplied connector file paths to one directory.
const ConnectorFileRootEnv = "CONNECTOR_FILE_ROOT"

// resolveConnectorFilePath confines a caller-supplied file path to the directory
// named by CONNECTOR_FILE_ROOT and returns the resolved absolute path.
//
// Without this the connector endpoints are an arbitrary file-read primitive: the
// caller names any path the server process can open and IntrospectSchema returns
// its header row.
//
// Unset means DENY. Refusing by default is the safe failure: an operator who has
// not chosen a directory has not agreed to expose one, and the alternative
// default -- the whole filesystem -- is the vulnerability itself.
//
// Symlinks are resolved BEFORE the containment check, so a symlink planted inside
// the root cannot point out of it. Both sides are resolved because the root
// itself may be reached through a symlink (/tmp on macOS, for one).
func resolveConnectorFilePath(path string) (string, error) {
	root := strings.TrimSpace(os.Getenv(ConnectorFileRootEnv))
	if root == "" {
		return "", fmt.Errorf("server-side file paths are disabled; set %s to a directory to enable them", ConnectorFileRootEnv)
	}

	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("file_path is required")
	}

	realRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("%s is not a usable directory: %w", ConnectorFileRootEnv, err)
	}
	realRoot, err = filepath.EvalSymlinks(realRoot)
	if err != nil {
		return "", fmt.Errorf("%s is not a usable directory: %w", ConnectorFileRootEnv, err)
	}

	var candidate string
	if filepath.IsAbs(path) {
		candidate = path
	} else {
		candidate = filepath.Join(realRoot, path)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("file not found or not readable: %s", path)
	}
	candidate, err = filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("file not found or not readable: %s", path)
	}

	if candidate == realRoot || strings.HasPrefix(candidate, realRoot+string(os.PathSeparator)) {
		info, err := os.Stat(candidate)
		if err != nil {
			return "", fmt.Errorf("file not found or not readable: %s", path)
		}
		if info.IsDir() {
			return "", fmt.Errorf("file_path is a directory, not a file")
		}
		return candidate, nil
	}

	return "", fmt.Errorf("file_path is outside the permitted directory")
}

// saveUploadedFileToTemp validates the extension of an uploaded multipart file, copies its
// content to a new temp file on disk, and returns the temp file's path. The caller owns the
// returned path and is responsible for os.Remove'ing it once done.
func saveUploadedFileToTemp(fh *multipart.FileHeader) (string, error) {
	multipartFile, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer multipartFile.Close()

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext != ".csv" && ext != ".xlsx" && ext != ".xls" {
		return "", fmt.Errorf("unsupported file extension %q: allowed extensions are .csv, .xlsx, .xls", ext)
	}

	tmpFile, err := os.CreateTemp("", "ingest-*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, multipartFile); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	// Close explicitly and check the error: a deferred Close would discard a
	// write error from the final flush, which would leave a silently truncated
	// file for the connector to parse.
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	return tmpPath, nil
}

// HandleUploadFile ingests a source and destination dataset from real CSV/Excel files
// submitted as multipart/form-data, using the matcher.CSVConnector / matcher.ExcelConnector
// FetchRecords implementations rather than accepting pre-parsed JSON arrays.
func (s *Server) HandleUploadFile(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes)
	err := r.ParseMultipartForm(MaxUploadBytes)
	if err != nil {
		http.Error(w, "Failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	batchID := r.FormValue("batch_id")
	if batchID == "" {
		batchID = fmt.Sprintf("batch-%d", time.Now().UnixNano())
	}

	columnMappingStr := r.FormValue("column_mapping")
	cfg := s.store.GetConfig()
	if columnMappingStr != "" {
		var cm matcher.ColumnMapping
		if err := json.Unmarshal([]byte(columnMappingStr), &cm); err != nil {
			http.Error(w, "Invalid column_mapping JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		cfg.ColumnMapping = cm
		s.store.UpdateConfig(cfg)
	}

	sourceFile, sourceHeader, err := r.FormFile("source_file")
	if err != nil {
		http.Error(w, "source_file is required: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer sourceFile.Close()

	sourcePath, err := saveUploadedFileToTemp(sourceHeader)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer os.Remove(sourcePath)

	sourceExt := strings.ToLower(filepath.Ext(sourceHeader.Filename))
	var sourceType matcher.SourceType
	if sourceExt == ".csv" {
		sourceType = matcher.SourceTypeCSV
	} else {
		sourceType = matcher.SourceTypeExcel
	}

	sourceSheet := r.FormValue("source_sheet")
	sourceConnCfg := matcher.ConnectionConfig{
		Type:     sourceType,
		FilePath: sourcePath,
	}
	if sourceType == matcher.SourceTypeExcel && sourceSheet != "" {
		sourceConnCfg.ExtraParams = map[string]interface{}{"sheet": sourceSheet}
	}

	sourceConn, err := matcher.NewDataConnector(sourceConnCfg)
	if err != nil {
		http.Error(w, "Failed to initialize connector for source_file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = sourceConn.Close() }()

	sourceRows, sourceTruncated, err := fetchAllRecords(r.Context(), sourceConn, MaxIngestRecords)
	if err != nil {
		http.Error(w, "Failed to read source_file: "+err.Error(), http.StatusBadRequest)
		return
	}

	destinationFile, destinationHeader, err := r.FormFile("destination_file")
	if err != nil {
		http.Error(w, "destination_file is required: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer destinationFile.Close()

	destinationPath, err := saveUploadedFileToTemp(destinationHeader)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer os.Remove(destinationPath)

	destinationExt := strings.ToLower(filepath.Ext(destinationHeader.Filename))
	var destType matcher.SourceType
	if destinationExt == ".csv" {
		destType = matcher.SourceTypeCSV
	} else {
		destType = matcher.SourceTypeExcel
	}

	destSheet := r.FormValue("destination_sheet")
	destConnCfg := matcher.ConnectionConfig{
		Type:     destType,
		FilePath: destinationPath,
	}
	if destType == matcher.SourceTypeExcel && destSheet != "" {
		destConnCfg.ExtraParams = map[string]interface{}{"sheet": destSheet}
	}

	destConn, err := matcher.NewDataConnector(destConnCfg)
	if err != nil {
		http.Error(w, "Failed to initialize connector for destination_file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = destConn.Close() }()

	destRows, destTruncated, err := fetchAllRecords(r.Context(), destConn, MaxIngestRecords)
	if err != nil {
		http.Error(w, "Failed to read destination_file: "+err.Error(), http.StatusBadRequest)
		return
	}

	truncated := sourceTruncated || destTruncated

	sources := buildSourceRecords(sourceRows, batchID, cfg)
	destinations := buildDestinationRecords(destRows, batchID, cfg)

	if err := s.store.SaveDataset(batchID, sources, destinations); err != nil {
		http.Error(w, "Failed to persist dataset: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.schedulerManager.SetLastBatchID(batchID)

	resp := map[string]interface{}{
		"status":            "success",
		"batch_id":          batchID,
		"source_count":      len(sources),
		"destination_count": len(destinations),
		"column_mapping":    cfg.ColumnMapping,
		"truncated":         truncated,
	}

	if truncated {
		resp["warning"] = fmt.Sprintf("one or more files returned the maximum of %d rows; additional rows may have been truncated. Increase MaxIngestRecords or split the file if this is unexpected.", MaxIngestRecords)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ingestableSourceTypes are the connector types this endpoint will read from.
// CSV and Excel are excluded on purpose -- see HandleConnectorIngest.
func validateIngestableType(t matcher.SourceType) error {
	switch t {
	case matcher.SourceTypePostgres, matcher.SourceTypeSQLServer, matcher.SourceTypeMongoDB:
		return nil
	case matcher.SourceTypeCSV, matcher.SourceTypeExcel:
		return fmt.Errorf("connector type %s cannot be ingested here; upload the file to /api/upload/file instead", t)
	default:
		return fmt.Errorf("unsupported connector type for ingestion: %q", t)
	}
}

// HandleConnectorIngest ingests a source and destination dataset from configured
// database connectors, paging each to exhaustion, and writes them as a batch a
// match run can then use.
//
// File-backed connector types are deliberately refused here. A .csv/.xlsx is
// ingested through POST /api/upload/file, which takes the bytes from the
// request; accepting a server-side file_path on this endpoint would let any
// ADMIN or ENGINEER read an arbitrary file off the server in full. Confining
// server-side reads to a configured directory is backlog item M1, and this
// endpoint should accept file paths only once that lands.
func (s *Server) HandleConnectorIngest(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		BatchID       string                   `json:"batch_id"`
		ColumnMapping *matcher.ColumnMapping   `json:"column_mapping"`
		Source        matcher.ConnectionConfig `json:"source"`
		Destination   matcher.ConnectionConfig `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateIngestableType(req.Source.Type); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateIngestableType(req.Destination.Type); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	batchID := req.BatchID
	if batchID == "" {
		batchID = fmt.Sprintf("batch-%d", time.Now().UnixNano())
	}

	cfg := s.store.GetConfig()
	if req.ColumnMapping != nil {
		cfg.ColumnMapping = *req.ColumnMapping
		s.store.UpdateConfig(cfg)
	}

	sourceConn, err := matcher.NewDataConnector(req.Source)
	if err != nil {
		http.Error(w, "Failed to initialize connector for source: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = sourceConn.Close() }()

	sourceRows, sourceTruncated, err := fetchAllRecords(r.Context(), sourceConn, MaxIngestRecords)
	if err != nil {
		http.Error(w, "Failed to read source: "+err.Error(), http.StatusBadRequest)
		return
	}

	destConn, err := matcher.NewDataConnector(req.Destination)
	if err != nil {
		http.Error(w, "Failed to initialize connector for destination: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = destConn.Close() }()

	destRows, destTruncated, err := fetchAllRecords(r.Context(), destConn, MaxIngestRecords)
	if err != nil {
		http.Error(w, "Failed to read destination: "+err.Error(), http.StatusBadRequest)
		return
	}

	sources := buildSourceRecords(sourceRows, batchID, cfg)
	destinations := buildDestinationRecords(destRows, batchID, cfg)

	if err := s.store.SaveDataset(batchID, sources, destinations); err != nil {
		http.Error(w, "Failed to persist dataset: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.schedulerManager.SetLastBatchID(batchID)

	resp := map[string]interface{}{
		"status":                "success",
		"batch_id":              batchID,
		"source_count":          len(sources),
		"destination_count":     len(destinations),
		"column_mapping":        cfg.ColumnMapping,
		"truncated":             sourceTruncated || destTruncated,
		"source_truncated":      sourceTruncated,
		"destination_truncated": destTruncated,
	}

	if sourceTruncated || destTruncated {
		resp["warning"] = fmt.Sprintf("ingestion stopped at the %d row cap and more rows remain; the batch is incomplete", MaxIngestRecords)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) HandleRunMatch(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	batchID := r.URL.Query().Get("batch_id")
	if batchID == "" {
		http.Error(w, "batch_id parameter is required", http.StatusBadRequest)
		return
	}

	// Run matching in background using the shared path
	go func() {
		// Use a reasonable timeout for the handler call
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()
		_, err := s.runBatchAndPersist(ctx, batchID)
		if err != nil {
			log.Printf("HandleRunMatch error for batch %s: %v", batchID, err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "matching_started",
		"batch_id": batchID,
	})
}

func (s *Server) HandleSSEProgress(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	batchID := r.URL.Query().Get("batch_id")
	if batchID == "" {
		http.Error(w, "batch_id required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := s.store.RegisterSSEClient(batchID)
	defer s.store.UnregisterSSEClient(batchID, ch)

	// Send initial progress if available
	if p, exists := s.store.GetProgress(batchID); exists {
		pData, _ := json.Marshal(p)
		fmt.Fprintf(w, "data: %s\n\n", pData)
		flusher.Flush()
	}

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case p, open := <-ch:
			if !open {
				return
			}
			pData, _ := json.Marshal(p)
			fmt.Fprintf(w, "data: %s\n\n", pData)
			flusher.Flush()
		}
	}
}

// HandleGetResults returns one page of match results for a batch. Filtering,
// sorting and paging all happen in the store layer as a single bounded SQL
// query (or the equivalent in-memory pass), so a page costs O(page size) work
// instead of loading and slicing the entire batch in this handler.
func (s *Server) HandleGetResults(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	batchID := r.URL.Query().Get("batch_id")
	if batchID == "" {
		http.Error(w, "batch_id required", http.StatusBadRequest)
		return
	}

	status := r.URL.Query().Get("status")
	// Trimmed but not lowercased: the store layer now owns case-insensitivity
	// (SQL ILIKE / Go strings.ToLower internally), so lowercasing here would
	// just be a redundant second transform.
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	sortBy := r.URL.Query().Get("sort_by")
	sortDir := r.URL.Query().Get("sort_dir")
	includeCounts := r.URL.Query().Get("include_counts") == "1" || r.URL.Query().Get("include_counts") == "true"

	// An absurd page number is clamped rather than rejected, matching how
	// Normalized() below already silently corrects an invalid sort field or an
	// oversized limit. The upper clamp specifically prevents (page-1)*q.Limit
	// from overflowing into a negative offset, which Postgres rejects outright.
	if page <= 0 {
		page = 1
	} else if page > store.MaxResultsPage {
		page = store.MaxResultsPage
	}

	// Normalized() clamps/defaults limit, sort field and sort direction, so the
	// effective values it produces -- not the raw query params -- are what get
	// echoed back in the response and used to compute the offset below.
	q := store.ResultsQuery{
		BatchID: batchID,
		Status:  status,
		Search:  search,
		SortBy:  sortBy,
		SortDir: sortDir,
		Limit:   limit,
	}.Normalized()
	q.Offset = (page - 1) * q.Limit

	results, totalCount, err := s.store.GetResultsPage(q)
	if err != nil {
		log.Printf("HandleGetResults: failed to load results for batch %s: %v", batchID, err)
		http.Error(w, "failed to load results", http.StatusInternalServerError)
		return
	}
	if results == nil {
		results = []matcher.MatchResultItem{}
	}

	// Floored at 1 so an empty batch reports one (empty) page rather than zero
	// pages, which would otherwise look like an out-of-range request to a client.
	totalPages := (totalCount + q.Limit - 1) / q.Limit
	if totalPages < 1 {
		totalPages = 1
	}

	response := map[string]interface{}{
		"batch_id":    batchID,
		"total_count": totalCount,
		"total_pages": totalPages,
		"page":        page,
		"limit":       q.Limit,
		"sort_by":     q.SortBy,
		"sort_dir":    q.SortDir,
		"results":     results,
	}

	if includeCounts {
		// Status counts only label UI tabs, so a failure here must not fail the
		// whole results request -- log it and omit the field instead.
		statusCounts, countErr := s.store.CountResultsByStatus(batchID, q.Search)
		if countErr != nil {
			log.Printf("HandleGetResults: failed to count results by status for batch %s: %v", batchID, countErr)
		} else {
			response["status_counts"] = statusCounts
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DefaultJobsPageSize is the page size used when the caller does not ask for one.
const DefaultJobsPageSize = 20

// MaxJobsPageSize bounds what a caller may request, so a single request cannot
// ask the store for an unbounded scan.
const MaxJobsPageSize = 200

// HandleListJobs returns historical match runs, most recent first, with their
// counters and duration.
func (s *Server) HandleListJobs(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 {
		limit = DefaultJobsPageSize
	} else if limit > MaxJobsPageSize {
		limit = MaxJobsPageSize
	}
	if offset < 0 {
		offset = 0
	}

	jobs, err := s.store.ListJobs(limit, offset)
	if err != nil {
		http.Error(w, "Failed to list jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Encode an empty list as [], never null: a nil slice marshals to null, which
	// breaks any caller doing .map() on the result.
	if jobs == nil {
		jobs = []store.JobSummary{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jobs":   jobs,
		"count":  len(jobs),
		"limit":  limit,
		"offset": offset,
	})
}

type ActionPayload struct {
	BatchID        string `json:"batch_id"`
	MatchID        string `json:"match_id"`
	Action         string `json:"action"` // CONFIRM | REJECT | UNLINK
	ReviewComments string `json:"review_comments,omitempty"`
}

func (s *Server) HandleMatchAction(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload ActionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	var targetStatus string
	switch payload.Action {
	case "CONFIRM":
		targetStatus = "CONFIRMED"
	case "REJECT":
		targetStatus = "REJECTED"
	case "UNLINK":
		targetStatus = "REJECTED"
	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	// Fetch current item for audit logging.
	//
	// GetResultByID is an indexed lookup in both stores -- a map read in memory,
	// a composite-key SELECT in PostgreSQL. The previous code pulled the whole
	// batch and scanned it for one id, which at benchmark scale is a 573k-row
	// scan per reviewer click.
	var prevStatus string
	var srcID, destID string
	var confScore float64
	if item, found := s.store.GetResultByID(payload.BatchID, payload.MatchID); found {
		prevStatus = item.MatchStatus
		srcID = item.SourceID
		destID = item.DestinationID
		confScore = item.ConfidenceScore
	}

	err := s.store.UpdateMatchStatus(payload.BatchID, payload.MatchID, targetStatus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get user ID from JWT claims
	claims := ClaimsFrom(r.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	// Record compliance audit log entry
	s.store.RecordAuditLog(store.AuditLogEntry{
		BatchID:         payload.BatchID,
		SourceID:        srcID,
		DestinationID:   destID,
		UserID:          userID,
		Action:          payload.Action,
		PreviousStatus:  prevStatus,
		NewStatus:       targetStatus,
		ConfidenceScore: confScore,
		ReviewComments:  payload.ReviewComments,
		Timestamp:       time.Now(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "success",
		"match_id":   payload.MatchID,
		"new_status": targetStatus,
	})
}

func (s *Server) HandleLLMEvaluate(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload matcher.LLMRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request JSON", http.StatusBadRequest)
		return
	}

	res, err := s.llmResolver.EvaluateEdgeCases(r.Context(), payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("LLM Error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (s *Server) HandleSearchDestinations(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	batchID := r.URL.Query().Get("batch_id")
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))

	_, dests, ok := s.store.GetDataset(batchID)
	if !ok {
		http.Error(w, "Batch not found", http.StatusNotFound)
		return
	}

	var matches []matcher.DestinationRecord
	for _, dst := range dests {
		if query == "" || strings.Contains(strings.ToLower(dst.CustomerNameRaw), query) ||
			strings.Contains(strings.ToLower(dst.CustomerID), query) {
			matches = append(matches, dst)
			if len(matches) >= 30 {
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matches)
}

type ManualLinkPayload struct {
	BatchID       string `json:"batch_id"`
	SourceID      string `json:"source_id"`
	DestinationID string `json:"destination_id"`
}

func (s *Server) HandleManualLink(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload ManualLinkPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	newItem, err := s.store.ManualLink(payload.BatchID, payload.SourceID, payload.DestinationID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newItem)
}

func (s *Server) HandleExportCSV(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	batchID := r.URL.Query().Get("batch_id")
	if batchID == "" {
		http.Error(w, "batch_id parameter required", http.StatusBadRequest)
		return
	}

	results, exists := s.store.GetResults(batchID)
	if !exists {
		http.Error(w, "No results found for batch", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=entity_match_%s.csv", batchID))

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header row
	writer.Write([]string{
		"Match ID", "Batch ID", "Match Status", "Confidence Score",
		"Name Score", "Date Score", "Source Ref ID", "Source Customer Name",
		"Source Tx Date", "Destination Cust ID", "Destination Customer Name",
		"Destination Tx Date", "Match Reasons",
	})

	for _, item := range results {
		srcName, srcRef, srcDate := "", "", ""
		if item.Source != nil {
			srcName = item.Source.CustomerNameRaw
			srcRef = item.Source.ReferenceID
			srcDate = item.Source.TransactionDate.Format("2006-01-02")
		}

		dstName, dstID, dstDate := "", "", ""
		if item.Destination != nil {
			dstName = item.Destination.CustomerNameRaw
			dstID = item.Destination.CustomerID
			dstDate = item.Destination.TransactionDate.Format("2006-01-02")
		}

		writer.Write([]string{
			item.ID,
			item.BatchID,
			item.MatchStatus,
			fmt.Sprintf("%.4f", item.ConfidenceScore),
			fmt.Sprintf("%.4f", item.NameScore),
			fmt.Sprintf("%.4f", item.DateScore),
			srcRef,
			srcName,
			srcDate,
			dstID,
			dstName,
			dstDate,
			strings.Join(item.MatchReasons, "; "),
		})
	}
}

func (s *Server) HandleSeedDataset(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	batchID := "benchmark-batch-001"

	sampleSources := []map[string]interface{}{
		{"reference_id": "REF-TH-001", "customer_name": "บริษัท สยามพารากอน ดีเวลลอปเม้นท์ จำกัด", "transaction_date": "2026-08-15", "transaction_type": "PAYMENT"},
		{"reference_id": "REF-TH-002", "customer_name": "นาย สมชาย เข็มกลัด", "transaction_date": "2026-08-10", "transaction_type": "TRANSFER"},
		{"reference_id": "REF-TH-003", "customer_name": "นางสาว อารียา สุขสันต์", "transaction_date": "2026-08-12", "transaction_type": "PAYMENT"},
		{"reference_id": "REF-EN-004", "customer_name": "Bangkok Bank Public Company Limited", "transaction_date": "2026-08-01", "transaction_type": "BILLING"},
		{"reference_id": "REF-EN-005", "customer_name": "John Michael Smith", "transaction_date": "2026-08-14", "transaction_type": "PAYMENT"},
		{"reference_id": "REF-BI-006", "customer_name": "Charoen Pokphand Group Co., Ltd.", "transaction_date": "2026-08-20", "transaction_type": "DEPOSIT"},
		{"reference_id": "REF-TH-007", "customer_name": "ดร. วีระชัย พงษ์สวัสดิ์", "transaction_date": "2026-08-18", "transaction_type": "PAYMENT"},
		{"reference_id": "REF-EN-008", "customer_name": "Advanced Info Service PLC", "transaction_date": "2026-08-05", "transaction_type": "TRANSFER"},
	}

	sampleDestinations := []map[string]interface{}{
		{"customer_id": "CUST-TH-901", "customer_name": "สยามพารากอน ดีเวลลอปเม้นท์ บจก.", "transaction_date": "2026-08-15"},
		{"customer_id": "CUST-TH-902", "customer_name": "เข็มกลัด สมชาย", "transaction_date": "2026-08-11"},
		{"customer_id": "CUST-TH-903", "customer_name": "คุณ อารียา สุขสันต์", "transaction_date": "2026-08-12"},
		{"customer_id": "CUST-EN-904", "customer_name": "Bangkok Bank PLC", "transaction_date": "2026-08-01"},
		{"customer_id": "CUST-EN-905", "customer_name": "Smith John Michael", "transaction_date": "2026-08-14"},
		{"customer_id": "CUST-BI-906", "customer_name": "บริษัท เจริญโภคภัณฑ์ กรุ๊ป จำกัด (มหาชน)", "transaction_date": "2026-08-20"},
		{"customer_id": "CUST-TH-907", "customer_name": "นาย วีระชัย พงษ์สวัสดิ์", "transaction_date": "2026-08-19"},
		{"customer_id": "CUST-EN-908", "customer_name": "AIS PLC", "transaction_date": "2026-08-05"},
	}

	// Add 50 synthetic records to demonstrate scaling capability
	for i := 1; i <= 50; i++ {
		sampleSources = append(sampleSources, map[string]interface{}{
			"reference_id":     fmt.Sprintf("REF-SYN-%03d", i),
			"customer_name":    fmt.Sprintf("บริษัท เทคโนโลยี อินโนเวชั่น สาขา %d จำกัด", i),
			"transaction_date": time.Now().AddDate(0, 0, -rand.Intn(15)).Format("2006-01-02"),
			"transaction_type": "PAYMENT",
		})
		sampleDestinations = append(sampleDestinations, map[string]interface{}{
			"customer_id":      fmt.Sprintf("CUST-SYN-%03d", i),
			"customer_name":    fmt.Sprintf("เทคโนโลยี อินโนเวชั่น บจก. สาขา %d", i),
			"transaction_date": time.Now().AddDate(0, 0, -rand.Intn(15)).Format("2006-01-02"),
		})
	}

	reqBody, _ := json.Marshal(DatasetPayload{
		BatchID:      batchID,
		Sources:      sampleSources,
		Destinations: sampleDestinations,
	})

	// Run upload handler internally
	rUpload, _ := http.NewRequest("POST", "/api/upload", bytes.NewBuffer(reqBody))
	wUpload := &responseRecorder{header: make(http.Header), body: &bytes.Buffer{}}
	s.HandleUpload(wUpload, rUpload)

	// SetLastBatchID is already called inside HandleUpload via our earlier change
	s.schedulerManager.SetLastBatchID(batchID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "benchmark_dataset_loaded",
		"batch_id": batchID,
		"sources":  len(sampleSources),
		"dests":    len(sampleDestinations),
	})
}

func (s *Server) HandleSeedBigDataset(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	batchID := "big-mock-batch-4000"
	sources, dests, _, _ := mockdata.GenerateBigMockDataset(1000)

	if err := s.store.SaveDataset(batchID, sources, dests); err != nil {
		http.Error(w, "Failed to persist dataset: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Record the batch as the scheduler's current target
	s.schedulerManager.SetLastBatchID(batchID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "big_mock_dataset_loaded",
		"batch_id": batchID,
		"sources":  len(sources),
		"dests":    len(dests),
	})
}

func (s *Server) HandleTestConnector(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cfg matcher.ConnectionConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid connection JSON", http.StatusBadRequest)
		return
	}

	if (cfg.Type == matcher.SourceTypeCSV || cfg.Type == matcher.SourceTypeExcel) &&
		!(cfg.FilePath == "" && len(cfg.ManualData) > 0) {
		resolved, err := resolveConnectorFilePath(cfg.FilePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cfg.FilePath = resolved
	}

	conn, err := matcher.NewDataConnector(cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = conn.Close() }()

	err = conn.TestConnection(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Connection failed: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Successfully connected to %s data source", cfg.Type),
	})
}

func (s *Server) HandleIntrospectSchema(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cfg matcher.ConnectionConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid connection JSON", http.StatusBadRequest)
		return
	}

	// This confinement applies to caller-supplied paths at the API boundary; the upload
	// handler's server-generated temp path (from saveUploadedFileToTemp) never reaches
	// here as a caller-supplied path, so it is unaffected and does not need this guard.
	if (cfg.Type == matcher.SourceTypeCSV || cfg.Type == matcher.SourceTypeExcel) &&
		!(cfg.FilePath == "" && len(cfg.ManualData) > 0) {
		resolved, err := resolveConnectorFilePath(cfg.FilePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cfg.FilePath = resolved
	}

	conn, err := matcher.NewDataConnector(cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = conn.Close() }()

	cols, err := conn.IntrospectSchema(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Schema introspection failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"type":    cfg.Type,
		"columns": cols,
	})
}

// HandleIntrospectUploadedFile returns the column headers of a CSV/Excel file
// submitted as multipart/form-data. The bytes come from the request, so unlike
// HandleIntrospectSchema this endpoint needs no CONNECTOR_FILE_ROOT confinement:
// the caller can only read back a file it already had. The temp file is removed
// before the handler returns; nothing is persisted.
func (s *Server) HandleIntrospectUploadedFile(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes)
	err := r.ParseMultipartForm(MaxUploadBytes)
	if err != nil {
		http.Error(w, "Failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	path, err := saveUploadedFileToTemp(header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer os.Remove(path)

	ext := strings.ToLower(filepath.Ext(header.Filename))
	var srcType matcher.SourceType
	if ext == ".csv" {
		srcType = matcher.SourceTypeCSV
	} else {
		srcType = matcher.SourceTypeExcel
	}

	sheetValue := r.FormValue("sheet")
	cfg := matcher.ConnectionConfig{
		Type:     srcType,
		FilePath: path,
	}
	if srcType == matcher.SourceTypeExcel && sheetValue != "" {
		cfg.ExtraParams = map[string]interface{}{"sheet": sheetValue}
	}

	conn, err := matcher.NewDataConnector(cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = conn.Close() }()

	cols, err := conn.IntrospectSchema(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Schema introspection failed: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"type":     cfg.Type,
		"filename": header.Filename,
		"columns":  cols,
	})
}

// HandleConnectorSettings reads and writes the connector configuration the UI
// shows. Passwords are never stored: store.ConnectorEndpoint has no password
// field, so any password in the request body is discarded at decode time.
func (s *Server) HandleConnectorSettings(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	switch r.Method {
	case "GET":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.store.GetConnectorSettings())
	case "PUT", "POST":
		var cs store.ConnectorSettings
		if err := json.NewDecoder(r.Body).Decode(&cs); err != nil {
			http.Error(w, "Invalid connector settings JSON", http.StatusBadRequest)
			return
		}
		s.store.UpdateConnectorSettings(cs)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.store.GetConnectorSettings())
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) HandleSchedulerConfig(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.schedulerManager.GetConfig())
		return
	}

	if r.Method == "POST" || r.Method == "PUT" {
		var cfg matcher.SchedulerConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		// Validate cron expression if enabled and expression is provided
		if cfg.Enabled && cfg.CronExpression != "" {
			if err := s.schedulerManager.ValidateCronExpression(cfg.CronExpression); err != nil {
				http.Error(w, fmt.Sprintf("Invalid cron expression: %v", err), http.StatusBadRequest)
				return
			}
		}

		s.schedulerManager.UpdateConfig(cfg)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) HandleDictionary(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	dict := matcher.GetGlobalDictionary()
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entries": dict.ListEntries(),
		})
		return
	}

	if r.Method == "POST" {
		var entry matcher.SynonymEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		if entry.Alias != "" && entry.Canonical != "" {
			dict.Set(entry.Alias, entry.Canonical)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"entries": dict.ListEntries(),
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) HandleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	batchID := r.URL.Query().Get("batch_id")
	userID := r.URL.Query().Get("user_id")
	actionFilter := r.URL.Query().Get("action")

	logs := s.store.GetAuditLogs(batchID, userID, actionFilter)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_count": len(logs),
		"logs":        logs,
	})
}

func (s *Server) HandleExportAuditCSV(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	batchID := r.URL.Query().Get("batch_id")
	logs := s.store.GetAuditLogs(batchID, "", "")

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=compliance_audit_%s.csv", batchID))

	writer := csv.NewWriter(w)
	defer writer.Flush()

	writer.Write([]string{"Audit Log ID", "Batch ID", "Timestamp", "Reviewer User ID", "Action", "Previous Status", "New Status", "Confidence Score", "Reviewer Comments"})

	for _, entry := range logs {
		writer.Write([]string{
			entry.ID,
			entry.BatchID,
			entry.Timestamp.Format(time.RFC3339),
			entry.UserID,
			entry.Action,
			entry.PreviousStatus,
			entry.NewStatus,
			fmt.Sprintf("%.4f", entry.ConfidenceScore),
			entry.ReviewComments,
		})
	}
}

type responseRecorder struct {
	header http.Header
	body   *bytes.Buffer
	status int
}

func (r *responseRecorder) Header() http.Header { return r.header }
func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = 200
	}
	return r.body.Write(b)
}
func (r *responseRecorder) WriteHeader(statusCode int) { r.status = statusCode }

// MinCalibrationObservations is the minimum number of labelled observations required
// before HandleCalibrationFit will fit and persist a calibrator. Below this, the
// every-5th-element holdout in splitObservations is too small (or empty) for the
// reported Brier/ECE metrics to mean anything -- an empty holdout scores 0.0, which
// is indistinguishable from a perfect calibrator. Rejecting is safer than persisting
// a model whose quality was never actually measured.
const MinCalibrationObservations = 20

func splitObservations(obs []matcher.LabelledScore) (train, holdout []matcher.LabelledScore) {
	for i, o := range obs {
		if i%5 == 4 {
			holdout = append(holdout, o)
		} else {
			train = append(train, o)
		}
	}
	return train, holdout
}

func brierScore(obs []matcher.LabelledScore, predictFn func(score float64) float64) float64 {
	if len(obs) == 0 {
		return 0
	}
	var sum float64
	for _, o := range obs {
		p := predictFn(o.Score)
		y := 0.0
		if o.IsMatch {
			y = 1.0
		}
		diff := p - y
		sum += diff * diff
	}
	return sum / float64(len(obs))
}

func expectedCalibrationError(obs []matcher.LabelledScore, predictFn func(score float64) float64) float64 {
	const numBins = 10
	if len(obs) == 0 {
		return 0
	}
	type bin struct {
		sumPred float64
		sumY    float64
		count   int
	}
	bins := make([]bin, numBins)
	for _, o := range obs {
		p := predictFn(o.Score)
		if p < 0 {
			p = 0
		}
		if p > 1 {
			p = 1
		}
		idx := int(p * float64(numBins))
		if idx >= numBins {
			idx = numBins - 1
		}
		y := 0.0
		if o.IsMatch {
			y = 1.0
		}
		bins[idx].sumPred += p
		bins[idx].sumY += y
		bins[idx].count++
	}
	var ece float64
	total := float64(len(obs))
	for _, b := range bins {
		if b.count == 0 {
			continue
		}
		meanPred := b.sumPred / float64(b.count)
		meanY := b.sumY / float64(b.count)
		weight := float64(b.count) / total
		diff := meanPred - meanY
		if diff < 0 {
			diff = -diff
		}
		ece += weight * diff
	}
	return ece
}

type CalibrationFitRequest struct {
	BatchID string `json:"batch_id"`
}

type CalibrationFitResponse struct {
	Status           string         `json:"status"`
	ModelID          string         `json:"model_id"`
	BatchID          string         `json:"batch_id"`
	ObservationCount int            `json:"observation_count"`
	PositiveCount    int            `json:"positive_count"`
	NegativeCount    int            `json:"negative_count"`
	TrainCount       int            `json:"train_count"`
	HoldoutCount     int            `json:"holdout_count"`
	BrierScoreBefore float64        `json:"brier_score_before"`
	BrierScoreAfter  float64        `json:"brier_score_after"`
	ECEScoreBefore   float64        `json:"ece_score_before"`
	ECEScoreAfter    float64        `json:"ece_score_after"`
	ByPreviousStatus map[string]int `json:"by_previous_status"`
	Caveat           string         `json:"caveat"`
}

func (s *Server) HandleCalibrationFit(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CalibrationFitRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req) // tolerate empty body -> batch_id ""
	}

	obs, err := s.store.CalibrationObservations(req.BatchID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load calibration observations: %v", err), http.StatusInternalServerError)
		return
	}

	stats, err := s.store.CalibrationObservationStats(req.BatchID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load calibration observation stats: %v", err), http.StatusInternalServerError)
		return
	}

	if len(obs) < MinCalibrationObservations {
		http.Error(w, fmt.Sprintf(
			"insufficient calibration observations: have %d, need at least %d; "+
				"reviewer decisions are the only source of labels, so review more pairs before fitting",
			len(obs), MinCalibrationObservations), http.StatusBadRequest)
		return
	}

	train, holdout := splitObservations(obs)

	cal, err := matcher.FitCalibrator(train)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	identity := func(score float64) float64 { return score }
	brierBefore := brierScore(holdout, identity)
	brierAfter := brierScore(holdout, cal.Calibrate)
	eceBefore := expectedCalibrationError(holdout, identity)
	eceAfter := expectedCalibrationError(holdout, cal.Calibrate)

	modelJSON, err := json.Marshal(cal)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to serialize fitted calibrator: %v", err), http.StatusInternalServerError)
		return
	}

	fittedBy := ""
	if claims := ClaimsFrom(r.Context()); claims != nil {
		fittedBy = claims.UserID
	}

	model := store.CalibrationModel{
		FittedBy:         fittedBy,
		BatchID:          req.BatchID,
		ObservationCount: stats.Total,
		PositiveCount:    stats.Positive,
		BrierScore:       brierAfter,
		ECEScore:         eceAfter,
		ModelJSON:        string(modelJSON),
		Active:           true,
	}

	saved, err := s.store.SaveCalibrationModel(model)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to persist calibration model: %v", err), http.StatusInternalServerError)
		return
	}

	// Install immediately so subsequent match runs pick it up as soon as calibration is
	// enabled in config -- fitting and enabling remain separate operator decisions.
	s.SetCalibrator(cal)

	resp := CalibrationFitResponse{
		Status:           "fitted",
		ModelID:          saved.ID,
		BatchID:          req.BatchID,
		ObservationCount: stats.Total,
		PositiveCount:    stats.Positive,
		NegativeCount:    stats.Negative,
		TrainCount:       len(train),
		HoldoutCount:     len(holdout),
		BrierScoreBefore: brierBefore,
		BrierScoreAfter:  brierAfter,
		ECEScoreBefore:   eceBefore,
		ECEScoreAfter:    eceAfter,
		ByPreviousStatus: stats.ByPreviousStatus,
		Caveat: "Training data is drawn almost entirely from the human review queue " +
			"(auto-matched pairs are rarely reviewed and so rarely labelled). This calibrator " +
			"is well-calibrated for the review-band score range and is extrapolating outside it.",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type CalibrationStatusResponse struct {
	CalibrationEnabled bool                    `json:"calibration_enabled"`
	HasActiveModel     bool                    `json:"has_active_model"`
	ActiveModel        *store.CalibrationModel `json:"active_model,omitempty"`
	ObservationCount   int                     `json:"observation_count"`
	PositiveCount      int                     `json:"positive_count"`
	NegativeCount      int                     `json:"negative_count"`
	ByPreviousStatus   map[string]int          `json:"by_previous_status"`
	Caveat             string                  `json:"caveat"`
}

func (s *Server) HandleCalibrationStatus(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg := s.store.GetConfig()

	model, hasActive, err := s.store.GetActiveCalibrationModel()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load active calibration model: %v", err), http.StatusInternalServerError)
		return
	}

	stats, err := s.store.CalibrationObservationStats("")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load calibration observation stats: %v", err), http.StatusInternalServerError)
		return
	}

	resp := CalibrationStatusResponse{
		CalibrationEnabled: cfg.CalibrationEnabled,
		HasActiveModel:     hasActive,
		ObservationCount:   stats.Total,
		PositiveCount:      stats.Positive,
		NegativeCount:      stats.Negative,
		ByPreviousStatus:   stats.ByPreviousStatus,
		Caveat: "Training data is drawn almost entirely from the human review queue " +
			"(auto-matched pairs are rarely reviewed and so rarely labelled). Any active " +
			"calibrator is well-calibrated for the review-band score range and is " +
			"extrapolating outside it.",
	}
	if hasActive {
		resp.ActiveModel = &model
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
