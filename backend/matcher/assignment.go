package matcher

import (
	"fmt"
	"sort"
)

// ResolveAssignments enforces a one-to-one constraint over proposed matches.
func ResolveAssignments(items []MatchResultItem, strategy string, cfg Config) []MatchResultItem {
	// Handle different strategies
	switch strategy {
	case "ALL_CANDIDATES":
		return items
	case "TOP_1":
		return resolveTop1(items)
	default: // Includes "GREEDY_1_1" and any unknown strategy
		return resolveGreedy1to1(items, cfg)
	}
}

// resolveGreedy1to1 implements the GREEDY_1_1 strategy
func resolveGreedy1to1(items []MatchResultItem, cfg Config) []MatchResultItem {
	// DEFECT 1 FIX: Build rank-2 score map in a single O(n) pass instead of nested loop
	// This prevents O(n^2) performance regression from 7,841 to 1,645 sources/sec
	rank2ScoreMap := make(map[string]float64) // sourceID -> rank-2 ConfidenceScore
	for _, item := range items {
		if item.Rank == 2 {
			// Store rank-2 score for each source (last one wins, but all rank-2 scores are the same per source)
			rank2ScoreMap[item.SourceID] = item.ConfidenceScore
		}
	}

	// Re-evaluate all rank-1 items for auto-matchability and track them for 1:1 constraint
	var rank1Items []MatchResultItem
	nonRank1Items := make([]MatchResultItem, 0)

	for _, item := range items {
		if item.Rank == 1 {
			// Re-evaluate if this rank-1 item qualifies for auto-matching
			// using the same rules as the initial decision
			// DEFECT 1 FIX: O(1) lookup instead of nested loop
			runnerUpScore := rank2ScoreMap[item.SourceID]

			canAutoMatch, decisionNote := IsAutoMatchable(
				item.ConfidenceScore,
				runnerUpScore,
				cfg.AutoThresholdFor(item.CrossScript),
				cfg.MarginThreshold,
				cfg.ExactMatchFloor,
			)

			// Update the item's decision based on isAutoMatchable
			itemCopy := item
			if canAutoMatch {
				itemCopy.MatchStatus = "AUTO_MATCHED"
			} else {
				itemCopy.MatchStatus = "REVIEW_NEEDED"
			}
			itemCopy.DecisionNote = decisionNote
			rank1Items = append(rank1Items, itemCopy)
		} else {
			// DEFECT 3 FIX: Keep ALL non-rank-1 items (including rank-0 with any status)
			// This ensures no input rows are lost
			nonRank1Items = append(nonRank1Items, item)
		}
	}

	// Sort rank-1 items by ConfidenceScore descending (ties broken by sourceID then destID)
	// This ensures consistent processing order for 1:1 constraint
	sort.SliceStable(rank1Items, func(i, j int) bool {
		if rank1Items[i].ConfidenceScore != rank1Items[j].ConfidenceScore {
			return rank1Items[i].ConfidenceScore > rank1Items[j].ConfidenceScore
		}
		if rank1Items[i].SourceID != rank1Items[j].SourceID {
			return rank1Items[i].SourceID < rank1Items[j].SourceID
		}
		return rank1Items[i].DestinationID < rank1Items[j].DestinationID
	})

	// DEFECT 2 FIX: Track both source ID and score for claimed destinations
	// Prevents reporting loser's score instead of winner's score in demotion note
	type claimedDest struct {
		SourceID string
		Score    float64
	}
	claimedDestinations := make(map[string]claimedDest) // destination ID -> claimedDest{sourceID, score}

	// Process rank-1 items in order: enforce 1:1 constraint
	// Items that would violate the constraint are demoted to REVIEW_NEEDED
	for i := range rank1Items {
		item := &rank1Items[i]
		if item.MatchStatus == "AUTO_MATCHED" {
			// This item qualifies for auto-matching, check 1:1 constraint
			if claimed, exists := claimedDestinations[item.DestinationID]; exists {
				// Destination already claimed by a higher-scoring source, demote to REVIEW_NEEDED
				item.MatchStatus = "REVIEW_NEEDED"
				// DEFECT 2 FIX: Report the WINNER's score, not the loser's
				item.DecisionNote = fmt.Sprintf("Destination already assigned to %s at %.3f", claimed.SourceID, claimed.Score)
			} else {
				// Claim this destination (keep as AUTO_MATCHED)
				claimedDestinations[item.DestinationID] = claimedDest{
					SourceID: item.SourceID,
					Score:    item.ConfidenceScore,
				}
			}
		}
	}

	// Combine results: rank-1 items first, then non-rank-1 items
	result := append(rank1Items, nonRank1Items...)
	return result
}

// resolveTop1 implements the TOP_1 strategy
func resolveTop1(items []MatchResultItem) []MatchResultItem {
	var result []MatchResultItem
	for _, item := range items {
		if item.Rank == 1 {
			// Keep rank-1 rows as proposed
			result = append(result, item)
		}
		// Drop all rank >= 2 rows entirely
	}
	return result
}
