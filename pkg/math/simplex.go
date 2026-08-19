package math

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

var (
	// ErrSimplexEmpty is returned when an empty probability slice is passed to simplex validation.
	ErrSimplexEmpty = errors.New("pkg/math: probability vector is empty")
	// ErrSimplexNegative is returned when a probability element is negative.
	ErrSimplexNegative = errors.New("pkg/math: probability element is negative")
	// ErrSimplexNonFinite is returned when a probability element is NaN or infinite.
	ErrSimplexNonFinite = errors.New("pkg/math: probability element is non-finite (NaN or Inf)")
	// ErrSimplexSumMismatch is returned when the sum of probabilities violates the 1.0 +- epsilon tolerance.
	ErrSimplexSumMismatch = errors.New("pkg/math: probability vector does not sum to 1.0 within tolerance")
)

// DefaultSimplexTolerance defines the canonical floating-point tolerance (1e-9)
// for probability simplex mass conservation under Inviolate 8 and AGENTS.md Section 7.2.
const DefaultSimplexTolerance = 1e-9

// CompensatedSum computes the sum of a float64 slice using Neumaier's compensated summation algorithm.
//
// Unlike naive floating-point accumulation (which loses precision when summing numbers of disparate magnitudes),
// Neumaier summation tracks high-order and low-order lost bits, achieving machine-epsilon precision.
func CompensatedSum(xs []float64) float64 {
	if len(xs) == 0 {
		return 0.0
	}
	sum := 0.0
	c := 0.0 // Running compensation for lost low-order bits

	for _, x := range xs {
		t := sum + x
		if math.Abs(sum) >= math.Abs(x) {
			c += (sum - t) + x
		} else {
			c += (x - t) + sum
		}
		sum = t
	}
	return sum + c
}

// ValidateSimplex checks whether the given probability vector represents a valid probability
// distribution on the unit simplex:
//  1. The vector is non-empty.
//  2. All elements are finite and non-negative (p_i >= 0).
//  3. The compensated sum equals 1.0 within the specified epsilon tolerance (|sum - 1.0| <= epsilon).
//
// If any condition is violated, ValidateSimplex returns an explanatory error.
func ValidateSimplex(probs []float64, epsilon float64) error {
	if len(probs) == 0 {
		return ErrSimplexEmpty
	}
	if epsilon < 0.0 {
		epsilon = DefaultSimplexTolerance
	}

	for i, p := range probs {
		if math.IsNaN(p) || math.IsInf(p, 0) {
			return fmt.Errorf("%w: index %d has non-finite value %f", ErrSimplexNonFinite, i, p)
		}
		if p < -1e-15 { // Allow microscopic negative floating-point zero jitter
			return fmt.Errorf("%w: index %d has negative probability %e", ErrSimplexNegative, i, p)
		}
	}

	sum := CompensatedSum(probs)
	diff := math.Abs(sum - 1.0)
	if diff > epsilon {
		return fmt.Errorf("%w: sum is %.16f (diff %.4e > tolerance %.4e)", ErrSimplexSumMismatch, sum, diff, epsilon)
	}

	return nil
}

// MustValidateSimplex validates that the probability vector lies on the unit simplex.
//
// In accordance with Inviolate 8 (Fail-Closed Robustness), if the simplex condition is violated,
// MustValidateSimplex panics immediately rather than permitting invalid belief states to corrupt
// physical vehicle dispatches.
func MustValidateSimplex(probs []float64, epsilon float64) {
	if err := ValidateSimplex(probs, epsilon); err != nil {
		panic(fmt.Sprintf("pkg/math: fail-closed simplex violation: %v", err))
	}
}

// NormalizeSimplex performs L1 probability normalization using compensated summation:
//
//	p_i = v_i / \sum_k v_k
//
// Returns an error if the sum of elements is non-positive or non-finite (Inviolate 8).
func NormalizeSimplex(v []float64) ([]float64, error) {
	if len(v) == 0 {
		return nil, ErrSimplexEmpty
	}
	for i, val := range v {
		if math.IsNaN(val) || math.IsInf(val, 0) {
			return nil, fmt.Errorf("%w: index %d has non-finite value %f", ErrSimplexNonFinite, i, val)
		}
		if val < 0.0 {
			return nil, fmt.Errorf("%w: index %d has negative value %f", ErrSimplexNegative, i, val)
		}
	}
	sum := CompensatedSum(v)
	if sum <= 0.0 || math.IsNaN(sum) || math.IsInf(sum, 0) {
		return nil, fmt.Errorf("%w: total vector mass is non-positive or non-finite (%f)", ErrSimplexNonFinite, sum)
	}

	normalized := make([]float64, len(v))
	for i, val := range v {
		normalized[i] = val / sum
	}
	return normalized, nil
}

// ProjectToSimplex computes the exact Euclidean projection of an arbitrary vector v in R^D
// onto the probability simplex:
//
//	\Delta^D = { p in R^D : \sum_{i=1}^D p_i = 1, p_i >= 0 }
//
// It implements the O(D log D) algorithm of Wang & Carreira-Perpiñán (2013).
//
// In accordance with Inviolate 5 (State Immutability), ProjectToSimplex allocates and returns
// a new slice without mutating the input vector.
func ProjectToSimplex(v []float64) []float64 {
	d := len(v)
	if d == 0 {
		return nil
	}
	if d == 1 {
		return []float64{1.0}
	}

	// Make a copy to sort descending
	u := make([]float64, d)
	copy(u, v)
	sort.Slice(u, func(i, j int) bool {
		return u[i] > u[j]
	})

	// Find rho = max { j in [1, d] : u_j + (1/j) * (1 - \sum_{i=1}^j u_i) > 0 }
	cumsum := 0.0
	rho := 0
	theta := 0.0

	for j := 0; j < d; j++ {
		cumsum += u[j]
		t := (cumsum - 1.0) / float64(j+1)
		if u[j]-t > 0 {
			rho = j
			theta = t
		}
	}
	_ = rho

	// Allocate and project
	p := make([]float64, d)
	for i, val := range v {
		proj := val - theta
		if proj > 0 {
			p[i] = proj
		} else {
			p[i] = 0.0
		}
	}

	return p
}
