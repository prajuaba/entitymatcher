package matcher

import (
	"strings"
	"unicode"
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
	vowel := romanizeVowel(syl.Vowel)
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

	// อ-class (glottal stop, handled in romanizeSyllable)
	case "อ":
		return ""

	// If initial is a cluster, recursively handle it
	// (clusters should already be in the initial position from segmenter)
	default:
		// Try to handle clusters by mapping component consonants
		if len(initial) > 1 {
			// For clusters like "กร", map each component
			runes := []rune(initial)
			var result string
			for _, r := range runes {
				result += romanizeInitial(string(r))
			}
			return result
		}
		return ""
	}
}

// romanizeVowel maps Thai vowels to RTGS romanization
// Handles written vowels and approximations for rare ones
// Implicit vowel (when empty): defaults to "a"
func romanizeVowel(vowel string) string {
	if vowel == "" {
		// Implicit vowel: default to "a"
		return "a"
	}

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

// PhoneticSkeleton reduces a romanized or Latin string to a comparable consonant skeleton.
// Identical reduction for BOTH scripts: lowercase, keep digraphs as single units,
// drop vowels except a leading one.
//
// HAZARD: Do NOT map Latin "ph" to "f". RTGS uses ph for aspirated /p/ (พ);
// treating it as English /f/ would destroy the alignment this exists to create.
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
			result = append(result, string(r))
			leadingVowel = false
		}

		i++
	}

	return strings.Join(result, "")
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
