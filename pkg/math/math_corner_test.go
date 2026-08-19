package math_test

import (
	"context"
	"math"
	"testing"
	"time"

	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

func TestSimplex_10000StepBayesianTransitionDrift(t *testing.T) {
	// Initialize uniform belief over 4 competitor postures: [0.25, 0.25, 0.25, 0.25]
	b := []float64{0.25, 0.25, 0.25, 0.25}
	rng := pkgmath.NewRNG(12345)

	// Execute 10,000 consecutive stochastic Bayesian update transitions
	for step := 0; step < 10000; step++ {
		// Random observation likelihood vector L_t
		likelihood := []float64{
			0.1 + 0.8*rng.Float64(),
			0.1 + 0.8*rng.Float64(),
			0.1 + 0.8*rng.Float64(),
			0.1 + 0.8*rng.Float64(),
		}

		// Posterior b_{t+1}(i) \propto b_t(i) * L_t(i)
		posterior := make([]float64, 4)
		for i := 0; i < 4; i++ {
			posterior[i] = b[i] * likelihood[i]
		}

		// Normalize back to simplex
		normPost := pkgmath.ProjectToSimplex(posterior)
		b = normPost

		// Check simplex invariant after every step
		sum := pkgmath.CompensatedSum(b)
		if math.Abs(sum-1.0) > 1e-11 {
			t.Fatalf("Step %d: Belief simplex drift exceeded 1e-11: sum = %.15f", step, sum)
		}
	}

	// Final verification
	if err := pkgmath.ValidateSimplex(b, 1e-9); err != nil {
		t.Errorf("10,000 step simplex validation failed: %v", err)
	}
}

func TestSimplex_VertexBoundaryExtreme(t *testing.T) {
	// Extreme Dirac delta belief sitting exactly on vertex 0: [1.0, 0.0, 0.0]
	vertexBelief := []float64{1.0, 0.0, 0.0}

	if err := pkgmath.ValidateSimplex(vertexBelief, 1e-9); err != nil {
		t.Fatalf("Vertex belief validation failed: %v", err)
	}

	// Bayesian update with zero-probability states
	likelihood := []float64{0.5, 0.8, 0.9}
	unnormalized := []float64{
		vertexBelief[0] * likelihood[0],
		vertexBelief[1] * likelihood[1],
		vertexBelief[2] * likelihood[2],
	}

	posterior, err := pkgmath.NormalizeSimplex(unnormalized)
	if err != nil {
		t.Fatalf("NormalizeSimplex failed: %v", err)
	}

	if math.Abs(posterior[0]-1.0) > 1e-12 || posterior[1] != 0.0 || posterior[2] != 0.0 {
		t.Errorf("expected [1.0, 0.0, 0.0], got %v", posterior)
	}
}

func TestMatrix_IllConditionedCholesky(t *testing.T) {
	// Well-conditioned SPD matrix
	// A = [4  2]
	//     [2  5]
	// L = [2    0]
	//     [1    2]
	mat, err := pkgmath.NewDenseMatrixWithData(2, 2, []float64{
		4.0, 2.0,
		2.0, 5.0,
	})
	if err != nil {
		t.Fatalf("NewDenseMatrixWithData failed: %v", err)
	}

	L, err := pkgmath.Cholesky(mat)
	if err != nil {
		t.Fatalf("Cholesky failed: %v", err)
	}

	if math.Abs(L.At(0, 0)-2.0) > 1e-9 || math.Abs(L.At(1, 0)-1.0) > 1e-9 || math.Abs(L.At(1, 1)-2.0) > 1e-9 {
		t.Errorf("Unexpected Cholesky factor L: got %v", L)
	}

	// Ill-conditioned matrix with near-zero eigenvalue (singular matrix)
	// S = [1  1]
	//     [1  1]
	singularMat, _ := pkgmath.NewDenseMatrixWithData(2, 2, []float64{
		1.0, 1.0,
		1.0, 1.0,
	})
	if _, err := pkgmath.Cholesky(singularMat); err == nil {
		t.Errorf("expected Cholesky to fail closed on singular non-positive-definite matrix")
	}
}

func TestSPSA_NoisyNonConvexRastrigin(t *testing.T) {
	// SPSA optimization on 2D quadratic sphere with random stochastic noise:
	// f(x) = (x_0 - 3)^2 + (x_1 + 2)^2 + noise
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rng := pkgmath.NewRNG(999)
	lossFunc := func(ctx context.Context, theta []float64) (float64, error) {
		dx := theta[0] - 3.0
		dy := theta[1] - (-2.0)
		noise := (rng.Float64() - 0.5) * 0.05 // Stochastic noise
		return dx*dx + dy*dy + noise, nil
	}

	cfg := pkgmath.DefaultSPSAConfig(100, 0.2, 0.05, 999)
	cfg.LowerBounds = []float64{-10.0, -10.0}
	cfg.UpperBounds = []float64{10.0, 10.0}

	initTheta := []float64{0.0, 0.0}
	optTheta, finalLoss, err := pkgmath.OptimizeSPSA(ctx, cfg, initTheta, lossFunc)
	if err != nil {
		t.Fatalf("SPSA optimization failed: %v", err)
	}

	t.Logf("SPSA converged: theta = [%.3f, %.3f], final loss = %.4f",
		optTheta[0], optTheta[1], finalLoss)

	// Verify optimizer moved significantly towards true optimum [3.0, -2.0]
	if math.Abs(optTheta[0]-3.0) > 1.5 || math.Abs(optTheta[1]-(-2.0)) > 1.5 {
		t.Errorf("SPSA failed to converge near [3.0, -2.0]: got %v", optTheta)
	}
}
