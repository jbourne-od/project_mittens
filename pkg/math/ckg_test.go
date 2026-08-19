package math_test

import (
	"math"
	"testing"

	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

func TestCKG_SpatialCovariance(t *testing.T) {
	// 3 spatial regions:
	// Region 0: (0, 0)
	// Region 1: (100 miles away)
	// Region 2: (500 miles away)
	distMatrix, _ := pkgmath.NewDenseMatrixWithData(3, 3, []float64{
		0, 100, 500,
		100, 0, 400,
		500, 400, 0,
	})

	cfg := pkgmath.SpatialKernelConfig{
		SignalVariance:   100.0,
		LengthScaleMiles: 200.0,
		NoiseVariance:    1.0,
	}

	cov, err := pkgmath.BuildSpatialCovariance(distMatrix, cfg)
	if err != nil {
		t.Fatalf("BuildSpatialCovariance failed: %v", err)
	}

	// Diagonals must be SignalVariance + NoiseVariance = 101.0
	for i := 0; i < 3; i++ {
		diag := cov.At(i, i)
		if math.Abs(diag-101.0) > 1e-10 {
			t.Fatalf("cov(%d, %d) = %v; expected 101.0", i, i, diag)
		}
	}

	// Cov(0, 1) with d=100: 100 * exp(-100^2 / (2 * 200^2)) = 100 * exp(-10000 / 80000) = 100 * exp(-0.125) = 88.24969
	cov01 := cov.At(0, 1)
	expected01 := 100.0 * math.Exp(-0.125)
	if math.Abs(cov01-expected01) > 1e-5 {
		t.Fatalf("cov(0, 1) = %v; expected %v", cov01, expected01)
	}

	// Cov(0, 2) with d=500: 100 * exp(-500^2 / (2 * 200^2)) = 100 * exp(-250000 / 80000) = 100 * exp(-3.125) = 4.39369
	cov02 := cov.At(0, 2)
	expected02 := 100.0 * math.Exp(-3.125)
	if math.Abs(cov02-expected02) > 1e-5 {
		t.Fatalf("cov(0, 2) = %v; expected %v", cov02, expected02)
	}

	// Verify positive definiteness via Cholesky
	_, err = pkgmath.Cholesky(cov)
	if err != nil {
		t.Fatalf("spatial covariance matrix is not positive-definite: %v", err)
	}
}

func TestCKG_BayesianCorrelatedUpdating(t *testing.T) {
	distMatrix, _ := pkgmath.NewDenseMatrixWithData(3, 3, []float64{
		0, 50, 600,
		50, 0, 550,
		600, 550, 0,
	})

	cfg := pkgmath.SpatialKernelConfig{
		SignalVariance:   100.0,
		LengthScaleMiles: 150.0,
		NoiseVariance:    0.5,
	}

	cov, _ := pkgmath.BuildSpatialCovariance(distMatrix, cfg)
	priorMean := []float64{50.0, 50.0, 50.0}

	ckg, err := pkgmath.NewCorrelatedKnowledgeGradient(priorMean, cov)
	if err != nil {
		t.Fatalf("NewCKG failed: %v", err)
	}

	// Observe high value sample at Region 0: y_0 = 80.0 (high driver demand)
	obsVariance := 4.0
	updatedCKG, err := ckg.UpdateBayesian(0, 80.0, obsVariance)
	if err != nil {
		t.Fatalf("UpdateBayesian failed: %v", err)
	}

	// Region 0 mean should increase significantly towards 80.0
	if updatedCKG.MeanAt(0) <= 50.0 || updatedCKG.MeanAt(0) >= 80.0 {
		t.Errorf("expected Region 0 mean to move between 50 and 80, got %v", updatedCKG.MeanAt(0))
	}

	// Region 1 (close, 50 miles away) should ALSO increase due to positive spatial covariance!
	if updatedCKG.MeanAt(1) <= 50.0 {
		t.Errorf("expected correlated Region 1 to increase, got %v", updatedCKG.MeanAt(1))
	}

	// Region 2 (far, 600 miles away) should experience almost no change
	deltaFar := math.Abs(updatedCKG.MeanAt(2) - 50.0)
	if deltaFar > 1.0 {
		t.Errorf("expected distant Region 2 to remain close to prior (delta < 1.0), got delta = %v", deltaFar)
	}

	// Variance in Region 0 must strictly decrease (epistemic uncertainty reduction)
	if updatedCKG.VarianceAt(0) >= ckg.VarianceAt(0) {
		t.Errorf("expected posterior variance %v < prior variance %v", updatedCKG.VarianceAt(0), ckg.VarianceAt(0))
	}

	// Immutability: original ckg mean and covariance must be unchanged (Inviolate 5)
	if ckg.MeanAt(0) != 50.0 {
		t.Errorf("original prior mean mutated: %v", ckg.MeanAt(0))
	}
}

func TestCKG_LongSequenceStability(t *testing.T) {
	distMatrix, _ := pkgmath.NewDenseMatrixWithData(2, 2, []float64{
		0, 100,
		100, 0,
	})

	cfg := pkgmath.SpatialKernelConfig{
		SignalVariance:   50.0,
		LengthScaleMiles: 200.0,
		NoiseVariance:    1.0,
	}

	cov, _ := pkgmath.BuildSpatialCovariance(distMatrix, cfg)
	currentCKG, _ := pkgmath.NewCorrelatedKnowledgeGradient([]float64{10.0, 10.0}, cov)

	// Perform 100 consecutive observations alternating between Region 0 and Region 1
	for step := 0; step < 100; step++ {
		region := step % 2
		obsVal := 25.0 + float64(step)*0.1
		var err error
		currentCKG, err = currentCKG.UpdateBayesian(region, obsVal, 2.0)
		if err != nil {
			t.Fatalf("update failed at step %d: %v", step, err)
		}

		// Ensure variance remains non-negative and finite
		for r := 0; r < 2; r++ {
			v := currentCKG.VarianceAt(r)
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				t.Fatalf("invalid variance at step %d, region %d: %v", step, r, v)
			}
		}
	}

	t.Logf("CKG 100-step posterior mean: %v, variance: [%.4f, %.4f]",
		currentCKG.Mean(), currentCKG.VarianceAt(0), currentCKG.VarianceAt(1))
}
