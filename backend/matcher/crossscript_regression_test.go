package matcher

import (
	"strings"
	"testing"
	"time"
)

// TestCrossScriptRegression guards the cross-script Thai/Latin matching behavior
// for the canonical pair "Somchai Jaidee" / "สมชาย ใจดี". It verifies that:
// 1. Thai จ maps correctly to "ch" under RTGS (Royal Thai General System),
//    not the incorrect "j" that would artificially boost cross-script scores
// 2. The composite score for this pair is now 0.8721, which is ABOVE the review threshold (0.70)
//    but BELOW the auto threshold (0.90). This reflects the fix to phonetic folds:
//    j->ch and v->w folds in PhoneticSkeleton now align skeletons from both sides.
//    Benchmark data from internal/mockdata's TestFullLoopBigDatasetBenchmark:
//    BILINGUAL_OUT_OF_DICT category: true positives 2->3 (out of 30),
//    mean correct-score 0.836->0.844, precision held at 1.0000.
//    NEG_BILINGUAL_FALSE_FRIEND category unchanged (TN=52, FP=0).
//    However, WrongScored>=Auto for BILINGUAL_OUT_OF_DICT rose from 1 to 2:
//    one wrong pair now exceeds the auto threshold too, and overall precision
//    held only due to margin rule and 1:1 assignment demoting that wrong pair.
//    The category remains dominated by spelling variance (examples: Suchat/Suchart,
//    Bunmi/Boonmee, Narongse, Seawat, Aswin) which is why recall barely moved (0.5900 -> 0.5902).
// 3. Both retrieval directions work (Latin->Thai and Thai->Latin)
// 4. Romanization symmetry is preserved between Thai and Latin forms
// 5. The phonetic skeleton mechanism that now aligns skeletons (both "smchchd") is intact

func TestCrossScriptRegression(t *testing.T) {
	// Helper to compute cross-script score in isolation
	crossScriptScore := func(t *testing.T) (result ScoreResult) {
		srcClean := Normalize("Somchai Jaidee")
		destClean := Normalize("สมชาย ใจดี")
		sameDate := time.Now()
		toggles := DefaultAlgorithms

		return CalculateCompositeScore(srcClean, destClean, sameDate, sameDate, DefaultWeights, toggles, 30)
	}

	// Helper to check if Latin->Thai retrieval works
	latinToThaiRetrievalSucceeds := func(t *testing.T) bool {
		dests := []DestinationRecord{
			{ID: "dest-true", CustomerNameRaw: "สมชาย ใจดี", NormalizedName: Normalize("สมชาย ใจดี")},
			{ID: "dest-filler-1", CustomerNameRaw: "บริษัท เทคโนโลยี จำกัด", NormalizedName: Normalize("บริษัท เทคโนโลยี จำกัด")},
			{ID: "dest-filler-2", CustomerNameRaw: "วีระชัย สมบูรณ์", NormalizedName: Normalize("วีระชัย สมบูรณ์")},
			{ID: "dest-filler-3", CustomerNameRaw: "อารียา สุขใจ", NormalizedName: Normalize("อารียา สุขใจ")},
			{ID: "dest-filler-4", CustomerNameRaw: "John Anderson", NormalizedName: Normalize("John Anderson")},
			{ID: "dest-filler-5", CustomerNameRaw: "Tech Solutions Group", NormalizedName: Normalize("Tech Solutions Group")},
			{ID: "dest-filler-6", CustomerNameRaw: "Maria Garcia", NormalizedName: Normalize("Maria Garcia")},
			{ID: "dest-filler-7", CustomerNameRaw: "David Wilson", NormalizedName: Normalize("David Wilson")},
		}

		idx := NewBlockingIndexWithOptions(dests, 0.05, DefaultAbsoluteCeiling, true)
		src := SourceRecord{ID: "src-1", CustomerNameRaw: "Somchai Jaidee", NormalizedName: Normalize("Somchai Jaidee")}
		candidates := idx.QueryCandidates(src, 3)

		truePartnerFound := false
		for _, cand := range candidates {
			if cand.ID == "dest-true" {
				truePartnerFound = true
				break
			}
		}
		return truePartnerFound
	}

	t.Run("Normalization", func(t *testing.T) {
		srcClean := Normalize("Somchai Jaidee")
		destClean := Normalize("สมชาย ใจดี")

		t.Logf("Source Normalization:")
		t.Logf("  Raw: %s", srcClean.Raw)
		t.Logf("  Cleaned: %s", srcClean.Cleaned)
		t.Logf("  Tokens: %v", srcClean.Tokens)
		t.Logf("  SortedTokens: %s", srcClean.SortedTokens)
		t.Logf("  PhoneticKey: %s", srcClean.PhoneticKey)
		t.Logf("  PhoneticForm: %s", srcClean.PhoneticForm)
		t.Logf("  Romanized: %s", srcClean.Romanized)

		t.Logf("Destination Normalization:")
		t.Logf("  Raw: %s", destClean.Raw)
		t.Logf("  Cleaned: %s", destClean.Cleaned)
		t.Logf("  Tokens: %v", destClean.Tokens)
		t.Logf("  SortedTokens: %s", destClean.SortedTokens)
		t.Logf("  PhoneticKey: %s", destClean.PhoneticKey)
		t.Logf("  PhoneticForm: %s", destClean.PhoneticForm)
		t.Logf("  Romanized: %s", destClean.Romanized)

		t.Logf("Romanized match: %v (src=%q, dest=%q)", srcClean.Romanized == destClean.Romanized, srcClean.Romanized, destClean.Romanized)
	})

	t.Run("ScoringInIsolation", func(t *testing.T) {
		result := crossScriptScore(t)

		t.Logf("ScoreResult fields:")
		t.Logf("  TotalScore: %.4f", result.TotalScore)
		t.Logf("  NameScore: %.4f", result.NameScore)
		t.Logf("  DateScore: %.4f", result.DateScore)
		t.Logf("  JWScore: %.4f", result.JWScore)
		t.Logf("  LevScore: %.4f", result.LevScore)
		t.Logf("  TokenScore: %.4f", result.TokenScore)
		t.Logf("  TrigramScore: %.4f", result.TrigramScore)
		t.Logf("  RomanizedScore: %.4f", result.RomanizedScore)
		t.Logf("  MatchReasons: %v", result.MatchReasons)

		meetsReview := result.TotalScore >= 0.70
		meetsAuto := result.TotalScore >= 0.90
		t.Logf("VERDICT: TotalScore=%.4f -> meets review threshold(0.70)=%v, meets auto threshold(0.90)=%v", result.TotalScore, meetsReview, meetsAuto)
		t.Logf("NOTE: Pair now scores %.4f (meets review threshold but NOT auto threshold)", result.TotalScore)

		if result.TotalScore < 0.85 {
			t.Errorf("cross-script scoring has REGRESSED: score dropped below the measured floor of 0.8721 (actual=%.4f). The j->ch and/or v->w phonetic folds may have been weakened or removed.", result.TotalScore)
		} else if result.TotalScore >= 0.90 {
			t.Errorf("cross-script score %.4f now meets the auto threshold — this is an IMPROVEMENT, not a failure: re-measure the BILINGUAL_OUT_OF_DICT category in internal/mockdata's TestFullLoopBigDatasetBenchmark and update this probe's expected band.", result.TotalScore)
		} else {
			t.Logf("cross-script score %.4f is within expected range [0.85, 0.90) — this reflects the fix to phonetic folds (j->ch, v->w)", result.TotalScore)
		}
	})

	t.Run("RTGSCorrectnessGuard", func(t *testing.T) {
		romanized := RomanizeThai("ใจดี")
		if !strings.Contains(romanized, "ch") {
			t.Errorf("Thai จ should map to 'ch' under correct RTGS, not 'j'. Actual romanization: %q. Changing จ to romanize as 'j' to make cross-script pairs match would corrupt the RTGS transcription standard.", romanized)
		}
		if strings.Contains(romanized, "j") {
			t.Errorf("Thai จ maps to 'j' but should be 'ch' under correct RTGS (Royal Thai General System). Actual romanization: %q. This mapping must not be changed to force cross-script matches — that would corrupt the transcription standard.", romanized)
		}
	})

	t.Run("RetrievalLatinSourceThaiDest", func(t *testing.T) {
		dests := []DestinationRecord{
			{ID: "dest-true", CustomerNameRaw: "สมชาย ใจดี", NormalizedName: Normalize("สมชาย ใจดี")},
			{ID: "dest-filler-1", CustomerNameRaw: "บริษัท เทคโนโลยี จำกัด", NormalizedName: Normalize("บริษัท เทคโนโลยี จำกัด")},
			{ID: "dest-filler-2", CustomerNameRaw: "วีระชัย สมบูรณ์", NormalizedName: Normalize("วีระชัย สมบูรณ์")},
			{ID: "dest-filler-3", CustomerNameRaw: "อารียา สุขใจ", NormalizedName: Normalize("อารียา สุขใจ")},
			{ID: "dest-filler-4", CustomerNameRaw: "John Anderson", NormalizedName: Normalize("John Anderson")},
			{ID: "dest-filler-5", CustomerNameRaw: "Tech Solutions Group", NormalizedName: Normalize("Tech Solutions Group")},
			{ID: "dest-filler-6", CustomerNameRaw: "Maria Garcia", NormalizedName: Normalize("Maria Garcia")},
			{ID: "dest-filler-7", CustomerNameRaw: "David Wilson", NormalizedName: Normalize("David Wilson")},
		}

		idx := NewBlockingIndexWithOptions(dests, 0.05, DefaultAbsoluteCeiling, true)
		src := SourceRecord{ID: "src-1", CustomerNameRaw: "Somchai Jaidee", NormalizedName: Normalize("Somchai Jaidee")}
		candidates := idx.QueryCandidates(src, 3)

		t.Logf("len(candidates): %d", len(candidates))
		for i, cand := range candidates {
			t.Logf("  candidate[%d]: ID=%s, NameRaw=%s", i, cand.ID, cand.CustomerNameRaw)
		}

		truePartnerFound := false
		for _, cand := range candidates {
			if cand.ID == "dest-true" {
				truePartnerFound = true
				break
			}
		}

		t.Logf("len(destinations)=%d maxCandidates=%d (destinations > maxCandidates: index logic exercised, not bypassed)", len(dests), 3)

		if !truePartnerFound {
			t.Errorf("RETRIEVAL FAILURE: true partner not in candidate set. Latin->Thai retrieval must work to distinguish this from a blocking-index bug (scoring would accept it).")
		}

		// ACTIONABLE BUG check
		result := crossScriptScore(t)
		isolatedScore := result.TotalScore
		if isolatedScore >= 0.70 && !truePartnerFound {
			t.Errorf("ACTIONABLE BUG: pair scores %.4f in isolation (>= review threshold 0.70) but blocking/retrieval never offers it as a candidate — scorer would accept it, blocking drops it before scoring ever runs", isolatedScore)
		}
	})

	t.Run("RetrievalThaiSourceLatinDest", func(t *testing.T) {
		dests2 := []DestinationRecord{
			{ID: "dest-true-latin", CustomerNameRaw: "Somchai Jaidee", NormalizedName: Normalize("Somchai Jaidee")},
			{ID: "dest-filler-a", CustomerNameRaw: "บริษัท เทคโนโลยี จำกัด", NormalizedName: Normalize("บริษัท เทคโนโลยี จำกัด")},
			{ID: "dest-filler-b", CustomerNameRaw: "วีระชัย สมบูรณ์", NormalizedName: Normalize("วีระชัย สมบูรณ์")},
			{ID: "dest-filler-c", CustomerNameRaw: "อารียา สุขใจ", NormalizedName: Normalize("อารียา สุขใจ")},
			{ID: "dest-filler-d", CustomerNameRaw: "John Anderson", NormalizedName: Normalize("John Anderson")},
			{ID: "dest-filler-e", CustomerNameRaw: "Tech Solutions Group", NormalizedName: Normalize("Tech Solutions Group")},
			{ID: "dest-filler-f", CustomerNameRaw: "Maria Garcia", NormalizedName: Normalize("Maria Garcia")},
			{ID: "dest-filler-g", CustomerNameRaw: "David Wilson", NormalizedName: Normalize("David Wilson")},
		}

		idx2 := NewBlockingIndexWithOptions(dests2, 0.05, DefaultAbsoluteCeiling, true)
		src2 := SourceRecord{ID: "src-2", CustomerNameRaw: "สมชาย ใจดี", NormalizedName: Normalize("สมชาย ใจดี")}
		candidates2 := idx2.QueryCandidates(src2, 3)

		t.Logf("len(candidates2): %d", len(candidates2))
		truePartnerFound2 := false
		for _, cand := range candidates2 {
			if cand.ID == "dest-true-latin" {
				truePartnerFound2 = true
				break
			}
		}

		latinToThaiRetrieved := latinToThaiRetrievalSucceeds(t)
		t.Logf("DIRECTION: Latin->Thai retrieved=%v, Thai->Latin retrieved=%v", latinToThaiRetrieved, truePartnerFound2)

		if !truePartnerFound2 {
			t.Errorf("RETRIEVAL FAILURE: true partner not in candidate set. Thai->Latin retrieval must work to distinguish this from a blocking-index bug (scoring would accept it).")
		}
	})

	t.Run("RomanizationSymmetryGuard", func(t *testing.T) {
		rom1 := Normalize("สมชาย").Romanized
		rom2 := Normalize("somchai").Romanized

		if rom1 != rom2 {
			t.Errorf("romanization symmetry broken: Thai 'สมชาย' romanizes to %q, Latin 'somchai' to %q", rom1, rom2)
		}
		if rom1 != "smch" {
			t.Errorf("romanization symmetry broken: expected both to be exactly 'smch', got %q", rom1)
		}
	})

	t.Run("MechanismPin", func(t *testing.T) {
		srcSkeleton := PhoneticSkeleton(Normalize("Somchai Jaidee").Romanized)
		destSkeleton := PhoneticSkeleton(Normalize("สมชาย ใจดี").Romanized)

		if srcSkeleton != destSkeleton {
			t.Errorf("skeletons should align, but got src=%q and dest=%q", srcSkeleton, destSkeleton)
		}
		if srcSkeleton != "smchchd" {
			t.Errorf("skeleton value changed from expected 'smchchd' to %q", srcSkeleton)
		}
		if strings.Contains(srcSkeleton, "j") {
			t.Errorf("source skeleton %q contains 'j' but should be folded to 'ch' by phoneticEquivalents (rtgs.go)", srcSkeleton)
		}

		romanized := RomanizeThai("ใจดี")
		if !strings.Contains(romanized, "ch") {
			t.Errorf("RomanizeThai('ใจดี') should still contain 'ch' under correct RTGS, not 'j'. Actual romanization: %q", romanized)
		}
		if strings.Contains(romanized, "j") {
			t.Errorf("RomanizeThai('ใจดี') maps to 'j' but should be 'ch' under correct RTGS (Royal Thai General System). Actual romanization: %q", romanized)
		}

		t.Logf("Source skeleton: %q, Destination skeleton: %q", srcSkeleton, destSkeleton)
		t.Logf("This documents the mechanism: skeletons now align perfectly ('smchchd' == 'smchchd') due to j->ch and v->w phonetic folds")
		// The residual gap comes from vowel-bearing per-part comparison in CrossScriptPartsScore,
		// not the skeleton mechanism. RomanizedScore is 0.9095, which blends down into 0.8721 composite.
	})

	t.Run("IndexContents", func(t *testing.T) {
		dests := []DestinationRecord{
			{ID: "dest-true", CustomerNameRaw: "สมชาย ใจดี", NormalizedName: Normalize("สมชาย ใจดี")},
			{ID: "dest-filler-1", CustomerNameRaw: "บริษัท เทคโนโลยี จำกัด", NormalizedName: Normalize("บริษัท เทคโนโลยี จำกัด")},
			{ID: "dest-filler-2", CustomerNameRaw: "วีระชัย สมบูรณ์", NormalizedName: Normalize("วีระชัย สมบูรณ์")},
			{ID: "dest-filler-3", CustomerNameRaw: "อารียา สุขใจ", NormalizedName: Normalize("อารียา สุขใจ")},
			{ID: "dest-filler-4", CustomerNameRaw: "John Anderson", NormalizedName: Normalize("John Anderson")},
			{ID: "dest-filler-5", CustomerNameRaw: "Tech Solutions Group", NormalizedName: Normalize("Tech Solutions Group")},
			{ID: "dest-filler-6", CustomerNameRaw: "Maria Garcia", NormalizedName: Normalize("Maria Garcia")},
			{ID: "dest-filler-7", CustomerNameRaw: "David Wilson", NormalizedName: Normalize("David Wilson")},
		}

		idx := NewBlockingIndexWithOptions(dests, 0.05, DefaultAbsoluteCeiling, true)

		t.Logf("idx.romanizedMap is non-empty: %v (len=%d)", len(idx.romanizedMap) > 0, len(idx.romanizedMap))

		destRomanized := Normalize("สมชาย ใจดี").Romanized
		t.Logf("destination romanized: %q", destRomanized)

		destTrigrams := extractTrigrams(destRomanized)
		t.Logf("destination trigrams (first 5): %v", destTrigrams[:min(5, len(destTrigrams))])

		srcRomanized := Normalize("Somchai Jaidee").Romanized
		t.Logf("source romanized: %q", srcRomanized)

		srcTrigrams := extractTrigrams(srcRomanized)
		t.Logf("source trigrams (first 5): %v", srcTrigrams[:min(5, len(srcTrigrams))])

		trueIdx := 0

		allSrcTrigramsFound := true
		for _, trigram := range srcTrigrams {
			if postingList, exists := idx.romanizedMap[trigram]; exists {
				containsTruePartner := false
				for _, idxInList := range postingList {
					if idxInList == trueIdx {
						containsTruePartner = true
						break
					}
				}
				t.Logf("trigram=%q inIndex=%v containsTruePartner=%v", trigram, exists, containsTruePartner)
				if !containsTruePartner {
					allSrcTrigramsFound = false
				}
			} else {
				t.Logf("trigram=%q inIndex=%v containsTruePartner=unknown (not found)", trigram, exists)
				allSrcTrigramsFound = false
			}
		}

		if len(idx.romanizedMap) == 0 {
			t.Logf("VERDICT: destination never indexed - romanizedMap is empty")
		} else if allSrcTrigramsFound {
			t.Logf("VERDICT: destination indexed and query trigrams overlap")
		} else {
			t.Logf("VERDICT: destination indexed but query trigrams don't overlap")
		}
	})
}
