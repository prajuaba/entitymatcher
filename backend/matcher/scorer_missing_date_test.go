package matcher

import (
	"math"
	"testing"
	"time"
)

func TestCompositeScoreDropsDateTermWhenDatesAbsent(t *testing.T) {
	const epsilon = 0.0001

	src := Normalize("John Smith")
	dest := Normalize("Jon Smyth")

	result := CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)

	// Assert that TotalScore equals NameScore (date term dropped)
	if math.Abs(result.TotalScore-result.NameScore) > epsilon {
		t.Errorf("TotalScore (%f) should equal NameScore (%f) when dates are absent", result.TotalScore, result.NameScore)
	}

	// Assert that DateScore is zero
	if result.DateScore != 0.0 {
		t.Errorf("DateScore should be 0.0 when dates are absent, got %f", result.DateScore)
	}

	// Assert that the reason string is present
	found := false
	for _, reason := range result.MatchReasons {
		if reason == "No comparable transaction date; scored on name similarity alone" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("MatchReasons should contain 'No comparable transaction date; scored on name similarity alone'")
	}

	// Assert that the old formula would have produced a higher score (regression check)
	oldFormulaTotal := result.NameScore*0.85 + 0.15
	if math.Abs(oldFormulaTotal-result.TotalScore) < epsilon {
		t.Errorf("Regression: TotalScore (%f) matches old formula (%f) which would have scored absent dates as perfect match", result.TotalScore, oldFormulaTotal)
	}
}

func TestCompositeScoreKeepsDateTermWhenBothDatesPresent(t *testing.T) {
	const epsilon = 0.0001

	src := Normalize("Acme Trading Company Limited")
	dest := Normalize("Acme Trading Company Limited")

	srcDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	destDate := srcDate.AddDate(0, 0, 1) // one day apart

	result := CalculateCompositeScore(src, dest, srcDate, destDate, DefaultWeights, DefaultAlgorithms, 30)

	// Compute expected total score
	expectedTotal := math.Round((result.NameScore*DefaultWeights.NameWeight+result.DateScore*DefaultWeights.DateWeight)*10000) / 10000

	// Assert that TotalScore matches expected
	if math.Abs(result.TotalScore-expectedTotal) > epsilon {
		t.Errorf("TotalScore (%f) should equal expected (%f) when both dates are present", result.TotalScore, expectedTotal)
	}

	// Assert that DateScore is greater than zero
	if result.DateScore <= 0.0 {
		t.Errorf("DateScore should be > 0.0 when dates are present, got %f", result.DateScore)
	}
}

func TestCompositeScoreDropsDateTermWhenOnlyOneSideHasDate(t *testing.T) {
	const epsilon = 0.0001

	src := Normalize("Acme Trading Company Limited")
	dest := Normalize("Acme Trading Company Limited")

	srcDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	destDate := time.Time{} // zero/absent

	t.Run("dest date absent", func(t *testing.T) {
		result := CalculateCompositeScore(src, dest, srcDate, destDate, DefaultWeights, DefaultAlgorithms, 30)

		if math.Abs(result.TotalScore-result.NameScore) > epsilon {
			t.Errorf("TotalScore (%f) should equal NameScore (%f) when only one side has date", result.TotalScore, result.NameScore)
		}
		if result.DateScore != 0.0 {
			t.Errorf("DateScore should be 0.0 when only one side has date, got %f", result.DateScore)
		}
	})

	t.Run("src date absent", func(t *testing.T) {
		result := CalculateCompositeScore(src, dest, destDate, srcDate, DefaultWeights, DefaultAlgorithms, 30)

		if math.Abs(result.TotalScore-result.NameScore) > epsilon {
			t.Errorf("TotalScore (%f) should equal NameScore (%f) when only one side has date", result.TotalScore, result.NameScore)
		}
		if result.DateScore != 0.0 {
			t.Errorf("DateScore should be 0.0 when only one side has date, got %f", result.DateScore)
		}
	})
}

func TestAbsentDateNoLongerInflatesAboveAutoThreshold(t *testing.T) {
	const epsilon = 0.0001

	// Empirical name pair confirmed to produce a nameScore in [0.882, 0.90) range
	srcRaw := "Southeast Asian Commerce Bank"
	destRaw := "SE Asian Commerce Bank"

	src := Normalize(srcRaw)
	dest := Normalize(destRaw)

	// Get baseline with dates present
	presentResult := CalculateCompositeScore(src, dest, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), DefaultWeights, DefaultAlgorithms, 30)

	// Sanity check: ensure nameScore is within expected range
	if presentResult.NameScore < 0.87 || presentResult.NameScore >= 0.90 {
		t.Fatalf("Fixture name pair does not produce expected nameScore in [0.87, 0.90) range; got %f - this test needs to be re-tuned", presentResult.NameScore)
	}

	// Compute what the old formula would have produced
	oldFormulaTotal := presentResult.NameScore*0.85 + 0.15

	// Ensure that old formula would have crossed auto-match threshold
	if oldFormulaTotal < 0.90 {
		t.Fatalf("Old formula does not push this pair above auto-match threshold; test fixture needs to be re-tuned, got %f", oldFormulaTotal)
	}

	// Now test with absent dates
	absentResult := CalculateCompositeScore(src, dest, time.Time{}, time.Time{}, DefaultWeights, DefaultAlgorithms, 30)

	// Assert that the new formula keeps it below auto-match threshold
	if absentResult.TotalScore >= 0.90 {
		t.Errorf("Absent date should not inflate score above 0.90 auto-match threshold; got %f", absentResult.TotalScore)
	}

	// Ensure the new result is consistent with no date term
	if math.Abs(absentResult.TotalScore-absentResult.NameScore) > epsilon {
		t.Errorf("TotalScore (%f) should equal NameScore (%f) when dates are absent", absentResult.TotalScore, absentResult.NameScore)
	}

	// Comment explaining defensive assertions:
	// Both the empirical name-pair fixture values (0.87-0.90 sanity range, and the fact the old formula pushes it to >= 0.90)
	// are asserted defensively with t.Fatalf so a future algorithm tweak that shifts nameScore fails loudly and explains why,
	// rather than silently testing nothing.
}
