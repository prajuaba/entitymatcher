package matcher

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	// Thai & English corporate and personal titles for stripping
	corporateTitles = []string{
		"บริษัท", "บจก.", "บจก", "บมจ.", "บมจ", "ห้างหุ้นส่วนจำกัด", "หจก.", "หจก", "จำกัด (มหาชน)", "จำกัด",
		"public company limited", "company limited", "co., ltd.", "co.,ltd.", "co. ltd", "co ltd",
		"corp.", "corp", "corporation", "inc.", "inc", "incorporated", "llc", "plc", "ltd.", "ltd",
		"สาขาที่", "สาขาใหญ่", "head office", "branch",
	}

	personalTitles = []string{
		"นางสาว", "นาย", "นาง", "น.ส.", "ด.ช.", "ด.ญ.", "คุณ", "นพ.", "พญ.", "ดร.",
		"ศาสตราจารย์", "ศ.", "รองศาสตราจารย์", "รศ.", "ผู้ช่วยศาสตราจารย์", "ผศ.",
		"พล.ต.", "พล.อ.", "พันตรี", "พ.ต.อ.", "พ.ต.ท.", "พ.ต.ต.",
		"mr.", "mr", "mrs.", "mrs", "ms.", "ms", "miss", "dr.", "dr", "prof.", "prof", "ph.d.", "phd", "m.d.", "md",
	}

	// Generic high-frequency corporate words that should carry lower matching weight
	genericWords = map[string]bool{
		"เทคโนโลยี": true, "นวัตกรรม": true, "อินโนเวชั่น": true, "กรุ๊ป": true, "โซลูชั่น": true, "การค้า": true, "บริการ": true, "ไทย": true,
		"technology": true, "technologies": true, "innovation": true, "innovations": true, "solutions": true, "service": true,
		"services": true, "group": true, "holding": true, "holdings": true, "global": true, "trading": true, "enterprise": true,
		"enterprises": true, "international": true, "system": true, "systems": true, "digital": true,
	}

	// Known Bilingual Transliteration Mappings (Thai <-> English)
	bilingualTransliterations = map[string]string{
		"สยาม": "siam", "siam": "สยาม",
		"กรุงเทพ": "bangkok", "bangkok": "กรุงเทพ",
		"กสิกรไทย": "kasikornbank", "kasikornbank": "กสิกรไทย", "กสิกร": "kasikorn", "kasikorn": "กสิกร",
		"เอไอเอส": "ais", "ais": "เอไอเอส",
		"เจริญโภคภัณฑ์": "charoen pokphand", "charoen pokphand": "เจริญโภคภัณฑ์", "ซีพี": "cp", "cp": "ซีพี",
		"สมชาย": "somchai", "somchai": "สมชาย",
		"อารียา": "areeya", "areeya": "อารียา",
		"วีระชัย": "weerachai", "weerachai": "วีระชัย",
		"พงษ์สวัสดิ์": "pongsawat", "pongsawat": "พงษ์สวัสดิ์",
	}

	reThaiCorporate  = regexp.MustCompile(`(?i)(บริษัท|บจก|บมจ|ห้างหุ้นส่วนจำกัด|หจก|จำกัด)`)
	reEngCorporate   = regexp.MustCompile(`(?i)(company|co\.|ltd|corp|inc|llc|plc)`)
	reCleanChars     = regexp.MustCompile(`[^a-zA-Z0-9\x{0E00}-\x{0E7F}\s]`)
	reThaiDiacritics = regexp.MustCompile(`[\x{0E31}\x{0E34}-\x{0E3A}\x{0E47}-\x{0E4C}]`)
	reMultiSpace     = regexp.MustCompile(`\s+`)
	reNumbers        = regexp.MustCompile(`\b\d+\b`)
)

type CleanName struct {
	Raw              string   `json:"raw"`
	Cleaned          string   `json:"cleaned"`
	SortedTokens     string   `json:"sorted_tokens"`
	Tokens           []string `json:"tokens"`
	DistinctiveTokens []string `json:"distinctive_tokens"`
	Numbers          []string `json:"numbers"`
	PhoneticKey      string   `json:"phonetic_key"`
	PhoneticForm     string   `json:"phonetic_form"`
	Romanized        string   `json:"romanized"`
}

func RunePrefix(s string, n int) string {
	runes := []rune(s)
	if n < len(runes) {
		return string(runes[:n])
	}
	return s
}

func isThai(r rune) bool {
	return r >= '฀' && r <= '๿'
}

func isThaiVowel(r rune) bool {
	switch r {
	case 'ะ', 'า', 'ำ', // U+0E30, U+0E32, U+0E33
		'เ', 'แ', 'โ', 'ใ', 'ไ', 'ๅ', 'ๆ': // U+0E40-U+0E46
		return true
	default:
		return false
	}
}

// Normalize applies NFC normalization, strips titles/honorifics, cleans special chars,
// extracts numbers, isolates distinctive tokens, and sorts tokens.
func Normalize(input string) CleanName {
	normText := norm.NFC.String(input)
	normText = strings.TrimSpace(normText)
	lower := strings.ToLower(normText)

	// Extract numbers before stripping
	numMatches := reNumbers.FindAllString(lower, -1)

	// Strip non-alphanumeric symbols
	text := reCleanChars.ReplaceAllString(lower, " ")
	text = reMultiSpace.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// Fallback if cleaning stripped everything
	if text == "" {
		text = strings.TrimSpace(reMultiSpace.ReplaceAllString(reCleanChars.ReplaceAllString(strings.ToLower(normText), " "), " "))
	}

	// Tokenize
	tokens := strings.Fields(text)

	// Strip corporate and personal titles using token-based approach (CRITICAL: C1)
	tokens = stripTitlesFromTokens(tokens)

	// Rejoin tokens to apply synonym replacement (C2)
	rejoined := strings.Join(tokens, " ")

	// Apply synonym replacement (C2)
	rejoined = ReplaceSynonymsInText(rejoined)

	// Re-tokenize after synonym replacement (since canonical forms may be multi-word)
	tokens = strings.Fields(rejoined)

	// Recompute text after all processing
	text = strings.Join(tokens, " ")

	// Filter out generic words for distinctive tokens
	var distinctive []string
	for _, tok := range tokens {
		if !genericWords[tok] {
			distinctive = append(distinctive, tok)
		}
	}

	// Sort tokens
	sortedTokens := make([]string, len(tokens))
	copy(sortedTokens, tokens)
	sort.Strings(sortedTokens)
	sortedTokensStr := strings.Join(sortedTokens, " ")

	// Only apply Thai phonetic normalization to strings containing Thai script
	var phoneticForm string
	if ContainsThai(text) {
		phoneticForm = ThaiPhoneticForm(text)
	}

	// Compute romanized form for cross-script retrieval (Stage 2)
	// For Thai-containing text, romanize and extract skeleton
	// For pure-Latin text, extract skeleton directly
	var romanized string
	if ContainsThai(text) {
		romanized = PhoneticSkeleton(RomanizeThai(text))
	} else {
		romanized = PhoneticSkeleton(text)
	}

	return CleanName{
		Raw:               input,
		Cleaned:           text,
		SortedTokens:     sortedTokensStr,
		Tokens:           tokens,
		DistinctiveTokens: distinctive,
		Numbers:          numMatches,
		PhoneticKey:      GeneratePhoneticKey(phoneticForm, text),
		PhoneticForm:     phoneticForm,
		Romanized:        romanized,
	}
}

func stripTitlesFromTokens(tokens []string) []string {
	if len(tokens) == 0 {
		return tokens
	}

	// Collect tokens to remove by index
	removeIndices := make(map[int]bool)

	// Process multi-word titles longest-first
	titleList := make([]string, 0, len(corporateTitles)+len(personalTitles))
	titleList = append(titleList, corporateTitles...)
	titleList = append(titleList, personalTitles...)

	// Sort by length descending to handle multi-word titles first
	sort.Slice(titleList, func(i, j int) bool {
		return len(titleList[i]) > len(titleList[j])
	})

	// For multi-word titles: try to match consecutive token sequences
	for i := 0; i < len(tokens); i++ {
		for length := 1; length <= len(titleList); length++ {
			if i+length > len(tokens) {
				break
			}
			// Build candidate phrase from tokens
			candidate := strings.Join(tokens[i:i+length], " ")
			candLower := strings.ToLower(candidate)
			for _, title := range titleList {
				if strings.ToLower(title) == candLower {
					// Mark tokens for removal
					for j := i; j < i+length; j++ {
						removeIndices[j] = true
					}
					break
				}
			}
		}
	}

	// For single tokens, also consider Thai-specific prefix/suffix stripping (C1.4)
	for i, tok := range tokens {
		if removeIndices[i] {
			continue
		}

		// Only apply if it's a Thai token
		if !isThaiToken(tok) {
			continue
		}

		// Strip corporate prefixes as prefix only (บริษัท)
		if strings.HasPrefix(tok, "บริษัท") && len(tok) > len("บริษัท") {
			after := strings.TrimPrefix(tok, "บริษัท")
			if len([]rune(after)) >= 2 {
				tokens[i] = after
				continue
			}
		}

		// Strip corporate suffixes as suffix only (จำกัด, บจก, บมจ, หจก)
		corpSuffixes := []string{"จำกัด", "บจก", "บมจ", "หจก"}
		for _, suf := range corpSuffixes {
			if strings.HasSuffix(tok, suf) && len(tok) > len(suf) {
				before := strings.TrimSuffix(tok, suf)
				if len([]rune(before)) >= 2 {
					tokens[i] = before
					break
				}
			}
		}

		// Strip personal titles: only as prefix, and not if result would be just a single Thai character
		personalPrefixes := []string{"นาย", "นาง", "น.ส.", "ด.ช.", "ด.ญ.", "คุณ"}
		for _, pref := range personalPrefixes {
			if strings.HasPrefix(tok, pref) && len(tok) > len(pref) {
				after := strings.TrimPrefix(tok, pref)
				if len([]rune(after)) >= 2 {
					tokens[i] = after
					break
				}
			}
		}
	}

	// Remove marked tokens
	result := make([]string, 0, len(tokens))
	for i, tok := range tokens {
		if !removeIndices[i] {
			result = append(result, tok)
		}
	}

	return result
}

func isThaiToken(s string) bool {
	for _, r := range s {
		if !isThai(r) && !unicode.IsSpace(r) && !unicode.IsPunct(r) {
			return false
		}
	}
	return true
}

func StripToneMarks(s string) string {
	return reThaiDiacritics.ReplaceAllString(s, "")
}

// GeneratePhoneticKey creates a consonant skeleton of the input string.
// If phoneticForm is provided (non-empty), it derives from the phonetic form (Thai only).
// Otherwise, it falls back to the original logic for pure-Latin strings.
func GeneratePhoneticKey(phoneticForm, originalText string) string {
	// Use phonetic form if available (Thai content only)
	var s string
	if phoneticForm != "" {
		s = phoneticForm
	} else {
		s = originalText
	}

	// Strip tone marks and Thai standalone vowels
	sClean := StripToneMarks(s)

	// Define Thai vowels to strip: U+0E30, U+0E32, U+0E33, U+0E40-U+0E46, U+0E45
	reThaiVowels := regexp.MustCompile(`[\x{0E30}\x{0E32}\x{0E33}\x{0E40}-\x{0E46}\x{0E45}]`)
	sClean = reThaiVowels.ReplaceAllString(sClean, "")

	// Convert to rune slice
	runes := []rune(sClean)
	result := []rune{}

	// Process each rune
	leadingVowel := true
	for _, r := range runes {
		// Drop whitespace and non-letter/digit
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}

		// Handle English vowels
		if !isThai(r) {
			lowR := unicode.ToLower(r)
			isVowel := lowR == 'a' || lowR == 'e' || lowR == 'i' || lowR == 'o' || lowR == 'u'
			// Keep leading vowel; drop others
			if isVowel {
				if leadingVowel {
					result = append(result, r)
				}
				// else: drop non-leading vowels
			} else {
				result = append(result, r)
			}
			leadingVowel = false
		} else {
			// Thai consonant/digit/letter: keep if it's not a vowel (already stripped)
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				result = append(result, r)
				leadingVowel = false
			}
		}
	}

	// Truncate to 8 runes
	return RunePrefix(string(result), 8)
}

// CheckBilingualMatch checks if Thai and English tokens have a mapped transliteration match.
func CheckBilingualMatch(tok1, tok2 string) bool {
	t1 := strings.ToLower(tok1)
	t2 := strings.ToLower(tok2)
	if val, ok := bilingualTransliterations[t1]; ok && (val == t2 || strings.Contains(val, t2) || strings.Contains(t2, val)) {
		return true
	}
	if val, ok := bilingualTransliterations[t2]; ok && (val == t1 || strings.Contains(val, t1) || strings.Contains(t1, val)) {
		return true
	}
	return false
}
