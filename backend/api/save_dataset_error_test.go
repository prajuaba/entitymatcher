package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"entitymatcher/matcher"
	"entitymatcher/store"

	"github.com/stretchr/testify/require"
)

// failingSaveStore is a Repository whose SaveDataset always fails, so a handler
// can be tested for what it does when persistence fails. Everything else
// delegates to a real in-memory store.
type failingSaveStore struct {
	store.Repository
	err error
}

func (f *failingSaveStore) SaveDataset(batchID string, sources []matcher.SourceRecord, dests []matcher.DestinationRecord) error {
	return f.err
}

func buildTestPayload(t *testing.T) []byte {
	payload := DatasetPayload{
		BatchID: "test-batch-123",
		Sources: []map[string]interface{}{
			{
				"reference_id":     "SRC001",
				"customer_name":    "Acme Corp",
				"transaction_date": "2024-01-15",
				"transaction_type": "sale",
			},
			{
				"reference_id":     "SRC002",
				"customer_name":    "Beta LLC",
				"transaction_date": "2024-02-20",
				"transaction_type": "purchase",
			},
		},
		Destinations: []map[string]interface{}{
			{
				"customer_id":      "DST001",
				"customer_name":    "Acme Corp",
				"transaction_date": "2024-01-15",
			},
			{
				"customer_id":      "DST002",
				"customer_name":    "Beta LLC",
				"transaction_date": "2024-02-20",
			},
		},
	}

	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return body
}

func TestUploadSurfacesPersistenceFailure(t *testing.T) {
	body := buildTestPayload(t)

	server := NewServer(&failingSaveStore{Repository: store.NewStore(), err: errors.New("disk on fire")})

	req := httptest.NewRequest("POST", "/api/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.HandleUpload(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "disk on fire")
}

func TestUploadSucceedsWhenPersistenceSucceeds(t *testing.T) {
	body := buildTestPayload(t)

	server := NewServer(store.NewStore())

	req := httptest.NewRequest("POST", "/api/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.HandleUpload(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}
