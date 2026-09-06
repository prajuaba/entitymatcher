package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"entitymatcher/matcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPostgresManualLinkValidIDsSucceeds tests successful manual linking with valid IDs.
func TestPostgresManualLinkValidIDsSucceeds(t *testing.T) {
	store := testPostgresStore(t)

	batchID := fmt.Sprintf("manuallink-valid-%d", time.Now().UnixNano())

	// Seed one source and one destination
	err := store.SaveDataset(batchID,
		[]matcher.SourceRecord{
			{
				ID:              "src-1",
				BatchID:         batchID,
				ReferenceID:     "ref-1",
				CustomerNameRaw: "Alice",
				TransactionDate: time.Now().Truncate(time.Second).UTC(),
				TransactionType: "DEPOSIT",
			},
		},
		[]matcher.DestinationRecord{
			{
				ID:              "dst-1",
				BatchID:         batchID,
				CustomerID:      "cust-1",
				CustomerNameRaw: "Alice",
				TransactionDate: time.Now().Truncate(time.Second).UTC(),
			},
		})
	require.NoError(t, err)

	item, err := store.ManualLink(batchID, "src-1", "dst-1")
	require.NoError(t, err)
	require.NotNil(t, item)

	assert.Equal(t, 1.0, item.ConfidenceScore)
	assert.Equal(t, "CONFIRMED", item.MatchStatus)
	assert.Equal(t, batchID+"-src-1-dst-1-manual", item.ID)
}

// TestPostgresManualLinkEmptySourceIDReturnsNotFoundAndInsertsNoRow tests that
// linking with an empty source ID returns not found and inserts no row.
func TestPostgresManualLinkEmptySourceIDReturnsNotFoundAndInsertsNoRow(t *testing.T) {
	store := testPostgresStore(t)

	batchID := fmt.Sprintf("manuallink-emptysrc-%d", time.Now().UnixNano())

	// Seed one destination only
	err := store.SaveDataset(batchID,
		[]matcher.SourceRecord{},
		[]matcher.DestinationRecord{
			{
				ID:              "dst-1",
				BatchID:         batchID,
				CustomerID:      "cust-1",
				CustomerNameRaw: "Alice",
				TransactionDate: time.Now().Truncate(time.Second).UTC(),
			},
		})
	require.NoError(t, err)

	item, err := store.ManualLink(batchID, "", "dst-1")
	require.Error(t, err)
	assert.Equal(t, "source or destination record not found", err.Error())
	assert.Nil(t, item)

	// Verify no row was inserted
	var count int
	err = store.pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM match_results WHERE batch_id = $1", batchID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestPostgresManualLinkInventedSourceIDReturnsNotFoundAndInsertsNoRow tests that
// linking with a non-existent source ID returns not found and inserts no row.
func TestPostgresManualLinkInventedSourceIDReturnsNotFoundAndInsertsNoRow(t *testing.T) {
	store := testPostgresStore(t)

	batchID := fmt.Sprintf("manuallink-invented-%d", time.Now().UnixNano())

	// Seed one destination only
	err := store.SaveDataset(batchID,
		[]matcher.SourceRecord{},
		[]matcher.DestinationRecord{
			{
				ID:              "dst-1",
				BatchID:         batchID,
				CustomerID:      "cust-1",
				CustomerNameRaw: "Alice",
				TransactionDate: time.Now().Truncate(time.Second).UTC(),
			},
		})
	require.NoError(t, err)

	item, err := store.ManualLink(batchID, "does-not-exist-src", "dst-1")
	require.Error(t, err)
	assert.Equal(t, "source or destination record not found", err.Error())
	assert.Nil(t, item)

	// Verify no row was inserted
	var count int
	err = store.pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM match_results WHERE batch_id = $1", batchID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
