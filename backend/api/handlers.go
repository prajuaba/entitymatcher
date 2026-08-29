package api

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"entitymatcher/matcher"
	"entitymatcher/store"
	"entitymatcher/testdata"
)

type Server struct {
	store       *store.Store
	llmResolver *matcher.LLMResolver
}

func NewServer(st *store.Store) *Server {
	return &Server{
		store:       st,
		llmResolver: matcher.NewLLMResolver(),
	}
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
		var cfg matcher.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "Invalid request JSON", http.StatusBadRequest)
			return
		}
		s.store.UpdateConfig(cfg)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
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

	sources := make([]matcher.SourceRecord, 0, len(payload.Sources))
	for i, rawMap := range payload.Sources {
		refID := matcher.ExtractFieldValue(rawMap, cfg.ColumnMapping.RefIDSrc)
		if refID == "" {
			refID = matcher.ExtractFieldValue(rawMap, "reference_id")
		}
		if refID == "" {
			refID = fmt.Sprintf("SRC-%04d", i+1)
		}

		nameStr := matcher.ExtractCompositeName(rawMap, cfg.ColumnMapping.NameFieldsSrc)

		dateStr := matcher.ExtractFieldValue(rawMap, cfg.ColumnMapping.DateFieldSrc)
		txDate, _ := time.Parse("2006-01-02", dateStr)
		if txDate.IsZero() {
			txDate = time.Now()
		}

		txType := matcher.ExtractFieldValue(rawMap, "transaction_type")

		sources = append(sources, matcher.SourceRecord{
			ID:              fmt.Sprintf("src-%d", i+1),
			BatchID:         payload.BatchID,
			ReferenceID:     refID,
			CustomerNameRaw: nameStr,
			NormalizedName:  matcher.Normalize(nameStr),
			TransactionDate: txDate,
			TransactionType: txType,
			Attributes:      rawMap,
		})
	}

	destinations := make([]matcher.DestinationRecord, 0, len(payload.Destinations))
	for i, rawMap := range payload.Destinations {
		custID := matcher.ExtractFieldValue(rawMap, cfg.ColumnMapping.RefIDDest)
		if custID == "" {
			custID = matcher.ExtractFieldValue(rawMap, "customer_id")
		}
		if custID == "" {
			custID = fmt.Sprintf("DEST-%04d", i+1)
		}

		nameStr := matcher.ExtractCompositeName(rawMap, cfg.ColumnMapping.NameFieldsDest)

		dateStr := matcher.ExtractFieldValue(rawMap, cfg.ColumnMapping.DateFieldDest)
		txDate, _ := time.Parse("2006-01-02", dateStr)
		if txDate.IsZero() {
			txDate = time.Now()
		}

		destinations = append(destinations, matcher.DestinationRecord{
			ID:              fmt.Sprintf("dest-%d", i+1),
			BatchID:         payload.BatchID,
			CustomerID:      custID,
			CustomerNameRaw: nameStr,
			NormalizedName:  matcher.Normalize(nameStr),
			TransactionDate: txDate,
			Attributes:      rawMap,
		})
	}

	s.store.SaveDataset(payload.BatchID, sources, destinations)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":            "success",
		"batch_id":          payload.BatchID,
		"source_count":      len(sources),
		"destination_count": len(destinations),
		"column_mapping":    cfg.ColumnMapping,
	})
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

	sources, dests, ok := s.store.GetDataset(batchID)
	if !ok {
		http.Error(w, "Dataset for batch_id not found", http.StatusNotFound)
		return
	}

	cfg := s.store.GetConfig()
	engine := matcher.NewMatchEngine(cfg)

	go func() {
		results, progress := engine.ExecuteJob(
			context.Background(),
			batchID,
			sources,
			dests,
			func(p matcher.BatchProgress) {
				s.store.UpdateProgress(p)
			},
		)
		s.store.SaveResults(batchID, results)
		s.store.UpdateProgress(progress)
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

	statusFilter := r.URL.Query().Get("status")
	searchQuery := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}

	results, exists := s.store.GetResults(batchID)
	if !exists {
		results = []matcher.MatchResultItem{}
	}

	// Filter
	var filtered []matcher.MatchResultItem
	for _, item := range results {
		if statusFilter != "" && statusFilter != "ALL" && item.MatchStatus != statusFilter {
			continue
		}
		if searchQuery != "" {
			srcMatch := strings.Contains(strings.ToLower(item.Source.CustomerNameRaw), searchQuery) ||
				strings.Contains(strings.ToLower(item.Source.ReferenceID), searchQuery)
			destMatch := strings.Contains(strings.ToLower(item.Destination.CustomerNameRaw), searchQuery) ||
				strings.Contains(strings.ToLower(item.Destination.CustomerID), searchQuery)
			if !srcMatch && !destMatch {
				continue
			}
		}
		filtered = append(filtered, item)
	}

	totalItems := len(filtered)
	start := (page - 1) * limit
	end := start + limit

	if start > totalItems {
		start = totalItems
	}
	if end > totalItems {
		end = totalItems
	}

	paginated := filtered[start:end]

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"batch_id":    batchID,
		"total_count": totalItems,
		"page":        page,
		"limit":       limit,
		"results":     paginated,
	})
}

type ActionPayload struct {
	BatchID        string `json:"batch_id"`
	MatchID        string `json:"match_id"`
	Action         string `json:"action"` // CONFIRM | REJECT | UNLINK
	UserID         string `json:"user_id,omitempty"`
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

	// Fetch current item for audit logging
	results, _ := s.store.GetResults(payload.BatchID)
	var prevStatus string
	var srcID, destID string
	var confScore float64
	for _, item := range results {
		if item.ID == payload.MatchID {
			prevStatus = item.MatchStatus
			srcID = item.SourceID
			destID = item.DestinationID
			confScore = item.ConfidenceScore
			break
		}
	}

	err := s.store.UpdateMatchStatus(payload.BatchID, payload.MatchID, targetStatus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID := payload.UserID
	if userID == "" {
		userID = "reviewer_op"
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
		"status":       "success",
		"match_id":     payload.MatchID,
		"new_status":   targetStatus,
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
	sources, dests, _, _ := testdata.GenerateBigMockDataset(1000)

	s.store.SaveDataset(batchID, sources, dests)

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

	conn, err := matcher.NewDataConnector(cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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

	conn, err := matcher.NewDataConnector(cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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

func (s *Server) HandleSchedulerConfig(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	schedulerMgr := matcher.NewSchedulerManager()
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(schedulerMgr.GetConfig())
		return
	}

	if r.Method == "POST" || r.Method == "PUT" {
		var cfg matcher.SchedulerConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		schedulerMgr.UpdateConfig(cfg)
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

