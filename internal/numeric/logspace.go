package numeric

import (
	"errors"
	"math"
)

var (
	// ErrLogDomain is returned when a log-space operation receives arguments outside its mathematical domain.
	ErrLogDomain = errors.New("numeric: log domain error")
)

// LogSumExp computes log(sum_i exp(x_i)) in a numerically stable way.
// It subtracts the maximum value to prevent overflow and underflow:
//
//	LSE(x) = max(x) + log(sum_i exp(x_i - max(x)))
//
// Edge case semantics:
//   - If x is empty, returns -Inf.
//   - If any element is NaN, returns NaN.
//   - If any element is +Inf, returns +Inf.
//   - If all elements are -Inf, returns -Inf.
func LogSumExp(x []float64) float64 {
	if len(x) == 0 {
		return math.Inf(-1)
	}
	if len(x) == 1 {
		return x[0]
	}

	maxVal := math.Inf(-1)
	hasNaN := false
	hasPosInf := false

	for _, v := range x {
		if math.IsNaN(v) {
			hasNaN = true
		} else if math.IsInf(v, 1) {
			hasPosInf = true
		} else if v > maxVal {
			maxVal = v
		}
	}

	if hasNaN {
		return math.NaN()
	}
	if hasPosInf {
		return math.Inf(1)
	}
	if math.IsInf(maxVal, -1) {
		return math.Inf(-1)
	}

	var sumExp float64
	for _, v := range x {
		if !math.IsInf(v, -1) {
			sumExp += math.Exp(v - maxVal)
		}
	}

	return maxVal + math.Log(sumExp)
}

// LogAdd computes log(exp(a) + exp(b)) in a numerically stable way using math.Log1p.
//
//	LogAdd(a, b) = max(a, b) + log1p(exp(-|a - b|))
func LogAdd(a, b float64) float64 {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.NaN()
	}
	if math.IsInf(a, 1) || math.IsInf(b, 1) {
		return math.Inf(1)
	}
	if math.IsInf(a, -1) {
		return b
	}
	if math.IsInf(b, -1) {
		return a
	}

	if a > b {
		return a + math.Log1p(math.Exp(b-a))
	}
	return b + math.Log1p(math.Exp(a-b))
}

// LogSub computes log(exp(a) - exp(b)) in a numerically stable way for a >= b.
//
//	LogSub(a, b) = a + log1p(-exp(b - a))
//
// Returns an error if a < b (logarithm of a negative number).
// If a == b, returns -Inf.
// If b == -Inf, returns a.
func LogSub(a, b float64) (float64, error) {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.NaN(), ErrLogDomain
	}
	if a < b {
		return math.NaN(), ErrLogDomain
	}
	if a == b {
		return math.Inf(-1), nil
	}
	if math.IsInf(b, -1) {
		return a, nil
	}
	if math.IsInf(a, 1) {
		return math.Inf(1), nil
	}

	diff := b - a // diff <= 0
	// Using -math.Expm1(diff) is equivalent to 1 - exp(diff)
	return a + math.Log(-math.Expm1(diff)), nil
}
