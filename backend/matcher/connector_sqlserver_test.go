package matcher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitQualifiedName(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		defaultSchema  string
		expectedSchema string
		expectedTable  string
		wantErr        bool
	}{
		{
			name:           "simple table",
			input:          "Customers",
			defaultSchema:  "dbo",
			expectedSchema: "dbo",
			expectedTable:  "Customers",
			wantErr:        false,
		},
		{
			name:           "qualified name",
			input:          "dbo.Customers",
			defaultSchema:  "dbo",
			expectedSchema: "dbo",
			expectedTable:  "Customers",
			wantErr:        false,
		},
		{
			name:           "sales orders",
			input:          "sales.Orders",
			defaultSchema:  "dbo",
			expectedSchema: "sales",
			expectedTable:  "Orders",
			wantErr:        false,
		},
		{
			name:           "whitespace trimmed",
			input:          "  sales . Orders  ",
			defaultSchema:  "dbo",
			expectedSchema: "sales",
			expectedTable:  "Orders",
			wantErr:        false,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "too many dots",
			input:   "a.b.c",
			wantErr: true,
		},
		{
			name:    "invalid identifier with special chars",
			input:   "sales.Orders; DROP TABLE x",
			wantErr: true,
		},
		{
			name:    "starts with number",
			input:   "1bad",
			wantErr: true,
		},
		{
			name:    "schema empty",
			input:   "dbo.",
			wantErr: true,
		},
		{
			name:    "table empty",
			input:   ".Customers",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, table, err := splitQualifiedName(tt.input, tt.defaultSchema)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedSchema, schema)
				require.Equal(t, tt.expectedTable, table)
			}
		})
	}
}

func TestQuoteMSSQLIdentifier(t *testing.T) {
	require.Equal(t, "[Customers]", quoteMSSQLIdentifier("Customers"))
	require.Equal(t, "[Order]", quoteMSSQLIdentifier("Order"))
	require.Equal(t, "[Weird]]Name]", quoteMSSQLIdentifier("Weird]Name"))
}

func TestSQLServerConnectorRejectsUnqualifiableNames(t *testing.T) {
	connector := &SQLServerConnector{Config: ConnectionConfig{Type: SourceTypeSQLServer, TableOrQuery: "a.b.c"}}
	ctx := context.Background()

	_, err := connector.IntrospectSchema(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid qualified name")

	_, err = connector.FetchRecords(ctx, 10, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid qualified name")
}

func TestSQLServerIntrospectQueryFiltersBySchema(t *testing.T) {
	// Without the TABLE_SCHEMA predicate, two schemas holding a same-named table
	// would return merged/interleaved column sets from both schemas.
	require.Contains(t, sqlServerIntrospectColumnsQuery, "TABLE_SCHEMA = @TableSchema")
	require.Contains(t, sqlServerIntrospectColumnsQuery, "TABLE_NAME = @TableName")
	require.Contains(t, sqlServerIntrospectColumnsQuery, "ORDER BY ORDINAL_POSITION")
}

func TestSQLServerFetchQueryIsSchemaQualified(t *testing.T) {
	tests := []struct {
		name     string
		schema   string
		table    string
		orderBy  string
		expected string
	}{
		{
			name:     "simple case",
			schema:   "dbo",
			table:    "Customers",
			orderBy:  "[id]",
			expected: `SELECT * FROM [dbo].[Customers] ORDER BY [id] OFFSET @Offset ROWS FETCH NEXT @Limit ROWS ONLY`,
		},
		{
			name:    "sales orders",
			schema:  "sales",
			table:   "Orders",
			orderBy: "[id]",
		},
		{
			name:    "reserved word",
			schema:  "dbo",
			table:   "Order",
			orderBy: "[id]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := sqlServerFetchQuery(tt.schema, tt.table, tt.orderBy)
			if tt.expected != "" {
				require.Equal(t, tt.expected, q)
				require.NotContains(t, q, "FROM dbo.Customers")
			} else {
				require.Contains(t, q, "["+tt.schema+"].["+tt.table+"]")
			}
		})
	}
}

// TestSQLServerFetchQueryHasRealOrdering guards against a regression back to
// ORDER BY (SELECT NULL), which parses but orders nothing, so OFFSET/FETCH
// paging could duplicate and drop rows.
func TestSQLServerFetchQueryHasRealOrdering(t *testing.T) {
	q := sqlServerFetchQuery("dbo", "Customers", "[id]")
	require.NotContains(t, q, "(SELECT NULL)")
	require.Contains(t, q, "ORDER BY [id] OFFSET")
}
