package store

import (
	"testing"

	"entitymatcher/matcher"

	"github.com/stretchr/testify/require"
)

// GetProgress scans match_jobs.completed_at, which is NULL until a run finishes.
// Scanning a NULL into a non-pointer time.Time fails, and because this function
// flattens its error into a bool, that failure surfaced as "batch not found" --
// so /api/match/status returned 404 for every batch that was uploaded but never
// matched, AND for every batch that was currently RUNNING. A caller could not
// distinguish a live job from a missing one.
//
// This is Postgres-only by nature: the in-memory store holds a struct and has no
// NULL to mis-scan, which is why the whole test suite passed while the deployed
// PostgreSQL path was broken -- the same store-divergence class as K4/M5/I2/O3.
func TestGetProgressHandlesNullCompletedAt(t *testing.T) {
	s := testPostgresStore(t)

	// SaveDataset writes a match_jobs row with no completed_at, which is exactly
	// the state of a batch that has been ingested but never matched.
	require.NoError(t, s.SaveDataset("null-completed-at",
		[]matcher.SourceRecord{{ID: "src-1", ReferenceID: "REF-1", CustomerNameRaw: "Bangkok Bank"}},
		[]matcher.DestinationRecord{{ID: "dest-1", CustomerID: "CUST-1", CustomerNameRaw: "Bangkok Bank PLC"}},
	))

	p, ok := s.GetProgress("null-completed-at")
	require.True(t, ok, "a batch with a NULL completed_at must be found, not reported missing")
	require.Equal(t, "null-completed-at", p.BatchID)
	require.True(t, p.CompletedAt.IsZero(), "an unfinished run has no completion time")

	// A batch that genuinely does not exist must still report missing, so the
	// fix cannot have been "always return true".
	_, ok = s.GetProgress("definitely-not-a-batch")
	require.False(t, ok)
}
