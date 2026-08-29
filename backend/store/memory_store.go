package store

import (
	"fmt"
	"sync"
	"time"

	"entitymatcher/matcher"
)

type Store struct {
	mu           sync.RWMutex
	config       matcher.Config
	sources      map[string][]matcher.SourceRecord      // batch_id -> sources
	destinations map[string][]matcher.DestinationRecord // batch_id -> dests
	results      map[string][]matcher.MatchResultItem   // batch_id -> results
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

func (s *Store) SaveResults(batchID string, results []matcher.MatchResultItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[batchID] = results
}

func (s *Store) GetResults(batchID string) ([]matcher.MatchResultItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, ok := s.results[batchID]
	return res, ok
}

func (s *Store) UpdateMatchStatus(batchID, matchID, newStatus string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, exists := s.results[batchID]
	if !exists {
		return fmt.Errorf("batch not found")
	}

	found := false
	for i := range items {
		if items[i].ID == matchID {
			items[i].MatchStatus = newStatus
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("match record not found")
	}
	s.results[batchID] = items
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
	return &newItem, nil
}

func (s *Store) RecordAuditLog(entry AuditLogEntry) AuditLogEntry {
	return s.auditStore.RecordAuditLog(entry)
}

func (s *Store) GetAuditLogs(batchID, userID, actionFilter string) []AuditLogEntry {
	return s.auditStore.GetAuditLogs(batchID, userID, actionFilter)
}
