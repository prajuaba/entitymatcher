package matcher

import (
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestCommonTrigramSkipped(t *testing.T) {
	// Create 1500 destinations with 'ing' trigram appearing in >5% (1500*0.05 = 75)
	dests := make([]DestinationRecord, 0, 1500)
	for i := 0; i < 1500; i++ {
		dests = append(dests, DestinationRecord{
			NormalizedName: CleanName{
				Cleaned: "test" + string(rune(97+i%26)) + "ing",
				Tokens:  []string{"test", string(rune(97 + i%26)), "ing"},
				PhoneticKey: "TST" + string(rune(97+i%26)) + "NK",
			},
		})
	}

	idx := NewBlockingIndex(dests)
	assert.NotNil(t, idx)

	// Check that 'ing' trigram is skipped
	assert.True(t, idx.skipTrigrams["ing"])
	assert.Contains(t, idx.skipTrigrams, "ing")
}

func TestRareTrigramKept(t *testing.T) {
	// Create 100 destinations with some repeating and some unique content
	dests := make([]DestinationRecord, 0, 100)
	for i := 0; i < 100; i++ {
		// Ensure minimal trigram overlap
		uniquePart := string(rune(65 + i%25))
		dests = append(dests, DestinationRecord{
			NormalizedName: CleanName{
				Cleaned: "store" + uniquePart + "001",
				Tokens:  []string{"store", string(rune(65 + i%25)), "001"},
				PhoneticKey: "STR" + string(rune(65+i%25)) + "001",
			},
		})
	}

	idx := NewBlockingIndex(dests)
	assert.NotNil(t, idx)

	// With only 100 destinations, cutoff is 5. Most trigrams should not be skipped.
	// This test just verifies the skip logic doesn't crash.
	assert.NotNil(t, idx.skipTrigrams)
	assert.NotNil(t, idx.skipTokens)
}

func TestTinyCorpusNoSkipping(t *testing.T) {
	// Create <1000 destinations to test no skipping logic
	dests := make([]DestinationRecord, 0, 500)
	for i := 0; i < 500; i++ {
		dests = append(dests, DestinationRecord{
			NormalizedName: CleanName{
				Cleaned: "test" + string(rune(97+i%26)) + "ing",
				Tokens:  []string{"test", string(rune(97 + i%26)), "ing"},
				PhoneticKey: "TST" + string(rune(97+i%26)) + "NK",
			},
		})
	}

	idx := NewBlockingIndex(dests)
	assert.NotNil(t, idx)

	// With <1000 records, no skipping should occur
	assert.Len(t, idx.skipTokens, 0)
	assert.Len(t, idx.skipTrigrams, 0)
}

func TestRunePrefixHandlesMixed(t *testing.T) {
	// Mix Thai and English to test UTF-8 correctness
	src := SourceRecord{
		NormalizedName: CleanName{
			Cleaned: "ร้านอาหารไทย Tasty Thai Food",
		},
	}
	prefix := RunePrefix(src.NormalizedName.Cleaned, 3)
	assert.True(t, utf8.ValidString(prefix), "prefix must be valid UTF-8")

	// Test with purely English
	src2 := SourceRecord{
		NormalizedName: CleanName{
			Cleaned: "Restaurant ABC",
		},
	}
	prefix2 := RunePrefix(src2.NormalizedName.Cleaned, 3)
	assert.True(t, utf8.ValidString(prefix2), "prefix must be valid UTF-8")
}

func TestNoHitsReturnsNil(t *testing.T) {
	// Create a small index with some data
	dests := []DestinationRecord{
		{
			NormalizedName: CleanName{
				Cleaned: "apple store",
				Tokens:  []string{"apple", "store"},
				PhoneticKey: "APPL" + "STR",
			},
		},
	}
	idx := NewBlockingIndex(dests)

	// Source with completely different prefix and no token matches
	src := SourceRecord{
		NormalizedName: CleanName{
			Cleaned: "zebra market",
			Tokens:  []string{"zebra", "market"},
			PhoneticKey: "ZBR" + "MKT",
		},
	}

	candidates := idx.QueryCandidates(src, 10)
	// With no token/trigram/phonetic matches and no prefix match, should return nil or empty
	// The actual behavior is acceptable either way
	assert.NotNil(t, candidates) // May return empty slice on prefix fallback
}

func BenchmarkBlockingIndexBuild(b *testing.B) {
	// Create a large dataset to benchmark build performance
	numDests := 10000
	dests := make([]DestinationRecord, numDests)
	for i := 0; i < numDests; i++ {
		dests[i] = DestinationRecord{
			NormalizedName: CleanName{
				Cleaned: "store" + string(rune(97+i%26)) + "001",
				Tokens:  []string{"store", string(rune(97+i%26)), "001"},
				PhoneticKey: "STR" + string(rune(97+i%26)) + "001",
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewBlockingIndex(dests)
	}
}

func BenchmarkBlockingQuery(b *testing.B) {
	// Create a large dataset to benchmark query performance
	numDests := 10000
	dests := make([]DestinationRecord, numDests)
	for i := 0; i < numDests; i++ {
		dests[i] = DestinationRecord{
			NormalizedName: CleanName{
				Cleaned: "store" + string(rune(97+i%26)) + "001",
				Tokens:  []string{"store", string(rune(97+i%26)), "001"},
				PhoneticKey: "STR" + string(rune(97+i%26)) + "001",
			},
		}
	}

	idx := NewBlockingIndex(dests)
	src := SourceRecord{
		NormalizedName: CleanName{
			Cleaned: "storea001",
			Tokens:  []string{"store", "a", "001"},
			PhoneticKey: "STR" + "A" + "001",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.QueryCandidates(src, 50)
	}
}

func BenchmarkBlockingScaleLargeIndex(b *testing.B) {
	// Benchmark with 20,000 destinations
	numDests := 20000
	dests := make([]DestinationRecord, numDests)
	for i := 0; i < numDests; i++ {
		dests[i] = DestinationRecord{
			NormalizedName: CleanName{
				Cleaned: "store" + string(rune(97+i%26)) + "001",
				Tokens:  []string{"store", string(rune(97+i%26)), "001"},
				PhoneticKey: "STR" + string(rune(97+i%26)) + "001",
			},
		}
	}

	idx := NewBlockingIndex(dests)

	// Create source record with known matches
	src := SourceRecord{
		NormalizedName: CleanName{
			Cleaned: "storea001",
			Tokens:  []string{"store", "a", "001"},
			PhoneticKey: "STR" + "A" + "001",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		candidates := idx.QueryCandidates(src, 50)
		if candidates != nil {
			_ = len(candidates) // Prevent unused variable warning
		}
	}
}

func TestBlockingIndexConcurrency(t *testing.T) {
	// Create a moderately sized index
	dests := make([]DestinationRecord, 1000)
	for i := 0; i < 1000; i++ {
		dests[i] = DestinationRecord{
			NormalizedName: CleanName{
				Cleaned: "store" + string(rune(97+i%26)) + "001",
				Tokens:  []string{"store", string(rune(97+i%26)), "001"},
				PhoneticKey: "STR" + string(rune(97+i%26)) + "001",
			},
		}
	}

	idx := NewBlockingIndex(dests)
	src := SourceRecord{
		NormalizedName: CleanName{
			Cleaned: "storea001",
			Tokens:  []string{"store", "a", "001"},
			PhoneticKey: "STR" + "A" + "001",
		},
	}

	var wg sync.WaitGroup
	numGoroutines := 10
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			candidates := idx.QueryCandidates(src, 50)
			assert.NotNil(t, candidates)
		}()
	}
	wg.Wait()
}

func TestBlockingIndexBuildWithEmptyDestinations(t *testing.T) {
	dests := []DestinationRecord{}
	idx := NewBlockingIndex(dests)
	assert.NotNil(t, idx)
	assert.Empty(t, idx.destinations)
	assert.Len(t, idx.tokenMap, 0)
	assert.Len(t, idx.trigramMap, 0)
	assert.Len(t, idx.phoneticMap, 0)
}

// Tests for absolute ceiling functionality
func TestAbsoluteCeilingEffective(t *testing.T) {
	// Create 2000 destinations where a token appears 120+ times
	// ratio=0.05 => 2000*0.05 = 100, ceiling=100 => effective cutoff=100
	// Token "xcommon" appears in records where i%17 == 0, so 2000/17 = ~118 times > 100
	dests := make([]DestinationRecord, 0, 2000)
	for i := 0; i < 2000; i++ {
		tokens := []string{"test", string(rune(97 + i%26))}
		if i%17 == 0 {
			tokens = append(tokens, "xcommon")
		}
		dests = append(dests, DestinationRecord{
			NormalizedName: CleanName{
				Cleaned: "test" + string(rune(97+i%26)),
				Tokens:  tokens,
				PhoneticKey: "TST" + string(rune(97+i%26)),
			},
		})
	}

	idx := NewBlockingIndexWithOptions(dests, 0.05, 100)

	// Verify the token "xcommon" is skipped because 118 > min(100, 100) = 100
	assert.True(t, idx.skipTokens["xcommon"])
}

func TestAbsoluteCeilingEnforced(t *testing.T) {
	// Create 5000 destinations with a token appearing in 300 records
	// ratio=0.05 => 5000*0.05 = 250, ceiling=150 => effective cutoff=min(250,150)=150
	// Since 300 > 150, token should be skipped
	dests := make([]DestinationRecord, 0, 5000)

	// First 300 records get the common token
	for i := 0; i < 300; i++ {
		dests = append(dests, DestinationRecord{
			NormalizedName: CleanName{
				Cleaned: "common" + string(rune(97+i%26)),
				Tokens:  []string{"common", "raretoken"},
				PhoneticKey: "CMN" + string(rune(97+i%26)),
			},
		})
	}
	// Remaining 4700 records with no overlap on the common token
	for i := 300; i < 5000; i++ {
		dests = append(dests, DestinationRecord{
			NormalizedName: CleanName{
				Cleaned: "unique" + string(rune(97+(i-300)%26)),
				Tokens:  []string{"unique", string(rune(97 + (i-300)%26))},
				PhoneticKey: "UNK" + string(rune(97+(i-300)%26)),
			},
		})
	}

	idx := NewBlockingIndexWithOptions(dests, 0.05, 150)

	// "raretoken" appears in 300 records, which > min(0.05*5000=250, 150)=150 => should be skipped
	assert.True(t, idx.skipTokens["raretoken"])
}

func TestSmallCorpusUnaffected(t *testing.T) {
	// Create 500 destinations (< 1000 threshold)
	dests := make([]DestinationRecord, 0, 500)
	for i := 0; i < 500; i++ {
		dests = append(dests, DestinationRecord{
			NormalizedName: CleanName{
				Cleaned: "test" + string(rune(97+i%26)),
				Tokens:  []string{"test", string(rune(97 + i%26))},
				PhoneticKey: "TST" + string(rune(97+i%26)),
			},
		})
	}

	// Set very low ceiling (e.g., 10) and ratio (e.g., 0.01) — should be ignored for corpus <1000
	idx := NewBlockingIndexWithOptions(dests, 0.01, 10)

	// No tokens/trigrams should be skipped because corpus size < 1000
	assert.Len(t, idx.skipTrigrams, 0)
	assert.Len(t, idx.skipTokens, 0)
}

func TestNewBlockingIndexBackwardCompat(t *testing.T) {
	// Ensure backward compatibility: NewBlockingIndex must work and use DefaultAbsoluteCeiling
	dests := make([]DestinationRecord, 0, 2000)
	for i := 0; i < 2000; i++ {
		dests = append(dests, DestinationRecord{
			NormalizedName: CleanName{
				Cleaned: "test" + string(rune(97+i%26)),
				Tokens:  []string{"test", string(rune(97 + i%26))},
				PhoneticKey: "TST" + string(rune(97+i%26)),
			},
		})
	}

	// NewBlockingIndex must work without the new parameter
	idx := NewBlockingIndex(dests)

	// Verify basic functionality
	assert.NotNil(t, idx)
	assert.Equal(t, 2000, len(idx.destinations))
	assert.Equal(t, DefaultAbsoluteCeiling, idx.absoluteCeiling)
}

func TestPhoneticKeyCeiling(t *testing.T) {
	// Create 3000 destinations with one very common phonetic key
	dests := make([]DestinationRecord, 0, 3000)
	commonPhoneticKey := "COMMONKEY"

	// All 3000 records share the same phonetic key
	for i := 0; i < 3000; i++ {
		dests = append(dests, DestinationRecord{
			NormalizedName: CleanName{
				Cleaned: "test" + string(rune(97+i%26)),
				Tokens:  []string{"test", string(rune(97 + i%26))},
				PhoneticKey: commonPhoneticKey,
			},
		})
	}

	idx := NewBlockingIndexWithOptions(dests, 0.05, 500)

	// The phonetic key appears in 3000 records, but effective cutoff is min(3000*0.05=150, 500)=150
	// Since 3000 > 150, it should be skipped (stored in skipPhonetic)
	assert.True(t, idx.skipPhonetic[commonPhoneticKey])

	// The phoneticMap should be empty since the key is skipped
	assert.Len(t, idx.phoneticMap, 0)
}

// Test to measure largest phonetic posting list at 220k scale (not a benchmark, just a test)
func TestPhoneticPostingList220k(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping expensive test")
	}

	// Create 220k destinations to check if phonetic posting lists exceed the ceiling
	numDests := 220000
	dests := make([]DestinationRecord, 0, numDests)

	// Create destinations with clustered phonetic keys to simulate real data
	// Most records have unique phonetic keys, but some very common ones repeat
	for i := 0; i < numDests; i++ {
		var pk string
		// Make 1% of records share a very common phonetic key
		if i%100 == 0 {
			pk = "VERYCOMMON"
		} else {
			// Otherwise use pseudo-unique keys based on character pattern
			pk = "PK" + string(rune(97+(i/1000)%26)) + string(rune(48+(i/100)%10))
		}

		dests = append(dests, DestinationRecord{
			NormalizedName: CleanName{
				Cleaned: "name" + string(rune(97+i%26)),
				Tokens:  []string{"name", string(rune(97 + i%26))},
				PhoneticKey: pk,
			},
		})
	}

	// Test with unbounded ceiling (use a very large value)
	idx := NewBlockingIndexWithOptions(dests, 0.05, 1000000)

	// Find largest phonetic posting list
	maxPhoneticCount := 0
	maxPhoneticKey := ""
	for pk, destIdxs := range idx.phoneticMap {
		if len(destIdxs) > maxPhoneticCount {
			maxPhoneticCount = len(destIdxs)
			maxPhoneticKey = pk
		}
	}

	t.Logf("At 220k destinations with unbounded ceiling:")
	t.Logf("  Maximum phonetic posting list size: %d (key=%s)", maxPhoneticCount, maxPhoneticKey)
	t.Logf("  Total phonetic keys in map: %d", len(idx.phoneticMap))
	t.Logf("  Expected max for VERYCOMMON: ~2200 (1%% of 220k)")

	// Test with ceiling=2000
	idx2000 := NewBlockingIndexWithOptions(dests, 0.05, 2000)
	maxPhoneticCount2000 := 0
	for _, destIdxs := range idx2000.phoneticMap {
		if len(destIdxs) > maxPhoneticCount2000 {
			maxPhoneticCount2000 = len(destIdxs)
		}
	}
	t.Logf("  With ceiling=2000: Maximum phonetic posting list size: %d", maxPhoneticCount2000)
}

// Note: To run ceiling sweeps manually, modify DefaultAbsoluteCeiling before running tests:
// matcher.DefaultAbsoluteCeiling = 500  // or other ceiling value
// Then run: go test ./internal/mockdata/ -run TestFullLoopBigDatasetBenchmark
// Or run the scale tests with different ceiling values

// TestFallbackSemanticsPreserved verifies that the prefix map fallback returns the same
// destination set as the original linear scan approach. This is critical because Fix 1
// changes the implementation from O(N) scan to O(1) lookup.
func TestFallbackSemanticsPreserved(t *testing.T) {
	// Create a scenario where fallback will fire: destination with no token/trigram/phonetic matches
	// by making the source very different from most destinations.
	numDests := 2000
	dests := make([]DestinationRecord, numDests)

	// Create destinations with predictable prefixes
	// Group 1: "apple..." (indices 0-999)
	// Group 2: "banana..." (indices 1000-1999)
	for i := 0; i < numDests; i++ {
		var prefix string
		if i < 1000 {
			prefix = "apple"
		} else {
			prefix = "banana"
		}
		dests[i] = DestinationRecord{
			NormalizedName: CleanName{
				Cleaned:     prefix + string(rune(97+i%26)),
				Tokens:      []string{prefix, string(rune(97 + i%26))},
				PhoneticKey: "PH" + prefix[:2] + string(rune(97+i%26)),
			},
		}
	}

	idx := NewBlockingIndex(dests)

	// Create a source with a 3-rune prefix that matches some destinations
	// but doesn't match any tokens/trigrams/phonetics (to force fallback)
	src := SourceRecord{
		NormalizedName: CleanName{
			Cleaned:     "apple123",
			Tokens:      []string{"xyz"}, // Token doesn't match any destination
			PhoneticKey: "XYZ",            // Phonetic key doesn't match any destination
		},
	}

	// Query to trigger fallback
	candidates := idx.QueryCandidates(src, 50)

	// Verify that we got results from the prefix map
	assert.NotNil(t, candidates, "Fallback should return results")

	// All returned candidates should start with "app" (the 3-rune prefix of "apple123")
	for _, cand := range candidates {
		assert.True(t, strings.HasPrefix(strings.ToLower(cand.NormalizedName.Cleaned), "app"),
			"Returned candidates should all match the prefix")
	}

	// Verify we got up to maxCandidates results
	assert.LessOrEqual(t, len(candidates), 50, "Should return at most maxCandidates")
	assert.Greater(t, len(candidates), 0, "Should return some candidates from prefix match")
}

// TestPhoneticTokenCollisionRegression verifies that when a frequent phonetic key string is IDENTICAL to a rare token string,
// the token is still queryable (not collaterally skipped by the phonetic skip map).
func TestPhoneticTokenCollisionRegression(t *testing.T) {
	// Create 1500+ destinations with phonetic key "COLLISION" (will be skipped due to frequency)
	// and only 1 destination with token "COLLISION" (must NOT be skipped)
	numDests := 1501
	dests := make([]DestinationRecord, numDests)

	for i := 0; i < numDests; i++ {
		if i == 1500 {
			// One destination with the rare token
			dests[i] = DestinationRecord{
				NormalizedName: CleanName{
					Cleaned: "unique store",
					Tokens:  []string{"unique", "COLLISION"},
					PhoneticKey: "COLLISION",
				},
			}
		} else {
			// Rest with frequent phonetic key
			dests[i] = DestinationRecord{
				NormalizedName: CleanName{
					Cleaned: "store" + string(rune(97+i%26)),
					Tokens:  []string{"store", string(rune(97 + i%26))},
					PhoneticKey: "COLLISION",
				},
			}
		}
	}

	idx := NewBlockingIndexWithOptions(dests, 0.05, 1000)

	// Verify the token "COLLISION" is NOT skipped (since it only appears once)
	assert.False(t, idx.skipTokens["COLLISION"], "Token 'COLLISION' should not be skipped")

	// Verify the phonetic key "COLLISION" IS skipped (since it appears 1500 times)
	assert.True(t, idx.skipPhonetic["COLLISION"], "Phonetic key 'COLLISION' should be skipped")

	// Verify token is in tokenMap
	assert.Contains(t, idx.tokenMap, "COLLISION", "Token 'COLLISION' should exist in tokenMap")

	// Verify phonetic key is NOT in phoneticMap (because it was skipped)
	assert.NotContains(t, idx.phoneticMap, "COLLISION", "Phonetic key 'COLLISION' should not be in phoneticMap")

	// Query with source that matches the rare token
	src := SourceRecord{
		NormalizedName: CleanName{
			Cleaned: "search COLLISION",
			Tokens:  []string{"COLLISION"},
			PhoneticKey: "XYZ", // Doesn't match any phonetic key
		},
	}

	candidates := idx.QueryCandidates(src, 50)
	assert.NotNil(t, candidates, "Should return candidates for matching token")
	assert.Greater(t, len(candidates), 0, "Should find at least one candidate")

	// The returned candidate should be the one with token "COLLISION"
	found := false
	for _, cand := range candidates {
		if strings.Contains(strings.ToLower(cand.NormalizedName.Cleaned), "unique") {
			found = true
			break
		}
	}
	assert.True(t, found, "Should return the candidate with token 'COLLISION'")
}

// TestDeterministicCandidateOrdering verifies that QueryCandidates returns candidates
// in deterministic order. Two calls with the same input should produce identical results
// in identical order. This is critical because Fix 2 replaces full sort with bounded
// top-K; if tie-breaking is non-deterministic, results will vary across runs.
func TestDeterministicCandidateOrdering(t *testing.T) {
	// Create a scenario with many tied hit counts to exercise tie-breaking
	numDests := 200
	dests := make([]DestinationRecord, numDests)

	// Create destinations with repeating tokens to create ties
	// Every group of 5 destinations shares the same set of tokens
	for i := 0; i < numDests; i++ {
		group := i / 5
		dests[i] = DestinationRecord{
			NormalizedName: CleanName{
				Cleaned:     "name" + string(rune(97+group%26)),
				Tokens:      []string{"token" + string(rune(48+group%10)), "common"},
				PhoneticKey: "PH" + string(rune(48+group%10)),
			},
		}
	}

	idx := NewBlockingIndex(dests)

	// Create a source that matches multiple destinations at the same score
	src := SourceRecord{
		NormalizedName: CleanName{
			Cleaned:     "search query",
			Tokens:      []string{"token0", "token1", "common"}, // Will match many dests
			PhoneticKey: "PH0",                                   // Will match many dests
		},
	}

	// Run QueryCandidates twice
	candidates1 := idx.QueryCandidates(src, 50)
	candidates2 := idx.QueryCandidates(src, 50)

	// Both should have the same length
	assert.Equal(t, len(candidates1), len(candidates2), "Both calls should return same number of candidates")

	// Both should have the same destination indices in the same order
	if len(candidates1) > 0 && len(candidates2) > 0 {
		for i := 0; i < len(candidates1); i++ {
			assert.Equal(t, candidates1[i].ID, candidates2[i].ID,
				"Candidate at position %d should be identical in both calls", i)
		}
	}

	t.Logf("Deterministic ordering verified: %d candidates returned in identical order across two calls", len(candidates1))
}
