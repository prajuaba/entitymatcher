package matcher

import (
	"context"
	"testing"
	"time"
)

func TestManyToManyAutoMatchesTiedSiblings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AssignmentStrategy = "ALL_CANDIDATES"
	cfg.AutoMatchThreshold = 0.90

	sources := []SourceRecord{
		{
			ID:              "src-1",
			BatchID:         "batch-id",
			ReferenceID:     "REF-001",
			CustomerNameRaw: "Bangkok Bank Public Company Limited",
			NormalizedName:  Normalize("Bangkok Bank Public Company Limited"),
			TransactionDate: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	dests := []DestinationRecord{
		{
			ID:              "dest-1",
			BatchID:         "batch-id",
			CustomerID:      "CUST-001",
			CustomerNameRaw: "Bangkok Bank Public Company Limited",
			NormalizedName:  Normalize("Bangkok Bank Public Company Limited"),
			TransactionDate: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			ID:              "dest-2",
			BatchID:         "batch-id",
			CustomerID:      "CUST-002",
			CustomerNameRaw: "Bangkok Bank Public Company Limited",
			NormalizedName:  Normalize("Bangkok Bank Public Company Limited"),
			TransactionDate: time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC),
		},
	}

	engine := NewMatchEngine(cfg)
	results, progress := engine.ExecuteJob(context.Background(), "batch-id", sources, dests, nil)

	if progress.AutoMatched != 2 {
		t.Errorf("Expected 2 auto-matched items, got %d", progress.AutoMatched)
	}

	// Verify both destinations are auto-matched
	foundDest1 := false
	foundDest2 := false

	for _, item := range results {
		if item.SourceID == "src-1" {
			if item.DestinationID == "dest-1" && item.MatchStatus == "AUTO_MATCHED" {
				foundDest1 = true
			}
			if item.DestinationID == "dest-2" && item.MatchStatus == "AUTO_MATCHED" {
				foundDest2 = true
			}
		}
	}

	if !foundDest1 {
		t.Errorf("Expected AUTO_MATCHED result for dest-1, but not found")
	}
	if !foundDest2 {
		t.Errorf("Expected AUTO_MATCHED result for dest-2, but not found")
	}
}

func TestGreedyStillDowngradesTiedSiblings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AssignmentStrategy = "GREEDY_1_1"
	cfg.AutoMatchThreshold = 0.90

	sources := []SourceRecord{
		{
			ID:              "src-1",
			BatchID:         "batch-id",
			ReferenceID:     "REF-001",
			CustomerNameRaw: "Bangkok Bank Public Company Limited",
			NormalizedName:  Normalize("Bangkok Bank Public Company Limited"),
			TransactionDate: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	dests := []DestinationRecord{
		{
			ID:              "dest-1",
			BatchID:         "batch-id",
			CustomerID:      "CUST-001",
			CustomerNameRaw: "Bangkok Bank Public Company Limited",
			NormalizedName:  Normalize("Bangkok Bank Public Company Limited"),
			TransactionDate: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			ID:              "dest-2",
			BatchID:         "batch-id",
			CustomerID:      "CUST-002",
			CustomerNameRaw: "Bangkok Bank Public Company Limited",
			NormalizedName:  Normalize("Bangkok Bank Public Company Limited"),
			TransactionDate: time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC),
		},
	}

	engine := NewMatchEngine(cfg)
	results, _ := engine.ExecuteJob(context.Background(), "batch-id", sources, dests, nil)

	autoMatchCount := 0
	for _, item := range results {
		if item.SourceID == "src-1" && item.MatchStatus == "AUTO_MATCHED" {
			autoMatchCount++
		}
	}

	if autoMatchCount != 1 {
		t.Errorf("Expected exactly 1 AUTO_MATCHED item, got %d", autoMatchCount)
	}
}

func TestManyToManyStillRespectsThreshold(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AssignmentStrategy = "ALL_CANDIDATES"
	cfg.AutoMatchThreshold = 0.90
	cfg.ReviewThreshold = 0.10

	sources := []SourceRecord{
		{
			ID:              "src-1",
			BatchID:         "batch-id",
			ReferenceID:     "REF-001",
			CustomerNameRaw: "Bangkok Bank Public Company Limited",
			NormalizedName:  Normalize("Bangkok Bank Public Company Limited"),
			TransactionDate: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	dests := []DestinationRecord{
		{
			ID:              "dest-1",
			BatchID:         "batch-id",
			CustomerID:      "CUST-001",
			CustomerNameRaw: "Zebra Quantum Industries Corp",
			NormalizedName:  Normalize("Zebra Quantum Industries Corp"),
			TransactionDate: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	engine := NewMatchEngine(cfg)
	_, progress := engine.ExecuteJob(context.Background(), "batch-id", sources, dests, nil)

	if progress.AutoMatched != 0 {
		t.Errorf("Expected no auto-matched items due to score below threshold, got %d", progress.AutoMatched)
	}
}

func TestManyToManyOneApplicationManyCustomers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AssignmentStrategy = "ALL_CANDIDATES"
	cfg.AutoMatchThreshold = 0.90

	sources := []SourceRecord{
		{
			ID:              "src-1",
			BatchID:         "batch-id",
			ReferenceID:     "REF-001",
			CustomerNameRaw: "Bangkok Bank Public Company Limited",
			NormalizedName:  Normalize("Bangkok Bank Public Company Limited"),
			TransactionDate: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			ID:              "src-2",
			BatchID:         "batch-id",
			ReferenceID:     "REF-002",
			CustomerNameRaw: "Bangkok Bank Public Company Limited",
			NormalizedName:  Normalize("Bangkok Bank Public Company Limited"),
			TransactionDate: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	dests := []DestinationRecord{
		{
			ID:              "dest-1",
			BatchID:         "batch-id",
			CustomerID:      "CUST-001",
			CustomerNameRaw: "Bangkok Bank Public Company Limited",
			NormalizedName:  Normalize("Bangkok Bank Public Company Limited"),
			TransactionDate: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	engine := NewMatchEngine(cfg)
	results, progress := engine.ExecuteJob(context.Background(), "batch-id", sources, dests, nil)

	if progress.AutoMatched != 2 {
		t.Errorf("Expected 2 auto-matched items, got %d", progress.AutoMatched)
	}

	// Verify both sources are auto-matched to the same destination
	foundSrc1 := false
	foundSrc2 := false

	for _, item := range results {
		if item.SourceID == "src-1" && item.DestinationID == "dest-1" && item.MatchStatus == "AUTO_MATCHED" {
			foundSrc1 = true
		}
		if item.SourceID == "src-2" && item.DestinationID == "dest-1" && item.MatchStatus == "AUTO_MATCHED" {
			foundSrc2 = true
		}
	}

	if !foundSrc1 {
		t.Errorf("Expected AUTO_MATCHED result for src-1 -> dest-1, but not found")
	}
	if !foundSrc2 {
		t.Errorf("Expected AUTO_MATCHED result for src-2 -> dest-1, but not found")
	}
}

func TestIsManyToManyOnlyForAllCandidates(t *testing.T) {
	tests := []struct {
		strategy string
		expected bool
	}{
		{"ALL_CANDIDATES", true},
		{"GREEDY_1_1", false},
		{"TOP_1", false},
		{"", false},
		{"SOME_UNKNOWN_STRATEGY", false},
	}

	for _, tt := range tests {
		t.Run(tt.strategy, func(t *testing.T) {
			result := isManyToMany(tt.strategy)
			if result != tt.expected {
				t.Errorf("isManyToMany(%q) = %v; want %v", tt.strategy, result, tt.expected)
			}
		})
	}
}
