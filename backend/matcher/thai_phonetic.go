package matcher

// ContainsThai checks if a string contains Thai Unicode characters (U+0E00–U+0E7F)
func ContainsThai(s string) bool {
	for _, r := range s {
		if r >= 0xE00 && r <= 0xE7F {
			return true
		}
	}
	return false
}

// ThaiPhoneticForm normalizes Thai text to a phonetic representation
// implementing Thai Soundex-style canonicalization for entity matching
func ThaiPhoneticForm(s string) string {
	runes := []rune(s)
	var result []rune

	i := 0
	for i < len(runes) {
		c := runes[i]

		// Step 1: Merge leading vowels with following consonants
		// Leading vowels (เ แ โ ใ ไ) appear before consonant but are pronounced after
		// Map consonant phonetically and discard the leading vowel
		if isLeadingVowel(c) && i+1 < len(runes) && isConsonant(runes[i+1]) {
			consonant := runes[i+1]
			// Map consonant and skip both vowel and consonant
			phonetic := mapConsonant(consonant)
			result = append(result, phonetic)
			i += 2
			continue
		}

		// Step 2: Replace ใ with ไ (merge these homophones)
		if c == 0xE43 { // ใ
			result = append(result, 0xE44) // ไ
			i++
			continue
		}

		// Step 3: Handle ์ (thanthakhat, silent mark at U+0E4C)
		// Remove the mark AND the consonant it marks
		if c == 0xE4C {
			if len(result) > 0 {
				last := result[len(result)-1]
				if isConsonant(last) {
					result = result[:len(result)-1]
				}
			}
			i++
			continue
		}

		// Step 4: Handle รร (ro han) -> map to 'n' phonetic
		if c == 0xE23 && i+1 < len(runes) && runes[i+1] == 0xE23 {
			result = append(result, 'n') // รร becomes 'n' (ro han pronounced as 'an')
			i += 2
			continue
		}

		// Step 5: Strip leading vowels (reordered ones) and all other vowel marks
		if isLeadingVowel(c) || isToneMark(c) || isVowelMark(c) {
			i++
			continue
		}

		// Step 6: Map consonants to homophone phonetic classes
		if isConsonant(c) {
			phonetic := mapConsonant(c)
			result = append(result, phonetic)
			i++
			continue
		}

		// Step 7: Keep non-Thai characters (Latin, digits, etc.) untouched
		result = append(result, c)
		i++
	}

	return string(result)
}

func isLeadingVowel(c rune) bool {
	// เ แ โ ใ ไ
	switch c {
	case 0xE40, 0xE41, 0xE45, 0xE43, 0xE44:
		return true
	default:
		return false
	}
}

func isConsonant(c rune) bool {
	// Thai consonants are in Unicode block U+0E01-U+0E2E plus U+0E2C (ฬ)
	// Some are excluding but we include them all for the phonetic mapping
	switch {
	case c >= 0xE01 && c <= 0xE2E:
		return true
	case c == 0xE2C: // ฬ
		return true
	default:
		return false
	}
}

func isToneMark(c rune) bool {
	// Tone marks: ่ ้ ๊ ๋ (U+0E48-U+0E4B)
	return c >= 0xE48 && c <= 0xE4B
}

func isVowelMark(c rune) bool {
	// Vowel diacritics and length marks
	switch c {
	case 0xE31, 0xE34, 0xE35, 0xE36, 0xE37, 0xE38, 0xE39, 0xE47:
		return true
	default:
		return false
	}
}

func mapConsonant(c rune) rune {
	// Map Thai consonants to homophone classes using INITIAL-position sound classes
	switch c {
	// ก-class
	case 0xE01: // ก
		return 'k'

	// ข-class (aspirated k)
	case 0xE02, 0xE03, 0xE04, 0xE05, 0xE06: // ข ฃ ค ฅ ฆ
		return 'K'

	// ง-class
	case 0xE07: // ง
		return 'G'

	// จ-class
	case 0xE08: // จ
		return 'j'

	// ฉ-class (aspirated ch)
	case 0xE09, 0xE0A, 0xE0C: // ฉ ช ฌ
		return 'C'

	// ซ-class (s sound)
	case 0xE0B, 0xE28, 0xE29, 0xE2A: // ซ ศ ษ ส
		return 's'

	// ญ-class (y sound)
	case 0xE0D, 0xE22: // ญ ย
		return 'y'

	// ด-class (d sound)
	case 0xE0E, 0xE14: // ฎ ด
		return 'd'

	// ต-class (t sound)
	case 0xE0F, 0xE15: // ฏ ต
		return 't'

	// ถ-class (aspirated t)
	case 0xE10, 0xE11, 0xE12, 0xE16, 0xE17, 0xE18: // ฐ ฑ ฒ ถ ท ธ
		return 'T'

	// น-class (n sound - don't merge with r/l)
	case 0xE13, 0xE19: // ณ น
		return 'n'

	// บ-class
	case 0xE1A: // บ
		return 'b'

	// ป-class
	case 0xE1B: // ป
		return 'p'

	// ผ-class (aspirated p)
	case 0xE1C, 0xE1E, 0xE20: // ผ พ ภ
		return 'P'

	// ฝ-class (f sound)
	case 0xE1D, 0xE1F: // ฝ ฟ
		return 'f'

	// ม-class
	case 0xE21: // ม
		return 'm'

	// ร-class (r sound - don't merge with l/n)
	case 0xE23: // ร
		return 'r'

	// ล-class (l sound - don't merge with r/n)
	case 0xE25, 0xE2C: // ล ฬ
		return 'l'

	// ว-class
	case 0xE27: // ว
		return 'w'

	// ห-class (h sound)
	case 0xE2B, 0xE2E: // ห ฮ
		return 'h'

	// อ-class (glottal stop)
	case 0xE2D: // อ
		return '\''

	default:
		// Fallback: return as-is if no mapping found
		return c
	}
}
