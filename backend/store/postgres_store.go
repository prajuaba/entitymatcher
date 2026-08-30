package store

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"entitymatcher/matcher"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// PostgresStore implements Repository using PostgreSQL as the persistent backend.
type PostgresStore struct {
	pool       *pgxpool.Pool
	sseClients map[string][]chan matcher.BatchProgress
	sseMu      sync.Mutex
}

// NewPostgresStore creates a new PostgreSQL-backed store.
// It opens a connection pool, verifies connectivity, and applies the schema.
func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	// Parse and create pool configuration
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	// Apply schema (idempotent)
	if err := applySchema(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &PostgresStore{
		pool:       pool,
		sseClients: make(map[string][]chan matcher.BatchProgress),
	}, nil
}

// applySchema applies the embedded schema.sql to the database.
func applySchema(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, schemaSQL)
	return err
}

// GetConfig retrieves the current matching configuration.
func (s *PostgresStore) GetConfig() matcher.Config {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var configJSON []byte
	err := s.pool.QueryRow(ctx,
		"SELECT config FROM config WHERE id = 1").
		Scan(&configJSON)

	if err != nil {
		// Return default if not found
		return matcher.DefaultConfig()
	}

	var cfg matcher.Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return matcher.DefaultConfig()
	}
	return cfg
}

// UpdateConfig updates the matching configuration via upsert.
func (s *PostgresStore) UpdateConfig(cfg matcher.Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	configJSON, _ := json.Marshal(cfg)
	_, _ = s.pool.Exec(ctx,
		`INSERT INTO config (id, config, updated_at) VALUES (1, $1, CURRENT_TIMESTAMP)
		 ON CONFLICT (id) DO UPDATE SET config = EXCLUDED.config, updated_at = CURRENT_TIMESTAMP`,
		configJSON)
}

// SaveDataset stores source and destination records for a batch.
func (s *PostgresStore) SaveDataset(batchID string, sources []matcher.SourceRecord, dests []matcher.DestinationRecord) {
	// For now, we don't persist source/destination records to keep schema minimal.
}

// GetDataset retrieves source and destination records for a batch.
func (s *PostgresStore) GetDataset(batchID string) ([]matcher.SourceRecord, []matcher.DestinationRecord, bool) {
	return nil, nil, false
}

// SaveResultsCtx stores match results for a batch with proper transaction handling.
// Uses CopyFrom for bulk inserts. DEFECT 1: wraps DELETE + INSERT in ONE transaction.
// DEFECT 2: returns errors instead of discarding. DEFECT 4: uses CopyFrom + validates row count.
func (s *PostgresStore) SaveResultsCtx(ctx context.Context, batchID string, results []matcher.MatchResultItem) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// DEFECT 1: Wrap DELETE + INSERT in explicit transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Ensure batch exists in match_jobs (create minimal entry if needed for FK constraint)
	_, err = tx.Exec(ctx,
		`INSERT INTO match_jobs (batch_id, status, started_at)
		 VALUES ($1, $2, CURRENT_TIMESTAMP)
		 ON CONFLICT (batch_id) DO NOTHING`,
		batchID, "IDLE")
	if err != nil {
		return fmt.Errorf("create batch: %w", err)
	}

	// Delete existing results for this batch (within same transaction)
	_, err = tx.Exec(ctx, "DELETE FROM match_results WHERE batch_id = $1", batchID)
	if err != nil {
		return fmt.Errorf("delete existing results: %w", err)
	}

	if len(results) == 0 {
		return tx.Commit(ctx)
	}

	// DEFECT 4: Use CopyFrom for bulk inserts and verify row count
	rows := make([][]interface{}, len(results))
	for i, result := range results {
		matchReasons, _ := json.Marshal(result.MatchReasons)
		srcSnapshot, _ := json.Marshal(result.Source)
		dstSnapshot, _ := json.Marshal(result.Destination)

		rows[i] = []interface{}{
			result.BatchID,
			result.ID,
			result.SourceID,
			result.DestinationID,
			result.ConfidenceScore,
			result.NameScore,
			result.DateScore,
			result.MatchStatus,
			result.Rank,
			result.ScoreMargin,
			result.DecisionNote,
			string(matchReasons),
			string(srcSnapshot),
			string(dstSnapshot),
			result.CreatedAt,
		}
	}

	// DEFECT 4: CopyFrom returns count, verify all rows inserted
	rowCount, err := tx.CopyFrom(ctx,
		pgx.Identifier{"match_results"},
		[]string{
			"batch_id", "id", "source_id", "destination_id", "confidence_score",
			"name_score", "date_score", "match_status", "rank", "score_margin",
			"decision_note", "match_reasons", "source_snapshot", "destination_snapshot",
			"created_at",
		},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("copy from failed: %w", err)
	}

	if rowCount != int64(len(results)) {
		return fmt.Errorf("CopyFrom inserted %d rows, expected %d", rowCount, len(results))
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetResults retrieves all match results for a batch.
func (s *PostgresStore) GetResults(batchID string) ([]matcher.MatchResultItem, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First, verify the batch exists in match_jobs
	var exists int
	err := s.pool.QueryRow(ctx, "SELECT 1 FROM match_jobs WHERE batch_id = $1 LIMIT 1", batchID).Scan(&exists)
	if err != nil {
		return nil, false
	}

	// ORDER BY created_at ASC, id ASC: id is required as a tiebreaker because bulk-inserted
	// rows frequently share a created_at microsecond, which otherwise makes the order
	// non-deterministic. ASC (not DESC) is required so this matches the in-memory store,
	// which returns results in raw insertion order -- both backends must agree on order
	// for the same underlying data.
	rows, err := s.pool.Query(ctx,
		`SELECT batch_id, id, source_id, destination_id, confidence_score, name_score, date_score,
		        match_status, rank, score_margin, decision_note, match_reasons, source_snapshot,
		        destination_snapshot, created_at
		 FROM match_results WHERE batch_id = $1 ORDER BY created_at ASC, id ASC`,
		batchID)
	if err != nil {
		return nil, false
	}
	defer rows.Close()

	var results []matcher.MatchResultItem
	for rows.Next() {
		var result matcher.MatchResultItem
		var reasonsJSON, srcJSON, dstJSON string

		err := rows.Scan(
			&result.BatchID, &result.ID, &result.SourceID, &result.DestinationID,
			&result.ConfidenceScore, &result.NameScore, &result.DateScore,
			&result.MatchStatus, &result.Rank, &result.ScoreMargin, &result.DecisionNote,
			&reasonsJSON, &srcJSON, &dstJSON, &result.CreatedAt,
		)
		if err != nil {
			continue
		}

		// Unmarshal snapshots
		if srcJSON != "" {
			_ = json.Unmarshal([]byte(srcJSON), &result.Source)
		}
		if dstJSON != "" {
			_ = json.Unmarshal([]byte(dstJSON), &result.Destination)
		}
		if reasonsJSON != "" {
			_ = json.Unmarshal([]byte(reasonsJSON), &result.MatchReasons)
		}

		results = append(results, result)
	}

	if rows.Err() != nil {
		return nil, false
	}
	return results, true
}

// GetResultByID retrieves a single match result by ID (DEFECT 3: use composite key).
func (s *PostgresStore) GetResultByID(batchID, matchID string) (matcher.MatchResultItem, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result matcher.MatchResultItem
	var reasonsJSON, srcJSON, dstJSON string

	err := s.pool.QueryRow(ctx,
		`SELECT batch_id, id, source_id, destination_id, confidence_score, name_score, date_score,
		        match_status, rank, score_margin, decision_note, match_reasons, source_snapshot,
		        destination_snapshot, created_at
		 FROM match_results WHERE batch_id = $1 AND id = $2`,
		batchID, matchID).
		Scan(&result.BatchID, &result.ID, &result.SourceID, &result.DestinationID,
			&result.ConfidenceScore, &result.NameScore, &result.DateScore,
			&result.MatchStatus, &result.Rank, &result.ScoreMargin, &result.DecisionNote,
			&reasonsJSON, &srcJSON, &dstJSON, &result.CreatedAt)

	if err != nil {
		return matcher.MatchResultItem{}, false
	}

	// Unmarshal snapshots
	if srcJSON != "" {
		_ = json.Unmarshal([]byte(srcJSON), &result.Source)
	}
	if dstJSON != "" {
		_ = json.Unmarshal([]byte(dstJSON), &result.Destination)
	}
	if reasonsJSON != "" {
		_ = json.Unmarshal([]byte(reasonsJSON), &result.MatchReasons)
	}

	return result, true
}

// UpdateMatchStatus updates the status of a match result (DEFECT 3: use composite key).
func (s *PostgresStore) UpdateMatchStatus(batchID, matchID, newStatus string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := s.pool.Exec(ctx,
		`UPDATE match_results SET match_status = $1 WHERE batch_id = $2 AND id = $3`,
		newStatus, batchID, matchID)

	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("match record not found")
	}

	return nil
}

// GetResultsPage retrieves a paginated, filtered set of match results.
func (s *PostgresStore) GetResultsPage(batchID, status, search string, limit, offset int) ([]matcher.MatchResultItem, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build WHERE clause
	var whereConditions []string
	var args []interface{}
	argCount := 1

	whereConditions = append(whereConditions, fmt.Sprintf("batch_id = $%d", argCount))
	args = append(args, batchID)
	argCount++

	if status != "" && status != "ALL" {
		whereConditions = append(whereConditions, fmt.Sprintf("match_status = $%d", argCount))
		args = append(args, status)
		argCount++
	}

	if search != "" {
		searchLower := strings.ToLower(search)
		whereConditions = append(whereConditions,
			fmt.Sprintf("(LOWER(source_snapshot::text) LIKE $%d OR LOWER(destination_snapshot::text) LIKE $%d)",
				argCount, argCount+1))
		args = append(args, "%"+searchLower+"%", "%"+searchLower+"%")
		argCount += 2
	}

	whereClause := strings.Join(whereConditions, " AND ")

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM match_results WHERE %s", whereClause)
	var totalCount int
	err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("count query failed: %w", err)
	}

	// Get paginated results
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// ORDER BY created_at ASC, id ASC: id is required as a tiebreaker because bulk-inserted
	// rows frequently share a created_at microsecond; without it, Postgres does not guarantee
	// a consistent row order between the queries fetching different LIMIT/OFFSET pages, which
	// can cause a row to appear on two pages or on none. ASC matches the in-memory store's
	// insertion-order semantics so both backends agree on order for the same data.
	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT batch_id, id, source_id, destination_id, confidence_score, name_score, date_score,
		       match_status, rank, score_margin, decision_note, match_reasons, source_snapshot,
		       destination_snapshot, created_at
		FROM match_results WHERE %s
		ORDER BY created_at ASC, id ASC LIMIT $%d OFFSET $%d`,
		whereClause, argCount, argCount+1)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var results []matcher.MatchResultItem
	for rows.Next() {
		var result matcher.MatchResultItem
		var reasonsJSON, srcJSON, dstJSON string

		err := rows.Scan(
			&result.BatchID, &result.ID, &result.SourceID, &result.DestinationID,
			&result.ConfidenceScore, &result.NameScore, &result.DateScore,
			&result.MatchStatus, &result.Rank, &result.ScoreMargin, &result.DecisionNote,
			&reasonsJSON, &srcJSON, &dstJSON, &result.CreatedAt,
		)
		if err != nil {
			continue
		}

		// Unmarshal snapshots
		if srcJSON != "" {
			_ = json.Unmarshal([]byte(srcJSON), &result.Source)
		}
		if dstJSON != "" {
			_ = json.Unmarshal([]byte(dstJSON), &result.Destination)
		}
		if reasonsJSON != "" {
			_ = json.Unmarshal([]byte(reasonsJSON), &result.MatchReasons)
		}

		results = append(results, result)
	}

	return results, totalCount, nil
}

// UpdateProgress updates or creates a batch progress record.
func (s *PostgresStore) UpdateProgress(p matcher.BatchProgress) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	configJSON, _ := json.Marshal(p)

	_, _ = s.pool.Exec(ctx,
		`INSERT INTO match_jobs (batch_id, status, total_sources, auto_matched, review_needed,
		                        no_match_count, total_candidate_pairs, elapsed_ms, started_at,
		                        completed_at, config)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 ON CONFLICT (batch_id) DO UPDATE SET
		    status = EXCLUDED.status,
		    auto_matched = EXCLUDED.auto_matched,
		    review_needed = EXCLUDED.review_needed,
		    no_match_count = EXCLUDED.no_match_count,
		    total_candidate_pairs = EXCLUDED.total_candidate_pairs,
		    elapsed_ms = EXCLUDED.elapsed_ms,
		    completed_at = EXCLUDED.completed_at,
		    config = EXCLUDED.config`,
		p.BatchID, p.Status, p.TotalSources, p.AutoMatched, p.ReviewNeeded,
		p.NoMatchCount, p.TotalMatches, p.ElapsedMs, p.StartedAt, p.CompletedAt, configJSON)

	// Notify SSE listeners (in-memory)
	s.sseMu.Lock()
	if clients, exists := s.sseClients[p.BatchID]; exists {
		for _, ch := range clients {
			select {
			case ch <- p:
			default:
			}
		}
	}
	s.sseMu.Unlock()
}

// GetProgress retrieves the current progress for a batch.
func (s *PostgresStore) GetProgress(batchID string) (matcher.BatchProgress, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var p matcher.BatchProgress
	err := s.pool.QueryRow(ctx,
		`SELECT batch_id, total_sources, auto_matched, review_needed, no_match_count,
		        total_candidate_pairs, elapsed_ms, status, started_at, completed_at
		 FROM match_jobs WHERE batch_id = $1`,
		batchID).
		Scan(&p.BatchID, &p.TotalSources, &p.AutoMatched, &p.ReviewNeeded, &p.NoMatchCount,
			&p.TotalMatches, &p.ElapsedMs, &p.Status, &p.StartedAt, &p.CompletedAt)

	return p, err == nil
}

// RegisterSSEClient registers a new SSE client for progress updates.
func (s *PostgresStore) RegisterSSEClient(batchID string) chan matcher.BatchProgress {
	s.sseMu.Lock()
	defer s.sseMu.Unlock()

	ch := make(chan matcher.BatchProgress, 10)
	s.sseClients[batchID] = append(s.sseClients[batchID], ch)
	return ch
}

// UnregisterSSEClient removes an SSE client channel.
func (s *PostgresStore) UnregisterSSEClient(batchID string, ch chan matcher.BatchProgress) {
	s.sseMu.Lock()
	defer s.sseMu.Unlock()

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

// ManualLink creates a new match result via manual linking.
func (s *PostgresStore) ManualLink(batchID, sourceID, destinationID string) (*matcher.MatchResultItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	newID := fmt.Sprintf("%s-%s-%s-manual", batchID, sourceID, destinationID)
	createdAt := time.Now()

	srcSnapshot, _ := json.Marshal(map[string]string{"id": sourceID})
	dstSnapshot, _ := json.Marshal(map[string]string{"id": destinationID})
	reasons, _ := json.Marshal([]string{"Manually linked by user"})

	_, err := s.pool.Exec(ctx,
		`INSERT INTO match_results
		 (batch_id, id, source_id, destination_id, confidence_score, match_status,
		  match_reasons, source_snapshot, destination_snapshot, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		batchID, newID, sourceID, destinationID, 1.0, "CONFIRMED",
		string(reasons), string(srcSnapshot), string(dstSnapshot), createdAt)

	if err != nil {
		return nil, fmt.Errorf("insert manual link: %w", err)
	}

	return &matcher.MatchResultItem{
		ID:              newID,
		BatchID:         batchID,
		SourceID:        sourceID,
		DestinationID:   destinationID,
		ConfidenceScore: 1.0,
		MatchStatus:     "CONFIRMED",
		MatchReasons:    []string{"Manually linked by user"},
		CreatedAt:       createdAt,
	}, nil
}

// RecordAuditLog inserts an audit log entry (append-only).
func (s *PostgresStore) RecordAuditLog(entry AuditLogEntry) AuditLogEntry {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if entry.ID == "" {
		entry.ID = fmt.Sprintf("audit-%d", time.Now().UnixNano())
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.UserID == "" {
		entry.UserID = "reviewer_op"
	}

	_, _ = s.pool.Exec(ctx,
		`INSERT INTO match_audit_logs
		 (id, batch_id, source_id, destination_id, user_id, action, previous_status,
		  new_status, confidence_score, review_comments, timestamp)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		entry.ID, entry.BatchID, entry.SourceID, entry.DestinationID, entry.UserID,
		entry.Action, entry.PreviousStatus, entry.NewStatus, entry.ConfidenceScore,
		entry.ReviewComments, entry.Timestamp)

	return entry
}

// GetAuditLogs retrieves audit logs with optional filtering.
func (s *PostgresStore) GetAuditLogs(batchID, userID, actionFilter string) []AuditLogEntry {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var whereConditions []string
	var args []interface{}
	argCount := 1

	if batchID != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("batch_id = $%d", argCount))
		args = append(args, batchID)
		argCount++
	}

	if userID != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("user_id = $%d", argCount))
		args = append(args, userID)
		argCount++
	}

	if actionFilter != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("action = $%d", argCount))
		args = append(args, actionFilter)
		argCount++
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + strings.Join(whereConditions, " AND ")
	}

	// ORDER BY timestamp DESC, id DESC: id (the match_audit_logs primary key) is required as
	// a tiebreaker because multiple audit entries can share a timestamp, which otherwise
	// makes the returned order non-deterministic. DESC is kept for both columns to preserve
	// the existing most-recent-first intent.
	query := fmt.Sprintf(
		`SELECT id, batch_id, source_id, destination_id, user_id, action, previous_status,
		        new_status, confidence_score, review_comments, timestamp
		 FROM match_audit_logs %s ORDER BY timestamp DESC, id DESC`, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var entries []AuditLogEntry
	for rows.Next() {
		var entry AuditLogEntry
		_ = rows.Scan(&entry.ID, &entry.BatchID, &entry.SourceID, &entry.DestinationID,
			&entry.UserID, &entry.Action, &entry.PreviousStatus, &entry.NewStatus,
			&entry.ConfidenceScore, &entry.ReviewComments, &entry.Timestamp)
		entries = append(entries, entry)
	}

	return entries
}

// DeleteBatch deletes a batch and cascades to related records.
func (s *PostgresStore) DeleteBatch(batchID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = s.pool.Exec(ctx, "DELETE FROM match_jobs WHERE batch_id = $1", batchID)

	// Unregister SSE clients
	s.sseMu.Lock()
	if clients, exists := s.sseClients[batchID]; exists {
		for _, ch := range clients {
			close(ch)
		}
		delete(s.sseClients, batchID)
	}
	s.sseMu.Unlock()
}

// ListBatches lists all batches with summary information.
func (s *PostgresStore) ListBatches() []BatchSummary {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// ORDER BY created_at DESC, batch_id DESC: batch_id (the match_jobs primary key) is
	// required as a tiebreaker because multiple batches can share a created_at timestamp,
	// which otherwise makes the returned order non-deterministic. DESC is kept for both
	// columns to preserve the existing most-recent-first intent.
	rows, err := s.pool.Query(ctx,
		`SELECT batch_id, status, total_sources, total_destinations, created_at
		 FROM match_jobs ORDER BY created_at DESC, batch_id DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var summaries []BatchSummary
	for rows.Next() {
		var summary BatchSummary
		_ = rows.Scan(&summary.BatchID, &summary.Status, &summary.SourceCount,
			&summary.DestinationCount, &summary.CreatedAt)

		// Get result count
		_ = s.pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM match_results WHERE batch_id = $1", summary.BatchID).
			Scan(&summary.ResultCount)

		summaries = append(summaries, summary)
	}

	return summaries
}

// ListJobs lists matching jobs with pagination.
func (s *PostgresStore) ListJobs(limit, offset int) ([]JobSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// ORDER BY started_at DESC, batch_id DESC: batch_id (the match_jobs primary key) is
	// required as a tiebreaker because jobs started in the same batch/second otherwise sort
	// non-deterministically, which corrupts LIMIT/OFFSET pagination (a job can appear on two
	// pages or on none). DESC is kept for both columns to preserve the existing
	// most-recent-first intent.
	rows, err := s.pool.Query(ctx,
		`SELECT batch_id, status, total_sources, total_destinations, auto_matched, review_needed,
		        no_match_count, total_candidate_pairs, elapsed_ms, started_at, completed_at
		 FROM match_jobs ORDER BY started_at DESC, batch_id DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []JobSummary
	for rows.Next() {
		var job JobSummary
		var completedAt *time.Time
		var startedAt time.Time

		err := rows.Scan(&job.BatchID, &job.Status, &job.TotalSources, &job.TotalDestinations,
			&job.AutoMatched, &job.ReviewNeeded, &job.NoMatchCount, &job.TotalCandidatePairs,
			&job.ElapsedMs, &startedAt, &completedAt)
		if err != nil {
			continue
		}

		job.StartedAt = startedAt.Format(time.RFC3339)
		if completedAt != nil {
			job.CompletedAt = completedAt.Format(time.RFC3339)
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// Close gracefully closes the connection pool.
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// CalibrationObservations retrieves calibration observations for a batch from audit logs.
func (s *PostgresStore) CalibrationObservations(batchID string) ([]matcher.LabelledScore, error) {
	entries := s.GetAuditLogs(batchID, "", "")
	labels, _ := computeCalibrationObservations(entries)
	return labels, nil
}

// CalibrationObservationStats returns statistics about calibration observations for a batch.
func (s *PostgresStore) CalibrationObservationStats(batchID string) (CalibrationObservationStats, error) {
	entries := s.GetAuditLogs(batchID, "", "")
	_, stats := computeCalibrationObservations(entries)
	return stats, nil
}

// SaveCalibrationModel persists a calibration model, ensuring only one active model exists.
func (s *PostgresStore) SaveCalibrationModel(model CalibrationModel) (CalibrationModel, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if model.ID == "" {
		model.ID = fmt.Sprintf("calib-%d", time.Now().UnixNano())
	}
	if model.CreatedAt.IsZero() {
		model.CreatedAt = time.Now()
	}

	var err error
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CalibrationModel{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if model.Active {
		if _, err = tx.Exec(ctx, "UPDATE calibration_models SET active = false WHERE active = true"); err != nil {
			return CalibrationModel{}, fmt.Errorf("deactivate previous model: %w", err)
		}
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO calibration_models (id, created_at, fitted_by, batch_id, observation_count,
		                                 positive_count, brier_score, ece_score, model_json, active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		model.ID, model.CreatedAt, model.FittedBy, model.BatchID, model.ObservationCount,
		model.PositiveCount, model.BrierScore, model.ECEScore, model.ModelJSON, model.Active)
	if err != nil {
		return CalibrationModel{}, fmt.Errorf("insert model: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return CalibrationModel{}, fmt.Errorf("commit transaction: %w", err)
	}

	return model, nil
}

// GetActiveCalibrationModel retrieves the currently active calibration model, if any.
func (s *PostgresStore) GetActiveCalibrationModel() (CalibrationModel, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var model CalibrationModel
	err := s.pool.QueryRow(ctx,
		`SELECT id, created_at, fitted_by, batch_id, observation_count, positive_count,
		        brier_score, ece_score, model_json, active
		 FROM calibration_models WHERE active = true LIMIT 1`).
		Scan(&model.ID, &model.CreatedAt, &model.FittedBy, &model.BatchID,
			&model.ObservationCount, &model.PositiveCount, &model.BrierScore, &model.ECEScore,
			&model.ModelJSON, &model.Active)

	if err == pgx.ErrNoRows {
		return CalibrationModel{}, false, nil
	}
	if err != nil {
		return CalibrationModel{}, false, err
	}

	return model, true, nil
}

// ListCalibrationModels retrieves calibration models with pagination.
func (s *PostgresStore) ListCalibrationModels(limit, offset int) ([]CalibrationModel, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// ORDER BY created_at DESC, id DESC: id (the calibration_models primary key) is required
	// as a tiebreaker because multiple models can share a created_at timestamp, which
	// otherwise corrupts LIMIT/OFFSET pagination the same way it did in GetResultsPage (a
	// model can appear on two pages or on none). DESC is kept for both columns to preserve
	// the existing most-recent-first intent.
	rows, err := s.pool.Query(ctx,
		`SELECT id, created_at, fitted_by, batch_id, observation_count, positive_count,
		        brier_score, ece_score, model_json, active
		 FROM calibration_models ORDER BY created_at DESC, id DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query models: %w", err)
	}
	defer rows.Close()

	var models []CalibrationModel
	for rows.Next() {
		var model CalibrationModel
		if err := rows.Scan(&model.ID, &model.CreatedAt, &model.FittedBy, &model.BatchID,
			&model.ObservationCount, &model.PositiveCount, &model.BrierScore, &model.ECEScore,
			&model.ModelJSON, &model.Active); err != nil {
			continue
		}
		models = append(models, model)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return models, nil
}

// Compile-time assertion that PostgresStore implements Repository
var _ Repository = (*PostgresStore)(nil)
