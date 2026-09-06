package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"entitymatcher/matcher"
	"entitymatcher/store"

	"github.com/stretchr/testify/require"
)

// decodeResultsResponse decodes the handler's JSON body into a raw map so
// tests can assert both the VALUE of a field and the PRESENCE/ABSENCE of a
// field (e.g. status_counts), which a fixed struct with omitted fields
// wouldn't let us distinguish from "present but zero-valued".
func decodeResultsResponse(t *testing.T, body []byte) map[string]json.RawMessage {
	t.Helper()
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &raw))
	return raw
}

func rawInt(t *testing.T, raw map[string]json.RawMessage, key string) int {
	t.Helper()
	v, ok := raw[key]
	require.True(t, ok, "expected key %q in response", key)
	var n int
	require.NoError(t, json.Unmarshal(v, &n))
	return n
}

func rawString(t *testing.T, raw map[string]json.RawMessage, key string) string {
	t.Helper()
	v, ok := raw[key]
	require.True(t, ok, "expected key %q in response", key)
	var s string
	require.NoError(t, json.Unmarshal(v, &s))
	return s
}

// seedPagingBatch saves n results (with a distinct source each) into the
// given in-memory store so paging/limit/count tests have real rows to work
// with.
func seedPagingBatch(t *testing.T, st *store.Store, batchID string, n int) {
	t.Helper()

	sources := make([]matcher.SourceRecord, n)
	results := make([]matcher.MatchResultItem, n)
	for i := 0; i < n; i++ {
		srcID := fmt.Sprintf("%s-src-%d", batchID, i)
		sources[i] = matcher.SourceRecord{ID: srcID, BatchID: batchID, CustomerNameRaw: "Row"}
		results[i] = matcher.MatchResultItem{
			ID:              fmt.Sprintf("%s-result-%d", batchID, i),
			BatchID:         batchID,
			SourceID:        srcID,
			ConfidenceScore: float64(i) / 100,
			MatchStatus:     "REVIEW_NEEDED",
			CreatedAt:       time.Now(),
		}
	}

	require.NoError(t, st.SaveDataset(batchID, sources, nil))
	require.NoError(t, st.SaveResultsCtx(context.Background(), batchID, results))
}

// TestHandleGetResultsPagingMetadataFields verifies the JSON response always
// carries total_pages, page, limit, sort_by and sort_dir.
func TestHandleGetResultsPagingMetadataFields(t *testing.T) {
	st := store.NewStore()
	batchID := "paging-meta-batch"
	seedPagingBatch(t, st, batchID, 3)

	server := NewServer(st)

	req := httptest.NewRequest("GET", "/api/match/results?batch_id="+batchID, nil)
	w := httptest.NewRecorder()
	server.HandleGetResults(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	raw := decodeResultsResponse(t, w.Body.Bytes())

	require.Equal(t, 1, rawInt(t, raw, "total_pages"))
	require.Equal(t, 1, rawInt(t, raw, "page"))
	require.Equal(t, store.DefaultResultsPageSize, rawInt(t, raw, "limit"))
	require.Equal(t, store.SortByCreatedAt, rawString(t, raw, "sort_by"))
	require.Equal(t, "asc", rawString(t, raw, "sort_dir"))
}

// TestHandleGetResultsEmptyBatchTotalPagesIsOne verifies total_pages is
// floored at 1 (never 0) for a batch that has no results at all.
func TestHandleGetResultsEmptyBatchTotalPagesIsOne(t *testing.T) {
	st := store.NewStore()
	server := NewServer(st)

	req := httptest.NewRequest("GET", "/api/match/results?batch_id=totally-unknown-batch", nil)
	w := httptest.NewRecorder()
	server.HandleGetResults(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	raw := decodeResultsResponse(t, w.Body.Bytes())

	require.Equal(t, 1, rawInt(t, raw, "total_pages"))
	require.Equal(t, 0, rawInt(t, raw, "total_count"))

	// "results" must serialize as [], never JSON null.
	require.JSONEq(t, "[]", string(raw["results"]))
}

// TestHandleGetResultsLimitClampedTo200 verifies a limit above the maximum is
// clamped, and the CLAMPED value (not the raw request value) is echoed back.
func TestHandleGetResultsLimitClampedTo200(t *testing.T) {
	st := store.NewStore()
	batchID := "paging-limit-batch"
	seedPagingBatch(t, st, batchID, 3)

	server := NewServer(st)

	req := httptest.NewRequest("GET", "/api/match/results?batch_id="+batchID+"&limit=999999", nil)
	w := httptest.NewRecorder()
	server.HandleGetResults(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	raw := decodeResultsResponse(t, w.Body.Bytes())

	require.Equal(t, store.MaxResultsPageSize, rawInt(t, raw, "limit"))
}

// TestHandleGetResultsInvalidSortByFallsBackSilently verifies an invalid
// sort_by value is silently accepted and echoed back as created_at, rather
// than erroring or ever reaching a SQL layer.
func TestHandleGetResultsInvalidSortByFallsBackSilently(t *testing.T) {
	st := store.NewStore()
	batchID := "paging-badsort-batch"
	seedPagingBatch(t, st, batchID, 3)

	server := NewServer(st)

	req := httptest.NewRequest("GET", "/api/match/results?batch_id="+batchID+"&sort_by="+url.QueryEscape("id;DROP TABLE"), nil)
	w := httptest.NewRecorder()
	server.HandleGetResults(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	raw := decodeResultsResponse(t, w.Body.Bytes())

	require.Equal(t, store.SortByCreatedAt, rawString(t, raw, "sort_by"))
}

// TestHandleGetResultsStatusCountsFlag verifies status_counts is present only
// when include_counts is requested.
func TestHandleGetResultsStatusCountsFlag(t *testing.T) {
	st := store.NewStore()
	batchID := "paging-counts-batch"
	seedPagingBatch(t, st, batchID, 3)

	server := NewServer(st)

	// Without include_counts: the key must be ABSENT.
	req := httptest.NewRequest("GET", "/api/match/results?batch_id="+batchID, nil)
	w := httptest.NewRecorder()
	server.HandleGetResults(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	raw := decodeResultsResponse(t, w.Body.Bytes())
	_, present := raw["status_counts"]
	require.False(t, present, "status_counts must be absent when include_counts is not passed")

	// With include_counts=1: the key must be PRESENT.
	req = httptest.NewRequest("GET", "/api/match/results?batch_id="+batchID+"&include_counts=1", nil)
	w = httptest.NewRecorder()
	server.HandleGetResults(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	raw = decodeResultsResponse(t, w.Body.Bytes())
	_, present = raw["status_counts"]
	require.True(t, present, "status_counts must be present when include_counts=1")

	var counts map[string]int
	require.NoError(t, json.Unmarshal(raw["status_counts"], &counts))
	require.Equal(t, 3, counts["REVIEW_NEEDED"])
}

// TestHandleGetResultsBeyondLastPage verifies requesting a page far beyond
// the end of the data returns an empty results list with the TRUE
// total_count/total_pages, rather than an error.
func TestHandleGetResultsBeyondLastPage(t *testing.T) {
	st := store.NewStore()
	batchID := "paging-beyond-batch"
	seedPagingBatch(t, st, batchID, 25)

	server := NewServer(st)

	req := httptest.NewRequest("GET", "/api/match/results?batch_id="+batchID+"&limit=10&page=999", nil)
	w := httptest.NewRecorder()
	server.HandleGetResults(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	raw := decodeResultsResponse(t, w.Body.Bytes())

	require.Equal(t, 25, rawInt(t, raw, "total_count"))
	require.Equal(t, 3, rawInt(t, raw, "total_pages")) // ceil(25/10)
	require.JSONEq(t, "[]", string(raw["results"]))
}

// TestHandleGetResultsHugePageDoesNotOverflowToNegativeOffset is a regression test for the page-overflow defect:
// an unbounded page number, multiplied by the page size to compute a SQL OFFSET, could overflow into a negative number
// and turn a junk query parameter into a 500 instead of a harmlessly-clamped empty page.
func TestHandleGetResultsHugePageDoesNotOverflowToNegativeOffset(t *testing.T) {
	st := store.NewStore()
	batchID := "paging-hugepage-batch"
	seedPagingBatch(t, st, batchID, 5)
	server := NewServer(st)

	req := httptest.NewRequest("GET", "/api/match/results?batch_id="+batchID+"&page=9000000000000000000", nil)
	w := httptest.NewRecorder()
	server.HandleGetResults(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	body := w.Body.Bytes()
	raw := decodeResultsResponse(t, body)
	require.JSONEq(t, "[]", string(raw["results"]))
	require.Equal(t, store.MaxResultsPage, rawInt(t, raw, "page"))
	require.Equal(t, 5, rawInt(t, raw, "total_count"))
}
