package matcher

import (
	"testing"
)

func TestNormalizerC1C2(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedCleaned string
	}{
		// C1 - Token-boundary title stripping
		{
			name:            "Mr. William Williams",
			input:           "Mr. William Williams",
			expectedCleaned: "william williams",
		},
		{
			name:            "Adams Thomas",
			input:           "Adams Thomas",
			expectedCleaned: "adams thomas",
		},
		{
			name:            "Vincent Prince Inc.",
			input:           "Vincent Prince Inc.",
			expectedCleaned: "vincent prince",
		},
		{
			name:            "Miss Emma Watson",
			input:           "Miss Emma Watson",
			expectedCleaned: "emma watson",
		},
		{
			name:            "Thai: สมชาย ไทยนาย (suffix not stripped)",
			input:           "สมชาย ไทยนาย",
			expectedCleaned: "สมชาย ไทยนาย",
		},
		{
			name:            "Thai: นาย สมชาย เข็มกลัด (space-separated title)",
			input:           "นาย สมชาย เข็มกลัด",
			expectedCleaned: "สมชาย เข็มกลัด",  // Thai diacritics preserved
		},
		{
			name:            "Thai: นายสมชาย เข็มกลัด (prefix stripping)",
			input:           "นายสมชาย เข็มกลัด",
			expectedCleaned: "สมชาย เข็มกลัด",  // Thai diacritics preserved
		},
		{
			name:            "Bangkok Bank Public Company Limited",
			input:           "Bangkok Bank Public Company Limited",
			expectedCleaned: "bangkok bank",
		},
		{
			name:            "Thai corporate with title suffix",
			input:           "บริษัท สยามพารากอน ดีเวลลอปเม้นท์ จำกัด",
			expectedCleaned: "สยามพารากอน ดีเวลลอปเม้นท์",
		},
		// C2 - Synonym dictionary wiring
		{
			name:            "KBank alias",
			input:           "KBank",
			expectedCleaned: "kasikornbank",
		},
		{
			name:            "Thai Kasikornbank",
			input:           "กสิกรไทย",
			expectedCleaned: "kasikornbank",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Normalize(tt.input)
			if res.Cleaned != tt.expectedCleaned {
				t.Errorf("Normalize(%q).Cleaned = %q; want %q", tt.input, res.Cleaned, tt.expectedCleaned)
			}
		})
	}
}

func TestPhoneticKey(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectDifferent   string // Should be different from this
		expectSameKey     bool   // Whether to expect same key
	}{
		{
			name:              "somchai vs sumchai (vowel only diff - should collide)",
			input:             "somchai",
			expectDifferent:   "sumchai",
			expectSameKey:     true,
		},
		{
			name:              "somchai vs sonchai (consonant diff - should NOT collide)",
			input:             "somchai",
			expectDifferent:   "sonchai",
			expectSameKey:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := GeneratePhoneticKey(tt.input)
			key2 := GeneratePhoneticKey(tt.expectDifferent)

			if tt.expectSameKey && key1 != key2 {
				t.Errorf("GeneratePhoneticKey(%q) = %q; GeneratePhoneticKey(%q) = %q; expected same keys", tt.input, key1, tt.expectDifferent, key2)
			}
			if !tt.expectSameKey && key1 == key2 {
				t.Errorf("GeneratePhoneticKey(%q) = %q; GeneratePhoneticKey(%q) = %q; expected different keys", tt.input, key1, tt.expectDifferent, key2)
			}
		})
	}
}

func TestRunePrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		n        int
		expected string
	}{
		{
			name:     "simple ASCII",
			input:    "hello",
			n:        3,
			expected: "hel",
		},
		{
			name:     "Thai text",
			input:    "สมชาย",
			n:        2,
			expected: "สม",
		},
		{
			name:     "n greater than length",
			input:    "hi",
			n:        5,
			expected: "hi",
		},
		{
			name:     "n equals length",
			input:    "hello",
			n:        5,
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RunePrefix(tt.input, tt.n)
			if result != tt.expected {
				t.Errorf("RunePrefix(%q, %d) = %q; want %q", tt.input, tt.n, result, tt.expected)
			}
		})
	}
}
