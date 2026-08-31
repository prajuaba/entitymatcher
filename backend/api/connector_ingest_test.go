package api

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"entitymatcher/matcher"
	"entitymatcher/store"
)

// writeTestCSV creates a CSV file with exactly 'n' data rows (1-indexed) with columns "id,name"
// and returns the file path. Caller is responsible for cleanup if needed, but t.TempDir() is used.
func writeTestCSV(t *testing.T, n int) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "test.csv")

	f, err := os.Create(tmpPath)
	require.NoError(t, err)
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	// Header
	require.NoError(t, writer.Write([]string{"id", "name"}))

	// Data rows: row i has id=i and name=name-i
	for i := 1; i <= n; i++ {
		require.NoError(t, writer.Write([]string{strconv.Itoa(i), fmt.Sprintf("name-%d", i)}))
	}

	return tmpPath
}

func TestFetchAllRecordsPagesToExhaustion(t *testing.T) {
	tmpPath := writeTestCSV(t, 12345)

	cfg := matcher.ConnectionConfig{
		Type:     matcher.SourceTypeCSV,
		FilePath: tmpPath,
	}
	conn, err := matcher.NewDataConnector(cfg)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	rows, truncated, err := fetchAllRecords(context.Background(), conn, 50000)
	require.NoError(t, err)
	require.Len(t, rows, 12345)
	require.False(t, truncated)

	// Verify no duplicates and all IDs 1..12345 present
	idSet := make(map[int]bool)
	for _, row := range rows {
		idStr, ok := row["id"].(string)
		require.True(t, ok, "id field must be string")
		id, err := strconv.Atoi(idStr)
		require.NoError(t, err, "id must parse to int")
		require.False(t, idSet[id], "duplicate id %d", id)
		idSet[id] = true
	}

	require.Len(t, idSet, 12345, "expected exactly 12345 distinct IDs")
	for i := 1; i <= 12345; i++ {
		require.True(t, idSet[i], "missing id %d", i)
	}
}

func TestFetchAllRecordsReportsTruncation(t *testing.T) {
	tmpPath := writeTestCSV(t, 12345)

	cfg := matcher.ConnectionConfig{
		Type:     matcher.SourceTypeCSV,
		FilePath: tmpPath,
	}
	conn, err := matcher.NewDataConnector(cfg)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	rows, truncated, err := fetchAllRecords(context.Background(), conn, 100)
	require.NoError(t, err)
	require.Len(t, rows, 100)
	require.True(t, truncated)
}

func TestFetchAllRecordsExactCapIsNotTruncated(t *testing.T) {
	tmpPath := writeTestCSV(t, 100)

	cfg := matcher.ConnectionConfig{
		Type:     matcher.SourceTypeCSV,
		FilePath: tmpPath,
	}
	conn, err := matcher.NewDataConnector(cfg)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	rows, truncated, err := fetchAllRecords(context.Background(), conn, 100)
	require.NoError(t, err)
	require.Len(t, rows, 100)
	require.False(t, truncated)
}

// NOTE: This endpoint must never become an arbitrary server-side file read;
// CSV/Excel connector types must always be rejected here to prevent LFI.
func TestConnectorIngestRejectsFileTypes(t *testing.T) {
	server := NewServer(store.NewStore())

	body := map[string]interface{}{
		"source": matcher.ConnectionConfig{
			Type:     matcher.SourceTypeCSV,
			FilePath: "/etc/passwd",
		},
		"destination": matcher.ConnectionConfig{
			Type:     matcher.SourceTypeCSV,
			FilePath: "/etc/passwd",
		},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/connector/ingest", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	server.HandleConnectorIngest(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "/api/upload/file")
}

func TestConnectorIngestRejectsUnknownType(t *testing.T) {
	server := NewServer(store.NewStore())

	body := map[string]interface{}{
		"source": matcher.ConnectionConfig{
			Type: matcher.SourceType("REDIS"),
		},
		"destination": matcher.ConnectionConfig{
			Type: matcher.SourceType("REDIS"),
		},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/connector/ingest", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	server.HandleConnectorIngest(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "unsupported connector type")
}

func TestConnectorIngestFromPostgresThenMatch(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Open a pool directly to manage test tables
	u, err := url.Parse(dsn)
	require.NoError(t, err, "failed to parse TEST_DATABASE_URL")

	host := u.Hostname()
	portStr := u.Port()
	port, _ := strconv.Atoi(portStr)
	username := u.User.Username()
	password, _ := u.User.Password()
	database := strings.TrimPrefix(u.Path, "/")

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "failed to create pool")
	defer pool.Close()

	// Clean up any existing tables
	dropTables := []string{"em_ingest_src", "em_ingest_dst"}
	for _, tbl := range dropTables {
		_, err = pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", tbl))
		require.NoError(t, err, "failed to drop %s", tbl)
	}
	defer func() {
		for _, tbl := range dropTables {
			_, _ = pool.Exec(context.Background(), fmt.Sprintf("DROP TABLE IF EXISTS %s", tbl))
		}
	}()

	// Create tables
	_, err = pool.Exec(ctx, "CREATE TABLE em_ingest_src (id int primary key, reference_id text, customer_name text)")
	require.NoError(t, err, "failed to create em_ingest_src")
	_, err = pool.Exec(ctx, "CREATE TABLE em_ingest_dst (id int primary key, customer_id text, customer_name text)")
	require.NoError(t, err, "failed to create em_ingest_dst")

	// Populate tables
	const rowCount = 250
	batchSize := 100
	for start := 1; start <= rowCount; start += batchSize {
		end := start + batchSize - 1
		if end > rowCount {
			end = rowCount
		}

		var values []interface{}
		qb := strings.Builder{}
		qb.WriteString("INSERT INTO em_ingest_src (id, reference_id, customer_name) VALUES ")
		for i := start; i <= end; i++ {
			if i > start {
				qb.WriteString(", ")
			}
			idx := len(values) + 1
			qb.WriteString(fmt.Sprintf("($%d, $%d, $%d)", idx, idx+1, idx+2))
			values = append(values, i, fmt.Sprintf("SRC-%d", i), fmt.Sprintf("Customer %d", i))
		}
		_, err = pool.Exec(ctx, qb.String(), values...)
		require.NoError(t, err, "failed to insert source rows %d-%d", start, end)

		values = values[:0]
		qb.Reset()
		qb.WriteString("INSERT INTO em_ingest_dst (id, customer_id, customer_name) VALUES ")
		for i := start; i <= end; i++ {
			if i > start {
				qb.WriteString(", ")
			}
			idx := len(values) + 1
			qb.WriteString(fmt.Sprintf("($%d, $%d, $%d)", idx, idx+1, idx+2))
			values = append(values, i, fmt.Sprintf("DST-%d", i), fmt.Sprintf("Customer %d", i))
		}
		_, err = pool.Exec(ctx, qb.String(), values...)
		require.NoError(t, err, "failed to insert destination rows %d-%d", start, end)
	}

	// Build ConnectionConfig for source and destination
	srcCfg := matcher.ConnectionConfig{
		Type:         matcher.SourceTypePostgres,
		Host:         host,
		Port:         port,
		Username:     username,
		Password:     password,
		Database:     database,
		TableOrQuery: "em_ingest_src",
		ExtraParams:  map[string]interface{}{"sslmode": "disable"},
	}
	dstCfg := matcher.ConnectionConfig{
		Type:         matcher.SourceTypePostgres,
		Host:         host,
		Port:         port,
		Username:     username,
		Password:     password,
		Database:     database,
		TableOrQuery: "em_ingest_dst",
		ExtraParams:  map[string]interface{}{"sslmode": "disable"},
	}

	columnMapping := matcher.ColumnMapping{
		NameFieldsSrc:   []string{"customer_name"},
		NameFieldsDest:  []string{"customer_name"},
		RefIDSrc:        "reference_id",
		RefIDDest:       "customer_id",
		SecondaryFields: []matcher.SecondaryFieldMapping{},
	}

	batchID := fmt.Sprintf("ingest-test-%d", time.Now().UnixNano())
	reqBody := map[string]interface{}{
		"batch_id":       batchID,
		"column_mapping": columnMapping,
		"source":         srcCfg,
		"destination":    dstCfg,
	}
	bodyBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/connector/ingest", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	server := NewServer(store.NewStore())
	server.HandleConnectorIngest(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, float64(250), resp["source_count"])
	require.Equal(t, float64(250), resp["destination_count"])
	require.False(t, resp["truncated"].(bool))

	// Verify dataset persisted
	sources, destinations, ok := server.store.GetDataset(batchID)
	require.True(t, ok)
	require.Len(t, sources, 250)
	require.Len(t, destinations, 250)

	// Run match
	matchReq := httptest.NewRequest("POST", "/api/match/run?batch_id="+batchID, nil)
	matchW := httptest.NewRecorder()
	server.HandleRunMatch(matchW, matchReq)
	require.Equal(t, http.StatusOK, matchW.Code)

	// Poll for match completion with timeout
	pollCtx, pollCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer pollCancel()

	var progress matcher.BatchProgress
	found := false
	for !found {
		select {
		case <-pollCtx.Done():
			require.Fail(t, "match run did not complete within timeout")
			return
		default:
			p, exists := server.store.GetProgress(batchID)
			if exists && p.Status == "COMPLETED" {
				progress = p
				found = true
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	require.Equal(t, "COMPLETED", progress.Status)
}

// recordingConnector is a DataConnector that serves `total` synthetic rows and
// records the (limit, offset) of every FetchRecords call, so a test can assert
// how ingestion paged rather than only what it returned.
type recordingConnector struct {
	total int
	calls []fetchCall
}

type fetchCall struct {
	limit  int
	offset int
}

func (c *recordingConnector) TestConnection(ctx context.Context) error { return nil }

func (c *recordingConnector) IntrospectSchema(ctx context.Context) ([]matcher.ColumnDef, error) {
	return nil, nil
}

func (c *recordingConnector) Close() error { return nil }

func (c *recordingConnector) FetchRecords(ctx context.Context, limit, offset int) ([]map[string]interface{}, error) {
	c.calls = append(c.calls, fetchCall{limit: limit, offset: offset})

	if offset >= c.total {
		return []map[string]interface{}{}, nil
	}

	end := offset + limit
	if end > c.total {
		end = c.total
	}

	rows := make([]map[string]interface{}, 0, end-offset)
	for i := offset; i < end; i++ {
		rows = append(rows, map[string]interface{}{"id": i + 1})
	}
	return rows, nil
}

func TestFetchAllRecordsIssuesBoundedPages(t *testing.T) {
	conn := &recordingConnector{total: 12345}

	rows, truncated, err := fetchAllRecords(context.Background(), conn, 50000)
	require.NoError(t, err)
	require.Len(t, rows, 12345)
	require.False(t, truncated)

	// Every FetchRecords call must be bounded by IngestPageSize. A mutation that
	// replaced the paging loop with a single unbounded call (limit = maxRecords)
	// would produce a call with limit 50000 and trip this assertion — this is
	// exactly the regression this test exists to catch.
	for i, call := range conn.calls {
		require.LessOrEqual(t, call.limit, IngestPageSize, "call %d has unbounded limit %d", i, call.limit)
	}

	// 12345 rows at 5000 per page means exactly 3 calls (5000 + 5000 + 2345).
	require.Len(t, conn.calls, 3)

	// Offsets must be 0, 5000, 10000 in order.
	offsets := make([]int, len(conn.calls))
	for i, call := range conn.calls {
		offsets[i] = call.offset
	}
	require.Equal(t, []int{0, 5000, 10000}, offsets)

	// Verify the ids in rows are the complete set 1..12345 with no duplicates.
	idSet := make(map[int]bool, len(rows))
	for _, row := range rows {
		id, ok := row["id"].(int)
		require.True(t, ok, "id field must be int, got %T", row["id"])
		require.False(t, idSet[id], "duplicate id %d", id)
		idSet[id] = true
	}
	require.Len(t, idSet, 12345)
	for i := 1; i <= 12345; i++ {
		require.True(t, idSet[i], "missing id %d", i)
	}
}

// The truncation probe is the last FetchRecords call that asks for one more row
// beyond the cap. It is what distinguishes "source has exactly maxRecords rows"
// (probe returns empty → not truncated) from "source has at least maxRecords+1
// rows" (probe returns a row → truncated). A naive `len(rows) == cap` check
// cannot make that distinction, because both cases produce the same number of
// rows; only the probe reveals whether the source is exhausted or merely capped.
func TestFetchAllRecordsProbesBeyondTheCap(t *testing.T) {
	t.Run("exactly at cap", func(t *testing.T) {
		conn := &recordingConnector{total: 10000}
		rows, truncated, err := fetchAllRecords(context.Background(), conn, 10000)
		require.NoError(t, err)
		require.False(t, truncated)
		require.Len(t, rows, 10000)

		// The last call must be the truncation probe: limit=1, offset=10000.
		require.NotEmpty(t, conn.calls)
		last := conn.calls[len(conn.calls)-1]
		require.Equal(t, fetchCall{limit: 1, offset: 10000}, last,
			"last call must be the 1-row truncation probe at offset 10000")
	})

	t.Run("more rows remain", func(t *testing.T) {
		conn := &recordingConnector{total: 10001}
		rows, truncated, err := fetchAllRecords(context.Background(), conn, 10000)
		require.NoError(t, err)
		require.True(t, truncated)
		require.Len(t, rows, 10000)
	})
}

func TestFetchAllRecordsStopsOnEmptySource(t *testing.T) {
	conn := &recordingConnector{total: 0}

	rows, truncated, err := fetchAllRecords(context.Background(), conn, 50000)
	require.NoError(t, err)
	require.Len(t, rows, 0)
	require.False(t, truncated)

	// An empty source must be recognised from the single empty page; the function
	// must not loop forever or issue a pointless truncation probe on zero rows.
	require.Len(t, conn.calls, 1, "expected exactly one FetchRecords call for an empty source, got %d", len(conn.calls))
}
