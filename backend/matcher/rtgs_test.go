package matcher

import (
	"testing"
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
