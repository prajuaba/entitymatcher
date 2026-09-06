// Package mockdata — cross-script threshold measurement harness (backlog L3).
//
// This file is a MEASUREMENT HARNESS only. It deliberately changes no production
// behaviour: no engine is started, no blocking index is built, no assignment pass
// is run. It scores a fixed set of labelled pairs in isolation via
// matcher.CalculateCompositeScore and prints a threshold-sweep table.
//
// The table it prints is the evidence for deciding whether a cross-script-specific
// auto-match threshold (currently 0.90 auto / 0.70 review) can be lowered to
// improve recall on BILINGUAL_OUT_OF_DICT and BILINGUAL_IN_DICTIONARY pairs
// without admitting false friends. The NEG_BILINGUAL_FALSE_FRIEND column is the
// hard constraint: any threshold at which false-friend pairs cross the bar is
// unsafe regardless of the recall gain it buys.
//
// Run with:
//
//	go test -v -run TestCrossScriptAutoThresholdSweep ./internal/mockdata/
package mockdata

import (
	"fmt"
	"sort"
	"testing"

	"entitymatcher/matcher"

	"github.com/stretchr/testify/require"
)

func TestCrossScriptAutoThresholdSweep(t *testing.T) {
	// Build the labelled dataset (same generator and size as the main benchmark).
	// Only labeledPairs is needed for this harness; sources/dests/groundTruthMatches
	// are not used because we score pairs in isolation, not through the pipeline.
	_, _, _, labeledPairs := GenerateBigMockDataset(2000)

	// Filter into the three cross-script categories under study.
	var outOfDictPairs, inDictPairs, falseFriendPairs []LabeledPair
	for _, p := range labeledPairs {
		switch p.Category {
		case "BILINGUAL_OUT_OF_DICT":
			require.True(t, p.IsMatch, "BILINGUAL_OUT_OF_DICT pair must be a positive")
			outOfDictPairs = append(outOfDictPairs, p)
		case "BILINGUAL_IN_DICTIONARY":
			require.True(t, p.IsMatch, "BILINGUAL_IN_DICTIONARY pair must be a positive")
			inDictPairs = append(inDictPairs, p)
		case "NEG_BILINGUAL_FALSE_FRIEND":
			require.False(t, p.IsMatch, "NEG_BILINGUAL_FALSE_FRIEND pair must be a negative")
			falseFriendPairs = append(falseFriendPairs, p)
		}
	}

	// Sanity: the dataset must actually contain the categories we expect.
	require.NotEmpty(t, outOfDictPairs, "BILINGUAL_OUT_OF_DICT category is empty — dataset generation problem")
	require.NotEmpty(t, falseFriendPairs, "NEG_BILINGUAL_FALSE_FRIEND category is empty — dataset generation problem")

	totalOutOfDict := len(outOfDictPairs)
	totalInDict := len(inDictPairs)
	totalFalseFriend := len(falseFriendPairs)

	// Score every pair in isolation using the same single-pair call convention as
	// profile_test.go. No blocking index, no engine, no secondary-field evaluation
	// (DefaultConfig has empty SecondaryFields, so it would be a no-op anyway).
	cfg := matcher.DefaultConfig()

	type scoredPair struct {
		category string
		score    float64
	}
	var allScores []scoredPair
	allScores = make([]scoredPair, 0, totalOutOfDict+totalInDict+totalFalseFriend)

	scoreOne := func(p LabeledPair, category string) {
		scoreRes := matcher.CalculateCompositeScore(
			p.Source.NormalizedName,
			p.Destination.NormalizedName,
			p.Source.TransactionDate,
			p.Destination.TransactionDate,
			cfg.Weights,
			cfg.Algorithms,
			cfg.DateToleranceDays,
		)
		allScores = append(allScores, scoredPair{category: category, score: scoreRes.TotalScore})
	}

	for _, p := range outOfDictPairs {
		scoreOne(p, "BILINGUAL_OUT_OF_DICT")
	}
	for _, p := range inDictPairs {
		scoreOne(p, "BILINGUAL_IN_DICTIONARY")
	}
	for _, p := range falseFriendPairs {
		scoreOne(p, "NEG_BILINGUAL_FALSE_FRIEND")
	}

	// Threshold sweep
	thresholds := []float64{0.90, 0.88, 0.86, 0.85, 0.84, 0.82, 0.80, 0.78, 0.75, 0.70}

	t.Logf("==========================================================")
	t.Logf(" CROSS-SCRIPT AUTO-THRESHOLD SWEEP (backlog L3)            ")
	t.Logf(" autoThresh=%.2f  reviewThresh=%.2f  (from DefaultConfig)",
		cfg.AutoMatchThreshold, cfg.ReviewThreshold)
	t.Logf(" n(outOfDict)=%d  n(inDict)=%d  n(falseFriend)=%d",
		totalOutOfDict, totalInDict, totalFalseFriend)
	t.Logf("==========================================================")

	t.Logf("%-10s | %-22s | %-20s | %-22s",
		"Threshold", "OutOfDictAbove/Total", "InDictAbove/Total", "FalseFriendsAbove/52")
	t.Logf("-----------|--------------------------|------------------------|---------------------------")

	var falseAboveAtCurrentThreshold int

	for _, threshold := range thresholds {
		trueAboveOutOfDict := 0
		trueAboveInDict := 0
		falseAbove := 0

		for _, sp := range allScores {
			if sp.score >= threshold {
				switch sp.category {
				case "BILINGUAL_OUT_OF_DICT":
					trueAboveOutOfDict++
				case "BILINGUAL_IN_DICTIONARY":
					trueAboveInDict++
				case "NEG_BILINGUAL_FALSE_FRIEND":
					falseAbove++
				}
			}
		}

		t.Logf("%-10.2f | %-22s | %-20s | %-22s",
			threshold,
			fmt.Sprintf("%d/%d", trueAboveOutOfDict, totalOutOfDict),
			fmt.Sprintf("%d/%d", trueAboveInDict, totalInDict),
			fmt.Sprintf("%d/%d", falseAbove, totalFalseFriend))

		if threshold == cfg.AutoMatchThreshold {
			falseAboveAtCurrentThreshold = falseAbove
		}
	}

	t.Logf("==========================================================")

	// --- Assertions (real test, not just a printer) ---

	// 1. At the current auto threshold (0.90) no false-friend pair may score above it.
	//    This pins today's precision guarantee.
	require.Equal(t, 0, falseAboveAtCurrentThreshold,
		"at the current auto threshold %.2f, no false-friend pair should score above it",
		cfg.AutoMatchThreshold)

	// 2. The maximum false-friend score must be strictly below 1.0.
	var falseFriendScores []float64
	var outOfDictScores []float64
	for _, sp := range allScores {
		switch sp.category {
		case "NEG_BILINGUAL_FALSE_FRIEND":
			falseFriendScores = append(falseFriendScores, sp.score)
		case "BILINGUAL_OUT_OF_DICT":
			outOfDictScores = append(outOfDictScores, sp.score)
		}
	}

	require.NotEmpty(t, falseFriendScores, "no false-friend scores collected")
	require.NotEmpty(t, outOfDictScores, "no out-of-dict scores collected")

	ffMin, ffMed, ffMax := minMaxMedian(falseFriendScores)
	odMin, odMed, odMax := minMaxMedian(outOfDictScores)

	require.Less(t, ffMax, 1.0,
		"max NEG_BILINGUAL_FALSE_FRIEND score %.6f must be strictly less than 1.0", ffMax)

	// 3. Score distribution summary (informational, not asserted beyond the above).
	t.Logf("NEG_BILINGUAL_FALSE_FRIEND  scores: min=%.4f  median=%.4f  max=%.4f  (n=%d)",
		ffMin, ffMed, ffMax, len(falseFriendScores))
	t.Logf("BILINGUAL_OUT_OF_DICT       scores: min=%.4f  median=%.4f  max=%.4f  (n=%d)",
		odMin, odMed, odMax, len(outOfDictScores))
}

// minMaxMedian returns the minimum, median, and maximum of a float64 slice.
// It sorts a copy internally; the input slice is not mutated.
func minMaxMedian(scores []float64) (min, med, max float64) {
	n := len(scores)
	if n == 0 {
		return 0, 0, 0
	}
	sorted := make([]float64, n)
	copy(sorted, scores)
	sort.Float64s(sorted)

	min = sorted[0]
	max = sorted[n-1]

	if n%2 == 1 {
		med = sorted[n/2]
	} else {
		med = (sorted[n/2-1] + sorted[n/2]) / 2.0
	}
	return min, med, max
}
