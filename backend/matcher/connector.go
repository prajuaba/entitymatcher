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
	Config ConnectionConfig
	pool   *pgxpool.Pool
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
		query := c.Config.TableOrQuery + " LIMIT $1 OFFSET $2"
		rows, err = c.pool.Query(ctx, query, limit, offset)
	} else {
		query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT $1 OFFSET $2",
			quoteIdentifier(schema), quoteIdentifier(table))
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
	Config ConnectionConfig
	conn   *sql.DB
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

// sqlServerFetchQuery builds the paged read for a schema-qualified table.
func sqlServerFetchQuery(schema, table string) string {
	qualified := quoteMSSQLIdentifier(schema) + "." + quoteMSSQLIdentifier(table)
	return fmt.Sprintf("SELECT * FROM %s ORDER BY (SELECT NULL) OFFSET @Offset ROWS FETCH NEXT @Limit ROWS ONLY", qualified)
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

	query := sqlServerFetchQuery(schema, table)

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
	cursor, err := coll.Find(ctx, bson.D{}, options.Find().SetSkip(int64(offset)).SetLimit(int64(limit)))
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
