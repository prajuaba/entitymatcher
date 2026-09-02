package matcher

import (
	"context"
	"encoding/csv"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestPostgresConnectorConnectionFails(t *testing.T) {
	cfg := ConnectionConfig{
		Type:     SourceTypePostgres,
		Host:     "does-not-exist.invalid",
		Port:     5432,
		Database: "test",
		Username: "user",
		Password: "pass",
	}

	connector := &PostgresConnector{Config: cfg}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := connector.TestConnection(ctx)
	if err == nil {
		t.Fatal("Expected error for unreachable host, got nil")
	}

	t.Logf("Correctly got error for unreachable Postgres host: %v", err)
}

func TestSQLServerConnectorConnectionFails(t *testing.T) {
	cfg := ConnectionConfig{
		Type:     SourceTypeSQLServer,
		Host:     "does-not-exist.invalid",
		Port:     1433,
		Database: "test",
		Username: "user",
		Password: "pass",
	}

	connector := &SQLServerConnector{Config: cfg}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := connector.TestConnection(ctx)
	if err == nil {
		t.Fatal("Expected error for unreachable host, got nil")
	}

	t.Logf("Correctly got error for unreachable SQL Server host: %v", err)
}

func TestMongoConnectorIntegration(t *testing.T) {
	cfg := ConnectionConfig{
		Type:         SourceTypeMongoDB,
		Host:         "localhost",
		Port:         27017,
		Database:     "test",
		TableOrQuery: "test_collection",
	}

	connector := &MongoConnector{Config: cfg}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := connector.TestConnection(ctx)
	if err != nil {
		t.Skip("MongoDB not available on localhost:27017, skipping integration test")
	}

	if connector.client == nil {
		t.Fatal("Expected client to be set after successful connection")
	}

	defer connector.client.Disconnect(context.Background())

	t.Log("MongoDB connection successful")
}

func TestCSVConnectorWithFile(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "test*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	writer := csv.NewWriter(tmpfile)
	writer.Write([]string{"id", "name", "email"})
	writer.Write([]string{"1", "John", "john@example.com"})
	writer.Write([]string{"2", "Jane", "jane@example.com"})
	writer.Write([]string{"3", "Bob", "bob@example.com"})
	writer.Flush()
	tmpfile.Close()

	cfg := ConnectionConfig{
		Type:     SourceTypeCSV,
		FilePath: tmpfile.Name(),
	}

	connector := &CSVConnector{Config: cfg}
	ctx := context.Background()

	// Test IntrospectSchema
	schema, err := connector.IntrospectSchema(ctx)
	if err != nil {
		t.Fatalf("IntrospectSchema failed: %v", err)
	}

	if len(schema) != 3 {
		t.Fatalf("Expected 3 columns, got %d", len(schema))
	}

	if schema[0].Name != "id" || schema[1].Name != "name" || schema[2].Name != "email" {
		t.Fatalf("Column names mismatch: %v", schema)
	}

	// Test FetchRecords
	records, err := connector.FetchRecords(ctx, 10, 0)
	if err != nil {
		t.Fatalf("FetchRecords failed: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("Expected 3 records, got %d", len(records))
	}

	if records[0]["id"] != "1" || records[0]["name"] != "John" {
		t.Fatalf("Record data mismatch: %v", records[0])
	}

	// Test offset and limit
	records2, err := connector.FetchRecords(ctx, 1, 1)
	if err != nil {
		t.Fatalf("FetchRecords with offset failed: %v", err)
	}

	if len(records2) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records2))
	}

	if records2[0]["id"] != "2" {
		t.Fatalf("Expected id=2, got %v", records2[0]["id"])
	}

	t.Log("CSV connector test passed")
}

func TestCSVConnectorWithManualData(t *testing.T) {
	manualData := []map[string]interface{}{
		{"id": "1", "name": "Alice"},
		{"id": "2", "name": "Bob"},
	}

	cfg := ConnectionConfig{
		Type:       SourceTypeCSV,
		ManualData: manualData,
	}

	connector := &CSVConnector{Config: cfg}
	ctx := context.Background()

	// Test IntrospectSchema
	schema, err := connector.IntrospectSchema(ctx)
	if err != nil {
		t.Fatalf("IntrospectSchema failed: %v", err)
	}

	if len(schema) != 2 {
		t.Fatalf("Expected 2 columns, got %d", len(schema))
	}

	// Test FetchRecords
	records, err := connector.FetchRecords(ctx, 10, 0)
	if err != nil {
		t.Fatalf("FetchRecords failed: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(records))
	}

	t.Log("CSV manual data test passed")
}

func TestExcelConnectorWithFile(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "test*.xlsx")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	// Write headers
	f.SetCellValue(sheet, "A1", "id")
	f.SetCellValue(sheet, "B1", "name")
	f.SetCellValue(sheet, "C1", "email")

	// Write data rows
	f.SetCellValue(sheet, "A2", "1")
	f.SetCellValue(sheet, "B2", "John")
	f.SetCellValue(sheet, "C2", "john@example.com")

	f.SetCellValue(sheet, "A3", "2")
	f.SetCellValue(sheet, "B3", "Jane")
	f.SetCellValue(sheet, "C3", "jane@example.com")

	if err := f.SaveAs(tmpfile.Name()); err != nil {
		t.Fatalf("Failed to save Excel file: %v", err)
	}

	cfg := ConnectionConfig{
		Type:     SourceTypeExcel,
		FilePath: tmpfile.Name(),
	}

	connector := &ExcelConnector{Config: cfg}
	ctx := context.Background()

	// Test IntrospectSchema
	schema, err := connector.IntrospectSchema(ctx)
	if err != nil {
		t.Fatalf("IntrospectSchema failed: %v", err)
	}

	if len(schema) != 3 {
		t.Fatalf("Expected 3 columns, got %d", len(schema))
	}

	if schema[0].Name != "id" || schema[1].Name != "name" {
		t.Fatalf("Column names mismatch: %v", schema)
	}

	// Test FetchRecords
	records, err := connector.FetchRecords(ctx, 10, 0)
	if err != nil {
		t.Fatalf("FetchRecords failed: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(records))
	}

	if records[0]["id"] != "1" || records[0]["name"] != "John" {
		t.Fatalf("Record data mismatch: %v", records[0])
	}

	t.Log("Excel connector test passed")
}

func TestManualConnectorErrorWhenEmpty(t *testing.T) {
	cfg := ConnectionConfig{
		Type:       SourceTypeManual,
		ManualData: []map[string]interface{}{},
	}

	connector := &ManualConnector{Config: cfg}
	ctx := context.Background()

	_, err := connector.IntrospectSchema(ctx)
	if err == nil {
		t.Fatal("Expected error when ManualData is empty, got nil")
	}

	t.Logf("Correctly got error for empty ManualData: %v", err)
}

func TestManualConnectorWithData(t *testing.T) {
	manualData := []map[string]interface{}{
		{"id": "1", "name": "Alice"},
		{"id": "2", "name": "Bob"},
		{"id": "3", "name": "Charlie"},
	}

	cfg := ConnectionConfig{
		Type:       SourceTypeManual,
		ManualData: manualData,
	}

	connector := &ManualConnector{Config: cfg}
	ctx := context.Background()

	// Test IntrospectSchema
	schema, err := connector.IntrospectSchema(ctx)
	if err != nil {
		t.Fatalf("IntrospectSchema failed: %v", err)
	}

	if len(schema) != 2 {
		t.Fatalf("Expected 2 columns, got %d", len(schema))
	}

	// Test FetchRecords with pagination
	records, err := connector.FetchRecords(ctx, 2, 0)
	if err != nil {
		t.Fatalf("FetchRecords failed: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("Expected 2 records with limit=2, got %d", len(records))
	}

	// Test offset
	records2, err := connector.FetchRecords(ctx, 2, 1)
	if err != nil {
		t.Fatalf("FetchRecords with offset failed: %v", err)
	}

	if len(records2) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(records2))
	}

	if records2[0]["id"] != "2" {
		t.Fatalf("Expected id=2, got %v", records2[0]["id"])
	}

	t.Log("ManualConnector test passed")
}

func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "table", false},
		{"valid with underscore", "my_table", false},
		{"valid starting with underscore", "_table", false},
		{"invalid starting with number", "1table", true},
		{"invalid with space", "my table", true},
		{"invalid with dash", "my-table", true},
		{"invalid empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIdentifier(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIdentifier(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"table", "table"},
		{"my_table", "my_table"},
		{"my-table", `"my-table"`},
		{"my table", `"my table"`},
		{`my"table`, `"my""table"`},
	}

	for _, tt := range tests {
		result := quoteIdentifier(tt.input)
		if result != tt.expected {
			t.Errorf("quoteIdentifier(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		value    int
		min      int
		max      int
		expected int
	}{
		{-1, 1, 50000, 1000},      // <= 0 uses 1000
		{0, 1, 50000, 1000},       // <= 0 uses 1000
		{500, 1, 50000, 500},      // Within range
		{100000, 1, 50000, 50000}, // Exceeds max
		{1, 1, 50000, 1},          // At min
		{50000, 1, 50000, 50000},  // At max
	}

	for _, tt := range tests {
		result := clamp(tt.value, tt.min, tt.max)
		if result != tt.expected {
			t.Errorf("clamp(%d, %d, %d) = %d, expected %d", tt.value, tt.min, tt.max, result, tt.expected)
		}
	}
}
