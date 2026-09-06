package matcher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostgresRejectsQueryDatasource(t *testing.T) {
	ctx := context.Background()

	t.Run("SELECT query", func(t *testing.T) {
		conn := &PostgresConnector{Config: ConnectionConfig{Type: SourceTypePostgres, TableOrQuery: "SELECT id FROM users"}}
		_, err := conn.IntrospectSchema(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "query datasources are not supported")

		_, err = conn.FetchRecords(ctx, 10, 0)
		require.Error(t, err)
		require.Contains(t, err.Error(), "query datasources are not supported")
	})

	t.Run("lowercase select with whitespace", func(t *testing.T) {
		conn := &PostgresConnector{Config: ConnectionConfig{Type: SourceTypePostgres, TableOrQuery: "  select * from t  "}}
		_, err := conn.IntrospectSchema(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "query datasources are not supported")

		_, err = conn.FetchRecords(ctx, 10, 0)
		require.Error(t, err)
		require.Contains(t, err.Error(), "query datasources are not supported")
	})

	t.Run("WITH query", func(t *testing.T) {
		conn := &PostgresConnector{Config: ConnectionConfig{Type: SourceTypePostgres, TableOrQuery: "WITH x AS (SELECT 1) SELECT * FROM x"}}
		_, err := conn.IntrospectSchema(ctx)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "query datasources are not supported")

		_, err = conn.FetchRecords(ctx, 10, 0)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "query datasources are not supported")
	})

	t.Run("valid table name", func(t *testing.T) {
		conn := &PostgresConnector{Config: ConnectionConfig{Type: SourceTypePostgres, TableOrQuery: "customers"}}
		_, err := conn.IntrospectSchema(ctx)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "query datasources are not supported")

		_, err = conn.FetchRecords(ctx, 10, 0)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "query datasources are not supported")
	})
}
