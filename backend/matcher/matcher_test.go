package matcher

import (
	"context"
	"testing"
	"time"
)

func TestNormalizer(t *testing.T) {
	tests := []struct {
		input          string
		expectedClean  string
		expectedSorted string
	}{
		{
			input:          "บริษัท สยามพารากอน ดีเวลลอปเม้นท์ จำกัด",
			expectedClean:  "สยามพารากอน ดีเวลลอปเม้นท์",
			expectedSorted: "ดีเวลลอปเม้นท์ สยามพารากอน",
		},
		{
			input:          "นาย สมชาย เข็มกลัด",
			expectedClean:  "สมชาย เข็มกลัด",
			expectedSorted: "สมชาย เข็มกลัด",
		},
		{
			input:          "Bangkok Bank Public Company Limited",
			expectedClean:  "bangkok bank",
			expectedSorted: "bangkok bank",
		},
		{
			input:          "John Michael Smith",
			expectedClean:  "john michael smith",
			expectedSorted: "john michael smith",
		},
	}

	for _, tt := range tests {
		res := Normalize(tt.input)
		if res.Cleaned != tt.expectedClean {
			t.Errorf("Normalize(%q).Cleaned = %q; want %q", tt.input, res.Cleaned, tt.expectedClean)
		}
		if res.SortedTokens != tt.expectedSorted {
			t.Errorf("Normalize(%q).SortedTokens = %q; want %q", tt.input, res.SortedTokens, tt.expectedSorted)
		}
	}
}

func TestJaroWinklerRuneSafety(t *testing.T) {
	// Name transposition in Thai: First Last vs Last First
	s1 := "สมชาย เข็มกลัด"
	s2 := "เข็มกลัด สมชาย"

	// Raw Jaro-Winkler vs Token-sorted Jaro-Winkler
	norm1 := Normalize(s1)
	norm2 := Normalize(s2)

	scoreSorted := JaroWinkler(norm1.SortedTokens, norm2.SortedTokens)
	if scoreSorted != 1.0 {
		t.Errorf("Expected token-sorted Jaro-Winkler to be 1.0 for transposed Thai names, got %f", scoreSorted)
	}
}

func TestMatchEngineExecution(t *testing.T) {
	sources := []SourceRecord{
		{
			ID:              "src-1",
			BatchID:         "test-batch",
			ReferenceID:     "REF-001",
			CustomerNameRaw: "บริษัท สยามพารากอน ดีเวลลอปเม้นท์ จำกัด",
			NormalizedName:  Normalize("บริษัท สยามพารากอน ดีเวลลอปเม้นท์ จำกัด"),
			TransactionDate: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	dests := []DestinationRecord{
		{
			ID:              "dest-1",
			BatchID:         "test-batch",
			CustomerID:      "CUST-001",
			CustomerNameRaw: "สยามพารากอน ดีเวลลอปเม้นท์ บจก.",
			NormalizedName:  Normalize("สยามพารากอน ดีเวลลอปเม้นท์ บจก."),
			TransactionDate: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			ID:              "dest-2",
			BatchID:         "test-batch",
			CustomerID:      "CUST-002",
			CustomerNameRaw: "เซ็นทรัล พลาซา จำกัด",
			NormalizedName:  Normalize("เซ็นทรัล พลาซา จำกัด"),
			TransactionDate: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	engine := NewMatchEngine(DefaultConfig())
	results, progress := engine.ExecuteJob(context.Background(), "test-batch", sources, dests, nil)

	if progress.TotalMatches == 0 {
		t.Fatalf("Expected at least 1 match, got 0")
	}

	if results[0].DestinationID != "dest-1" {
		t.Errorf("Expected top match destination to be dest-1, got %s", results[0].DestinationID)
	}

	if results[0].ConfidenceScore < 0.90 {
		t.Errorf("Expected high confidence score (>=0.90), got %f", results[0].ConfidenceScore)
	}
}
