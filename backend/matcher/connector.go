package matcher

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/microsoft/go-mssqldb"
	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SourceType string

const (
	SourceTypePostgres  SourceType = "POSTGRES"
	SourceTypeSQLServer SourceType = "SQLSERVER"
	SourceTypeMongoDB   SourceType = "MONGODB"
	SourceTypeCSV       SourceType = "CSV"
	SourceTypeExcel     SourceType = "EXCEL"
	SourceTypeManual    SourceType = "MANUAL"
)

type ConnectionConfig struct {
	Type         SourceType               `json:"type"`
	Host         string                   `json:"host,omitempty"`
	Port         int                      `json:"port,omitempty"`
	Database     string                   `json:"database,omitempty"`
	Username     string                   `json:"username,omitempty"`
	Password     string                   `json:"password,omitempty"`
	TableOrQuery string                   `json:"table_or_query"`
	FilePath     string                   `json:"file_path,omitempty"`
	ManualData   []map[string]interface{} `json:"manual_data,omitempty"`
	ExtraParams  map[string]interface{}   `json:"extra_params,omitempty"`
}

type ColumnDef struct {
	Name     string `json:"name"`
	DataType string `json:"data_type"`
}

type DataConnector interface {
	TestConnection(ctx context.Context) error
	IntrospectSchema(ctx context.Context) ([]ColumnDef, error)
	FetchRecords(ctx context.Context, limit, offset int) ([]map[string]interface{}, error)
	Close() error
}

func NewDataConnector(cfg ConnectionConfig) (DataConnector, error) {
	switch cfg.Type {
	case SourceTypePostgres:
		return &PostgresConnector{Config: cfg}, nil
	case SourceTypeSQLServer:
		return &SQLServerConnector{Config: cfg}, nil
	case SourceTypeMongoDB:
		return &MongoConnector{Config: cfg}, nil
	case SourceTypeCSV:
		return &CSVConnector{Config: cfg}, nil
	case SourceTypeExcel:
		return &ExcelConnector{Config: cfg}, nil
	case SourceTypeManual:
		return &ManualConnector{Config: cfg}, nil
	default:
		return &ManualConnector{Config: cfg}, nil
	}
}

type PostgresConnector struct {
	Config  ConnectionConfig
	pool    *pgxpool.Pool
	orderBy string // cached ORDER BY list; resolved once, reused across pages
}

func (c *PostgresConnector) buildDSN() string {
	port := c.Config.Port
	if port == 0 {
		port = 5432
	}
	sslmode := "disable"
	if c.Config.ExtraParams != nil {
		if ssl, ok := c.Config.ExtraParams["sslmode"].(string); ok {
			sslmode = ssl
		}
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.Config.Username, c.Config.Password, c.Config.Host, port, c.Config.Database, sslmode)
}

func (c *PostgresConnector) TestConnection(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(c.buildDSN())
	if err != nil {
		return fmt.Errorf("postgres config error: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("postgres connection failed: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("postgres ping failed: %w", err)
	}

	c.pool = pool
	return nil
}

func (c *PostgresConnector) IntrospectSchema(ctx context.Context) ([]ColumnDef, error) {
	if c.pool == nil {
		if err := c.TestConnection(ctx); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	schema, table := "public", c.Config.TableOrQuery
	parts := strings.Split(c.Config.TableOrQuery, ".")
	if len(parts) == 2 {
		schema = parts[0]
		table = parts[1]
	}

	if err := validateIdentifier(schema); err != nil {
		return nil, err
	}
	if err := validateIdentifier(table); err != nil {
		return nil, err
	}

	if strings.HasPrefix(strings.TrimSpace(c.Config.TableOrQuery), "SELECT") {
		query := c.Config.TableOrQuery + " LIMIT 0"
		rows, err := c.pool.Query(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("query execution failed: %w", err)
		}
		defer rows.Close()

		cols := rows.FieldDescriptions()
		result := make([]ColumnDef, len(cols))
		for i, col := range cols {
			result[i] = ColumnDef{Name: string(col.Name), DataType: fmt.Sprintf("OID:%d", col.DataTypeOID)}
		}
		return result, nil
	}

	query := `
		SELECT column_name, data_type
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position
	`

	rows, err := c.pool.Query(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("introspect schema failed: %w", err)
	}
	defer rows.Close()

	var result []ColumnDef
	for rows.Next() {
		var colName, dataType string
		if err := rows.Scan(&colName, &dataType); err != nil {
			return nil, err
		}
		result = append(result, ColumnDef{Name: colName, DataType: dataType})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// explicitOrderBy reads and validates extra_params.order_by, returning "" when
// the operator supplied none. Each column is validated as an identifier, so
// this cannot become an injection point.
func explicitOrderBy(extra map[string]interface{}, quote func(string) string) (string, error) {
	if extra == nil {
		return "", nil
	}
	raw, ok := extra["order_by"].(string)
	if !ok || strings.TrimSpace(raw) == "" {
		return "", nil
	}
	var cols []string
	for _, part := range strings.Split(raw, ",") {
		col := strings.TrimSpace(part)
		if err := validateIdentifier(col); err != nil {
			return "", fmt.Errorf("invalid order_by column: %w", err)
		}
		cols = append(cols, quote(col))
	}
	return strings.Join(cols, ", "), nil
}

// resolveOrderBy determines a total ordering so LIMIT/OFFSET paging cannot
// duplicate or drop rows. Preference: an explicit extra_params.order_by, then
// the primary key, then every orderable column. A table with none of these
// cannot be paged safely, and that is reported rather than hidden.
func (c *PostgresConnector) resolveOrderBy(ctx context.Context, schema, table string) (string, error) {
	if c.orderBy != "" {
		return c.orderBy, nil
	}

	if explicit, err := explicitOrderBy(c.Config.ExtraParams, quoteIdentifier); err != nil {
		return "", err
	} else if explicit != "" {
		c.orderBy = explicit
		return c.orderBy, nil
	}

	qualified := quoteIdentifier(schema) + "." + quoteIdentifier(table)

	pkQuery := `
		SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		WHERE i.indrelid = $1::regclass AND i.indisprimary
		ORDER BY array_position(i.indkey, a.attnum)
	`
	rows, err := c.pool.Query(ctx, pkQuery, qualified)
	if err != nil {
		return "", fmt.Errorf("resolve primary key for %s.%s: %w", schema, table, err)
	}
	var pkCols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return "", fmt.Errorf("resolve primary key for %s.%s: %w", schema, table, err)
		}
		pkCols = append(pkCols, name)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", fmt.Errorf("resolve primary key for %s.%s: %w", schema, table, err)
	}
	rows.Close()

	if len(pkCols) > 0 {
		quoted := make([]string, len(pkCols))
		for i, col := range pkCols {
			quoted[i] = quoteIdentifier(col)
		}
		c.orderBy = strings.Join(quoted, ", ")
		return c.orderBy, nil
	}

	orderableQuery := `
		SELECT a.attname
		FROM pg_attribute a
		WHERE a.attrelid = $1::regclass AND a.attnum > 0 AND NOT a.attisdropped
		  AND EXISTS (SELECT 1 FROM pg_opclass o JOIN pg_am m ON m.oid = o.opcmethod
		              WHERE o.opcintype = a.atttypid AND m.amname = 'btree' AND o.opcdefault)
		ORDER BY a.attnum
	`
	rows2, err := c.pool.Query(ctx, orderableQuery, qualified)
	if err != nil {
		return "", fmt.Errorf("resolve orderable columns for %s.%s: %w", schema, table, err)
	}
	var orderableCols []string
	for rows2.Next() {
		var name string
		if err := rows2.Scan(&name); err != nil {
			rows2.Close()
			return "", fmt.Errorf("resolve orderable columns for %s.%s: %w", schema, table, err)
		}
		orderableCols = append(orderableCols, name)
	}
	if err := rows2.Err(); err != nil {
		rows2.Close()
		return "", fmt.Errorf("resolve orderable columns for %s.%s: %w", schema, table, err)
	}
	rows2.Close()

	if len(orderableCols) > 0 {
		quoted := make([]string, len(orderableCols))
		for i, col := range orderableCols {
			quoted[i] = quoteIdentifier(col)
		}
		c.orderBy = strings.Join(quoted, ", ")
		return c.orderBy, nil
	}

	return "", fmt.Errorf("cannot page %s.%s deterministically: it has no primary key and no orderable columns; set extra_params.order_by", schema, table)
}

// resolveQueryOrderBy orders a raw-SELECT datasource. There is no catalog to
// consult, so it falls back to every orderable output column by ordinal.
func (c *PostgresConnector) resolveQueryOrderBy(ctx context.Context) (string, error) {
	if c.orderBy != "" {
		return c.orderBy, nil
	}

	if explicit, err := explicitOrderBy(c.Config.ExtraParams, quoteIdentifier); err != nil {
		return "", err
	} else if explicit != "" {
		c.orderBy = explicit
		return c.orderBy, nil
	}

	probe := fmt.Sprintf("SELECT * FROM (%s) AS _em_page LIMIT 0", c.Config.TableOrQuery)
	rows, err := c.pool.Query(ctx, probe)
	if err != nil {
		return "", fmt.Errorf("probe query shape: %w", err)
	}
	fields := rows.FieldDescriptions()
	oids := make([]uint32, len(fields))
	for i, f := range fields {
		oids[i] = f.DataTypeOID
	}
	rows.Close()

	orderableSet := make(map[uint32]bool)
	if len(oids) > 0 {
		catalogQuery := `
			SELECT o.opcintype
			FROM pg_opclass o JOIN pg_am m ON m.oid = o.opcmethod
			WHERE m.amname = 'btree' AND o.opcdefault AND o.opcintype = ANY($1)
		`
		catRows, err := c.pool.Query(ctx, catalogQuery, oids)
		if err != nil {
			return "", fmt.Errorf("resolve orderable output columns: %w", err)
		}
		for catRows.Next() {
			var oid uint32
			if err := catRows.Scan(&oid); err != nil {
				catRows.Close()
				return "", fmt.Errorf("resolve orderable output columns: %w", err)
			}
			orderableSet[oid] = true
		}
		if err := catRows.Err(); err != nil {
			catRows.Close()
			return "", fmt.Errorf("resolve orderable output columns: %w", err)
		}
		catRows.Close()
	}

	var ordinals []string
	for i, f := range fields {
		if orderableSet[f.DataTypeOID] {
			ordinals = append(ordinals, strconv.Itoa(i+1))
		}
	}

	if len(ordinals) == 0 {
		return "", fmt.Errorf("cannot page this query deterministically: no orderable output columns; set extra_params.order_by")
	}

	c.orderBy = strings.Join(ordinals, ", ")
	return c.orderBy, nil
}

func (c *PostgresConnector) FetchRecords(ctx context.Context, limit, offset int) ([]map[string]interface{}, error) {
	if c.pool == nil {
		if err := c.TestConnection(ctx); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	limit = clamp(limit, 1, 50000)
	if offset < 0 {
		offset = 0
	}

	schema, table := "public", c.Config.TableOrQuery
	parts := strings.Split(c.Config.TableOrQuery, ".")
	if len(parts) == 2 {
		schema = parts[0]
		table = parts[1]
	}

	if err := validateIdentifier(schema); err != nil {
		return nil, err
	}
	if err := validateIdentifier(table); err != nil {
		return nil, err
	}

	var rows pgx.Rows
	var err error

	if strings.HasPrefix(strings.TrimSpace(c.Config.TableOrQuery), "SELECT") {
		orderBy, oerr := c.resolveQueryOrderBy(ctx)
		if oerr != nil {
			return nil, oerr
		}
		query := fmt.Sprintf("SELECT * FROM (%s) AS _em_page ORDER BY %s LIMIT $1 OFFSET $2",
			c.Config.TableOrQuery, orderBy)
		rows, err = c.pool.Query(ctx, query, limit, offset)
	} else {
		orderBy, oerr := c.resolveOrderBy(ctx, schema, table)
		if oerr != nil {
			return nil, oerr
		}
		query := fmt.Sprintf("SELECT * FROM %s.%s ORDER BY %s LIMIT $1 OFFSET $2",
			quoteIdentifier(schema), quoteIdentifier(table), orderBy)
		rows, err = c.pool.Query(ctx, query, limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("fetch records failed: %w", err)
	}
	defer rows.Close()

	cols := rows.FieldDescriptions()
	result := make([]map[string]interface{}, 0)

	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(values))
		for i := range values {
			var v interface{}
			ptrs[i] = &v
		}

		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range cols {
			row[string(col.Name)] = *ptrs[i].(*interface{})
		}
		result = append(result, row)
	}

	return result, rows.Err()
}

func (c *PostgresConnector) Close() error {
	if c.pool != nil {
		c.pool.Close()
		c.pool = nil
	}
	return nil
}

type SQLServerConnector struct {
	Config  ConnectionConfig
	conn    *sql.DB
	orderBy string // cached ORDER BY list; resolved once, reused across pages
}

func (c *SQLServerConnector) buildDSN() string {
	port := c.Config.Port
	if port == 0 {
		port = 1433
	}
	return fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
		c.Config.Username, c.Config.Password, c.Config.Host, port, c.Config.Database)
}

func (c *SQLServerConnector) TestConnection(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := sql.Open("sqlserver", c.buildDSN())
	if err != nil {
		return fmt.Errorf("sqlserver open failed: %w", err)
	}

	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return fmt.Errorf("sqlserver connection failed: %w", err)
	}

	c.conn = conn
	return nil
}

// splitQualifiedName splits an optionally schema-qualified table name into its
// schema and table halves, validating each. An unqualified name takes
// defaultSchema. Anything with more than one dot is rejected rather than
// guessed at.
func splitQualifiedName(name, defaultSchema string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", fmt.Errorf("table name is required")
	}

	parts := strings.Split(name, ".")
	var schema, table string
	switch len(parts) {
	case 1:
		schema = defaultSchema
		table = parts[0]
	case 2:
		schema = parts[0]
		table = parts[1]
	default:
		return "", "", fmt.Errorf("invalid qualified name: %s", name)
	}

	schema = strings.TrimSpace(schema)
	table = strings.TrimSpace(table)

	if err := validateIdentifier(schema); err != nil {
		return "", "", err
	}
	if err := validateIdentifier(table); err != nil {
		return "", "", err
	}

	return schema, table, nil
}

// sqlServerIntrospectColumnsQuery must filter on both TABLE_SCHEMA and
// TABLE_NAME: two schemas holding a same-named table would otherwise return
// both column sets, merged and interleaved by ORDINAL_POSITION.
const sqlServerIntrospectColumnsQuery = `
		SELECT COLUMN_NAME, DATA_TYPE
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = @TableSchema AND TABLE_NAME = @TableName
		ORDER BY ORDINAL_POSITION
	`

func (c *SQLServerConnector) IntrospectSchema(ctx context.Context) ([]ColumnDef, error) {
	schema, table, err := splitQualifiedName(c.Config.TableOrQuery, "dbo")
	if err != nil {
		return nil, err
	}

	if c.conn == nil {
		if err := c.TestConnection(ctx); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := c.conn.QueryContext(ctx, sqlServerIntrospectColumnsQuery,
		sql.Named("TableSchema", schema),
		sql.Named("TableName", table))
	if err != nil {
		return nil, fmt.Errorf("introspect schema failed: %w", err)
	}
	defer rows.Close()

	var cols []ColumnDef
	for rows.Next() {
		var name, dtype string
		if err := rows.Scan(&name, &dtype); err != nil {
			return nil, err
		}
		cols = append(cols, ColumnDef{Name: name, DataType: dtype})
	}

	return cols, rows.Err()
}

// quoteMSSQLIdentifier bracket-quotes an identifier. Brackets, unlike bare
// names, survive reserved words such as a table literally called Order.
func quoteMSSQLIdentifier(id string) string {
	return "[" + strings.ReplaceAll(id, "]", "]]") + "]"
}

// resolveOrderBy determines a total ordering so OFFSET/FETCH paging cannot
// duplicate or drop rows. Preference: an explicit extra_params.order_by, then
// the primary key, then every orderable column. ORDER BY (SELECT NULL) parses
// but orders nothing, which is what this replaces.
func (c *SQLServerConnector) resolveOrderBy(ctx context.Context, schema, table string) (string, error) {
	if c.orderBy != "" {
		return c.orderBy, nil
	}

	if explicit, err := explicitOrderBy(c.Config.ExtraParams, quoteMSSQLIdentifier); err != nil {
		return "", err
	} else if explicit != "" {
		c.orderBy = explicit
		return c.orderBy, nil
	}

	qualified := schema + "." + table

	pkQuery := `
		SELECT c.name
		FROM sys.indexes i
		JOIN sys.index_columns ic ON ic.object_id = i.object_id AND ic.index_id = i.index_id
		JOIN sys.columns c ON c.object_id = ic.object_id AND c.column_id = ic.column_id
		WHERE i.is_primary_key = 1 AND i.object_id = OBJECT_ID(@Table)
		ORDER BY ic.key_ordinal
	`
	rows, err := c.conn.QueryContext(ctx, pkQuery, sql.Named("Table", qualified))
	if err != nil {
		return "", fmt.Errorf("resolve primary key for %s: %w", qualified, err)
	}
	var pkCols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return "", fmt.Errorf("resolve primary key for %s: %w", qualified, err)
		}
		pkCols = append(pkCols, name)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", fmt.Errorf("resolve primary key for %s: %w", qualified, err)
	}
	rows.Close()

	if len(pkCols) > 0 {
		quoted := make([]string, len(pkCols))
		for i, col := range pkCols {
			quoted[i] = quoteMSSQLIdentifier(col)
		}
		c.orderBy = strings.Join(quoted, ", ")
		return c.orderBy, nil
	}

	orderableQuery := `
		SELECT c.name
		FROM sys.columns c
		JOIN sys.types t ON t.user_type_id = c.user_type_id
		WHERE c.object_id = OBJECT_ID(@Table)
		  AND t.name NOT IN ('text', 'ntext', 'image', 'xml', 'geography', 'geometry')
		ORDER BY c.column_id
	`
	rows2, err := c.conn.QueryContext(ctx, orderableQuery, sql.Named("Table", qualified))
	if err != nil {
		return "", fmt.Errorf("resolve orderable columns for %s: %w", qualified, err)
	}
	var orderableCols []string
	for rows2.Next() {
		var name string
		if err := rows2.Scan(&name); err != nil {
			rows2.Close()
			return "", fmt.Errorf("resolve orderable columns for %s: %w", qualified, err)
		}
		orderableCols = append(orderableCols, name)
	}
	if err := rows2.Err(); err != nil {
		rows2.Close()
		return "", fmt.Errorf("resolve orderable columns for %s: %w", qualified, err)
	}
	rows2.Close()

	if len(orderableCols) > 0 {
		quoted := make([]string, len(orderableCols))
		for i, col := range orderableCols {
			quoted[i] = quoteMSSQLIdentifier(col)
		}
		c.orderBy = strings.Join(quoted, ", ")
		return c.orderBy, nil
	}

	return "", fmt.Errorf("cannot page %s deterministically: it has no primary key and no orderable columns; set extra_params.order_by", qualified)
}

// sqlServerFetchQuery builds the paged read for a schema-qualified table.
// orderBy must be a resolved, validated column list -- OFFSET/FETCH requires an
// ORDER BY, and (SELECT NULL) would satisfy the parser while ordering nothing.
func sqlServerFetchQuery(schema, table, orderBy string) string {
	qualified := quoteMSSQLIdentifier(schema) + "." + quoteMSSQLIdentifier(table)
	return fmt.Sprintf("SELECT * FROM %s ORDER BY %s OFFSET @Offset ROWS FETCH NEXT @Limit ROWS ONLY", qualified, orderBy)
}

func (c *SQLServerConnector) FetchRecords(ctx context.Context, limit, offset int) ([]map[string]interface{}, error) {
	schema, table, err := splitQualifiedName(c.Config.TableOrQuery, "dbo")
	if err != nil {
		return nil, err
	}

	if c.conn == nil {
		if err := c.TestConnection(ctx); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	limit = clamp(limit, 1, 50000)
	if offset < 0 {
		offset = 0
	}

	orderBy, err := c.resolveOrderBy(ctx, schema, table)
	if err != nil {
		return nil, err
	}

	query := sqlServerFetchQuery(schema, table, orderBy)

	rows, err := c.conn.QueryContext(ctx, query, sql.Named("Offset", offset), sql.Named("Limit", limit))
	if err != nil {
		return nil, fmt.Errorf("fetch records failed: %w", err)
	}
	defer rows.Close()

	return sqlRowsToMaps(rows)
}

func (c *SQLServerConnector) Close() error {
	if c.conn != nil {
		conn := c.conn
		c.conn = nil
		return conn.Close()
	}
	return nil
}

type MongoConnector struct {
	Config     ConnectionConfig
	client     *mongo.Client
	dbName     string
	collection string
}

func (c *MongoConnector) buildURI() string {
	port := c.Config.Port
	if port == 0 {
		port = 27017
	}

	if c.Config.Username == "" {
		return fmt.Sprintf("mongodb://%s:%d", c.Config.Host, port)
	}

	return fmt.Sprintf("mongodb://%s:%s@%s:%d", c.Config.Username, c.Config.Password, c.Config.Host, port)
}

func (c *MongoConnector) TestConnection(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	opts := options.Client().ApplyURI(c.buildURI())
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return fmt.Errorf("mongodb connection failed: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		client.Disconnect(ctx)
		return fmt.Errorf("mongodb ping failed: %w", err)
	}

	c.client = client
	c.dbName = c.Config.Database
	c.collection = c.Config.TableOrQuery

	return nil
}

func (c *MongoConnector) IntrospectSchema(ctx context.Context) ([]ColumnDef, error) {
	if c.client == nil {
		if err := c.TestConnection(ctx); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	coll := c.client.Database(c.dbName).Collection(c.collection)
	cursor, err := coll.Find(ctx, bson.D{}, options.Find().SetLimit(100))
	if err != nil {
		return nil, fmt.Errorf("introspect schema failed: %w", err)
	}
	defer cursor.Close(ctx)

	colSet := make(map[string]string)
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		for k, v := range doc {
			if _, ok := colSet[k]; !ok {
				colSet[k] = bsonTypeToString(v)
			}
		}
	}

	var cols []ColumnDef
	for k, v := range colSet {
		cols = append(cols, ColumnDef{Name: k, DataType: v})
	}

	return cols, cursor.Err()
}

func (c *MongoConnector) FetchRecords(ctx context.Context, limit, offset int) ([]map[string]interface{}, error) {
	if c.client == nil {
		if err := c.TestConnection(ctx); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	limit = clamp(limit, 1, 50000)
	if offset < 0 {
		offset = 0
	}

	coll := c.client.Database(c.dbName).Collection(c.collection)

	// MongoDB does not guarantee natural order across queries, so skip/limit
	// without a sort can duplicate and drop documents as the collection is
	// concurrently modified. _id is always present and unique, which makes it
	// a safe default; extra_params.order_by can override it.
	sortField := "_id"
	if c.Config.ExtraParams != nil {
		if raw, ok := c.Config.ExtraParams["order_by"].(string); ok && strings.TrimSpace(raw) != "" {
			sortField = strings.TrimSpace(raw)
		}
	}

	findOpts := options.Find().
		SetSort(bson.D{{Key: sortField, Value: 1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := coll.Find(ctx, bson.D{}, findOpts)
	if err != nil {
		return nil, fmt.Errorf("fetch records failed: %w", err)
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		doc = convertObjectIDsToStrings(doc)
		results = append(results, doc)
	}

	return results, cursor.Err()
}

func (c *MongoConnector) Close() error {
	if c.client != nil {
		client := c.client
		c.client = nil
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return client.Disconnect(ctx)
	}
	return nil
}

type CSVConnector struct {
	Config ConnectionConfig
}

func (c *CSVConnector) TestConnection(ctx context.Context) error {
	if c.Config.FilePath == "" && len(c.Config.ManualData) == 0 {
		return fmt.Errorf("CSV file path or content required")
	}
	if len(c.Config.ManualData) > 0 {
		return nil
	}
	f, err := os.Open(c.Config.FilePath)
	if err != nil {
		return fmt.Errorf("cannot open CSV file: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	_, err = r.Read()
	if err != nil {
		return fmt.Errorf("cannot parse CSV file: %w", err)
	}
	return nil
}

func (c *CSVConnector) IntrospectSchema(ctx context.Context) ([]ColumnDef, error) {
	if len(c.Config.ManualData) > 0 {
		var cols []ColumnDef
		for k := range c.Config.ManualData[0] {
			cols = append(cols, ColumnDef{Name: k, DataType: "STRING"})
		}
		return cols, nil
	}

	file, err := os.Open(c.Config.FilePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("cannot read CSV headers: %w", err)
	}

	var cols []ColumnDef
	for _, h := range headers {
		cols = append(cols, ColumnDef{Name: strings.TrimSpace(h), DataType: "STRING"})
	}
	return cols, nil
}

func (c *CSVConnector) FetchRecords(ctx context.Context, limit, offset int) ([]map[string]interface{}, error) {
	if len(c.Config.ManualData) > 0 {
		if limit <= 0 {
			limit = len(c.Config.ManualData)
		}
		if offset < 0 {
			offset = 0
		}
		if offset >= len(c.Config.ManualData) {
			return []map[string]interface{}{}, nil
		}
		end := offset + limit
		if end > len(c.Config.ManualData) {
			end = len(c.Config.ManualData)
		}
		return c.Config.ManualData[offset:end], nil
	}

	file, err := os.Open(c.Config.FilePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("cannot read CSV headers: %w", err)
	}

	limit = clamp(limit, 1, 50000)
	if offset < 0 {
		offset = 0
	}

	var rows []map[string]interface{}
	skipped := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if skipped < offset {
			skipped++
			continue
		}

		if len(rows) >= limit {
			break
		}

		row := make(map[string]interface{})
		for i, val := range record {
			if i < len(headers) {
				row[strings.TrimSpace(headers[i])] = val
			}
		}
		rows = append(rows, row)
	}

	return rows, nil
}

func (c *CSVConnector) Close() error {
	return nil
}

type ExcelConnector struct {
	Config ConnectionConfig
}

func (c *ExcelConnector) TestConnection(ctx context.Context) error {
	if c.Config.FilePath == "" && len(c.Config.ManualData) == 0 {
		return fmt.Errorf("Excel file path or content required")
	}
	if len(c.Config.ManualData) > 0 {
		return nil
	}
	f, err := excelize.OpenFile(c.Config.FilePath)
	if err != nil {
		return fmt.Errorf("cannot open Excel file: %w", err)
	}
	defer f.Close()
	return nil
}

func (c *ExcelConnector) IntrospectSchema(ctx context.Context) ([]ColumnDef, error) {
	if len(c.Config.ManualData) > 0 {
		var cols []ColumnDef
		for k := range c.Config.ManualData[0] {
			cols = append(cols, ColumnDef{Name: k, DataType: "STRING"})
		}
		return cols, nil
	}

	f, err := excelize.OpenFile(c.Config.FilePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open Excel file: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if c.Config.ExtraParams != nil {
		if sheet, ok := c.Config.ExtraParams["sheet"].(string); ok {
			sheetName = sheet
		}
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("cannot read Excel sheet: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("Excel sheet is empty")
	}

	var cols []ColumnDef
	for _, h := range rows[0] {
		cols = append(cols, ColumnDef{Name: strings.TrimSpace(h), DataType: "STRING"})
	}
	return cols, nil
}

func (c *ExcelConnector) FetchRecords(ctx context.Context, limit, offset int) ([]map[string]interface{}, error) {
	if len(c.Config.ManualData) > 0 {
		if limit <= 0 {
			limit = len(c.Config.ManualData)
		}
		if offset < 0 {
			offset = 0
		}
		if offset >= len(c.Config.ManualData) {
			return []map[string]interface{}{}, nil
		}
		end := offset + limit
		if end > len(c.Config.ManualData) {
			end = len(c.Config.ManualData)
		}
		return c.Config.ManualData[offset:end], nil
	}

	f, err := excelize.OpenFile(c.Config.FilePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open Excel file: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if c.Config.ExtraParams != nil {
		if sheet, ok := c.Config.ExtraParams["sheet"].(string); ok {
			sheetName = sheet
		}
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("cannot read Excel sheet: %w", err)
	}

	if len(rows) == 0 {
		return []map[string]interface{}{}, nil
	}

	headers := rows[0]
	limit = clamp(limit, 1, 50000)
	if offset < 0 {
		offset = 0
	}

	var result []map[string]interface{}
	for i := 1 + offset; i < len(rows) && len(result) < limit; i++ {
		row := make(map[string]interface{})
		for j, h := range headers {
			if j < len(rows[i]) {
				row[strings.TrimSpace(h)] = rows[i][j]
			}
		}
		result = append(result, row)
	}

	return result, nil
}

func (c *ExcelConnector) Close() error {
	return nil
}

type ManualConnector struct {
	Config ConnectionConfig
}

func (c *ManualConnector) TestConnection(ctx context.Context) error { return nil }

func (c *ManualConnector) IntrospectSchema(ctx context.Context) ([]ColumnDef, error) {
	if len(c.Config.ManualData) == 0 {
		return nil, fmt.Errorf("manual data must be provided")
	}
	var cols []ColumnDef
	for k := range c.Config.ManualData[0] {
		cols = append(cols, ColumnDef{Name: k, DataType: "STRING"})
	}
	return cols, nil
}

func (c *ManualConnector) FetchRecords(ctx context.Context, limit, offset int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = len(c.Config.ManualData)
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(c.Config.ManualData) {
		return []map[string]interface{}{}, nil
	}
	end := offset + limit
	if end > len(c.Config.ManualData) {
		end = len(c.Config.ManualData)
	}
	return c.Config.ManualData[offset:end], nil
}

func (c *ManualConnector) Close() error {
	return nil
}

func ParseJSONPayload(r io.Reader) ([]map[string]interface{}, error) {
	var records []map[string]interface{}
	err := json.NewDecoder(r).Decode(&records)
	return records, err
}

func validateIdentifier(s string) error {
	matched, _ := regexp.MatchString(`^[A-Za-z_][A-Za-z0-9_]*$`, s)
	if !matched {
		return fmt.Errorf("invalid identifier: %s", s)
	}
	return nil
}

func quoteIdentifier(id string) string {
	id = strings.TrimSpace(id)
	if matched, _ := regexp.MatchString(`^[A-Za-z_][A-Za-z0-9_]*$`, id); matched {
		return id
	}
	return `"` + strings.ReplaceAll(id, `"`, `""`) + `"`
}

func clamp(v, min, max int) int {
	if v <= 0 {
		v = 1000
	}
	if v > max {
		v = max
	}
	if v < min {
		v = min
	}
	return v
}

func sqlRowsToMaps(rows *sql.Rows) ([]map[string]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		valPtrs := make([]interface{}, len(cols))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}

		if err := rows.Scan(valPtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range cols {
			row[col] = vals[i]
		}
		results = append(results, row)
	}

	return results, rows.Err()
}

func bsonTypeToString(v interface{}) string {
	switch v.(type) {
	case primitive.ObjectID:
		return "OBJECTID"
	case string:
		return "STRING"
	case float64, float32:
		return "DOUBLE"
	case int, int32, int64:
		return "INT"
	case bool:
		return "BOOLEAN"
	case primitive.DateTime:
		return "DATE"
	case []byte:
		return "BINARY"
	case primitive.A:
		return "ARRAY"
	case bson.M:
		return "OBJECT"
	default:
		return "STRING"
	}
}

func convertObjectIDsToStrings(doc bson.M) bson.M {
	for k, v := range doc {
		switch val := v.(type) {
		case primitive.ObjectID:
			doc[k] = val.Hex()
		case []interface{}:
			for i, item := range val {
				if itemDoc, ok := item.(bson.M); ok {
					val[i] = convertObjectIDsToStrings(itemDoc)
				}
			}
			doc[k] = val
		case bson.M:
			doc[k] = convertObjectIDsToStrings(val)
		}
	}
	return doc
}
