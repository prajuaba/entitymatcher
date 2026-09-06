package matcher

import (
	"regexp"
	"strings"
	"time"
)

// ParseFlexibleDate parses a date string using multiple flexible formats.
// It returns (time.Time{}, false) for empty or unparseable input.
// Supports Buddhist Era (B.E.) conversion: if year >= 2400, subtracts 543.
func ParseFlexibleDate(s string) (time.Time, bool) {
	return ParseFlexibleDateInCalendar(s, "AUTO")
}

// ParseFlexibleDateInCalendar is the main parsing function with explicit calendar control.
// calendar is case-insensitive and accepts "", "AUTO", "CE", "BE".
//   - "" or "AUTO": 2-digit year pivots CE-style (yy <= 69 -> 2000+yy, else 1900+yy);
//     a 4-digit year >= 2400 gets 543 subtracted.
//   - "CE": never subtract 543; 2-digit year still pivots CE-style as above.
//   - "BE": the year token is Buddhist. A 2-digit yy maps to 2500+yy, then 543 is
//     subtracted. A 4-digit year >= 2400 has 543 subtracted as today. A 4-digit
//     year < 2400 is left alone.
func ParseFlexibleDateInCalendar(s string, calendar string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}

	// Replace Thai digits with ASCII digits
	s = replaceThaiDigits(s)

	// Make calendar case-insensitive for every downstream comparison.
	calendar = strings.ToUpper(calendar)

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
			return applyBuddhistEra(t, calendar), true
		}
	}

	// Handle ambiguous formats like "02/01/2006" or "02-01-2006". A trailing time
	// component (e.g. "16/08/2026 11:00:00") is stripped only here, after the
	// directLayouts loop above has already had its chance to match the full
	// string with its time intact.
	stripped := stripTrailingTime(s)

	parts := strings.Split(stripped, "/")
	if len(parts) == 3 {
		if t, ok := parseAmbiguousDate(parts[0], parts[1], parts[2], calendar); ok {
			return applyBuddhistEra(t, calendar), true
		}
	}

	// Try with dashes
	parts = strings.Split(stripped, "-")
	if len(parts) == 3 {
		if t, ok := parseAmbiguousDate(parts[0], parts[1], parts[2], calendar); ok {
			return applyBuddhistEra(t, calendar), true
		}
	}

	return time.Time{}, false
}

// trailingTimeRegex matches a genuine trailing time-of-day component (e.g.
// " 11:00:00", " 11:00:00.000", " 11:00:00Z", " 11:00:00+07:00") so it can be
// stripped before an ambiguous date split is attempted. Compiled once at
// package level since ParseFlexibleDateInCalendar runs per row over ~130k
// rows per batch.
var trailingTimeRegex = regexp.MustCompile(`\s+\d{1,2}:\d{2}(:\d{2})?(\.\d+)?\s*(Z|[+-]\d{2}:?\d{2})?$`)

// stripTrailingTime removes a trailing time-of-day component from s, if present.
func stripTrailingTime(s string) string {
	return trailingTimeRegex.ReplaceAllString(s, "")
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
// calendar (already upper-cased by the caller) controls how a 2-digit year token is pivoted:
// "BE" maps yy to 2500+yy (a Buddhist-era year), anything else pivots CE-style.
func parseAmbiguousDate(part1, part2, part3 string, calendar string) (time.Time, bool) {
	// Trim and apply 2-digit year pivot rule
	trimmedPart3 := strings.TrimSpace(part3)
	year, err := parseAsInteger(trimmedPart3)
	if err != nil {
		return time.Time{}, false
	}

	// Apply 2-digit year pivot for real-world Thai dates (DD/MM/YY format)
	// If part3 is 2 digits or less, treat as YY and pivot to 19XX/20XX (CE) or
	// 25XX (BE, per calendar).
	// A 4-digit year like "2024" or "0024" must stay as-is to preserve explicit intent.
	// This runs before the range check so "00" pivots to 2000 rather than being
	// rejected as year 0.
	if len(trimmedPart3) <= 2 {
		switch calendar {
		case "BE":
			// Modern BE years are 25xx; the 543 conversion happens afterwards
			// in applyBuddhistEra.
			year = 2500 + year
		default: // AUTO, CE, or empty
			if year <= 69 {
				year = 2000 + year
			} else {
				year = 1900 + year
			}
		}
	}

	if year < 1 || year > 9999 {
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

// applyBuddhistEra converts a Buddhist Era year to Common Era according to calendar
// (already upper-cased by the caller): "CE" never subtracts; "AUTO"/"BE" (and any
// other value) subtract 543 when the year is >= 2400, which is a no-op for years
// below that threshold.
func applyBuddhistEra(t time.Time, calendar string) time.Time {
	calendar = strings.ToUpper(calendar)
	if calendar == "CE" {
		return t // Never subtract 543 for CE
	}
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
