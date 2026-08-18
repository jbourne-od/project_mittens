package numeric

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrZeroProbabilityMass is returned when the sum of unnormalized probabilities is zero or negative.
	ErrZeroProbabilityMass = errors.New("numeric: total probability mass is zero or negative")
)

// KahanSum computes the sum of a float64 slice using Neumaier's algorithm
// (an improved variant of Kahan compensated summation).
// It significantly reduces floating-point roundoff error when accumulating large slices
// or numbers with disparate scales.
func KahanSum(vals []float64) float64 {
	if len(vals) == 0 {
		return 0.0
	}
	if len(vals) == 1 {
		return vals[0]
	}

	sum := vals[0]
	c := 0.0 // Lost low-order bits compensation

	for i := 1; i < len(vals); i++ {
		v := vals[i]
		t := sum + v
		if math.Abs(sum) >= math.Abs(v) {
			c += (sum - t) + v // If sum is bigger, low-order digits of v are lost.
		} else {
			c += (v - t) + sum // If v is bigger, low-order digits of sum are lost.
		}
		sum = t
	}

	return sum + c
}

// NormalizeProbabilities normalizes a non-negative slice of weights so that its elements sum to 1.0.
//
// Arguments:
//   - dst: Destination buffer to write normalized probabilities. If dst is nil or len(dst) != len(src),
//     a new slice of length len(src) is allocated and returned.
//     Aliasing (dst == src) is fully supported and performs in-place normalization.
//   - src: Slice of non-negative unnormalized weights.
//
// Returns:
//   - The slice containing normalized probabilities, or an error if:
//   - src is empty,
//   - any element in src is negative, NaN, or infinite,
//   - the total mass sum(src) is zero or non-finite.
func NormalizeProbabilities(dst, src []float64) ([]float64, error) {
	if len(src) == 0 {
		return nil, ErrEmptySlice
	}

	for i, v := range src {
		if math.IsNaN(v) {
			return nil, fmt.Errorf("%w: src[%d] is NaN", ErrNaN, i)
		}
		if math.IsInf(v, 0) {
			return nil, fmt.Errorf("%w: src[%d] is Inf", ErrInf, i)
		}
		if v < 0 {
			return nil, fmt.Errorf("%w: src[%d] = %g < 0", ErrNegative, i, v)
		}
	}

	totalMass := KahanSum(src)
	if totalMass <= 0.0 || math.IsNaN(totalMass) || math.IsInf(totalMass, 0) {
		return nil, fmt.Errorf("%w: sum is %g", ErrZeroProbabilityMass, totalMass)
	}

	if dst == nil || len(dst) != len(src) {
		dst = make([]float64, len(src))
	}

	invMass := 1.0 / totalMass
	for i := range src {
		dst[i] = src[i] * invMass
	}

	return dst, nil
}

// Softmax computes the softmax distribution exp(src[i]) / sum_j exp(src[j]) in a numerically stable way.
//
// Arguments:
//   - dst: Destination buffer. If dst is nil or len(dst) != len(src), a new slice is allocated.
//     Aliasing (dst == src) is fully supported.
//   - src: Input logits.
func Softmax(dst, src []float64) []float64 {
	if len(src) == 0 {
		return nil
	}
	if dst == nil || len(dst) != len(src) {
		dst = make([]float64, len(src))
	}

	lse := LogSumExp(src)
	if math.IsNaN(lse) {
		for i := range dst {
			dst[i] = math.NaN()
		}
		return dst
	}

	for i, v := range src {
		dst[i] = math.Exp(v - lse)
	}

	return dst
}
