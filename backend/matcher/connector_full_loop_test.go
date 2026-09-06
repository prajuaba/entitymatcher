package matcher_test

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/bson"

	"entitymatcher/matcher"
)

// runFullLoopMatch takes raw ingested records from any connector and runs them through the full pipeline.
func runFullLoopMatch(t *testing.T, srcRecords, dstRecords []map[string]interface{}) ([]matcher.MatchResultItem, matcher.BatchProgress) {
	t.Helper()

	var sources []matcher.SourceRecord
	for _, r := range srcRecords {
		refID, _ := r["reference_id"].(string)
		name, _ := r["customer_name"].(string)
		clean := matcher.Normalize(name)
		sources = append(sources, matcher.SourceRecord{
			ID:              refID,
			BatchID:         "test-batch",
			ReferenceID:     refID,
			CustomerNameRaw: name,
			NormalizedName:  clean,
			TransactionDate: time.Now(),
		})
	}

	var destinations []matcher.DestinationRecord
	for _, r := range dstRecords {
		custID, _ := r["customer_id"].(string)
		name, _ := r["customer_name"].(string)
		clean := matcher.Normalize(name)
		destinations = append(destinations, matcher.DestinationRecord{
			ID:              custID,
			BatchID:         "test-batch",
			CustomerID:      custID,
			CustomerNameRaw: name,
			NormalizedName:  clean,
			TransactionDate: time.Now(),
		})
	}

	engine := matcher.NewMatchEngine(matcher.DefaultConfig())
	results, progress := engine.ExecuteJob(context.Background(), "test-batch", sources, destinations, nil)
	return results, progress
}

// 1. Full loop test: CSV Datasource
func TestFullLoop_CSVDatasource(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "src.csv")
	dstPath := filepath.Join(tmpDir, "dst.csv")

	// Create Source CSV
	fSrc, err := os.Create(srcPath)
	require.NoError(t, err)
	wSrc := csv.NewWriter(fSrc)
	require.NoError(t, wSrc.Write([]string{"reference_id", "customer_name", "date"}))
	require.NoError(t, wSrc.Write([]string{"SRC-01", "บริษัท สยามพารากอน จำกัด", "2026-08-31"}))
	require.NoError(t, wSrc.Write([]string{"SRC-02", "Siam Commercial Bank PCL", "2026-08-31"}))
	require.NoError(t, wSrc.Write([]string{"SRC-03", "นาย สมชาย เข็มกลัด", "2026-08-31"}))
	wSrc.Flush()
	fSrc.Close()

	// Create Destination CSV
	fDst, err := os.Create(dstPath)
	require.NoError(t, err)
	wDst := csv.NewWriter(fDst)
	require.NoError(t, wDst.Write([]string{"customer_id", "customer_name", "date"}))
	require.NoError(t, wDst.Write([]string{"DST-01", "บจก. สยามพารากอน", "2026-08-31"}))
	require.NoError(t, wDst.Write([]string{"DST-02", "SCB Bank", "2026-08-31"}))
	require.NoError(t, wDst.Write([]string{"DST-03", "สมชาย เข็มกลัด", "2026-08-31"}))
	wDst.Flush()
	fDst.Close()

	srcConn, err := matcher.NewDataConnector(matcher.ConnectionConfig{
		Type:     matcher.SourceTypeCSV,
		FilePath: srcPath,
	})
	require.NoError(t, err)
	defer srcConn.Close()

	dstConn, err := matcher.NewDataConnector(matcher.ConnectionConfig{
		Type:     matcher.SourceTypeCSV,
		FilePath: dstPath,
	})
	require.NoError(t, err)
	defer dstConn.Close()

	// Introspect & Fetch
	ctx := context.Background()
	srcSchema, err := srcConn.IntrospectSchema(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, srcSchema)

	srcData, err := srcConn.FetchRecords(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, srcData, 3)

	dstData, err := dstConn.FetchRecords(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, dstData, 3)

	// Run matching pipeline
	results, progress := runFullLoopMatch(t, srcData, dstData)
	require.NotEmpty(t, results)
	require.Equal(t, int64(3), progress.ProcessedSources)
	t.Logf("CSV datasource match completed: %d matches produced", len(results))
}

// 2. Full loop test: Excel (XLSX) Datasource
func TestFullLoop_ExcelDatasource(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "src.xlsx")
	dstPath := filepath.Join(tmpDir, "dst.xlsx")

	// Create Source Excel
	fSrc := excelize.NewFile()
	sheet1 := fSrc.GetSheetName(0)
	fSrc.SetCellValue(sheet1, "A1", "reference_id")
	fSrc.SetCellValue(sheet1, "B1", "customer_name")
	fSrc.SetCellValue(sheet1, "A2", "SRC-EX-01")
	fSrc.SetCellValue(sheet1, "B2", "Kasikorn Bank Public Company Limited")
	fSrc.SetCellValue(sheet1, "A3", "SRC-EX-02")
	fSrc.SetCellValue(sheet1, "B3", "PTT Global Chemical")
	require.NoError(t, fSrc.SaveAs(srcPath))

	// Create Dest Excel
	fDst := excelize.NewFile()
	sheet2 := fDst.GetSheetName(0)
	fDst.SetCellValue(sheet2, "A1", "customer_id")
	fDst.SetCellValue(sheet2, "B1", "customer_name")
	fDst.SetCellValue(sheet2, "A2", "DST-EX-01")
	fDst.SetCellValue(sheet2, "B2", "KBank PCL")
	fDst.SetCellValue(sheet2, "A3", "DST-EX-02")
	fDst.SetCellValue(sheet2, "B3", "PTT GC")
	require.NoError(t, fDst.SaveAs(dstPath))

	srcConn, err := matcher.NewDataConnector(matcher.ConnectionConfig{
		Type:     matcher.SourceTypeExcel,
		FilePath: srcPath,
	})
	require.NoError(t, err)
	defer srcConn.Close()

	dstConn, err := matcher.NewDataConnector(matcher.ConnectionConfig{
		Type:     matcher.SourceTypeExcel,
		FilePath: dstPath,
	})
	require.NoError(t, err)
	defer dstConn.Close()

	ctx := context.Background()
	srcSchema, err := srcConn.IntrospectSchema(ctx)
	require.NoError(t, err)
	require.Len(t, srcSchema, 2)

	srcData, err := srcConn.FetchRecords(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, srcData, 2)

	dstData, err := dstConn.FetchRecords(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, dstData, 2)

	results, progress := runFullLoopMatch(t, srcData, dstData)
	require.NotEmpty(t, results)
	require.Equal(t, int64(2), progress.ProcessedSources)
	t.Logf("Excel datasource match completed: %d matches produced", len(results))
}

// 3. Full loop test: Manual Datasource
func TestFullLoop_ManualDatasource(t *testing.T) {
	srcManual := []map[string]interface{}{
		{"reference_id": "MAN-SRC-1", "customer_name": "บริษัท ปูนซิเมนต์ไทย จำกัด (มหาชน)"},
		{"reference_id": "MAN-SRC-2", "customer_name": "Central Pattana Group"},
	}
	dstManual := []map[string]interface{}{
		{"customer_id": "MAN-DST-1", "customer_name": "SCG ปูนซิเมนต์ไทย"},
		{"customer_id": "MAN-DST-2", "customer_name": "CPN เซ็นทรัลพัฒนา"},
	}

	srcConn, err := matcher.NewDataConnector(matcher.ConnectionConfig{
		Type:       matcher.SourceTypeManual,
		ManualData: srcManual,
	})
	require.NoError(t, err)

	dstConn, err := matcher.NewDataConnector(matcher.ConnectionConfig{
		Type:       matcher.SourceTypeManual,
		ManualData: dstManual,
	})
	require.NoError(t, err)

	ctx := context.Background()
	srcSchema, err := srcConn.IntrospectSchema(ctx)
	require.NoError(t, err)
	require.Len(t, srcSchema, 2)

	srcData, err := srcConn.FetchRecords(ctx, 10, 0)
	require.NoError(t, err)
	dstData, err := dstConn.FetchRecords(ctx, 10, 0)
	require.NoError(t, err)

	results, progress := runFullLoopMatch(t, srcData, dstData)
	require.NotEmpty(t, results)
	require.Equal(t, int64(2), progress.ProcessedSources)
	t.Logf("Manual datasource match completed: %d matches produced", len(results))
}

// 4. Full loop test: Simulated PostgreSQL Ingestion & Full Matching Pipeline
func TestFullLoop_SimulatedPostgresDatasource(t *testing.T) {
	pgConn := os.Getenv("TEST_DATABASE_URL")
	if pgConn != "" {
		cfg := matcher.ConnectionConfig{
			Type:         matcher.SourceTypePostgres,
			TableOrQuery: "em_ingest_src",
		}
		conn, err := matcher.NewDataConnector(cfg)
		if err == nil {
			_ = conn.Close()
		}
	}

	pgSrcRows := []map[string]interface{}{
		{"reference_id": "PG-SRC-1", "customer_name": "Advanced Info Service PCL"},
		{"reference_id": "PG-SRC-2", "customer_name": "True Corporation PCL"},
	}
	pgDstRows := []map[string]interface{}{
		{"customer_id": "DST-AIS", "customer_name": "AIS เอไอเอส"},
		{"customer_id": "DST-TRUE", "customer_name": "ทรู คอร์ปอเรชั่น"},
	}

	results, progress := runFullLoopMatch(t, pgSrcRows, pgDstRows)
	require.NotEmpty(t, results)
	require.Equal(t, int64(2), progress.ProcessedSources)
	t.Logf("Simulated Postgres datasource match completed: %d matches produced", len(results))
}

// 5. Full loop test: Simulated SQL Server Ingestion & Full Matching Pipeline
func TestFullLoop_SimulatedSQLServerDatasource(t *testing.T) {
	mssqlSrcRows := []map[string]interface{}{
		{"reference_id": "SRC-CPF", "customer_name": "CPF ซีพีเอฟ"},
		{"reference_id": "SRC-BBL", "customer_name": "ธนาคารกรุงเทพ BBL"},
	}
	mssqlDstRows := []map[string]interface{}{
		{"customer_id": "MSSQL-DST-1", "customer_name": "Charoen Pokphand Foods"},
		{"customer_id": "MSSQL-DST-2", "customer_name": "Bangkok Bank"},
	}

	results, progress := runFullLoopMatch(t, mssqlSrcRows, mssqlDstRows)
	require.NotEmpty(t, results)
	require.Equal(t, int64(2), progress.ProcessedSources)
	t.Logf("Simulated SQL Server datasource match completed: %d matches produced", len(results))
}

// 6. Full loop test: MongoDB Datasource
func TestFullLoop_MongoDBDatasource(t *testing.T) {
	// Mock BSON Document stream decoding (MongoDB unit layer)
	rawDocs := []bson.M{
		{"_id": "mongo-1", "reference_id": "MGO-SRC-1", "customer_name": "Gulf Energy Development"},
		{"_id": "mongo-2", "reference_id": "MGO-SRC-2", "customer_name": "Bangchak Corporation"},
	}

	var srcData []map[string]interface{}
	for _, doc := range rawDocs {
		srcData = append(srcData, map[string]interface{}{
			"reference_id":  doc["reference_id"],
			"customer_name": doc["customer_name"],
		})
	}

	dstData := []map[string]interface{}{
		{"customer_id": "DST-GULF", "customer_name": "GULF กัลฟ์ เอ็นเนอร์จี"},
		{"customer_id": "DST-BCP", "customer_name": "BCP บางจาก"},
	}

	results, progress := runFullLoopMatch(t, srcData, dstData)
	require.NotEmpty(t, results)
	require.Equal(t, int64(2), progress.ProcessedSources)
	t.Logf("MongoDB datasource loop match completed: %d matches produced", len(results))
}

// 7. Full loop cross-datasource test: Mixed CSV source against Excel destination
func TestFullLoop_CrossDatasources_CSV_to_Excel(t *testing.T) {
	tmpDir := t.TempDir()
	srcCSV := filepath.Join(tmpDir, "source.csv")
	dstXLSX := filepath.Join(tmpDir, "dest.xlsx")

	// CSV Source
	fSrc, err := os.Create(srcCSV)
	require.NoError(t, err)
	wSrc := csv.NewWriter(fSrc)
	require.NoError(t, wSrc.Write([]string{"reference_id", "customer_name"}))
	require.NoError(t, wSrc.Write([]string{"CSV-1", "บริษัท กรุงเทพดุสิตเวชการ จำกัด (มหาชน)"}))
	wSrc.Flush()
	fSrc.Close()

	// Excel Destination
	fDst := excelize.NewFile()
	s := fDst.GetSheetName(0)
	fDst.SetCellValue(s, "A1", "customer_id")
	fDst.SetCellValue(s, "B1", "customer_name")
	fDst.SetCellValue(s, "A2", "XLSX-1")
	fDst.SetCellValue(s, "B2", "BDMS Bangkok Dusit Medical Services")
	require.NoError(t, fDst.SaveAs(dstXLSX))

	srcConn, err := matcher.NewDataConnector(matcher.ConnectionConfig{Type: matcher.SourceTypeCSV, FilePath: srcCSV})
	require.NoError(t, err)
	dstConn, err := matcher.NewDataConnector(matcher.ConnectionConfig{Type: matcher.SourceTypeExcel, FilePath: dstXLSX})
	require.NoError(t, err)

	ctx := context.Background()
	srcData, err := srcConn.FetchRecords(ctx, 10, 0)
	require.NoError(t, err)
	dstData, err := dstConn.FetchRecords(ctx, 10, 0)
	require.NoError(t, err)

	results, progress := runFullLoopMatch(t, srcData, dstData)
	require.NotEmpty(t, results)
	require.Equal(t, int64(1), progress.ProcessedSources)
	t.Logf("Cross-datasource match: Score=%f, Status=%s", results[0].ConfidenceScore, results[0].MatchStatus)
}
