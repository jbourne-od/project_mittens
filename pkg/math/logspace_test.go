package math_test

import (
	"math"
	"testing"

	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

func TestLogSumExp_StandardAndExtreme(t *testing.T) {
	// Standard values: ln(exp(1) + exp(2)) = ln(e + e^2) ~ 2.3132616875182228
	val := pkgmath.LogSumExp(1.0, 2.0)
	expected := 2.3132616875182228
	if math.Abs(val-expected) > 1e-14 {
		t.Fatalf("LogSumExp(1.0, 2.0) = %v; expected %v", val, expected)
	}

	// Extreme large values (would overflow standard exp(1000))
	largeVal := pkgmath.LogSumExp(1000.0, 1000.0)
	expectedLarge := 1000.0 + math.Ln2
	if math.Abs(largeVal-expectedLarge) > 1e-12 {
		t.Fatalf("LogSumExp(1000, 1000) = %v; expected %v", largeVal, expectedLarge)
	}

	// Extreme small values (would underflow standard exp(-1000))
	smallVal := pkgmath.LogSumExp(-1000.0, -1000.0)
	expectedSmall := -1000.0 + math.Ln2
	if math.Abs(smallVal-expectedSmall) > 1e-12 {
		t.Fatalf("LogSumExp(-1000, -1000) = %v; expected %v", smallVal, expectedSmall)
	}

	// Infinity handling
	if res := pkgmath.LogSumExp(math.Inf(-1), 5.0); res != 5.0 {
		t.Fatalf("LogSumExp(-Inf, 5.0) = %v; expected 5.0", res)
	}
	if res := pkgmath.LogSumExp(5.0, math.Inf(-1)); res != 5.0 {
		t.Fatalf("LogSumExp(5.0, -Inf) = %v; expected 5.0", res)
	}
	if res := pkgmath.LogSumExp(math.Inf(-1), math.Inf(-1)); res != math.Inf(-1) {
		t.Fatalf("LogSumExp(-Inf, -Inf) = %v; expected -Inf", res)
	}
}

func TestLogSumExpSlice(t *testing.T) {
	// Empty slice returns -Inf
	if res := pkgmath.LogSumExpSlice(nil); res != math.Inf(-1) {
		t.Fatalf("LogSumExpSlice(nil) = %v; expected -Inf", res)
	}

	// All -Inf returns -Inf
	if res := pkgmath.LogSumExpSlice([]float64{math.Inf(-1), math.Inf(-1)}); res != math.Inf(-1) {
		t.Fatalf("LogSumExpSlice([-Inf, -Inf]) = %v; expected -Inf", res)
	}

	// Standard slice: [ln(1), ln(2), ln(3)] = [0, ln(2), ln(3)], sum = 6, ln(6) ~ 1.791759469228055
	xs := []float64{math.Log(1.0), math.Log(2.0), math.Log(3.0)}
	res := pkgmath.LogSumExpSlice(xs)
	expected := math.Log(6.0)
	if math.Abs(res-expected) > 1e-14 {
		t.Fatalf("LogSumExpSlice([ln 1, ln 2, ln 3]) = %v; expected %v", res, expected)
	}
}

func TestLogSubExp(t *testing.T) {
	// ln(exp(3) - exp(2)) = ln(e^3 - e^2) ~ 2.537678508985959
	val, err := pkgmath.LogSubExp(3.0, 2.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := math.Log(math.Exp(3.0) - math.Exp(2.0))
	if math.Abs(val-expected) > 1e-14 {
		t.Fatalf("LogSubExp(3.0, 2.0) = %v; expected %v", val, expected)
	}

	// Identical values return -Inf (ln(0) = -Inf)
	valSame, err := pkgmath.LogSubExp(5.0, 5.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valSame != math.Inf(-1) {
		t.Fatalf("LogSubExp(5.0, 5.0) = %v; expected -Inf", valSame)
	}

	// a < b returns error
	_, err = pkgmath.LogSubExp(2.0, 3.0)
	if err == nil {
		t.Fatalf("expected error for a < b, got nil")
	}
}

func TestLogNormalize(t *testing.T) {
	// Normalizing [-1000, -1000, -1000] should yield [1/3, 1/3, 1/3]
	logProbs := []float64{-1000.0, -1000.0, -1000.0}
	probs, err := pkgmath.LogNormalize(logProbs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(probs) != 3 {
		t.Fatalf("expected length 3, got %d", len(probs))
	}
	expectedP := 1.0 / 3.0
	for i, p := range probs {
		if math.Abs(p-expectedP) > 1e-12 {
			t.Fatalf("probs[%d] = %v; expected %v", i, p, expectedP)
		}
	}

	// Normalizing empty or zero-mass slice
	_, err = pkgmath.LogNormalize(nil)
	if err == nil {
		t.Fatalf("expected error on empty slice")
	}

	_, err = pkgmath.LogNormalize([]float64{math.Inf(-1), math.Inf(-1)})
	if err == nil {
		t.Fatalf("expected error on all -Inf values")
	}
}
