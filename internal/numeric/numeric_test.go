package numeric

import (
	"errors"
	"math"
	"testing"
)

func TestTolerance_Equal(t *testing.T) {
	tol := DefaultTolerance()

	tests := []struct {
		name     string
		a, b     float64
		expected bool
	}{
		{"identical zeros", 0.0, 0.0, true},
		{"identical numbers", 42.123456, 42.123456, true},
		{"small difference within atol", 1.0, 1.0 + 1e-13, true},
		{"large difference beyond atol", 1.0, 1.0 + 1e-8, false},
		{"large numbers within rtol", 1e8, 1e8 + 0.01, true},
		{"large numbers beyond rtol", 1e8, 1e8 + 1.0, false},
		{"NaN comparisons", math.NaN(), 1.0, false},
		{"both NaN", math.NaN(), math.NaN(), false},
		{"positive infinity", math.Inf(1), math.Inf(1), true},
		{"negative infinity", math.Inf(-1), math.Inf(-1), true},
		{"mixed infinity", math.Inf(1), math.Inf(-1), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tol.Equal(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("Equal(%g, %g) = %v, expected %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestTolerance_SliceEqual(t *testing.T) {
	tol := DefaultTolerance()

	a := []float64{1.0, 2.0, 3.0}
	b := []float64{1.0 + 1e-13, 2.0 - 1e-13, 3.0}
	c := []float64{1.0, 2.0}
	d := []float64{1.0, 2.0, 4.0}

	if !tol.SliceEqual(a, b) {
		t.Errorf("expected a and b to be equal within tolerance")
	}
	if tol.SliceEqual(a, c) {
		t.Errorf("expected different length slices to be unequal")
	}
	if tol.SliceEqual(a, d) {
		t.Errorf("expected different value slices to be unequal")
	}
}

func TestValidation_AssertFinite(t *testing.T) {
	if err := AssertFinite(42.0, "x"); err != nil {
		t.Errorf("unexpected error for finite number: %v", err)
	}

	if err := AssertFinite(math.NaN(), "x"); !errors.Is(err, ErrNaN) {
		t.Errorf("expected ErrNaN, got %v", err)
	}

	if err := AssertFinite(math.Inf(1), "x"); !errors.Is(err, ErrInf) {
		t.Errorf("expected ErrInf, got %v", err)
	}
}

func TestValidation_AssertProbabilities(t *testing.T) {
	tol := DefaultTolerance()

	valid := []float64{0.2, 0.3, 0.5}
	if err := AssertProbabilities(valid, tol); err != nil {
		t.Errorf("unexpected error for valid probabilities: %v", err)
	}

	empty := []float64{}
	if err := AssertProbabilities(empty, tol); !errors.Is(err, ErrEmptySlice) {
		t.Errorf("expected ErrEmptySlice, got %v", err)
	}

	negative := []float64{0.5, -0.1, 0.6}
	if err := AssertProbabilities(negative, tol); !errors.Is(err, ErrNegative) {
		t.Errorf("expected ErrNegative, got %v", err)
	}

	invalidSum := []float64{0.5, 0.4}
	if err := AssertProbabilities(invalidSum, tol); !errors.Is(err, ErrInvalidProbabilitySum) {
		t.Errorf("expected ErrInvalidProbabilitySum, got %v", err)
	}
}

func TestLogSumExp_ExtremeDynamicRange(t *testing.T) {
	tol := DefaultTolerance()

	// High scale: naive exp would overflow to +Inf
	high := []float64{1000.0, 1000.0}
	expectedHigh := 1000.0 + math.Ln2
	gotHigh := LogSumExp(high)
	if !tol.Equal(gotHigh, expectedHigh) {
		t.Errorf("LogSumExp([1000, 1000]) = %g, expected %g", gotHigh, expectedHigh)
	}

	// Low scale: naive exp would underflow to 0
	low := []float64{-1000.0, -1000.0}
	expectedLow := -1000.0 + math.Ln2
	gotLow := LogSumExp(low)
	if !tol.Equal(gotLow, expectedLow) {
		t.Errorf("LogSumExp([-1000, -1000]) = %g, expected %g", gotLow, expectedLow)
	}

	// Single element
	if LogSumExp([]float64{5.5}) != 5.5 {
		t.Errorf("expected 5.5 for single element")
	}

	// Empty
	if !math.IsInf(LogSumExp([]float64{}), -1) {
		t.Errorf("expected -Inf for empty slice")
	}
}

func TestLogAdd_And_LogSub(t *testing.T) {
	tol := DefaultTolerance()

	// LogAdd
	a := 10.0
	b := 10.0
	expectedAdd := 10.0 + math.Ln2
	gotAdd := LogAdd(a, b)
	if !tol.Equal(gotAdd, expectedAdd) {
		t.Errorf("LogAdd(%g, %g) = %g, expected %g", a, b, gotAdd, expectedAdd)
	}

	// LogSub
	x := 5.0
	y := 4.0
	// log(exp(5) - exp(4)) = 5 + log(1 - exp(-1))
	expectedSub := 5.0 + math.Log(1.0-math.Exp(-1.0))
	gotSub, err := LogSub(x, y)
	if err != nil {
		t.Fatalf("unexpected error in LogSub: %v", err)
	}
	if !tol.Equal(gotSub, expectedSub) {
		t.Errorf("LogSub(%g, %g) = %g, expected %g", x, y, gotSub, expectedSub)
	}

	// LogSub equal elements
	gotEqual, err := LogSub(5.0, 5.0)
	if err != nil || !math.IsInf(gotEqual, -1) {
		t.Errorf("expected -Inf for LogSub(5, 5), got %g, err=%v", gotEqual, err)
	}

	// LogSub domain error
	_, err = LogSub(3.0, 4.0)
	if !errors.Is(err, ErrLogDomain) {
		t.Errorf("expected ErrLogDomain for LogSub(3, 4), got %v", err)
	}
}

func TestKahanSum_Precision(t *testing.T) {
	// Accumulate 10^6 small numbers 1e-16 onto 1.0
	// Naive floating-point summation loses all 1e-16 additions because 1.0 + 1e-16 == 1.0 in standard 53-bit mantissa.
	n := 1000000
	vals := make([]float64, n+1)
	vals[0] = 1.0
	fraction := 1e-14
	for i := 1; i <= n; i++ {
		vals[i] = fraction
	}

	var naiveSum float64
	for _, v := range vals {
		naiveSum += v
	}

	compensatedSum := KahanSum(vals)
	expected := 1.0 + float64(n)*fraction // 1.0 + 1e-8 = 1.00000001

	tol := NewTolerance(1e-14, 1e-12)
	if !tol.Equal(compensatedSum, expected) {
		t.Errorf("KahanSum = %.16f, expected %.16f", compensatedSum, expected)
	}
	if naiveSum == compensatedSum && fraction <= 1e-16 {
		t.Errorf("expected naive sum to have precision loss relative to compensated sum")
	}
}

func TestNormalizeProbabilities(t *testing.T) {
	tol := DefaultTolerance()

	// In-place normalization
	weights := []float64{1.0, 2.0, 3.0, 4.0}
	normalized, err := NormalizeProbabilities(weights, weights)
	if err != nil {
		t.Fatalf("unexpected error normalizing: %v", err)
	}

	expected := []float64{0.1, 0.2, 0.3, 0.4}
	if !tol.SliceEqual(normalized, expected) {
		t.Errorf("got %v, expected %v", normalized, expected)
	}
	if err := AssertProbabilities(normalized, tol); err != nil {
		t.Errorf("normalized result failed probability assertion: %v", err)
	}

	// Out-of-place normalization
	src := []float64{5.0, 5.0}
	dst, err := NormalizeProbabilities(nil, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dst) != 2 || dst[0] != 0.5 || dst[1] != 0.5 {
		t.Errorf("expected [0.5, 0.5], got %v", dst)
	}

	// Error cases
	if _, err := NormalizeProbabilities(nil, []float64{0.0, 0.0}); !errors.Is(err, ErrZeroProbabilityMass) {
		t.Errorf("expected ErrZeroProbabilityMass, got %v", err)
	}
	if _, err := NormalizeProbabilities(nil, []float64{1.0, -1.0}); !errors.Is(err, ErrNegative) {
		t.Errorf("expected ErrNegative, got %v", err)
	}
}

func TestSoftmax(t *testing.T) {
	tol := DefaultTolerance()
	logits := []float64{1000.0, 1000.0}
	probs := Softmax(nil, logits)

	expected := []float64{0.5, 0.5}
	if !tol.SliceEqual(probs, expected) {
		t.Errorf("Softmax([1000, 1000]) = %v, expected %v", probs, expected)
	}
	if err := AssertProbabilities(probs, tol); err != nil {
		t.Errorf("softmax result failed probability assertion: %v", err)
	}
}

func BenchmarkLogSumExp(b *testing.B) {
	data := []float64{1.2, 3.4, 5.6, 7.8, 9.0, 2.1, 4.3, 6.5, 8.7, 0.9}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = LogSumExp(data)
	}
}

func BenchmarkKahanSum(b *testing.B) {
	data := make([]float64, 1000)
	for i := range data {
		data[i] = 1.0 / float64(i+1)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = KahanSum(data)
	}
}

func BenchmarkNormalizeProbabilities(b *testing.B) {
	src := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}
	dst := make([]float64, len(src))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NormalizeProbabilities(dst, src)
	}
}
