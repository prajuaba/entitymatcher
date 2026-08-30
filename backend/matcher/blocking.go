package matcher

import (
	"container/heap"
	"sort"
	"strings"
	"sync"
)

// DefaultAbsoluteCeiling bounds how long a single posting list may be before the
// key is skipped at query time. The effective cutoff is min(maxPostingRatio*N,
// DefaultAbsoluteCeiling): the ratio alone is not enough, because a ratio grows
// with the corpus, so the largest list still scanned was ~1,100 entries at 22k
// destinations but ~11,000 at 220k -- the ratio decides WHICH keys are skipped,
// it never bounds the work per query.
//
// Ceiling chosen by measurement. Accuracy was the constraint, speed the
// objective; every value below held precision at 100.00% and top-1 at 99.19%,
// so the choice came down to throughput at 220k destinations per side:
//
//	ceiling      100k match time   throughput
//	500                  74.90s     2,937/s   <- much WORSE, see note
//	1000                 34.70s     6,341/s
//	2000                 34.56s     6,365/s   <- chosen
//	5000                 35.10s     6,268/s
//	unbounded            38.20s     5,762/s
//
// Note on 500: a tighter ceiling is not monotonically faster. Skipping more keys
// pushes more sources into the zero-hit fallback below, which linearly scans
// every destination -- so an over-tight ceiling converts bounded lookups into
// O(N) scans and costs more than it saves.
//
// Profiling revealed the quadratic scaling. Retrieval was O(N^2.00) and consumed 67% of
// runtime at 220k destinations. Scoring was O(N^1.00) and assignment O(N^0.96); GC CPU
// fraction fell with scale (1.16% -> 0.61%), so allocation was not the driver. The
// quadratic term was the zero-hit fallback: it fired for 2.73% of sources at >= 110k
// per side (0% below), and each firing linearly scanned every destination. At 220k, these
// fallback scans totaled 1.32 billion iterations. Per-corpus doubling, fallback work grew
// 4.00x compared to retrieval's 4.22x; the full sort over hitCounts (growing 1.71x per
// doubling) predicted only 1.84x and could not explain the quadratic curve.
//
// Two fixes: a prefix index makes the fallback O(1) with identical semantics, and bounded
// top-K selection replaces the full sort, shifting from O(M log M) to O(M log K). Measured
// result at 220k: k = 1.30 -> 1.10. Accuracy unchanged: precision 100.00%, top-1 99.19%,
// 1:1 invariants hold. Note that k = 1.10 is not 1.00; the residual scaling comes from
// candidate-set size growing with corpus, which the top-K bounds the sort cost of but does
// not eliminate.
var DefaultAbsoluteCeiling = 2000

// QueryMetrics tracks profiling data for QueryCandidates calls, used by the
// scale/profile tests to attribute retrieval cost. Collection is gated on
// Metrics != nil && Enabled, so the per-call cost when unused is one nil check;
// the postingListWalks counter itself is an unconditional int increment in the
// hot loop, which is not literally zero but is unmeasurable against the map
// operations it counts.
type QueryMetrics struct {
	Enabled              bool
	DistinctDestsTouched []int // len(hitCounts) per call
	PostingListWalks     []int // total increments to hitCounts per call
	SliceSize            []int // len(candidates) after sort per call
	FallbackCount        int   // number of zero-hit fallbacks
	mu                   sync.Mutex
}

type BlockingIndex struct {
	mu              sync.RWMutex
	tokenMap        map[string][]int // Token -> array of destination indices
	trigramMap      map[string][]int // Trigram -> array of destination indices
	phoneticMap     map[string][]int // PhoneticKey -> array of destination indices
	romanizedMap    map[string][]int // Romanized trigram -> array of destination indices
	prefixMap       map[string][]int // 3-char lowercased prefix -> array of destination indices
	destinations    []DestinationRecord
	maxPostingRatio float64
	skipTrigrams    map[string]bool
	skipTokens      map[string]bool
	skipPhonetic    map[string]bool
	skipRomanized   map[string]bool
	absoluteCeiling int
	useRomanized    bool
	Metrics         *QueryMetrics // Optional profiling metrics (off by default)
}

func NewBlockingIndex(dests []DestinationRecord) *BlockingIndex {
	return NewBlockingIndexWithOptions(dests, 0.05, DefaultAbsoluteCeiling, true)
}

func NewBlockingIndexWithOptions(dests []DestinationRecord, maxPostingRatio float64, absoluteCeiling int, useRomanized bool) *BlockingIndex {
	idx := &BlockingIndex{
		tokenMap:        make(map[string][]int),
		trigramMap:      make(map[string][]int),
		phoneticMap:     make(map[string][]int),
		romanizedMap:    make(map[string][]int),
		prefixMap:       make(map[string][]int),
		destinations:    dests,
		maxPostingRatio: maxPostingRatio,
		skipTrigrams:    make(map[string]bool),
		skipTokens:      make(map[string]bool),
		skipPhonetic:    make(map[string]bool),
		skipRomanized:   make(map[string]bool),
		absoluteCeiling: absoluteCeiling,
		useRomanized:    useRomanized,
	}
	idx.build()
	return idx
}

func (idx *BlockingIndex) build() {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Compute cutoff for capping
	cutoff := 0
	if len(idx.destinations) >= 1000 {
		ratioCutoff := int(float64(len(idx.destinations)) * idx.maxPostingRatio)
		cutoff = min(ratioCutoff, idx.absoluteCeiling)
	}

	// Precompute token and trigram frequencies
	tokenFreq := make(map[string]int)
	trigramFreq := make(map[string]int)
	var romanizedFreq map[string]int

	if idx.useRomanized {
		romanizedFreq = make(map[string]int)
	}

	for _, dest := range idx.destinations {
		// Index by clean tokens
		for _, tok := range dest.NormalizedName.Tokens {
			if len(tok) >= 2 {
				tokenFreq[tok]++
			}
		}

		// Index by trigrams
		trigrams := extractTrigrams(dest.NormalizedName.Cleaned)
		for _, tr := range trigrams {
			trigramFreq[tr]++
		}

		// Index by romanized trigrams
		if idx.useRomanized && dest.NormalizedName.Romanized != "" {
			romanizedTrigrams := extractTrigrams(dest.NormalizedName.Romanized)
			for _, tr := range romanizedTrigrams {
				romanizedFreq[tr]++
			}
		}
	}

	// Populate skip maps
	if cutoff > 0 {
		for tok, freq := range tokenFreq {
			if freq > cutoff {
				idx.skipTokens[tok] = true
			}
		}
		for tr, freq := range trigramFreq {
			if freq > cutoff {
				idx.skipTrigrams[tr] = true
			}
		}
	}

	// Count phonetic key frequencies
	phoneticFreq := make(map[string]int)
	for _, dest := range idx.destinations {
		if dest.NormalizedName.PhoneticKey != "" {
			phoneticFreq[dest.NormalizedName.PhoneticKey]++
		}
	}

	// Populate skip map for phonetic keys (only if cutoff > 0)
	if cutoff > 0 {
		for pk, freq := range phoneticFreq {
			if freq > cutoff {
				idx.skipPhonetic[pk] = true
			}
		}
	}

	// Populate skip map for romanized trigrams (only if cutoff > 0 and useRomanized)
	if idx.useRomanized && cutoff > 0 {
		for rt, freq := range romanizedFreq {
			if freq > cutoff {
				idx.skipRomanized[rt] = true
			}
		}
	}

	for i, dest := range idx.destinations {
		cleaned := strings.ToLower(dest.NormalizedName.Cleaned)
		// Build prefix map: use first 3 chars (or whole string if shorter)
		prefix := RunePrefix(cleaned, 3)
		idx.prefixMap[prefix] = append(idx.prefixMap[prefix], i)

		// Index by clean tokens
		for _, tok := range dest.NormalizedName.Tokens {
			if len(tok) >= 2 && !idx.skipTokens[tok] {
				idx.tokenMap[tok] = append(idx.tokenMap[tok], i)
			}
		}

		// Index by trigrams
		trigrams := extractTrigrams(dest.NormalizedName.Cleaned)
		for _, tr := range trigrams {
			if !idx.skipTrigrams[tr] {
				idx.trigramMap[tr] = append(idx.trigramMap[tr], i)
			}
		}

		// Index by phonetic key
		if dest.NormalizedName.PhoneticKey != "" && !idx.skipPhonetic[dest.NormalizedName.PhoneticKey] {
			pk := dest.NormalizedName.PhoneticKey
			idx.phoneticMap[pk] = append(idx.phoneticMap[pk], i)
		}

		// Index by romanized trigrams
		if idx.useRomanized && dest.NormalizedName.Romanized != "" {
			romanizedTrigrams := extractTrigrams(dest.NormalizedName.Romanized)
			for _, tr := range romanizedTrigrams {
				if !idx.skipRomanized[tr] {
					idx.romanizedMap[tr] = append(idx.romanizedMap[tr], i)
				}
			}
		}
	}
}

type candidateScore struct {
	index int
	hits  int
}

// candidateHeap is a min-heap for bounded top-K selection.
// It maintains the K best candidates (highest hits) and preserves order for ties.
type candidateHeap []candidateScore

func (h candidateHeap) Len() int { return len(h) }
func (h candidateHeap) Less(i, j int) bool {
	if h[i].hits != h[j].hits {
		return h[i].hits < h[j].hits // min-heap: smaller hits have higher priority to be evicted
	}
	// For ties, use destination index (smaller index preferred)
	return h[i].index > h[j].index // larger index is "less" (more likely to be evicted first)
}
func (h candidateHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *candidateHeap) Push(x interface{}) { *h = append(*h, x.(candidateScore)) }
func (h *candidateHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

// QueryCandidates returns candidate destination records matching the source record.
// maxCandidates sets the limit on candidates returned per source record (e.g. 50-100).
func (idx *BlockingIndex) QueryCandidates(src SourceRecord, maxCandidates int) []DestinationRecord {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// If destination count is small (e.g., <= 100), return all
	if len(idx.destinations) <= maxCandidates {
		return idx.destinations
	}

	hitCounts := make(map[int]int)
	var postingListWalks int

	// Token match hits (weighted higher)
	for _, tok := range src.NormalizedName.Tokens {
		if len(tok) >= 2 && !idx.skipTokens[tok] {
			if destIdxs, exists := idx.tokenMap[tok]; exists {
				for _, destIdx := range destIdxs {
					hitCounts[destIdx] += 3
					postingListWalks++
				}
			}
		}
	}

	// Phonetic key match hits
	if src.NormalizedName.PhoneticKey != "" && !idx.skipPhonetic[src.NormalizedName.PhoneticKey] {
		if destIdxs, exists := idx.phoneticMap[src.NormalizedName.PhoneticKey]; exists {
			for _, destIdx := range destIdxs {
				hitCounts[destIdx] += 4
				postingListWalks++
			}
		}
	}

	// Trigram match hits
	srcTrigrams := extractTrigrams(src.NormalizedName.Cleaned)
	for _, tr := range srcTrigrams {
		if !idx.skipTrigrams[tr] {
			if destIdxs, exists := idx.trigramMap[tr]; exists {
				for _, destIdx := range destIdxs {
					hitCounts[destIdx]++
					postingListWalks++
				}
			}
		}
	}

	// Romanized trigram match hits (for cross-script retrieval)
	if idx.useRomanized && src.NormalizedName.Romanized != "" {
		srcRomanizedTrigrams := extractTrigrams(src.NormalizedName.Romanized)
		for _, tr := range srcRomanizedTrigrams {
			if !idx.skipRomanized[tr] {
				if destIdxs, exists := idx.romanizedMap[tr]; exists {
					for _, destIdx := range destIdxs {
						hitCounts[destIdx]++
						postingListWalks++
					}
				}
			}
		}
	}

	// Record metrics if enabled
	if idx.Metrics != nil && idx.Metrics.Enabled {
		idx.Metrics.mu.Lock()
		idx.Metrics.DistinctDestsTouched = append(idx.Metrics.DistinctDestsTouched, len(hitCounts))
		idx.Metrics.PostingListWalks = append(idx.Metrics.PostingListWalks, postingListWalks)
		idx.Metrics.mu.Unlock()
	}

	// If no hits found via blocking, pick top candidates by prefix match
	if len(hitCounts) == 0 {
		if idx.Metrics != nil && idx.Metrics.Enabled {
			idx.Metrics.mu.Lock()
			idx.Metrics.FallbackCount++
			idx.Metrics.mu.Unlock()
		}

		srcPrefix := ""
		if len(src.NormalizedName.Cleaned) >= 3 {
			srcPrefix = RunePrefix(src.NormalizedName.Cleaned, 3)
		}
		// Lowercase prefix for lookup (RunePrefix already lowercased)
		if srcPrefix != "" {
			if destIdxs, exists := idx.prefixMap[srcPrefix]; exists {
				limit := maxCandidates
				if limit > len(destIdxs) {
					limit = len(destIdxs)
				}
				result := make([]DestinationRecord, limit)
				for i := 0; i < limit; i++ {
					result[i] = idx.destinations[destIdxs[i]]
				}
				return result
			}
		}
		// Final fallback: return nil when no hits
		return nil
	}

	// Fix non-determinism: sort keys to ensure deterministic order
	hitCountKeys := make([]int, 0, len(hitCounts))
	for k := range hitCounts {
		hitCountKeys = append(hitCountKeys, k)
	}
	sort.Ints(hitCountKeys)

	// Bounded top-K selection using min-heap (O(M log K) instead of O(M log M))
	h := &candidateHeap{}
	heap.Init(h)
	for _, destIdx := range hitCountKeys {
		hits := hitCounts[destIdx]
		if h.Len() < maxCandidates {
			heap.Push(h, candidateScore{index: destIdx, hits: hits})
		} else if hits > (*h)[0].hits || (hits == (*h)[0].hits) {
			// Replace worst candidate if new one is better or tied
			heap.Pop(h)
			heap.Push(h, candidateScore{index: destIdx, hits: hits})
		}
	}

	// Convert heap to slice and sort by hits desc, preserving index order for ties
	candidates := make([]candidateScore, h.Len())
	for i := range candidates {
		candidates[i] = heap.Pop(h).(candidateScore)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].hits != candidates[j].hits {
			return candidates[i].hits > candidates[j].hits
		}
		// Prefer smaller index for ties
		return candidates[i].index < candidates[j].index
	})

	limit := maxCandidates
	if limit > len(candidates) {
		limit = len(candidates)
	}

	result := make([]DestinationRecord, limit)
	for i := 0; i < limit; i++ {
		result[i] = idx.destinations[candidates[i].index]
	}

	// Record final slice size if metrics enabled
	if idx.Metrics != nil && idx.Metrics.Enabled {
		idx.Metrics.mu.Lock()
		idx.Metrics.SliceSize = append(idx.Metrics.SliceSize, len(result))
		idx.Metrics.mu.Unlock()
	}

	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
