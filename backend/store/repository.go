package store

import (
	"context"
	"strings"

	"entitymatcher/matcher"
)

// ConnectorEndpoint is one side (source or destination) of the connector
// configuration the UI shows. There is deliberately no password field: the
// struct is the persistence boundary, so a password sent by a client is
// dropped by encoding/json rather than relying on a caller to strip it.
type ConnectorEndpoint struct {
	Type         string   `json:"type"`
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	Database     string   `json:"database"`
	Username     string   `json:"username"`
	TableOrQuery string   `json:"table_or_query"`
	FilePath     string   `json:"file_path"`
	Columns      []string `json:"columns"`
}

// ConnectorSettings is the single stored row of connector configuration.
type ConnectorSettings struct {
	Source      ConnectorEndpoint `json:"source"`
	Destination ConnectorEndpoint `json:"destination"`
}

// JobSummary represents a summary of a matching job
type JobSummary struct {
	BatchID             string `json:"batch_id"`
	Status              string `json:"status"`
	TotalSources        int    `json:"total_sources"`
	TotalDestinations   int    `json:"total_destinations"`
	AutoMatched         int    `json:"auto_matched"`
	ReviewNeeded        int    `json:"review_needed"`
	NoMatchCount        int    `json:"no_match_count"`
	TotalCandidatePairs int    `json:"total_candidate_pairs"`
	ElapsedMs           int64  `json:"elapsed_ms"`
	StartedAt           string `json:"started_at"`   // RFC3339
	CompletedAt         string `json:"completed_at"` // RFC3339
}

// Sortable result fields. These are the only values a caller may sort by; the
// SQL layer maps them to column expressions rather than interpolating caller
// input into an ORDER BY, which cannot be parameterized.
const (
	SortByCreatedAt   = "created_at"
	SortByConfidence  = "confidence_score"
	SortByNameScore   = "name_score"
	SortByDateScore   = "date_score"
	SortByStatus      = "match_status"
	SortBySourceName  = "source_name"
	SortByReferenceID = "reference_id"
)

// DefaultResultsPageSize / MaxResultsPageSize bound what one request may pull,
// so a caller cannot ask the store for an unbounded scan.
const (
	DefaultResultsPageSize = 20
	MaxResultsPageSize     = 200
)

// MaxResultsPage bounds the page NUMBER a caller may request (as opposed to
// MaxResultsPageSize, which bounds the page's row count). A UI paging through
// 20-200 rows at a time will never legitimately reach page 1,000,000. Clamping
// the page number to this constant belongs in the HTTP handler rather than in
// Normalized() above, since ResultsQuery has no concept of a "page number" --
// only a Limit/Offset already-computed by the caller -- and clamping there
// keeps an absurd page value from overflowing the handler's
// (page-1)*limit offset multiplication into a negative number.
const MaxResultsPage = 1_000_000

// ResultsQuery describes one page of match results. Passing a struct rather
// than a widening parameter list keeps the signature stable as filters grow.
type ResultsQuery struct {
	BatchID string
	Status  string // "" or "ALL" means every status
	Search  string
	SortBy  string
	SortDir string // "asc" or "desc"
	Limit   int
	Offset  int
}

// Normalized returns a copy of q with defaults applied and invalid sort/paging
// values silently corrected, so a caller (e.g. an HTTP handler) never needs to
// validate a ResultsQuery itself before handing it to the store.
func (q ResultsQuery) Normalized() ResultsQuery {
	n := q
	n.Search = strings.TrimSpace(n.Search)

	switch n.SortBy {
	case SortByCreatedAt, SortByConfidence, SortByNameScore, SortByDateScore,
		SortByStatus, SortBySourceName, SortByReferenceID:
		// already a valid sort field
	default:
		n.SortBy = SortByCreatedAt
	}

	n.SortDir = strings.ToLower(n.SortDir)
	if n.SortDir != "desc" {
		n.SortDir = "asc"
	}

	if n.Limit <= 0 {
		n.Limit = DefaultResultsPageSize
	} else if n.Limit > MaxResultsPageSize {
		n.Limit = MaxResultsPageSize
	}

	if n.Offset < 0 {
		n.Offset = 0
	}

	return n
}

// Repository defines the storage interface for the entity matcher.
// Implementations must be thread-safe for concurrent access.
type Repository interface {
	// Config management
	GetConfig() matcher.Config
	UpdateConfig(cfg matcher.Config)

	// Connector settings (never includes passwords)
	GetConnectorSettings() ConnectorSettings
	UpdateConnectorSettings(cs ConnectorSettings)

	// Dataset management
	// Callers MUST check and surface the returned error; silent write failure is
	// indistinguishable from success to the caller.
	SaveDataset(batchID string, sources []matcher.SourceRecord, dests []matcher.DestinationRecord) error
	GetDataset(batchID string) ([]matcher.SourceRecord, []matcher.DestinationRecord, bool)

	// Results management
	SaveResultsCtx(ctx context.Context, batchID string, results []matcher.MatchResultItem) error
	GetResults(batchID string) ([]matcher.MatchResultItem, bool)
	GetResultByID(batchID, matchID string) (matcher.MatchResultItem, bool)
	UpdateMatchStatus(batchID, matchID, newStatus string) error

	// Pagination support for results
	// GetResultsPage returns one page of results plus the total number of rows
	// matching the query's filters (ignoring Limit/Offset). A batch that does
	// not exist is an empty page, not an error -- only a genuine store failure
	// returns one.
	GetResultsPage(q ResultsQuery) ([]matcher.MatchResultItem, int, error)

	// CountResultsByStatus returns row counts per match_status for a batch,
	// honouring the same search filter but ignoring any status filter, so a UI
	// can label its status tabs without issuing one request per tab.
	CountResultsByStatus(batchID, search string) (map[string]int, error)

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

	// Calibration: connects reviewer decisions to score calibration

	// CalibrationObservations extracts a deduplicated, labelled training set from
	// reviewer decisions recorded in the audit log, suitable for matcher.FitCalibrator.
	// Pass batchID == "" to include observations from all batches.
	//
	// Mapping: Action=="CONFIRM" -> IsMatch=true; Action=="REJECT" -> IsMatch=false;
	// Action=="OVERRIDE" is ambiguous on its own and is resolved from NewStatus
	// (CONFIRMED -> true, REJECTED -> false; any other NewStatus is skipped as
	// unusable). Any other Action value is skipped.
	//
	// Deduplication: entries are grouped by (batch_id, source_id, destination_id) and
	// only the LATEST decision (by Timestamp) for each pair contributes an observation,
	// so a pair reviewed multiple times counts once and a later reversal wins.
	//
	// SELECTION BIAS WARNING: auto-matched pairs are typically never reviewed and so
	// generate no audit entry and no label. This training set is therefore drawn almost
	// entirely from the human review queue — a biased sample that over-represents
	// ambiguous mid-range scores and under-represents confident correct matches. A
	// calibrator fitted on it will be well-calibrated for the review band and is
	// extrapolating everywhere else. This method does not correct that bias, it only
	// supplies the raw labelled data; see CalibrationObservationStats for a way to
	// surface the skew, and GET /api/calibration/status for where operators see it.
	CalibrationObservations(batchID string) ([]matcher.LabelledScore, error)

	// CalibrationObservationStats reports the composition of the CalibrationObservations
	// training set (after the same mapping and dedup rules) broken down by the
	// PreviousStatus of the winning audit entries, so an operator can see the
	// review-queue selection-bias skew directly instead of just being told about it.
	// Pass batchID == "" to include observations from all batches.
	CalibrationObservationStats(batchID string) (CalibrationObservationStats, error)

	// SaveCalibrationModel persists a newly-fitted calibrator as a new, append-only row.
	// If model.Active is true, any previously active model is deactivated first so at
	// most one model is ever active at a time. Returns the saved model with ID and
	// CreatedAt populated (generated by the store if the caller left them zero-valued).
	SaveCalibrationModel(model CalibrationModel) (CalibrationModel, error)

	// GetActiveCalibrationModel returns the currently active calibration model, if any.
	// The bool return is false (with a zero-value CalibrationModel and nil error) when
	// no model has ever been activated.
	GetActiveCalibrationModel() (CalibrationModel, bool, error)

	// ListCalibrationModels returns calibration model fit history, most recent first.
	ListCalibrationModels(limit, offset int) ([]CalibrationModel, error)
}
