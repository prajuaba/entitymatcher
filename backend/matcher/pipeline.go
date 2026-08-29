package matcher

import (
	"context"
	"runtime"
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
	NameScore       float64            `json:"name_score"`
	DateScore       float64            `json:"date_score"`
	JWScore         float64            `json:"jw_score"`
	LevScore        float64            `json:"lev_score"`
	TokenScore      float64            `json:"token_score"`
	TrigramScore    float64            `json:"trigram_score"`
	MatchStatus     string             `json:"match_status"` // AUTO_MATCHED, REVIEW_NEEDED, CONFIRMED, REJECTED
	MatchReasons    []string           `json:"match_reasons"`
	CreatedAt       time.Time          `json:"created_at"`
}

type Config struct {
	AutoMatchThreshold  float64          `json:"auto_match_threshold"` // Default: 0.90
	ReviewThreshold     float64          `json:"review_threshold"`     // Default: 0.70
	DateToleranceDays   int              `json:"date_tolerance_days"`  // Default: 30
	Weights             MatchWeights     `json:"weights"`
	Algorithms          AlgorithmToggles `json:"algorithms"`
	ColumnMapping       ColumnMapping    `json:"column_mapping"`
	WorkerCount         int              `json:"worker_count"`
	MaxCandidatesPerSrc int              `json:"max_candidates_per_src"`
}

func DefaultConfig() Config {
	return Config{
		AutoMatchThreshold:  0.90,
		ReviewThreshold:     0.70,
		DateToleranceDays:   30,
		Weights:             DefaultWeights,
		Algorithms:          DefaultAlgorithms,
		ColumnMapping:       DefaultColumnMapping(),
		WorkerCount:         runtime.NumCPU() * 2,
		MaxCandidatesPerSrc: 50,
	}
}

type BatchProgress struct {
	BatchID          string    `json:"batch_id"`
	TotalSources     int64     `json:"total_sources"`
	ProcessedSources int64     `json:"processed_sources"`
	TotalMatches     int64     `json:"total_matches"`
	AutoMatched      int64     `json:"auto_matched"`
	ReviewNeeded     int64     `json:"review_needed"`
	Status           string    `json:"status"` // IDLE, RUNNING, COMPLETED, FAILED
	StartedAt        time.Time `json:"started_at"`
	CompletedAt      time.Time `json:"completed_at"`
	ElapsedMs        int64     `json:"elapsed_ms"`
}

type MatchEngine struct {
	Config Config
}

func NewMatchEngine(cfg Config) *MatchEngine {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = runtime.NumCPU() * 2
	}
	if cfg.MaxCandidatesPerSrc <= 0 {
		cfg.MaxCandidatesPerSrc = 50
	}
	return &MatchEngine{Config: cfg}
}

type matchTask struct {
	source SourceRecord
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

	// Build High-Scale Blocking Index
	blockingIdx := NewBlockingIndex(dests)

	tasks := make(chan matchTask, totalSources)
	resultsChan := make(chan []MatchResultItem, totalSources)

	var wg sync.WaitGroup
	var processedCount int64
	var autoMatchedCount int64
	var reviewNeededCount int64
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

					for _, cand := range candidates {
						scoreRes := CalculateCompositeScore(
							task.source.NormalizedName,
							cand.NormalizedName,
							task.source.TransactionDate,
							cand.TransactionDate,
							e.Config.Weights,
							e.Config.Algorithms,
							e.Config.DateToleranceDays,
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

						// Check against thresholds
						if scoreRes.TotalScore >= e.Config.ReviewThreshold {
							status := "REVIEW_NEEDED"
							if scoreRes.TotalScore >= e.Config.AutoMatchThreshold {
								status = "AUTO_MATCHED"
								atomic.AddInt64(&autoMatchedCount, 1)
							} else {
								atomic.AddInt64(&reviewNeededCount, 1)
							}
							atomic.AddInt64(&totalMatchesCount, 1)

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
								NameScore:       scoreRes.NameScore,
								DateScore:       scoreRes.DateScore,
								JWScore:         scoreRes.JWScore,
								LevScore:        scoreRes.LevScore,
								TokenScore:      scoreRes.TokenScore,
								TrigramScore:    scoreRes.TrigramScore,
								MatchStatus:     status,
								MatchReasons:    scoreRes.MatchReasons,
								CreatedAt:       time.Now(),
							})
						}
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

	completedAt := time.Now()
	progress.Status = "COMPLETED"
	progress.ProcessedSources = atomic.LoadInt64(&processedCount)
	progress.TotalMatches = atomic.LoadInt64(&totalMatchesCount)
	progress.AutoMatched = atomic.LoadInt64(&autoMatchedCount)
	progress.ReviewNeeded = atomic.LoadInt64(&reviewNeededCount)
	progress.CompletedAt = completedAt
	progress.ElapsedMs = completedAt.Sub(startedAt).Milliseconds()

	if onProgress != nil {
		onProgress(progress)
	}

	return allResults, progress
}
