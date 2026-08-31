package matcher

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

var _ DataConnector = (*PostgresConnector)(nil)
var _ DataConnector = (*SQLServerConnector)(nil)
var _ DataConnector = (*MongoConnector)(nil)
var _ DataConnector = (*CSVConnector)(nil)
var _ DataConnector = (*ExcelConnector)(nil)
var _ DataConnector = (*ManualConnector)(nil)

func TestConnectorsImplementDataConnector(t *testing.T) {
	types := []SourceType{
		SourceTypePostgres,
		SourceTypeSQLServer,
		SourceTypeMongoDB,
		SourceTypeCSV,
		SourceTypeExcel,
		SourceTypeManual,
	}

	for _, st := range types {
		conn, err := NewDataConnector(ConnectionConfig{Type: st})
		require.NoError(t, err, "NewDataConnector(%s) returned error", st)
		require.NotNil(t, conn, "NewDataConnector(%s) returned nil", st)
	}
}

func TestCloseIsSafeWithoutConnect(t *testing.T) {
	connectors := []DataConnector{
		&PostgresConnector{Config: ConnectionConfig{Type: SourceTypePostgres}},
		&SQLServerConnector{Config: ConnectionConfig{Type: SourceTypeSQLServer}},
		&MongoConnector{Config: ConnectionConfig{Type: SourceTypeMongoDB}},
		&CSVConnector{Config: ConnectionConfig{Type: SourceTypeCSV}},
		&ExcelConnector{Config: ConnectionConfig{Type: SourceTypeExcel}},
		&ManualConnector{Config: ConnectionConfig{Type: SourceTypeManual}},
	}

	for _, c := range connectors {
		require.NoError(t, c.Close(), "Close() on %T without prior connection", c)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	connectors := []DataConnector{
		&PostgresConnector{Config: ConnectionConfig{Type: SourceTypePostgres}},
		&SQLServerConnector{Config: ConnectionConfig{Type: SourceTypeSQLServer}},
		&MongoConnector{Config: ConnectionConfig{Type: SourceTypeMongoDB}},
		&CSVConnector{Config: ConnectionConfig{Type: SourceTypeCSV}},
		&ExcelConnector{Config: ConnectionConfig{Type: SourceTypeExcel}},
		&ManualConnector{Config: ConnectionConfig{Type: SourceTypeManual}},
	}

	for _, c := range connectors {
		require.NoError(t, c.Close(), "first Close() on %T", c)
		require.NoError(t, c.Close(), "second Close() on %T", c)
	}
}

func TestPostgresConnectorCloseReleasesServerConnection(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	u, err := url.Parse(dsn)
	require.NoError(t, err, "failed to parse TEST_DATABASE_URL")

	host := u.Hostname()
	portStr := u.Port()
	port, _ := strconv.Atoi(portStr)
	username := u.User.Username()
	password, _ := u.User.Password()
	database := strings.TrimPrefix(u.Path, "/")

	cfg := ConnectionConfig{
		Type:         SourceTypePostgres,
		Host:         host,
		Port:         port,
		Username:     username,
		Password:     password,
		Database:     database,
		TableOrQuery: "em_close_probe",
		ExtraParams:  map[string]interface{}{"sslmode": "disable"},
	}

	observerPool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "failed to create observer pool")
	defer observerPool.Close()

	_, err = observerPool.Exec(ctx, "CREATE TABLE IF NOT EXISTS em_close_probe (id int primary key, name text)")
	require.NoError(t, err, "failed to create probe table")
	defer func() {
		_, _ = observerPool.Exec(context.Background(), "DROP TABLE IF EXISTS em_close_probe")
	}()

	// countConns counts only this connector's own backends (tagged via
	// application_name in buildDSN). Counting every backend on the database
	// would race against other packages' tests, which connect to the same
	// database concurrently under `go test ./...`.
	countConns := func() int {
		var n int
		err := observerPool.QueryRow(ctx,
			`SELECT count(*) FROM pg_stat_activity
			 WHERE datname = current_database()
			   AND pid <> pg_backend_pid()
			   AND application_name = $1`,
			connectorApplicationName,
		).Scan(&n)
		require.NoError(t, err, "failed to count connections")
		return n
	}

	before := countConns()

	conn, err := NewDataConnector(cfg)
	require.NoError(t, err, "NewDataConnector failed")

	err = conn.TestConnection(ctx)
	require.NoError(t, err, "TestConnection failed")

	_, err = conn.FetchRecords(ctx, 10, 0)
	require.NoError(t, err, "FetchRecords failed")

	during := countConns()
	require.Greater(t, during, before, "expected an additional server connection after TestConnection+FetchRecords")

	err = conn.Close()
	require.NoError(t, err, "Close failed")

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if countConns() <= before {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	finalCount := countConns()
	require.LessOrEqual(t, finalCount, before,
		fmt.Sprintf("connector still holds %d server connection(s) after Close (before=%d, after=%d)", finalCount, before, finalCount))
}
