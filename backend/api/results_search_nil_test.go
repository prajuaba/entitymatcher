package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"entitymatcher/matcher"
	"entitymatcher/store"

	"github.com/stretchr/testify/require"
)

func TestResultsSearchSurvivesNoMatchRow(t *testing.T) {
	store := store.NewStore()

	src := matcher.SourceRecord{
		ID:              "s1",
		BatchID:         "b1",
		ReferenceID:     "SRC1",
		CustomerNameRaw: "Alice Wonderland",
	}

	require.NoError(t, store.SaveDataset("b1", []matcher.SourceRecord{src}, nil))

	result := matcher.MatchResultItem{
		ID:            "r1",
		BatchID:       "b1",
		SourceID:      "s1",
		Source:        &src,
		DestinationID: "",
		Destination:   nil,
		MatchStatus:   "NO_MATCH",
		Rank:          1,
	}

	require.NoError(t, store.SaveResultsCtx(context.Background(), "b1", []matcher.MatchResultItem{result}))

	server := NewServer(store)

	req := httptest.NewRequest("GET", "/api/match/results?batch_id=b1&search=alice", nil)
	w := httptest.NewRecorder()

	// The mere fact this line is reached (no panic propagated out of HandleGetResults) is itself
	// the regression assertion for the nil-pointer-dereference bug being fixed.
	server.HandleGetResults(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestResultsSearchMatchesOnSourceWhenDestinationIsNil(t *testing.T) {
	store := store.NewStore()

	src := matcher.SourceRecord{
		ID:              "s1",
		BatchID:         "b1",
		ReferenceID:     "SRC1",
		CustomerNameRaw: "Alice Wonderland",
	}

	require.NoError(t, store.SaveDataset("b1", []matcher.SourceRecord{src}, nil))

	result := matcher.MatchResultItem{
		ID:            "r1",
		BatchID:       "b1",
		SourceID:      "s1",
		Source:        &src,
		DestinationID: "",
		Destination:   nil,
		MatchStatus:   "NO_MATCH",
		Rank:          1,
	}

	require.NoError(t, store.SaveResultsCtx(context.Background(), "b1", []matcher.MatchResultItem{result}))

	server := NewServer(store)

	req := httptest.NewRequest("GET", "/api/match/results?batch_id=b1&search=alice", nil)
	w := httptest.NewRecorder()

	server.HandleGetResults(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		TotalCount int                       `json:"total_count"`
		Results    []matcher.MatchResultItem `json:"results"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	require.Equal(t, 1, len(response.Results))
	require.Equal(t, 1, response.TotalCount)
}

func TestResultsSearchExcludesNonMatchingRow(t *testing.T) {
	store := store.NewStore()

	src := matcher.SourceRecord{
		ID:              "s1",
		BatchID:         "b1",
		ReferenceID:     "SRC1",
		CustomerNameRaw: "Alice Wonderland",
	}

	require.NoError(t, store.SaveDataset("b1", []matcher.SourceRecord{src}, nil))

	result := matcher.MatchResultItem{
		ID:            "r1",
		BatchID:       "b1",
		SourceID:      "s1",
		Source:        &src,
		DestinationID: "",
		Destination:   nil,
		MatchStatus:   "NO_MATCH",
		Rank:          1,
	}

	require.NoError(t, store.SaveResultsCtx(context.Background(), "b1", []matcher.MatchResultItem{result}))

	server := NewServer(store)

	req := httptest.NewRequest("GET", "/api/match/results?batch_id=b1&search=zzzznomatch", nil)
	w := httptest.NewRecorder()

	server.HandleGetResults(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		TotalCount int                       `json:"total_count"`
		Results    []matcher.MatchResultItem `json:"results"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	require.Equal(t, 0, len(response.Results))
	require.Equal(t, 0, response.TotalCount)
}
