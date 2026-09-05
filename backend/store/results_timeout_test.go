package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResultsSaveTimeout(t *testing.T) {
	tests := []struct {
		name string
		n    int
	}{
		{
			name: "zero rows",
			n:    0,
		},
		{
			name: "production batch size",
			n:    280049,
		},
		{
			name: "very large batch",
			n:    100_000_000,
		},
		{
			name: "small batch",
			n:    100,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			timeout := resultsSaveTimeout(tc.n)

			switch tc.n {
			case 0:
				assert.Equal(t, resultsSaveBaseTimeout, timeout)
			case 280049:
				expected := resultsSaveBaseTimeout + time.Duration(tc.n)*resultsSaveTimeoutPerRow
				assert.Less(t, timeout, resultsSaveTimeoutCap)
				assert.Greater(t, timeout, 30*time.Second)
				assert.InDelta(t, expected.Seconds(), timeout.Seconds(), 0.001)
			case 100_000_000:
				assert.Equal(t, resultsSaveTimeoutCap, timeout)
			case 100:
				expected := resultsSaveBaseTimeout + time.Duration(tc.n)*resultsSaveTimeoutPerRow
				assert.Equal(t, expected, timeout)
			}
		})
	}
}
