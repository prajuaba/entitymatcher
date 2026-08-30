package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"entitymatcher/matcher"
)

type BatchSummary struct {
	BatchID          string    `json:"batch_id"`
	SourceCount      int       `json:"source_count"`
	DestinationCount int       `json:"destination_count"`
	ResultCount      int       `json:"result_count"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

type Store struct {
	mu           sync.RWMutex
	config       matcher.Config
	sources      map[string][]matcher.SourceRecord      // batch_id -> sources
	destinations map[string][]matcher.DestinationRecord // batch_id -> dests
	results      map[string][]matcher.MatchResultItem   // batch_id -> results
	resultIndex  map[string]map[string]int              // batch_id -> matchID -> slice position
	progresses   map[string]matcher.BatchProgress       // batch_id -> progress
	sseClients   map[string][]chan matcher.BatchProgress
	auditStore   *AuditStore
}

func NewStore() *Store {
	return &Store{
		config:       matcher.DefaultConfig(),
		sources:      make(map[string][]matcher.SourceRecord),
		destinations: make(map[string][]matcher.DestinationRecord),
		results:      make(map[string][]matcher.MatchResultItem),
		resultIndex:  make(map[string]map[string]int),
		progresses:   make(map[string]matcher.BatchProgress),
		sseClients:   make(map[string][]chan matcher.BatchProgress),
		auditStore:   NewAuditStore(),
	}
}

func (s *Store) GetConfig() matcher.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *Store) UpdateConfig(cfg matcher.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}

func (s *Store) SaveDataset(batchID string, sources []matcher.SourceRecord, dests []matcher.DestinationRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sources[batchID] = sources
	s.destinations[batchID] = dests
}

func (s *Store) GetDataset(batchID string) ([]matcher.SourceRecord, []matcher.DestinationRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src, ok1 := s.sources[batchID]
	dst, ok2 := s.destinations[batchID]
	return src, dst, ok1 && ok2
}

func (s *Store) SaveResultsCtx(ctx context.Context, batchID string, results []matcher.MatchResultItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[batchID] = results

	// Rebuild resultIndex from scratch. Reusing the previous map would leave stale
	// matchID -> position entries behind when a batch is re-run with a different
	// result set, and a stale ID landing on a still-valid position would mutate the
	// wrong record on the next status update.
	index := make(map[string]int, len(results))
	for i, item := range results {
		index[item.ID] = i
	}
	s.resultIndex[batchID] = index
	return nil
}

func (s *Store) GetResults(batchID string) ([]matcher.MatchResultItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, ok := s.results[batchID]
	return res, ok
}

func (s *Store) GetResultByID(batchID, matchID string) (matcher.MatchResultItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	index, exists := s.resultIndex[batchID]
	if !exists {
		return matcher.MatchResultItem{}, false
	}

	pos, exists := index[matchID]
	if !exists {
		return matcher.MatchResultItem{}, false
	}

	items := s.results[batchID]
	if pos >= len(items) {
		return matcher.MatchResultItem{}, false
	}

	return items[pos], true
}

func (s *Store) UpdateMatchStatus(batchID, matchID, newStatus string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, exists := s.resultIndex[batchID]
	if !exists {
		return fmt.Errorf("batch not found")
	}

	pos, exists := index[matchID]
	if !exists {
		return fmt.Errorf("match record not found")
	}

	items := s.results[batchID]
	if pos >= len(items) {
		return fmt.Errorf("match record not found")
	}

	items[pos].MatchStatus = newStatus
	return nil
}

func (s *Store) UpdateProgress(p matcher.BatchProgress) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.progresses[p.BatchID] = p

	// Notify SSE listeners
	if clients, exists := s.sseClients[p.BatchID]; exists {
		for _, ch := range clients {
			select {
			case ch <- p:
			default:
			}
		}
	}
}

func (s *Store) GetProgress(batchID string) (matcher.BatchProgress, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.progresses[batchID]
	return p, ok
}

func (s *Store) RegisterSSEClient(batchID string) chan matcher.BatchProgress {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan matcher.BatchProgress, 10)
	s.sseClients[batchID] = append(s.sseClients[batchID], ch)
	return ch
}

// UnregisterSSEClient removes the client channel from the list.
// Invariant: UpdateProgress sends under s.mu before checking if client exists,
// and UnregisterSSEClient removes client under s.mu before closing,
// so there's no send-on-closed-channel panic.
func (s *Store) UnregisterSSEClient(batchID string, ch chan matcher.BatchProgress) {
	s.mu.Lock()
	defer s.mu.Unlock()

	clients, exists := s.sseClients[batchID]
	if !exists {
		return
	}

	for i, c := range clients {
		if c == ch {
			s.sseClients[batchID] = append(clients[:i], clients[i+1:]...)
			close(ch)
			break
		}
	}
}

func (s *Store) ManualLink(batchID, sourceID, destinationID string) (*matcher.MatchResultItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sources := s.sources[batchID]
	dests := s.destinations[batchID]

	var src *matcher.SourceRecord
	for i := range sources {
		if sources[i].ID == sourceID {
			src = &sources[i]
			break
		}
	}

	var dst *matcher.DestinationRecord
	for i := range dests {
		if dests[i].ID == destinationID {
			dst = &dests[i]
			break
		}
	}

	if src == nil || dst == nil {
		return nil, fmt.Errorf("source or destination record not found")
	}

	scoreRes := matcher.CalculateCompositeScore(
		src.NormalizedName,
		dst.NormalizedName,
		src.TransactionDate,
		dst.TransactionDate,
		s.config.Weights,
		s.config.Algorithms,
		s.config.DateToleranceDays,
	)

	newItem := matcher.MatchResultItem{
		ID:              fmt.Sprintf("%s-%s-%s-manual", batchID, sourceID, destinationID),
		BatchID:         batchID,
		SourceID:        sourceID,
		Source:          src,
		DestinationID:   destinationID,
		Destination:     dst,
		ConfidenceScore: scoreRes.TotalScore,
		NameScore:       scoreRes.NameScore,
		DateScore:       scoreRes.DateScore,
		JWScore:         scoreRes.JWScore,
		LevScore:        scoreRes.LevScore,
		TokenScore:      scoreRes.TokenScore,
		TrigramScore:    scoreRes.TrigramScore,
		MatchStatus:     "CONFIRMED",
		MatchReasons:    append(scoreRes.MatchReasons, "Manually linked by user"),
		CreatedAt:       time.Now(),
	}

	s.results[batchID] = append(s.results[batchID], newItem)

	// Maintain resultIndex
	if _, exists := s.resultIndex[batchID]; !exists {
		s.resultIndex[batchID] = make(map[string]int)
	}
	index := s.resultIndex[batchID]
	index[newItem.ID] = len(s.results[batchID]) - 1

	return &newItem, nil
}

func (s *Store) RecordAuditLog(entry AuditLogEntry) AuditLogEntry {
	return s.auditStore.RecordAuditLog(entry)
}

func (s *Store) GetAuditLogs(batchID, userID, actionFilter string) []AuditLogEntry {
	return s.auditStore.GetAuditLogs(batchID, userID, actionFilter)
}

// DeleteBatch removes all entries for a given batch ID.
func (s *Store) DeleteBatch(batchID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sources, batchID)
	delete(s.destinations, batchID)
	delete(s.results, batchID)
	delete(s.resultIndex, batchID)
	delete(s.progresses, batchID)

	// Close all SSE clients for this batch
	if clients, exists := s.sseClients[batchID]; exists {
		for _, ch := range clients {
			close(ch)
		}
		delete(s.sseClients, batchID)
	}
}

// ListBatches returns a list of summaries for all known batches.
func (s *Store) ListBatches() []BatchSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var summaries []BatchSummary

	for batchID := range s.sources {
		srcCount := len(s.sources[batchID])
		dstCount := len(s.destinations[batchID])
		resCount := len(s.results[batchID])

		status := "UNKNOWN"
		createdAt := time.Time{}
		if progress, exists := s.progresses[batchID]; exists {
			status = progress.Status
			createdAt = progress.StartedAt
		}

		summaries = append(summaries, BatchSummary{
			BatchID:          batchID,
			SourceCount:      srcCount,
			DestinationCount: dstCount,
			ResultCount:      resCount,
			Status:           status,
			CreatedAt:        createdAt,
		})
	}

	return summaries
}

// GetResultsPage returns a paginated, filtered set of match results for a batch.
func (s *Store) GetResultsPage(batchID, status, search string, limit, offset int) ([]matcher.MatchResultItem, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results, ok := s.results[batchID]
	if !ok {
		return nil, 0, fmt.Errorf("batch not found")
	}

	// Filter by status and search
	var filtered []matcher.MatchResultItem
	for _, item := range results {
		if status != "" && status != "ALL" && item.MatchStatus != status {
			continue
		}
		if search != "" {
			searchLower := strings.ToLower(search)
			srcMatch := (item.Source != nil && (strings.Contains(strings.ToLower(item.Source.CustomerNameRaw), searchLower) ||
				strings.Contains(strings.ToLower(item.Source.ReferenceID), searchLower))) ||
				(item.Source == nil)
			dstMatch := (item.Destination != nil && (strings.Contains(strings.ToLower(item.Destination.CustomerNameRaw), searchLower) ||
				strings.Contains(strings.ToLower(item.Destination.CustomerID), searchLower))) ||
				(item.Destination == nil)
			if !srcMatch && !dstMatch {
				continue
			}
		}
		filtered = append(filtered, item)
	}

	totalCount := len(filtered)

	// Apply pagination
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	start := offset
	end := offset + limit
	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}

	return filtered[start:end], totalCount, nil
}

// ListJobs returns job summaries with pagination, ordered by started_at descending.
func (s *Store) ListJobs(limit, offset int) ([]JobSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var summaries []JobSummary

	// Collect all jobs from progresses
	for batchID, progress := range s.progresses {
		startedAtStr := ""
		if !progress.StartedAt.IsZero() {
			startedAtStr = progress.StartedAt.Format(time.RFC3339)
		}
		completedAtStr := ""
		if !progress.CompletedAt.IsZero() {
			completedAtStr = progress.CompletedAt.Format(time.RFC3339)
		}

		summaries = append(summaries, JobSummary{
			BatchID:             batchID,
			Status:              progress.Status,
			TotalSources:        int(progress.TotalSources),
			TotalDestinations:   int(len(s.destinations[batchID])),
			AutoMatched:         int(progress.AutoMatched),
			ReviewNeeded:        int(progress.ReviewNeeded),
			NoMatchCount:        int(progress.NoMatchCount),
			TotalCandidatePairs: int(progress.TotalMatches),
			ElapsedMs:           progress.ElapsedMs,
			StartedAt:           startedAtStr,
			CompletedAt:         completedAtStr,
		})
	}

	// Sort by started_at descending
	sort.Slice(summaries, func(i, j int) bool {
		iTime, _ := time.Parse(time.RFC3339, summaries[i].StartedAt)
		jTime, _ := time.Parse(time.RFC3339, summaries[j].StartedAt)
		return iTime.After(jTime)
	})

	// Apply pagination
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	start := offset
	end := offset + limit
	if start > len(summaries) {
		start = len(summaries)
	}
	if end > len(summaries) {
		end = len(summaries)
	}

	return summaries[start:end], nil
}

// Compile-time assertion that Store implements Repository
var _ Repository = (*Store)(nil)
