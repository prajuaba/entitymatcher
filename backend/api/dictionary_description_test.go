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

// TestHandleDictionaryPostThenGetReturnsDescription verifies that when a dictionary entry
// is posted with a description, the description is preserved and returned on GET.
func TestHandleDictionaryPostThenGetReturnsDescription(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)

	alias := "test-desc-alias-xyz789"
	canonical := "Test Canonical Co"
	description := "A test bank alias with a description"

	t.Cleanup(func() {
		// Clean up the global dictionary to avoid leaking into other tests
		matcher.GetGlobalDictionary().Delete(alias)
	})

	// Prepare POST body
	entry := matcher.SynonymEntry{
		Alias:       alias,
		Canonical:   canonical,
		Description: description,
	}
	body, err := json.Marshal(entry)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/dictionary", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.HandleDictionary(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Now GET the dictionary
	req2 := httptest.NewRequest("GET", "/api/dictionary", nil)
	w2 := httptest.NewRecorder()
	srv.HandleDictionary(w2, req2)

	require.Equal(t, http.StatusOK, w2.Code)

	// Decode response
	var resp struct {
		Entries []matcher.SynonymEntry `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))

	// Find our entry
	var foundEntry matcher.SynonymEntry
	found := false
	for _, e := range resp.Entries {
		if e.Alias == alias {
			foundEntry = e
			found = true
			break
		}
	}

	require.True(t, found, "Entry with alias %s should be present in the dictionary", alias)
	require.Equal(t, description, foundEntry.Description)
	require.Equal(t, canonical, foundEntry.Canonical)
}
