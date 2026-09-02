package matcher

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func buildTransportCorpus(t *testing.T) *CorpusStats {
	var sources []SourceRecord
	var dests []DestinationRecord

	// 60 SourceRecords with "ทรานสปอร์ต" and unique filler tokens
	for i := 0; i < 60; i++ {
		sources = append(sources, SourceRecord{
			NormalizedName: Normalize(fmt.Sprintf("บริษัท ฟิลเลอร์%d ทรานสปอร์ต จำกัด", i)),
		})
	}

	// 38 SourceRecords with unique filler tokens but no "ทรานสปอร์ต"
	for i := 0; i < 38; i++ {
		sources = append(sources, SourceRecord{
			NormalizedName: Normalize(fmt.Sprintf("บริษัท เทค%d จำกัด", i)),
		})
	}

	// 2 DestinationRecords with rare tokens "สาน" and "สุทิน"
	dests = append(dests, DestinationRecord{
		NormalizedName: Normalize("บริษัท สาน จำกัด"),
	})
	dests = append(dests, DestinationRecord{
		NormalizedName: Normalize("บริษัท สุทิน จำกัด"),
	})

	corpus := BuildCorpusStats(sources, dests)
	if corpus == nil {
		t.Fatal("expected non-nil corpus stats")
	}
	return corpus
}

func TestGenericOverlapAloneIsCapped(t *testing.T) {
	corpus := buildTransportCorpus(t)

	src := Normalize("บริษัท สาน ทรานสปอร์ต จำกัด")
	dest := Normalize("บริษัท สุทิน ทรานสปอร์ต จำกัด")

	// Compute uncapped score
	algosWithoutIDF := DefaultAlgorithms
	algosWithoutIDF.UseCorpusIDF = false
	before := CalculateCompositeScoreWithCorpus(src, dest, time.Time{}, time.Time{}, DefaultWeights, algosWithoutIDF, 30, corpus)

	// Compute capped score
	after := CalculateCompositeScoreWithCorpus(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30, corpus)

	t.Logf("before (uncapped) TotalScore=%.4f, after (capped) TotalScore=%.4f", before.TotalScore, after.TotalScore)

	if after.TotalScore > 0.85 {
		t.Errorf("expected capped score to be <= 0.85, got %.4f", after.TotalScore)
	}

	found := false
	for _, reason := range after.MatchReasons {
		if strings.Contains(reason, "No distinctive token") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected match reasons to contain 'No distinctive token', got: %v", after.MatchReasons)
	}
}

func TestSharedDistinctiveTokenIsNotCapped(t *testing.T) {
	corpus := buildTransportCorpus(t)

	src := Normalize("บริษัท สาน ทรานสปอร์ต จำกัด")
	dest := Normalize("บจก.สาน ทรานสปอร์ต")

	result := CalculateCompositeScoreWithCorpus(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30, corpus)

	t.Logf("TotalScore=%.4f, MatchReasons=%v", result.TotalScore, result.MatchReasons)

	if result.TotalScore <= 0.85 {
		t.Errorf("expected score > 0.85, got %.4f", result.TotalScore)
	}
}

func TestIdenticalNamesNeverCapped(t *testing.T) {
	corpus := buildTransportCorpus(t)

	src := Normalize("บริษัท ฟิลเลอร์5 ทรานสปอร์ต จำกัด")
	dest := src

	result := CalculateCompositeScoreWithCorpus(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30, corpus)

	if result.TotalScore != 1.0 {
		t.Errorf("expected identical names to score exactly 1.0, got %.4f", result.TotalScore)
	}
}

func TestNilCorpusDoesNotCap(t *testing.T) {
	src := Normalize("บริษัท สาน ทรานสปอร์ต จำกัด")
	dest := Normalize("บริษัท สุทิน ทรานสปอร์ต จำกัด")

	result := CalculateCompositeScoreWithCorpus(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30, nil)

	for _, reason := range result.MatchReasons {
		if strings.Contains(reason, "No distinctive token") {
			t.Errorf("expected no 'No distinctive token' in match reasons for nil corpus, got: %v", result.MatchReasons)
		}
	}
}

func TestCrossScriptPairsExempt(t *testing.T) {
	// Build a separate corpus without Thai/Latin tokens
	var dests []DestinationRecord

	dests = append(dests, DestinationRecord{NormalizedName: Normalize("บริษัท เทคโนโลยี จำกัด")})
	dests = append(dests, DestinationRecord{NormalizedName: Normalize("วีระชัย สมบูรณ์")})
	dests = append(dests, DestinationRecord{NormalizedName: Normalize("อารียา สุขใจ")})
	dests = append(dests, DestinationRecord{NormalizedName: Normalize("John Anderson")})
	dests = append(dests, DestinationRecord{NormalizedName: Normalize("Tech Solutions Group")})
	dests = append(dests, DestinationRecord{NormalizedName: Normalize("Maria Garcia")})
	dests = append(dests, DestinationRecord{NormalizedName: Normalize("David Wilson")})

	corpus := BuildCorpusStats([]SourceRecord{}, dests)
	if corpus == nil {
		t.Fatal("expected non-nil corpus stats")
	}

	src := Normalize("Somchai Jaidee")
	dest := Normalize("สมชาย ใจดี")
	sameDate := time.Now()

	result := CalculateCompositeScoreWithCorpus(src, dest, sameDate, sameDate, DefaultWeights, DefaultAlgorithms, 30, corpus)

	t.Logf("TotalScore=%.4f, CrossScript=%v, MatchReasons=%v", result.TotalScore, result.CrossScript, result.MatchReasons)

	if !result.CrossScript {
		t.Fatalf("expected cross-script pair to have CrossScript=true, got false")
	}

	found := false
	for _, reason := range result.MatchReasons {
		if strings.Contains(reason, "No distinctive token") {
			found = true
			break
		}
	}
	if found {
		t.Errorf("expected no 'No distinctive token' in match reasons for cross-script pair, got: %v", result.MatchReasons)
	}

	if math.Abs(result.TotalScore-0.8721) > 0.01 {
		t.Errorf("expected score close to 0.8721 (within 0.01), got %.4f", result.TotalScore)
	}
}
