package numeric

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrNaN is returned when a NaN value is encountered where a finite number is required.
	ErrNaN = errors.New("numeric: value is NaN")

	// ErrInf is returned when an infinite value is encountered where a finite number is required.
	ErrInf = errors.New("numeric: value is infinite")

	// ErrNegative is returned when a negative value is encountered where non-negative is required.
	ErrNegative = errors.New("numeric: value is negative")

	// ErrEmptySlice is returned when an operation expects a non-empty slice.
	ErrEmptySlice = errors.New("numeric: slice is empty")

	// ErrInvalidProbabilitySum is returned when a probability distribution does not sum to 1 within tolerance.
	ErrInvalidProbabilitySum = errors.New("numeric: probabilities do not sum to 1")
)

// IsFinite returns true if v is neither NaN nor infinite.
func IsFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// AssertFinite validates that val is finite (not NaN or Inf).
// Returns a contextual error if validation fails.
func AssertFinite(val float64, name string) error {
	if math.IsNaN(val) {
		return fmt.Errorf("%w: %s is NaN", ErrNaN, name)
	}
	if math.IsInf(val, 1) {
		return fmt.Errorf("%w: %s is +Inf", ErrInf, name)
	}
	if math.IsInf(val, -1) {
		return fmt.Errorf("%w: %s is -Inf", ErrInf, name)
	}
	return nil
}

// AssertFiniteSlice validates that all elements in vals are finite.
func AssertFiniteSlice(vals []float64, name string) error {
	for i, v := range vals {
		if math.IsNaN(v) {
			return fmt.Errorf("%w: %s[%d] is NaN", ErrNaN, name, i)
		}
		if math.IsInf(v, 0) {
			return fmt.Errorf("%w: %s[%d] is Inf", ErrInf, name, i)
		}
	}
	return nil
}

// AssertNonNegative validates that val is finite and non-negative (>= 0).
func AssertNonNegative(val float64, name string) error {
	if err := AssertFinite(val, name); err != nil {
		return err
	}
	if val < 0 {
		return fmt.Errorf("%w: %s = %g < 0", ErrNegative, name, val)
	}
	return nil
}

// AssertNonNegativeSlice validates that all elements in vals are finite and non-negative.
func AssertNonNegativeSlice(vals []float64, name string) error {
	for i, v := range vals {
		if err := AssertNonNegative(v, fmt.Sprintf("%s[%d]", name, i)); err != nil {
			return err
		}
	}
	return nil
}

// AssertProbabilities validates that a slice represents a valid probability distribution:
// 1. Slice must be non-empty.
// 2. All elements must be finite and >= 0.
// 3. Sum of elements must equal 1.0 within the specified tolerance.
func AssertProbabilities(p []float64, tol Tolerance) error {
	if len(p) == 0 {
		return ErrEmptySlice
	}

	var sum float64
	for i, v := range p {
		if math.IsNaN(v) {
			return fmt.Errorf("%w: p[%d] is NaN", ErrNaN, i)
		}
		if math.IsInf(v, 0) {
			return fmt.Errorf("%w: p[%d] is Inf", ErrInf, i)
		}
		if v < 0 {
			return fmt.Errorf("%w: p[%d] = %g < 0", ErrNegative, i, v)
		}
		sum += v
	}

	if !tol.Equal(sum, 1.0) {
		return fmt.Errorf("%w: sum is %g (expected 1.0 within tol atol=%g, rtol=%g)",
			ErrInvalidProbabilitySum, sum, tol.Atol, tol.Rtol)
	}

	return nil
}
