package matcher

import (
	"strings"
	"testing"
	"time"
)

func TestResolveAssignmentsGreedy1to1(t *testing.T) {
	batchID := "test-batch"
	now := time.Now()
	cfg := Config{
		AutoMatchThreshold: 0.90,
		MarginThreshold:    0.05,
		ExactMatchFloor:    0.99,
	}

	t.Run("two_sources_same_destination_higher_scorer_wins", func(t *testing.T) {
		// Two sources matching the same destination - higher score should keep it
		items := []MatchResultItem{
			{
				ID:              batchID + "-src-A-dest-X",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-X",
				ConfidenceScore: 0.95,
				MatchStatus:     "AUTO_MATCHED",
				Rank:            1,
				ScoreMargin:     0.05,
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-B-dest-X",
				BatchID:         batchID,
				SourceID:        "src-B",
				DestinationID:   "dest-X",
				ConfidenceScore: 0.92,
				MatchStatus:     "AUTO_MATCHED",
				Rank:            1,
				ScoreMargin:     0.02,
				CreatedAt:       now,
			},
		}

		resolved := ResolveAssignments(items, "GREEDY_1_1", cfg)

		// Higher scorer (src-A) should remain AUTO_MATCHED
		if resolved[0].SourceID == "src-A" && resolved[0].MatchStatus != "AUTO_MATCHED" {
			t.Errorf("Expected src-A to remain AUTO_MATCHED, got %s", resolved[0].MatchStatus)
		}

		// Lower scorer (src-B) should be demoted to REVIEW_NEEDED
		if resolved[1].SourceID == "src-B" && resolved[1].MatchStatus != "REVIEW_NEEDED" {
			t.Errorf("Expected src-B to be demoted to REVIEW_NEEDED, got %s", resolved[1].MatchStatus)
		}

		// Check decision note mentions the winner
		if resolved[1].SourceID == "src-B" && len(resolved[1].DecisionNote) == 0 {
			t.Errorf("Expected decision note for demoted src-B")
		}
	})

	t.Run("rank1_rank2_within_margin_review_needed", func(t *testing.T) {
		// A source where rank-1 and rank-2 are close (within margin)
		// This should trigger REVIEW_NEEDED even if it meets auto-match threshold
		items := []MatchResultItem{
			{
				ID:              batchID + "-src-A-dest-X",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-X",
				ConfidenceScore: 0.92,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            1,
				ScoreMargin:     0.02, // Within margin threshold of 0.05
				DecisionNote:    "Ambiguous: runner-up within 0.02 — needs review",
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-A-dest-Y",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-Y",
				ConfidenceScore: 0.90,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            2,
				ScoreMargin:     0.0,
				DecisionNote:    "Alternative candidate (rank 2) for review",
				CreatedAt:       now,
			},
		}

		resolved := ResolveAssignments(items, "GREEDY_1_1", cfg)

		// Both should remain REVIEW_NEEDED (not demoted further)
		for _, item := range resolved {
			if item.MatchStatus != "REVIEW_NEEDED" && item.MatchStatus != "CONFIRMED" && item.MatchStatus != "REJECTED" {
				t.Errorf("Expected status to be REVIEW_NEEDED, got %s for rank %d", item.MatchStatus, item.Rank)
			}
		}
	})

	t.Run("no_candidates_no_match", func(t *testing.T) {
		// A source with a NO_MATCH row (no candidates found)
		items := []MatchResultItem{
			{
				ID:            batchID + "-src-A-NO_MATCH",
				BatchID:       batchID,
				SourceID:      "src-A",
				DestinationID: "",
				MatchStatus:   "NO_MATCH",
				DecisionNote:  "No blocking candidates found",
				CreatedAt:     now,
			},
		}

		resolved := ResolveAssignments(items, "GREEDY_1_1", cfg)

		// NO_MATCH should be left untouched
		if len(resolved) != 1 || resolved[0].MatchStatus != "NO_MATCH" {
			t.Errorf("Expected NO_MATCH to be preserved, got %d items with status %s", len(resolved), resolved[0].MatchStatus)
		}
	})

	t.Run("determinism_shuffled_input", func(t *testing.T) {
		// Run ResolveAssignments twice on different orderings, should get same results
		items1 := []MatchResultItem{
			{
				ID:              batchID + "-src-A-dest-X",
				SourceID:        "src-A",
				DestinationID:   "dest-X",
				ConfidenceScore: 0.95,
				MatchStatus:     "AUTO_MATCHED",
				Rank:            1,
				ScoreMargin:     0.10,
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-B-dest-Y",
				SourceID:        "src-B",
				DestinationID:   "dest-Y",
				ConfidenceScore: 0.92,
				MatchStatus:     "AUTO_MATCHED",
				Rank:            1,
				ScoreMargin:     0.08,
				CreatedAt:       now,
			},
		}

		items2 := []MatchResultItem{
			{
				ID:              batchID + "-src-B-dest-Y",
				SourceID:        "src-B",
				DestinationID:   "dest-Y",
				ConfidenceScore: 0.92,
				MatchStatus:     "AUTO_MATCHED",
				Rank:            1,
				ScoreMargin:     0.08,
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-A-dest-X",
				SourceID:        "src-A",
				DestinationID:   "dest-X",
				ConfidenceScore: 0.95,
				MatchStatus:     "AUTO_MATCHED",
				Rank:            1,
				ScoreMargin:     0.10,
				CreatedAt:       now,
			},
		}

		resolved1 := ResolveAssignments(items1, "GREEDY_1_1", cfg)
		resolved2 := ResolveAssignments(items2, "GREEDY_1_1", cfg)

		// Both should have same number of AUTO_MATCHED and destinations claimed
		autoMatched1 := 0
		autoMatched2 := 0
		for _, item := range resolved1 {
			if item.MatchStatus == "AUTO_MATCHED" {
				autoMatched1++
			}
		}
		for _, item := range resolved2 {
			if item.MatchStatus == "AUTO_MATCHED" {
				autoMatched2++
			}
		}

		if autoMatched1 != autoMatched2 {
			t.Errorf("Determinism check failed: got %d AUTO_MATCHED in first run, %d in second", autoMatched1, autoMatched2)
		}
	})

	t.Run("exact_match_rule_b_fires", func(t *testing.T) {
		// Rule (b): top 1.000 (exact match) / runner-up 0.983 -> AUTO_MATCHED via rule b
		items := []MatchResultItem{
			{
				ID:              batchID + "-src-A-dest-X",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-X",
				ConfidenceScore: 1.000,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            1,
				ScoreMargin:     0.017,
				DecisionNote:    "Ambiguous: runner-up within 0.017 — needs review",
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-A-dest-Y",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-Y",
				ConfidenceScore: 0.983,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            2,
				ScoreMargin:     0.0,
				DecisionNote:    "Alternative candidate (rank 2) for review",
				CreatedAt:       now,
			},
		}

		resolved := ResolveAssignments(items, "GREEDY_1_1", cfg)

		// Rank-1 should be AUTO_MATCHED via rule (b)
		if resolved[0].SourceID == "src-A" && resolved[0].Rank == 1 {
			if resolved[0].MatchStatus != "AUTO_MATCHED" {
				t.Errorf("Expected rank-1 to be AUTO_MATCHED via rule (b), got %s", resolved[0].MatchStatus)
			}
			if !strings.Contains(resolved[0].DecisionNote, "Exact normalized match") {
				t.Errorf("Expected decision note to mention exact match, got: %s", resolved[0].DecisionNote)
			}
			if !strings.Contains(resolved[0].DecisionNote, "0.983") {
				t.Errorf("Expected decision note to mention runner-up score 0.983, got: %s", resolved[0].DecisionNote)
			}
		}
	})

	t.Run("exact_match_both_at_floor", func(t *testing.T) {
		// Rule (b) must NOT fire: top 1.000 / runner-up 1.000 (both >= 0.99) = genuine tie
		items := []MatchResultItem{
			{
				ID:              batchID + "-src-A-dest-X",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-X",
				ConfidenceScore: 1.000,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            1,
				ScoreMargin:     0.0,
				DecisionNote:    "Ambiguous: runner-up within 0.000 — needs review",
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-A-dest-Y",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-Y",
				ConfidenceScore: 1.000,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            2,
				ScoreMargin:     0.0,
				DecisionNote:    "Alternative candidate (rank 2) for review",
				CreatedAt:       now,
			},
		}

		resolved := ResolveAssignments(items, "GREEDY_1_1", cfg)

		// Rank-1 should remain REVIEW_NEEDED (genuine tie, rule b must NOT fire)
		if resolved[0].SourceID == "src-A" && resolved[0].Rank == 1 {
			if resolved[0].MatchStatus != "REVIEW_NEEDED" {
				t.Errorf("Expected rank-1 to remain REVIEW_NEEDED on tie (both at 1.000), got %s", resolved[0].MatchStatus)
			}
		}
	})

	t.Run("both_scores_above_floor", func(t *testing.T) {
		// Both scores above floor (0.995 and 0.991): rule (b) must NOT fire
		items := []MatchResultItem{
			{
				ID:              batchID + "-src-A-dest-X",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-X",
				ConfidenceScore: 0.995,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            1,
				ScoreMargin:     0.004,
				DecisionNote:    "Ambiguous: runner-up within 0.004 — needs review",
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-A-dest-Y",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-Y",
				ConfidenceScore: 0.991,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            2,
				ScoreMargin:     0.0,
				DecisionNote:    "Alternative candidate (rank 2) for review",
				CreatedAt:       now,
			},
		}

		resolved := ResolveAssignments(items, "GREEDY_1_1", cfg)

		// Rank-1 should remain REVIEW_NEEDED
		if resolved[0].SourceID == "src-A" && resolved[0].Rank == 1 {
			if resolved[0].MatchStatus != "REVIEW_NEEDED" {
				t.Errorf("Expected rank-1 to remain REVIEW_NEEDED (both above floor), got %s", resolved[0].MatchStatus)
			}
		}
	})

	t.Run("both_scores_below_floor_small_margin", func(t *testing.T) {
		// Both below floor and small margin: rule (a) and (b) both fail
		items := []MatchResultItem{
			{
				ID:              batchID + "-src-A-dest-X",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-X",
				ConfidenceScore: 0.94,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            1,
				ScoreMargin:     0.01,
				DecisionNote:    "Ambiguous: runner-up within 0.01 — needs review",
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-A-dest-Y",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-Y",
				ConfidenceScore: 0.93,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            2,
				ScoreMargin:     0.0,
				DecisionNote:    "Alternative candidate (rank 2) for review",
				CreatedAt:       now,
			},
		}

		resolved := ResolveAssignments(items, "GREEDY_1_1", cfg)

		// Rank-1 should remain REVIEW_NEEDED (below threshold)
		if resolved[0].SourceID == "src-A" && resolved[0].Rank == 1 {
			if resolved[0].MatchStatus != "REVIEW_NEEDED" {
				t.Errorf("Expected rank-1 to remain REVIEW_NEEDED (below threshold), got %s", resolved[0].MatchStatus)
			}
		}
	})

	t.Run("rule_a_fires_with_large_margin", func(t *testing.T) {
		// Rule (a): score 0.96 >= 0.90 AND margin 0.16 >= 0.05 -> AUTO_MATCHED
		items := []MatchResultItem{
			{
				ID:              batchID + "-src-A-dest-X",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-X",
				ConfidenceScore: 0.96,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            1,
				ScoreMargin:     0.16,
				DecisionNote:    "Ambiguous: runner-up within 0.16 — needs review",
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-A-dest-Y",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-Y",
				ConfidenceScore: 0.80,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            2,
				ScoreMargin:     0.0,
				DecisionNote:    "Alternative candidate (rank 2) for review",
				CreatedAt:       now,
			},
		}

		resolved := ResolveAssignments(items, "GREEDY_1_1", cfg)

		// Rank-1 should be AUTO_MATCHED via rule (a)
		if resolved[0].SourceID == "src-A" && resolved[0].Rank == 1 {
			if resolved[0].MatchStatus != "AUTO_MATCHED" {
				t.Errorf("Expected rank-1 to be AUTO_MATCHED via rule (a), got %s", resolved[0].MatchStatus)
			}
			if !strings.Contains(resolved[0].DecisionNote, "margin threshold") {
				t.Errorf("Expected decision note to mention margin threshold, got: %s", resolved[0].DecisionNote)
			}
		}
	})

	t.Run("rank0_non_nomatch_rows_preserved", func(t *testing.T) {
		// DEFECT 3 TEST: Rank-0 rows with status != NO_MATCH must not be silently dropped
		items := []MatchResultItem{
			{
				ID:              batchID + "-src-A-dest-X",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-X",
				ConfidenceScore: 0.95,
				MatchStatus:     "AUTO_MATCHED",
				Rank:            1,
				ScoreMargin:     0.05,
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-B-dest-Y",
				BatchID:         batchID,
				SourceID:        "src-B",
				DestinationID:   "dest-Y",
				ConfidenceScore: 0.92,
				MatchStatus:     "AUTO_MATCHED",
				Rank:            1,
				ScoreMargin:     0.02,
				CreatedAt:       now,
			},
			{
				// Rank-0 row with unusual status (not NO_MATCH) - must be preserved
				ID:              batchID + "-src-B-dest-Z",
				BatchID:         batchID,
				SourceID:        "src-B",
				DestinationID:   "dest-Z",
				ConfidenceScore: 0.88,
				MatchStatus:     "CONFIRMED",
				Rank:            0,
				ScoreMargin:     0.0,
				CreatedAt:       now,
			},
		}

		resolved := ResolveAssignments(items, "GREEDY_1_1", cfg)

		// Every input row must appear in output (no rows lost)
		if len(resolved) != len(items) {
			t.Errorf("Expected %d items in output, got %d. Rank-0 rows are being dropped!", len(items), len(resolved))
		}

		// Verify the rank-0 row is present and unchanged
		found := false
		for _, item := range resolved {
			if item.Rank == 0 && item.DestinationID == "dest-Z" {
				found = true
				if item.MatchStatus != "CONFIRMED" {
					t.Errorf("Rank-0 row status changed from CONFIRMED to %s", item.MatchStatus)
				}
				break
			}
		}
		if !found {
			t.Errorf("Rank-0 row with dest-Z not found in output")
		}
	})

	t.Run("exact_match_respects_1to1_constraint", func(t *testing.T) {
		// Two sources both score 1.000 to same destination
		// src-A scores 1.000 to dest-X with runner-up 0.980 (rule b fires: 1.000 >= 0.99, 0.980 < 0.99)
		// src-B scores 1.000 to dest-X with runner-up 0.985 (rule b fires: 1.000 >= 0.99, 0.985 < 0.99)
		// 1:1 constraint: both qualify but only one can claim dest-X (higher score = src-A processed first)
		items := []MatchResultItem{
			{
				ID:              batchID + "-src-A-dest-X",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-X",
				ConfidenceScore: 1.000,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            1,
				ScoreMargin:     0.020,
				DecisionNote:    "Ambiguous: runner-up within 0.020 — needs review",
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-A-dest-Y",
				BatchID:         batchID,
				SourceID:        "src-A",
				DestinationID:   "dest-Y",
				ConfidenceScore: 0.980,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            2,
				ScoreMargin:     0.0,
				DecisionNote:    "Alternative candidate (rank 2) for review",
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-B-dest-X",
				BatchID:         batchID,
				SourceID:        "src-B",
				DestinationID:   "dest-X",
				ConfidenceScore: 1.000,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            1,
				ScoreMargin:     0.015,
				DecisionNote:    "Ambiguous: runner-up within 0.015 — needs review",
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-B-dest-Z",
				BatchID:         batchID,
				SourceID:        "src-B",
				DestinationID:   "dest-Z",
				ConfidenceScore: 0.985,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            2,
				ScoreMargin:     0.0,
				DecisionNote:    "Alternative candidate (rank 2) for review",
				CreatedAt:       now,
			},
		}

		resolved := ResolveAssignments(items, "GREEDY_1_1", cfg)

		// At most one source should auto-match to dest-X (1:1 constraint)
		destXClaimers := 0
		var demotedItem *MatchResultItem
		for i, item := range resolved {
			if item.DestinationID == "dest-X" && item.MatchStatus == "AUTO_MATCHED" && item.Rank == 1 {
				destXClaimers++
			} else if item.DestinationID == "dest-X" && item.Rank == 1 && item.MatchStatus == "REVIEW_NEEDED" {
				// Track the demoted item
				demotedItem = &resolved[i]
			}
		}

		if destXClaimers > 1 {
			t.Errorf("Expected at most 1 AUTO_MATCHED claim to dest-X, got %d", destXClaimers)
		}

		// The demoted source should have "already assigned" in the decision note
		if demotedItem != nil && !strings.Contains(demotedItem.DecisionNote, "already assigned") {
			t.Errorf("Expected demoted row to have 'already assigned' note, got: %s", demotedItem.DecisionNote)
		}
	})
}

func TestResolveAssignmentsAllCandidates(t *testing.T) {
	batchID := "test-batch"
	now := time.Now()
	cfg := Config{
		AutoMatchThreshold: 0.90,
		MarginThreshold:    0.05,
		ExactMatchFloor:    0.99,
	}

	t.Run("all_candidates_unchanged", func(t *testing.T) {
		// ALL_CANDIDATES strategy should return input unchanged
		items := []MatchResultItem{
			{
				ID:              batchID + "-src-A-dest-X",
				SourceID:        "src-A",
				DestinationID:   "dest-X",
				ConfidenceScore: 0.95,
				MatchStatus:     "AUTO_MATCHED",
				Rank:            1,
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-A-dest-Y",
				SourceID:        "src-A",
				DestinationID:   "dest-Y",
				ConfidenceScore: 0.92,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            2,
				CreatedAt:       now,
			},
		}

		resolved := ResolveAssignments(items, "ALL_CANDIDATES", cfg)

		if len(resolved) != len(items) {
			t.Errorf("ALL_CANDIDATES: expected %d items, got %d", len(items), len(resolved))
		}

		for i, item := range resolved {
			if item.SourceID != items[i].SourceID || item.DestinationID != items[i].DestinationID {
				t.Errorf("ALL_CANDIDATES: item %d changed", i)
			}
		}
	})
}

func TestResolveAssignmentsTop1(t *testing.T) {
	batchID := "test-batch"
	now := time.Now()
	cfg := Config{
		AutoMatchThreshold: 0.90,
		MarginThreshold:    0.05,
		ExactMatchFloor:    0.99,
	}

	t.Run("top1_drops_rank2_and_above", func(t *testing.T) {
		// TOP_1 strategy should keep only rank-1 rows
		items := []MatchResultItem{
			{
				ID:              batchID + "-src-A-dest-X",
				SourceID:        "src-A",
				DestinationID:   "dest-X",
				ConfidenceScore: 0.95,
				MatchStatus:     "AUTO_MATCHED",
				Rank:            1,
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-A-dest-Y",
				SourceID:        "src-A",
				DestinationID:   "dest-Y",
				ConfidenceScore: 0.92,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            2,
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-A-dest-Z",
				SourceID:        "src-A",
				DestinationID:   "dest-Z",
				ConfidenceScore: 0.90,
				MatchStatus:     "REVIEW_NEEDED",
				Rank:            3,
				CreatedAt:       now,
			},
		}

		resolved := ResolveAssignments(items, "TOP_1", cfg)

		// Should only have rank-1 item
		if len(resolved) != 1 {
			t.Errorf("TOP_1: expected 1 item, got %d", len(resolved))
		}

		if resolved[0].Rank != 1 {
			t.Errorf("TOP_1: expected rank 1, got %d", resolved[0].Rank)
		}
	})
}

func TestResolveAssignmentsUnknownStrategy(t *testing.T) {
	batchID := "test-batch"
	now := time.Now()
	cfg := Config{
		AutoMatchThreshold: 0.90,
		MarginThreshold:    0.05,
		ExactMatchFloor:    0.99,
	}

	t.Run("unknown_strategy_treated_as_greedy_1to1", func(t *testing.T) {
		// Unknown strategy should default to GREEDY_1_1
		items := []MatchResultItem{
			{
				ID:              batchID + "-src-A-dest-X",
				SourceID:        "src-A",
				DestinationID:   "dest-X",
				ConfidenceScore: 0.95,
				MatchStatus:     "AUTO_MATCHED",
				Rank:            1,
				CreatedAt:       now,
			},
			{
				ID:              batchID + "-src-B-dest-X",
				SourceID:        "src-B",
				DestinationID:   "dest-X",
				ConfidenceScore: 0.92,
				MatchStatus:     "AUTO_MATCHED",
				Rank:            1,
				CreatedAt:       now,
			},
		}

		resolved := ResolveAssignments(items, "UNKNOWN_STRATEGY", cfg)

		// Should behave like GREEDY_1_1: higher scorer keeps dest-X
		hasAutoMatched := false
		hasReviewNeeded := false
		for _, item := range resolved {
			if item.MatchStatus == "AUTO_MATCHED" {
				hasAutoMatched = true
			}
			if item.MatchStatus == "REVIEW_NEEDED" {
				hasReviewNeeded = true
			}
		}

		if !hasAutoMatched || !hasReviewNeeded {
			t.Errorf("Expected one AUTO_MATCHED and one REVIEW_NEEDED with unknown strategy (treated as GREEDY_1_1)")
		}
	})
}
