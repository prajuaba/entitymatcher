package matcher

import (
	"strings"
	"time"
)

// ParseFlexibleDate parses a date string using multiple flexible formats.
// It returns (time.Time{}, false) for empty or unparseable input.
// Supports Buddhist Era (B.E.) conversion: if year >= 2400, subtracts 543.
func ParseFlexibleDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}

	// Replace Thai digits with ASCII digits
	s = replaceThaiDigits(s)

	// Direct layouts
	directLayouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006/01/02",
		"20060102",
		"2006-01-02 15:04:05",
		"2 January 2006",
		"02 Jan 2006",
	}

	for _, layout := range directLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return applyBuddhistEra(t), true
		}
	}

	// Handle ambiguous formats like "02/01/2006" or "02-01-2006"
	parts := strings.Split(s, "/")
	if len(parts) == 3 {
		if t, ok := parseAmbiguousDate(parts[0], parts[1], parts[2]); ok {
			return applyBuddhistEra(t), true
		}
	}

	// Try with dashes
	parts = strings.Split(s, "-")
	if len(parts) == 3 {
		if t, ok := parseAmbiguousDate(parts[0], parts[1], parts[2]); ok {
			return applyBuddhistEra(t), true
		}
	}

	return time.Time{}, false
}

// replaceThaiDigits replaces Thai digits (๐-๙) with ASCII digits (0-9)
func replaceThaiDigits(s string) string {
	// Thai digits: ๐ (U+0E50) to ๙ (U+0E59)
	for i := 0; i <= 9; i++ {
		thaiDigit := string(rune(0x0E50 + i))
		asciiDigit := string(rune('0' + i))
		s = strings.ReplaceAll(s, thaiDigit, asciiDigit)
	}
	return s
}

// parseAmbiguousDate handles date strings like "02/01/2006" with day-first preference (EU/Thai convention)
// When ambiguous (both day-first and month-first valid), prefers day-first.
func parseAmbiguousDate(part1, part2, part3 string) (time.Time, bool) {
	// Parse year (part3)
	year, err := parseAsInteger(part3)
	if err != nil || year < 1 || year > 9999 {
		return time.Time{}, false
	}

	// Try day-first interpretation (EU/Thai convention): part1=day, part2=month
	potentialDay, err1 := parseAsInteger(part1)
	potentialMonth, err2 := parseAsInteger(part2)

	if err1 == nil && err2 == nil {
		// Check if day-first interpretation is valid
		if potentialMonth >= 1 && potentialMonth <= 12 && potentialDay >= 1 && potentialDay <= 31 {
			t := time.Date(year, time.Month(potentialMonth), potentialDay, 0, 0, 0, 0, time.UTC)
			// Verify the date was constructed correctly (to catch invalid dates like Feb 30)
			if t.Day() == potentialDay && t.Month() == time.Month(potentialMonth) && t.Year() == year {
				return t, true
			}
		}

		// Day-first interpretation failed, try month-first: part1=month, part2=day
		if potentialDay >= 1 && potentialDay <= 12 && potentialMonth >= 1 && potentialMonth <= 31 {
			t := time.Date(year, time.Month(potentialDay), potentialMonth, 0, 0, 0, 0, time.UTC)
			// Verify the date was constructed correctly
			if t.Day() == potentialMonth && t.Month() == time.Month(potentialDay) && t.Year() == year {
				return t, true
			}
		}
	}

	return time.Time{}, false
}

// parseAsInteger parses a string as a non-negative integer
func parseAsInteger(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, &parseError{}
	}

	val := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, &parseError{}
		}
		val = val*10 + int(r-'0')
	}

	return val, nil
}

// applyBuddhistEra converts Buddhist Era year to Common Era if needed (B.E. >= 2400)
func applyBuddhistEra(t time.Time) time.Time {
	if t.Year() >= 2400 {
		newYear := t.Year() - 543
		// Validate the new year is reasonable
		if newYear >= 1 && newYear <= 9999 {
			t = time.Date(newYear, t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
		}
	}
	return t
}

// parseError is a simple error type for parsing failures
type parseError struct{}

func (e *parseError) Error() string {
	return "parse error"
}
