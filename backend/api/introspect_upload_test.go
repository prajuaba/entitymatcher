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

	"entitymatcher/store"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestIntrospectUploadedFileCSVReturnsColumns(t *testing.T) {
	server := NewServer(store.NewStore())

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "customers.csv")
	require.NoError(t, err)
	_, err = part.Write([]byte("id,customer_name,tax_id\n1,Acme Corp,XYZ123"))
	require.NoError(t, err)

	writer.Close()

	req := httptest.NewRequest("POST", "/api/connector/introspect/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	server.HandleIntrospectUploadedFile(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	require.Equal(t, "success", resp["status"])
	require.Equal(t, "CSV", resp["type"])
	require.Equal(t, "customers.csv", resp["filename"])

	columns, ok := resp["columns"].([]interface{})
	require.True(t, ok, "columns should be a slice")
	require.Len(t, columns, 3)

	expected := []string{"id", "customer_name", "tax_id"}
	for i, want := range expected {
		colMap, ok := columns[i].(map[string]interface{})
		require.True(t, ok, "column entry should be a map")
		require.Equal(t, want, colMap["name"], "column %d name mismatch", i)
	}
}

func TestIntrospectUploadedFileExcelReturnsColumns(t *testing.T) {
	// This is the most important test — the exact user-reported flow of
	// introspecting an uploaded Excel file and verifying the returned column names.
	server := NewServer(store.NewStore())
	tmpDir := t.TempDir()

	xlsxPath := filepath.Join(tmpDir, "customers.xlsx")
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	f.SetCellValue(sheet, "A1", "CustID")
	f.SetCellValue(sheet, "B1", "CustomerName")
	f.SetCellValue(sheet, "C1", "TaxRegistrationNo")
	f.SetCellValue(sheet, "A2", "C001")
	f.SetCellValue(sheet, "B2", "Acme Corp")
	f.SetCellValue(sheet, "C2", "XYZ123")
	require.NoError(t, f.SaveAs(xlsxPath))
	require.NoError(t, f.Close())

	srcFile, err := os.Open(xlsxPath)
	require.NoError(t, err)
	defer srcFile.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "customers.xlsx")
	require.NoError(t, err)
	_, err = io.Copy(part, srcFile)
	require.NoError(t, err)

	writer.Close()

	req := httptest.NewRequest("POST", "/api/connector/introspect/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	server.HandleIntrospectUploadedFile(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	require.Equal(t, "success", resp["status"])
	require.Equal(t, "EXCEL", resp["type"])
	require.Equal(t, "customers.xlsx", resp["filename"])

	columns, ok := resp["columns"].([]interface{})
	require.True(t, ok, "columns should be a slice")
	require.Len(t, columns, 3)

	expected := []string{"CustID", "CustomerName", "TaxRegistrationNo"}
	for i, want := range expected {
		colMap, ok := columns[i].(map[string]interface{})
		require.True(t, ok, "column entry should be a map")
		require.Equal(t, want, colMap["name"], "column %d name mismatch", i)
	}
}

func TestIntrospectUploadedFileExcelHonoursSheetField(t *testing.T) {
	server := NewServer(store.NewStore())
	tmpDir := t.TempDir()

	xlsxPath := filepath.Join(tmpDir, "multi_sheet.xlsx")
	f := excelize.NewFile()

	// Default sheet (Sheet1) has the "wrong" headers
	defaultSheet := f.GetSheetName(0)
	f.SetCellValue(defaultSheet, "A1", "WrongA")
	f.SetCellValue(defaultSheet, "B1", "WrongB")

	// Second sheet "Customers" has the correct headers
	f.NewSheet("Customers")
	f.SetCellValue("Customers", "A1", "RightA")
	f.SetCellValue("Customers", "B1", "RightB")
	f.SetCellValue("Customers", "C1", "RightC")

	require.NoError(t, f.SaveAs(xlsxPath))
	require.NoError(t, f.Close())

	srcFile, err := os.Open(xlsxPath)
	require.NoError(t, err)
	defer srcFile.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "multi_sheet.xlsx")
	require.NoError(t, err)
	_, err = io.Copy(part, srcFile)
	require.NoError(t, err)

	// Select the "Customers" sheet via the optional form field
	require.NoError(t, writer.WriteField("sheet", "Customers"))

	writer.Close()

	req := httptest.NewRequest("POST", "/api/connector/introspect/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	server.HandleIntrospectUploadedFile(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	require.Equal(t, "success", resp["status"])

	columns, ok := resp["columns"].([]interface{})
	require.True(t, ok, "columns should be a slice")
	require.Len(t, columns, 3)

	expected := []string{"RightA", "RightB", "RightC"}
	for i, want := range expected {
		colMap, ok := columns[i].(map[string]interface{})
		require.True(t, ok, "column entry should be a map")
		require.Equal(t, want, colMap["name"], "column %d should come from Customers sheet", i)
	}
}

func TestIntrospectUploadedFileRejectsUnsupportedExtension(t *testing.T) {
	server := NewServer(store.NewStore())

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "payload.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("this is not a csv or excel file"))
	require.NoError(t, err)

	writer.Close()

	req := httptest.NewRequest("POST", "/api/connector/introspect/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	server.HandleIntrospectUploadedFile(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), ".csv")
	require.Contains(t, w.Body.String(), ".xlsx")
}

func TestIntrospectUploadedFileRequiresFileField(t *testing.T) {
	server := NewServer(store.NewStore())

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Only write a "sheet" field; no "file" part at all
	require.NoError(t, writer.WriteField("sheet", "x"))

	writer.Close()

	req := httptest.NewRequest("POST", "/api/connector/introspect/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	server.HandleIntrospectUploadedFile(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "file is required")
}

func TestIntrospectUploadedFileNeedsNoConnectorFileRoot(t *testing.T) {
	// This endpoint reads request bytes directly, so the CONNECTOR_FILE_ROOT
	// confinement that guards HandleIntrospectSchema deliberately does not apply here.
	t.Setenv(ConnectorFileRootEnv, "")

	server := NewServer(store.NewStore())

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "customers.csv")
	require.NoError(t, err)
	_, err = part.Write([]byte("id,customer_name,tax_id\n1,Acme Corp,XYZ123"))
	require.NoError(t, err)

	writer.Close()

	req := httptest.NewRequest("POST", "/api/connector/introspect/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	server.HandleIntrospectUploadedFile(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "server-side file paths are disabled")

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "success", resp["status"])
	require.Equal(t, "CSV", resp["type"])
}

func TestIntrospectUploadedFileRejectsNonPost(t *testing.T) {
	server := NewServer(store.NewStore())

	req := httptest.NewRequest("GET", "/api/connector/introspect/upload", nil)

	w := httptest.NewRecorder()
	server.HandleIntrospectUploadedFile(w, req)

	require.Equal(t, http.StatusMethodNotAllowed, w.Code)
}
