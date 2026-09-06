package matcher

import (
	"strings"
	"testing"
	"time"
)

func TestNangSaoNormalizesSameAsNorSor(t *testing.T) {
	res1 := Normalize("นางสาวกัลยา คุ้มพงษ์")
	res2 := Normalize("น.ส.กัลยา คุ้มพงษ์")

	if res1.Cleaned != res2.Cleaned {
		t.Errorf("Normalize(\"นางสาวกัลยา คุ้มพงษ์\").Cleaned = %q; want %q", res1.Cleaned, res2.Cleaned)
	}
}

func TestGluedTitleVariantsAllNormalizeAlike(t *testing.T) {
	variants := []string{
		"นางสาวสมหญิง ใจดี",
		"น.ส.สมหญิง ใจดี",
		"น.ส. สมหญิง ใจดี",
		"นส.สมหญิง ใจดี",
	}

	base := Normalize(variants[0])
	var mismatches []string

	for _, variant := range variants {
		res := Normalize(variant)
		if res.Cleaned != base.Cleaned {
			mismatches = append(mismatches, variant)
		}
	}

	if len(mismatches) > 0 {
		t.Errorf("Some variants did not normalize to the same value as %q: got %v", variants[0], mismatches)
	}
}

func TestCorporateLegalFormsStripped(t *testing.T) {
	res := Normalize("ห้างหุ้นส่วนจำกัด ภูเก็ตไก่สด")

	if !strings.Contains(res.Cleaned, "ภูเก็ตไก่สด") {
		t.Errorf("Expected 'ภูเก็ตไก่สด' in cleaned result, got %q", res.Cleaned)
	}

	if strings.Contains(res.Cleaned, "ห้างหุ้นส่วน") {
		t.Errorf("Expected 'ห้างหุ้นส่วน' to be stripped from cleaned result, got %q", res.Cleaned)
	}
}

func TestDifferentPeopleWithSameTitleStayDistinct(t *testing.T) {
	srcClean := Normalize("น.ส.รัชดาภรณ์ แย้มขะมัง")
	destClean := Normalize("น.ส.ชัชฎาภรณ์ ไชยมงคล")

	if srcClean.Cleaned == destClean.Cleaned {
		t.Errorf("Different people normalized to same value: %q", srcClean.Cleaned)
	}

	result := CalculateCompositeScore(srcClean, destClean, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)

	if result.TotalScore >= 0.90 {
		t.Errorf("Expected score < 0.90 for different people with same title, got %.4f", result.TotalScore)
	}
}

func TestThaiPrefixListsAreLongestFirst(t *testing.T) {
	assertNoEarlierEntryIsPrefixOfLater := func(t *testing.T, listName string, list []string) {
		for i := 0; i < len(list); i++ {
			for j := i + 1; j < len(list); j++ {
				if list[i] != list[j] && strings.HasPrefix(list[j], list[i]) {
					t.Errorf("Prefix list %s: entry at index %d (%q) is a prefix of later entry at index %d (%q)", listName, i, list[i], j, list[j])
				}
			}
		}
	}

	assertNoEarlierEntryIsSuffixOfLater := func(t *testing.T, listName string, list []string) {
		for i := 0; i < len(list); i++ {
			for j := i + 1; j < len(list); j++ {
				if list[i] != list[j] && strings.HasSuffix(list[j], list[i]) {
					t.Errorf("Suffix list %s: entry at index %d (%q) is a suffix of later entry at index %d (%q)", listName, i, list[i], j, list[j])
				}
			}
		}
	}

	assertNoEarlierEntryIsPrefixOfLater(t, "thaiPersonalPrefixes", thaiPersonalPrefixes)
	assertNoEarlierEntryIsPrefixOfLater(t, "thaiCorporatePrefixes", thaiCorporatePrefixes)
	assertNoEarlierEntryIsSuffixOfLater(t, "thaiCorporateSuffixes", thaiCorporateSuffixes)
}

func TestNormalizeTitlesDoesNotOverStrip(t *testing.T) {
	// "นายก" is a real Thai word meaning "prime minister", starting with the prefix "นาย"
	// which is also a personal title. This test ensures that such glued forms are not
	// incorrectly stripped due to the >=2 rune guard.
	res := Normalize("นายก")

	if res.Cleaned != "นายก" {
		t.Errorf("Expected 'นายก' to remain unchanged, got %q", res.Cleaned)
	}
}
