package matcher

import (
	"fmt"
	"testing"
)

// These guard the stack-overflow crash where romanizeInitial's default branch
// used a byte-length check: a single 3-byte Thai consonant looked like a
// cluster and recursed on itself forever, taking the whole process down.
func TestRomanizeInitialSingleUnmappedRuneDoesNotRecurse(t *testing.T) {
	// Reaching this assertion is proof there is no infinite recursion
	result := romanizeInitial("ฤ")
	if result != "rue" {
		t.Errorf("romanizeInitial(\"ฤ\") = %q, want \"rue\"", result)
	}

	// Reaching this assertion is proof there is no infinite recursion
	result = romanizeInitial("ฦ")
	if result != "lue" {
		t.Errorf("romanizeInitial(\"ฦ\") = %q, want \"lue\"", result)
	}
}

// TestRomanizeInitialEveryThaiConsonantTerminates ensures that every Thai consonant
// codepoint from 0x0E01 to 0x0E2E (inclusive) terminates without hanging or crashing.
// This test would have caught the original bug for ANY unmapped consonant, not just
// the two known culprits ฤ and ฦ.
func TestRomanizeInitialEveryThaiConsonantTerminates(t *testing.T) {
	for r := rune(0x0E01); r <= 0x0E2E; r++ {
		t.Run(fmt.Sprintf("U+%04X", r), func(t *testing.T) {
			result := romanizeInitial(string(r))
			t.Logf("Input rune U+%04X -> result %q", r, result)
			// Just confirming it doesn't hang or crash is sufficient
		})
	}
}

func TestRomanizeInitialClusterStillMaps(t *testing.T) {
	result := romanizeInitial("กร")
	if result != "kr" {
		t.Errorf("romanizeInitial(\"กร\") = %q, want \"kr\"", result)
	}
}

func TestRomanizeThaiSurvivesNameContainingRue(t *testing.T) {
	input := "พฤกษา"
	result := RomanizeThai(input)
	if result == "" {
		t.Errorf("RomanizeThai(%q) returned empty string", input)
	}
	t.Logf("Input: %q -> Romanized: %q", input, result)
}
