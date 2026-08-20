package math_test

import (
	"math"
	"testing"

	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

func TestStats_IncompleteBeta(t *testing.T) {
	// Standard test points against known values
	// I_0.5(1, 1) = 0.5
	val := pkgmath.IncompleteBeta(1.0, 1.0, 0.5)
	if math.Abs(val-0.5) > 1e-6 {
		t.Errorf("expected I_0.5(1,1)=0.5, got %f", val)
	}

	// Boundary values
	if pkgmath.IncompleteBeta(2.0, 2.0, 0.0) != 0.0 {
		t.Errorf("expected 0 at x=0")
	}
	if pkgmath.IncompleteBeta(2.0, 2.0, 1.0) != 1.0 {
		t.Errorf("expected 1 at x=1")
	}
}

func TestStats_StudentTCDF(t *testing.T) {
	// For df=30, t=2.042 -> p approx 0.05
	pVal := pkgmath.StudentTCDFTwoTailed(2.042, 30)
	if math.Abs(pVal-0.05) > 0.005 {
		t.Errorf("expected p approx 0.05 for t=2.042, df=30, got %f", pVal)
	}

	// For very high t, p should approach 0
	pSmall := pkgmath.StudentTCDFTwoTailed(10.0, 50)
	if pSmall > 1e-10 {
		t.Errorf("expected p near 0 for t=10, got %e", pSmall)
	}
}

func TestStats_PairedTTest_StatisticallySignificantLift(t *testing.T) {
	// 10 paired simulation runs where Candidate consistently beats Baseline
	baseline := []float64{100, 105, 98, 110, 102, 95, 108, 101, 99, 104}
	candidate := []float64{115, 120, 112, 128, 118, 110, 122, 116, 114, 121}

	res, err := pkgmath.ComputePairedTTest(baseline, candidate)
	if err != nil {
		t.Fatalf("ComputePairedTTest failed: %v", err)
	}

	t.Logf("\n%s", res.SummaryString())

	if res.N != 10 {
		t.Errorf("expected N=10, got %d", res.N)
	}
	if res.MeanDifference <= 0 {
		t.Errorf("expected positive mean difference, got %f", res.MeanDifference)
	}
	if res.TStatistic < 10.0 {
		t.Errorf("expected large t-statistic (>10), got %f", res.TStatistic)
	}
	if res.PValueOneTailed > 1e-5 {
		t.Errorf("expected p-value < 1e-5, got %e", res.PValueOneTailed)
	}
	if res.ConfidenceLow95 <= 0 {
		t.Errorf("expected lower 95%% CI > 0, got %f", res.ConfidenceLow95)
	}
	if res.PercentLift < 10.0 {
		t.Errorf("expected lift > 10%%, got %.2f%%", res.PercentLift)
	}
}

func TestStats_PairedTTest_IdenticalArrays(t *testing.T) {
	// N=0 Degenerate parity case: differences are exactly 0
	baseline := []float64{100, 200, 300, 400, 500}
	candidate := []float64{100, 200, 300, 400, 500}

	res, err := pkgmath.ComputePairedTTest(baseline, candidate)
	if err != nil {
		t.Fatalf("ComputePairedTTest failed: %v", err)
	}

	if math.Abs(res.MeanDifference) > 1e-9 {
		t.Errorf("expected mean difference 0, got %f", res.MeanDifference)
	}
	if math.Abs(res.TStatistic) > 1e-9 {
		t.Errorf("expected t-statistic 0, got %f", res.TStatistic)
	}
}

func TestStats_PairedTTest_ValidationErrors(t *testing.T) {
	// 1. Mismatched length
	_, err := pkgmath.ComputePairedTTest([]float64{1}, []float64{1, 2})
	if err != pkgmath.ErrMismatchedSampleSizes {
		t.Errorf("expected ErrMismatchedSampleSizes, got %v", err)
	}

	// 2. Insufficient samples
	_, err = pkgmath.ComputePairedTTest([]float64{1}, []float64{1})
	if err != pkgmath.ErrInsufficientSamples {
		t.Errorf("expected ErrInsufficientSamples, got %v", err)
	}

	// 3. Non-finite values
	_, err = pkgmath.ComputePairedTTest([]float64{1, math.NaN()}, []float64{1, 2})
	if err == nil {
		t.Errorf("expected error on NaN")
	}
}
