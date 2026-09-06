package matcher

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPhoneticSkeletonFoldsJToCh(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
	}{
		{
			name: "Jaidee vs chaidi",
			a:    "Jaidee",
			b:    "chaidi",
		},
		{
			name: "Somchai Jaidee vs somchai chaidi",
			a:    "Somchai Jaidee",
			b:    "somchai chaidi",
		},
		{
			name: "Jira vs chira",
			a:    "Jira",
			b:    "chira",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, PhoneticSkeleton(tt.a), PhoneticSkeleton(tt.b))
		})
	}

	// Additional assertions for specific cases
	require.Equal(t, "smchchd", PhoneticSkeleton("Somchai Jaidee"))
	require.Equal(t, "smchchd", PhoneticSkeleton("somchai chaidi"))
	require.Equal(t, "chd", PhoneticSkeleton("Jaidee"))
}

func TestRomanizationStaysRTGSCorrect(t *testing.T) {
	// the j->ch fold lives in the comparison key (PhoneticSkeleton via phoneticEquivalents),
	// so RomanizeThai's RTGS output is untouched by this change.
	romanized := RomanizeThai("ใจดี")
	require.Contains(t, romanized, "ch")
	require.NotContains(t, romanized, "j")
}

func TestPhoneticSkeletonDigraphsUnchanged(t *testing.T) {
	// Regression guard that the new equivalence table does not affect existing digraph handling
	require.Equal(t, "thnwt", PhoneticSkeleton("Thanawat"))
	require.Contains(t, PhoneticSkeleton("Phuket"), "ph")
	require.Contains(t, PhoneticSkeleton("Khon"), "kh")
	require.Contains(t, PhoneticSkeleton("Ngam"), "ng")
	require.Equal(t, "", PhoneticSkeleton(""))
}

func TestPhoneticSkeletonLeadingVowelPreserved(t *testing.T) {
	result := PhoneticSkeleton("Anan")
	require.True(t, strings.HasPrefix(result, "a"), "expected leading vowel 'a' to be preserved, got %q", result)
}

func TestPhoneticComparisonFormFoldsJ(t *testing.T) {
	require.Equal(t, "chaidee", PhoneticComparisonForm("Jaidee"))
	require.Equal(t, "chaidi", PhoneticComparisonForm("chaidi"))
	require.Equal(t, "", PhoneticComparisonForm(""))
}

func TestCrossScriptPartsScoreAlignsJAndCh(t *testing.T) {
	// This score was approximately 0.6667 before the PhoneticComparisonForm fold,
	// because the untouched vowel-bearing comparison path bound the score down
	// by treating "jaidee" and "chaidi" as distinct due to differing vowel patterns.
	score := CrossScriptPartsScore([]string{"somchai", "jaidee"}, []string{"สมชาย", "ใจดี"})
	require.Greater(t, score, 0.85)

	unrelatedScore := CrossScriptPartsScore([]string{"somchai", "jaidee"}, []string{"สมศักดิ์", "ทองดี"})
	require.Less(t, unrelatedScore, score)
	// The fold ensures that "jaidee" and "chaidi" align well, but does not
	// artificially inflate similarity for truly unrelated names.
}

func TestRomanizationOutputUnchangedByFold(t *testing.T) {
	// The fold is applied at comparison time only, never to stored/displayed romanization
	tokens := RomanizeThaiTokens([]string{"ใจดี"})
	require.Len(t, tokens, 1)
	require.Contains(t, tokens[0], "ch")
	require.NotContains(t, tokens[0], "j")
}
