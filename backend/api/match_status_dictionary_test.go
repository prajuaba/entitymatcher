package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"entitymatcher/matcher"
	"entitymatcher/store"

	"github.com/stretchr/testify/require"
)

type failingDeleteStore struct {
	*store.Store
}

func (f *failingDeleteStore) DeleteDictionaryEntry(alias string) error {
	return fmt.Errorf("simulated store failure")
}

// TestHandleMatchStatusKnownBatchReturnsCounters verifies that GETting the status
// of a known batch returns the stored counters and metadata in the expected format.
func TestHandleMatchStatusKnownBatchReturnsCounters(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)

	st.UpdateProgress(matcher.BatchProgress{
		BatchID:          "batch-x",
		TotalSources:     10,
		ProcessedSources: 4,
		Status:           "RUNNING",
	})

	req := httptest.NewRequest("GET", "/api/match/status?batch_id=batch-x", nil)
	w := httptest.NewRecorder()

	srv.HandleMatchStatus(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp matcher.BatchProgress
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	require.Equal(t, "batch-x", resp.BatchID)
	require.Equal(t, int64(10), resp.TotalSources)
	require.Equal(t, int64(4), resp.ProcessedSources)
	require.Equal(t, "RUNNING", resp.Status)
}

// TestHandleMatchStatusUnknownBatchReturns404 verifies that GETting the status
// of an unknown batch returns a 404 error with appropriate message.
func TestHandleMatchStatusUnknownBatchReturns404(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)

	req := httptest.NewRequest("GET", "/api/match/status?batch_id=does-not-exist", nil)
	w := httptest.NewRecorder()

	srv.HandleMatchStatus(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "Batch not found")
}

// TestHandleMatchStatusCompletedDerivesProcessedSourcesFromTotal verifies that
// when a batch is completed, the processed_sources in the response equals total_sources
// regardless of what was stored (this ensures the handler's derivation logic is preserved).
func TestHandleMatchStatusCompletedDerivesProcessedSourcesFromTotal(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)

	st.UpdateProgress(matcher.BatchProgress{
		BatchID:          "batch-done",
		TotalSources:     7,
		ProcessedSources: 0,
		Status:           "COMPLETED",
	})

	req := httptest.NewRequest("GET", "/api/match/status?batch_id=batch-done", nil)
	w := httptest.NewRecorder()

	srv.HandleMatchStatus(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp matcher.BatchProgress
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// When status is COMPLETED, processed_sources must equal total_sources
	require.Equal(t, int64(7), resp.TotalSources)
	require.Equal(t, int64(7), resp.ProcessedSources)
}

// TestHandleDictionaryDeleteRemovesFromGlobalDictAndStore verifies that DELETEing
// an alias from the dictionary removes it from both the in-process global dictionary
// and the persistent store, and records it in ListDeletedDictionaryAliases.
func TestHandleDictionaryDeleteRemovesFromGlobalDictAndStore(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)

	alias := "test-delete-alias-xyz"
	dict := matcher.GetGlobalDictionary()
	dict.Set(alias, "Some Canonical")
	require.NoError(t, st.SaveDictionaryEntry(matcher.SynonymEntry{Alias: alias, Canonical: "Some Canonical"}))

	req := httptest.NewRequest("DELETE", "/api/dictionary?alias="+alias, nil)
	w := httptest.NewRecorder()

	srv.HandleDictionary(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify alias is removed from global dictionary
	_, found := dict.Lookup(alias)
	require.False(t, found, "alias should be deleted from global dictionary")

	// Verify alias is removed from persistent store
	entries, err := st.ListDictionaryEntries()
	require.NoError(t, err)
	for _, entry := range entries {
		require.NotEqual(t, alias, entry.Alias, "alias should not appear in list of entries")
	}

	// Verify alias is recorded in deleted aliases
	deleted, err := st.ListDeletedDictionaryAliases()
	require.NoError(t, err)
	found = false
	for _, delAlias := range deleted {
		if delAlias == alias {
			found = true
			break
		}
	}
	require.True(t, found, "alias should appear in list of deleted aliases")
}

// TestHandleDictionaryDeleteMissingAliasReturns400 verifies that DELETEing a dictionary
// entry without providing an alias query parameter returns a 400 error.
func TestHandleDictionaryDeleteMissingAliasReturns400(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)

	req := httptest.NewRequest("DELETE", "/api/dictionary", nil)
	w := httptest.NewRecorder()

	srv.HandleDictionary(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestHandleDictionaryDeleteStoreFailureReturns500 verifies that if the store's
// DeleteDictionaryEntry method fails, the handler returns a 500 error.
func TestHandleDictionaryDeleteStoreFailureReturns500(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(&failingDeleteStore{Store: st})

	alias := "test-fail-alias"
	dict := matcher.GetGlobalDictionary()
	dict.Set(alias, "Some Canonical")
	// The global dictionary is process-wide state; leaving this alias behind
	// would leak into any other test that reads it.
	t.Cleanup(func() { dict.Delete(alias) })

	req := httptest.NewRequest("DELETE", "/api/dictionary?alias="+alias, nil)
	w := httptest.NewRecorder()

	srv.HandleDictionary(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	// A failed delete must be a no-op, not a half-applied one. The handler
	// persists before mutating the in-process map precisely so that answering
	// 500 and silently dropping the alias from scoring cannot both happen; this
	// fails if that order is reversed.
	_, stillPresent := dict.Lookup(alias)
	require.True(t, stillPresent,
		"alias must survive in the live dictionary when the store write failed")
}
