package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"entitymatcher/store"

	"github.com/stretchr/testify/require"
)

// Backlog K1 (file_path half), unblocked by M1.
//
// HandleConnectorIngest refused CSV/Excel outright until M1 shipped
// resolveConnectorFilePath, because accepting a caller-supplied server-side path
// without confinement is an arbitrary file-read primitive. These tests pin the
// two properties that make accepting it safe: a path inside CONNECTOR_FILE_ROOT
// is ingested, and anything outside it is refused before any file is opened.

func writeIngestCSV(t *testing.T, dir, name, header string, rows ...string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	body := header + "\n"
	for _, r := range rows {
		body += r + "\n"
	}
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

func ingestRequest(t *testing.T, srcPath, dstPath string) *httptest.ResponseRecorder {
	t.Helper()
	payload := map[string]interface{}{
		"batch_id": "filepath-ingest-test",
		"source": map[string]interface{}{
			"type": "CSV", "file_path": srcPath,
			"columns": []string{"reference_id", "customer_name"},
		},
		"destination": map[string]interface{}{
			"type": "CSV", "file_path": dstPath,
			"columns": []string{"customer_id", "CustomerName"},
		},
		"column_mapping": map[string]interface{}{
			"name_fields_src":  []string{"customer_name"},
			"name_fields_dest": []string{"CustomerName"},
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	srv := NewServer(store.NewStore())
	req := httptest.NewRequest("POST", "/api/connector/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.HandleConnectorIngest(w, req)
	return w
}

func TestConnectorIngestAcceptsFilePathInsideRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv(ConnectorFileRootEnv, root)

	src := writeIngestCSV(t, root, "src.csv", "reference_id,customer_name", "SRC-1,Bangkok Bank")
	dst := writeIngestCSV(t, root, "dst.csv", "customer_id,CustomerName", "DEST-1,Bangkok Bank PLC")

	w := ingestRequest(t, src, dst)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, float64(1), resp["source_count"])
	require.Equal(t, float64(1), resp["destination_count"])
}

func TestConnectorIngestRefusesFilePathOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir() // a sibling directory, deliberately not under root
	t.Setenv(ConnectorFileRootEnv, root)

	escaped := writeIngestCSV(t, outside, "secret.csv", "reference_id,customer_name", "SRC-1,Nope")
	ok := writeIngestCSV(t, root, "dst.csv", "customer_id,CustomerName", "DEST-1,Fine")

	// Source outside the root must be refused, and the message must name the side
	// so an operator can tell which half of the request was wrong.
	w := ingestRequest(t, escaped, ok)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "source")

	// The destination is caller-controlled too, so it must be checked as well.
	// Checking only the source would leave the hole half open.
	w = ingestRequest(t, ok, escaped)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "destination")
}

func TestConnectorIngestRefusesFilePathWhenRootUnset(t *testing.T) {
	dir := t.TempDir()
	// Unset must DENY. An operator who has not chosen a directory has not agreed
	// to expose one, and the alternative default is the whole filesystem.
	t.Setenv(ConnectorFileRootEnv, "")

	src := writeIngestCSV(t, dir, "src.csv", "reference_id,customer_name", "SRC-1,Bangkok Bank")
	dst := writeIngestCSV(t, dir, "dst.csv", "customer_id,CustomerName", "DEST-1,Bangkok Bank PLC")

	w := ingestRequest(t, src, dst)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), ConnectorFileRootEnv)
}
