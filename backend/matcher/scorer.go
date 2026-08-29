package matcher

import (
	"math"
	"strings"
	"time"
)

type MatchWeights struct {
	NameWeight float64 `json:"name_weight"`
	DateWeight float64 `json:"date_weight"`
}

type AlgorithmToggles struct {
	UseJaroWinkler bool `json:"use_jaro_winkler"`
	UseLevenshtein bool `json:"use_levenshtein"`
	UseTokenSort   bool `json:"use_token_sort"`
	UsePhonetic    bool `json:"use_phonetic"`
	UseTrigram     bool `json:"use_trigram"`
}

var DefaultWeights = MatchWeights{
	NameWeight: 0.85,
	DateWeight: 0.15,
}

var DefaultAlgorithms = AlgorithmToggles{
	UseJaroWinkler: true,
	UseLevenshtein: true,
	UseTokenSort:   true,
	UsePhonetic:    true,
	UseTrigram:     true,
}

// JaroWinkler computes rune-safe Jaro-Winkler distance between two strings s1 and s2.
func JaroWinkler(s1, s2 string) float64 {
	r1, r2 := []rune(s1), []rune(s2)
	l1, l2 := len(r1), len(r2)

	if l1 == 0 && l2 == 0 {
		return 1.0
	}
	if l1 == 0 || l2 == 0 {
		return 0.0
	}
	if s1 == s2 {
		return 1.0
	}

	matchDistance := int(math.Max(float64(l1), float64(l2))/2) - 1
	if matchDistance < 0 {
		matchDistance = 0
	}

	r1Matches := make([]bool, l1)
	r2Matches := make([]bool, l2)

	var matches float64
	for i := 0; i < l1; i++ {
		start := int(math.Max(0, float64(i-matchDistance)))
		end := int(math.Min(float64(i+matchDistance+1), float64(l2)))

		for j := start; j < end; j++ {
			if r2Matches[j] || r1[i] != r2[j] {
				continue
			}
			r1Matches[i] = true
			r2Matches[j] = true
			matches++
			break
		}
	}

	if matches == 0 {
		return 0.0
	}

	var k, transpositions float64
	for i := 0; i < l1; i++ {
		if !r1Matches[i] {
			continue
		}
		for !r2Matches[int(k)] {
			k++
		}
		if r1[i] != r2[int(k)] {
			transpositions++
		}
		k++
	}

	jaro := ((matches / float64(l1)) + (matches / float64(l2)) + ((matches - transpositions/2.0) / matches)) / 3.0

	prefixLength := 0
	maxPrefix := int(math.Min(4, math.Min(float64(l1), float64(l2))))
	for i := 0; i < maxPrefix; i++ {
		if r1[i] == r2[i] {
			prefixLength++
		} else {
			break
		}
	}

	return jaro + (float64(prefixLength) * 0.1 * (1.0 - jaro))
}

// LevenshteinSimilarity computes normalized edit distance similarity (0.0 to 1.0) on runes.
func LevenshteinSimilarity(s1, s2 string) float64 {
	r1, r2 := []rune(s1), []rune(s2)
	l1, l2 := len(r1), len(r2)

	if l1 == 0 && l2 == 0 {
		return 1.0
	}
	if l1 == 0 || l2 == 0 {
		return 0.0
	}

	column := make([]int, l1+1)
	for y := 1; y <= l1; y++ {
		column[y] = y
	}

	for x := 1; x <= l2; x++ {
		column[0] = x
		lastkey := x - 1
		for y := 1; y <= l1; y++ {
			oldkey := column[y]
			incr := 0
			if r1[y-1] != r2[x-1] {
				incr = 1
			}
			column[y] = int(math.Min(float64(column[y]+1), math.Min(float64(column[y-1]+1), float64(lastkey+incr))))
			lastkey = oldkey
		}
	}

	dist := float64(column[l1])
	maxLen := float64(int(math.Max(float64(l1), float64(l2))))
	return 1.0 - (dist / maxLen)
}

// TrigramSimilarity computes character trigram Jaccard similarity.
func TrigramSimilarity(s1, s2 string) float64 {
	t1 := extractTrigrams(s1)
	t2 := extractTrigrams(s2)

	if len(t1) == 0 && len(t2) == 0 {
		return 1.0
	}
	if len(t1) == 0 || len(t2) == 0 {
		return 0.0
	}

	intersection := 0
	t2Map := make(map[string]int)
	for _, tr := range t2 {
		t2Map[tr]++
	}

	for _, tr := range t1 {
		if count, exists := t2Map[tr]; exists && count > 0 {
			intersection++
			t2Map[tr]--
		}
	}

	union := len(t1) + len(t2) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

func extractTrigrams(s string) []string {
	padded := "  " + strings.ToLower(s) + " "
	runes := []rune(padded)
	if len(runes) < 3 {
		return nil
	}
	var trigrams []string
	for i := 0; i <= len(runes)-3; i++ {
		trigrams = append(trigrams, string(runes[i:i+3]))
	}
	return trigrams
}

// CalculateDateScore uses exponential decay for date proximity scoring.
func CalculateDateScore(srcDate, destDate time.Time, maxToleranceDays int) float64 {
	if srcDate.IsZero() || destDate.IsZero() {
		return 1.0
	}
	diffDays := math.Abs(srcDate.Sub(destDate).Hours() / 24)

	if maxToleranceDays > 0 && diffDays > float64(maxToleranceDays) {
		return 0.0
	}

	// Exponential decay formula: e^(-0.05 * diffDays)
	score := math.Exp(-0.05 * diffDays)
	return math.Round(score*10000) / 10000
}

// DistinctiveTokenScore computes similarity prioritizing non-generic distinctive words.
func DistinctiveTokenScore(srcName, destName CleanName) (float64, bool) {
	if len(srcName.DistinctiveTokens) == 0 || len(destName.DistinctiveTokens) == 0 {
		return 0.0, false
	}

	var matched float64
	total := float64(int(math.Max(float64(len(srcName.DistinctiveTokens)), float64(len(destName.DistinctiveTokens)))))

	bilingualMatch := false

	for _, t1 := range srcName.DistinctiveTokens {
		for _, t2 := range destName.DistinctiveTokens {
			if t1 == t2 || JaroWinkler(t1, t2) >= 0.85 {
				matched++
				break
			} else if CheckBilingualMatch(t1, t2) {
				matched += 0.95
				bilingualMatch = true
				break
			}
		}
	}

	return matched / total, bilingualMatch
}

// CheckNumberMismatch checks if branch/store numerical IDs conflict.
func CheckNumberMismatch(srcName, destName CleanName) bool {
	if len(srcName.Numbers) == 0 || len(destName.Numbers) == 0 {
		return false
	}
	srcSet := make(map[string]bool)
	for _, n := range srcName.Numbers {
		srcSet[n] = true
	}
	destSet := make(map[string]bool)
	for _, n := range destName.Numbers {
		destSet[n] = true
	}
	for _, n := range srcName.Numbers {
		if !destSet[n] {
			return true
		}
	}
	for _, n := range destName.Numbers {
		if !srcSet[n] {
			return true
		}
	}
	return false
}

// ScoreResult details metrics generated for candidate pair.
type ScoreResult struct {
	TotalScore   float64  `json:"total_score"`
	NameScore    float64  `json:"name_score"`
	DateScore    float64  `json:"date_score"`
	JWScore      float64  `json:"jw_score"`
	LevScore     float64  `json:"lev_score"`
	TokenScore   float64  `json:"token_score"`
	TrigramScore float64  `json:"trigram_score"`
	MatchReasons []string `json:"match_reasons"`
}

// CalculateCompositeScore calculates name and date metrics according to weights, algorithms, and rules.
func CalculateCompositeScore(
	srcName, destName CleanName,
	srcDate, destDate time.Time,
	weights MatchWeights,
	algos AlgorithmToggles,
	dateTolerance int,
) ScoreResult {
	var scores []float64
	var jwScore, levScore, tokenScore, trigramScore float64
	var reasons []string

	// Distinctive Token Check
	distinctScore, isBilingual := DistinctiveTokenScore(srcName, destName)
	if isBilingual {
		reasons = append(reasons, "Bilingual Thai-English transliteration dictionary match")
	}

	// Raw Jaro-Winkler
	if algos.UseJaroWinkler {
		jwScore = JaroWinkler(srcName.Cleaned, destName.Cleaned)
		scores = append(scores, jwScore)
		if jwScore > 0.85 {
			reasons = append(reasons, "High Jaro-Winkler raw string similarity")
		}
	}

	// Token Sort Jaro-Winkler
	if algos.UseTokenSort {
		tokenScore = JaroWinkler(srcName.SortedTokens, destName.SortedTokens)
		scores = append(scores, tokenScore)
		if tokenScore > 0.85 {
			reasons = append(reasons, "High token-sorted similarity (name order invariant)")
		}
	}

	// Levenshtein
	if algos.UseLevenshtein {
		levScore = LevenshteinSimilarity(srcName.Cleaned, destName.Cleaned)
		scores = append(scores, levScore)
		if levScore > 0.80 {
			reasons = append(reasons, "Low edit distance similarity")
		}
	}

	// Trigram
	if algos.UseTrigram {
		trigramScore = TrigramSimilarity(srcName.Cleaned, destName.Cleaned)
		scores = append(scores, trigramScore)
		if trigramScore > 0.75 {
			reasons = append(reasons, "Significant character trigram overlap")
		}
	}

	// Phonetic Match
	if algos.UsePhonetic && srcName.PhoneticKey != "" && srcName.PhoneticKey == destName.PhoneticKey {
		reasons = append(reasons, "Exact phonetic consonant key match")
	}

	var nameScore float64
	if len(scores) > 0 {
		var sum float64
		var max float64
		for _, s := range scores {
			sum += s
			if s > max {
				max = s
			}
		}
		avg := sum / float64(len(scores))
		nameScore = (max * 0.6) + (avg * 0.4)

		// Integrate distinctive token score if available
		if distinctScore > 0 {
			if distinctScore > nameScore {
				nameScore = (nameScore * 0.4) + (distinctScore * 0.6)
			}
			if distinctScore >= 0.85 {
				reasons = append(reasons, "High distinctive non-generic token match")
			}
		}
	}

	// Check Branch / Number mismatch penalty
	if CheckNumberMismatch(srcName, destName) {
		nameScore *= 0.50 // Heavy penalty for different branch numbers
		reasons = append(reasons, "Branch / numerical identifier mismatch penalty (-50%)")
	}

	// Compute date score
	dateScore := CalculateDateScore(srcDate, destDate, dateTolerance)
	if dateScore >= 0.95 {
		reasons = append(reasons, "Exact or 1-day transaction date proximity")
	} else if dateScore >= 0.70 {
		reasons = append(reasons, "Transaction date within close tolerance window")
	} else if dateScore == 0.0 {
		reasons = append(reasons, "Transaction date exceeds tolerance delta")
	}

	// Composite total score
	totalScore := (nameScore * weights.NameWeight) + (dateScore * weights.DateWeight)

	return ScoreResult{
		TotalScore:   math.Round(totalScore*10000) / 10000,
		NameScore:    math.Round(nameScore*10000) / 10000,
		DateScore:    math.Round(dateScore*10000) / 10000,
		JWScore:      math.Round(jwScore*10000) / 10000,
		LevScore:     math.Round(levScore*10000) / 10000,
		TokenScore:   math.Round(tokenScore*10000) / 10000,
		TrigramScore: math.Round(trigramScore*10000) / 10000,
		MatchReasons: reasons,
	}
}
