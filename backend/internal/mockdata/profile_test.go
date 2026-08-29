package mockdata

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"

	"entitymatcher/matcher"
)

// GCMetrics holds GC statistics during a profiling run
type GCMetrics struct {
	NumGCBefore          uint32
	NumGCAfter           uint32
	PauseTotalNsBefore   uint64
	PauseTotalNsAfter    uint64
	GCCPUFractionPercent float64
}

// CorpusSizeReport holds profiling results for one corpus size
type CorpusSizeReport struct {
	Size                  int
	TotalExecuteJobTimeMs float64
	RetrievalTimeMs       float64
	RetrievalPercent      float64
	ScoringTimeMs         float64
	ScoringPercent        float64
	AssignmentTimeMs      float64
	AssignmentPercent     float64
	Throughput            float64
	GCStats               GCMetrics
}

// CandidateSetStats aggregates per-QueryCandidates observations
type CandidateSetStats struct {
	DistinctDestsTouched []int
	PostingListWalks     []int
	SliceSize            []int
	FallbackCount        int
	TotalQueries         int
}

func (css *CandidateSetStats) Mean(vals []int) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum int64
	for _, v := range vals {
		sum += int64(v)
	}
	return float64(sum) / float64(len(vals))
}

func (css *CandidateSetStats) P50(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]int, len(vals))
	copy(sorted, vals)
	sort.Ints(sorted)
	return sorted[len(sorted)/2]
}

func (css *CandidateSetStats) P95(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]int, len(vals))
	copy(sorted, vals)
	sort.Ints(sorted)
	idx := len(sorted) * 95 / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func (css *CandidateSetStats) Max(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	maxVal := vals[0]
	for _, v := range vals {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

// TestProfileExecution runs detailed profiling at 4 corpus sizes
func TestProfileExecution(t *testing.T) {
	if os.Getenv("PROFILE_TEST") != "1" {
		t.Skip("Skipping profile test. Set PROFILE_TEST=1")
	}

	sizes := []int{10000, 25000, 50000, 100000}
	const batchID = "profile-batch"

	var reports []CorpusSizeReport

	for _, corpusSize := range sizes {
		t.Logf("\n=== Profiling corpus size N=%d ===", corpusSize)

		runtime.GC()
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		// Generate dataset
		sources, dests, _, _ := GenerateBigMockDataset(corpusSize)
		t.Logf("Generated %d sources, %d destinations", len(sources), len(dests))

		runtime.ReadMemStats(&m2)

		// Build blocking index with profiling metrics enabled
		blockingIdx := matcher.NewBlockingIndex(dests)
		metrics := &matcher.QueryMetrics{Enabled: true}
		blockingIdx.Metrics = metrics

		// Snapshot GC before execution
		var gcBefore runtime.MemStats
		runtime.ReadMemStats(&gcBefore)

		// Single-threaded execution for accurate wall-time measurement
		cfg := matcher.DefaultConfig()

		var totalRetrievalNs int64
		var totalScoringNs int64
		var autoMatchedCount int64
		var reviewNeededCount int64
		var noMatchCount int64
		var totalMatchesCount int64

		var allResults []matcher.MatchResultItem

		execStart := time.Now()

		// Process sources one by one for accurate wall-time measurement
		for _, src := range sources {
			// Step 1: Query candidates (measure retrieval time)
			candidateStart := time.Now()
			candidates := blockingIdx.QueryCandidates(src, cfg.MaxCandidatesPerSrc)
			retrievalTime := time.Since(candidateStart)
			totalRetrievalNs += retrievalTime.Nanoseconds()

			// Step 2: Score candidates (measure scoring time)
			var scoredCandidates []matcher.ScoredCandidate
			scoringStart := time.Now()

			for _, cand := range candidates {
				scoreRes := matcher.CalculateCompositeScore(
					src.NormalizedName,
					cand.NormalizedName,
					src.TransactionDate,
					cand.TransactionDate,
					cfg.Weights,
					cfg.Algorithms,
					cfg.DateToleranceDays,
				)

				// Evaluate secondary fields (part of scoring)
				secScore, secReasons, ok := matcher.EvaluateSecondaryFields(
					src.Attributes,
					cand.Attributes,
					cfg.ColumnMapping.SecondaryFields,
				)
				if !ok {
					continue
				}
				if len(secReasons) > 0 {
					scoreRes.MatchReasons = append(scoreRes.MatchReasons, secReasons...)
				}
				if len(cfg.ColumnMapping.SecondaryFields) > 0 {
					scoreRes.TotalScore = (scoreRes.TotalScore * 0.8) + (secScore * 0.2)
				}

				if scoreRes.TotalScore >= cfg.ReviewThreshold {
					scoredCandidates = append(scoredCandidates, matcher.ScoredCandidate{
						Candidate: cand,
						ScoreRes:  scoreRes,
					})
				}
			}
			scoringTime := time.Since(scoringStart)
			totalScoringNs += scoringTime.Nanoseconds()

			// Step 3: Build match results
			var matchedItems []matcher.MatchResultItem
			sort.Slice(scoredCandidates, func(i, j int) bool {
				if scoredCandidates[i].ScoreRes.TotalScore != scoredCandidates[j].ScoreRes.TotalScore {
					return scoredCandidates[i].ScoreRes.TotalScore > scoredCandidates[j].ScoreRes.TotalScore
				}
				return scoredCandidates[i].Candidate.ID < scoredCandidates[j].Candidate.ID
			})

			for rank, item := range scoredCandidates {
				rankNum := rank + 1
				if cfg.MaxAlternativesPerSource > 0 && rankNum > cfg.MaxAlternativesPerSource {
					continue
				}

				cand := item.Candidate
				scoreRes := item.ScoreRes

				var margin float64
				var runnerUpScore float64
				if len(scoredCandidates) > 1 && rank == 0 {
					runnerUpScore = scoredCandidates[1].ScoreRes.TotalScore
					margin = scoredCandidates[0].ScoreRes.TotalScore - runnerUpScore
				} else if rank == 0 && len(scoredCandidates) == 1 {
					margin = scoredCandidates[0].ScoreRes.TotalScore
				}

				status := "REVIEW_NEEDED"
				note := ""

				if rankNum == 1 {
					canAutoMatch, decisionNote := matcher.IsAutoMatchable(
						scoreRes.TotalScore,
						runnerUpScore,
						cfg.AutoMatchThreshold,
						cfg.MarginThreshold,
						cfg.ExactMatchFloor,
					)
					if canAutoMatch {
						status = "AUTO_MATCHED"
						autoMatchedCount++
					} else {
						reviewNeededCount++
					}
					note = decisionNote
				} else {
					reviewNeededCount++
					note = fmt.Sprintf("Alternative candidate (rank %d) for review", rankNum)
				}

				srcCopy := src
				candCopy := cand

				matchedItems = append(matchedItems, matcher.MatchResultItem{
					ID:              batchID + "-" + src.ID + "-" + cand.ID,
					BatchID:         batchID,
					SourceID:        src.ID,
					Source:          &srcCopy,
					DestinationID:   cand.ID,
					Destination:     &candCopy,
					ConfidenceScore: scoreRes.TotalScore,
					NameScore:       scoreRes.NameScore,
					DateScore:       scoreRes.DateScore,
					JWScore:         scoreRes.JWScore,
					LevScore:        scoreRes.LevScore,
					TokenScore:      scoreRes.TokenScore,
					TrigramScore:    scoreRes.TrigramScore,
					MatchStatus:     status,
					MatchReasons:    scoreRes.MatchReasons,
					Rank:            rankNum,
					ScoreMargin:     margin,
					DecisionNote:    note,
					CreatedAt:       time.Now(),
				})

				totalMatchesCount++
			}

			if len(scoredCandidates) == 0 && cfg.EmitUnmatched {
				note := ""
				if len(candidates) == 0 {
					note = "No blocking candidates found"
				} else {
					note = "All blocking candidates scored below review threshold"
				}

				srcCopy := src
				matchedItems = append(matchedItems, matcher.MatchResultItem{
					ID:            batchID + "-" + src.ID + "-NO_MATCH",
					BatchID:       batchID,
					SourceID:      src.ID,
					Source:        &srcCopy,
					DestinationID: "",
					Destination:   nil,
					MatchStatus:   "NO_MATCH",
					DecisionNote:  note,
					CreatedAt:     time.Now(),
				})

				noMatchCount++
			}

			allResults = append(allResults, matchedItems...)
		}

		totalExecuteJobTime := time.Since(execStart)

		// Measure assignment time (separate for accuracy)
		assignmentStart := time.Now()
		_ = matcher.ResolveAssignments(allResults, cfg.AssignmentStrategy, cfg)
		assignmentTime := time.Since(assignmentStart)

		// Snapshot GC after execution
		var gcAfter runtime.MemStats
		runtime.ReadMemStats(&gcAfter)

		// Calculate totals
		totalRetrievalMs := float64(totalRetrievalNs) / 1e6
		totalScoringMs := float64(totalScoringNs) / 1e6
		totalAssignmentMs := float64(assignmentTime.Nanoseconds()) / 1e6
		totalTimeMs := totalExecuteJobTime.Seconds() * 1000

		retrievalPct := (totalRetrievalMs / totalTimeMs) * 100
		scoringPct := (totalScoringMs / totalTimeMs) * 100
		assignmentPct := (totalAssignmentMs / totalTimeMs) * 100

		throughput := float64(len(sources)) / totalExecuteJobTime.Seconds()

		// GC metrics
		gcNumBefore := gcBefore.NumGC
		gcNumAfter := gcAfter.NumGC
		gcNumDuring := gcNumAfter - gcNumBefore
		gcPauseNsBefore := gcBefore.PauseNs[(gcBefore.NumGC+255)%256]
		gcPauseNsAfter := gcAfter.PauseNs[(gcAfter.NumGC+255)%256]
		var gcPauseTotal int64
		for i := 0; i < 256; i++ {
			if i < len(gcAfter.PauseNs) {
				gcPauseTotal += int64(gcAfter.PauseNs[i])
			}
		}
		gcCPUFraction := (gcAfter.GCCPUFraction * 100)

		report := CorpusSizeReport{
			Size:                  corpusSize,
			TotalExecuteJobTimeMs: totalTimeMs,
			RetrievalTimeMs:       totalRetrievalMs,
			RetrievalPercent:      retrievalPct,
			ScoringTimeMs:         totalScoringMs,
			ScoringPercent:        scoringPct,
			AssignmentTimeMs:      totalAssignmentMs,
			AssignmentPercent:     assignmentPct,
			Throughput:            throughput,
			GCStats: GCMetrics{
				NumGCBefore:          gcNumBefore,
				NumGCAfter:           gcNumAfter,
				PauseTotalNsBefore:   gcPauseNsBefore,
				PauseTotalNsAfter:    gcPauseNsAfter,
				GCCPUFractionPercent: gcCPUFraction,
			},
		}
		reports = append(reports, report)

		// Log candidate set stats from BlockingIndex metrics
		css := &CandidateSetStats{
			DistinctDestsTouched: metrics.DistinctDestsTouched,
			PostingListWalks:     metrics.PostingListWalks,
			SliceSize:            metrics.SliceSize,
			FallbackCount:        metrics.FallbackCount,
			TotalQueries:         len(sources),
		}

		fallbackRate := float64(css.FallbackCount) / float64(css.TotalQueries) * 100

		t.Logf("\n--- TABLE B: Candidate-Set Growth (N=%d) ---", corpusSize)
		t.Logf("Distinct Destinations Touched:")
		t.Logf("  Mean=%.0f, P50=%v, P95=%v, Max=%v",
			css.Mean(css.DistinctDestsTouched),
			css.P50(css.DistinctDestsTouched),
			css.P95(css.DistinctDestsTouched),
			css.Max(css.DistinctDestsTouched))
		t.Logf("Posting List Entries Walked:")
		t.Logf("  Mean=%.0f, P50=%v, P95=%v, Max=%v",
			css.Mean(css.PostingListWalks),
			css.P50(css.PostingListWalks),
			css.P95(css.PostingListWalks),
			css.Max(css.PostingListWalks))
		t.Logf("Slice Size for sort.Slice:")
		t.Logf("  Mean=%.2f, P50=%v, P95=%v, Max=%v",
			css.Mean(css.SliceSize),
			css.P50(css.SliceSize),
			css.P95(css.SliceSize),
			css.Max(css.SliceSize))
		t.Logf("\n--- TABLE C: Fallback Rate (N=%d) ---", corpusSize)
		t.Logf("Fallback Count: %d / %d sources (%.2f%%)", css.FallbackCount, css.TotalQueries, fallbackRate)

		t.Logf("\n--- TABLE D: GC Pressure (N=%d) ---", corpusSize)
		t.Logf("GC Collections During ExecuteJob: %d (before=%d, after=%d)",
			gcNumDuring, gcNumBefore, gcNumAfter)
		t.Logf("GC Pause Total Ns: %d (sum of pause times)", gcPauseTotal)
		t.Logf("GC CPU Fraction: %.2f%%", gcCPUFraction)
		t.Logf("GC time as %% of wall time: %.2f%%", (gcCPUFraction / 100.0 * 100))

		// Cleanup
		sources = nil
		dests = nil
		runtime.GC()
	}

	// Print TABLE A: Time Split (summary across all sizes)
	t.Logf("\n\n=== TABLE A: Time Split Across All Sizes ===")
	t.Logf("%-10s | %-15s | %-15s(%%%%) | %-15s(%%%%) | %-15s(%%%%) | %-12s",
		"N", "Total(ms)", "Retrieval(ms)", "Scoring(ms)", "Assignment(ms)", "Throughput")
	t.Logf("-----------|-----------------|------------------|------------------|------------------|---------------")
	for _, r := range reports {
		t.Logf("%-10d | %-15.2f | %-8.2f(%-5.1f%%) | %-8.2f(%-5.1f%%) | %-8.2f(%-5.1f%%) | %-12.2f",
			r.Size,
			r.TotalExecuteJobTimeMs,
			r.RetrievalTimeMs, r.RetrievalPercent,
			r.ScoringTimeMs, r.ScoringPercent,
			r.AssignmentTimeMs, r.AssignmentPercent,
			r.Throughput)
	}

	// Analysis
	t.Logf("\n=== Analysis ===")
	if len(reports) >= 2 {
		r1 := reports[0]
		r2 := reports[len(reports)-1]
		sizeRatio := float64(r2.Size) / float64(r1.Size)
		timeRatio := r2.TotalExecuteJobTimeMs / r1.TotalExecuteJobTimeMs

		retrievalRatio := r2.RetrievalTimeMs / r1.RetrievalTimeMs
		scoringRatio := r2.ScoringTimeMs / r1.ScoringTimeMs
		assignmentRatio := r2.AssignmentTimeMs / r1.AssignmentTimeMs

		t.Logf("Size ratio (100k/10k): %.2fx", sizeRatio)
		t.Logf("Total time ratio (100k/10k): %.2fx (O(N^%.2f))", timeRatio, logX(timeRatio)/logX(sizeRatio))
		t.Logf("Retrieval time scales as ~O(N^%.2f)", logX(retrievalRatio)/logX(sizeRatio))
		t.Logf("Scoring time scales as ~O(N^%.2f)", logX(scoringRatio)/logX(sizeRatio))
		t.Logf("Assignment time scales as ~O(N^%.2f)", logX(assignmentRatio)/logX(sizeRatio))
	}
}

func logX(val float64) float64 {
	if val <= 0 {
		return 0
	}
	const ln2 = 0.693147180559945309417232121458
	return logLn(val) / ln2
}

func logLn(val float64) float64 {
	// Simple natural log approximation for positive numbers
	if val <= 0 {
		return 0
	}
	if val == 1 {
		return 0
	}
	// Use math.Log if available; we'll use a rough implementation
	// For this test, we just need relative scaling
	x := val
	y := 0.0
	for x > 1.1 {
		x /= 1.1
		y += 0.0953101798043248
	}
	for x < 0.9 {
		x /= 0.9
		y -= 0.1053605156578263
	}
	return y
}
