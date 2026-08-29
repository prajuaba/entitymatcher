package testdata

import (
	"context"
	"fmt"
	"testing"
	"time"

	"entitymatcher/matcher"
)

func TestFullLoopBigDatasetBenchmark(t *testing.T) {
	// Generate 2,000 source records and 2,000 destination records (4,000,000 potential pair combinations)
	datasetSize := 2000
	t.Logf("=== GENERATING BIG MOCK DATASET (%d Records) ===", datasetSize)

	sources, dests, _, labeledPairs := GenerateBigMockDataset(datasetSize)

	t.Logf("Generated %d Source Records and %d Destination Records", len(sources), len(dests))
	t.Logf("Total potential Cartesian pair space: %d combinations", len(sources)*len(dests))

	// Instantiate Match Engine with default configuration
	cfg := matcher.DefaultConfig()
	cfg.WorkerCount = 8
	cfg.MaxCandidatesPerSrc = 50
	engine := matcher.NewMatchEngine(cfg)

	t.Log("=== EXECUTING MATCHING ENGINE WITH IN-MEMORY BLOCKING INDEX ===")
	startTime := time.Now()

	results, progress := engine.ExecuteJob(context.Background(), "big-mock-batch-5000", sources, dests, nil)

	elapsed := time.Since(startTime)
	recPerSec := float64(len(sources)) / elapsed.Seconds()

	t.Logf("Execution Completed in %v (%.2f sources/sec)", elapsed, recPerSec)
	t.Logf("Total Candidate Matches Identified: %d", len(results))
	t.Logf("Auto-Matched Count (>=90%%): %d", progress.AutoMatched)
	t.Logf("Review Queue Count (70-89%%): %d", progress.ReviewNeeded)

	// Evaluate Ground-Truth Accuracy Metrics
	var tp, fp, tn, fn int

	matchedSet := make(map[string]bool)
	for _, res := range results {
		key := fmt.Sprintf("%s_%s", res.SourceID, res.DestinationID)
		matchedSet[key] = true
	}

	categoryStats := make(map[string]map[string]int)

	for _, pair := range labeledPairs {
		key := fmt.Sprintf("%s_%s", pair.Source.ID, pair.Destination.ID)
		isPredictedMatch := matchedSet[key]

		if _, exists := categoryStats[pair.Category]; !exists {
			categoryStats[pair.Category] = make(map[string]int)
		}

		if pair.IsMatch && isPredictedMatch {
			tp++
			categoryStats[pair.Category]["TP"]++
		} else if !pair.IsMatch && isPredictedMatch {
			fp++
			categoryStats[pair.Category]["FP"]++
		} else if !pair.IsMatch && !isPredictedMatch {
			tn++
			categoryStats[pair.Category]["TN"]++
		} else if pair.IsMatch && !isPredictedMatch {
			fn++
			categoryStats[pair.Category]["FN"]++
		}
	}

	precision := 0.0
	if (tp + fp) > 0 {
		precision = float64(tp) / float64(tp+fp)
	}

	recall := 0.0
	if (tp + fn) > 0 {
		recall = float64(tp) / float64(tp+fn)
	}

	f1Score := 0.0
	if (precision + recall) > 0 {
		f1Score = 2 * (precision * recall) / (precision + recall)
	}

	totalEvaluated := tp + fp + tn + fn
	accuracy := 0.0
	if totalEvaluated > 0 {
		accuracy = float64(tp+tn) / float64(totalEvaluated)
	}

	t.Log("==========================================================")
	t.Log("            ACCURACY & CONFIDENCE SCORE BENCHMARK REPORT    ")
	t.Log("==========================================================")
	t.Logf(" True Positives  (TP) : %d", tp)
	t.Logf(" False Positives (FP) : %d", fp)
	t.Logf(" True Negatives  (TN) : %d", tn)
	t.Logf(" False Negatives (FN) : %d", fn)
	t.Logf("----------------------------------------------------------")
	for cat, stats := range categoryStats {
		t.Logf(" Category [%s]: TP=%d, FN=%d, FP=%d, TN=%d", cat, stats["TP"], stats["FN"], stats["FP"], stats["TN"])
	}
	t.Logf("----------------------------------------------------------")
	t.Logf(" Precision            : %.2f%%", precision*100)
	t.Logf(" Recall               : %.2f%%", recall*100)
	t.Logf(" F1-Score             : %.2f%%", f1Score*100)
	t.Logf(" Overall Accuracy     : %.2f%%", accuracy*100)
	t.Logf(" Throughput Speed     : %.2f records/sec", recPerSec)
	t.Log("==========================================================")

	if accuracy < 0.90 {
		t.Errorf("Expected benchmark accuracy to be >= 90%%, got %.2f%%", accuracy*100)
	}
}
