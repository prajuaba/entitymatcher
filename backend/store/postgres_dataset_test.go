package store

import (
	"fmt"
	"os"
	"testing"
	"time"

	"entitymatcher/matcher"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPostgresSaveAndGetDatasetRoundTrip covers the core save/read path: 3 sources and 3
// destinations, including non-ASCII names and populated Attributes, survive a round trip.
func TestPostgresSaveAndGetDatasetRoundTrip(t *testing.T) {
	store := testPostgresStore(t)

	batchID := fmt.Sprintf("dataset-roundtrip-%d", time.Now().UnixNano())
	txnDate := time.Now().Truncate(time.Second).UTC()

	sources := []matcher.SourceRecord{
		{
			ID:              "src-1",
			BatchID:         batchID,
			ReferenceID:     "ref-1",
			CustomerNameRaw: "สมชาย ใจดี",
			TransactionDate: txnDate,
			TransactionType: "DEPOSIT",
			Attributes:      map[string]interface{}{"branch": "BKK-01", "amount": float64(1000)},
		},
		{
			ID:              "src-2",
			BatchID:         batchID,
			ReferenceID:     "ref-2",
			CustomerNameRaw: "John Doe",
			TransactionDate: txnDate,
			TransactionType: "WITHDRAWAL",
			Attributes:      map[string]interface{}{"branch": "NYC-02"},
		},
		{
			ID:              "src-3",
			BatchID:         batchID,
			ReferenceID:     "ref-3",
			CustomerNameRaw: "Jane Smith",
			TransactionDate: txnDate,
			TransactionType: "TRANSFER",
			Attributes:      map[string]interface{}{"branch": "LON-03"},
		},
	}

	dests := []matcher.DestinationRecord{
		{
			ID:              "dst-1",
			BatchID:         batchID,
			CustomerID:      "cust-1",
			CustomerNameRaw: "สมชาย ใจดี",
			TransactionDate: txnDate,
			Attributes:      map[string]interface{}{"region": "APAC"},
		},
		{
			ID:              "dst-2",
			BatchID:         batchID,
			CustomerID:      "cust-2",
			CustomerNameRaw: "John Doe",
			TransactionDate: txnDate,
			Attributes:      map[string]interface{}{"region": "NA"},
		},
		{
			ID:              "dst-3",
			BatchID:         batchID,
			CustomerID:      "cust-3",
			CustomerNameRaw: "Jane Smith",
			TransactionDate: txnDate,
			Attributes:      map[string]interface{}{"region": "EMEA"},
		},
	}

	store.SaveDataset(batchID, sources, dests)

	gotSrc, gotDst, ok := store.GetDataset(batchID)
	require.True(t, ok)
	require.Len(t, gotSrc, 3)
	require.Len(t, gotDst, 3)

	for i, want := range sources {
		got := gotSrc[i]
		assert.Equal(t, want.ID, got.ID)
		assert.Equal(t, want.ReferenceID, got.ReferenceID)
		assert.Equal(t, want.CustomerNameRaw, got.CustomerNameRaw)
		assert.WithinDuration(t, want.TransactionDate, got.TransactionDate, time.Second)
		assert.Equal(t, want.TransactionType, got.TransactionType)
		assert.Equal(t, want.Attributes, got.Attributes)
		assert.Equal(t, matcher.Normalize(want.CustomerNameRaw), got.NormalizedName)
		assert.NotEmpty(t, got.NormalizedName.Cleaned)
	}

	for i, want := range dests {
		got := gotDst[i]
		assert.Equal(t, want.ID, got.ID)
		assert.Equal(t, want.CustomerID, got.CustomerID)
		assert.Equal(t, want.CustomerNameRaw, got.CustomerNameRaw)
		assert.WithinDuration(t, want.TransactionDate, got.TransactionDate, time.Second)
		assert.Equal(t, want.Attributes, got.Attributes)
		assert.Equal(t, matcher.Normalize(want.CustomerNameRaw), got.NormalizedName)
	}
}

// TestPostgresGetDatasetMissingBatch confirms ok=false for a batch that was never saved.
func TestPostgresGetDatasetMissingBatch(t *testing.T) {
	store := testPostgresStore(t)

	batchID := fmt.Sprintf("dataset-missing-%d", time.Now().UnixNano())

	gotSrc, gotDst, ok := store.GetDataset(batchID)
	assert.False(t, ok)
	assert.Nil(t, gotSrc)
	assert.Nil(t, gotDst)
}

// TestPostgresSaveDatasetReplaces confirms re-saving a batch replaces rather than
// accumulates rows.
func TestPostgresSaveDatasetReplaces(t *testing.T) {
	store := testPostgresStore(t)

	batchID := fmt.Sprintf("dataset-replace-%d", time.Now().UnixNano())
	txnDate := time.Now().Truncate(time.Second).UTC()

	firstSources := []matcher.SourceRecord{
		{ID: "s1", BatchID: batchID, ReferenceID: "r1", CustomerNameRaw: "Alice", TransactionDate: txnDate, TransactionType: "DEPOSIT"},
		{ID: "s2", BatchID: batchID, ReferenceID: "r2", CustomerNameRaw: "Bob", TransactionDate: txnDate, TransactionType: "DEPOSIT"},
		{ID: "s3", BatchID: batchID, ReferenceID: "r3", CustomerNameRaw: "Carol", TransactionDate: txnDate, TransactionType: "DEPOSIT"},
	}
	firstDests := []matcher.DestinationRecord{
		{ID: "d1", BatchID: batchID, CustomerID: "c1", CustomerNameRaw: "Alice", TransactionDate: txnDate},
		{ID: "d2", BatchID: batchID, CustomerID: "c2", CustomerNameRaw: "Bob", TransactionDate: txnDate},
		{ID: "d3", BatchID: batchID, CustomerID: "c3", CustomerNameRaw: "Carol", TransactionDate: txnDate},
	}

	store.SaveDataset(batchID, firstSources, firstDests)

	gotSrc, gotDst, ok := store.GetDataset(batchID)
	require.True(t, ok)
	require.Len(t, gotSrc, 3)
	require.Len(t, gotDst, 3)

	secondSources := []matcher.SourceRecord{
		{ID: "s4", BatchID: batchID, ReferenceID: "r4", CustomerNameRaw: "Dave", TransactionDate: txnDate, TransactionType: "DEPOSIT"},
		{ID: "s5", BatchID: batchID, ReferenceID: "r5", CustomerNameRaw: "Erin", TransactionDate: txnDate, TransactionType: "DEPOSIT"},
	}
	secondDests := []matcher.DestinationRecord{
		{ID: "d4", BatchID: batchID, CustomerID: "c4", CustomerNameRaw: "Dave", TransactionDate: txnDate},
		{ID: "d5", BatchID: batchID, CustomerID: "c5", CustomerNameRaw: "Erin", TransactionDate: txnDate},
	}

	store.SaveDataset(batchID, secondSources, secondDests)

	gotSrc, gotDst, ok = store.GetDataset(batchID)
	require.True(t, ok)
	assert.Len(t, gotSrc, 2)
	assert.Len(t, gotDst, 2)
	assert.Equal(t, "s4", gotSrc[0].ID)
	assert.Equal(t, "s5", gotSrc[1].ID)
	assert.Equal(t, "d4", gotDst[0].ID)
	assert.Equal(t, "d5", gotDst[1].ID)
}

// TestPostgresSaveDatasetEmpty confirms an explicitly empty dataset is persisted as
// present-but-empty (ok=true, zero rows), distinguishable from a batch never saved at all.
func TestPostgresSaveDatasetEmpty(t *testing.T) {
	store := testPostgresStore(t)

	batchID := fmt.Sprintf("dataset-empty-%d", time.Now().UnixNano())

	store.SaveDataset(batchID, []matcher.SourceRecord{}, []matcher.DestinationRecord{})

	gotSrc, gotDst, ok := store.GetDataset(batchID)
	assert.True(t, ok)
	assert.Len(t, gotSrc, 0)
	assert.Len(t, gotDst, 0)
}

// TestPostgresDeleteBatchRemovesDataset confirms DeleteBatch cascades to match_sources and
// match_destinations: GetDataset flips to ok=false, and the underlying tables have zero rows.
func TestPostgresDeleteBatchRemovesDataset(t *testing.T) {
	store := testPostgresStore(t)

	batchID := fmt.Sprintf("dataset-delete-%d", time.Now().UnixNano())
	txnDate := time.Now().Truncate(time.Second).UTC()

	sources := []matcher.SourceRecord{
		{ID: "s1", BatchID: batchID, ReferenceID: "r1", CustomerNameRaw: "Alice", TransactionDate: txnDate, TransactionType: "DEPOSIT"},
	}
	dests := []matcher.DestinationRecord{
		{ID: "d1", BatchID: batchID, CustomerID: "c1", CustomerNameRaw: "Alice", TransactionDate: txnDate},
	}

	store.SaveDataset(batchID, sources, dests)

	_, _, ok := store.GetDataset(batchID)
	require.True(t, ok)

	store.DeleteBatch(batchID)

	_, _, ok = store.GetDataset(batchID)
	assert.False(t, ok)

	// Confirm the cascade actually removed rows, not just that match_jobs is gone.
	dsn := os.Getenv("TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "TEST_DATABASE_URL must be set (testPostgresStore above would have skipped otherwise)")
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	require.NoError(t, err)
	defer pool.Close()

	var srcCount, dstCount int
	err = pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM match_sources WHERE batch_id = $1", batchID).Scan(&srcCount)
	require.NoError(t, err)
	err = pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM match_destinations WHERE batch_id = $1", batchID).Scan(&dstCount)
	require.NoError(t, err)

	assert.Equal(t, 0, srcCount)
	assert.Equal(t, 0, dstCount)
}

// TestPostgresDatasetAttributesNestedAndUnicode confirms nested JSON structures and
// Unicode keys in Attributes survive the round trip intact.
func TestPostgresDatasetAttributesNestedAndUnicode(t *testing.T) {
	store := testPostgresStore(t)

	batchID := fmt.Sprintf("dataset-attrs-%d", time.Now().UnixNano())
	txnDate := time.Now().Truncate(time.Second).UTC()

	attrs := map[string]interface{}{
		"ชื่อ": "สมชาย",
		"nested": map[string]interface{}{
			"level2": map[string]interface{}{
				"list": []interface{}{"a", "b", float64(3)},
			},
		},
	}

	sources := []matcher.SourceRecord{
		{
			ID:              "s1",
			BatchID:         batchID,
			ReferenceID:     "r1",
			CustomerNameRaw: "สมชาย ใจดี",
			TransactionDate: txnDate,
			TransactionType: "DEPOSIT",
			Attributes:      attrs,
		},
	}
	dests := []matcher.DestinationRecord{
		{
			ID:              "d1",
			BatchID:         batchID,
			CustomerID:      "c1",
			CustomerNameRaw: "สมชาย ใจดี",
			TransactionDate: txnDate,
			Attributes:      attrs,
		},
	}

	store.SaveDataset(batchID, sources, dests)

	gotSrc, gotDst, ok := store.GetDataset(batchID)
	require.True(t, ok)
	require.Len(t, gotSrc, 1)
	require.Len(t, gotDst, 1)

	assert.Equal(t, attrs, gotSrc[0].Attributes)
	assert.Equal(t, attrs, gotDst[0].Attributes)
}
