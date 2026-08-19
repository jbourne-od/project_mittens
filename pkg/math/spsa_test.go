package math_test

import (
	"context"
	"math"
	"testing"
	"time"

	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

func TestOptimizeSPSA_QuadraticConvergence(t *testing.T) {
	// Minimize L(theta) = (theta_0 - 2.0)^2 + (theta_1 - 3.0)^2 + (theta_2 + 1.0)^2
	target := []float64{2.0, 3.0, -1.0}

	loss := func(_ context.Context, theta []float64) (float64, error) {
		sum := 0.0
		for i := range theta {
			diff := theta[i] - target[i]
			sum += diff * diff
		}
		return sum, nil
	}

	initialTheta := []float64{10.0, -5.0, 8.0}
	cfg := pkgmath.DefaultSPSAConfig(1000, 0.5, 0.1, 42)

	ctx := context.Background()
	bestTheta, bestLoss, err := pkgmath.OptimizeSPSA(ctx, cfg, initialTheta, loss)
	if err != nil {
		t.Fatalf("OptimizeSPSA failed: %v", err)
	}

	if bestLoss > 0.05 {
		t.Fatalf("SPSA failed to converge: bestLoss = %v (expected < 0.05)", bestLoss)
	}

	for i := range target {
		if math.Abs(bestTheta[i]-target[i]) > 0.25 {
			t.Fatalf("bestTheta[%d] = %v; expected close to target %v", i, bestTheta[i], target[i])
		}
	}
}

func TestOptimizeSPSA_BoxConstraints(t *testing.T) {
	// Target is at (10, 10), but bounds are [0, 5] for each dimension.
	target := []float64{10.0, 10.0}
	loss := func(_ context.Context, theta []float64) (float64, error) {
		sum := 0.0
		for i := range theta {
			diff := theta[i] - target[i]
			sum += diff * diff
		}
		return sum, nil
	}

	cfg := pkgmath.DefaultSPSAConfig(200, 0.2, 0.1, 999)
	cfg.LowerBounds = []float64{0.0, 0.0}
	cfg.UpperBounds = []float64{5.0, 5.0}

	ctx := context.Background()
	bestTheta, _, err := pkgmath.OptimizeSPSA(ctx, cfg, []float64{1.0, 1.0}, loss)
	if err != nil {
		t.Fatalf("OptimizeSPSA failed: %v", err)
	}

	for i, val := range bestTheta {
		if val < cfg.LowerBounds[i] || val > cfg.UpperBounds[i] {
			t.Fatalf("bestTheta[%d] = %v violated box constraint [%v, %v]", i, val, cfg.LowerBounds[i], cfg.UpperBounds[i])
		}
	}

	// Should converge to upper boundary (5.0, 5.0)
	for i, val := range bestTheta {
		if math.Abs(val-5.0) > 0.1 {
			t.Fatalf("bestTheta[%d] = %v did not converge to upper bound 5.0", i, val)
		}
	}
}

func TestOptimizeSPSA_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	slowLoss := func(_ context.Context, _ []float64) (float64, error) {
		time.Sleep(5 * time.Millisecond)
		return 1.0, nil
	}

	cfg := pkgmath.DefaultSPSAConfig(10000, 0.1, 0.1, 123)
	_, _, err := pkgmath.OptimizeSPSA(ctx, cfg, []float64{0.0, 0.0}, slowLoss)

	if err == nil || (err != context.DeadlineExceeded && err != context.Canceled) {
		t.Fatalf("expected context cancellation error, got: %v", err)
	}
}
