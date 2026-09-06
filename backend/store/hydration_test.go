package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"entitymatcher/matcher"
	"github.com/stretchr/testify/require"
)

// The pipeline no longer embeds Source/Destination on each result item, so the store must attach them from its own dataset at save time.
func TestMemoryStoreHydratesResultsOnSave(t *testing.T) {
	store := NewStore()
	batchID := "hydrate-batch-1"

	sources := []matcher.SourceRecord{
		{ID: "src-1", BatchID: batchID, CustomerNameRaw: "Alice Smith"},
		{ID: "src-2", BatchID: batchID, CustomerNameRaw: "Bob Jones"},
	}
	dests := []matcher.DestinationRecord{
		{ID: "dst-1", BatchID: batchID, CustomerNameRaw: "Alice S"},
		{ID: "dst-2", BatchID: batchID, CustomerNameRaw: "Bob J"},
	}

	store.SaveDataset(batchID, sources, dests)

	results := []matcher.MatchResultItem{
		{ID: "res-1", BatchID: batchID, SourceID: "src-1", DestinationID: "dst-1", MatchStatus: "AUTO_MATCHED"},
		{ID: "res-2", BatchID: batchID, SourceID: "src-2", DestinationID: "dst-2", MatchStatus: "AUTO_MATCHED"},
	}

	err := store.SaveResultsCtx(context.Background(), batchID, results)
	require.NoError(t, err)

	retrieved, ok := store.GetResults(batchID)
	require.True(t, ok)
	require.Len(t, retrieved, 2)

	// Verify hydration by checking that Source and Destination are not nil
	sourceMap := make(map[string]*matcher.SourceRecord)
	for i := range sources {
		sourceMap[sources[i].ID] = &sources[i]
	}

	destMap := make(map[string]*matcher.DestinationRecord)
	for i := range dests {
		destMap[dests[i].ID] = &dests[i]
	}

	for _, item := range retrieved {
		require.NotNil(t, item.Source)
		require.NotNil(t, item.Destination)

		expectedSrc := sourceMap[item.SourceID]
		expectedDst := destMap[item.DestinationID]

		require.Equal(t, expectedSrc.CustomerNameRaw, item.Source.CustomerNameRaw)
		require.Equal(t, expectedDst.CustomerNameRaw, item.Destination.CustomerNameRaw)
	}
}

// A NO_MATCH row has an empty DestinationID; hydration must not invent a destination for it.
func TestMemoryStoreLeavesNoMatchDestinationNil(t *testing.T) {
	store := NewStore()
	batchID := "no-match-batch-1"

	sources := []matcher.SourceRecord{
		{ID: "src-1", BatchID: batchID, CustomerNameRaw: "Carol White"},
	}
	dests := []matcher.DestinationRecord{} // empty

	store.SaveDataset(batchID, sources, dests)

	results := []matcher.MatchResultItem{
		{ID: "res-1", BatchID: batchID, SourceID: "src-1", DestinationID: "", MatchStatus: "NO_MATCH"},
	}

	store.SaveResultsCtx(context.Background(), batchID, results)

	retrieved, ok := store.GetResults(batchID)
	require.True(t, ok)
	require.Len(t, retrieved, 1)

	require.NotNil(t, retrieved[0].Source)
	require.Nil(t, retrieved[0].Destination)
}

// This proves hydration shares storage with the dataset rather than copying it, which is the entire point of the fix.
func TestMemoryStoreHydrationSharesDatasetStorage(t *testing.T) {
	store := NewStore()
	batchID := "share-storage-batch-1"

	sources := []matcher.SourceRecord{
		{ID: "src-1", BatchID: batchID, CustomerNameRaw: "Dave Lee"},
	}
	dests := []matcher.DestinationRecord{
		{ID: "dst-1", BatchID: batchID, CustomerNameRaw: "Dave L"},
	}

	store.SaveDataset(batchID, sources, dests)

	results := []matcher.MatchResultItem{
		{ID: "res-1", BatchID: batchID, SourceID: "src-1", DestinationID: "dst-1", MatchStatus: "AUTO_MATCHED"},
	}

	store.SaveResultsCtx(context.Background(), batchID, results)

	retrieved, ok := store.GetResults(batchID)
	require.True(t, ok)
	require.Len(t, retrieved, 1)

	require.NotNil(t, retrieved[0].Source)

	storedSources, _, ok2 := store.GetDataset(batchID)
	require.True(t, ok2)

	// Find the index of the source record in storedSources
	var i int
	for i = range storedSources {
		if storedSources[i].ID == "src-1" {
			break
		}
	}

	// Mutate the stored record through the dataset
	storedSources[i].CustomerNameRaw = "Mutated Name"

	// Verify that the change is visible through the result item's Source pointer
	require.Equal(t, "Mutated Name", retrieved[0].Source.CustomerNameRaw)
}

// Proves the snapshot is written from the batch's dataset at save time, not from the (now-nil) Source/Destination fields on the result item.
func TestPostgresSnapshotsWrittenFromDataset(t *testing.T) {
	store := testPostgresStore(t)
	batchID := fmt.Sprintf("hydration-pg-%d", time.Now().UnixNano())
	defer store.DeleteBatch(batchID)

	sources := []matcher.SourceRecord{
		{ID: "src-1", BatchID: batchID, CustomerNameRaw: "Erin Park"},
	}
	dests := []matcher.DestinationRecord{
		{ID: "dst-1", BatchID: batchID, CustomerNameRaw: "Erin P"},
	}

	err := store.SaveDataset(batchID, sources, dests)
	require.NoError(t, err)

	results := []matcher.MatchResultItem{
		{ID: "res-1", BatchID: batchID, SourceID: "src-1", DestinationID: "dst-1", MatchStatus: "AUTO_MATCHED", CreatedAt: time.Now()},
	}

	err = store.SaveResultsCtx(context.Background(), batchID, results)
	require.NoError(t, err)

	retrieved, ok := store.GetResults(batchID)
	require.True(t, ok)
	require.Len(t, retrieved, 1)

	require.NotNil(t, retrieved[0].Source)
	require.Equal(t, "Erin Park", retrieved[0].Source.CustomerNameRaw)

	require.NotNil(t, retrieved[0].Destination)
	require.Equal(t, "Erin P", retrieved[0].Destination.CustomerNameRaw)
}
