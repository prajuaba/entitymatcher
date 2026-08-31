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

func TestPostgresPagingIsStableUnderConcurrentUpdate(t *testing.T) {
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
		TableOrQuery: "em_page_pk",
		ExtraParams:  map[string]interface{}{"sslmode": "disable"},
	}

	observerPool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "failed to create observer pool")
	defer observerPool.Close()

	_, err = observerPool.Exec(ctx, "CREATE TABLE IF NOT EXISTS em_page_pk (id int primary key, name text)")
	require.NoError(t, err, "failed to create probe table")
	_, err = observerPool.Exec(ctx, "TRUNCATE em_page_pk")
	require.NoError(t, err, "failed to truncate probe table")
	defer func() {
		_, _ = observerPool.Exec(context.Background(), "DROP TABLE IF EXISTS em_page_pk")
	}()

	// Insert ids 1..40
	for i := 1; i <= 40; i++ {
		_, err = observerPool.Exec(ctx, "INSERT INTO em_page_pk (id, name) VALUES ($1, $2)", i, fmt.Sprintf("name-%d", i))
		require.NoError(t, err, "failed to insert row %d", i)
	}

	conn, err := NewDataConnector(cfg)
	require.NoError(t, err, "NewDataConnector failed")

	err = conn.TestConnection(ctx)
	require.NoError(t, err, "TestConnection failed")

	// Fetch page 1
	page1, err := conn.FetchRecords(ctx, 10, 0)
	require.NoError(t, err, "FetchRecords page 1 failed")

	var ids1 []int64
	for _, row := range page1 {
		ids1 = append(ids1, toInt64(t, row["id"]))
	}

	// Force heap movement by updating first 10 rows
	_, err = observerPool.Exec(ctx, "UPDATE em_page_pk SET name = name || 'x' WHERE id <= 10")
	require.NoError(t, err, "failed to update rows")

	// Fetch pages 2, 3, 4
	page2, err := conn.FetchRecords(ctx, 10, 10)
	require.NoError(t, err, "FetchRecords page 2 failed")
	page3, err := conn.FetchRecords(ctx, 10, 20)
	require.NoError(t, err, "FetchRecords page 3 failed")
	page4, err := conn.FetchRecords(ctx, 10, 30)
	require.NoError(t, err, "FetchRecords page 4 failed")

	// Collect all IDs
	allIDs := make(map[int64]int)
	for _, row := range page1 {
		id := toInt64(t, row["id"])
		allIDs[id]++
	}
	for _, row := range page2 {
		id := toInt64(t, row["id"])
		allIDs[id]++
	}
	for _, row := range page3 {
		id := toInt64(t, row["id"])
		allIDs[id]++
	}
	for _, row := range page4 {
		id := toInt64(t, row["id"])
		allIDs[id]++
	}

	// Verify no duplicates or missing IDs
	require.Len(t, allIDs, 40, "expected exactly 40 distinct IDs, got %d", len(allIDs))

	for i := 1; i <= 40; i++ {
		_, exists := allIDs[int64(i)]
		require.True(t, exists, "ID %d is missing", i)
		require.Equal(t, 1, allIDs[int64(i)], "ID %d appears %d times, expected 1", i, allIDs[int64(i)])
	}

	err = conn.Close()
	require.NoError(t, err, "Close failed")
}

func TestPostgresPagingWithoutPrimaryKey(t *testing.T) {
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
		TableOrQuery: "em_page_nopk",
		ExtraParams:  map[string]interface{}{"sslmode": "disable"},
	}

	observerPool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "failed to create observer pool")
	defer observerPool.Close()

	_, err = observerPool.Exec(ctx, "CREATE TABLE IF NOT EXISTS em_page_nopk (name text, city text, payload json)")
	require.NoError(t, err, "failed to create probe table")
	_, err = observerPool.Exec(ctx, "TRUNCATE em_page_nopk")
	require.NoError(t, err, "failed to truncate probe table")
	defer func() {
		_, _ = observerPool.Exec(context.Background(), "DROP TABLE IF EXISTS em_page_nopk")
	}()

	// Insert 30 distinct rows
	for i := 0; i < 30; i++ {
		_, err = observerPool.Exec(ctx, "INSERT INTO em_page_nopk (name, city, payload) VALUES ($1, $2, $3::json)",
			fmt.Sprintf("person-%d", i), fmt.Sprintf("city-%d", i), "{}")
		require.NoError(t, err, "failed to insert row %d", i)
	}

	conn, err := NewDataConnector(cfg)
	require.NoError(t, err, "NewDataConnector failed")

	err = conn.TestConnection(ctx)
	require.NoError(t, err, "TestConnection failed")

	// Fetch records in chunks
	page1, err := conn.FetchRecords(ctx, 10, 0)
	require.NoError(t, err, "FetchRecords page 1 failed")
	page2, err := conn.FetchRecords(ctx, 10, 10)
	require.NoError(t, err, "FetchRecords page 2 failed")
	page3, err := conn.FetchRecords(ctx, 10, 20)
	require.NoError(t, err, "FetchRecords page 3 failed")

	// Collect all names
	nameSet := make(map[string]bool)
	for _, row := range page1 {
		nameSet[row["name"].(string)] = true
	}
	for _, row := range page2 {
		nameSet[row["name"].(string)] = true
	}
	for _, row := range page3 {
		nameSet[row["name"].(string)] = true
	}

	require.Len(t, nameSet, 30, "expected exactly 30 distinct names")

	err = conn.Close()
	require.NoError(t, err, "Close failed")
}

func TestPostgresResolveOrderByPrefersPrimaryKey(t *testing.T) {
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
		TableOrQuery: "em_page_composite",
		ExtraParams:  map[string]interface{}{"sslmode": "disable"},
	}

	observerPool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "failed to create observer pool")
	defer observerPool.Close()

	_, err = observerPool.Exec(ctx, "DROP TABLE IF EXISTS em_page_composite")
	require.NoError(t, err, "failed to drop probe table")
	_, err = observerPool.Exec(ctx, "CREATE TABLE em_page_composite (a int, b int, PRIMARY KEY (b, a))")
	require.NoError(t, err, "failed to create probe table")
	defer func() {
		_, _ = observerPool.Exec(context.Background(), "DROP TABLE IF EXISTS em_page_composite")
	}()

	connector := &PostgresConnector{Config: cfg}
	err = connector.TestConnection(ctx)
	require.NoError(t, err, "TestConnection failed")
	defer connector.Close()

	result, err := connector.resolveOrderBy(ctx, "public", "em_page_composite")
	require.NoError(t, err)
	require.Equal(t, "b, a", result)
}

func TestPostgresResolveOrderByHonoursExplicitOverride(t *testing.T) {
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

	cfg1 := ConnectionConfig{
		Type:         SourceTypePostgres,
		Host:         host,
		Port:         port,
		Username:     username,
		Password:     password,
		Database:     database,
		TableOrQuery: "em_page_pk",
		ExtraParams:  map[string]interface{}{"sslmode": "disable", "order_by": "name, id"},
	}

	observerPool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "failed to create observer pool")
	defer observerPool.Close()

	_, err = observerPool.Exec(ctx, "CREATE TABLE IF NOT EXISTS em_page_pk (id int primary key, name text)")
	require.NoError(t, err, "failed to create probe table")
	_, err = observerPool.Exec(ctx, "TRUNCATE em_page_pk")
	require.NoError(t, err, "failed to truncate probe table")
	defer func() {
		_, _ = observerPool.Exec(context.Background(), "DROP TABLE IF EXISTS em_page_pk")
	}()

	connector1 := &PostgresConnector{Config: cfg1}
	err = connector1.TestConnection(ctx)
	require.NoError(t, err, "TestConnection failed")
	defer connector1.Close()

	result, err := connector1.resolveOrderBy(ctx, "public", "em_page_pk")
	require.NoError(t, err)
	require.Equal(t, "name, id", result)

	// Test invalid order_by column
	cfg2 := ConnectionConfig{
		Type:         SourceTypePostgres,
		Host:         host,
		Port:         port,
		Username:     username,
		Password:     password,
		Database:     database,
		TableOrQuery: "em_page_pk",
		ExtraParams:  map[string]interface{}{"sslmode": "disable", "order_by": "id; DROP TABLE x"},
	}

	connector2 := &PostgresConnector{Config: cfg2}
	err = connector2.TestConnection(ctx)
	require.NoError(t, err, "TestConnection failed")
	defer connector2.Close()

	result, err = connector2.resolveOrderBy(ctx, "public", "em_page_pk")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid order_by column")
}

func TestPostgresPagingRefusesUnorderableTable(t *testing.T) {
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
		TableOrQuery: "em_page_json",
		ExtraParams:  map[string]interface{}{"sslmode": "disable"},
	}

	observerPool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "failed to create observer pool")
	defer observerPool.Close()

	_, err = observerPool.Exec(ctx, "DROP TABLE IF EXISTS em_page_json")
	require.NoError(t, err, "failed to drop probe table")
	_, err = observerPool.Exec(ctx, "CREATE TABLE em_page_json (payload json)")
	require.NoError(t, err, "failed to create probe table")
	defer func() {
		_, _ = observerPool.Exec(context.Background(), "DROP TABLE IF EXISTS em_page_json")
	}()

	connector := &PostgresConnector{Config: cfg}
	err = connector.TestConnection(ctx)
	require.NoError(t, err, "TestConnection failed")
	defer connector.Close()

	_, err = connector.FetchRecords(ctx, 10, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "set extra_params.order_by")
}

func toInt64(t *testing.T, v interface{}) int64 {
	switch i := v.(type) {
	case int:
		return int64(i)
	case int32:
		return int64(i)
	case int64:
		return i
	default:
		t.Fatalf("unexpected type for ID: %T", v)
		return 0
	}
}
