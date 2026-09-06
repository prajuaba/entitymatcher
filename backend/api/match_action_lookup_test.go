package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"entitymatcher/matcher"
	"entitymatcher/store"

	"github.com/stretchr/testify/require"
)

// countingStore records how the handler looked a result up, so a test can tell
// an indexed lookup from a full-batch scan.
type countingStore struct {
	store.Repository
	getResultsCalls    int
	getResultByIDCalls int
}

func (c *countingStore) GetResults(batchID string) ([]matcher.MatchResultItem, bool) {
	c.getResultsCalls++
	return c.Repository.GetResults(batchID)
}

func (c *countingStore) GetResultByID(batchID, matchID string) (matcher.MatchResultItem, bool) {
	c.getResultByIDCalls++
	return c.Repository.GetResultByID(batchID, matchID)
}

func TestMatchActionUsesIndexedLookup(t *testing.T) {
	st := store.NewStore()
	batchID := "batch-lookup-1"

	results := []matcher.MatchResultItem{
		{
			ID:              "match-1",
			BatchID:         batchID,
			SourceID:        "src-1",
			DestinationID:   "dst-1",
			MatchStatus:     "REVIEW_NEEDED",
			ConfidenceScore: 0.87,
		},
	}
	require.NoError(t, st.SaveResultsCtx(context.Background(), batchID, results))

	cs := &countingStore{Repository: st}
	srv := NewServer(cs)

	payload := ActionPayload{
		BatchID: batchID,
		MatchID: "match-1",
		Action:  "CONFIRM",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/match-action", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.HandleMatchAction(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, cs.getResultByIDCalls)
	// This assertion fails if the handler falls back to a full-batch scan via GetResults instead of the indexed GetResultByID lookup.
	require.Equal(t, 0, cs.getResultsCalls)
}

func TestMatchActionStillRecordsAuditFieldsFromTheRow(t *testing.T) {
	st := store.NewStore()
	batchID := "batch-audit-1"

	results := []matcher.MatchResultItem{
		{
			ID:              "match-2",
			BatchID:         batchID,
			SourceID:        "src-2",
			DestinationID:   "dst-2",
			MatchStatus:     "REVIEW_NEEDED",
			ConfidenceScore: 0.92,
		},
	}
	require.NoError(t, st.SaveResultsCtx(context.Background(), batchID, results))

	srv := NewServer(st)

	payload := ActionPayload{
		BatchID:        batchID,
		MatchID:        "match-2",
		Action:         "CONFIRM",
		ReviewComments: "Looks good to me",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/match-action", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.HandleMatchAction(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	logs := st.GetAuditLogs(batchID, "", "")
	require.Len(t, logs, 1)

	entry := logs[0]
	require.Equal(t, "src-2", entry.SourceID)
	require.Equal(t, "dst-2", entry.DestinationID)
	require.Equal(t, "REVIEW_NEEDED", entry.PreviousStatus)
	require.Equal(t, 0.92, entry.ConfidenceScore)
}

func TestMatchActionOnUnknownMatchIDBehavesAsBefore(t *testing.T) {
	st := store.NewStore()
	batchID := "batch-unknown-1"

	results := []matcher.MatchResultItem{
		{
			ID:              "match-3",
			BatchID:         batchID,
			SourceID:        "src-3",
			DestinationID:   "dst-3",
			MatchStatus:     "REVIEW_NEEDED",
			ConfidenceScore: 0.75,
		},
	}
	require.NoError(t, st.SaveResultsCtx(context.Background(), batchID, results))

	srv := NewServer(st)

	payload := ActionPayload{
		BatchID: batchID,
		MatchID: "nonexistent-match-id",
		Action:  "CONFIRM",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/match-action", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.HandleMatchAction(w, req)

	// This pins the pre-existing not-found behaviour: GetResultByID finds nothing
	// but the handler still calls UpdateMatchStatus, which fails for an unknown id
	// and the handler responds 400; this must not regress to a different status.
	require.Equal(t, http.StatusBadRequest, w.Code)
}
