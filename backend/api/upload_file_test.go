package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"entitymatcher/matcher"
	"entitymatcher/store"
	"github.com/xuri/excelize/v2"
)

func TestHandleUploadFileCSVHappyPath(t *testing.T) {
	server := NewServer(store.NewStore())
	tmpDir := t.TempDir()

	// Create source CSV
	sourcePath := filepath.Join(tmpDir, "source.csv")
	sourceCSV := `reference_id,customer_name,transaction_date
SRC001,Acme Corp,2024-01-15
SRC002,Beta LLC,2024-02-20
SRC003,Gamma Inc,2024-03-10`
	if err := os.WriteFile(sourcePath, []byte(sourceCSV), 0644); err != nil {
		t.Fatalf("Failed to write source CSV: %v", err)
	}

	// Create destination CSV
	destPath := filepath.Join(tmpDir, "dest.csv")
	destCSV := `customer_id,customer_name,transaction_date
DST001,Acme Corp,2024-01-15
DST002,Beta LLC,2024-02-20`
	if err := os.WriteFile(destPath, []byte(destCSV), 0644); err != nil {
		t.Fatalf("Failed to write destination CSV: %v", err)
	}

	// Build multipart body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		t.Fatalf("Failed to open source file: %v", err)
	}
	defer sourceFile.Close()
	part, err := writer.CreateFormFile("source_file", "source.csv")
	if err != nil {
		t.Fatalf("Failed to create source_file part: %v", err)
	}
	if _, err = io.Copy(part, sourceFile); err != nil {
		t.Fatalf("Failed to copy source file: %v", err)
	}

	// Add destination file
	destFile, err := os.Open(destPath)
	if err != nil {
		t.Fatalf("Failed to open destination file: %v", err)
	}
	defer destFile.Close()
	part, err = writer.CreateFormFile("destination_file", "dest.csv")
	if err != nil {
		t.Fatalf("Failed to create destination_file part: %v", err)
	}
	if _, err = io.Copy(part, destFile); err != nil {
		t.Fatalf("Failed to copy destination file: %v", err)
	}

	// Add batch_id
	if err := writer.WriteField("batch_id", "test-csv-happy-1"); err != nil {
		t.Fatalf("Failed to write batch_id: %v", err)
	}

	writer.Close()

	// Get admin token
	token, err := generateToken(sampleUsers["admin"])
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/upload/file", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler := RequireAuth(RequireRole(RoleAdmin, RoleEngineer)(http.HandlerFunc(server.HandleUploadFile)))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["status"] != "success" {
		t.Fatalf("Expected status 'success', got '%v'", resp["status"])
	}

	batchID, ok := resp["batch_id"].(string)
	if !ok {
		t.Fatalf("Expected batch_id as string, got %T", resp["batch_id"])
	}
	if batchID != "test-csv-happy-1" {
		t.Fatalf("Expected batch_id 'test-csv-happy-1', got '%s'", batchID)
	}

	sourceCount, ok := resp["source_count"].(float64)
	if !ok || sourceCount != 3 {
		t.Fatalf("Expected source_count 3, got %v", resp["source_count"])
	}

	destCount, ok := resp["destination_count"].(float64)
	if !ok || destCount != 2 {
		t.Fatalf("Expected destination_count 2, got %v", resp["destination_count"])
	}

	// Verify dataset persisted correctly
	_, _, ok = server.store.GetDataset(batchID)
	if !ok {
		t.Fatalf("Expected dataset to be persisted for batch_id '%s'", batchID)
	}
}

func TestHandleUploadFileXLSXHappyPath(t *testing.T) {
	server := NewServer(store.NewStore())
	tmpDir := t.TempDir()

	// Create source Excel file
	sourcePath := filepath.Join(tmpDir, "source.xlsx")
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	f.SetCellValue(sheet, "A1", "reference_id")
	f.SetCellValue(sheet, "B1", "customer_name")
	f.SetCellValue(sheet, "C1", "transaction_date")
	f.SetCellValue(sheet, "A2", "SRC001")
	f.SetCellValue(sheet, "B2", "Acme Corp")
	f.SetCellValue(sheet, "C2", "2024-01-15")
	f.SetCellValue(sheet, "A3", "SRC002")
	f.SetCellValue(sheet, "B3", "Beta LLC")
	f.SetCellValue(sheet, "C3", "2024-02-20")
	f.SetCellValue(sheet, "A4", "SRC003")
	f.SetCellValue(sheet, "B4", "Gamma Inc")
	f.SetCellValue(sheet, "C4", "2024-03-10")
	if err := f.SaveAs(sourcePath); err != nil {
		t.Fatalf("Failed to save source Excel: %v", err)
	}

	// Create destination Excel file
	destPath := filepath.Join(tmpDir, "dest.xlsx")
	f2 := excelize.NewFile()
	sheet2 := f2.GetSheetName(0)
	f2.SetCellValue(sheet2, "A1", "customer_id")
	f2.SetCellValue(sheet2, "B1", "customer_name")
	f2.SetCellValue(sheet2, "C1", "transaction_date")
	f2.SetCellValue(sheet2, "A2", "DST001")
	f2.SetCellValue(sheet2, "B2", "Acme Corp")
	f2.SetCellValue(sheet2, "C2", "2024-01-15")
	f2.SetCellValue(sheet2, "A3", "DST002")
	f2.SetCellValue(sheet2, "B3", "Beta LLC")
	f2.SetCellValue(sheet2, "C3", "2024-02-20")
	if err := f2.SaveAs(destPath); err != nil {
		t.Fatalf("Failed to save destination Excel: %v", err)
	}

	// Build multipart body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		t.Fatalf("Failed to open source file: %v", err)
	}
	defer sourceFile.Close()
	part, err := writer.CreateFormFile("source_file", "source.xlsx")
	if err != nil {
		t.Fatalf("Failed to create source_file part: %v", err)
	}
	if _, err = io.Copy(part, sourceFile); err != nil {
		t.Fatalf("Failed to copy source file: %v", err)
	}

	// Add destination file
	destFile, err := os.Open(destPath)
	if err != nil {
		t.Fatalf("Failed to open destination file: %v", err)
	}
	defer destFile.Close()
	part, err = writer.CreateFormFile("destination_file", "dest.xlsx")
	if err != nil {
		t.Fatalf("Failed to create destination_file part: %v", err)
	}
	if _, err = io.Copy(part, destFile); err != nil {
		t.Fatalf("Failed to copy destination file: %v", err)
	}

	// Add batch_id
	if err := writer.WriteField("batch_id", "test-xlsx-happy-1"); err != nil {
		t.Fatalf("Failed to write batch_id: %v", err)
	}

	writer.Close()

	// Get admin token
	token, err := generateToken(sampleUsers["admin"])
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/upload/file", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler := RequireAuth(RequireRole(RoleAdmin, RoleEngineer)(http.HandlerFunc(server.HandleUploadFile)))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	batchID, ok := resp["batch_id"].(string)
	if !ok {
		t.Fatalf("Expected batch_id as string, got %T", resp["batch_id"])
	}
	if batchID != "test-xlsx-happy-1" {
		t.Fatalf("Expected batch_id 'test-xlsx-happy-1', got '%s'", batchID)
	}

	sourceCount, ok := resp["source_count"].(float64)
	if !ok || sourceCount != 3 {
		t.Fatalf("Expected source_count 3, got %v", resp["source_count"])
	}

	destCount, ok := resp["destination_count"].(float64)
	if !ok || destCount != 2 {
		t.Fatalf("Expected destination_count 2, got %v", resp["destination_count"])
	}

	// Verify dataset persisted correctly
	_, _, ok = server.store.GetDataset(batchID)
	if !ok {
		t.Fatalf("Expected dataset to be persisted for batch_id '%s'", batchID)
	}
}

func TestHandleUploadFileRejectedExtension(t *testing.T) {
	server := NewServer(store.NewStore())
	tmpDir := t.TempDir()

	// Create source text file (invalid)
	sourcePath := filepath.Join(tmpDir, "source.txt")
	sourceContent := "id,name\n1,Test"
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Create destination CSV (valid)
	destPath := filepath.Join(tmpDir, "dest.csv")
	destContent := `id,name
1,Test`
	if err := os.WriteFile(destPath, []byte(destContent), 0644); err != nil {
		t.Fatalf("Failed to write destination file: %v", err)
	}

	// Build multipart body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		t.Fatalf("Failed to open source file: %v", err)
	}
	defer sourceFile.Close()
	part, err := writer.CreateFormFile("source_file", "source.txt")
	if err != nil {
		t.Fatalf("Failed to create source_file part: %v", err)
	}
	if _, err = io.Copy(part, sourceFile); err != nil {
		t.Fatalf("Failed to copy source file: %v", err)
	}

	// Add destination file
	destFile, err := os.Open(destPath)
	if err != nil {
		t.Fatalf("Failed to open destination file: %v", err)
	}
	defer destFile.Close()
	part, err = writer.CreateFormFile("destination_file", "dest.csv")
	if err != nil {
		t.Fatalf("Failed to create destination_file part: %v", err)
	}
	if _, err = io.Copy(part, destFile); err != nil {
		t.Fatalf("Failed to copy destination file: %v", err)
	}

	// Add batch_id
	batchID := "test-rejected-ext-1"
	if err := writer.WriteField("batch_id", batchID); err != nil {
		t.Fatalf("Failed to write batch_id: %v", err)
	}

	writer.Close()

	// Get admin token
	token, err := generateToken(sampleUsers["admin"])
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/upload/file", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler := RequireAuth(RequireRole(RoleAdmin, RoleEngineer)(http.HandlerFunc(server.HandleUploadFile)))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify no dataset was persisted
	_, _, ok := server.store.GetDataset(batchID)
	if ok {
		t.Fatalf("Expected no dataset for rejected extension, but got ok=true")
	}
}

func TestHandleUploadFileMissingDestination(t *testing.T) {
	server := NewServer(store.NewStore())
	tmpDir := t.TempDir()

	// Create source CSV
	sourcePath := filepath.Join(tmpDir, "source.csv")
	sourceContent := `id,name
1,Test`
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Build multipart body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add source file only
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		t.Fatalf("Failed to open source file: %v", err)
	}
	defer sourceFile.Close()
	part, err := writer.CreateFormFile("source_file", "source.csv")
	if err != nil {
		t.Fatalf("Failed to create source_file part: %v", err)
	}
	if _, err = io.Copy(part, sourceFile); err != nil {
		t.Fatalf("Failed to copy source file: %v", err)
	}

	// Add batch_id
	batchID := "test-missing-dest-1"
	if err := writer.WriteField("batch_id", batchID); err != nil {
		t.Fatalf("Failed to write batch_id: %v", err)
	}

	writer.Close()

	// Get admin token
	token, err := generateToken(sampleUsers["admin"])
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/upload/file", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler := RequireAuth(RequireRole(RoleAdmin, RoleEngineer)(http.HandlerFunc(server.HandleUploadFile)))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify no dataset was persisted
	_, _, ok := server.store.GetDataset(batchID)
	if ok {
		t.Fatalf("Expected no dataset for missing destination, but got ok=true")
	}
}

func TestHandleUploadFileRBACForbiddenReviewer(t *testing.T) {
	server := NewServer(store.NewStore())
	tmpDir := t.TempDir()

	// Create minimal CSVs
	sourcePath := filepath.Join(tmpDir, "source.csv")
	if err := os.WriteFile(sourcePath, []byte("id,name\n1,Test"), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	destPath := filepath.Join(tmpDir, "dest.csv")
	if err := os.WriteFile(destPath, []byte("id,name\n2,Test2"), 0644); err != nil {
		t.Fatalf("Failed to write destination file: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		t.Fatalf("Failed to open source file: %v", err)
	}
	defer sourceFile.Close()
	part, err := writer.CreateFormFile("source_file", "source.csv")
	if err != nil {
		t.Fatalf("Failed to create source_file part: %v", err)
	}
	if _, err = io.Copy(part, sourceFile); err != nil {
		t.Fatalf("Failed to copy source file: %v", err)
	}

	destFile, err := os.Open(destPath)
	if err != nil {
		t.Fatalf("Failed to open destination file: %v", err)
	}
	defer destFile.Close()
	part, err = writer.CreateFormFile("destination_file", "dest.csv")
	if err != nil {
		t.Fatalf("Failed to create destination_file part: %v", err)
	}
	if _, err = io.Copy(part, destFile); err != nil {
		t.Fatalf("Failed to copy destination file: %v", err)
	}

	if err := writer.WriteField("batch_id", "test-rbac-forbidden-1"); err != nil {
		t.Fatalf("Failed to write batch_id: %v", err)
	}

	writer.Close()

	token, err := generateToken(sampleUsers["reviewer_sarah"])
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/upload/file", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler := RequireAuth(RequireRole(RoleAdmin, RoleEngineer)(http.HandlerFunc(server.HandleUploadFile)))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403, got %d", w.Code)
	}
}

func TestHandleUploadFileRBACUnauthorizedNoToken(t *testing.T) {
	server := NewServer(store.NewStore())
	tmpDir := t.TempDir()

	sourcePath := filepath.Join(tmpDir, "source.csv")
	if err := os.WriteFile(sourcePath, []byte("id,name\n1,Test"), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	destPath := filepath.Join(tmpDir, "dest.csv")
	if err := os.WriteFile(destPath, []byte("id,name\n2,Test2"), 0644); err != nil {
		t.Fatalf("Failed to write destination file: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		t.Fatalf("Failed to open source file: %v", err)
	}
	defer sourceFile.Close()
	part, err := writer.CreateFormFile("source_file", "source.csv")
	if err != nil {
		t.Fatalf("Failed to create source_file part: %v", err)
	}
	if _, err = io.Copy(part, sourceFile); err != nil {
		t.Fatalf("Failed to copy source file: %v", err)
	}

	destFile, err := os.Open(destPath)
	if err != nil {
		t.Fatalf("Failed to open destination file: %v", err)
	}
	defer destFile.Close()
	part, err = writer.CreateFormFile("destination_file", "dest.csv")
	if err != nil {
		t.Fatalf("Failed to create destination_file part: %v", err)
	}
	if _, err = io.Copy(part, destFile); err != nil {
		t.Fatalf("Failed to copy destination file: %v", err)
	}

	if err := writer.WriteField("batch_id", "test-rbac-unauthorized-1"); err != nil {
		t.Fatalf("Failed to write batch_id: %v", err)
	}

	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload/file", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	handler := RequireAuth(RequireRole(RoleAdmin, RoleEngineer)(http.HandlerFunc(server.HandleUploadFile)))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

func TestHandleUploadFileColumnMappingHonored(t *testing.T) {
	server := NewServer(store.NewStore())
	tmpDir := t.TempDir()

	// Create source CSV with non-standard headers
	sourcePath := filepath.Join(tmpDir, "source.csv")
	sourceContent := `ref,full_name,txn_date
R001,Alice Smith,2024-03-01
R002,Bob Jones,2024-03-02`
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Create destination CSV with non-standard headers
	destPath := filepath.Join(tmpDir, "dest.csv")
	destContent := `cust,full_name,txn_date
D001,Alice Smith,2024-03-01`
	if err := os.WriteFile(destPath, []byte(destContent), 0644); err != nil {
		t.Fatalf("Failed to write destination file: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		t.Fatalf("Failed to open source file: %v", err)
	}
	defer sourceFile.Close()
	part, err := writer.CreateFormFile("source_file", "source.csv")
	if err != nil {
		t.Fatalf("Failed to create source_file part: %v", err)
	}
	if _, err = io.Copy(part, sourceFile); err != nil {
		t.Fatalf("Failed to copy source file: %v", err)
	}

	destFile, err := os.Open(destPath)
	if err != nil {
		t.Fatalf("Failed to open destination file: %v", err)
	}
	defer destFile.Close()
	part, err = writer.CreateFormFile("destination_file", "dest.csv")
	if err != nil {
		t.Fatalf("Failed to create destination_file part: %v", err)
	}
	if _, err = io.Copy(part, destFile); err != nil {
		t.Fatalf("Failed to copy destination file: %v", err)
	}

	// Add batch_id and column_mapping
	batchID := "test-column-mapping-1"
	if err := writer.WriteField("batch_id", batchID); err != nil {
		t.Fatalf("Failed to write batch_id: %v", err)
	}

	columnMapping := matcher.ColumnMapping{
		NameFieldsSrc:   []string{"full_name"},
		NameFieldsDest:  []string{"full_name"},
		RefIDSrc:        "ref",
		RefIDDest:       "cust",
		SecondaryFields: []matcher.SecondaryFieldMapping{},
	}
	columnMappingJSON, err := json.Marshal(columnMapping)
	if err != nil {
		t.Fatalf("Failed to marshal column_mapping: %v", err)
	}
	if err := writer.WriteField("column_mapping", string(columnMappingJSON)); err != nil {
		t.Fatalf("Failed to write column_mapping: %v", err)
	}

	writer.Close()

	token, err := generateToken(sampleUsers["admin"])
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/upload/file", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	handler := RequireAuth(RequireRole(RoleAdmin, RoleEngineer)(http.HandlerFunc(server.HandleUploadFile)))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify column_mapping in response
	columnMappingResp := resp["column_mapping"]
	columnMappingBytes, err := json.Marshal(columnMappingResp)
	if err != nil {
		t.Fatalf("Failed to marshal column_mapping from response: %v", err)
	}
	var parsedMapping matcher.ColumnMapping
	if err := json.Unmarshal(columnMappingBytes, &parsedMapping); err != nil {
		t.Fatalf("Failed to unmarshal column_mapping: %v", err)
	}

	if parsedMapping.RefIDSrc != "ref" {
		t.Fatalf("Expected RefIDSrc='ref', got '%s'", parsedMapping.RefIDSrc)
	}
	if parsedMapping.RefIDDest != "cust" {
		t.Fatalf("Expected RefIDDest='cust', got '%s'", parsedMapping.RefIDDest)
	}
	if len(parsedMapping.NameFieldsSrc) != 1 || parsedMapping.NameFieldsSrc[0] != "full_name" {
		t.Fatalf("Expected NameFieldsSrc=['full_name'], got %v", parsedMapping.NameFieldsSrc)
	}

	// Verify dataset with applied mapping
	sourceRecords, destRecords, ok := server.store.GetDataset(batchID)
	if !ok {
		t.Fatalf("Expected dataset to be persisted")
	}

	if len(sourceRecords) != 2 {
		t.Fatalf("Expected 2 source records, got %d", len(sourceRecords))
	}

	// Check first source record for correct reference_id and name
	if sourceRecords[0].ReferenceID != "R001" {
		t.Fatalf("Expected source reference_id='R001', got '%s'", sourceRecords[0].ReferenceID)
	}
	if sourceRecords[0].CustomerNameRaw != "Alice Smith" {
		t.Fatalf("Expected customer_name='Alice Smith', got '%s'", sourceRecords[0].CustomerNameRaw)
	}

	if len(destRecords) != 1 {
		t.Fatalf("Expected 1 destination record, got %d", len(destRecords))
	}
	if destRecords[0].CustomerID != "D001" {
		t.Fatalf("Expected destination customer_id='D001', got '%s'", destRecords[0].CustomerID)
	}
	if destRecords[0].CustomerNameRaw != "Alice Smith" {
		t.Fatalf("Expected destination customer_name='Alice Smith', got '%s'", destRecords[0].CustomerNameRaw)
	}
}
