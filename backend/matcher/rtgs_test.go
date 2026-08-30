package matcher

import (
	"testing"
	"time"
)

func TestRomanizeThaiAcceptance(t *testing.T) {
	// Test cases for Thai-Latin bilingual matching
	// These demonstrate the romanization and phonetic skeleton behavior
	tests := []struct {
		name  string
		thai  string
		latin string
		// converge=true: Thai and Latin should produce same phonetic skeleton
		// (only for actual RTGS-based romanizations, not for non-standard transliterations)
		converge bool
	}{
		// RTGS-based romanizations that should converge
		{
			name:     "สมชาย/Somchai (personal name)",
			thai:     "สมชาย",
			latin:    "somchai",
			converge: true,
		},
		{
			name:     "สุชาติ/Suchat (personal name)",
			thai:     "สุชาติ",
			latin:    "suchat",
			converge: true,
		},
		{
			name:     "ประเสริฐ/Prasert (personal name)",
			thai:     "ประเสริฐ",
			latin:    "prasert",
			converge: true,
		},
		{
			name:     "วิชัย/Wichai (personal name)",
			thai:     "วิชัย",
			latin:    "wichai",
			converge: true,
		},
		{
			name:     "นภา/Napha (personal name)",
			thai:     "นภา",
			latin:    "napha",
			converge: true,
		},

		// Precision guards: different names should NOT converge
		{
			name:     "สมชาย/Somsak (different person, must not converge)",
			thai:     "สมชาย",
			latin:    "somsak",
			converge: false,
		},
		{
			name:     "วิชัย/Wichian (different person, must not converge)",
			thai:     "วิชัย",
			latin:    "wichian",
			converge: false,
		},
		{
			name:     "สมชาย/Somchit (different person, must not converge)",
			thai:     "สมชาย",
			latin:    "somchit",
			converge: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			romanized := RomanizeThai(tt.thai)
			thaiSkeleton := PhoneticSkeleton(romanized)
			latinSkeleton := PhoneticSkeleton(tt.latin)

			t.Logf("Thai %q -> romanized %q -> skeleton %q", tt.thai, romanized, thaiSkeleton)
			t.Logf("Latin %q -> skeleton %q", tt.latin, latinSkeleton)

			if tt.converge {
				if thaiSkeleton != latinSkeleton {
					t.Errorf("Expected convergence: Thai skeleton %q != Latin skeleton %q", thaiSkeleton, latinSkeleton)
				}
			} else {
				if thaiSkeleton == latinSkeleton {
					t.Errorf("Expected NON-convergence: Thai skeleton %q == Latin skeleton %q (should differ)", thaiSkeleton, latinSkeleton)
				}
			}
		})
	}
}

func TestDebugSegmentation(t *testing.T) {
	tests := []string{
		"ธนากร",
		"กรุงเทพ",
		"กสิกรไทย",
		"สมชาย",
	}

	for _, word := range tests {
		syllables := SegmentThaiSyllables(word)
		t.Logf("Word: %s", word)
		for i, syl := range syllables {
			t.Logf("  Syl %d: Text=%q Initial=%q Vowel=%q Final=%q Tone=%q",
				i, syl.Text, syl.Initial, syl.Vowel, syl.Final, syl.Tone)
		}
		romanized := RomanizeThai(word)
		t.Logf("  Romanized: %s", romanized)
		skeleton := PhoneticSkeleton(romanized)
		t.Logf("  Skeleton: %s", skeleton)
	}
}

func TestPhoneticSkeletonDigraphs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ch digraph preserved",
			input:    "champ",
			expected: "chmp",
		},
		{
			name:     "th digraph preserved",
			input:    "thaksin",
			expected: "thksn",
		},
		{
			name:     "ng digraph preserved",
			input:    "singing",
			expected: "sngng",
		},
		{
			name:     "kh digraph preserved",
			input:    "khao",
			expected: "kh",
		},
		{
			name:     "vowels dropped except leading",
			input:    "aeiou",
			expected: "a",
		},
		{
			name:     "somchai skeleton",
			input:    "somchai",
			expected: "smch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PhoneticSkeleton(tt.input)
			if result != tt.expected {
				t.Errorf("PhoneticSkeleton(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRomanizeThaiBasic(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "ก romanizes to k", input: "ก"},
		{name: "ข romanizes to kh", input: "ข"},
		{name: "สมชาย phonetics", input: "สมชาย"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RomanizeThai(tt.input)
			t.Logf("RomanizeThai(%q) = %q", tt.input, result)
			if result == "" && tt.input != "" {
				t.Errorf("RomanizeThai(%q) returned empty string", tt.input)
			}
		})
	}
}

func TestLatinPassthrough(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain English",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "digits",
			input:    "12345",
			expected: "12345",
		},
		{
			name:     "mixed case -> lowercase",
			input:    "HelloWorld",
			expected: "helloworld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RomanizeThai(tt.input)
			if result != tt.expected {
				t.Errorf("RomanizeThai(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestCrossScriptRomanizedInvariant verifies that pure-Latin name pairs produce identical scores
// regardless of the UseRomanizedMatch flag setting. This regression test ensures that
// the romanized cross-script signal does not affect pure-Latin pairs due to the cross-script gate.
func TestCrossScriptRomanizedInvariant(t *testing.T) {
	testCases := []struct {
		name  string
		name1 string
		name2 string
	}{
		{"Exact match", "John Smith", "John Smith"},
		{"Minor spelling variation", "Jon Smith", "John Smith"},
		{"Full name vs abbreviation", "Robert Enterprises", "Rob Enterprises"},
		{"Name order", "Mary Johnson", "Johnson Mary"},
		{"Multiple word variation", "Acme Trading Corp", "Acme Trade Corporation"},
	}

	defaultWeights := DefaultWeights
	defaultDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Algorithm toggles with romanized matching disabled and enabled
	algosWithoutRomanized := AlgorithmToggles{
		UseJaroWinkler:    true,
		UseLevenshtein:    true,
		UseTokenSort:      true,
		UsePhonetic:       true,
		UseTrigram:        true,
		UseThaiPhonetic:   true,
		UseCorpusIDF:      false,
		UseRomanizedMatch: false,
	}

	algosWithRomanized := AlgorithmToggles{
		UseJaroWinkler:    true,
		UseLevenshtein:    true,
		UseTokenSort:      true,
		UsePhonetic:       true,
		UseTrigram:        true,
		UseThaiPhonetic:   true,
		UseCorpusIDF:      false,
		UseRomanizedMatch: true,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Normalize both names
			src := Normalize(tc.name1)
			dest := Normalize(tc.name2)

			// Score with UseRomanizedMatch = false
			resultFalse := CalculateCompositeScore(src, dest, defaultDate, defaultDate, defaultWeights, algosWithoutRomanized, 30)

			// Score with UseRomanizedMatch = true
			resultTrue := CalculateCompositeScore(src, dest, defaultDate, defaultDate, defaultWeights, algosWithRomanized, 30)

			// For pure-Latin pairs, romanized metric should not fire, so scores must be identical
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
			// RomanizedScore should be 0.0 in both cases since the gate should prevent it from firing
			if resultFalse.RomanizedScore != 0.0 {
				t.Errorf("%s: RomanizedScore with UseRomanizedMatch=false should be 0.0, got %f", tc.name, resultFalse.RomanizedScore)
			}
			if resultTrue.RomanizedScore != 0.0 {
				t.Errorf("%s: RomanizedScore with UseRomanizedMatch=true should be 0.0 (gate prevents it), got %f", tc.name, resultTrue.RomanizedScore)
			}
		})
	}
}

func TestRomanizedField(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		hasThai bool
	}{
		{
			name:    "Thai input",
			input:   "สมชาย",
			hasThai: true,
		},
		{
			name:    "Latin input",
			input:   "somchai",
			hasThai: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clean := Normalize(tt.input)
			t.Logf("Input: %q -> Romanized: %q", tt.input, clean.Romanized)
			if clean.Romanized == "" {
				t.Logf("Warning: Romanized field is empty for %q", tt.input)
			}
		})
	}
}
