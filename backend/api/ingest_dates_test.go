package api

import (
	"testing"
	"time"

	"entitymatcher/matcher"
)

func TestBuildSourceRecordsParsesFlexibleDates(t *testing.T) {
	cfg := matcher.Config{
		ColumnMapping: matcher.ColumnMapping{
			NameFieldsSrc:  []string{"customer_name"},
			NameFieldsDest: []string{"customer_name"},
			RefIDSrc:       "reference_id",
			RefIDDest:      "customer_id",
			DateFieldSrc:   "txn_date",
			DateFieldDest:  "txn_date",
		},
	}

	rows := []map[string]interface{}{
		{
			"reference_id":  "R001",
			"customer_name": "Acme Corp",
			"txn_date":      "2024-01-15",
		},
		{
			"reference_id":  "R002",
			"customer_name": "Beta LLC",
			"txn_date":      "15/01/2024",
		},
		{
			"reference_id":  "R003",
			"customer_name": "Gamma Inc",
			"txn_date":      "2567-01-15",
		},
		{
			"reference_id":  "R004",
			"customer_name": "Delta Ltd",
			"txn_date":      "๑๕/๐๑/๒๕๖๗",
		},
	}

	records := buildSourceRecords(rows, "test-batch", cfg)

	expectedDate := time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC)
	for i, record := range records {
		if record.TransactionDate.Year() != expectedDate.Year() ||
			record.TransactionDate.Month() != expectedDate.Month() ||
			record.TransactionDate.Day() != expectedDate.Day() {
			t.Errorf("Record %d: expected date %v, got %v", i, expectedDate, record.TransactionDate)
		}
	}
}

func TestBuildDestinationRecordsParsesFlexibleDates(t *testing.T) {
	cfg := matcher.Config{
		ColumnMapping: matcher.ColumnMapping{
			NameFieldsSrc:  []string{"customer_name"},
			NameFieldsDest: []string{"customer_name"},
			RefIDSrc:       "reference_id",
			RefIDDest:      "customer_id",
			DateFieldSrc:   "txn_date",
			DateFieldDest:  "txn_date",
		},
	}

	rows := []map[string]interface{}{
		{
			"customer_id":   "D001",
			"customer_name": "Acme Corp",
			"txn_date":      "2024-01-15",
		},
		{
			"customer_id":   "D002",
			"customer_name": "Beta LLC",
			"txn_date":      "15/01/2024",
		},
		{
			"customer_id":   "D003",
			"customer_name": "Gamma Inc",
			"txn_date":      "2567-01-15",
		},
		{
			"customer_id":   "D004",
			"customer_name": "Delta Ltd",
			"txn_date":      "๑๕/๐๑/๒๕๖๗",
		},
	}

	records := buildDestinationRecords(rows, "test-batch", cfg)

	expectedDate := time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC)
	for i, record := range records {
		if record.TransactionDate.Year() != expectedDate.Year() ||
			record.TransactionDate.Month() != expectedDate.Month() ||
			record.TransactionDate.Day() != expectedDate.Day() {
			t.Errorf("Record %d: expected date %v, got %v", i, expectedDate, record.TransactionDate)
		}
	}
}

func TestBuildRecordsLeavesDateZeroWhenAbsent(t *testing.T) {
	cfg := matcher.Config{
		ColumnMapping: matcher.ColumnMapping{
			NameFieldsSrc:  []string{"customer_name"},
			NameFieldsDest: []string{"customer_name"},
			RefIDSrc:       "reference_id",
			RefIDDest:      "customer_id",
			DateFieldSrc:   "txn_date",
			DateFieldDest:  "txn_date",
		},
	}

	// Raw row without "txn_date" key
	row := map[string]interface{}{
		"reference_id":  "R001",
		"customer_name": "Acme Corp",
		// no txn_date key
	}

	srcRecords := buildSourceRecords([]map[string]interface{}{row}, "test-batch", cfg)
	if !srcRecords[0].TransactionDate.IsZero() {
		t.Errorf("absent source date must stay zero, got %v", srcRecords[0].TransactionDate)
	}

	// Same for destination
	destRow := map[string]interface{}{
		"customer_id":   "D001",
		"customer_name": "Acme Corp",
		// no txn_date key
	}

	destRecords := buildDestinationRecords([]map[string]interface{}{destRow}, "test-batch", cfg)
	if !destRecords[0].TransactionDate.IsZero() {
		t.Errorf("absent destination date must stay zero, got %v", destRecords[0].TransactionDate)
	}
	// An absent date MUST stay the zero value. These functions used to substitute
	// time.Now(), which was indistinguishable from a genuine same-day transaction:
	// CalculateCompositeScoreWithCorpus then scored the pair as a perfect date
	// match and handed it a free DateWeight of confidence. The scorer now relies
	// on IsZero to drop the date term, so fabricating a date here would silently
	// re-inflate every score.
}

func TestMaxIngestRecordsDefaultAndOverride(t *testing.T) {
	// Default
	t.Setenv(MaxIngestRecordsEnv, "")
	if got := resolveMaxIngestRecords(); got != defaultMaxIngestRecords {
		t.Errorf("Expected default %d, got %d", defaultMaxIngestRecords, got)
	}

	// Override with valid number
	t.Setenv(MaxIngestRecordsEnv, "1234")
	if got := resolveMaxIngestRecords(); got != 1234 {
		t.Errorf("Expected 1234, got %d", got)
	}

	// Override with garbage - should fall back to default
	t.Setenv(MaxIngestRecordsEnv, "abc")
	if got := resolveMaxIngestRecords(); got != defaultMaxIngestRecords {
		t.Errorf("Expected default %d, got %d", defaultMaxIngestRecords, got)
	}

	// Override with zero - should fall back to default
	t.Setenv(MaxIngestRecordsEnv, "0")
	if got := resolveMaxIngestRecords(); got != defaultMaxIngestRecords {
		t.Errorf("Expected default %d, got %d", defaultMaxIngestRecords, got)
	}
}
