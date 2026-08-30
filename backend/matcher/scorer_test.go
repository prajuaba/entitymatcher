package matcher

import (
	"testing"
	"time"
)

// TestLatinPairsInvariant verifies that pure-Latin name pairs produce identical scores
// regardless of the UseThaiPhonetic flag setting. This regression test ensures that
// the Thai phonetic signal does not affect Latin-only pairs.
func TestLatinPairsInvariant(t *testing.T) {
	testCases := []struct {
		name  string
		name1 string
		name2 string
	}{
		{"Exact match", "John Smith", "John Smith"},
		{"Minor spelling variation", "John Smith", "Jon Smith"},
		{"Full name vs abbreviation", "Acme Corporation", "Acme Corp"},
		{"Middle initial omission", "Robert John Doe", "Robert Doe"},
		{"Extra spaces", "John  Smith", "John Smith"},
	}

	defaultWeights := DefaultWeights
	defaultDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Algorithm toggles with Thai phonetic disabled and enabled
	algosWithoutThai := AlgorithmToggles{
		UseJaroWinkler:  true,
		UseLevenshtein:  true,
		UseTokenSort:    true,
		UsePhonetic:     true,
		UseTrigram:      true,
		UseThaiPhonetic: false,
	}

	algosWithThai := AlgorithmToggles{
		UseJaroWinkler:  true,
		UseLevenshtein:  true,
		UseTokenSort:    true,
		UsePhonetic:     true,
		UseTrigram:      true,
		UseThaiPhonetic: true,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Normalize both names
			src := Normalize(tc.name1)
			dest := Normalize(tc.name2)

			// Score with UseThaiPhonetic = false
			resultFalse := CalculateCompositeScore(src, dest, defaultDate, defaultDate, defaultWeights, algosWithoutThai, 30)

			// Score with UseThaiPhonetic = true
			resultTrue := CalculateCompositeScore(src, dest, defaultDate, defaultDate, defaultWeights, algosWithThai, 30)

			// Check each score component for equality
			// For Latin pairs, scores must be identical because Thai phonetic metric should not fire
			if resultFalse.TotalScore != resultTrue.TotalScore {
				t.Errorf("%s: TotalScore differs: false=%f, true=%f (delta=%f)",
					tc.name, resultFalse.TotalScore, resultTrue.TotalScore, resultTrue.TotalScore-resultFalse.TotalScore)
			}
			if resultFalse.NameScore != resultTrue.NameScore {
				t.Errorf("%s: NameScore differs: false=%f, true=%f", tc.name, resultFalse.NameScore, resultTrue.NameScore)
			}
			if resultFalse.JWScore != resultTrue.JWScore {
				t.Errorf("%s: JWScore differs: false=%f, true=%f", tc.name, resultFalse.JWScore, resultTrue.JWScore)
			}
			if resultFalse.LevScore != resultTrue.LevScore {
				t.Errorf("%s: LevScore differs: false=%f, true=%f", tc.name, resultFalse.LevScore, resultTrue.LevScore)
			}
			if resultFalse.TokenScore != resultTrue.TokenScore {
				t.Errorf("%s: TokenScore differs: false=%f, true=%f", tc.name, resultFalse.TokenScore, resultTrue.TokenScore)
			}
			if resultFalse.TrigramScore != resultTrue.TrigramScore {
				t.Errorf("%s: TrigramScore differs: false=%f, true=%f", tc.name, resultFalse.TrigramScore, resultTrue.TrigramScore)
			}
		})
	}
}
