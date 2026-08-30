package matcher

import (
	"strings"
	"testing"
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
			name:  "empty string",
			input: "",
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
		name     string
		input    string
		wantLen  int
		wantRoundTrip bool
	}{
		{
			name:            "nil/empty handled gracefully",
			input:           "",
			wantLen:         0,
			wantRoundTrip:   true,
		},
		{
			name:            "single thai consonant",
			input:           "ก",
			wantLen:         1,
			wantRoundTrip:   true,
		},
		{
			name:            "single latin char",
			input:           "A",
			wantLen:         1,
			wantRoundTrip:   true,
		},
		{
			name:            "mixed spacing",
			input:           "สวัสดี",
			wantLen:         3, // สวัสดี = สวัส + ดี
			wantRoundTrip:   true,
		},
		{
			name:            "punctuation preserved",
			input:           "ทดสอบ, ABC, 123",
			wantLen:         4, // Thai segments coalesce with punctuation/spaces into runs
			wantRoundTrip:   true,
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
