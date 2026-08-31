package store

import (
	"fmt"
	"testing"
	"time"

	"entitymatcher/matcher"
	"github.com/stretchr/testify/require"
)

func TestListJobsReportsDatasetCounts(t *testing.T) {
	store := testPostgresStore(t)
	batchID := fmt.Sprintf("jobcount-batch-%d", time.Now().UnixNano())
	defer store.DeleteBatch(batchID)

	sources := make([]matcher.SourceRecord, 4)
	for i := range sources {
		sources[i] = matcher.SourceRecord{ID: fmt.Sprintf("src-%d", i)}
	}

	dests := make([]matcher.DestinationRecord, 7)
	for i := range dests {
		dests[i] = matcher.DestinationRecord{ID: fmt.Sprintf("dst-%d", i)}
	}

	err := store.SaveDataset(batchID, sources, dests)
	require.NoError(t, err)

	jobs, err := store.ListJobs(50, 0)
	require.NoError(t, err)

	var job JobSummary
	found := false
	for _, j := range jobs {
		if j.BatchID == batchID {
			job = j
			found = true
			break
		}
	}
	require.True(t, found, "batch not found in ListJobs")
	require.EqualValues(t, 4, job.TotalSources)
	require.EqualValues(t, 7, job.TotalDestinations)

	store.UpdateProgress(matcher.BatchProgress{
		BatchID:      batchID,
		Status:       "COMPLETED",
		AutoMatched:  2,
		TotalSources: 0, // deliberately zero, to prove UpdateProgress must not clobber SaveDataset's counts
		StartedAt:    time.Now(),
		CompletedAt:  time.Now(),
	})

	jobs, err = store.ListJobs(50, 0)
	require.NoError(t, err)

	job = JobSummary{}
	found = false
	for _, j := range jobs {
		if j.BatchID == batchID {
			job = j
			found = true
			break
		}
	}
	require.True(t, found, "batch not found in ListJobs")
	require.EqualValues(t, 4, job.TotalSources)
	require.EqualValues(t, 7, job.TotalDestinations)
	require.EqualValues(t, 2, job.AutoMatched)
}

// findJob returns the ListJobs row for batchID, failing the test if absent.
func findJob(t *testing.T, s *PostgresStore, batchID string) JobSummary {
	t.Helper()
	jobs, err := s.ListJobs(50, 0)
	require.NoError(t, err)
	for _, j := range jobs {
		if j.BatchID == batchID {
			return j
		}
	}
	require.FailNow(t, "batch not found in ListJobs", batchID)
	return JobSummary{}
}

// TestSaveDatasetRefreshesJobCountsOnReupload exercises the ON CONFLICT ... DO UPDATE SET
// path, which TestListJobsReportsDatasetCounts cannot reach because it saves each batch
// only once. With DO NOTHING the counts would still read 4 and 7 after the second upload:
// that stale-count bug is what this guards.
func TestSaveDatasetRefreshesJobCountsOnReupload(t *testing.T) {
	store := testPostgresStore(t)
	batchID := fmt.Sprintf("jobrecount-batch-%d", time.Now().UnixNano())
	defer store.DeleteBatch(batchID)

	makeSet := func(nSrc, nDst int) ([]matcher.SourceRecord, []matcher.DestinationRecord) {
		sources := make([]matcher.SourceRecord, nSrc)
		for i := range sources {
			sources[i] = matcher.SourceRecord{ID: fmt.Sprintf("src-%d", i)}
		}
		dests := make([]matcher.DestinationRecord, nDst)
		for i := range dests {
			dests[i] = matcher.DestinationRecord{ID: fmt.Sprintf("dst-%d", i)}
		}
		return sources, dests
	}

	sources, dests := makeSet(4, 7)
	require.NoError(t, store.SaveDataset(batchID, sources, dests))

	job := findJob(t, store, batchID)
	require.EqualValues(t, 4, job.TotalSources)
	require.EqualValues(t, 7, job.TotalDestinations)

	// Re-upload the same batch: more sources, fewer destinations. Moving the two counts in
	// opposite directions catches a fix that only ever grows a count, or that swaps them.
	sources, dests = makeSet(9, 2)
	require.NoError(t, store.SaveDataset(batchID, sources, dests))

	job = findJob(t, store, batchID)
	require.EqualValues(t, 9, job.TotalSources, "re-upload must refresh total_sources")
	require.EqualValues(t, 2, job.TotalDestinations, "re-upload must refresh total_destinations")
}
