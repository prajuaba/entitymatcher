package matcher

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSegmentThaiSyllables(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Syllable
	}{
		{
			name:  "สมชาย - critical test for initial vs final",
			input: "สมชาย",
			expected: []Syllable{
				{Text: "สม", Initial: "ส", Final: "ม"},
				{Text: "ชาย", Initial: "ช", Vowel: "า", Final: "ย"},
			},
		},
		{
			name:  "วิชัย - vowel mark handling",
			input: "วิชัย",
			expected: []Syllable{
				{Text: "วิ", Initial: "ว", Vowel: "ิ"},
				{Text: "ชัย", Initial: "ช", Vowel: "ั", Final: "ย"},
			},
		},
		{
			name:  "นภา - implied vowels and following vowel",
			input: "นภา",
			expected: []Syllable{
				{Text: "น", Initial: "น"},
				{Text: "ภา", Initial: "ภ", Vowel: "า"},
			},
		},
		{
			name:  "ธนากร - multiple syllables with cluster",
			input: "ธนากร",
			expected: []Syllable{
				{Text: "ธ", Initial: "ธ"},
				{Text: "นา", Initial: "น", Vowel: "า"},
				{Text: "กร", Initial: "ก", Final: "ร"},
			},
		},
		{
			name:  "กรุงเทพ - cluster with vowel and final consonant",
			input: "กรุงเทพ",
			expected: []Syllable{
				{Text: "กรุง", Initial: "กร", Vowel: "ุ", Final: "ง"},
				{Text: "เทพ", Initial: "ท", Vowel: "เ", Final: "พ"},
			},
		},
		{
			name:  "เอก - leading vowel with initial and final",
			input: "เอก",
			expected: []Syllable{
				{Text: "เอก", Initial: "อ", Vowel: "เ", Final: "ก"},
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []Syllable{},
		},
		{
			name:  "pure latin passthrough",
			input: "ABCD",
			expected: []Syllable{
				{Text: "ABCD"},
			},
		},
		{
			name:  "mixed thai and latin",
			input: "บริษัท ABC จำกัด",
			expected: []Syllable{
				{Text: "บริ", Initial: "บร", Vowel: "ิ"},
				{Text: "ษัท", Initial: "ษ", Vowel: "ั", Final: "ท"},
				{Text: " ABC ", Vowel: ""},
				{Text: "จำ", Initial: "จ", Vowel: "ำ"},
				{Text: "กัด", Initial: "ก", Vowel: "ั", Final: "ด"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SegmentThaiSyllables(tt.input)

			// Check segment count
			if len(got) != len(tt.expected) {
				t.Errorf("segment count mismatch: got %d, want %d", len(got), len(tt.expected))
			}

			// Check each segment
			for i := 0; i < len(got) && i < len(tt.expected); i++ {
				gotSyl := got[i]
				wantSyl := tt.expected[i]

				if gotSyl.Text != wantSyl.Text {
					t.Errorf("syllable %d Text mismatch: got %q, want %q", i, gotSyl.Text, wantSyl.Text)
				}
				if gotSyl.Initial != wantSyl.Initial {
					t.Errorf("syllable %d Initial mismatch: got %q, want %q", i, gotSyl.Initial, wantSyl.Initial)
				}
				if gotSyl.Vowel != wantSyl.Vowel {
					t.Errorf("syllable %d Vowel mismatch: got %q, want %q", i, gotSyl.Vowel, wantSyl.Vowel)
				}
				if gotSyl.Final != wantSyl.Final {
					t.Errorf("syllable %d Final mismatch: got %q, want %q", i, gotSyl.Final, wantSyl.Final)
				}
				if gotSyl.Tone != wantSyl.Tone {
					t.Errorf("syllable %d Tone mismatch: got %q, want %q", i, gotSyl.Tone, wantSyl.Tone)
				}
			}

			// Verify round-trip: concatenating Text values must reproduce input
			var reconstructed strings.Builder
			for _, syl := range got {
				reconstructed.WriteString(syl.Text)
			}
			if reconstructed.String() != tt.input {
				t.Errorf("round-trip failed: reconstructed %q != input %q", reconstructed.String(), tt.input)
			}
		})
	}
}

// TestSegmentThaiSyllablesKnownFailures documents cases that cannot be segmented correctly
// without lexical knowledge (Thai dictionary). These are Sanskrit/Pali-derived words with
// silent written vowels that close syllables.
func TestSegmentThaiSyllablesKnownFailures(t *testing.T) {
	failures := []struct {
		name           string
		input          string
		expectedOutput []Syllable
		reason         string
	}{
		{
			name:  "ประเสริฐ - Prasert/Praserit",
			input: "ประเสริฐ",
			expectedOutput: []Syllable{
				{Text: "ประ", Initial: "ปร", Vowel: "ะ"},
				{Text: "เสริฐ", Initial: "ส", Vowel: "เ", Final: "ฐ"},
			},
			reason: "The ร and ิ form a silent-vowel sequence: ร closes the preceding syllable, making เสร one unit with final ร (not shown), then ิฐ continues. Without a dictionary, the algorithm treats ร as starting a new syllable.",
		},
		{
			name:  "สุชาติ - Suchat",
			input: "สุชาติ",
			expectedOutput: []Syllable{
				{Text: "สุ", Initial: "ส", Vowel: "ุ"},
				{Text: "ชาติ", Initial: "ช", Vowel: "า", Final: "ต"},
			},
			reason: "The ิ is a silent mark on ต (thanthakhat-like usage), making ต the final of the ชา syllable. The algorithm sees the written ิ after ต and treats ต as starting a new syllable.",
		},
	}

	for _, f := range failures {
		t.Run(f.name, func(t *testing.T) {
			got := SegmentThaiSyllables(f.input)

			// Log the actual output for documentation
			t.Logf("Known failure (requires lexicon): %s", f.name)
			t.Logf("  Input: %q", f.input)
			t.Logf("  Expected: %d syllables", len(f.expectedOutput))
			t.Logf("  Actual:   %d syllables", len(got))
			t.Logf("  Reason: %s", f.reason)

			// Verify round-trip even though segmentation is wrong
			var reconstructed strings.Builder
			for _, syl := range got {
				reconstructed.WriteString(syl.Text)
			}
			if reconstructed.String() != f.input {
				t.Errorf("round-trip failed: reconstructed %q != input %q", reconstructed.String(), f.input)
			}
		})
	}
}

func TestSegmentThaiSyllablesEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantLen       int
		wantRoundTrip bool
	}{
		{
			name:          "nil/empty handled gracefully",
			input:         "",
			wantLen:       0,
			wantRoundTrip: true,
		},
		{
			name:          "single thai consonant",
			input:         "ก",
			wantLen:       1,
			wantRoundTrip: true,
		},
		{
			name:          "single latin char",
			input:         "A",
			wantLen:       1,
			wantRoundTrip: true,
		},
		{
			name:          "mixed spacing",
			input:         "สวัสดี",
			wantLen:       3, // สวัสดี = สวัส + ดี
			wantRoundTrip: true,
		},
		{
			name:          "punctuation preserved",
			input:         "ทดสอบ, ABC, 123",
			wantLen:       4, // Thai segments coalesce with punctuation/spaces into runs
			wantRoundTrip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SegmentThaiSyllables(tt.input)

			// Check round-trip property (always required)
			var reconstructed strings.Builder
			for _, syl := range got {
				reconstructed.WriteString(syl.Text)
			}
			if reconstructed.String() != tt.input {
				t.Errorf("round-trip failed: reconstructed %q != input %q", reconstructed.String(), tt.input)
			}

			// Check length if specified
			if tt.wantLen > 0 && len(got) != tt.wantLen {
				t.Errorf("expected %d segments, got %d", tt.wantLen, len(got))
			}
		})
	}
}

// TestSegmentThaiSyllablesTermination is a NON-NEGOTIABLE test that ensures the segmenter
// terminates on all inputs, especially the previously-hanging cases where a leading vowel
// is followed by a consonant with no final.
func TestSegmentThaiSyllablesTermination(t *testing.T) {
	// Critical test cases that previously hung: leading vowel + consonant with no final
	hangCases := []string{
		"โน",        // leading vowel + single consonant
		"โลยี",      // leading vowel + consonant + vowel + consonant
		"โนโลยี",    // multiple leading-vowel cases
		"เทคโนโลยี", // mixed case with both final and non-final
		"โซลูชั่น",  // complex case with multiple vowels
	}

	// All leading vowels paired with single consonants (most likely to hang)
	leadingVowels := []rune{0xE40, 0xE41, 0xE42, 0xE43, 0xE44} // เ แ โ ใ ไ
	consonants := []rune{0xE01, 0xE0A, 0xE19, 0xE23, 0xE2A}    // ก ช น ร ส (sample)

	for _, tc := range hangCases {
		t.Run("Hang:"+tc, func(t *testing.T) {
			done := make(chan bool, 1)
			go func() {
				_ = SegmentThaiSyllables(tc)
				done <- true
			}()

			select {
			case <-done:
				// Test passed: segmenter terminated
			case <-time.After(2 * time.Second):
				t.Fatalf("SegmentThaiSyllables(%q) did not terminate within 2 seconds", tc)
			}
		})
	}

	// Test all combinations of leading vowel + consonant
	for _, vowel := range leadingVowels {
		for _, consonant := range consonants {
			input := string([]rune{vowel, consonant})
			t.Run("LeadingVowel:"+input, func(t *testing.T) {
				done := make(chan bool, 1)
				go func() {
					_ = SegmentThaiSyllables(input)
					done <- true
				}()

				select {
				case <-done:
					// Test passed: segmenter terminated
				case <-time.After(2 * time.Second):
					t.Fatalf("SegmentThaiSyllables(%q) did not terminate within 2 seconds", input)
				}
			})
		}
	}
}

// TestSegmentThaiSyllablesRandomProperty tests segmenter termination and lossless round-trip
// on randomly-generated Thai strings. This catches classes of bugs that hand-picked examples miss.
func TestSegmentThaiSyllablesRandomProperty(t *testing.T) {
	consonants := []rune{0xE01, 0xE04, 0xE08, 0xE0A, 0xE0D, 0xE0E, 0xE0F, 0xE10, 0xE13,
		0xE14, 0xE15, 0xE17, 0xE19, 0xE1A, 0xE1B, 0xE1C, 0xE1E, 0xE1F, 0xE21, 0xE23, 0xE25, 0xE27, 0xE2A, 0xE2B, 0xE2D}

	vowelMarks := []rune{0xE31, 0xE34, 0xE35, 0xE36, 0xE37, 0xE38, 0xE39, 0xE30, 0xE32, 0xE33}

	leadingVowels := []rune{0xE40, 0xE41, 0xE42, 0xE43, 0xE44}

	// Generate 50 random strings
	for testNum := 0; testNum < 50; testNum++ {
		// Build a random Thai string with 2-8 runes
		var input []rune
		length := 2 + (testNum % 7)
		for i := 0; i < length; i++ {
			choice := testNum*7 + i
			if choice%3 == 0 && i > 0 {
				// Add a vowel mark sometimes
				input = append(input, vowelMarks[choice%len(vowelMarks)])
			} else if choice%5 == 0 && i == 0 {
				// Start with a leading vowel sometimes
				input = append(input, leadingVowels[choice%len(leadingVowels)])
			} else {
				// Add a consonant
				input = append(input, consonants[choice%len(consonants)])
			}
		}

		inputStr := string(input)

		t.Run(fmt.Sprintf("Random_%d", testNum), func(t *testing.T) {
			// Test 1: Must terminate within 2 seconds
			done := make(chan []Syllable, 1)
			go func() {
				done <- SegmentThaiSyllables(inputStr)
			}()

			var result []Syllable
			select {
			case result = <-done:
				// Test passed: segmenter terminated
			case <-time.After(2 * time.Second):
				t.Fatalf("SegmentThaiSyllables(%q) did not terminate within 2 seconds", inputStr)
			}

			// Test 2: Must round-trip losslessly
			var reconstructed strings.Builder
			for _, syl := range result {
				reconstructed.WriteString(syl.Text)
			}
			if reconstructed.String() != inputStr {
				t.Errorf("Round-trip failed for %q: reconstructed %q", inputStr, reconstructed.String())
			}
		})
	}
}

func TestSegmentThaiSyllablesNoPanic(t *testing.T) {
	tests := []string{
		"",
		"สวัสดี",
		"Hello, World!",
		"123456",
		"สวัสดี123Hello",
		"   \t\n  ",
	}

	for _, input := range tests {
		t.Run("NoPanic:"+input, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("SegmentThaiSyllables panicked on input %q: %v", input, r)
				}
			}()

			_ = SegmentThaiSyllables(input)
		})
	}
}
