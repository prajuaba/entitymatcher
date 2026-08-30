package matcher

// Syllable represents a segmented Thai syllable with its components
type Syllable struct {
	Text    string // the original substring
	Initial string // initial consonant or cluster
	Vowel   string // written vowel, empty when implicit
	Final   string // final consonant, empty when none
	Tone    string // tone mark, empty when none
}

// SegmentThaiSyllables splits a string into Thai syllables.
// For non-Thai runs (Latin, digits, punctuation), returns a single Syllable with Text only.
// This is an approximation: Thai syllable segmentation without a dictionary is inherently ambiguous.
//
// Known failure classes that require lexical knowledge to resolve:
//   - Sanskrit/Pali-derived words with silent written vowels. Example: สุชาติ (Suchat) correctly
//     should be [สุ] [ชาติ] where ชา is one syllable with ช initial and า vowel, then ติ with ต initial
//     and ิ silent mark. But the algorithm segments it as three: [สุ] [ชา] [ติ]. Similarly, ประเสริฐ
//     (Prasert) is correctly [ประ] [เสริฐ] but segments as three: [ประ] [เส] [ริฐ]. These require
//     a Thai dictionary to disambiguate which written vowels are pronounced vs. silent (silent vowels
//     make their consonant a syllable-closer).
//   - Clusters not in the defined list of initial consonant clusters are *not* treated as clusters
func SegmentThaiSyllables(s string) []Syllable {
	if s == "" {
		return []Syllable{}
	}

	var result []Syllable
	runes := []rune(s)
	i := 0
	n := len(runes)

	// Accumulator for non-Thai runs
	var nonThaiRun []rune

	for i < n {
		r := runes[i]

		// If current char is non-Thai, accumulate until Thai starts again
		if !isThaiChar(r) {
			nonThaiRun = append(nonThaiRun, r)
			i++
			continue
		}

		// If we have accumulated non-Thai characters, flush them as a single Syllable
		if len(nonThaiRun) > 0 {
			result = append(result, Syllable{Text: string(nonThaiRun)})
			nonThaiRun = nil
		}

		// Start of Thai syllable segmentation
		start := i

		var initial, vowel, final, tone string

		// Step 1: Consume leading vowels (before initial consonant)
		for i < n && isLeadingVowel(runes[i]) {
			vowel += string(runes[i])
			i++
		}

		// Step 2: Consume initial consonant or cluster
		// Only consonants can be initials; vowel marks and other non-consonant Thai characters are skipped
		// IMPORTANT: A cluster (CC) is only recognized as a cluster if followed by a vowel mark or vowel.
		// At word-end or before another consonant, treat as single initial + final.
		if i < n && isConsonant(runes[i]) {
			if i+1 < n && isConsonant(runes[i+1]) {
				two := string(runes[i]) + string(runes[i+1])
				if isInitialCluster(two) {
					// Verify cluster is followed by a vowel; if not, it's initial+final
					if i+2 < n && (isVowelMark(runes[i+2]) || isFollowingVowel(runes[i+2]) || isBelowVowel(runes[i+2])) {
						initial = two
						i += 2
					} else if i+2 >= n {
						// Cluster at end of word: not a valid cluster start, treat as initial+final
						initial = string(runes[i])
						i++
					} else {
						// Cluster before consonant: not a valid cluster start, treat as initial+final
						initial = string(runes[i])
						i++
					}
				} else {
					initial = string(runes[i])
					i++
				}
			} else if i+1 < n {
				two := string(runes[i]) + string(runes[i+1])
				if isInitialCluster(two) {
					// Check if the next char is a vowel
					if i+2 < n && (isVowelMark(runes[i+2]) || isFollowingVowel(runes[i+2]) || isBelowVowel(runes[i+2])) {
						initial = two
						i += 2
					} else if i+2 >= n {
						// Cluster at end of word: treat as initial+final
						initial = string(runes[i])
						i++
					} else {
						initial = string(runes[i])
						i++
					}
				} else {
					initial = string(runes[i])
					i++
				}
			} else {
				initial = string(runes[i])
				i++
			}
		}

		// Step 3: Consume vowel marks and following vowels (positioned after consonant, before final)
		// This includes: vowel marks (ิ ี ึ ื ั ็), below vowels (ุ ู), and following vowels (ะ า ำ ๅ)
		for i < n {
			r := runes[i]
			if isVowelMark(r) || isFollowingVowel(r) || isBelowVowel(r) {
				// Special handling for ั: if it's followed by ว, consume as "ัว"
				if r == 0xE31 && i+1 < n && runes[i+1] == 0xE27 { // ั + ว
					vowel += "ัว"
					i += 2
					continue
				}
				vowel += string(r)
				i++
			} else {
				break
			}
		}

		// Step 4: Consume final consonant(s)
		// A consonant can only be a final if:
		// 1. NOT followed by vowel marks/following vowels (those would make it an initial of next syllable)
		// 2. NOT the first character of a valid initial cluster
		if i < n && isConsonant(runes[i]) {
			// Lookahead: check if vowel marks follow or if this starts a cluster
			nextIdx := i + 1
			hasFollowingVowels := false
			startsCluster := false

			if nextIdx < n {
				if isVowelMark(runes[nextIdx]) || isFollowingVowel(runes[nextIdx]) || isBelowVowel(runes[nextIdx]) {
					hasFollowingVowels = true
				} else if isConsonant(runes[nextIdx]) {
					// Check if these two consonants form a valid initial cluster
					cluster := string(runes[i]) + string(runes[nextIdx])
					if isInitialCluster(cluster) {
						startsCluster = true
					}
				}
			}

			// Only treat as final if no vowel marks follow and doesn't start a cluster
			if !hasFollowingVowels && !startsCluster {
				final = string(runes[i])
				i++
				// Some final consonants are silent due to ์
				if i < n && runes[i] == 0xE4C { // ์
					final = ""
					i++
				}
			}
		}

		// Step 5: Consume tone mark(s)
		for i < n && isToneMark(runes[i]) {
			tone += string(runes[i])
			i++
		}

		// Safety guard: if no progress was made (index didn't advance), consume at least one rune
		// to prevent infinite loops. This ensures termination even on malformed input.
		if i == start {
			i++
		}

		// Build syllable text from original range [start, i)
		text := string(runes[start:i])
		result = append(result, Syllable{
			Text:    text,
			Initial: initial,
			Vowel:   vowel,
			Final:   final,
			Tone:    tone,
		})
	}

	// Flush any remaining non-Thai buffer
	if len(nonThaiRun) > 0 {
		result = append(result, Syllable{Text: string(nonThaiRun)})
	}

	return result
}

// isThaiChar checks if a rune is any Thai character (consonant, vowel, tone, etc.)
func isThaiChar(r rune) bool {
	// Thai block: U+0E00–U+0E7F
	return r >= 0x0E00 && r <= 0x0E7F
}

// isInitialCluster returns true if the 2-character string is one of the Thai initial consonant clusters
func isInitialCluster(s string) bool {
	switch s {
	case "กร", "กล", "กว", "ขร", "ขล", "ขว", "คร", "คล", "คว", "ตร",
		"ปร", "ปล", "ผล", "พร", "พล", "บร", "บล", "ฟร", "ฟล", "ทร":
		return true
	default:
		return false
	}
}

// isBelowVowel returns true if the rune is a below vowel mark (ุ ู)
func isBelowVowel(r rune) bool {
	return r == 0xE38 || r == 0xE39 // ุ ู
}

// isFollowingVowel returns true if the rune is a following vowel (ะ า ำ ๅ)
func isFollowingVowel(r rune) bool {
	return r == 0xE30 || r == 0xE32 || r == 0xE33 || r == 0xE45 // ะ า ำ ๅ
}
