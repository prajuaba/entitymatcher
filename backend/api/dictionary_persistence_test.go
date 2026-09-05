package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"entitymatcher/matcher"
	"entitymatcher/store"

	"github.com/stretchr/testify/require"
)

// TestHandleDictionaryPostPersistsEntry verifies that POSTing a new alias to
// HandleDictionary not only updates the in-process matcher.CustomDictionary
// but also persists it via the store, so it survives a restart. Asserting on
// st.ListDictionaryEntries() (rather than only on the HTTP response body,
// which is built from the in-memory dict.ListEntries()) is what fails if the
// store.SaveDictionaryEntry call is ever removed from the handler.
func TestHandleDictionaryPostPersistsEntry(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)

	payload := matcher.SynonymEntry{
		Alias:       "TestAlias",
		Canonical:   "Test Canonical",
		Description: "added by test",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/dictionary", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.HandleDictionary(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	entries, err := st.ListDictionaryEntries()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "testalias", entries[0].Alias)
	require.Equal(t, "Test Canonical", entries[0].Canonical)
}

// TestHandleDictionaryPostBlankAliasPersistsNothing verifies that a POST with
// a blank alias is a no-op on the store, matching the existing guard that
// already prevented it from being set in the in-memory dictionary.
func TestHandleDictionaryPostBlankAliasPersistsNothing(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)

	payload := matcher.SynonymEntry{
		Alias:     "",
		Canonical: "Some Canonical",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/dictionary", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.HandleDictionary(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	entries, err := st.ListDictionaryEntries()
	require.NoError(t, err)
	require.Len(t, entries, 0)
}

// TestHandleDictionaryPostBlankCanonicalPersistsNothing verifies that a POST
// with a blank canonical is a no-op on the store, matching the existing
// guard that already prevented it from being set in the in-memory dictionary.
func TestHandleDictionaryPostBlankCanonicalPersistsNothing(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)

	payload := matcher.SynonymEntry{
		Alias:     "SomeAlias",
		Canonical: "",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/dictionary", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.HandleDictionary(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	entries, err := st.ListDictionaryEntries()
	require.NoError(t, err)
	require.Len(t, entries, 0)
}
