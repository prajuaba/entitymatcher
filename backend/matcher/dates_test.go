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
