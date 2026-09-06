package matcher

import (
	"math"
)

// CorpusStats holds inverse document frequency statistics for tokens.
// It is immutable after construction and safe for concurrent reads.
type CorpusStats struct {
	df map[string]int // token -> document frequency count
	n  int            // total document count
}

// BuildCorpusStats computes token document frequencies over sources and destinations.
// Document frequency counts each token ONCE per record (repeated tokens in a name don't inflate df).
// Returns nil if the corpus is empty.
func BuildCorpusStats(sources []SourceRecord, dests []DestinationRecord) *CorpusStats {
	// Combine all records
	totalDocs := len(sources) + len(dests)
	if totalDocs == 0 {
		return nil
	}

	// Count unique tokens per document (document frequency)
	df := make(map[string]int)

	// Process source records
	for _, src := range sources {
		seen := make(map[string]bool)
		for _, token := range src.NormalizedName.Tokens {
			if seen[token] {
				continue
			}
			seen[token] = true
			df[token]++
		}
	}

	// Process destination records
	for _, dest := range dests {
		seen := make(map[string]bool)
		for _, token := range dest.NormalizedName.Tokens {
			if seen[token] {
				continue
			}
			seen[token] = true
			df[token]++
		}
	}

	return &CorpusStats{
		df: df,
		n:  totalDocs,
	}
}

// Weight returns the normalized IDF weight of a token in [0,1].
// Unseen tokens return 1.0 (maximally distinctive).
// Uses formula: normalized_idf = log(1 + N/(1+df)) / log(1+N)
func (c *CorpusStats) Weight(token string) float64 {
	// Handle nil stats (fallback to genericWords behavior)
	if c == nil {
		return 1.0
	}

	if c.n == 0 {
		return 1.0
	}

	df := c.df[token]
	if df == 0 {
		// Unseen token: return maximum possible weight (1.0)
		return 1.0
	}

	// Smoothed and normalized IDF: log(1 + N/(1+df)) / log(1+N)
	idf := math.Log(1.0 + float64(c.n)/float64(1+df))
	logN := math.Log(1.0 + float64(c.n))
	normalizedIDF := idf / logN

	// Clamp to [0, 1]
	if normalizedIDF > 1.0 {
		return 1.0
	}
	if normalizedIDF < 0.0 {
		return 0.0
	}

	return normalizedIDF
}

// GetDocumentFrequency returns the document frequency for a token (0 if not found).
func (c *CorpusStats) GetDocumentFrequency(token string) int {
	if c == nil {
		return 0
	}
	return c.df[token]
}

// GetDocumentCount returns the total number of documents in the corpus.
func (c *CorpusStats) GetDocumentCount() int {
	if c == nil {
		return 0
	}
	return c.n
}
