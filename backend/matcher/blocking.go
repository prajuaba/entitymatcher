package matcher

import (
	"sort"
	"strings"
	"sync"
)

type BlockingIndex struct {
	mu              sync.RWMutex
	tokenMap        map[string][]int // Token -> array of destination indices
	trigramMap      map[string][]int // Trigram -> array of destination indices
	phoneticMap     map[string][]int // PhoneticKey -> array of destination indices
	destinations    []DestinationRecord
	maxPostingRatio float64
	skipTrigrams    map[string]bool
	skipTokens      map[string]bool
}

func NewBlockingIndex(dests []DestinationRecord) *BlockingIndex {
	return NewBlockingIndexWithOptions(dests, 0.05)
}

func NewBlockingIndexWithOptions(dests []DestinationRecord, maxPostingRatio float64) *BlockingIndex {
	idx := &BlockingIndex{
		tokenMap:        make(map[string][]int),
		trigramMap:      make(map[string][]int),
		phoneticMap:     make(map[string][]int),
		destinations:    dests,
		maxPostingRatio: maxPostingRatio,
		skipTrigrams:    make(map[string]bool),
		skipTokens:      make(map[string]bool),
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
		cutoff = int(float64(len(idx.destinations)) * idx.maxPostingRatio)
	}

	// Precompute token and trigram frequencies
	tokenFreq := make(map[string]int)
	trigramFreq := make(map[string]int)

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

	for i, dest := range idx.destinations {
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
		if dest.NormalizedName.PhoneticKey != "" {
			pk := dest.NormalizedName.PhoneticKey
			idx.phoneticMap[pk] = append(idx.phoneticMap[pk], i)
		}
	}
}

type candidateScore struct {
	index int
	hits  int
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

	// Token match hits (weighted higher)
	for _, tok := range src.NormalizedName.Tokens {
		if len(tok) >= 2 && !idx.skipTokens[tok] {
			if destIdxs, exists := idx.tokenMap[tok]; exists {
				for _, destIdx := range destIdxs {
					hitCounts[destIdx] += 3
				}
			}
		}
	}

	// Phonetic key match hits
	if src.NormalizedName.PhoneticKey != "" {
		if destIdxs, exists := idx.phoneticMap[src.NormalizedName.PhoneticKey]; exists {
			for _, destIdx := range destIdxs {
				hitCounts[destIdx] += 4
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
				}
			}
		}
	}

	// If no hits found via blocking, pick top candidates by date proximity or prefix match
	if len(hitCounts) == 0 {
		srcPrefix := ""
		if len(src.NormalizedName.Cleaned) >= 3 {
			srcPrefix = RunePrefix(src.NormalizedName.Cleaned, 3)
		}
		var fallback []DestinationRecord
		for _, dest := range idx.destinations {
			if srcPrefix != "" && strings.HasPrefix(strings.ToLower(dest.NormalizedName.Cleaned), srcPrefix) {
				fallback = append(fallback, dest)
				if len(fallback) >= maxCandidates {
					return fallback
				}
			}
		}
		if len(fallback) > 0 {
			return fallback
		}
		// Final fallback: return nil when no hits
		return nil
	}

	// Sort candidates by hit counts descending
	candidates := make([]candidateScore, 0, len(hitCounts))
	for destIdx, hits := range hitCounts {
		candidates = append(candidates, candidateScore{index: destIdx, hits: hits})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].hits > candidates[j].hits
	})

	limit := maxCandidates
	if limit > len(candidates) {
		limit = len(candidates)
	}

	result := make([]DestinationRecord, limit)
	for i := 0; i < limit; i++ {
		result[i] = idx.destinations[candidates[i].index]
	}

	return result
}
