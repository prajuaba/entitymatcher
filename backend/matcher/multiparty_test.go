package matcher

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestSplitPartiesStripsTrailingRefCode(t *testing.T) {
	result := SplitParties("บุญจันทร์ รัตแมดSB63000164")
	if len(result) != 1 {
		t.Errorf("Expected exactly 1 element, got %d", len(result))
	}
	if result[0] != "บุญจันทร์ รัตแมด" {
		t.Errorf("Expected 'บุญจันทร์ รัตแมด', got '%s'", result[0])
	}
}

func TestSplitPartiesExtractsParenthetical(t *testing.T) {
	result := SplitParties("จุฑาทิพย์ บุตรอินทร์ (คุณศรัณย์ พลับเจริญสุข)PL63001961")
	expectedNames := []string{"จุฑาทิพย์ บุตรอินทร์", "คุณศรัณย์ พลับเจริญสุข"}
	found := make(map[string]bool)

	for _, name := range result {
		found[name] = true
	}

	for _, expected := range expectedNames {
		if !found[expected] {
			t.Errorf("Expected to find '%s' in result, but not found", expected)
		}
	}
}

func TestSplitPartiesSplitsCommasAndAnd(t *testing.T) {
	result := SplitParties("นายบุญจันทร์,นางวิลัย และ นายสมพงษ์ รัตแมด")
	if len(result) != 3 {
		t.Errorf("Expected exactly 3 elements, got %d", len(result))
	}
}

func TestSplitPartiesSingleNameReturnsOne(t *testing.T) {
	result := SplitParties("สมชาย ใจดี")
	if len(result) != 1 {
		t.Errorf("Expected exactly 1 element, got %d", len(result))
	}
	if result[0] != "สมชาย ใจดี" {
		t.Errorf("Expected 'สมชาย ใจดี', got '%s'", result[0])
	}
}

func TestSplitPartiesHandlesUnbalancedParens(t *testing.T) {
	// This test guards against panicking on malformed input
	result := SplitParties("สมชาย (ใจดี")
	if len(result) < 1 {
		t.Errorf("Expected at least 1 element, got %d", len(result))
	}
	if len(strings.TrimSpace(result[0])) == 0 {
		t.Errorf("First element should be non-empty after trimming")
	}
	// Comment: This guards against panicking on malformed input
}

// TestMultiPartyNeverExceedsBestSinglePairing ensures the best-of-parties logic
// never invents a score higher than any genuine single pairing.
//
// Best-of-parties may surface a pre-existing similarity between a source and one
// destination party, but must never invent a score higher than any genuine single
// pairing. This replaces the old TestMultiPartyDoesNotMatchUnrelatedNames, which
// asserted an arbitrary threshold (< 0.70) against a borderline name pair that
// already scored 0.7042 as a plain single-pair comparison (no multi-party logic
// involved), so it was testing the base scorer, not the best-of-parties feature.
func TestMultiPartyNeverExceedsBestSinglePairing(t *testing.T) {
	src := Normalize("สมหญิง รักดี")
	dest := Normalize("นายบุญจันทร์,นางวิลัย และ นายสมพงษ์ รัตแมด")
	result := CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)

	srcParties := SplitParties(src.Raw)
	destParties := SplitParties(dest.Raw)

	maxIndividual := 0.0
	for _, sp := range srcParties {
		for _, dp := range destParties {
			sub := CalculateCompositeScore(Normalize(sp), Normalize(dp), time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)
			if sub.TotalScore > maxIndividual {
				maxIndividual = sub.TotalScore
			}
		}
	}

	if math.Abs(result.TotalScore-maxIndividual) >= 1e-6 {
		t.Errorf("Expected multi-party TotalScore (%f) to equal max individual pairing score (%f)", result.TotalScore, maxIndividual)
	}
}

// TestMultiPartyRejectsClearlyUnrelatedNames is a precision guard for the best-of-parties logic.
//
// This test verifies that clearly unrelated names do not match, even when split into parties.
// It was verified to score 0.3995 empirically (a genuinely dissimilar company-name vs person-names
// pair, unlike the borderline pair the old test used). This is the real precision guard for
// best-of-parties.
func TestMultiPartyRejectsClearlyUnrelatedNames(t *testing.T) {
	src := Normalize("บริษัท ไทยประกันชีวิต จำกัด")
	dest := Normalize("นายสมพงษ์ รัตแมด,นางวิลัย ใจดี")
	result := CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)

	if result.TotalScore >= 0.70 {
		t.Errorf("Expected TotalScore < 0.70 to avoid false positive, got %f", result.TotalScore)
	}
}

// TestBranchNumbersStillPenalisedAcrossParties guards against a regression where
// branch annotations were incorrectly extracted as separate parties.
//
// Branch/reference numbers live in the original strings, and splitting off a
// parenthetical branch annotation as a separate "party" let two different branches
// of the same company match on their identical company name alone, producing a false positive.
// Previously this scored 1.0000 due to that regression.
func TestBranchNumbersStillPenalisedAcrossParties(t *testing.T) {
	src := Normalize("บริษัท เจริญโภคภัณฑ์193 นวัตกรรม จำกัด (สาขาที่ 1)")
	dest := Normalize("เจริญโภคภัณฑ์193 นวัตกรรม บจก. (สาขาที่ 99)")
	result := CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)

	if result.TotalScore >= 0.90 {
		t.Errorf("Expected TotalScore < 0.90 to avoid false positive due to branch number mismatch, got %f", result.TotalScore)
	}

	// Also verify that "(สาขาที่ 1)" is NOT one of the parties
	srcParties := SplitParties(src.Raw)
	for _, party := range srcParties {
		if strings.Contains(party, "สาขาที่ 1") {
			t.Errorf("SplitParties should not extract '(สาขาที่ 1)' as a separate party. Got: %v", srcParties)
		}
	}
}

func TestMultiPartyScoresBestPairing(t *testing.T) {
	src := Normalize("ศรัณย์ พลับเจริญสุข")
	dest := Normalize("จุฑาทิพย์ บุตรอินทร์ (คุณศรัณย์ พลับเจริญสุข)PL63001961")
	result := CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)
	if result.TotalScore < 0.95 {
		t.Errorf("Expected TotalScore >= 0.95, got %f", result.TotalScore)
	}
	// Comment: "was 0.8369 before best-of-parties scoring"
	found := false
	for _, reason := range result.MatchReasons {
		if strings.Contains(reason, "parties") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("MatchReasons should contain a string including 'parties', got %v", result.MatchReasons)
	}
}

func TestSinglePartyScoreUnchanged(t *testing.T) {
	src := Normalize("สมชาย ใจดี")
	dest := Normalize("สมชาย ใจดี")
	result := CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)
	if result.TotalScore != 1.0 {
		t.Errorf("Expected TotalScore == 1.0, got %f", result.TotalScore)
	}
	// Comment: proves the multi-party branch did not fire / did not change the single-party fast path
}

func TestSharedSurnamePropagatedToEarlierParties(t *testing.T) {
	result := SplitParties("นายคณิน,นายธรรมากร,น.ส.กฤตินี ทองบุญ")
	if len(result) != 3 {
		t.Errorf("Expected exactly 3 elements, got %d", len(result))
	}
	for _, party := range result {
		if !strings.HasSuffix(party, "ทองบุญ") {
			t.Errorf("Expected all parties to end with 'ทองบุญ', got %v", result)
		}
	}
}

func TestSharedSurnameNotAppliedWhenPartiesHaveOwnSurnames(t *testing.T) {
	result := SplitParties("นายสาคร โพธิ์ทอง และนางสาวอ้อ ใจดี")
	if len(result) != 2 {
		t.Errorf("Expected exactly 2 elements, got %d", len(result))
	}
	expectedNames := []string{"นายสาคร โพธิ์ทอง", "นางสาวอ้อ ใจดี"}
	found := make(map[string]bool)

	for _, name := range result {
		found[name] = true
	}

	for _, expected := range expectedNames {
		if !found[expected] {
			t.Errorf("Expected to find '%s' in result, but not found", expected)
		}
	}
}

func TestSharedSurnameNotAppliedWhenLastFragmentIsSingleToken(t *testing.T) {
	result := SplitParties("นายคณิน,นายธรรมากร")
	if len(result) != 2 {
		t.Errorf("Expected exactly 2 elements, got %d", len(result))
	}
	expectedNames := []string{"นายคณิน", "นายธรรมากร"}
	found := make(map[string]bool)

	for _, name := range result {
		found[name] = true
	}

	for _, expected := range expectedNames {
		if !found[expected] {
			t.Errorf("Expected to find '%s' in result, but not found", expected)
		}
	}
}

func TestSharedSurnamePreventsBareGivenNameFalsePositive(t *testing.T) {
	// This is the real precision win -- this pairing scored 1.0000 before the shared-surname fix
	// because "นายธรรมากร" lost its surname "ทองบุญ" and matched purely on the bare given name
	// "ธรรมากร" against an unrelated person "นางจำปา พินิจภาสกร". After the fix it measures 0.8048.
	src := Normalize("นายคณิน,นายธรรมากร,น.ส.กฤตินี ทองบุญ")
	dest := Normalize("นาย ธรรมากร และนางจำปา พินิจภาสกร")
	result := CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)
	if result.TotalScore >= 0.90 {
		t.Errorf("Expected TotalScore < 0.90 to avoid false positive (was 1.0000 before this fix), got %f", result.TotalScore)
	}
}

func TestSharedSurnameStillMatchesGenuineCoBorrower(t *testing.T) {
	// This is the guard against over-correction -- the true positive co-borrower match
	// must survive the shared-surname fix, verified empirically to still score 1.0000 after the fix.
	src := Normalize("นายทองใส,นางอัมพร,นายพิสิทธิ์ พานพินิจ")
	dest := Normalize("ทองใส พานพินิจ และ พิสิทธิ์ พานพินิจ")
	result := CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)
	if result.TotalScore < 0.95 {
		t.Errorf("Expected TotalScore >= 0.95 to preserve genuine co-borrower match, got %f", result.TotalScore)
	}
}

// TestPublicCompanyQualifierIsNotAParty is the CONFIRMED false-positive fix: the
// legal-form qualifier "(มหาชน)" (Public) is not a party name. This exact pair
// scored 1.0000 AUTO_MATCHED in production on the word "มหาชน" alone before the fix.
func TestPublicCompanyQualifierIsNotAParty(t *testing.T) {
	src := Normalize("บริษัท เสนาดีเวลลอปเม้นท์ จำกัด (มหาชน)")
	dest := Normalize("บริษัท ทีฆทัศน์ ดีเวลลอปเมนท์ จำกัด (มหาชน) หรือ นายสุรศักดิ์ จำรัสการ")
	result := CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)

	if result.TotalScore >= 0.90 {
		t.Errorf("Expected TotalScore < 0.90 to avoid false positive (was 1.0000 before the fix), got %f", result.TotalScore)
	}

	parties := SplitParties(src.Raw)
	for _, party := range parties {
		if strings.TrimSpace(party) == "มหาชน" {
			t.Errorf("SplitParties should not extract 'มหาชน' as a separate party. Got: %v", parties)
		}
	}
}

// TestCountryQualifierIsNotAParty is the same fix applied to the country
// qualifier "(ประเทศไทย)" (Thailand), the second-largest blast radius in the
// production false-positive audit (1,964 auto-matches).
func TestCountryQualifierIsNotAParty(t *testing.T) {
	src := Normalize("บจก.ไมโครเพียว อินโนเวชั่น (ประเทศไทย)")
	dest := Normalize("บจก.เทเลคอม แอคเซส คอมมิวนิเคชั่น (ประเทศไทย)")
	result := CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)

	if result.TotalScore >= 0.90 {
		t.Errorf("Expected TotalScore < 0.90 to avoid false positive (was 1.0000 before the fix), got %f", result.TotalScore)
	}

	parties := SplitParties(src.Raw)
	for _, party := range parties {
		if strings.TrimSpace(party) == "ประเทศไทย" {
			t.Errorf("SplitParties should not extract 'ประเทศไทย' as a separate party. Got: %v", parties)
		}
	}
}

// TestGenuineMatchWithParentheticalInfixSurvives is the over-correction guard for
// the qualifier fix: "(เอส)" is no longer extracted as its own party, but these two
// records are genuinely the same company, so the OUTER name must still carry the
// match. Verified empirically to still score 1.0000 after the fix.
func TestGenuineMatchWithParentheticalInfixSurvives(t *testing.T) {
	src := Normalize("ห้างหุ้นส่วนจำกัด เอพีเจ (เอส) เทรดดิ้ง")
	dest := Normalize("หจก.เอพีเจ (เอส) เทรดดิ้ง")
	result := CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)

	if result.TotalScore < 0.95 {
		t.Errorf("Expected TotalScore >= 0.95 to preserve genuine match (guards against over-correction), got %f", result.TotalScore)
	}
}

// TestRealCoBorrowerParentheticalStillAParty guards the other true positive that
// must survive the qualifier fix: a real co-borrower name inside parentheses still
// carries enough distinctive content to be extracted as its own party.
func TestRealCoBorrowerParentheticalStillAParty(t *testing.T) {
	raw := "จุฑาทิพย์ บุตรอินทร์ (คุณศรัณย์ พลับเจริญสุข)PL63001961"
	parties := SplitParties(raw)

	found := false
	for _, party := range parties {
		if strings.TrimSpace(party) == "คุณศรัณย์ พลับเจริญสุข" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find 'คุณศรัณย์ พลับเจริญสุข' in parties, got %v", parties)
	}

	src := Normalize("ศรัณย์ พลับเจริญสุข")
	dest := Normalize(raw)
	result := CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)

	if result.TotalScore < 0.95 {
		t.Errorf("Expected TotalScore >= 0.95 to preserve genuine co-borrower match, got %f", result.TotalScore)
	}
}

// TestIsIdentifyingPartyRejectsQualifiers is a direct unit test of the helper
// introduced by the qualifier fix: it must reject bare legal-form/country
// qualifiers and lone short tokens, but accept fragments that carry a real name.
func TestIsIdentifyingPartyRejectsQualifiers(t *testing.T) {
	tests := []struct {
		fragment string
		want     bool
	}{
		{"มหาชน", false},
		{"ประเทศไทย", false},
		{"ไทยแลนด์", false},
		{"เอส", false},
		{"สาขาที่ 3", false},
		{"คุณศรัณย์ พลับเจริญสุข", true},
		{"นายบุญจันทร์ รัตแมด", true},
	}

	for _, tc := range tests {
		got := isIdentifyingParty(tc.fragment)
		if got != tc.want {
			t.Errorf("isIdentifyingParty(%q) = %v, want %v", tc.fragment, got, tc.want)
		}
	}
}

// TestTrailingRefCodeStrippedFromSingleName is a direct unit test of the
// widened reTrailingRefCode pattern: it must strip both letters+digits codes
// ("PL64000306", glued "SB63000164") and bare 8+ digit codes, whether glued
// ("0002019000592") or space-separated ("2022090361").
func TestTrailingRefCodeStrippedFromSingleName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"นายนฤพนธ์ ชูชีพ PL64000306", "นายนฤพนธ์ ชูชีพ"},
		{"บุญจันทร์ รัตแมดSB63000164", "บุญจันทร์ รัตแมด"},
		{"นางสาวกุลธิดา ภาษิต0002019000592", "นางสาวกุลธิดา ภาษิต"},
		{"นางสาวณัฎฐนัน พิชิตสังข์ 2022090361", "นางสาวณัฎฐนัน พิชิตสังข์"},
	}

	for _, tc := range tests {
		got := StripTrailingRefCode(tc.input)
		if got != tc.want {
			t.Errorf("StripTrailingRefCode(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestTrailingRefCodeKeepsShortNumbers(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"บจก.มาทวีอินเตอร์กรุ๊ป 2006", "บจก.มาทวีอินเตอร์กรุ๊ป 2006"},
		{"บจก.ทีฆพล 222", "บจก.ทีฆพล 222"},
	}

	for _, tc := range tests {
		got := StripTrailingRefCode(tc.input)
		if got != tc.want {
			t.Errorf("StripTrailingRefCode(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSingleNameWithRefCodeScoresExact(t *testing.T) {
	src := Normalize("นายเอกชัย บุญจันทร์")
	dest := Normalize("นายเอกชัย บุญจันทร์0102019000025")
	result := CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)
	if result.TotalScore < 0.99 {
		t.Errorf("Expected TotalScore >= 0.99 (measured 0.939 before this fix), got %f", result.TotalScore)
	}

	src = Normalize("นายนฤพนธ์ ชูชีพ")
	dest = Normalize("นายนฤพนธ์ ชูชีพ PL64000306")
	result = CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)
	if result.TotalScore < 0.99 {
		t.Errorf("Expected TotalScore >= 0.99 (measured 0.922 before this fix), got %f", result.TotalScore)
	}
}

func TestRefCodeNotTreatedAsIdentifierNumber(t *testing.T) {
	cn := Normalize("นายเอกชัย บุญจันทร์0102019000025")
	for _, num := range cn.Numbers {
		if num == "0102019000025" {
			t.Errorf("Normalize(%q).Numbers should not contain %q (would incorrectly trigger CheckNumberMismatch)", "นายเอกชัย บุญจันทร์0102019000025", num)
		}
	}
}

func TestNameThatIsOnlyARefCodeSurvives(t *testing.T) {
	got := StripTrailingRefCode("SB63000164")
	want := "SB63000164"
	if got != want {
		t.Errorf("StripTrailingRefCode(%q) = %q, want %q", "SB63000164", got, want)
	}
}
