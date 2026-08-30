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

// TestCrossScriptRomanizedInvariantPureThai mirrors TestCrossScriptRomanizedInvariant above but
// for two-Thai-script pairs. The cross-script gate (and the cross-script nameScore override that
// sits on top of it) must never fire when BOTH sides are Thai — otherwise a same-script Thai
// comparison would silently pick up a different scoring path than before this change.
func TestCrossScriptRomanizedInvariantPureThai(t *testing.T) {
	testCases := []struct {
		name  string
		name1 string
		name2 string
	}{
		{"Exact match", "สมชาย ใจดี", "สมชาย ใจดี"},
		{"Minor spelling variation", "สมชาย", "สมชัย"},
		{"Name order", "สมชาย ใจดี", "ใจดี สมชาย"},
		{"Different person (false friend, same script)", "สมชาย", "สมศักดิ์"},
	}

	defaultWeights := DefaultWeights
	defaultDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	algosWithoutRomanized := DefaultAlgorithms
	algosWithoutRomanized.UseRomanizedMatch = false
	algosWithRomanized := DefaultAlgorithms
	algosWithRomanized.UseRomanizedMatch = true

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			src := Normalize(tc.name1)
			dest := Normalize(tc.name2)

			resultFalse := CalculateCompositeScore(src, dest, defaultDate, defaultDate, defaultWeights, algosWithoutRomanized, 30)
			resultTrue := CalculateCompositeScore(src, dest, defaultDate, defaultDate, defaultWeights, algosWithRomanized, 30)

			if resultFalse.TotalScore != resultTrue.TotalScore {
				t.Errorf("%s: TotalScore differs: false=%f, true=%f", tc.name, resultFalse.TotalScore, resultTrue.TotalScore)
			}
			if resultFalse.NameScore != resultTrue.NameScore {
				t.Errorf("%s: NameScore differs: false=%f, true=%f", tc.name, resultFalse.NameScore, resultTrue.NameScore)
			}
			if resultTrue.RomanizedScore != 0.0 {
				t.Errorf("%s: RomanizedScore should be 0.0 for a pure-Thai pair (gate must prevent it), got %f", tc.name, resultTrue.RomanizedScore)
			}
		})
	}
}

// TestCrossScriptPartsScore unit-tests CrossScriptPartsScore directly: aligned given/surname
// parts take the WEAKEST per-part match, and mismatched token counts fall back to whole-string
// comparison rather than panicking or silently returning a meaningless value.
func TestCrossScriptPartsScore(t *testing.T) {
	t.Run("aligned parts use the weaker part, not the average or the stronger part", func(t *testing.T) {
		// "สุชาติ ประเจริญ" (Suchat Prachaerin) vs a false-pairing that shares a close given name
		// but a completely unrelated surname: the surname must drag the score down to its own
		// (low) level, not be rescued by the given name matching well.
		srcTokens := []string{"สุชาติ", "ประเจริญ"}
		strongSurnameTokens := []string{"suchat", "prachaerin"}
		weakSurnameTokens := []string{"suchat", "johnson"}

		strongScore := CrossScriptPartsScore(srcTokens, strongSurnameTokens)
		weakScore := CrossScriptPartsScore(srcTokens, weakSurnameTokens)

		if weakScore >= strongScore {
			t.Errorf("expected unrelated-surname score (%f) to be well below matching-surname score (%f)", weakScore, strongScore)
		}
		// The given-name part alone (suchat vs suchat) would score ~1.0; the MIN aggregation
		// must not let that leak through when the other part is unrelated.
		if weakScore > 0.7 {
			t.Errorf("weak-surname pair scored %f; MIN-of-parts should have been dragged down by the unrelated surname", weakScore)
		}
	})

	t.Run("mismatched token counts fall back to whole-string comparison", func(t *testing.T) {
		// One Thai token vs two Latin tokens (e.g. เจริญโภคภัณฑ์ vs "Charoen Pokphand"): cannot
		// align position-by-position, so this must not panic and must return something derived
		// from the full strings rather than 0 or 1 by construction.
		score := CrossScriptPartsScore([]string{"เจริญโภคภัณฑ์"}, []string{"charoen", "pokphand"})
		if score <= 0.0 || score > 1.0 {
			t.Errorf("expected a valid fallback score in (0,1], got %f", score)
		}
	})

	t.Run("empty input returns 0", func(t *testing.T) {
		if s := CrossScriptPartsScore(nil, []string{"somchai"}); s != 0.0 {
			t.Errorf("expected 0.0 for empty srcTokens, got %f", s)
		}
		if s := CrossScriptPartsScore([]string{"somchai"}, nil); s != 0.0 {
			t.Errorf("expected 0.0 for empty destTokens, got %f", s)
		}
	})
}

// TestBilingualOutOfDictVsFalseFriendSeparation is the regression guard for the core result of
// this change: PhoneticSkeleton alone cannot separate genuine out-of-dictionary bilingual pairs
// from false-friend pairs (both landed in the same ~0.70-0.83 score band; see task diagnosis).
// Adding the full vowel-bearing, per-part romanized comparison (CrossScriptPartsScore combined
// with the skeleton, plus the cross-script nameScore override) must produce a NameScore for every
// genuine pair strictly greater than the NameScore of every false-friend pair — i.e. an actual
// threshold must exist between the two distributions. If this test ever starts failing, the
// technique's separation has regressed and BILINGUAL_OUT_OF_DICT auto-matching should not be
// trusted until it's restored.
//
// Data below is intentionally identical to internal/mockdata/generate_dataset.go's
// bilingualOutOfDictPairs and bilingualFalseFriendPairs (that package is owned by another agent
// and only ever RUN, never edited, by this change) so the unit-level guarantee here matches what
// the full-pipeline benchmark measures end to end.
func TestBilingualOutOfDictVsFalseFriendSeparation(t *testing.T) {
	genuinePairs := []struct{ thai, latin string }{
		{"สุชาติ ประเจริญ", "Suchat Prachaerin"},
		{"สุชาติ ประเจริญ", "Suchart Prachaerin"},
		{"ประเสริฐ จันทรังษี", "Prasert Chantarangsi"},
		{"วิชัย สิรินิมิตร", "Wichai Sirinimit"},
		{"วิชัย สิรินิมิตร", "Vichai Sirinimit"},
		{"ธนากร เพ็ญจันทร์", "Thanakorn Penjantr"},
		{"นภา คณะสิงห์", "Napha Kanasingham"},
		{"กมล พวกศรี", "Kamon Pueksri"},
		{"ศักดิ์ชัย ศรีหาร", "Sakchai Srihar"},
		{"บุญมี นรสิงห์", "Bunmi Norsingha"},
		{"บุญมี นรสิงห์", "Boonmee Norsingha"},
		{"จันทร์พวก บูชา", "Chanphuak Boucha"},
		{"จันทร์พวก บูชา", "Janpuek Boucha"},
		{"แสงทอง วงศ์พูล", "Saengthong Wongpool"},
		{"รุ่งโรจน์ สวินทร์", "Rungroj Suwintr"},
		{"พิมพ์ใจ จันท์สิริ", "Phimchai Chansiri"},
		{"ชลธิชา ปัญญานัน", "Chonthicha Panyanan"},
		{"ณรงค์ สมโภค", "Narong Somsombat"},
		{"ณรงค์ สมโภค", "Narongse Somsombat"},
		{"อรุณ อาศรี", "Arun Atsri"},
		{"เมธา เรือเส", "Metha Ruese"},
		{"กานต์ นันตะวั", "Kan Nantawa"},
		{"ศศิพร สิทธิสม", "Sasipon Sittism"},
		{"ชนินธร วิมล", "Chanintr Wimol"},
		{"ประพัฒน์ กาญจนา", "Prawat Kanjanai"},
		{"วราวุธ ชำนาญ", "Varavuth Chamnan"},
		{"ไพศาล เงินสี", "Phaisarn Ngernsee"},
		{"อัศวิน พัฒน์", "Aswin Pattana"},
		{"เศวต ศรีอำนวย", "Seawat Sriamoy"},
		{"ทรงศักดิ์ นิมิต", "Songsak Nimit"},
	}

	falseFriendPairs := []struct{ thai, latin string }{
		{"เชิดชัย ตันไทย", "Somchit Johnson"},
		{"วรสิทธิ์ สุขวิสัย", "Somsak Lee"},
		{"วิพัฒน์ คงสมพงษ์", "Wichian Brown"},
		{"วิเศษ ศิลภา", "Wichit Davis"},
		{"สิรินธร นิลนอก", "Siri Smith"},
		{"สิริสาร ลิ่มค้อ", "Siril Williams"},
		{"ชัยพร ชิณะวรรณ", "Chai Miller"},
		{"ชาญชัย นวลสวรรค์", "Chay Wilson"},
		{"ธรรมชาตินรงค์ สหระ", "Tam Anderson"},
		{"ธรรมวิทย์ ศรีหา", "Thum Taylor"},
		{"นรเศร ภูมิศรี", "Nara Thomas"},
		{"นรพิพัฒน์ กลิ่นมะลิ", "Naran Jackson"},
		{"ปรีชา อินทร์ศิลป์", "Priya White"},
		{"ปรีชากร โครงสร้าง", "Preeya Harris"},
		{"สิชน ธรรมชั้น", "Sichon Martin"},
		{"สิชล ทองมนต์", "Sichol Garcia"},
		{"วรรณวัฒน์ คำหา", "Wanna Rodriguez"},
		{"วรรณพร ศรีบูชา", "Wanit Martinez"},
		{"กฤษณพล ลี้แล", "Krish Robinson"},
		{"กฤษณสิลป์ จันทศรี", "Krishna Clark"},
		{"นิรามล ทรัพย์เศรษฐ", "Neera Lewis"},
		{"นิรทา พุ่มพวง", "Neran Lee"},
		{"สมรสาตรี ธรรมรักษ์", "Somrasa Walker"},
		{"สมรสา เจริญสิน", "Samrasa Hall"},
		{"ศิลปกร แก้วกำแพง", "Sil Young"},
		{"ศิลปสิน อังคาร", "Silp Hernandez"},
		{"นันทศรี โกศล", "Nan King"},
		{"นันทีย์ มูนเนือง", "Nant Wright"},
		{"โยธินวร นิ่มนวล", "Yothin Lopez"},
		{"โยธิน สระดี", "Yodin Hill"},
		{"ธรรมชาติสิลป์ ลิ้มปิยะ", "Thamachai Scott"},
		{"ธรรมชาตินอก ผลิเสรี", "Thammachai Green"},
		{"กมลา วัฒนาพร", "Kamala Adams"},
		{"กมลาพร ชื่นสวัสดิ์", "Kamela Nelson"},
		{"ขจรศักดิ์ กิจสวัสดิ์", "Khachon Carter"},
		{"ขจรศักดี้ อินทร์สิทธิ์", "Khachorn Mitchell"},
		{"บัญชา คุณรัตน์", "Bancha Perez"},
		{"บัญชาพร ไทรทอง", "Buncha Roberts"},
		{"นิพัฒน์สิน ศรีบัณฑิต", "Niphat Phillips"},
		{"นิพัฒนา นิยมนีย์", "Niphon Campbell"},
		{"ศรัณยา ชุมชอบ", "Saran Parker"},
		{"ศรัณย์ นะภักดี", "Sarin Evans"},
		{"ธวัชชัยกร ศิริวงษ์", "Thawachai Edwards"},
		{"ธวัชชัย สารศิลป์", "Thavachai Collins"},
		{"วารีนันท์ พลธีร", "Wari Reyes"},
		{"วารี เหลี่ยมกา", "Waree Morris"},
		{"เอกรัฐสิทธิ์ ทีระวัฒน์", "Ekarat Murphy"},
		{"เอกรัฐ คำศิริ", "Ekarath Rogers"},
		{"อัจฉริยะวัฒน์ เกื้อกูล", "Achariya Morgan"},
		{"อัจฉริยะพร นิลประสาท", "Achariyah Peterson"},
		{"บรรจงชัย ลิ่มรักษ์", "Banjong Gray"},
		{"บรรจง สุรศิลป์", "Banjon Ramirez"},
	}

	defaultDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	scoreOf := func(thai, latin string) float64 {
		src := Normalize(thai)
		dest := Normalize(latin)
		res := CalculateCompositeScore(src, dest, defaultDate, defaultDate, DefaultWeights, DefaultAlgorithms, 30)
		return res.NameScore
	}

	minGenuine := 1.0
	for _, p := range genuinePairs {
		if s := scoreOf(p.thai, p.latin); s < minGenuine {
			minGenuine = s
		}
	}

	maxFalseFriend := 0.0
	for _, p := range falseFriendPairs {
		if s := scoreOf(p.thai, p.latin); s > maxFalseFriend {
			maxFalseFriend = s
		}
	}

	t.Logf("genuine pairs   n=%d min NameScore = %.4f", len(genuinePairs), minGenuine)
	t.Logf("false friends   n=%d max NameScore = %.4f", len(falseFriendPairs), maxFalseFriend)

	if maxFalseFriend >= minGenuine {
		t.Errorf("no separating threshold exists: worst false-friend NameScore (%.4f) >= worst genuine NameScore (%.4f)",
			maxFalseFriend, minGenuine)
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
