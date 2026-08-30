package matcher

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

const FitThresholdObservations = 500

// LabelledScore represents a scored match with its known true label.
type LabelledScore struct {
	Score  float64 `json:"score"`  // Raw confidence score (0.0–1.0)
	IsMatch bool   `json:"is_match"` // True if the pair is a true match (ground truth)
}

// Calibrator transforms raw scores into calibrated probabilities.
type Calibrator interface {
	// Calibrate returns calibrated probability in [0.0, 1.0].
	Calibrate(score float64) float64
}

// IdentityCalibrator returns the raw score unchanged.
type IdentityCalibrator struct{}

func (c *IdentityCalibrator) Calibrate(score float64) float64 {
	if score < 0.0 {
		score = 0.0
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func (c *IdentityCalibrator) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": "identity",
	})
}

func (c *IdentityCalibrator) UnmarshalJSON(data []byte) error {
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if m["type"] != "identity" {
		return fmt.Errorf("invalid type for IdentityCalibrator: %s", m["type"])
	}
	return nil
}

// PlattCalibrator implements Platt scaling (logistic regression) on scores.
// Model: p = 1 / (1 + exp(A * score + B))
type PlattCalibrator struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

func (c *PlattCalibrator) Calibrate(score float64) float64 {
	if score < 0.0 {
		score = 0.0
	}
	if score > 1.0 {
		score = 1.0
	}

	// Clamp to avoid overflow in exp
	linear := c.A*score + c.B
	if linear > 700 {
		linear = 700
	}
	if linear < -700 {
		linear = -700
	}

	prob := 1.0 / (1.0 + math.Exp(linear))
	return prob
}

func (c *PlattCalibrator) MarshalJSON() ([]byte, error) {
	data := map[string]interface{}{
		"type": "platt",
		"a":    c.A,
		"b":    c.B,
	}
	return json.Marshal(data)
}

func (c *PlattCalibrator) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if t, ok := m["type"].(string); !ok || t != "platt" {
		return fmt.Errorf("invalid type for PlattCalibrator")
	}
	if a, ok := m["a"].(float64); ok {
		c.A = a
	}
	if b, ok := m["b"].(float64); ok {
		c.B = b
	}
	return nil
}

// IsotonicCalibrator implements non-parametric isotonic regression via PAV.
// It maps sorted unique raw scores to increasing calibrated probabilities.
type IsotonicCalibrator struct {
	Scores   []float64 `json:"scores"`  // Monotonically increasing raw scores
	Probs    []float64 `json:"probs"`   // Monotonically increasing calibrated probabilities
	minScore float64   // min(Scores)
	maxScore float64   // max(Scores)
}

func (c *IsotonicCalibrator) Calibrate(score float64) float64 {
	if len(c.Scores) == 0 || len(c.Probs) == 0 {
		return score
	}

	if score <= c.minScore {
		return c.Probs[0]
	}
	if score >= c.maxScore {
		return c.Probs[len(c.Probs)-1]
	}

	// Binary search for rank
	idx := sort.Search(len(c.Scores), func(i int) bool {
		return c.Scores[i] >= score
	})

	// If exact match
	if idx < len(c.Scores) && c.Scores[idx] == score {
		return c.Probs[idx]
	}

	// Linear interpolation between neighbors
	if idx == 0 {
		return c.Probs[0]
	}
	if idx >= len(c.Scores) {
		return c.Probs[len(c.Probs)-1]
	}

	s1, s2 := c.Scores[idx-1], c.Scores[idx]
	p1, p2 := c.Probs[idx-1], c.Probs[idx]
	t := (score - s1) / (s2 - s1)
	return p1 + t*(p2-p1)
}

func (c *IsotonicCalibrator) MarshalJSON() ([]byte, error) {
	data := map[string]interface{}{
		"type":   "isotonic",
		"scores": c.Scores,
		"probs":  c.Probs,
	}
	return json.Marshal(data)
}

func (c *IsotonicCalibrator) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if t, ok := m["type"].(string); !ok || t != "isotonic" {
		return fmt.Errorf("invalid type for IsotonicCalibrator")
	}

	if scoresRaw, ok := m["scores"].([]interface{}); ok {
		c.Scores = make([]float64, len(scoresRaw))
		for i, v := range scoresRaw {
			if f, ok := v.(float64); ok {
				c.Scores[i] = f
			}
		}
	}
	if probsRaw, ok := m["probs"].([]interface{}); ok {
		c.Probs = make([]float64, len(probsRaw))
		for i, v := range probsRaw {
			if f, ok := v.(float64); ok {
				c.Probs[i] = f
			}
		}
	}

	if len(c.Scores) > 0 {
		c.minScore = c.Scores[0]
		c.maxScore = c.Scores[len(c.Scores)-1]
	}

	return nil
}

// FitCalibrator fits a calibrator on labelled scores, choosing between Platt and Isotonic based on count.
// Platt is used if len(observations) < FitThresholdObservations, otherwise Isotonic.
// Returns an error if there are insufficient observations or only one class present.
func FitCalibrator(obs []LabelledScore) (Calibrator, error) {
	n := len(obs)
	if n == 0 {
		return nil, fmt.Errorf("FitCalibrator: zero observations")
	}

	// Count positive and negative examples
	var nPos, nNeg int
	for _, o := range obs {
		if o.IsMatch {
			nPos++
		} else {
			nNeg++
		}
	}

	// Error if only one class present
	if nPos == 0 || nNeg == 0 {
		return nil, fmt.Errorf("FitCalibrator: only one class present (pos=%d, neg=%d)", nPos, nNeg)
	}

	if n < FitThresholdObservations {
		return fitPlatt(obs), nil
	}
	return fitIsotonic(obs), nil
}

// fitPlatt fits a Platt scaling model using Newton–Raphson to maximize likelihood.
func fitPlatt(labels []LabelledScore) *PlattCalibrator {
	// Compute targets
	var nPos, nNeg int
	for _, ls := range labels {
		if ls.IsMatch {
			nPos++
		} else {
			nNeg++
		}
	}

	if nPos == 0 || nNeg == 0 {
		return &PlattCalibrator{A: 0, B: 0}
	}

	// Initialize with simple heuristics
	a, b := 0.0, 0.0

	// Compute targets with smoothing for numerical stability
	targetPos := (float64(nPos) + 1.0) / (float64(nPos) + 2.0)
	targetNeg := 1.0 / (float64(nNeg) + 2.0)

	// Simple Newton iterations (few steps suffice for calibration)
	for iter := 0; iter < 10; iter++ {
		gradA, gradB := 0.0, 0.0
		hessAA, hessBB, hessAB := 0.0, 0.0, 0.0

		for _, ls := range labels {
			linear := a*ls.Score + b
			// Clamp to avoid overflow
			if linear > 700 {
				linear = 700
			}
			if linear < -700 {
				linear = -700
			}

			expLinear := math.Exp(linear)
			p := 1.0 / (1.0 + expLinear)

			target := targetPos
			if !ls.IsMatch {
				target = targetNeg
			}

			diff := p - target
			weight := expLinear / ((1.0 + expLinear) * (1.0 + expLinear))

			gradA += diff * ls.Score
			gradB += diff
			hessAA += weight * ls.Score * ls.Score
			hessBB += weight
			hessAB += weight * ls.Score
		}

		// Newton step: solve Hessian * delta = gradient
		det := hessAA*hessBB - hessAB*hessAB
		if math.Abs(det) < 1e-10 {
			break
		}

		deltaA := (hessBB*gradA - hessAB*gradB) / det
		deltaB := (-hessAB*gradA + hessAA*gradB) / det

		// Adaptive learning rate (backtracking line search)
		lambda := 1.0
		for lambda > 1e-10 {
			newA := a - lambda*deltaA
			newB := b - lambda*deltaB

			// Evaluate loss at new point
			newLoss := 0.0
			for _, ls := range labels {
				linear := newA*ls.Score + newB
				if linear > 700 {
					linear = 700
				}
				if linear < -700 {
					linear = -700
				}
				expLinear := math.Exp(linear)
				p := 1.0 / (1.0 + expLinear)

				// Negative log-likelihood
				if ls.IsMatch {
					newLoss -= math.Log(p + 1e-15)
				} else {
					newLoss -= math.Log(1.0-p + 1e-15)
				}
			}

			// Accept step if loss improves
			oldLoss := 0.0
			for _, ls := range labels {
				linear := a*ls.Score + b
				if linear > 700 {
					linear = 700
				}
				if linear < -700 {
					linear = -700
				}
				expLinear := math.Exp(linear)
				p := 1.0 / (1.0 + expLinear)

				if ls.IsMatch {
					oldLoss -= math.Log(p + 1e-15)
				} else {
					oldLoss -= math.Log(1.0-p + 1e-15)
				}
			}

			if newLoss <= oldLoss {
				a = newA
				b = newB
				break
			}
			lambda *= 0.5
		}
	}

	// Clip to reasonable range
	if a > 10 {
		a = 10
	}
	if a < -10 {
		a = -10
	}
	if b > 10 {
		b = 10
	}
	if b < -10 {
		b = -10
	}

	return &PlattCalibrator{A: a, B: b}
}

// fitIsotonic fits an isotonic regression model using Pool Adjacent Violators (PAV).
func fitIsotonic(labels []LabelledScore) *IsotonicCalibrator {
	if len(labels) == 0 {
		return &IsotonicCalibrator{Scores: []float64{0}, Probs: []float64{0}, minScore: 0, maxScore: 0}
	}

	// Sort by score ascending
	sort.Slice(labels, func(i, j int) bool {
		if labels[i].Score == labels[j].Score {
			return !labels[i].IsMatch // Put non-matches before matches for stability
		}
		return labels[i].Score < labels[j].Score
	})

	// Group by unique score and count positives/negatives in each group
	type point struct {
		score    float64
		posCount float64
		negCount float64
	}

	var points []point
	for i := 0; i < len(labels); {
		score := labels[i].Score
		var pos, neg float64
		j := i
		for j < len(labels) && labels[j].Score == score {
			if labels[j].IsMatch {
				pos++
			} else {
				neg++
			}
			j++
		}
		points = append(points, point{score: score, posCount: pos, negCount: neg})
		i = j
	}

	// Initialize blocks: one block per point
	type block struct {
		posSum float64
		negSum float64
		idx    int // Starting index in points
		len    int // Number of points in this block
	}
	blocks := make([]block, len(points))
	for i := range blocks {
		blocks[i] = block{
			posSum: points[i].posCount,
			negSum: points[i].negCount,
			idx:    i,
			len:    1,
		}
	}

	// PAVA: merge adjacent blocks that violate monotonicity
	for {
		merged := false
		i := 0
		for i < len(blocks)-1 {
			p1 := blocks[i].posSum / (blocks[i].posSum + blocks[i].negSum)
			p2 := blocks[i+1].posSum / (blocks[i+1].posSum + blocks[i+1].negSum)

			if p1 > p2+1e-9 { // Violation with small tolerance
				// Merge blocks i and i+1
				blocks[i].posSum += blocks[i+1].posSum
				blocks[i].negSum += blocks[i+1].negSum
				blocks[i].len += blocks[i+1].len
				blocks = append(blocks[:i+1], blocks[i+2:]...)
				merged = true
			} else {
				i++
			}
		}
		if !merged {
			break
		}
	}

	// Extract the final scores and probabilities
	scores := make([]float64, len(blocks))
	probs := make([]float64, len(blocks))
	for i, blk := range blocks {
		// Use the last score in the block
		lastIdx := blk.idx + blk.len - 1
		scores[i] = points[lastIdx].score

		total := blk.posSum + blk.negSum
		if total == 0 {
			probs[i] = 0.5
		} else {
			probs[i] = blk.posSum / total
		}
	}

	minS := scores[0]
	maxS := scores[len(scores)-1]

	return &IsotonicCalibrator{
		Scores:   scores,
		Probs:    probs,
		minScore: minS,
		maxScore: maxS,
	}
}

// IsMonotonic checks that a calibrator preserves the order of scores (i.e., calibration doesn't reorder).
func IsMonotonic(scores []float64, calibrator Calibrator) bool {
	if len(scores) < 2 {
		return true
	}

	calibrated := make([]float64, len(scores))
	for i, s := range scores {
		calibrated[i] = calibrator.Calibrate(s)
	}

	// Check that calibrated scores are monotonically increasing
	for i := 0; i < len(calibrated)-1; i++ {
		if calibrated[i] > calibrated[i+1]+1e-9 { // Small tolerance for floating point
			return false
		}
	}

	return true
}
