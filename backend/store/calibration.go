package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"entitymatcher/matcher"
)

// CalibrationModel is a persisted, fitted Calibrator plus the metadata needed to
// audit and reproduce it. Rows are append-only: a new fit always inserts a new row
// rather than mutating an existing one's model_json/metrics. The `Active` flag is the
// one mutable piece of state — it tracks which single model is currently wired into
// the matching engine, and flipping it is a normal, expected operation (deactivating
// the previous active model when a new one is promoted), not a rewrite of history.
type CalibrationModel struct {
	ID               string    `json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	FittedBy         string    `json:"fitted_by"`
	BatchID          string    `json:"batch_id"`
	ObservationCount int       `json:"observation_count"`
	PositiveCount    int       `json:"positive_count"`
	BrierScore       float64   `json:"brier_score"`
	ECEScore         float64   `json:"ece_score"`
	ModelJSON        string    `json:"model_json"`
	Active           bool      `json:"active"`
}

// CalibrationObservationStats summarizes the composition of a CalibrationObservations
// training set, broken down by the PreviousStatus of the underlying audit log entries,
// so an operator can see at a glance how skewed the sample is toward the review queue.
type CalibrationObservationStats struct {
	Total            int            `json:"total"`
	Positive         int            `json:"positive"`
	Negative         int            `json:"negative"`
	ByPreviousStatus map[string]int `json:"by_previous_status"`
}

// auditEntryLabel maps a single audit log entry's Action/NewStatus to a training
// label for the calibrator. Returns ok=false if the entry carries no usable label.
//
// Mapping rules (deliberately conservative — see doc comment on
// computeCalibrationObservations for why):
//   - Action == "CONFIRM" -> IsMatch=true, ok=true
//   - Action == "REJECT"  -> IsMatch=false, ok=true
//   - Action == "OVERRIDE": ambiguous on its own. Decide from NewStatus:
//     NewStatus == "CONFIRMED" -> IsMatch=true, ok=true
//     NewStatus == "REJECTED"  -> IsMatch=false, ok=true
//     anything else            -> ok=false (skip; we cannot tell what the reviewer decided)
//   - Any other Action (e.g. "UNLINK", or anything not explicitly listed here) -> ok=false.
//     This is a deliberate, documented choice: the AuditLogEntry.Action field's documented
//     values are CONFIRM/REJECT/OVERRIDE, and we only trust those three to carry an
//     unambiguous ground-truth label. If a caller wants other actions folded in later,
//     that mapping should be added here explicitly rather than guessed at.
func auditEntryLabel(entry AuditLogEntry) (isMatch bool, ok bool) {
	switch entry.Action {
	case "CONFIRM":
		return true, true
	case "REJECT":
		return false, true
	case "OVERRIDE":
		switch entry.NewStatus {
		case "CONFIRMED":
			return true, true
		case "REJECTED":
			return false, true
		default:
			return false, false
		}
	default:
		return false, false
	}
}

// computeCalibrationObservations turns raw audit log entries into a deduplicated,
// labelled training set for FitCalibrator, plus a stats breakdown for surfacing
// selection bias to operators.
//
// SELECTION BIAS WARNING: auto-matched pairs are typically never reviewed, so they
// generate no audit entry and therefore no label. This training set is drawn almost
// entirely from the human review queue, which over-represents ambiguous mid-range
// scores and under-represents confident correct matches. A calibrator fitted on it
// will be well-calibrated for the review band and is extrapolating everywhere else.
// This function does not attempt to correct that bias — it only makes it visible via
// the returned CalibrationObservationStats.ByPreviousStatus breakdown.
//
// Deduplication: entries are grouped by (BatchID, SourceID, DestinationID). Within each
// group, only entries with a usable label (see auditEntryLabel) are considered, and the
// one with the LATEST Timestamp wins — so a pair reviewed twice contributes exactly one
// observation, and a later reversal (e.g. CONFIRM then REJECT) overrides the earlier one.
func computeCalibrationObservations(entries []AuditLogEntry) ([]matcher.LabelledScore, CalibrationObservationStats) {
	if len(entries) == 0 {
		return []matcher.LabelledScore{}, CalibrationObservationStats{ByPreviousStatus: map[string]int{}}
	}

	// Build deduplication map: key -> winner entry
	type entryKey struct {
		batchID  string
		sourceID string
		destID   string
	}
	winnerMap := make(map[entryKey]AuditLogEntry)

	for _, entry := range entries {
		if _, ok := auditEntryLabel(entry); !ok {
			continue
		}

		key := entryKey{batchID: entry.BatchID, sourceID: entry.SourceID, destID: entry.DestinationID}
		existing, exists := winnerMap[key]

		// If no existing winner, or this entry is strictly later, replace
		if !exists || entry.Timestamp.After(existing.Timestamp) {
			winnerMap[key] = entry
		}
	}

	// Extract winners and prepare results
	winners := make([]AuditLogEntry, 0, len(winnerMap))
	for _, entry := range winnerMap {
		winners = append(winners, entry)
	}

	// Sort winners by key for deterministic output
	sort.Slice(winners, func(i, j int) bool {
		ki := entryKey{batchID: winners[i].BatchID, sourceID: winners[i].SourceID, destID: winners[i].DestinationID}
		kj := entryKey{batchID: winners[j].BatchID, sourceID: winners[j].SourceID, destID: winners[j].DestinationID}

		if ki.batchID != kj.batchID {
			return ki.batchID < kj.batchID
		}
		if ki.sourceID != kj.sourceID {
			return ki.sourceID < kj.sourceID
		}
		return ki.destID < kj.destID
	})

	// Convert winners to labels and compute stats
	labels := make([]matcher.LabelledScore, 0, len(winners))
	stats := CalibrationObservationStats{
		ByPreviousStatus: make(map[string]int),
	}

	for _, entry := range winners {
		label, ok := auditEntryLabel(entry)
		if !ok {
			continue
		}

		labels = append(labels, matcher.LabelledScore{Score: entry.ConfidenceScore, IsMatch: label})

		stats.Total++
		if label {
			stats.Positive++
		} else {
			stats.Negative++
		}

		stats.ByPreviousStatus[entry.PreviousStatus]++
	}

	return labels, stats
}

// UnmarshalCalibrator reconstructs a matcher.Calibrator from its JSON representation
// (as produced by json.Marshal on a matcher.Calibrator value), by first peeking at the
// "type" discriminator field to pick the concrete type, then delegating to that type's
// own UnmarshalJSON.
func UnmarshalCalibrator(data []byte) (matcher.Calibrator, error) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("unmarshal calibrator: %w", err)
	}
	switch probe.Type {
	case "identity":
		c := &matcher.IdentityCalibrator{}
		if err := c.UnmarshalJSON(data); err != nil {
			return nil, fmt.Errorf("unmarshal identity calibrator: %w", err)
		}
		return c, nil
	case "platt":
		c := &matcher.PlattCalibrator{}
		if err := c.UnmarshalJSON(data); err != nil {
			return nil, fmt.Errorf("unmarshal platt calibrator: %w", err)
		}
		return c, nil
	case "isotonic":
		c := &matcher.IsotonicCalibrator{}
		if err := c.UnmarshalJSON(data); err != nil {
			return nil, fmt.Errorf("unmarshal isotonic calibrator: %w", err)
		}
		return c, nil
	default:
		return nil, fmt.Errorf("unmarshal calibrator: unknown type %q", probe.Type)
	}
}
