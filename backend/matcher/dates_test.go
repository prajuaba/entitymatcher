package matcher

import (
	"testing"
	"time"
)

func TestParseFlexibleDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Time
		wantOk   bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: time.Time{},
			wantOk:   false,
		},
		{
			name:     "garbage input",
			input:    "not a date",
			expected: time.Time{},
			wantOk:   false,
		},
		{
			name:     "RFC3339 layout",
			input:    "2023-12-25T15:30:45Z",
			expected: time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "YYYY-MM-DD layout",
			input:    "2023-12-25",
			expected: time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "YYYY/MM/DD layout",
			input:    "2023/12/25",
			expected: time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "YYYYMMDD layout",
			input:    "20231225",
			expected: time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "YYYY-MM-DD HH:MM:SS layout",
			input:    "2023-12-25 15:30:45",
			expected: time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "2 January 2006 layout",
			input:    "25 December 2023",
			expected: time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "02 Jan 2006 layout",
			input:    "25 Dec 2023",
			expected: time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "Buddhist Era conversion: YYYY-MM-DD",
			input:    "2569-08-15",
			expected: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "Buddhist Era conversion: DD/MM/YYYY",
			input:    "15/08/2569",
			expected: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "Thai digits conversion",
			input:    "๒๕๖๙-๐๘-๑๕",
			expected: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "ambiguity: day-first for DD/MM/YYYY (2nd Jan)",
			input:    "02/01/2006",
			expected: time.Date(2006, 1, 2, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "ambiguity: day-first for DD/MM/YYYY (15th Jan)",
			input:    "15/01/2006",
			expected: time.Date(2006, 1, 15, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "invalid date: Feb 30",
			input:    "2023-02-30",
			expected: time.Time{},
			wantOk:   false,
		},
		{
			name:     "Thai DD/MM/YY: year 00 pivots to 2000, not rejected as year 0",
			input:    "15/03/00",
			expected: time.Date(2000, 3, 15, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "null placeholder 00/00/00 stays unparseable",
			input:    "00/00/00",
			expected: time.Time{},
			wantOk:   false,
		},
		{
			name:     "Thai DD/MM/YY: 25/10/24 pivots to 2024",
			input:    "25/10/24",
			expected: time.Date(2024, 10, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "Thai DD/MM/YY: 04/10/24 pivots to 2024",
			input:    "04/10/24",
			expected: time.Date(2024, 10, 4, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "Thai DD/MM/YY: 15/01/26 pivots to 2026",
			input:    "15/01/26",
			expected: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "Thai DD/MM/YY: 23/01/26 pivots to 2026",
			input:    "23/01/26",
			expected: time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "2-digit year pivot: yy=99 goes to 1900s",
			input:    "01/02/99",
			expected: time.Date(1999, 2, 1, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "2-digit year pivot boundary: yy=70 goes to 1970",
			input:    "01/02/70",
			expected: time.Date(1970, 2, 1, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "2-digit year pivot boundary: yy=69 goes to 2069",
			input:    "01/02/69",
			expected: time.Date(2069, 2, 1, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "4-digit year is never pivoted",
			input:    "25/10/2024",
			expected: time.Date(2024, 10, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "direct YYYY-MM-DD layout unaffected by pivot logic",
			input:    "2024-10-25",
			expected: time.Date(2024, 10, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "day-first date with seconds and time is stripped",
			input:    "16/08/2026 11:00:00",
			expected: time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "day-first date with HH:MM time is stripped",
			input:    "16/08/2026 11:00",
			expected: time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "day-first date without time is unchanged",
			input:    "16/08/2026",
			expected: time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "ISO date with fractional-second time is unchanged, keeps its time",
			input:    "2026-08-16 11:00:00.000",
			expected: time.Date(2026, 8, 16, 11, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseFlexibleDate(tt.input)
			if ok != tt.wantOk {
				t.Errorf("ParseFlexibleDate(%q) ok = %v, want %v", tt.input, ok, tt.wantOk)
			}
			if ok && !got.Equal(tt.expected) {
				t.Errorf("ParseFlexibleDate(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseFlexibleDateInCalendar(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		calendar string
		expected time.Time
		wantOk   bool
	}{
		{
			name:     "BE calendar with yy=68",
			input:    "25/10/68",
			calendar: "BE",
			expected: time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "BE calendar with yyyy=2568",
			input:    "25/10/2568",
			calendar: "BE",
			expected: time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "CE calendar with yy=68",
			input:    "25/10/68",
			calendar: "CE",
			expected: time.Date(2068, 10, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "CE calendar with yy=25",
			input:    "25/10/25",
			calendar: "CE",
			expected: time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "AUTO calendar with yy=68",
			input:    "25/10/68",
			calendar: "AUTO",
			expected: time.Date(2068, 10, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "AUTO calendar with yyyy=2568",
			input:    "25/10/2568",
			calendar: "AUTO",
			expected: time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
		{
			name:     "BE calendar with Thai digits",
			input:    "๒๕/๑๐/๒๕๖๘",
			calendar: "BE",
			expected: time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC),
			wantOk:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseFlexibleDateInCalendar(tt.input, tt.calendar)
			if ok != tt.wantOk {
				t.Errorf("ParseFlexibleDateInCalendar(%q, %q) ok = %v, want %v", tt.input, tt.calendar, ok, tt.wantOk)
			}
			if ok && !got.Equal(tt.expected) {
				t.Errorf("ParseFlexibleDateInCalendar(%q, %q) = %v, want %v", tt.input, tt.calendar, got, tt.expected)
			}
		})
	}
}

func TestCalculateDateScoreSameCalendarDay(t *testing.T) {
	srcDate := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	destDate := time.Date(2026, 8, 16, 11, 0, 0, 0, time.UTC)
	got := CalculateDateScore(srcDate, destDate, 30)
	if got != 1.0 {
		t.Errorf("CalculateDateScore(%v, %v, 30) = %v, want 1.0", srcDate, destDate, got)
	}
}
