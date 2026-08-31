package matcher

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestExcelFetchRecordsPaging(t *testing.T) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	// Write header
	f.SetCellValue(sheet, "A1", "id")
	f.SetCellValue(sheet, "B1", "name")

	// Write 100 data rows
	for i := 0; i < 100; i++ {
		row := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("%d", i))
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), fmt.Sprintf("name-%d", i))
	}

	path := filepath.Join(t.TempDir(), "test_paging.xlsx")
	require.NoError(t, f.SaveAs(path))

	connector := &ExcelConnector{
		Config: ConnectionConfig{
			Type:     SourceTypeExcel,
			FilePath: path,
		},
	}
	ctx := context.Background()

	// Test basic pagination
	for offset := 0; offset < 100; offset += 10 {
		result, err := connector.FetchRecords(ctx, 10, offset)
		require.NoError(t, err, "FetchRecords at offset %d failed", offset)
		require.Len(t, result, 10, "expected 10 records at offset %d", offset)

		expectedStart := offset
		for i, row := range result {
			expectedID := fmt.Sprintf("%d", expectedStart+i)
			require.Equal(t, expectedID, row["id"], "id mismatch at offset %d, index %d", offset, i)
		}
	}

	// Test partial page at end
	result, err := connector.FetchRecords(ctx, 10, 95)
	require.NoError(t, err)
	require.Len(t, result, 5, "expected 5 records at offset 95")

	// Union-of-pages check
	counts := make(map[string]int)
	for offset := 0; offset < 100; offset += 10 {
		result, err := connector.FetchRecords(ctx, 10, offset)
		require.NoError(t, err)
		for _, row := range result {
			id := row["id"].(string)
			counts[id]++
		}
	}

	require.Len(t, counts, 100, "expected exactly 100 distinct IDs")
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("%d", i)
		require.Equal(t, 1, counts[id], "ID %s appears %d times, expected 1", id, counts[id])
	}
}

func TestExcelFetchRecordsShortRowOmitsKey(t *testing.T) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	// Write header
	require.NoError(t, f.SetCellValue(sheet, "A1", "a"))
	require.NoError(t, f.SetCellValue(sheet, "B1", "b"))
	require.NoError(t, f.SetCellValue(sheet, "C1", "c"))

	// Write one data row with only two cells populated
	require.NoError(t, f.SetCellValue(sheet, "A2", "x1"))
	require.NoError(t, f.SetCellValue(sheet, "B2", "y1"))
	// Intentionally leave C2 unset

	path := filepath.Join(t.TempDir(), "test_short_row.xlsx")
	require.NoError(t, f.SaveAs(path))

	connector := &ExcelConnector{
		Config: ConnectionConfig{
			Type:     SourceTypeExcel,
			FilePath: path,
		},
	}
	ctx := context.Background()

	result, err := connector.FetchRecords(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)

	// This pins the pre-existing contract that a short row omits the key rather than emptying it
	require.Len(t, result[0], 2, "expected exactly 2 keys in the map")
	require.Equal(t, "x1", result[0]["a"])
	require.Equal(t, "y1", result[0]["b"])
	require.NotContains(t, result[0], "c")
}

func TestExcelFetchRecordsEmptySheet(t *testing.T) {
	f := excelize.NewFile()
	_ = f.GetSheetName(0)

	// Do not write any cells - sheet is completely empty
	path := filepath.Join(t.TempDir(), "test_empty.xlsx")
	require.NoError(t, f.SaveAs(path))

	connector := &ExcelConnector{
		Config: ConnectionConfig{
			Type:     SourceTypeExcel,
			FilePath: path,
		},
	}
	ctx := context.Background()

	result, err := connector.FetchRecords(ctx, 10, 0)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, result)
}

func TestExcelFetchRecordsHeaderOnly(t *testing.T) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	// Write header only
	require.NoError(t, f.SetCellValue(sheet, "A1", "id"))
	require.NoError(t, f.SetCellValue(sheet, "B1", "name"))
	// No data rows

	path := filepath.Join(t.TempDir(), "test_header_only.xlsx")
	require.NoError(t, f.SaveAs(path))

	connector := &ExcelConnector{
		Config: ConnectionConfig{
			Type:     SourceTypeExcel,
			FilePath: path,
		},
	}
	ctx := context.Background()

	result, err := connector.FetchRecords(ctx, 10, 0)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, result)
}

func TestExcelIntrospectReadsHeaderOnly(t *testing.T) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	// Write header row with spaces around 'id' to test trimming
	require.NoError(t, f.SetCellValue(sheet, "A1", "  id  "))
	require.NoError(t, f.SetCellValue(sheet, "B1", "name"))
	require.NoError(t, f.SetCellValue(sheet, "C1", "email"))

	// Write 500 data rows
	for i := 0; i < 500; i++ {
		row := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("%d", i))
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), "n")
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), "e")
	}

	path := filepath.Join(t.TempDir(), "test_introspect.xlsx")
	require.NoError(t, f.SaveAs(path))

	connector := &ExcelConnector{
		Config: ConnectionConfig{
			Type:     SourceTypeExcel,
			FilePath: path,
		},
	}
	ctx := context.Background()

	cols, err := connector.IntrospectSchema(ctx)
	require.NoError(t, err)
	require.Len(t, cols, 3)

	require.Equal(t, "id", cols[0].Name, "first column name mismatch")
	require.Equal(t, "name", cols[1].Name, "second column name mismatch")
	require.Equal(t, "email", cols[2].Name, "third column name mismatch")

	for _, col := range cols {
		require.Equal(t, "STRING", col.DataType, "data type mismatch")
	}
}

func measureExcelAlloc(t *testing.T, cellValue func(i int) (string, string)) (smallAlloc, fullAlloc uint64) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	require.NoError(t, f.SetCellValue(sheet, "A1", "id"))
	require.NoError(t, f.SetCellValue(sheet, "B1", "name"))

	for i := 0; i < 20000; i++ {
		a, b := cellValue(i)
		row := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), a)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), b)
	}

	path := filepath.Join(t.TempDir(), "test_alloc.xlsx")
	require.NoError(t, f.SaveAs(path))

	connector := &ExcelConnector{
		Config: ConnectionConfig{
			Type:     SourceTypeExcel,
			FilePath: path,
		},
	}
	ctx := context.Background()

	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	_, err := connector.FetchRecords(ctx, 5, 0)
	require.NoError(t, err)
	runtime.ReadMemStats(&m2)
	smallAlloc = m2.TotalAlloc - m1.TotalAlloc

	runtime.GC()
	var m3, m4 runtime.MemStats
	runtime.ReadMemStats(&m3)
	result, err := connector.FetchRecords(ctx, 20000, 0)
	require.NoError(t, err)
	require.Len(t, result, 20000)
	runtime.ReadMemStats(&m4)
	fullAlloc = m4.TotalAlloc - m3.TotalAlloc

	return smallAlloc, fullAlloc
}

func TestExcelPagedReadDoesNotScaleWithSheetSize(t *testing.T) {
	t.Run("repeated_cell_values", func(t *testing.T) {
		smallAlloc, fullAlloc := measureExcelAlloc(t, func(i int) (string, string) {
			return "x", "y"
		})
		t.Logf("repeated_cell_values: smallAlloc=%d bytes, fullAlloc=%d bytes", smallAlloc, fullAlloc)
		// A tiny shared-string table isolates the per-row decoding cost that streaming actually removes.
		require.Less(t, smallAlloc, fullAlloc/2,
			fmt.Sprintf("a five-row read allocating as much as a twenty-thousand-row read (smallAlloc=%d fullAlloc=%d) means the sheet is still being materialized up front instead of streamed", smallAlloc, fullAlloc))
	})

	t.Run("unique_cell_values", func(t *testing.T) {
		smallAlloc, fullAlloc := measureExcelAlloc(t, func(i int) (string, string) {
			return fmt.Sprintf("id-%d", i), fmt.Sprintf("Customer Name %d", i)
		})
		t.Logf("unique_cell_values: smallAlloc=%d bytes, fullAlloc=%d bytes", smallAlloc, fullAlloc)
		// excelize parses the shared-string table up front and streaming cannot avoid that fixed cost,
		// so on text-heavy realistic sheets streaming roughly halves allocations rather than eliminating them.
		// This bound is deliberately weaker than the repeated-value case because it must hold for real data,
		// and reverting to GetRows makes the two reads allocate the same and fails this bound too.
		require.Less(t, smallAlloc, (fullAlloc*3)/4,
			fmt.Sprintf("a five-row read allocating 75%%+ of a twenty-thousand-row read (smallAlloc=%d fullAlloc=%d) means the sheet is still being materialized up front instead of streamed", smallAlloc, fullAlloc))
	})
}
