package numeric

import "math"

// Tolerance defines absolute and relative floating-point tolerances.
// Two values a and b are considered equal if:
//
//	|a - b| <= Atol + Rtol * max(|a|, |b|)
type Tolerance struct {
	Atol float64
	Rtol float64
}

// NewTolerance constructs a custom Tolerance with validation.
// Tolerances must be non-negative and finite.
func NewTolerance(atol, rtol float64) Tolerance {
	if atol < 0 || math.IsNaN(atol) || math.IsInf(atol, 0) {
		atol = 0
	}
	if rtol < 0 || math.IsNaN(rtol) || math.IsInf(rtol, 0) {
		rtol = 0
	}
	return Tolerance{Atol: atol, Rtol: rtol}
}

// DefaultTolerance returns a standard tolerance suitable for general float64 operations.
// Atol = 1e-12, Rtol = 1e-9.
func DefaultTolerance() Tolerance {
	return Tolerance{
		Atol: 1e-12,
		Rtol: 1e-9,
	}
}

// ExactTolerance returns a zero tolerance requiring exact equality.
func ExactTolerance() Tolerance {
	return Tolerance{
		Atol: 0.0,
		Rtol: 0.0,
	}
}

// LooseTolerance returns a relaxed tolerance suitable for Monte Carlo or approximate operations.
// Atol = 1e-6, Rtol = 1e-4.
func LooseTolerance() Tolerance {
	return Tolerance{
		Atol: 1e-6,
		Rtol: 1e-4,
	}
}

// Equal returns true if floating-point values a and b are within tolerance.
// Returns false if either a or b is NaN.
// If both a and b are identical infinities with the same sign, returns true.
func (t Tolerance) Equal(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return false
	}
	if a == b {
		return true
	}
	if math.IsInf(a, 0) || math.IsInf(b, 0) {
		return false
	}

	diff := math.Abs(a - b)
	maxMag := math.Max(math.Abs(a), math.Abs(b))
	threshold := t.Atol + t.Rtol*maxMag
	return diff <= threshold
}

// SliceEqual checks element-wise equality of two float64 slices under the given tolerance.
// Returns false if lengths differ.
func (t Tolerance) SliceEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !t.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}
