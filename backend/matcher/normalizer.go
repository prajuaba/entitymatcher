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

	// thaiPersonalPrefixes are honorifics stripped when they are glued to the name
	// with no space ("นางสาวกัลยา"). ORDER MATTERS: entries must be longest-first,
	// because stripping stops at the first match. "นาง" before "นางสาว" would turn
	// "นางสาวกัลยา" into "สาวกัลยา" and never match the equivalent "น.ส.กัลยา".
	thaiPersonalPrefixes = []string{
		"ว่าที่ร้อยตรี", "ว่าที่ร.ต.",
		"เด็กชาย", "เด็กหญิง",
		"นางสาว", "น.ส.", "น.ส", "นส.",
		"ด.ช.", "ด.ญ.", "ดช.", "ดญ.",
		"หม่อมหลวง", "หม่อมราชวงศ์", "ม.ล.", "ม.ร.ว.",
		"นาย", "นาง", "คุณ",
		"ดร.", "นพ.", "พญ.", "ศ.", "รศ.", "ผศ.",
	}

	// thaiCorporateSuffixes are legal-form suffixes, also longest-first so
	// "ห้างหุ้นส่วนจำกัด" is not shortened to "ห้างหุ้นส่วน" by the "จำกัด" entry.
	thaiCorporateSuffixes = []string{
		"จำกัด(มหาชน)", "จำกัดมหาชน", "ห้างหุ้นส่วนจำกัด", "ห้างหุ้นส่วนสามัญ",
		"จำกัด", "บมจ", "บจก", "หจก",
	}

	// thaiCorporatePrefixes are legal-form prefixes, longest-first.
	thaiCorporatePrefixes = []string{
		"ห้างหุ้นส่วนจำกัด", "ห้างหุ้นส่วนสามัญ", "บริษัท", "บมจ.", "บจก.", "หจก.", "บมจ", "บจก", "หจก",
	}

	// thaiAbbrevExpansions rewrite dotted Thai honorific and legal-form abbreviations
	// to their full spellings BEFORE punctuation is stripped. reCleanChars replaces
	// "." with a space, which shatters "น.ส." into the fragments "น ส" that no title
	// rule can recognise -- that is why "น.ส.กัลยา" and "นางสาวกัลยา" (the same
	// person, same title) scored 0.81 instead of matching. Ordered longest-first so
	// a shorter entry cannot consume the prefix of a longer one.
	thaiAbbrevExpansions = []struct{ from, to string }{
		{"ว่าที่ร.ต.", "ว่าที่ร้อยตรี"},
		{"ม.ร.ว.", "หม่อมราชวงศ์"},
		{"ม.ล.", "หม่อมหลวง"},
		{"น.ส.", "นางสาว"},
		{"นส.", "นางสาว"},
		{"ด.ช.", "เด็กชาย"},
		{"ด.ญ.", "เด็กหญิง"},
		{"บมจ.", "บริษัท"},
		{"บจก.", "บริษัท"},
		{"หจก.", "ห้างหุ้นส่วนจำกัด"},
	}

	// Generic high-frequency corporate words that should carry lower matching weight
	genericWords = map[string]bool{
		"เทคโนโลยี": true, "นวัตกรรม": true, "อินโนเวชั่น": true, "กรุ๊ป": true, "โซลูชั่น": true, "การค้า": true, "บริการ": true, "ไทย": true,
		"technology": true, "technologies": true, "innovation": true, "innovations": true, "solutions": true, "service": true,
		"services": true, "group": true, "holding": true, "holdings": true, "global": true, "trading": true, "enterprise": true,
		"enterprises": true, "international": true, "system": true, "systems": true, "digital": true,
		"มหาชน": true, "ประเทศไทย": true, "ไทยแลนด์": true, "thailand": true,
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

	// reTrailingRefCode matches a record-keeping reference code glued to the end of
	// a name: "...รัตแมดSB63000164", "...ชูชีพ PL64000306", "...ภาษิต0002019000592".
	// Two shapes occur: letters followed by digits, or a long run of bare digits.
	// The bare-digit arm requires 8+ digits so that a number which is genuinely part
	// of a name survives -- "บจก.มาทวีอินเตอร์กรุ๊ป 2006" keeps its 2006.
	reTrailingRefCode = regexp.MustCompile(`(?:[A-Za-z]{2,}[0-9]{5,}|[0-9]{8,})\s*$`)

	// reBranchAnnotation matches a parenthetical that is branch/office metadata
	// rather than a party name -- "(สาขาที่ 1)", "(สาขา 5)", "(2)", "(branch 3)".
	// Treating these as parties lets two different branches of one company match
	// on their identical company name alone.
	reBranchAnnotation = regexp.MustCompile(`^\s*(?i:สาขาที่|สาขา|branch|br\.?|no\.?|#)?\s*[0-9]+\s*$`)
)

// StripTrailingRefCode removes a trailing record-keeping reference code. It is
// applied to every name, not only to ones that split into several parties: the
// overwhelming majority of records carry no separator at all, and for those the
// code used to survive into scoring and cost roughly 0.06-0.08 of similarity on
// an otherwise exact match.
func StripTrailingRefCode(s string) string {
	stripped := strings.TrimSpace(reTrailingRefCode.ReplaceAllString(s, ""))
	// Never return empty: a name that is nothing but a code is better compared as
	// itself than as the empty string.
	if stripped == "" {
		return strings.TrimSpace(s)
	}
	return stripped
}

type CleanName struct {
	Raw               string   `json:"raw"`
	Cleaned           string   `json:"cleaned"`
	SortedTokens      string   `json:"sorted_tokens"`
	Tokens            []string `json:"tokens"`
	DistinctiveTokens []string `json:"distinctive_tokens"`
	Numbers           []string `json:"numbers"`
	PhoneticKey       string   `json:"phonetic_key"`
	PhoneticForm      string   `json:"phonetic_form"`
	Romanized         string   `json:"romanized"`
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

	// Expand dotted abbreviations while the dots still exist (see thaiAbbrevExpansions).
	for _, e := range thaiAbbrevExpansions {
		lower = strings.ReplaceAll(lower, e.from, e.to)
	}

	// Drop a trailing reference code before anything else reads the text, so it
	// neither dilutes the similarity nor pollutes Numbers.
	lower = StripTrailingRefCode(lower)

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
		SortedTokens:      sortedTokensStr,
		Tokens:            tokens,
		DistinctiveTokens: distinctive,
		Numbers:           numMatches,
		PhoneticKey:       GeneratePhoneticKey(phoneticForm, text),
		PhoneticForm:      phoneticForm,
		Romanized:         romanized,
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

		// Strip corporate prefixes as prefix only (บริษัท, ห้างหุ้นส่วนจำกัด, ...), longest-first.
		strippedCorpPrefix := false
		for _, pref := range thaiCorporatePrefixes {
			if strings.HasPrefix(tok, pref) && len(tok) > len(pref) {
				after := strings.TrimPrefix(tok, pref)
				if len([]rune(after)) >= 2 {
					tokens[i] = after
					strippedCorpPrefix = true
					break
				}
			}
		}
		if strippedCorpPrefix {
			continue
		}

		// Strip corporate suffixes as suffix only (จำกัด, บจก, บมจ, หจก, ...), longest-first.
		for _, suf := range thaiCorporateSuffixes {
			if strings.HasSuffix(tok, suf) && len(tok) > len(suf) {
				before := strings.TrimSuffix(tok, suf)
				if len([]rune(before)) >= 2 {
					tokens[i] = before
					break
				}
			}
		}

		// Strip personal titles: only as prefix, and not if result would be just a single Thai character.
		// thaiPersonalPrefixes is ordered longest-first so "นางสาว" is tried before "นาง".
		for _, pref := range thaiPersonalPrefixes {
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

// propagateSharedSurname applies the Thai convention that a comma- or
// "และ"-separated list of people carries the surname only once, after the last
// name: "นายคณิน,นายธรรมากร,น.ส.กฤตินี ทองบุญ" is three people all surnamed
// ทองบุญ. Without this, the earlier parties reduce to bare given names, which
// match any unrelated person sharing that given name.
//
// Only the LAST fragment in the list is treated as carrying the shared surname
// (its final whitespace-separated token). Any earlier fragment that already has
// two or more tokens is assumed to carry its own surname and is left untouched.
// This must be called per-candidate (i.e. per one comma/และ list), never across
// unrelated parenthetical groups, since those are independent parties.
func propagateSharedSurname(fragments []string) []string {
	if len(fragments) < 2 {
		return fragments
	}

	lastIdx := len(fragments) - 1
	lastTokens := strings.Fields(fragments[lastIdx])
	if len(lastTokens) < 2 {
		// No surname to share.
		return fragments
	}
	surname := lastTokens[len(lastTokens)-1]

	result := make([]string, len(fragments))
	copy(result, fragments)
	for i := 0; i < lastIdx; i++ {
		tokens := strings.Fields(fragments[i])
		if len(tokens) == 1 {
			result[i] = fragments[i] + " " + surname
		}
	}
	return result
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

// isIdentifyingParty reports whether a fragment carries enough content to name a
// party. A parenthetical often holds a qualifier rather than a name -- "(มหาชน)"
// (Public), "(ประเทศไทย)" (Thailand), "(สาขาที่ 3)" (branch 3) -- and treating
// those as parties makes every company sharing a qualifier match every other at
// 1.0: "บริษัท เสนาดีเวลลอปเม้นท์ จำกัด (มหาชน)" matched an unrelated
// "บริษัท ทีฆทัศน์ ดีเวลลอปเมนท์ จำกัด (มหาชน)" on the word "มหาชน" alone.
//
// A fragment qualifies only if, once normalised, it retains at least one
// distinctive (non-generic) token, and is not a lone very short token that
// cannot identify an entity on its own.
func isIdentifyingParty(fragment string) bool {
	n := Normalize(fragment)
	if len(n.DistinctiveTokens) == 0 {
		return false
	}
	if len(n.Tokens) == 1 && len([]rune(n.Tokens[0])) <= 3 {
		return false
	}
	return true
}

// SplitParties breaks a raw name field into the individual parties it names.
//
// A single field routinely carries more than one party: a customer who changed
// their name, or co-borrowers. They appear inside parentheses, or separated by
// commas or "และ". Scoring the whole concatenated string against a single name
// dilutes the similarity below any useful threshold, so callers score each
// party separately and keep the best pairing.
//
// The result always has at least one element. A name with no separators yields
// exactly one party, so single-party records take an unchanged code path.
func SplitParties(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	trimmed = StripTrailingRefCode(trimmed)

	// Step 2: Scan for parenthetical groups
	var candidates []string

	// Track depth and extract parenthetical content
	var depth int
	var start int
	var inParen bool
	// Handle both ASCII and full-width parentheses
	for i, r := range trimmed {
		switch r {
		case '(', '（':
			if !inParen {
				start = i
				inParen = true
				depth = 1
			} else {
				depth++
			}
		case ')', '）':
			if inParen {
				depth--
				if depth == 0 {
					// Found a complete parenthetical group
					// Use byte slice to preserve Thai characters correctly
					content := trimmed[start+1 : i]
					if !reBranchAnnotation.MatchString(strings.TrimSpace(content)) && isIdentifyingParty(strings.TrimSpace(content)) {
						candidates = append(candidates, content)
					}
					inParen = false
				}
			}
		default:
			if inParen {
				// We're inside a parenthesis, so accumulate content
			}
		}
	}

	// If we had unmatched opening parentheses, treat the whole string as one candidate
	var unbalanced bool
	if inParen {
		candidates = candidates[:0] // Reset if unbalanced
		candidates = append(candidates, trimmed)
		unbalanced = true
	}

	// Collect all text outside parentheses
	var nonParenContent strings.Builder
	depth = 0
	inParen = false
	for _, r := range trimmed {
		switch r {
		case '(', '（':
			if !inParen {
				inParen = true
				depth = 1
			} else {
				depth++
			}
		case ')', '）':
			if inParen {
				depth--
				if depth == 0 {
					inParen = false
				}
			}
		default:
			if !inParen {
				nonParenContent.WriteRune(r)
			}
		}
	}
	if !unbalanced && nonParenContent.Len() > 0 {
		candidates = append(candidates, nonParenContent.String())
	}

	// If no candidates were collected due to unbalanced parentheses or other issue,
	// fall back to using the entire string as a single candidate
	if len(candidates) == 0 {
		candidates = []string{trimmed}
	}

	// Step 3: Split each candidate on comma, "และ", "&", and "/" in sequence
	var fragments []string
	for _, cand := range candidates {
		// Helper function to split a slice of strings by a delimiter and flatten
		splitAndFlatten := func(input []string, sep string) []string {
			var result []string
			for _, s := range input {
				parts := strings.Split(s, sep)
				result = append(result, parts...)
			}
			return result
		}

		// Chain the splits in order: ",", "และ", "&", "/"
		current := []string{cand}
		current = splitAndFlatten(current, ",")
		current = splitAndFlatten(current, "และ")
		current = splitAndFlatten(current, "&")
		current = splitAndFlatten(current, "/")

		// Propagate a surname shared across this one candidate's fragments
		// (see propagateSharedSurname doc comment). Scoped per-candidate so
		// it never bleeds across an unrelated parenthetical group.
		current = propagateSharedSurname(current)

		fragments = append(fragments, current...)
	}

	// Step 4: trim whitespace, drop short runes, drop duplicates
	var deduped []string
	seen := make(map[string]bool)
	for _, frag := range fragments {
		trimmedFrag := strings.Trim(frag, " \t\n\r()") // also remove stray parentheses
		if len([]rune(trimmedFrag)) < 2 {
			continue
		}
		if seen[trimmedFrag] {
			continue
		}
		seen[trimmedFrag] = true
		deduped = append(deduped, trimmedFrag)
	}

	// Step 5: Return if non-empty, else fallback to original string
	if len(deduped) > 0 {
		return deduped
	}
	return []string{strings.TrimSpace(raw)}
}
