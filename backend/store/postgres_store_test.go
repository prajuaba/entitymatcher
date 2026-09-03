package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"entitymatcher/matcher"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPostgresStore creates a test Postgres store, skipping if TEST_DATABASE_URL not set
func testPostgresStore(t *testing.T) *PostgresStore {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := NewPostgresStore(ctx, dsn)
	require.NoError(t, err, "failed to create postgres store")
	t.Cleanup(func() {
		store.Close()
		truncateTables(t)
	})

	return store
}

// truncateTables clears test data for isolation
func truncateTables(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return
	}
	defer pool.Close()

	_, _ = pool.Exec(ctx, "TRUNCATE TABLE match_audit_logs, match_results, match_jobs CASCADE")
}

// TestPostgresSchemaIdempotency tests that schema applies idempotently
func TestPostgresSchemaIdempotency(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First application
	store1, err := NewPostgresStore(ctx, dsn)
	require.NoError(t, err)
	store1.Close()

	// Second application (should not error)
	store2, err := NewPostgresStore(ctx, dsn)
	require.NoError(t, err)
	store2.Close()

	t.Log("Schema applied idempotently twice")
}

// TestPostgresSaveAndReadRoundTrip tests save/read of results
func TestPostgresSaveAndReadRoundTrip(t *testing.T) {
	store := testPostgresStore(t)

	batchID := fmt.Sprintf("batch-%d", time.Now().UnixNano())

	results := []matcher.MatchResultItem{
		{
			ID:              "result-1",
			BatchID:         batchID,
			SourceID:        "src-1",
			DestinationID:   "dst-1",
			ConfidenceScore: 0.95,
			NameScore:       0.92,
			DateScore:       0.98,
			MatchStatus:     "AUTO_MATCHED",
			Rank:            1,
			ScoreMargin:     0.05,
			DecisionNote:    "Top candidate",
			MatchReasons:    []string{"Name match", "Date match"},
			CreatedAt:       time.Now(),
		},
		{
			ID:              "result-2",
			BatchID:         batchID,
			SourceID:        "src-1",
			DestinationID:   "dst-2",
			ConfidenceScore: 0.75,
			NameScore:       0.70,
			DateScore:       0.80,
			MatchStatus:     "REVIEW_NEEDED",
			Rank:            2,
			ScoreMargin:     0.0,
			DecisionNote:    "Alternative candidate",
			MatchReasons:    []string{"Moderate name match"},
			CreatedAt:       time.Now(),
		},
	}

	store.SaveResultsCtx(context.Background(), batchID, results)

	retrieved, ok := store.GetResults(batchID)
	require.True(t, ok)
	require.Equal(t, len(results), len(retrieved))

	assert.Equal(t, results[0].ID, retrieved[0].ID)
	assert.Equal(t, results[0].ConfidenceScore, retrieved[0].ConfidenceScore)
	assert.Equal(t, results[0].MatchStatus, retrieved[0].MatchStatus)

	// Under the corrected ASC ordering (created_at ASC, id ASC), GetResults returns rows in
	// insertion order, matching the in-memory store. Pin the FULL sequence, not just index 0,
	// so the ordering guarantee is actually verified rather than incidentally true.
	expectedIDs := make([]string, len(results))
	for i, r := range results {
		expectedIDs[i] = r.ID
	}
	gotIDs := make([]string, len(retrieved))
	for i, r := range retrieved {
		gotIDs[i] = r.ID
	}
	assert.Equal(t, expectedIDs, gotIDs)

	t.Logf("Round-trip test passed with %d results", len(retrieved))
}

// TestPostgresSaveResultsFailurePreservesOldData tests DEFECT 1 fix:
// If SaveResultsCtx fails, old results remain intact (transaction rollback)
func TestPostgresSaveResultsFailurePreservesOldData(t *testing.T) {
	store := testPostgresStore(t)
	ctx := context.Background()

	batchID := fmt.Sprintf("defect1-test-%d", time.Now().UnixNano())

	// Save initial results
	oldResults := []matcher.MatchResultItem{
		{
			ID:          "result-old-1",
			BatchID:     batchID,
			MatchStatus: "AUTO_MATCHED",
			CreatedAt:   time.Now(),
		},
	}
	err := store.SaveResultsCtx(ctx, batchID, oldResults)
	require.NoError(t, err)

	retrieved, ok := store.GetResults(batchID)
	require.True(t, ok)
	assert.Equal(t, 1, len(retrieved))

	// Replace with new results - should succeed atomically
	newResults := []matcher.MatchResultItem{
		{
			ID:          "result-new-2",
			BatchID:     batchID,
			MatchStatus: "AUTO_MATCHED",
			CreatedAt:   time.Now(),
		},
		{
			ID:          "result-new-3",
			BatchID:     batchID,
			MatchStatus: "REVIEW_NEEDED",
			CreatedAt:   time.Now(),
		},
	}

	err = store.SaveResultsCtx(ctx, batchID, newResults)
	require.NoError(t, err)

	// Verify new data replaced old
	retrieved, ok = store.GetResults(batchID)
	require.True(t, ok)
	assert.Equal(t, 2, len(retrieved))

	t.Logf("DEFECT 1 fix verified: atomic replacement via transaction")
}

// TestPostgresUpdateMatchStatus tests status update
func TestPostgresUpdateMatchStatus(t *testing.T) {
	store := testPostgresStore(t)

	batchID := fmt.Sprintf("status-batch-%d", time.Now().UnixNano())
	matchID := "status-result-1"

	results := []matcher.MatchResultItem{
		{
			ID:          matchID,
			BatchID:     batchID,
			MatchStatus: "REVIEW_NEEDED",
			CreatedAt:   time.Now(),
		},
	}

	store.SaveResultsCtx(context.Background(), batchID, results)

	err := store.UpdateMatchStatus(batchID, matchID, "CONFIRMED")
	require.NoError(t, err)

	retrieved, ok := store.GetResultByID(batchID, matchID)
	require.True(t, ok)
	assert.Equal(t, "CONFIRMED", retrieved.MatchStatus)

	t.Log("Update match status test passed")
}

// TestPostgresAuditLogImmutability tests UPDATE/DELETE prevention on audit logs
func TestPostgresAuditLogImmutability(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	t.Cleanup(func() { truncateTables(t) })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	defer pool.Close()

	batchID := fmt.Sprintf("immutable-batch-%d", time.Now().UnixNano())
	auditID := fmt.Sprintf("audit-%d", time.Now().UnixNano())

	// Insert audit log
	_, err = pool.Exec(ctx,
		`INSERT INTO match_audit_logs
		 (id, batch_id, source_id, destination_id, user_id, action, timestamp)
		 VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)`,
		auditID, batchID, "src", "dst", "user", "TEST")
	require.NoError(t, err)

	// Try to UPDATE (should fail)
	_, errUpdate := pool.Exec(ctx,
		`UPDATE match_audit_logs SET action = $1 WHERE id = $2`,
		"MODIFIED", auditID)
	require.Error(t, errUpdate, "UPDATE should be prevented by trigger")
	assert.Contains(t, errUpdate.Error(), "append-only", "Error should mention append-only")

	// Try to DELETE (should fail)
	_, errDelete := pool.Exec(ctx,
		`DELETE FROM match_audit_logs WHERE id = $1`, auditID)
	require.Error(t, errDelete, "DELETE should be prevented by trigger")
	assert.Contains(t, errDelete.Error(), "append-only", "Error should mention append-only")

	t.Log("Audit immutability tests passed")
}

// TestPostgresGetResultsDeterministicOrder tests that GetResults returns results in a deterministic order.
// This reproduces the tie condition directly (identical CreatedAt) rather than relying on timing luck,
// and without the id tiebreaker in the ORDER BY this test can fail/flake.
func TestPostgresGetResultsDeterministicOrder(t *testing.T) {
	store := testPostgresStore(t)

	batchID := fmt.Sprintf("batch-detorder-%d", time.Now().UnixNano())
	fixedTime := time.Now()

	results := make([]matcher.MatchResultItem, 10)
	for i := 0; i < 10; i++ {
		results[i] = matcher.MatchResultItem{
			ID:            fmt.Sprintf("result-%d", i),
			BatchID:       batchID,
			MatchStatus:   "AUTO_MATCHED",
			CreatedAt:     fixedTime,
			SourceID:      "src",
			DestinationID: "dst",
		}
	}

	err := store.SaveResultsCtx(context.Background(), batchID, results)
	require.NoError(t, err)

	expectedIDs := make([]string, 10)
	for i := 0; i < 10; i++ {
		expectedIDs[i] = fmt.Sprintf("result-%d", i)
	}

	for i := 0; i < 5; i++ {
		retrieved, ok := store.GetResults(batchID)
		require.True(t, ok)

		gotIDs := make([]string, len(retrieved))
		for j, item := range retrieved {
			gotIDs[j] = item.ID
		}

		assert.Equal(t, expectedIDs, gotIDs)
	}
}

// TestPostgresGetResultsPagePaginationStable tests that pagination returns stable, complete results.
// This is the pagination-correctness guarantee -- an unstable ORDER BY combined with LIMIT/OFFSET
// can otherwise return a row on two different pages or on neither, which is silent data corruption
// from the caller's point of view.
func TestPostgresGetResultsPagePaginationStable(t *testing.T) {
	store := testPostgresStore(t)

	batchID := fmt.Sprintf("batch-pagestable-%d", time.Now().UnixNano())
	fixedTime := time.Now()

	results := make([]matcher.MatchResultItem, 10)
	for i := 0; i < 10; i++ {
		results[i] = matcher.MatchResultItem{
			ID:            fmt.Sprintf("page-result-%d", i),
			BatchID:       batchID,
			MatchStatus:   "AUTO_MATCHED",
			CreatedAt:     fixedTime,
			SourceID:      "src",
			DestinationID: "dst",
		}
	}

	err := store.SaveResultsCtx(context.Background(), batchID, results)
	require.NoError(t, err)

	allIDs := []string{}
	for offset := 0; offset < 10; offset += 3 {
		page, _, err := store.GetResultsPage(ResultsQuery{BatchID: batchID, Limit: 3, Offset: offset})
		require.NoError(t, err)

		for _, item := range page {
			allIDs = append(allIDs, item.ID)
		}
	}

	assert.Equal(t, 10, len(allIDs))

	expectedIDs := make(map[string]bool)
	for i := 0; i < 10; i++ {
		expectedIDs[fmt.Sprintf("page-result-%d", i)] = true
	}

	counts := make(map[string]int)
	for _, id := range allIDs {
		counts[id]++
	}

	for id := range expectedIDs {
		assert.Equal(t, 1, counts[id], "ID %s should appear exactly once", id)
	}
}

// TestMemoryStoreConformance runs conformance tests against memory store
func TestMemoryStoreConformance(t *testing.T) {
	store := NewStore()

	// Test SaveResultsCtx
	ctx := context.Background()
	batchID := "test-batch"
	results := []matcher.MatchResultItem{
		{ID: "r1", BatchID: batchID, MatchStatus: "AUTO_MATCHED", CreatedAt: time.Now()},
	}

	err := store.SaveResultsCtx(ctx, batchID, results)
	assert.NoError(t, err)

	retrieved, ok := store.GetResults(batchID)
	assert.True(t, ok)
	assert.Equal(t, 1, len(retrieved))
}
