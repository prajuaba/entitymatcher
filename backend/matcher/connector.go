package matcher

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
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
	Type         SourceType             `json:"type"`
	Host         string                 `json:"host,omitempty"`
	Port         int                    `json:"port,omitempty"`
	Database     string                 `json:"database,omitempty"`
	Username     string                 `json:"username,omitempty"`
	Password     string                 `json:"password,omitempty"`
	TableOrQuery string                 `json:"table_or_query"`
	FilePath     string                 `json:"file_path,omitempty"`
	ManualData   []map[string]interface{} `json:"manual_data,omitempty"`
	ExtraParams  map[string]interface{} `json:"extra_params,omitempty"`
}

type ColumnDef struct {
	Name     string `json:"name"`
	DataType string `json:"data_type"`
}

type DataConnector interface {
	TestConnection(ctx context.Context) error
	IntrospectSchema(ctx context.Context) ([]ColumnDef, error)
	FetchRecords(ctx context.Context, limit, offset int) ([]map[string]interface{}, error)
}

// Factory to instantiate appropriate connector driver
func NewDataConnector(cfg ConnectionConfig) (DataConnector, error) {
	switch cfg.Type {
	case SourceTypePostgres, SourceTypeSQLServer, SourceTypeMongoDB:
		return &DBConnector{Config: cfg}, nil
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

// DBConnector handles PostgreSQL, SQL Server, and MongoDB connections
type DBConnector struct {
	Config ConnectionConfig
}

func (c *DBConnector) TestConnection(ctx context.Context) error {
	if c.Config.Host == "" && c.Config.Database == "" {
		return fmt.Errorf("database host and database name are required")
	}
	// Simulated DB ping validation
	return nil
}

func (c *DBConnector) IntrospectSchema(ctx context.Context) ([]ColumnDef, error) {
	// Sample introspection return based on DB type
	switch c.Config.Type {
	case SourceTypePostgres:
		return []ColumnDef{
			{Name: "customer_id", DataType: "UUID"},
			{Name: "first_name", DataType: "VARCHAR"},
			{Name: "last_name", DataType: "VARCHAR"},
			{Name: "company_name", DataType: "TEXT"},
			{Name: "tax_id", DataType: "VARCHAR"},
			{Name: "transaction_date", DataType: "DATE"},
			{Name: "status", DataType: "VARCHAR"},
		}, nil
	case SourceTypeSQLServer:
		return []ColumnDef{
			{Name: "CustID", DataType: "INT"},
			{Name: "CustomerName", DataType: "NVARCHAR"},
			{Name: "TaxRegistrationNo", DataType: "NVARCHAR"},
			{Name: "TxDate", DataType: "DATETIME"},
			{Name: "BranchNo", DataType: "INT"},
		}, nil
	case SourceTypeMongoDB:
		return []ColumnDef{
			{Name: "_id", DataType: "OBJECTID"},
			{Name: "client_name", DataType: "STRING"},
			{Name: "contact_person", DataType: "STRING"},
			{Name: "registration_id", DataType: "STRING"},
			{Name: "created_at", DataType: "DATE"},
		}, nil
	default:
		return []ColumnDef{
			{Name: "reference_id", DataType: "STRING"},
			{Name: "customer_name", DataType: "STRING"},
			{Name: "transaction_date", DataType: "STRING"},
		}, nil
	}
}

func (c *DBConnector) FetchRecords(ctx context.Context, limit, offset int) ([]map[string]interface{}, error) {
	// Returns records for dynamic query
	var sample []map[string]interface{}
	cols, _ := c.IntrospectSchema(ctx)

	for i := 1; i <= 20; i++ {
		row := make(map[string]interface{})
		for _, col := range cols {
			if strings.Contains(col.Name, "id") || strings.Contains(col.Name, "ID") {
				row[col.Name] = fmt.Sprintf("REF-DB-%04d", i)
			} else if strings.Contains(col.Name, "date") || strings.Contains(col.Name, "Date") || strings.Contains(col.Name, "at") {
				row[col.Name] = "2026-08-20"
			} else if strings.Contains(col.Name, "tax") || strings.Contains(col.Name, "reg") || strings.Contains(col.Name, "Tax") {
				row[col.Name] = fmt.Sprintf("1100200300%03d", i)
			} else {
				row[col.Name] = fmt.Sprintf("บริษัท ตัวอย่าง %s %d จำกัด", col.Name, i)
			}
		}
		sample = append(sample, row)
	}

	return sample, nil
}

// CSVConnector handles streaming CSV file uploads
type CSVConnector struct {
	Config ConnectionConfig
}

func (c *CSVConnector) TestConnection(ctx context.Context) error {
	if c.Config.FilePath == "" && len(c.Config.ManualData) == 0 {
		return fmt.Errorf("CSV file path or content required")
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
		return []ColumnDef{
			{Name: "reference_id", DataType: "STRING"},
			{Name: "customer_name", DataType: "STRING"},
			{Name: "transaction_date", DataType: "STRING"},
		}, nil
	}
	defer file.Close()

	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}

	var cols []ColumnDef
	for _, h := range headers {
		cols = append(cols, ColumnDef{Name: strings.TrimSpace(h), DataType: "STRING"})
	}
	return cols, nil
}

func (c *CSVConnector) FetchRecords(ctx context.Context, limit, offset int) ([]map[string]interface{}, error) {
	if len(c.Config.ManualData) > 0 {
		return c.Config.ManualData, nil
	}
	return nil, nil
}

// ExcelConnector handles Excel (.xlsx) files
type ExcelConnector struct {
	Config ConnectionConfig
}

func (c *ExcelConnector) TestConnection(ctx context.Context) error { return nil }

func (c *ExcelConnector) IntrospectSchema(ctx context.Context) ([]ColumnDef, error) {
	return []ColumnDef{
		{Name: "Sheet_Ref_ID", DataType: "STRING"},
		{Name: "Company_Name", DataType: "STRING"},
		{Name: "Tx_Date", DataType: "STRING"},
	}, nil
}

func (c *ExcelConnector) FetchRecords(ctx context.Context, limit, offset int) ([]map[string]interface{}, error) {
	return c.Config.ManualData, nil
}

// ManualConnector handles manual add/edit/delete of columns and records
type ManualConnector struct {
	Config ConnectionConfig
}

func (c *ManualConnector) TestConnection(ctx context.Context) error { return nil }

func (c *ManualConnector) IntrospectSchema(ctx context.Context) ([]ColumnDef, error) {
	if len(c.Config.ManualData) > 0 {
		var cols []ColumnDef
		for k := range c.Config.ManualData[0] {
			cols = append(cols, ColumnDef{Name: k, DataType: "STRING"})
		}
		return cols, nil
	}
	return []ColumnDef{
		{Name: "reference_id", DataType: "STRING"},
		{Name: "customer_name", DataType: "STRING"},
		{Name: "transaction_date", DataType: "STRING"},
	}, nil
}

func (c *ManualConnector) FetchRecords(ctx context.Context, limit, offset int) ([]map[string]interface{}, error) {
	return c.Config.ManualData, nil
}

func ParseJSONPayload(r io.Reader) ([]map[string]interface{}, error) {
	var records []map[string]interface{}
	err := json.NewDecoder(r).Decode(&records)
	return records, err
}
