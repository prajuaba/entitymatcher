package store

import (
	"context"
	"entitymatcher/matcher"
)

// JobSummary represents a summary of a matching job
type JobSummary struct {
	BatchID              string `json:"batch_id"`
	Status               string `json:"status"`
	TotalSources         int    `json:"total_sources"`
	TotalDestinations    int    `json:"total_destinations"`
	AutoMatched          int    `json:"auto_matched"`
	ReviewNeeded         int    `json:"review_needed"`
	NoMatchCount         int    `json:"no_match_count"`
	TotalCandidatePairs  int    `json:"total_candidate_pairs"`
	ElapsedMs            int64  `json:"elapsed_ms"`
	StartedAt            string `json:"started_at"` // RFC3339
	CompletedAt          string `json:"completed_at"` // RFC3339
}

// Repository defines the storage interface for the entity matcher.
// Implementations must be thread-safe for concurrent access.
type Repository interface {
	// Config management
	GetConfig() matcher.Config
	UpdateConfig(cfg matcher.Config)

	// Dataset management
	SaveDataset(batchID string, sources []matcher.SourceRecord, dests []matcher.DestinationRecord)
	GetDataset(batchID string) ([]matcher.SourceRecord, []matcher.DestinationRecord, bool)

	// Results management
	SaveResults(batchID string, results []matcher.MatchResultItem)
	SaveResultsCtx(ctx context.Context, batchID string, results []matcher.MatchResultItem) error
	GetResults(batchID string) ([]matcher.MatchResultItem, bool)
	GetResultByID(batchID, matchID string) (matcher.MatchResultItem, bool)
	UpdateMatchStatus(batchID, matchID, newStatus string) error

	// Pagination support for results
	GetResultsPage(batchID, status, search string, limit, offset int) ([]matcher.MatchResultItem, int, error)

	// Progress tracking
	UpdateProgress(p matcher.BatchProgress)
	GetProgress(batchID string) (matcher.BatchProgress, bool)

	// Server-Sent Events (SSE) client registration for real-time progress updates
	RegisterSSEClient(batchID string) chan matcher.BatchProgress
	UnregisterSSEClient(batchID string, ch chan matcher.BatchProgress)

	// Match operations
	ManualLink(batchID, sourceID, destinationID string) (*matcher.MatchResultItem, error)

	// Audit logging (append-only compliance records)
	RecordAuditLog(entry AuditLogEntry) AuditLogEntry
	GetAuditLogs(batchID, userID, actionFilter string) []AuditLogEntry

	// Batch management
	DeleteBatch(batchID string)
	ListBatches() []BatchSummary
	ListJobs(limit, offset int) ([]JobSummary, error)
}
