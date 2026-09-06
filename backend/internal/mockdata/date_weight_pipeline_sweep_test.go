// Package mockdata — date-weight pipeline-level sweep (backlog S1).
//
// This file is a PIPELINE-LEVEL MEASUREMENT HARNESS only. It changes no production
// behaviour: no engine is modified, no scoring logic is altered, no assignment pass
// is changed. It runs the existing matcher.MatchEngine with different Weights.DateWeight
// values and reads back the resulting MatchStatus field to measure precision.
//
// It complements an isolation-level sweep that scores ground-truth pairings IN ISOLATION
// via matcher.CalculateCompositeScore, but such an isolation harness structurally cannot
// observe a source being auto-matched to the WRONG destination — the pipeline's blocking
// index, ranking, and 1:1 assignment pass all operate on the full candidate set, not a
// single pair. The false positives counted in this file's table are precisely the cases
// an isolation sweep cannot produce: rank-1 AUTO_MATCHED rows whose (sourceID, destinationID)
// key is absent from groundTruthMatches, but would have been blocked by a far-apart date
// under the current weight scheme.
//
// Context: the composite score is `total = name*NameWeight + date*DateWeight`. Defaults are
// NameWeight 0.85 / DateWeight 0.15. When two dates exceed the tolerance, date_score is 0,
// so total caps at name*0.85 = 0.85, below the default auto_match_threshold of 0.90. A
// perfect name match can therefore never auto-match if the dates are far apart — the date
// acts as a hard veto, not as a contribution. Real production data has 113,661 review rows
// in exactly this vetoed state because two date columns represent different business events.
// Lowering DateWeight lets the date inform ranking without vetoing.
//
// Run with:
//
//	go test -v -run TestDateWeightPipelineSweep ./internal/mockdata/
package mockdata

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"entitymatcher/matcher"

	"github.com/stretchr/testify/require"
)

func TestDateWeightPipelineSweep(t *testing.T) {
	// 1. Generate the dataset once.
	sources, dests, groundTruthMatches, _ := GenerateBigMockDataset(2000)

	// 2. Build lookup maps once, outside the sweep.
	sourceByID := make(map[string]matcher.SourceRecord, len(sources))
	for _, s := range sources {
		sourceByID[s.ID] = s
	}
	destByID := make(map[string]matcher.DestinationRecord, len(dests))
	for _, d := range dests {
		destByID[d.ID] = d
	}

	// 3. Count vetoed ground-truth pairs BEFORE the sweep.
	tolerance := matcher.DefaultConfig().DateToleranceDays
	var totalGroundTruth, vetoedGroundTruth int
	for key := range groundTruthMatches {
		parts := strings.SplitN(key, "_", 2)
		if len(parts) != 2 {
			continue
		}
		srcID, destID := parts[0], parts[1]

		srcRec, srcOK := sourceByID[srcID]
		if !srcOK {
			continue
		}
		destRec, destOK := destByID[destID]
		if !destOK {
			continue
		}

		totalGroundTruth++
		if !srcRec.TransactionDate.IsZero() && !destRec.TransactionDate.IsZero() {
			if matcher.CalculateDateScore(srcRec.TransactionDate, destRec.TransactionDate, tolerance) == 0.0 {
				vetoedGroundTruth++
			}
		}
	}

	vetoedPct := 0.0
	if totalGroundTruth > 0 {
		vetoedPct = float64(vetoedGroundTruth) / float64(totalGroundTruth) * 100.0
	}
	t.Logf("vetoed ground-truth pairs: %d of %d (%.1f%%)", vetoedGroundTruth, totalGroundTruth, vetoedPct)

	if vetoedPct < 1.0 {
		t.Log("HONESTY NOTE: This benchmark dataset's date distribution does not exercise the veto condition seen in production (where 113,661 rows are vetoed). The sweep below is therefore not informative about the real-data situation; do not read false confidence into a flat precision column.")
	} else {
		t.Log("HONESTY NOTE: This sweep IS informative about the veto scenario because a non-negligible fraction of ground-truth pairs have dates beyond tolerance.")
	}

	// 4. Sweep values.
	dateWeights := []float64{0.15, 0.12, 0.10, 0.08, 0.05, 0.03, 0.01}

	// 5. Per-value result struct.
	type sweepRow struct {
		dateWeight   float64
		nameWeight   float64
		autoMatched  int
		truePos      int
		falsePos     int
		reviewNeeded int
		precision    float64
		recall       float64
		f1           float64
	}
	var rows []sweepRow

	// 6. DateWeight loop.
	for _, dateWeight := range dateWeights {
		cfg := matcher.DefaultConfig()
		cfg.WorkerCount = 8
		cfg.MaxCandidatesPerSrc = 50
		cfg.Weights.DateWeight = dateWeight
		cfg.Weights.NameWeight = 1.0 - dateWeight
		// cfg.AutoMatchThreshold left at default 0.90 (do not overwrite)

		engine := matcher.NewMatchEngine(cfg)

		batchID := fmt.Sprintf("date-weight-sweep-%.2f", dateWeight)
		results, _ := engine.ExecuteJob(context.Background(), batchID, sources, dests, nil)

		autoMatched, truePos, falsePos, reviewNeeded := 0, 0, 0, 0

		for _, res := range results {
			if res.MatchStatus == "REVIEW_NEEDED" {
				reviewNeeded++
			}

			if res.Rank == 1 && res.MatchStatus == "AUTO_MATCHED" {
				autoMatched++
				_, srcOK := sourceByID[res.SourceID]
				_, destOK := destByID[res.DestinationID]
				if !srcOK {
					t.Fatalf("lookup bug: sourceID %q not found in sourceByID (dateWeight=%.2f)", res.SourceID, dateWeight)
				}
				if !destOK {
					t.Fatalf("lookup bug: destinationID %q not found in destByID (dateWeight=%.2f)", res.DestinationID, dateWeight)
				}

				if groundTruthMatches[res.SourceID+"_"+res.DestinationID] {
					truePos++
				} else {
					falsePos++
				}
			}
		}

		precision := 0.0
		if autoMatched > 0 {
			precision = float64(truePos) / float64(autoMatched)
		}

		recall := 0.0
		if len(groundTruthMatches) > 0 {
			recall = float64(truePos) / float64(len(groundTruthMatches))
		}

		f1 := 0.0
		if precision+recall > 0 {
			f1 = 2.0 * precision * recall / (precision + recall)
		}

		row := sweepRow{
			dateWeight:   dateWeight,
			nameWeight:   1.0 - dateWeight,
			autoMatched:  autoMatched,
			truePos:      truePos,
			falsePos:     falsePos,
			reviewNeeded: reviewNeeded,
			precision:    precision,
			recall:       recall,
			f1:           f1,
		}
		rows = append(rows, row)

		// At the 0.15 baseline, assert precision must be 1.0
		if dateWeight == 0.15 {
			require.InDelta(t, 1.0, precision, 1e-9,
				"precision at dateWeight 0.15 (current default) must equal 1.0 (documented current pipeline state); a deviation means the harness itself broke, not that production precision changed")
		}
	}

	// 7. Verify autoMatched varies across the sweep (catches bug where swept weight has no effect).
	var allAutoMatchedEqual = true
	for _, r := range rows {
		if r.autoMatched != rows[0].autoMatched {
			allAutoMatchedEqual = false
			break
		}
	}
	require.False(t, allAutoMatchedEqual,
		"autoMatched is constant across every dateWeight in the sweep — the harness is not actually measuring anything; a flat line means the swept weight has no effect on the observed outcome (this is exactly the bug this test exists to catch)")

	// 8. Table: raw per-weight pipeline counts.
	t.Log("")
	t.Log("==========================================================")
	t.Log(" PIPELINE-LEVEL DATE-WEIGHT SWEEP (backlog S1)")
	t.Log("==========================================================")
	t.Logf("%-12s | %-12s | %-12s | %-12s | %-12s | %-12s | %-12s | %-12s | %-12s",
		"dateWeight", "nameWeight", "autoMatched", "truePos", "falsePos", "reviewNeeded", "precision", "recall", "f1")
	t.Log("-------------|--------------|--------------|--------------|--------------|--------------|--------------|--------------|--------------")
	for _, r := range rows {
		t.Logf("%-12.2f | %-12.2f | %-12d | %-12d | %-12d | %-12d | %-12.4f | %-12.4f | %-12.4f",
			r.dateWeight, r.nameWeight, r.autoMatched, r.truePos, r.falsePos, r.reviewNeeded, r.precision, r.recall, r.f1)
	}
	t.Log("==========================================================")

	// 9. Caveat.
	t.Log("")
	t.Log("CAVEAT: This measures the synthetic benchmark corpus, not production. The vetoed ground-truth-pairs line above determines whether the table's numbers are relevant to the real 113,661-row veto problem in production.")
}
