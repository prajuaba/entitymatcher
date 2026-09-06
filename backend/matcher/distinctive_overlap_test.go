package matcher

import (
	"fmt"
	"math"
	"reflect"
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

func buildSharedTokenCorpus(t *testing.T) *CorpusStats {
	var sources []SourceRecord
	var dests []DestinationRecord

	// 10 SourceRecords with "ร่วม"
	for i := 0; i < 10; i++ {
		sources = append(sources, SourceRecord{
			NormalizedName: Normalize(fmt.Sprintf("บริษัท ฟิลเลอร์%d ร่วม จำกัด", i)),
		})
	}

	// 87 SourceRecords without "ร่วม"
	for i := 0; i < 87; i++ {
		sources = append(sources, SourceRecord{
			NormalizedName: Normalize(fmt.Sprintf("บริษัท เทค%d จำกัด", i)),
		})
	}

	// 2 DestinationRecords without "ร่วม"
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

func TestTuningOverridesDefaultCap(t *testing.T) {
	corpus := buildTransportCorpus(t)

	src := Normalize("บริษัท สาน ทรานสปอร์ต จำกัด")
	dest := Normalize("บริษัท สุทิน ทรานสปอร์ต จำกัด")

	result := CalculateCompositeScoreWithCorpusTuned(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30, corpus, ScoreTuning{NoDistinctiveOverlapCap: 0.60})

	t.Logf("TotalScore=%.4f, MatchReasons=%v", result.TotalScore, result.MatchReasons)

	if result.TotalScore > 0.60 {
		t.Errorf("expected score capped to <= 0.60 with custom tuning, got %.4f", result.TotalScore)
	}
	if result.TotalScore >= 0.85 {
		t.Errorf("expected tuned cap (0.60) to override the default cap (0.85), got %.4f", result.TotalScore)
	}

	found := false
	for _, reason := range result.MatchReasons {
		if strings.Contains(reason, "No distinctive token") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected match reasons to contain 'No distinctive token', got: %v", result.MatchReasons)
	}
}

func TestTuningOverridesMinWeight(t *testing.T) {
	corpus := buildSharedTokenCorpus(t)

	sharedWeight := corpus.Weight("ร่วม")
	t.Logf("Weight(ร่วม)=%.4f (expected exactly 0.5 by construction: N=99, df=10)", sharedWeight)
	if math.Abs(sharedWeight-0.5) > 0.001 {
		t.Fatalf("test fixture assumption broken: expected Weight(ร่วม) close to 0.5, got %.4f", sharedWeight)
	}

	src := Normalize("บริษัท สาน ร่วม จำกัด")
	dest := Normalize("บริษัท สุทิน ร่วม จำกัด")

	// Low floor (0.3): "ร่วม" (weight 0.5) clears the floor and is shared by both
	// sides, so the pair is judged to share a distinctive token and is NOT capped.
	low := CalculateCompositeScoreWithCorpusTuned(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30, corpus, ScoreTuning{DistinctiveOverlapMinWeight: 0.3})
	t.Logf("low floor (0.3): TotalScore=%.4f, MatchReasons=%v", low.TotalScore, low.MatchReasons)
	for _, reason := range low.MatchReasons {
		if strings.Contains(reason, "No distinctive token") {
			t.Errorf("with a 0.3 floor, expected 'ร่วม' (weight 0.5) to count as a shared distinctive token and NOT be capped, got reasons: %v", low.MatchReasons)
		}
	}

	// High floor (0.7): "ร่วม" (weight 0.5) no longer clears the floor, so it does
	// not count as distinctive. Each side's only remaining distinctive token
	// ("สาน" / "สุทิน", weight ~0.85) differs from the other side's, so the pair
	// shares no distinctive token and IS capped.
	high := CalculateCompositeScoreWithCorpusTuned(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30, corpus, ScoreTuning{DistinctiveOverlapMinWeight: 0.7})
	t.Logf("high floor (0.7): TotalScore=%.4f, MatchReasons=%v", high.TotalScore, high.MatchReasons)
	found := false
	for _, reason := range high.MatchReasons {
		if strings.Contains(reason, "No distinctive token") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("with a 0.7 floor, expected 'ร่วม' to no longer count as distinctive and the pair to be capped for lacking a shared distinctive token, got reasons: %v", high.MatchReasons)
	}
	if high.TotalScore > 0.85 {
		t.Errorf("expected high-floor result to be capped to the default 0.85, got %.4f", high.TotalScore)
	}
}

func TestZeroTuningUsesDefaults(t *testing.T) {
	corpus := buildTransportCorpus(t)

	src := Normalize("บริษัท สาน ทรานสปอร์ต จำกัด")
	dest := Normalize("บริษัท สุทิน ทรานสปอร์ต จำกัด")

	untuned := CalculateCompositeScoreWithCorpus(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30, corpus)
	tunedZero := CalculateCompositeScoreWithCorpusTuned(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30, corpus, ScoreTuning{})

	if untuned.TotalScore != tunedZero.TotalScore {
		t.Errorf("expected ScoreTuning{} to reproduce the untuned score exactly: untuned=%.4f, tunedZero=%.4f", untuned.TotalScore, tunedZero.TotalScore)
	}
	if !reflect.DeepEqual(untuned, tunedZero) {
		t.Errorf("expected ScoreTuning{} to reproduce the exact same ScoreResult as CalculateCompositeScoreWithCorpus, got different results:\nuntuned=%+v\ntunedZero=%+v", untuned, tunedZero)
	}
}

func TestMultiPartyForwardsTuning(t *testing.T) {
	corpus := buildTransportCorpus(t)

	// srcRaw has two comma-separated parties; the first is the same generic-overlap
	// pair used elsewhere in this file, the second is an unrelated single name that
	// should never win the best-of-parties pairing. This forces the multi-party
	// branch in CalculateCompositeScoreWithCorpusTuned, whose recursive call must
	// forward the caller's ScoreTuning or this test fails.
	srcRaw := "บริษัท สาน ทรานสปอร์ต จำกัด,นายศักดิ์ชาย ทดสอบ"
	destRaw := "บริษัท สุทิน ทรานสปอร์ต จำกัด"

	src := Normalize(srcRaw)
	dest := Normalize(destRaw)

	if len(SplitParties(src.Raw)) < 2 {
		t.Fatalf("test fixture assumption broken: expected srcRaw to split into >= 2 parties, got %v", SplitParties(src.Raw))
	}

	result := CalculateCompositeScoreWithCorpusTuned(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30, corpus, ScoreTuning{NoDistinctiveOverlapCap: 0.55})

	t.Logf("TotalScore=%.4f, MatchReasons=%v", result.TotalScore, result.MatchReasons)

	if result.TotalScore > 0.55 {
		t.Errorf("expected the multi-party recursive call to forward the custom 0.55 cap, got %.4f (if this is ~0.85 the recursion is silently using the default instead of forwarding tuning)", result.TotalScore)
	}

	found := false
	for _, reason := range result.MatchReasons {
		if strings.Contains(reason, "No distinctive token") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected match reasons to contain 'No distinctive token', got: %v", result.MatchReasons)
	}
}
