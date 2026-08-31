// Package mockdata — cross-script pipeline-level threshold sweep (backlog L3).
//
// This file is a PIPELINE-LEVEL MEASUREMENT HARNESS only. It changes no production
// behaviour: no engine is modified, no scoring logic is altered, no assignment pass
// is changed. It runs the existing matcher.MatchEngine at different AutoMatchThreshold
// values and reads back the resulting MatchStatus field to measure precision.
//
// It complements crossscript_threshold_test.go (TestCrossScriptAutoThresholdSweep),
// which scores ground-truth pairings IN ISOLATION via matcher.CalculateCompositeScore.
// Because the isolation harness scores each (source, destination) pair in isolation,
// it structurally cannot observe a source being auto-matched to the WRONG destination —
// the pipeline's blocking index, ranking, and 1:1 assignment pass all operate on the
// full candidate set, not a single pair. The crossFP column in this file's tables is
// exactly the number that the isolation sweep cannot produce: it counts rank-1
// AUTO_MATCHED rows whose (sourceID, destinationID) key is absent from groundTruthMatches.
//
// Run with:
//
//	go test -v -run TestCrossScriptPipelineThresholdSweep ./internal/mockdata/
package mockdata

import (
	"context"
	"fmt"
	"testing"

	"entitymatcher/matcher"

	"github.com/stretchr/testify/require"
)

func TestCrossScriptPipelineThresholdSweep(t *testing.T) {
	// 1. Generate the dataset once.
	sources, dests, groundTruthMatches, _ := GenerateBigMockDataset(2000)

	// 2. Build lookup maps once, outside the threshold loop.
	sourceByID := make(map[string]matcher.SourceRecord, len(sources))
	for _, s := range sources {
		sourceByID[s.ID] = s
	}
	destByID := make(map[string]matcher.DestinationRecord, len(dests))
	for _, d := range dests {
		destByID[d.ID] = d
	}

	// 3. Threshold sweep values.
	thresholds := []float64{0.90, 0.88, 0.86, 0.84, 0.82, 0.80, 0.78, 0.75}

	// 4. Per-threshold result struct and baseline capture variables.
	type pipelineRow struct {
		threshold        float64
		crossTP          int
		crossFP          int
		sameTP           int
		sameFP           int
		overallPrecision float64
	}
	var rows []pipelineRow

	baselineSameTP := 0
	baselineSameFP := 0
	baselineOverallPrecision := 0.0

	// 5. Threshold loop.
	for _, threshold := range thresholds {
		cfg := matcher.DefaultConfig()
		cfg.WorkerCount = 8
		cfg.MaxCandidatesPerSrc = 50
		cfg.AutoMatchThreshold = threshold

		engine := matcher.NewMatchEngine(cfg)

		batchID := fmt.Sprintf("sweep-pipeline-%.2f", threshold)
		results, _ := engine.ExecuteJob(context.Background(), batchID, sources, dests, nil)

		crossTP, crossFP, sameTP, sameFP := 0, 0, 0, 0

		for _, res := range results {
			if res.Rank != 1 || res.MatchStatus != "AUTO_MATCHED" {
				continue
			}

			srcRec, srcOK := sourceByID[res.SourceID]
			destRec, destOK := destByID[res.DestinationID]
			if !srcOK {
				t.Fatalf("lookup bug: sourceID %q not found in sourceByID (threshold=%.2f)", res.SourceID, threshold)
			}
			if !destOK {
				t.Fatalf("lookup bug: destinationID %q not found in destByID (threshold=%.2f)", res.DestinationID, threshold)
			}

			isCrossScript := matcher.ContainsThai(srcRec.CustomerNameRaw) != matcher.ContainsThai(destRec.CustomerNameRaw)
			isCorrect := groundTruthMatches[res.SourceID+"_"+res.DestinationID]

			switch {
			case isCrossScript && isCorrect:
				crossTP++
			case isCrossScript && !isCorrect:
				crossFP++
			case !isCrossScript && isCorrect:
				sameTP++
			case !isCrossScript && !isCorrect:
				sameFP++
			}
		}

		denom := crossTP + crossFP + sameTP + sameFP
		overallPrecision := 0.0
		if denom > 0 {
			overallPrecision = float64(crossTP+sameTP) / float64(denom)
		}

		row := pipelineRow{
			threshold:        threshold,
			crossTP:          crossTP,
			crossFP:          crossFP,
			sameTP:           sameTP,
			sameFP:           sameFP,
			overallPrecision: overallPrecision,
		}
		rows = append(rows, row)

		// 5h. At the 0.90 baseline, save counters and assert invariants.
		if threshold == 0.90 {
			baselineSameTP = sameTP
			baselineSameFP = sameFP
			baselineOverallPrecision = overallPrecision

			require.Equal(t, 0, crossFP,
				"at threshold 0.90 cross-script auto-matches must have zero false positives (pins the pipeline-level precision guarantee)")
			require.Greater(t, crossTP, 0,
				"at threshold 0.90 the sweep must observe at least one cross-script auto-match — zero would mean the classifier is not observing anything, not that precision is perfect")
			require.InDelta(t, 1.0, overallPrecision, 1e-9,
				"overall precision at threshold 0.90 must equal 1.0 (documented current pipeline state)")
		}
	}

	// 6. Table 1: raw per-threshold pipeline counts.
	t.Log("")
	t.Log("==========================================================")
	t.Log(" PIPELINE-LEVEL CROSS-SCRIPT THRESHOLD SWEEP (backlog L3)")
	t.Log("==========================================================")
	t.Logf("%-10s | %-8s | %-8s | %-8s | %-8s | %-16s",
		"threshold", "crossTP", "crossFP", "sameTP", "sameFP", "overallPrecision")
	t.Log("-----------|----------|----------|----------|----------|------------------")
	for _, r := range rows {
		t.Logf("%-10.2f | %-8d | %-8d | %-8d | %-8d | %-16.4f",
			r.threshold, r.crossTP, r.crossFP, r.sameTP, r.sameFP, r.overallPrecision)
	}
	t.Log("==========================================================")

	// 7. Table 2: estimated policy precision (same-script held at 0.90 baseline).
	t.Log("")
	t.Log("==========================================================")
	t.Log(" ESTIMATED POLICY PRECISION (same-script held at threshold=0.90)")
	t.Log("==========================================================")
	t.Logf(" baseline: sameTP=%d  sameFP=%d  overallPrecision=%.4f",
		baselineSameTP, baselineSameFP, baselineOverallPrecision)
	t.Logf("%-10s | %-8s | %-8s | %-18s | %-16s",
		"threshold", "crossTP", "crossFP", "estPolicyPrecision", "deltaVsBaseline")
	t.Log("-----------|----------|----------|------------------|----------------")
	for _, r := range rows {
		num := r.crossTP + baselineSameTP
		den := r.crossTP + r.crossFP + baselineSameTP + baselineSameFP
		estPolicyPrecision := 0.0
		if den > 0 {
			estPolicyPrecision = float64(num) / float64(den)
		}
		deltaVsBaseline := estPolicyPrecision - baselineOverallPrecision
		t.Logf("%-10.2f | %-8d | %-8d | %-18.4f | %+.4f",
			r.threshold, r.crossTP, r.crossFP, estPolicyPrecision, deltaVsBaseline)
	}
	t.Log("==========================================================")

	// 8. Caveat.
	t.Log("")
	t.Log("CAVEAT: The estimated policy precision in the table above holds the")
	t.Log("same-script decisions fixed at their threshold=0.90 values. A residual")
	t.Log("coupling remains because the 1:1 assignment pass (ResolveAssignments)")
	t.Log("is global: a same-script source could claim a destination that a")
	t.Log("cross-script source would otherwise have taken at the lower threshold,")
	t.Log("or vice versa. The estimate is therefore optimistic to an unknown degree.")
}
