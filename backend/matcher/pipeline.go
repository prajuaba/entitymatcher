package matcher

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type SourceRecord struct {
	ID              string                 `json:"id"`
	BatchID         string                 `json:"batch_id"`
	ReferenceID     string                 `json:"reference_id"`
	CustomerNameRaw string                 `json:"customer_name_raw"`
	NormalizedName  CleanName              `json:"normalized_name"`
	TransactionDate time.Time              `json:"transaction_date"`
	TransactionType string                 `json:"transaction_type"`
	Attributes      map[string]interface{} `json:"attributes,omitempty"`
}

type DestinationRecord struct {
	ID              string                 `json:"id"`
	BatchID         string                 `json:"batch_id"`
	CustomerID      string                 `json:"customer_id"`
	CustomerNameRaw string                 `json:"customer_name_raw"`
	NormalizedName  CleanName              `json:"normalized_name"`
	TransactionDate time.Time              `json:"transaction_date"`
	Attributes      map[string]interface{} `json:"attributes,omitempty"`
}

type MatchResultItem struct {
	ID              string             `json:"id"`
	BatchID         string             `json:"batch_id"`
	SourceID        string             `json:"source_id"`
	Source          *SourceRecord      `json:"source,omitempty"`
	DestinationID   string             `json:"destination_id"`
	Destination     *DestinationRecord `json:"destination,omitempty"`
	ConfidenceScore float64            `json:"confidence_score"`
	CalibratedScore float64            `json:"calibrated_score"` // Calibrated probability P(match), mirrors ConfidenceScore when calibration disabled
	NameScore       float64            `json:"name_score"`
	DateScore       float64            `json:"date_score"`
	JWScore         float64            `json:"jw_score"`
	LevScore        float64            `json:"lev_score"`
	TokenScore      float64            `json:"token_score"`
	TrigramScore    float64            `json:"trigram_score"`
	MatchStatus     string             `json:"match_status"` // AUTO_MATCHED, REVIEW_NEEDED, CONFIRMED, REJECTED, NO_MATCH
	MatchReasons    []string           `json:"match_reasons"`
	Rank            int                `json:"rank"`          // 1 = best candidate for this source
	ScoreMargin     float64            `json:"score_margin"`  // best - runner_up, 0 when no runner-up
	DecisionNote    string             `json:"decision_note"` // why this row got its status
	CreatedAt       time.Time          `json:"created_at"`
}

type Config struct {
	AutoMatchThreshold       float64          `json:"auto_match_threshold"` // Default: 0.90
	ReviewThreshold          float64          `json:"review_threshold"`     // Default: 0.70
	DateToleranceDays        int              `json:"date_tolerance_days"`  // Default: 30
	Weights                  MatchWeights     `json:"weights"`
	Algorithms               AlgorithmToggles `json:"algorithms"`
	ColumnMapping            ColumnMapping    `json:"column_mapping"`
	WorkerCount              int              `json:"worker_count"`
	MaxCandidatesPerSrc      int              `json:"max_candidates_per_src"`
	MarginThreshold          float64          `json:"margin_threshold"`            // Default: 0.05
	ExactMatchFloor          float64          `json:"exact_match_floor"`           // Default: 0.99
	AssignmentStrategy       string           `json:"assignment_strategy"`         // Default: "GREEDY_1_1"
	EmitUnmatched            bool             `json:"emit_unmatched"`              // Default: true
	MaxAlternativesPerSource int              `json:"max_alternatives_per_source"` // Default: 5. Use negative to keep all alternatives.
	CalibrationEnabled       bool             `json:"calibration_enabled"`         // Default: false. IMPORTANT: Only enable after fitting a calibrator on reviewed data from this deployment. A calibrator fitted on synthetic data encodes generator quirks, not production data patterns. See SetCalibrator().
}

func DefaultConfig() Config {
	return Config{
		AutoMatchThreshold:       0.90,
		ReviewThreshold:          0.70,
		DateToleranceDays:        30,
		Weights:                  DefaultWeights,
		Algorithms:               DefaultAlgorithms,
		ColumnMapping:            DefaultColumnMapping(),
		WorkerCount:              runtime.NumCPU() * 2,
		MaxCandidatesPerSrc:      50,
		MarginThreshold:          0.05,
		ExactMatchFloor:          0.99,
		AssignmentStrategy:       "GREEDY_1_1",
		EmitUnmatched:            true,
		MaxAlternativesPerSource: 5,
		CalibrationEnabled:       false,
	}
}

type BatchProgress struct {
	BatchID          string    `json:"batch_id"`
	TotalSources     int64     `json:"total_sources"`
	ProcessedSources int64     `json:"processed_sources"`
	TotalMatches     int64     `json:"total_candidate_pairs"`
	AutoMatched      int64     `json:"auto_matched"`
	ReviewNeeded     int64     `json:"review_needed"`
	NoMatchCount     int64     `json:"no_match_count"`
	TotalDecisions   int64     `json:"total_decisions"`
	Status           string    `json:"status"` // IDLE, RUNNING, COMPLETED, FAILED
	StartedAt        time.Time `json:"started_at"`
	CompletedAt      time.Time `json:"completed_at"`
	ElapsedMs        int64     `json:"elapsed_ms"`
}

type MatchEngine struct {
	Config     Config
	calibrator Calibrator // Optional calibrator for converting raw scores to probabilities
}

func NewMatchEngine(cfg Config) *MatchEngine {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = runtime.NumCPU() * 2
	}
	if cfg.MaxCandidatesPerSrc <= 0 {
		cfg.MaxCandidatesPerSrc = 50
	}
	if cfg.MarginThreshold == 0 {
		cfg.MarginThreshold = 0.05
	}
	if cfg.ExactMatchFloor == 0 {
		cfg.ExactMatchFloor = 0.99
	}
	if cfg.AssignmentStrategy == "" {
		cfg.AssignmentStrategy = "GREEDY_1_1"
	}
	if cfg.MaxAlternativesPerSource == 0 {
		cfg.MaxAlternativesPerSource = 5
	}
	return &MatchEngine{Config: cfg, calibrator: nil}
}

// SetCalibrator assigns a fitted Calibrator to the engine for score calibration.
// If the engine's CalibrationEnabled is true and a calibrator is set, scores will be
// calibrated to probabilities P(match) for threshold comparisons.
func (e *MatchEngine) SetCalibrator(cal Calibrator) {
	e.calibrator = cal
}

// IsAutoMatchable determines if a rank-1 match qualifies for auto-matching.
// It implements two decision rules:
// (a) Score meets auto-match threshold AND margin meets threshold (conservative rule)
// (b) Top score is an exact match (>= floor) AND runner-up is below floor (decisive rule)
// Critical: Rule (b) does NOT fire if both scores are >= floor (that's a genuine tie)
func IsAutoMatchable(topScore, runnerUpScore, autoMatchThreshold, marginThreshold, exactMatchFloor float64) (bool, string) {
	margin := topScore - runnerUpScore

	// Rule (b): Exact normalized match
	// Both conditions must be true: top >= floor AND runner-up < floor
	// This ensures rule (b) does NOT fire on ties (both >= floor)
	if topScore >= exactMatchFloor && runnerUpScore < exactMatchFloor {
		note := fmt.Sprintf("Exact normalized match; best fuzzy alternative scored %.3f", runnerUpScore)
		return true, note
	}

	// Rule (a): Conservative margin-based rule
	if topScore >= autoMatchThreshold && margin >= marginThreshold {
		return true, "Top candidate meets auto-match threshold and margin threshold"
	}

	// Below both rules
	if topScore >= autoMatchThreshold && margin < marginThreshold {
		return false, fmt.Sprintf("Ambiguous: runner-up within %.3f — needs review", margin)
	}
	return false, "Below auto-match threshold — needs review"
}

type matchTask struct {
	source SourceRecord
}

// ScoredCandidate represents a candidate with its calculated score and details
type ScoredCandidate struct {
	Candidate DestinationRecord
	ScoreRes  ScoreResult
}

// ExecuteJob runs matching over sources and dests using worker pool and blocking index.
// Progress callback is invoked periodically for SSE updates.
func (e *MatchEngine) ExecuteJob(
	ctx context.Context,
	batchID string,
	sources []SourceRecord,
	dests []DestinationRecord,
	onProgress func(BatchProgress),
) ([]MatchResultItem, BatchProgress) {
	startedAt := time.Now()
	totalSources := int64(len(sources))

	progress := BatchProgress{
		BatchID:      batchID,
		TotalSources: totalSources,
		Status:       "RUNNING",
		StartedAt:    startedAt,
	}

	if totalSources == 0 || len(dests) == 0 {
		progress.Status = "COMPLETED"
		progress.CompletedAt = time.Now()
		progress.ElapsedMs = progress.CompletedAt.Sub(startedAt).Milliseconds()
		return nil, progress
	}

	// Build corpus statistics for IDF weighting (once before worker pool)
	corpusStats := BuildCorpusStats(sources, dests)

	// Build High-Scale Blocking Index
	blockingIdx := NewBlockingIndexWithOptions(dests, 0.05, DefaultAbsoluteCeiling, e.Config.Algorithms.UseRomanizedMatch)

	tasks := make(chan matchTask, totalSources)
	resultsChan := make(chan []MatchResultItem, totalSources)

	var wg sync.WaitGroup
	var processedCount int64
	var autoMatchedCount int64
	var reviewNeededCount int64
	var noMatchCount int64
	var totalMatchesCount int64

	// Launch Workers
	workerCount := e.Config.WorkerCount
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case task, ok := <-tasks:
					if !ok {
						return
					}

					// Query top candidates via Blocking Index
					candidates := blockingIdx.QueryCandidates(task.source, e.Config.MaxCandidatesPerSrc)
					var matchedItems []MatchResultItem

					// Step 1: Score all candidates and keep those above ReviewThreshold
					var ScoredCandidates []ScoredCandidate

					for _, cand := range candidates {
						scoreRes := CalculateCompositeScoreWithCorpus(
							task.source.NormalizedName,
							cand.NormalizedName,
							task.source.TransactionDate,
							cand.TransactionDate,
							e.Config.Weights,
							e.Config.Algorithms,
							e.Config.DateToleranceDays,
							corpusStats,
						)

						// Evaluate dynamic secondary field mappings if configured
						secScore, secReasons, ok := EvaluateSecondaryFields(task.source.Attributes, cand.Attributes, e.Config.ColumnMapping.SecondaryFields)
						if !ok {
							continue // Mandatory field mismatch -> skip candidate
						}
						if len(secReasons) > 0 {
							scoreRes.MatchReasons = append(scoreRes.MatchReasons, secReasons...)
						}
						if len(e.Config.ColumnMapping.SecondaryFields) > 0 {
							scoreRes.TotalScore = (scoreRes.TotalScore * 0.8) + (secScore * 0.2)
						}

						// Keep candidates at or above ReviewThreshold
						if scoreRes.TotalScore >= e.Config.ReviewThreshold {
							ScoredCandidates = append(ScoredCandidates, ScoredCandidate{
								Candidate: cand,
								ScoreRes:  scoreRes,
							})
						}
					}

					// Step 2: Sort candidates by score descending, then by destination ID for determinism
					sort.Slice(ScoredCandidates, func(i, j int) bool {
						if ScoredCandidates[i].ScoreRes.TotalScore != ScoredCandidates[j].ScoreRes.TotalScore {
							return ScoredCandidates[i].ScoreRes.TotalScore > ScoredCandidates[j].ScoreRes.TotalScore
						}
						return ScoredCandidates[i].Candidate.ID < ScoredCandidates[j].Candidate.ID
					})

					// Step 3: Create MatchResultItems with Rank, ScoreMargin, and DecisionNote
					// DEFECT 4: Filter alternatives to retain only rank 1..MaxAlternativesPerSource
					// This prevents review queue flooding while keeping ranking metrics unchanged
					for rank, item := range ScoredCandidates {
						cand := item.Candidate
						scoreRes := item.ScoreRes
						rankNum := rank + 1

						// Skip alternatives beyond the limit if configured (0 or negative means keep all)
						if e.Config.MaxAlternativesPerSource > 0 && rankNum > e.Config.MaxAlternativesPerSource {
							continue
						}

						// Compute score margin and runner-up score for rank-1
						var margin float64
						var runnerUpScore float64
						if len(ScoredCandidates) > 1 && rank == 0 {
							runnerUpScore = ScoredCandidates[1].ScoreRes.TotalScore
							margin = ScoredCandidates[0].ScoreRes.TotalScore - runnerUpScore
						} else if rank == 0 && len(ScoredCandidates) == 1 {
							margin = ScoredCandidates[0].ScoreRes.TotalScore
						}

						// Compute calibrated score if enabled
						calibratedScore := scoreRes.TotalScore
						if e.Config.CalibrationEnabled && e.calibrator != nil {
							calibratedScore = e.calibrator.Calibrate(scoreRes.TotalScore)
						}

						// Compute calibrated runner-up score for threshold decisions
						calibratedRunnerUpScore := runnerUpScore
						if rankNum == 1 && runnerUpScore > 0 && e.Config.CalibrationEnabled && e.calibrator != nil {
							calibratedRunnerUpScore = e.calibrator.Calibrate(runnerUpScore)
						}

						// Propose decision for rank-1 only
						status := "REVIEW_NEEDED"
						note := ""

						if rankNum == 1 {
							// First-ranked candidate: apply decision rules via helper
							// Use calibrated score for decisions if calibration is enabled, else use raw score
							decisionScore := scoreRes.TotalScore
							decisionRunnerUp := runnerUpScore
							if e.Config.CalibrationEnabled && e.calibrator != nil {
								decisionScore = calibratedScore
								decisionRunnerUp = calibratedRunnerUpScore
							}

							canAutoMatch, decisionNote := IsAutoMatchable(
								decisionScore,
								decisionRunnerUp,
								e.Config.AutoMatchThreshold,
								e.Config.MarginThreshold,
								e.Config.ExactMatchFloor,
							)
							if canAutoMatch {
								status = "AUTO_MATCHED"
								atomic.AddInt64(&autoMatchedCount, 1)
							} else {
								atomic.AddInt64(&reviewNeededCount, 1)
							}
							note = decisionNote
						} else {
							// Rank >= 2: alternative for human review
							atomic.AddInt64(&reviewNeededCount, 1)
							note = fmt.Sprintf("Alternative candidate (rank %d) for review", rankNum)
						}

						srcCopy := task.source
						candCopy := cand

						matchedItems = append(matchedItems, MatchResultItem{
							ID:              batchID + "-" + task.source.ID + "-" + cand.ID,
							BatchID:         batchID,
							SourceID:        task.source.ID,
							Source:          &srcCopy,
							DestinationID:   cand.ID,
							Destination:     &candCopy,
							ConfidenceScore: scoreRes.TotalScore,
							CalibratedScore: calibratedScore,
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

						atomic.AddInt64(&totalMatchesCount, 1)
					}

					// Step 4: Handle NO_MATCH (A4) - when source has no candidates >= ReviewThreshold
					if len(ScoredCandidates) == 0 && e.Config.EmitUnmatched {
						note := ""
						if len(candidates) == 0 {
							note = "No blocking candidates found"
						} else {
							note = "All blocking candidates scored below review threshold"
						}

						srcCopy := task.source

						matchedItems = append(matchedItems, MatchResultItem{
							ID:            batchID + "-" + task.source.ID + "-NO_MATCH",
							BatchID:       batchID,
							SourceID:      task.source.ID,
							Source:        &srcCopy,
							DestinationID: "",
							Destination:   nil,
							MatchStatus:   "NO_MATCH",
							DecisionNote:  note,
							CreatedAt:     time.Now(),
						})

						atomic.AddInt64(&noMatchCount, 1)
					}

					resultsChan <- matchedItems
					cnt := atomic.AddInt64(&processedCount, 1)

					// Trigger SSE progress update periodically
					if onProgress != nil && (cnt%100 == 0 || cnt == totalSources) {
						onProgress(BatchProgress{
							BatchID:          batchID,
							TotalSources:     totalSources,
							ProcessedSources: cnt,
							TotalMatches:     atomic.LoadInt64(&totalMatchesCount),
							AutoMatched:      atomic.LoadInt64(&autoMatchedCount),
							ReviewNeeded:     atomic.LoadInt64(&reviewNeededCount),
							NoMatchCount:     atomic.LoadInt64(&noMatchCount),
							Status:           "RUNNING",
							StartedAt:        startedAt,
							ElapsedMs:        time.Since(startedAt).Milliseconds(),
						})
					}
				}
			}
		}()
	}

	// Feed tasks into channel
	go func() {
		for _, src := range sources {
			tasks <- matchTask{source: src}
		}
		close(tasks)
		wg.Wait()
		close(resultsChan)
	}()

	var allResults []MatchResultItem
	for res := range resultsChan {
		allResults = append(allResults, res...)
	}

	// After all workers finish, resolve assignments (A3)
	resolvedItems := ResolveAssignments(allResults, e.Config.AssignmentStrategy, e.Config)

	// Recompute counters and progress based on resolved items
	var finalAutoMatched int64
	var finalReviewNeeded int64
	var finalNoMatch int64

	for _, item := range resolvedItems {
		if item.Rank > 2 && e.Config.AssignmentStrategy == "TOP_1" {
			// TOP_1 strategy should have filtered these out already
			continue
		}
		switch item.MatchStatus {
		case "AUTO_MATCHED":
			finalAutoMatched++
		case "REVIEW_NEEDED":
			finalReviewNeeded++
		case "NO_MATCH":
			finalNoMatch++
		}
	}

	completedAt := time.Now()
	progress.Status = "COMPLETED"
	progress.ProcessedSources = atomic.LoadInt64(&processedCount)
	progress.TotalMatches = int64(len(resolvedItems))
	progress.AutoMatched = finalAutoMatched
	progress.ReviewNeeded = finalReviewNeeded
	progress.NoMatchCount = finalNoMatch
	progress.TotalDecisions = atomic.LoadInt64(&processedCount) // One decision per source
	progress.CompletedAt = completedAt
	progress.ElapsedMs = completedAt.Sub(startedAt).Milliseconds()

	if onProgress != nil {
		onProgress(progress)
	}

	return resolvedItems, progress
}
