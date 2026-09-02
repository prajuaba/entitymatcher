package mockdata

import (
	"context"
	"fmt"
	"math"
	"sort"
	"testing"

	"entitymatcher/matcher"
)

// HelperBrierScore computes the Brier score: mean((predicted - actual)^2)
func HelperBrierScore(predicted []float64, actual []bool) float64 {
	if len(predicted) == 0 || len(actual) == 0 || len(predicted) != len(actual) {
		return math.NaN()
	}

	sum := 0.0
	for i := range predicted {
		p := predicted[i]
		if p < 0.0 {
			p = 0.0
		}
		if p > 1.0 {
			p = 1.0
		}
		a := 0.0
		if actual[i] {
			a = 1.0
		}
		diff := p - a
		sum += diff * diff
	}
	return sum / float64(len(predicted))
}

// HelperECE computes Expected Calibration Error with nbins equal-width bins
func HelperECE(predicted []float64, actual []bool, nbins int) float64 {
	if len(predicted) == 0 || len(actual) == 0 || len(predicted) != len(actual) {
		return math.NaN()
	}

	type bin struct {
		predSum  float64
		count    int
		posCount int
	}
	bins := make([]bin, nbins)

	for i := range predicted {
		p := predicted[i]
		if p < 0.0 {
			p = 0.0
		}
		if p > 1.0 {
			p = 1.0
		}
		binIdx := int(p * float64(nbins))
		if binIdx == nbins {
			binIdx = nbins - 1
		}
		bins[binIdx].predSum += p
		bins[binIdx].count++
		if actual[i] {
			bins[binIdx].posCount++
		}
	}

	var weightedError float64
	totalSamples := len(predicted)

	for _, b := range bins {
		if b.count == 0 {
			continue
		}
		meanPred := b.predSum / float64(b.count)
		meanActual := float64(b.posCount) / float64(b.count)
		absDiff := math.Abs(meanPred - meanActual)
		weightedError += absDiff * float64(b.count)
	}

	if totalSamples == 0 {
		return math.NaN()
	}
	return weightedError / float64(totalSamples)
}

// HelperReliabilityTable prints a reliability diagram table with sample counts
func HelperReliabilityTable(predicted []float64, actual []bool, nbins int) {
	if len(predicted) == 0 || len(actual) == 0 || len(predicted) != len(actual) {
		fmt.Println("Error: Empty or mismatched arrays")
		return
	}

	type bin struct {
		start    float64
		end      float64
		predSum  float64
		count    int
		posCount int
	}
	bins := make([]bin, nbins)
	for i := range bins {
		bins[i] = bin{start: float64(i) / float64(nbins), end: float64(i+1) / float64(nbins)}
	}

	for i := range predicted {
		p := predicted[i]
		if p < 0.0 {
			p = 0.0
		}
		if p > 1.0 {
			p = 1.0
		}
		binIdx := int(p * float64(nbins))
		if binIdx == nbins {
			binIdx = nbins - 1
		}
		bins[binIdx].predSum += p
		bins[binIdx].count++
		if actual[i] {
			bins[binIdx].posCount++
		}
	}

	fmt.Println("\nReliability Table:")
	fmt.Printf("%-20s %-8s %-10s %-12s %-12s %-12s\n", "Predicted Range", "Count", "MeanPred", "ObservedFreq", "ECE Weight", "Pos/Total")
	fmt.Println(string(make([]byte, 85)))
	for _, b := range bins {
		rangeStr := fmt.Sprintf("[%.2f, %.2f)", b.start, b.end)
		countStr := fmt.Sprintf("%-8d", b.count)
		meanPredStr := "N/A"
		observedFreqStr := "N/A"
		weightStr := ""
		posStr := "N/A"

		if b.count > 0 {
			meanPred := b.predSum / float64(b.count)
			observedFreq := float64(b.posCount) / float64(b.count)
			meanPredStr = fmt.Sprintf("%.4f", meanPred)
			observedFreqStr = fmt.Sprintf("%.4f", observedFreq)
			absDiff := math.Abs(meanPred-observedFreq) * float64(b.count)
			weightStr = fmt.Sprintf("%.4f", absDiff)
			posStr = fmt.Sprintf("%d/%d", b.posCount, b.count)
		}

		fmt.Printf("%-20s %s %10s %12s %12s %12s\n",
			rangeStr, countStr, meanPredStr, observedFreqStr, weightStr, posStr)
	}
}

// deterministicSplit performs a 70/30 train/test split deterministically
func deterministicSplit(labels []matcher.LabelledScore, trainRatio float64) ([]matcher.LabelledScore, []matcher.LabelledScore) {
	if len(labels) == 0 {
		return nil, nil
	}

	sorted := make([]matcher.LabelledScore, len(labels))
	copy(sorted, labels)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Score != sorted[j].Score {
			return sorted[i].Score < sorted[j].Score
		}
		return !sorted[i].IsMatch && sorted[j].IsMatch
	})

	nTrain := int(math.Floor(float64(len(sorted)) * trainRatio))
	if nTrain < 0 {
		nTrain = 0
	}
	if nTrain > len(sorted) {
		nTrain = len(sorted)
	}

	train := sorted[:nTrain]
	test := sorted[nTrain:]
	return train, test
}

// TestCalibratorFitting tests calibrator fitting and evaluation on ALL candidate pairs
func TestCalibratorFitting(t *testing.T) {
	const datasetSize = 1000

	t.Logf("Generating mock dataset with %d records...", datasetSize)
	sources, dests, groundTruthMatches, labeledPairs := GenerateBigMockDataset(datasetSize)

	// Run engine with MaxAlternativesPerSource = -1 to keep ALL candidates above review threshold
	cfg := matcher.DefaultConfig()
	cfg.WorkerCount = 4
	cfg.MaxCandidatesPerSrc = 50
	cfg.MaxAlternativesPerSource = -1 // Keep all alternatives
	engine := matcher.NewMatchEngine(cfg)

	t.Log("Running matching engine (keeping all candidates for observation collection)...")
	results, _ := engine.ExecuteJob(context.Background(), "calibration-test-batch", sources, dests, nil)

	// Build map of true matches from ground truth
	trueMatches := make(map[string]string) // sourceID -> destinationID
	for _, pair := range labeledPairs {
		if pair.IsMatch {
			key := fmt.Sprintf("%s_%s", pair.Source.ID, pair.Destination.ID)
			if groundTruthMatches[key] {
				trueMatches[pair.Source.ID] = pair.Destination.ID
			}
		}
	}

	// Extract ALL scored pairs (not just rank-1) and label by ground truth
	// This includes high-scoring negatives (near-misses) which are calibration signal
	var allObs []matcher.LabelledScore
	for _, res := range results {
		if res.MatchStatus != "NO_MATCH" { // Skip NO_MATCH (score is 0)
			trueDestID := trueMatches[res.SourceID]
			isMatch := (res.DestinationID == trueDestID)
			allObs = append(allObs, matcher.LabelledScore{
				Score:   res.ConfidenceScore,
				IsMatch: isMatch,
			})
		}
	}

	if len(allObs) == 0 {
		t.Fatalf("No observations extracted from results")
	}

	// Count class distribution
	var nPos, nNeg int
	for _, obs := range allObs {
		if obs.IsMatch {
			nPos++
		} else {
			nNeg++
		}
	}
	posRatio := float64(nPos) / float64(len(allObs))

	// Deterministic train/test split (70/30)
	trainObs, testObs := deterministicSplit(allObs, 0.7)

	if len(trainObs) == 0 || len(testObs) == 0 {
		t.Fatalf("Empty train or test split: train=%d, test=%d", len(trainObs), len(testObs))
	}

	// Count test class distribution
	var testPos, testNeg int
	for _, obs := range testObs {
		if obs.IsMatch {
			testPos++
		} else {
			testNeg++
		}
	}
	testPosRatio := float64(testPos) / float64(len(testObs))

	// Extract test scores and labels
	testScores := make([]float64, len(testObs))
	testLabels := make([]bool, len(testObs))
	for i := range testObs {
		testScores[i] = testObs[i].Score
		testLabels[i] = testObs[i].IsMatch
	}

	fmt.Println("\n=== Calibration Evaluation (ALL Candidate Pairs) ===")
	fmt.Printf("Total observations:            %d\n", len(allObs))
	fmt.Printf("Class distribution (all):      %d positive (%.1f%%), %d negative (%.1f%%)\n",
		nPos, posRatio*100, nNeg, (1-posRatio)*100)
	fmt.Printf("Training split:                %d samples\n", len(trainObs))
	fmt.Printf("Test split:                    %d samples\n", len(testObs))
	fmt.Printf("Test class distribution:       %d positive (%.1f%%), %d negative (%.1f%%)\n",
		testPos, testPosRatio*100, testNeg, (1-testPosRatio)*100)

	// Evaluate raw scores (before calibration)
	rawBrier := HelperBrierScore(testScores, testLabels)
	rawECE := HelperECE(testScores, testLabels, 10)

	fmt.Println("\n--- BEFORE Calibration (Raw Scores) ---")
	fmt.Printf("Brier Score:                  %.8f\n", rawBrier)
	fmt.Printf("ECE:                          %.8f\n", rawECE)
	HelperReliabilityTable(testScores, testLabels, 10)

	// Fit calibrator on train split
	t.Logf("Fitting calibrator on %d training samples...", len(trainObs))
	calibrator, err := matcher.FitCalibrator(trainObs)
	if err != nil {
		t.Fatalf("FitCalibrator failed: %v", err)
	}

	t.Logf("Fitted calibrator type: %T", calibrator)

	// Evaluate calibrated scores on test split
	calibratedScores := make([]float64, len(testScores))
	for i, s := range testScores {
		calibratedScores[i] = calibrator.Calibrate(s)
		if calibratedScores[i] < 0 || calibratedScores[i] > 1 {
			t.Errorf("Calibrated score out of range: %.6f", calibratedScores[i])
		}
	}

	calBrier := HelperBrierScore(calibratedScores, testLabels)
	calECE := HelperECE(calibratedScores, testLabels, 10)

	fmt.Println("\n--- AFTER Calibration ---")
	fmt.Printf("Brier Score:                  %.8f (delta: %+.8f)\n", calBrier, calBrier-rawBrier)
	fmt.Printf("ECE:                          %.8f (delta: %+.8f)\n", calECE, calECE-rawECE)
	HelperReliabilityTable(calibratedScores, testLabels, 10)

	// Assert monotonicity
	uniqueScores := make([]float64, 0)
	seen := make(map[float64]bool)
	for _, s := range testScores {
		if !seen[s] {
			uniqueScores = append(uniqueScores, s)
			seen[s] = true
		}
	}
	sort.Float64s(uniqueScores)

	isMono := matcher.IsMonotonic(uniqueScores, calibrator)
	fmt.Printf("\nMonotonicity check: %v (calibration preserves ranking)\n", isMono)
	if !isMono {
		t.Errorf("Calibrator is not monotonic")
	}

	// Test JSON roundtrip
	fmt.Println("\nTesting serialization roundtrip...")
	jsonData, err := calibrator.(interface {
		MarshalJSON() ([]byte, error)
	}).MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var unmarshaledCal matcher.Calibrator
	switch calibrator.(type) {
	case *matcher.PlattCalibrator:
		var platt matcher.PlattCalibrator
		if err := platt.UnmarshalJSON(jsonData); err != nil {
			t.Fatalf("PlattUnmarshalJSON failed: %v", err)
		}
		unmarshaledCal = &platt
	case *matcher.IsotonicCalibrator:
		var iso matcher.IsotonicCalibrator
		if err := iso.UnmarshalJSON(jsonData); err != nil {
			t.Fatalf("IsotonicUnmarshalJSON failed: %v", err)
		}
		unmarshaledCal = &iso
	case *matcher.IdentityCalibrator:
		var id matcher.IdentityCalibrator
		if err := id.UnmarshalJSON(jsonData); err != nil {
			t.Fatalf("IdentityUnmarshalJSON failed: %v", err)
		}
		unmarshaledCal = &id
	default:
		t.Fatalf("Unknown calibrator type: %T", calibrator)
	}

	for _, s := range uniqueScores {
		c1 := calibrator.Calibrate(s)
		c2 := unmarshaledCal.Calibrate(s)
		if math.Abs(c1-c2) > 1e-9 {
			t.Errorf("Calibration mismatch after roundtrip for score %.4f: before=%.9f, after=%.9f", s, c1, c2)
		}
	}

	fmt.Println("Serialization roundtrip test passed")
	fmt.Println("\n=== All calibration tests passed ===")
}

// TestCalibratorBenchmarkComparison runs baseline vs calibrated multiple times to measure effect size
func TestCalibratorBenchmarkComparison(t *testing.T) {
	const datasetSize = 2000
	const numRuns = 3

	t.Logf("Generating mock dataset with %d records...", datasetSize)
	sources, dests, groundTruthMatches, labeledPairs := GenerateBigMockDataset(datasetSize)

	// Build map of true matches
	trueMatches := make(map[string]string)
	for _, pair := range labeledPairs {
		if pair.IsMatch {
			key := fmt.Sprintf("%s_%s", pair.Source.ID, pair.Destination.ID)
			if groundTruthMatches[key] {
				trueMatches[pair.Source.ID] = pair.Destination.ID
			}
		}
	}

	// First, fit a calibrator from a single baseline run with all candidates
	t.Log("Fitting calibrator from baseline run...")
	cfg := matcher.DefaultConfig()
	cfg.WorkerCount = 4
	cfg.MaxCandidatesPerSrc = 50
	cfg.MaxAlternativesPerSource = -1
	engine := matcher.NewMatchEngine(cfg)

	baselineResults, _ := engine.ExecuteJob(context.Background(), "calibration-fit", sources, dests, nil)

	// Extract all pairs for fitting
	var fitObs []matcher.LabelledScore
	for _, res := range baselineResults {
		if res.MatchStatus != "NO_MATCH" {
			trueDestID := trueMatches[res.SourceID]
			isMatch := (res.DestinationID == trueDestID)
			fitObs = append(fitObs, matcher.LabelledScore{
				Score:   res.ConfidenceScore,
				IsMatch: isMatch,
			})
		}
	}

	trainObs, _ := deterministicSplit(fitObs, 0.7)
	calibrator, err := matcher.FitCalibrator(trainObs)
	if err != nil {
		t.Fatalf("FitCalibrator failed: %v", err)
	}

	// Now run multiple times: baseline and calibrated
	baselineTPValues := make([]int, numRuns)
	baselineFPValues := make([]int, numRuns)
	calibratedTPValues := make([]int, numRuns)
	calibratedFPValues := make([]int, numRuns)

	for run := 0; run < numRuns; run++ {
		t.Logf("Run %d/%d: baseline...", run+1, numRuns)
		cfg1 := matcher.DefaultConfig()
		cfg1.WorkerCount = 4
		cfg1.MaxCandidatesPerSrc = 50
		cfg1.CalibrationEnabled = false
		engine1 := matcher.NewMatchEngine(cfg1)

		results1, _ := engine1.ExecuteJob(context.Background(), fmt.Sprintf("baseline-%d", run), sources, dests, nil)

		// Count rank-1 decisions
		var tp1, fp1 int
		seenSrc := make(map[string]bool)
		for _, res := range results1 {
			if res.Rank == 1 && !seenSrc[res.SourceID] {
				seenSrc[res.SourceID] = true
				trueDestID := trueMatches[res.SourceID]
				if res.MatchStatus == "AUTO_MATCHED" {
					if res.DestinationID == trueDestID {
						tp1++
					} else {
						fp1++
					}
				}
			}
		}
		baselineTPValues[run] = tp1
		baselineFPValues[run] = fp1

		t.Logf("Run %d/%d: calibrated...", run+1, numRuns)
		cfg2 := matcher.DefaultConfig()
		cfg2.WorkerCount = 4
		cfg2.MaxCandidatesPerSrc = 50
		cfg2.CalibrationEnabled = true
		engine2 := matcher.NewMatchEngine(cfg2)
		engine2.SetCalibrator(calibrator)

		results2, _ := engine2.ExecuteJob(context.Background(), fmt.Sprintf("calibrated-%d", run), sources, dests, nil)

		// Count rank-1 decisions
		var tp2, fp2 int
		seenSrc2 := make(map[string]bool)
		for _, res := range results2 {
			if res.Rank == 1 && !seenSrc2[res.SourceID] {
				seenSrc2[res.SourceID] = true
				trueDestID := trueMatches[res.SourceID]
				if res.MatchStatus == "AUTO_MATCHED" {
					if res.DestinationID == trueDestID {
						tp2++
					} else {
						fp2++
					}
				}
			}
		}
		calibratedTPValues[run] = tp2
		calibratedFPValues[run] = fp2
	}

	// Compute statistics
	meanTP1 := float64(0)
	minTP1, maxTP1 := baselineTPValues[0], baselineTPValues[0]
	for _, v := range baselineTPValues {
		meanTP1 += float64(v)
		if v < minTP1 {
			minTP1 = v
		}
		if v > maxTP1 {
			maxTP1 = v
		}
	}
	meanTP1 /= float64(numRuns)

	meanTP2 := float64(0)
	minTP2, maxTP2 := calibratedTPValues[0], calibratedTPValues[0]
	for _, v := range calibratedTPValues {
		meanTP2 += float64(v)
		if v < minTP2 {
			minTP2 = v
		}
		if v > maxTP2 {
			maxTP2 = v
		}
	}
	meanTP2 /= float64(numRuns)

	meanFP1 := float64(0)
	for _, v := range baselineFPValues {
		meanFP1 += float64(v)
	}
	meanFP1 /= float64(numRuns)

	meanFP2 := float64(0)
	for _, v := range calibratedFPValues {
		meanFP2 += float64(v)
	}
	meanFP2 /= float64(numRuns)

	// Report results
	fmt.Println("\n=== BENCHMARK COMPARISON (Multiple Runs) ===")
	fmt.Printf("\nBaseline (CalibrationEnabled=false), %d runs:\n", numRuns)
	fmt.Printf("  True Positives:  %.1f (range: %d–%d, spread: %d)\n", meanTP1, minTP1, maxTP1, maxTP1-minTP1)
	fmt.Printf("  False Positives: %.1f (range: %d–%d)\n", meanFP1, baselineFPValues[0], baselineFPValues[0])

	fmt.Printf("\nCalibrated (CalibrationEnabled=true), %d runs:\n", numRuns)
	fmt.Printf("  True Positives:  %.1f (range: %d–%d, spread: %d)\n", meanTP2, minTP2, maxTP2, maxTP2-minTP2)
	fmt.Printf("  False Positives: %.1f (range: %d–%d)\n", meanFP2, calibratedFPValues[0], calibratedFPValues[0])

	tpDelta := meanTP2 - meanTP1
	tpSpread := math.Max(float64(maxTP1-minTP1), float64(maxTP2-minTP2))

	fmt.Printf("\nTP Delta:        %+.1f\n", tpDelta)
	fmt.Printf("Measurement noise (spread):  %.1f (%d runs)\n", tpSpread, numRuns)

	if math.Abs(tpDelta) > tpSpread {
		fmt.Printf("CONCLUSION: Effect is larger than measurement noise\n")
	} else {
		fmt.Printf("CONCLUSION: Delta %.1f is within measurement noise (±%.1f). No significant effect detected.\n", tpDelta, tpSpread)
	}
}
