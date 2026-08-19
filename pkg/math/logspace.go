package math

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrEmptySlice is returned when an operation requires at least one element.
	ErrEmptySlice = errors.New("pkg/math: input slice must not be empty")
	// ErrInvalidLogDomain is returned when a log-space difference would result in taking the log of a non-positive number.
	ErrInvalidLogDomain = errors.New("pkg/math: LogSubExp requires minuend >= subtrahend (a >= b)")
	// ErrZeroProbabilityMass is returned when all log-probabilities are -Inf, making normalization impossible.
	ErrZeroProbabilityMass = errors.New("pkg/math: total probability mass is zero (-Inf in log-space)")
)

// LogSumExp computes ln(exp(a) + exp(b)) in a numerically stable manner without overflowing.
//
// Mathematical Formulation:
// Let m = max(a, b). Then:
//
//	ln(exp(a) + exp(b)) = m + ln(1 + exp(-|a - b|)) = m + math.Log1p(exp(-|a - b|))
//
// Special cases:
//   - If both a and b are -Inf, returns -Inf.
//   - If a is -Inf, returns b; if b is -Inf, returns a.
//   - If either operand is NaN, returns NaN.
func LogSumExp(a, b float64) float64 {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.NaN()
	}
	if a == math.Inf(-1) {
		return b
	}
	if b == math.Inf(-1) {
		return a
	}
	if a == math.Inf(1) || b == math.Inf(1) {
		return math.Inf(1)
	}

	maxVal := a
	diff := b - a
	if b > a {
		maxVal = b
		diff = a - b
	}

	// diff is non-positive: diff <= 0
	return maxVal + math.Log1p(math.Exp(diff))
}

// LogSumExpSlice computes ln(\sum_{i} exp(xs[i])) across a slice of log-space values.
//
// If the slice is empty, LogSumExpSlice returns -Inf.
// If all elements are -Inf, it returns -Inf.
func LogSumExpSlice(xs []float64) float64 {
	if len(xs) == 0 {
		return math.Inf(-1)
	}

	// Find the maximum value to center the exponentiation
	maxVal := math.Inf(-1)
	for _, x := range xs {
		if math.IsNaN(x) {
			return math.NaN()
		}
		if x > maxVal {
			maxVal = x
		}
	}

	if maxVal == math.Inf(-1) || maxVal == math.Inf(1) {
		return maxVal
	}

	sumExp := 0.0
	for _, x := range xs {
		if x != math.Inf(-1) {
			sumExp += math.Exp(x - maxVal)
		}
	}

	return maxVal + math.Log(sumExp)
}

// LogSubExp computes ln(exp(a) - exp(b)) for a >= b.
//
// Mathematical Formulation:
// For a > b:
//
//	ln(exp(a) - exp(b)) = a + ln(1 - exp(b - a)) = a + math.Log1p(-math.Exp(b - a))
//
// Special cases:
//   - If a == b, returns -Inf (since ln(0) = -Inf).
//   - If b is -Inf, returns a.
//   - If a < b, returns NaN and ErrInvalidLogDomain.
func LogSubExp(a, b float64) (float64, error) {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.NaN(), nil
	}
	if a < b {
		return math.NaN(), fmt.Errorf("%w: a=%f < b=%f", ErrInvalidLogDomain, a, b)
	}
	if a == b {
		return math.Inf(-1), nil
	}
	if b == math.Inf(-1) {
		return a, nil
	}

	// b < a, so (b - a) is strictly negative and exp(b - a) in (0, 1)
	diff := b - a
	return a + math.Log1p(-math.Exp(diff)), nil
}

// LogNormalize takes unnormalized log-probabilities ln(\tilde{p}_i) and returns a freshly
// allocated slice of normalized linear probabilities p_i on the simplex (\sum p_i = 1.0).
//
// Mathematical Guarantee:
// Each probability is calculated as:
//
//	p_i = exp(logProbs[i] - LogSumExpSlice(logProbs))
//
// In accordance with Inviolate 5 (State Immutability), the input slice is never modified.
// If all values are -Inf or the slice is empty, LogNormalize returns an error.
func LogNormalize(logProbs []float64) ([]float64, error) {
	if len(logProbs) == 0 {
		return nil, ErrEmptySlice
	}

	totalLogMass := LogSumExpSlice(logProbs)
	if totalLogMass == math.Inf(-1) {
		return nil, ErrZeroProbabilityMass
	}
	if math.IsNaN(totalLogMass) {
		return nil, errors.New("pkg/math: logProbs contains NaN values")
	}

	probs := make([]float64, len(logProbs))
	for i, lp := range logProbs {
		if lp == math.Inf(-1) {
			probs[i] = 0.0
		} else {
			probs[i] = math.Exp(lp - totalLogMass)
		}
	}

	return probs, nil
}
