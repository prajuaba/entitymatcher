package matcher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultCrossScriptAutoThreshold(t *testing.T) {
	require.InDelta(t, 0.84, DefaultConfig().CrossScriptAutoThreshold, 0.0001)
}

func TestAutoThresholdForSelectsPerPair(t *testing.T) {
	tests := []struct {
		name              string
		crossScript       bool
		config            Config
		expectedThreshold float64
	}{
		{
			name:              "cross-script with custom threshold",
			crossScript:       true,
			config:            Config{CrossScriptAutoThreshold: 0.84, AutoMatchThreshold: 0.90},
			expectedThreshold: 0.84,
		},
		{
			name:              "same-script with custom threshold",
			crossScript:       false,
			config:            Config{CrossScriptAutoThreshold: 0.84, AutoMatchThreshold: 0.90},
			expectedThreshold: 0.90,
		},
		{
			name:              "cross-script with zero threshold (old config case)",
			crossScript:       true,
			config:            Config{CrossScriptAutoThreshold: 0, AutoMatchThreshold: 0.90},
			expectedThreshold: 0.90,
		},
		{
			name:              "same-script with zero threshold (old config case)",
			crossScript:       false,
			config:            Config{CrossScriptAutoThreshold: 0, AutoMatchThreshold: 0.90},
			expectedThreshold: 0.90,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.AutoThresholdFor(tt.crossScript)
			require.InDelta(t, tt.expectedThreshold, result, 0.0001)
		})
	}
}

func TestCrossScriptPairAutoMatchesAtLowerBar(t *testing.T) {
	cfg := Config{
		AutoMatchThreshold:       0.90,
		CrossScriptAutoThreshold: 0.84,
		MarginThreshold:          0.05,
		ExactMatchFloor:          0.99,
	}

	topScore := 0.86
	runnerUpScore := 0.70

	// Cross-script auto-match should succeed with threshold 0.84
	autoMatched, _ := IsAutoMatchable(topScore, runnerUpScore, cfg.AutoThresholdFor(true), cfg.MarginThreshold, cfg.ExactMatchFloor)
	require.True(t, autoMatched, "expected cross-script pair to auto-match with score %f vs threshold %f", topScore, cfg.AutoThresholdFor(true))

	// Same-script auto-match should fail with threshold 0.90
	autoMatched, _ = IsAutoMatchable(topScore, runnerUpScore, cfg.AutoThresholdFor(false), cfg.MarginThreshold, cfg.ExactMatchFloor)
	require.False(t, autoMatched, "expected same-script pair to NOT auto-match with score %f vs threshold %f", topScore, cfg.AutoThresholdFor(false))
}

func TestCrossScriptThresholdStillRespectsMargin(t *testing.T) {
	cfg := Config{
		AutoMatchThreshold:       0.90,
		CrossScriptAutoThreshold: 0.84,
		MarginThreshold:          0.05,
		ExactMatchFloor:          0.99,
	}

	topScore := 0.86
	runnerUpScore := 0.85

	// Margin is only 0.01, less than required 0.05
	autoMatched, _ := IsAutoMatchable(topScore, runnerUpScore, cfg.AutoThresholdFor(true), cfg.MarginThreshold, cfg.ExactMatchFloor)
	require.False(t, autoMatched, "cross-script threshold must not bypass margin rule - margin was %f but required %f", topScore-runnerUpScore, cfg.MarginThreshold)
}
