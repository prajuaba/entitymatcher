package matcher

import (
	"context"
	"math"
	"testing"
	"time"
)

// --- Unit tests for EvaluateSecondaryFields ---

func TestEvaluateSecondaryFields_NoMappingsIsNeutralPass(t *testing.T) {
	score, reasons, ok := EvaluateSecondaryFields(
		map[string]interface{}{"tax_id": "123"},
		map[string]interface{}{"tax_id": "456"},
		nil,
	)
	if score != 1.0 || !ok || len(reasons) != 0 {
		t.Fatalf("expected neutral pass with no mappings, got score=%v ok=%v reasons=%v", score, ok, reasons)
	}
}

func TestEvaluateSecondaryFields_ExactMatch(t *testing.T) {
	mappings := []SecondaryFieldMapping{
		{Name: "Tax ID", FieldSrc: "tax_id", FieldDest: "tax_id", MatchType: "EXACT", Weight: 0.20},
	}
	score, reasons, ok := EvaluateSecondaryFields(
		map[string]interface{}{"tax_id": "123-45-6789"},
		map[string]interface{}{"tax_id": "123-45-6789"},
		mappings,
	)
	if score != 1.0 || !ok {
		t.Fatalf("expected exact match score=1.0 ok=true, got score=%v ok=%v", score, ok)
	}
	if len(reasons) != 1 {
		t.Fatalf("expected 1 reason describing the exact match, got %v", reasons)
	}
}

func TestEvaluateSecondaryFields_ExactMismatchNonMandatoryDoesNotExclude(t *testing.T) {
	mappings := []SecondaryFieldMapping{
		{Name: "Category", FieldSrc: "category", FieldDest: "category", MatchType: "EXACT", Weight: 0.20, IsMandatory: false},
	}
	score, _, ok := EvaluateSecondaryFields(
		map[string]interface{}{"category": "Retail"},
		map[string]interface{}{"category": "Wholesale"},
		mappings,
	)
	if !ok {
		t.Fatalf("non-mandatory mismatch must not exclude the candidate, got ok=%v", ok)
	}
	if score != 0.0 {
		t.Fatalf("expected mismatched field score 0.0, got %v", score)
	}
}

func TestEvaluateSecondaryFields_MandatoryMismatchExcludes(t *testing.T) {
	mappings := []SecondaryFieldMapping{
		{Name: "Tax ID", FieldSrc: "tax_id", FieldDest: "tax_id", MatchType: "EXACT", Weight: 0.20, IsMandatory: true},
	}
	_, reasons, ok := EvaluateSecondaryFields(
		map[string]interface{}{"tax_id": "111"},
		map[string]interface{}{"tax_id": "222"},
		mappings,
	)
	if ok {
		t.Fatalf("expected mandatory mismatch to signal exclusion (ok=false), got ok=%v", ok)
	}
	if len(reasons) != 1 {
		t.Fatalf("expected 1 reason describing the mandatory mismatch, got %v", reasons)
	}
}

func TestEvaluateSecondaryFields_MandatoryMatchDoesNotExclude(t *testing.T) {
	mappings := []SecondaryFieldMapping{
		{Name: "Tax ID", FieldSrc: "tax_id", FieldDest: "tax_id", MatchType: "EXACT", Weight: 0.20, IsMandatory: true},
	}
	_, _, ok := EvaluateSecondaryFields(
		map[string]interface{}{"tax_id": "999"},
		map[string]interface{}{"tax_id": "999"},
		mappings,
	)
	if !ok {
		t.Fatalf("a satisfied mandatory field must not exclude the candidate, got ok=%v", ok)
	}
}

func TestEvaluateSecondaryFields_Fuzzy(t *testing.T) {
	mappings := []SecondaryFieldMapping{
		{Name: "Category", FieldSrc: "category", FieldDest: "category", MatchType: "FUZZY", Weight: 0.15},
	}
	score, _, ok := EvaluateSecondaryFields(
		map[string]interface{}{"category": "Wholesale Trading Co"},
		map[string]interface{}{"category": "Wholesale Trading Company"},
		mappings,
	)
	if !ok {
		t.Fatalf("fuzzy evaluation should never exclude a candidate on its own, got ok=%v", ok)
	}
	if score <= 0.5 {
		t.Fatalf("expected high fuzzy similarity for near-identical strings, got %v", score)
	}
}

func TestEvaluateSecondaryFields_NumericDelta(t *testing.T) {
	tests := []struct {
		name          string
		src, dest     string
		expectMinimum float64
	}{
		{"exact", "1000.00", "1000.00", 1.0},
		{"withinOnePercent", "1000.00", "1000.05", 0.90},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappings := []SecondaryFieldMapping{
				{Name: "Amount", FieldSrc: "amount", FieldDest: "amount", MatchType: "NUMERIC_DELTA", Weight: 0.15},
			}
			score, _, ok := EvaluateSecondaryFields(
				map[string]interface{}{"amount": tt.src},
				map[string]interface{}{"amount": tt.dest},
				mappings,
			)
			if !ok {
				t.Fatalf("numeric delta evaluation should not exclude the candidate, got ok=%v", ok)
			}
			if score < tt.expectMinimum {
				t.Fatalf("expected score >= %v, got %v", tt.expectMinimum, score)
			}
		})
	}
}

func TestEvaluateSecondaryFields_NumericDeltaFarApartScoresZero(t *testing.T) {
	mappings := []SecondaryFieldMapping{
		{Name: "Amount", FieldSrc: "amount", FieldDest: "amount", MatchType: "NUMERIC_DELTA", Weight: 0.15},
	}
	score, _, ok := EvaluateSecondaryFields(
		map[string]interface{}{"amount": "1000"},
		map[string]interface{}{"amount": "50"},
		mappings,
	)
	if !ok {
		t.Fatalf("expected ok=true (non-mandatory), got ok=%v", ok)
	}
	if score != 0.0 {
		t.Fatalf("expected 0.0 for values far outside tolerance, got %v", score)
	}
}

func TestEvaluateSecondaryFields_MissingValueOnEitherSideSkipsField(t *testing.T) {
	mappings := []SecondaryFieldMapping{
		{Name: "Tax ID", FieldSrc: "tax_id", FieldDest: "tax_id", MatchType: "EXACT", Weight: 0.20},
	}
	// Source is missing the field entirely.
	score, reasons, ok := EvaluateSecondaryFields(
		map[string]interface{}{},
		map[string]interface{}{"tax_id": "123"},
		mappings,
	)
	if score != 1.0 || !ok || len(reasons) != 0 {
		t.Fatalf("expected neutral pass when a value is missing, got score=%v ok=%v reasons=%v", score, ok, reasons)
	}
}

func TestEvaluateSecondaryFields_NonPositiveWeightDefaultsTo010(t *testing.T) {
	// Two mismatched EXACT fields, one with an explicit positive weight and one
	// with a non-positive weight that must fall back to the documented 0.10
	// default rather than being dropped from the weighted average entirely.
	mappings := []SecondaryFieldMapping{
		{Name: "A", FieldSrc: "a", FieldDest: "a", MatchType: "EXACT", Weight: 0.30},
		{Name: "B", FieldSrc: "b", FieldDest: "b", MatchType: "EXACT", Weight: 0},
	}
	src := map[string]interface{}{"a": "match", "b": "match"}
	dest := map[string]interface{}{"a": "match", "b": "match"}
	score, _, ok := EvaluateSecondaryFields(src, dest, mappings)
	if !ok || math.Abs(score-1.0) > 1e-9 {
		t.Fatalf("expected both exact matches to average to 1.0, got score=%v ok=%v", score, ok)
	}
}

func TestEvaluateSecondaryFields_WeightedAverageAcrossMultipleFields(t *testing.T) {
	// Field A matches (weight 0.30), field B mismatches (weight 0.10):
	// expected = (1.0*0.30 + 0.0*0.10) / 0.40 = 0.75
	mappings := []SecondaryFieldMapping{
		{Name: "A", FieldSrc: "a", FieldDest: "a", MatchType: "EXACT", Weight: 0.30},
		{Name: "B", FieldSrc: "b", FieldDest: "b", MatchType: "EXACT", Weight: 0.10},
	}
	src := map[string]interface{}{"a": "match", "b": "mismatch-src"}
	dest := map[string]interface{}{"a": "match", "b": "mismatch-dest"}
	score, _, ok := EvaluateSecondaryFields(src, dest, mappings)
	if !ok {
		t.Fatalf("non-mandatory mismatch must not exclude, got ok=%v", ok)
	}
	if math.Abs(score-0.75) > 1e-9 {
		t.Fatalf("expected weighted average 0.75, got %v", score)
	}
}

// --- Pipeline integration tests ---

func candidateSources() []SourceRecord {
	return []SourceRecord{
		{
			ID:              "src-1",
			BatchID:         "batch-sec",
			ReferenceID:     "REF-001",
			CustomerNameRaw: "Acme Trading Company",
			NormalizedName:  Normalize("Acme Trading Company"),
			TransactionDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			Attributes:      map[string]interface{}{"tax_id": "TAX-100"},
		},
	}
}

// TestPipeline_SecondaryFields_MandatoryMismatchExcludesCandidate configures a
// mandatory EXACT tax_id pairing and gives the pipeline two near-identical
// name candidates, only one of which carries the matching tax_id. The
// mismatched candidate must never appear in the results.
func TestPipeline_SecondaryFields_MandatoryMismatchExcludesCandidate(t *testing.T) {
	sources := candidateSources()
	dests := []DestinationRecord{
		{
			ID:              "dest-match",
			BatchID:         "batch-sec",
			CustomerID:      "CUST-MATCH",
			CustomerNameRaw: "Acme Trading Company",
			NormalizedName:  Normalize("Acme Trading Company"),
			TransactionDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			Attributes:      map[string]interface{}{"tax_id": "TAX-100"},
		},
		{
			ID:              "dest-mismatch",
			BatchID:         "batch-sec",
			CustomerID:      "CUST-MISMATCH",
			CustomerNameRaw: "Acme Trading Company",
			NormalizedName:  Normalize("Acme Trading Company"),
			TransactionDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			Attributes:      map[string]interface{}{"tax_id": "TAX-999"},
		},
	}

	cfg := DefaultConfig()
	cfg.ColumnMapping.SecondaryFields = []SecondaryFieldMapping{
		{Name: "Tax ID", FieldSrc: "tax_id", FieldDest: "tax_id", MatchType: "EXACT", Weight: 0.20, IsMandatory: true},
	}
	cfg.MaxAlternativesPerSource = -1 // keep every candidate above ReviewThreshold

	engine := NewMatchEngine(cfg)
	results, _ := engine.ExecuteJob(context.Background(), "batch-sec", sources, dests, nil)

	for _, r := range results {
		if r.DestinationID == "dest-mismatch" {
			t.Fatalf("mandatory tax_id mismatch should have excluded dest-mismatch, but it appeared in results: %+v", r)
		}
	}
	found := false
	for _, r := range results {
		if r.DestinationID == "dest-match" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected dest-match (matching tax_id) to appear in results, got %+v", results)
	}
}

// TestPipeline_SecondaryFields_ScoreBlendAndExposedFields verifies that, once
// secondary fields are configured, the pipeline (a) blends 20% of the
// secondary score into ConfidenceScore, and (b) exposes SecondaryScore /
// SecondaryWeight on the result so the UI can render them without recomputing
// the blend itself.
func TestPipeline_SecondaryFields_ScoreBlendAndExposedFields(t *testing.T) {
	sources := candidateSources()
	dest := DestinationRecord{
		ID:              "dest-1",
		BatchID:         "batch-sec",
		CustomerID:      "CUST-1",
		CustomerNameRaw: "Acme Trading Company",
		NormalizedName:  Normalize("Acme Trading Company"),
		TransactionDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		Attributes:      map[string]interface{}{"tax_id": "TAX-100"},
	}

	// Baseline run with no secondary fields configured.
	baseCfg := DefaultConfig()
	baseEngine := NewMatchEngine(baseCfg)
	baseResults, _ := baseEngine.ExecuteJob(context.Background(), "batch-sec", sources, []DestinationRecord{dest}, nil)
	if len(baseResults) == 0 {
		t.Fatalf("expected at least one baseline result")
	}
	if baseResults[0].SecondaryWeight != 0 {
		t.Fatalf("SecondaryWeight must be 0 when no secondary fields are configured, got %v", baseResults[0].SecondaryWeight)
	}
	baseScore := baseResults[0].ConfidenceScore

	// Same pair, with a matching mandatory-free EXACT secondary field.
	secCfg := DefaultConfig()
	secCfg.ColumnMapping.SecondaryFields = []SecondaryFieldMapping{
		{Name: "Tax ID", FieldSrc: "tax_id", FieldDest: "tax_id", MatchType: "EXACT", Weight: 0.20},
	}
	secEngine := NewMatchEngine(secCfg)
	secResults, _ := secEngine.ExecuteJob(context.Background(), "batch-sec", sources, []DestinationRecord{dest}, nil)
	if len(secResults) == 0 {
		t.Fatalf("expected at least one result with secondary fields configured")
	}
	item := secResults[0]

	if item.SecondaryWeight != 0.2 {
		t.Fatalf("expected SecondaryWeight=0.2, got %v", item.SecondaryWeight)
	}
	if item.SecondaryScore != 1.0 {
		t.Fatalf("expected SecondaryScore=1.0 for an exact tax_id match, got %v", item.SecondaryScore)
	}

	expectedBlend := math.Round((baseScore*0.8+1.0*0.2)*10000) / 10000
	if math.Abs(item.ConfidenceScore-expectedBlend) > 1e-9 {
		t.Fatalf("expected blended ConfidenceScore %v (base=%v * 0.8 + secondary 1.0 * 0.2), got %v", expectedBlend, baseScore, item.ConfidenceScore)
	}

	foundReason := false
	for _, reason := range item.MatchReasons {
		if reason == "Exact match on Tax ID (TAX-100 == TAX-100)" {
			foundReason = true
		}
	}
	if !foundReason {
		t.Fatalf("expected match_reasons to include the secondary field's exact-match reason, got %v", item.MatchReasons)
	}
}
