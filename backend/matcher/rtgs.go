package matcher

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// RomanizeThai converts Thai text to its RTGS (Royal Thai General System) romanization.
// Per syllable, emits initial + vowel + final using RTGS mapping tables.
// Non-Thai runs (Latin, digits, punctuation) are passed through unchanged.
func RomanizeThai(s string) string {
	if s == "" {
		return ""
	}

	syllables := SegmentThaiSyllables(s)
	var result []string

	for _, syl := range syllables {
		// If not Thai (no Initial/Final/Vowel, just Text), pass through
		if syl.Initial == "" && syl.Final == "" && syl.Vowel == "" {
			result = append(result, strings.ToLower(syl.Text))
			continue
		}

		romanized := romanizeSyllable(syl)
		result = append(result, romanized)
	}

	return strings.Join(result, "")
}

func romanizeSyllable(syl Syllable) string {
	var sb strings.Builder

	// Romanize initial consonant (if not อ)
	if syl.Initial != "" && syl.Initial != "อ" {
		sb.WriteString(romanizeInitial(syl.Initial))
	}

	// Romanize vowel
	// Implicit vowel rule (task-documented approximation): when no vowel is written,
	// a syllable that closes on its own final consonant is pronounced with an
	// inherent "o" (e.g. กร with no written vowel and final ร -> "kon"-like); a
	// syllable with no final of its own (it bridges into the next consonant, e.g.
	// the ธ in ธนากร) is pronounced with inherent "a". This mirrors real Thai
	// orthography (cf. ถนน "thanon": first syllable ถ -> "tha", second syllable
	// นน -> "non").
	var vowel string
	if syl.Vowel == "" {
		if syl.Final != "" {
			vowel = "o"
		} else {
			vowel = "a"
		}
	} else {
		vowel = romanizeVowel(syl.Vowel)
	}
	sb.WriteString(vowel)

	// Romanize final consonant
	if syl.Final != "" {
		sb.WriteString(romanizeFinal(syl.Final))
	}

	return strings.ToLower(sb.String())
}

// romanizeInitial maps Thai initial consonants to RTGS romanization
func romanizeInitial(initial string) string {
	switch initial {
	// ก-class
	case "ก":
		return "k"

	// ข-class (aspirated k)
	case "ข", "ฃ", "ค", "ฅ", "ฆ":
		return "kh"

	// ง-class
	case "ง":
		return "ng"

	// จ-class
	case "จ":
		return "ch"

	// ฉ-class (aspirated ch)
	case "ฉ", "ช", "ฌ":
		return "ch"

	// ซ-class (s sound)
	case "ซ", "ศ", "ษ", "ส":
		return "s"

	// ญ-class (y sound)
	case "ญ", "ย":
		return "y"

	// ด-class (d sound)
	case "ฎ", "ด":
		return "d"

	// ต-class (t sound)
	case "ฏ", "ต":
		return "t"

	// ถ-class (aspirated t)
	case "ฐ", "ฑ", "ฒ", "ถ", "ท", "ธ":
		return "th"

	// น-class (n sound)
	case "ณ", "น":
		return "n"

	// บ-class
	case "บ":
		return "b"

	// ป-class
	case "ป":
		return "p"

	// ผ-class (aspirated p)
	case "ผ", "พ", "ภ":
		return "ph"

	// ฝ-class (f sound)
	case "ฝ", "ฟ":
		return "f"

	// ม-class
	case "ม":
		return "m"

	// ร-class (r sound)
	case "ร":
		return "r"

	// ล-class (l sound)
	case "ล", "ฬ":
		return "l"

	// ว-class
	case "ว":
		return "w"

	// ห-class (h sound)
	case "ห", "ฮ":
		return "h"

	// ฤ/ฦ are vowel-consonants: RTGS romanizes them as rue/lue. Without these
	// they fall to default, which is what exposed the recursion bug above.
	case "ฤ":
		return "rue"
	case "ฦ":
		return "lue"

	// อ-class (glottal stop, handled in romanizeSyllable)
	case "อ":
		return ""

	// If initial is a cluster, recursively handle it
	// (clusters should already be in the initial position from segmenter)
	default:
		// Try to handle clusters by mapping component consonants.
		//
		// This MUST count runes, not bytes: a Thai consonant is 3 bytes in UTF-8,
		// so a byte-length check treats a single unmapped character (ฤ, ฦ) as a
		// cluster, decodes it to one rune, and recurses on the identical string
		// until the stack is exhausted and the process dies.
		if utf8.RuneCountInString(initial) > 1 {
			// For clusters like "กร", map each component
			var result string
			for _, r := range initial {
				result += romanizeInitial(string(r))
			}
			return result
		}
		return ""
	}
}

// romanizeVowel maps Thai vowels to RTGS romanization for WRITTEN vowels.
// The implicit-vowel case (vowel == "") is handled by the caller (romanizeSyllable),
// which needs Syllable.Final to choose between the "a" (open, bridges to next
// consonant) and "o" (closed, own final) approximations. This function should
// never be called with an empty vowel string.
func romanizeVowel(vowel string) string {
	// Check for compound vowels first (must be checked before component vowels)
	// Compound patterns: เ-ีย (ia), เ-ือ (uea), ั-ว (ua)
	if strings.Contains(vowel, "เ") && strings.Contains(vowel, "ย") && strings.Contains(vowel, "ี") {
		// Pattern: เ + ี + ย -> ia
		return "ia"
	}
	if strings.Contains(vowel, "เ") && (strings.Contains(vowel, "ือ") || (strings.Contains(vowel, "ื") && strings.Contains(vowel, "อ"))) {
		// Pattern: เ + ือ -> uea (or เ + ื + อ)
		return "uea"
	}
	if strings.Contains(vowel, "ั") && strings.Contains(vowel, "ว") {
		// Pattern: ั + ว -> ua
		return "ua"
	}

	// Single or simple vowels - scan through and map by character
	for _, ch := range vowel {
		switch ch {
		case 'ะ', 'ั': // U+0E30, U+0E31
			return "a"
		case 'า': // U+0E32
			return "a"
		case 'ิ', 'ี': // U+0E34, U+0E35
			return "i"
		case 'ึ', 'ื': // U+0E36, U+0E37
			return "ue"
		case 'ุ', 'ู': // U+0E38, U+0E39
			return "u"
		case 'เ': // U+0E40
			return "e"
		case 'แ': // U+0E41
			return "ae"
		case 'โ': // U+0E42
			return "o"
		case 'ใ', 'ไ': // U+0E43, U+0E44
			return "ai"
		case 'ำ': // U+0E33
			return "am"
		case 'ๅ': // U+0E45 (duplicate inherent vowel)
			return "o"
		}
	}

	// Fallback: if no matching vowel found, return empty (rare case)
	return ""
}

// romanizeFinal maps Thai final consonants to RTGS codas (8 codas)
// Aspirates lose aspiration: ข ค ฆ -> k, etc.
func romanizeFinal(final string) string {
	switch final {
	// k-coda: ก ข ค ฆ
	case "ก", "ข", "ค", "ฆ":
		return "k"

	// ng-coda: ง
	case "ง":
		return "ng"

	// t-coda: จ ช ซ ฎ ฏ ฐ ฑ ฒ ด ต ถ ท ธ ศ ษ ส
	case "จ", "ช", "ซ", "ฎ", "ฏ", "ฐ", "ฑ", "ฒ", "ด", "ต", "ถ", "ท", "ธ", "ศ", "ษ", "ส":
		return "t"

	// n-coda: ญ ณ น ร ล ฬ
	case "ญ", "ณ", "น", "ร", "ล", "ฬ":
		return "n"

	// p-coda: บ ป พ ฟ ภ
	case "บ", "ป", "พ", "ฟ", "ภ":
		return "p"

	// m-coda: ม
	case "ม":
		return "m"

	// i-coda: ย
	case "ย":
		return "i"

	// o-coda: ว
	case "ว":
		return "o"

	default:
		return ""
	}
}

type phoneticEquivalence struct{ from, to string }

// phoneticEquivalence is one spelling-only fold applied when COMPARING names.
//
// These folds unify spellings that RTGS romanizes differently onto a single comparison form.
// RTGS writes จ as "ch" and ว as "w"; English conventionally writes them "j" and "v".
// Example: ใจดี -> RTGS "chaidi", commonly written "Jaidee"; วิชัย -> RTGS "wichai",
// commonly written "Vichai".
//
// SELECTION RULE: Fold only a Latin letter that RTGS never emits in its own output alphabet.
// RTGS's output alphabet is:
//
//	a, ae, ai, am, b, ch, d, e, f, h, i, ia, k, kh, l, m, n, ng, o, p, ph, r, s, t, th, u, ua, ue, uea, w, y
//
// "j" and "v" never appear in that alphabet, so folding them only rewrites the English side
// and destroys no information.
//
// REJECTED FOLDS (with measured benchmark effect):
//   - g->k: rejected because "g" occurs in RTGS only inside the digraph "ng", so the fold corrupts it.
//     Example: "wongpool" would become "wonkpool". Measured effect on the benchmark: none
//     (true-positive count unchanged).
//   - t->th and p->ph: rejected because both "t" and "p" are emitted alone AND inside the digraphs
//     "th"/"ph" in RTGS output, so folding them corrupts the RTGS side itself (example:
//     "thongdi" would become "thhongdi") and also merges the Thai consonant classes ต/ท and ป/พ,
//     which Thai contrasts phonemically. Note that each of these two folds measured +1 true positive
//     on the benchmark with no precision loss when tried, but the benchmark contains no pair that would
//     expose the resulting merged-contrast error, so that apparent gain is fitting the test set,
//     not a genuine accuracy improvement, which is why they were not added.
//
// This fold list is used for a COMPARISON key only. RomanizeThai and RomanizeThaiTokens must keep
// emitting RTGS-correct output (this guarantee is already stated in a nearby comment).
//
// Ordered, not a map, so a multi-entry fold is deterministic.
// Deliberately minimal. Each additional fold merges two sounds that Thai distinguishes,
// so it buys recall at precision's expense and must be justified by measurement, not by
// symmetry with this entry.
var phoneticEquivalents = []phoneticEquivalence{
	{from: "j", to: "ch"},
	{from: "v", to: "w"},
}

// PhoneticSkeleton reduces a romanized or Latin string to a comparable consonant skeleton.
// Identical reduction for BOTH scripts: lowercase, keep digraphs as single units,
// drop vowels except a leading one.
//
// HAZARD: Do NOT map Latin "ph" to "f". RTGS uses ph for aspirated /p/ (พ);
// treating it as English /f/ would destroy the alignment this exists to create.
//
// Latin "j" folds to "ch" via phoneticEquivalents so an English spelling aligns with the RTGS one.
func PhoneticSkeleton(s string) string {
	if s == "" {
		return ""
	}

	s = strings.ToLower(s)

	// Extract digraphs and non-vowel consonants
	var result []string
	runes := []rune(s)
	i := 0
	leadingVowel := true

	for i < len(runes) {
		r := runes[i]

		// Check for digraphs: ch, th, ph, ng, kh
		if i+1 < len(runes) {
			digraph := string([]rune{runes[i], runes[i+1]})
			switch digraph {
			case "ch", "th", "ph", "ng", "kh":
				result = append(result, digraph)
				i += 2
				leadingVowel = false
				continue
			}
		}

		// Handle vowels
		if isLatinVowel(r) {
			if leadingVowel {
				result = append(result, string(r))
				leadingVowel = false
			}
			// else: drop non-leading vowels
			i++
			continue
		}

		// Handle consonants and other characters
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			c := string(r)
			for _, eq := range phoneticEquivalents {
				if c == eq.from {
					c = eq.to
					break
				}
			}
			result = append(result, c)
			leadingVowel = false
		}

		i++
	}

	return strings.Join(result, "")
}

// PhoneticComparisonForm folds spelling-only differences in an already-romanized
// string so two conventions for the same sound compare equal. Unlike
// PhoneticSkeleton it keeps vowels, so it is usable where the vowel-bearing form
// is what carries the discriminating information.
//
// Comparison only -- callers must not store or display the result as
// romanization, which stays RTGS-correct.
func PhoneticComparisonForm(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)
	for _, eq := range phoneticEquivalents {
		s = strings.ReplaceAll(s, eq.from, eq.to)
	}
	return s
}

// RomanizeThaiTokens romanizes each token independently, preserving vowels (unlike
// PhoneticSkeleton, which discards them). Non-Thai tokens pass through unchanged except for
// lowercasing, since RomanizeThai already does that per rune for non-Thai runs.
//
// This exists for cross-script comparison of names PART BY PART (e.g. given name vs surname)
// rather than as one joined string: aligning by token position lets a scorer require agreement
// on every part, not just the string as a whole. See CrossScriptPartsScore in scorer.go, which
// is the reason this function exists.
func RomanizeThaiTokens(tokens []string) []string {
	out := make([]string, len(tokens))
	for i, t := range tokens {
		out[i] = RomanizeThai(t)
	}
	return out
}

// isLatinVowel checks if a rune is a Latin vowel
func isLatinVowel(r rune) bool {
	switch unicode.ToLower(r) {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	default:
		return false
	}
}
