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
}

// Normalize applies NFC normalization, strips titles/honorifics, cleans special chars,
// extracts numbers, isolates distinctive tokens, and sorts tokens.
func Normalize(input string) CleanName {
	normText := norm.NFC.String(input)
	normText = strings.TrimSpace(normText)
	lower := strings.ToLower(normText)

	// Extract numbers before stripping
	numMatches := reNumbers.FindAllString(lower, -1)

	// Strip corporate and personal titles case-insensitively
	for _, title := range corporateTitles {
		tLower := strings.ToLower(title)
		lower = strings.ReplaceAll(lower, tLower, " ")
	}
	for _, title := range personalTitles {
		tLower := strings.ToLower(title)
		lower = strings.ReplaceAll(lower, tLower, " ")
	}

	// Strip non-alphanumeric symbols
	text := reCleanChars.ReplaceAllString(lower, " ")
	text = reMultiSpace.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// Fallback if cleaning stripped everything
	if text == "" {
		text = strings.TrimSpace(reMultiSpace.ReplaceAllString(reCleanChars.ReplaceAllString(strings.ToLower(normText), " "), " "))
	}

	// Tokenize and sort
	rawTokens := strings.Fields(text)
	tokens := make([]string, len(rawTokens))
	copy(tokens, rawTokens)

	var distinctive []string
	for _, tok := range rawTokens {
		if !genericWords[tok] {
			distinctive = append(distinctive, tok)
		}
	}

	sort.Strings(tokens)
	sortedTokens := strings.Join(tokens, " ")

	return CleanName{
		Raw:               input,
		Cleaned:           text,
		SortedTokens:     sortedTokens,
		Tokens:           rawTokens,
		DistinctiveTokens: distinctive,
		Numbers:          numMatches,
		PhoneticKey:      GeneratePhoneticKey(text),
	}
}

func StripToneMarks(s string) string {
	return reThaiDiacritics.ReplaceAllString(s, "")
}

func GeneratePhoneticKey(s string) string {
	sClean := StripToneMarks(s)
	var consonants []rune
	for _, r := range sClean {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			consonants = append(consonants, r)
		}
	}
	if len(consonants) > 8 {
		return string(consonants[:8])
	}
	return string(consonants)
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
