package math_test

import (
	"math"
	"testing"

	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

func TestCompensatedSum_Precision(t *testing.T) {
	// Standard floating-point catastrophic cancellation:
	// 1e16 + 1.0 - 1e16 evaluates to 0.0 with naive addition in float64.
	xs := []float64{1e16, 1.0, -1e16}
	sum := pkgmath.CompensatedSum(xs)
	if sum != 1.0 {
		t.Fatalf("CompensatedSum(%v) = %v; expected 1.0", xs, sum)
	}

	// Summing 1,000,000 small terms
	const n = 1000000
	const val = 1e-6
	terms := make([]float64, n)
	for i := range terms {
		terms[i] = val
	}
	largeSum := pkgmath.CompensatedSum(terms)
	if math.Abs(largeSum-1.0) > 1e-15 {
		t.Fatalf("CompensatedSum of %d terms of %e = %v; expected 1.0", n, val, largeSum)
	}
}

func TestValidateSimplex_Cases(t *testing.T) {
	// Valid simplex
	valid := []float64{0.2, 0.3, 0.5}
	if err := pkgmath.ValidateSimplex(valid, pkgmath.DefaultSimplexTolerance); err != nil {
		t.Fatalf("expected valid simplex, got error: %v", err)
	}

	// Degenerate singleton simplex (N=0)
	singleton := []float64{1.0}
	if err := pkgmath.ValidateSimplex(singleton, pkgmath.DefaultSimplexTolerance); err != nil {
		t.Fatalf("expected valid singleton simplex, got error: %v", err)
	}

	// Negative probability
	neg := []float64{0.5, -0.1, 0.6}
	if err := pkgmath.ValidateSimplex(neg, pkgmath.DefaultSimplexTolerance); err == nil {
		t.Fatalf("expected error on negative probability, got nil")
	}

	// Sum mismatch (0.95 != 1.0)
	mismatch := []float64{0.4, 0.3, 0.25}
	if err := pkgmath.ValidateSimplex(mismatch, pkgmath.DefaultSimplexTolerance); err == nil {
		t.Fatalf("expected error on sum mismatch, got nil")
	}

	// Non-finite value
	nanVec := []float64{0.5, math.NaN(), 0.5}
	if err := pkgmath.ValidateSimplex(nanVec, pkgmath.DefaultSimplexTolerance); err == nil {
		t.Fatalf("expected error on NaN element, got nil")
	}

	// Empty vector
	if err := pkgmath.ValidateSimplex(nil, pkgmath.DefaultSimplexTolerance); err == nil {
		t.Fatalf("expected error on empty vector, got nil")
	}
}

func TestMustValidateSimplex_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected MustValidateSimplex to panic on invalid simplex")
		}
	}()

	invalid := []float64{0.5, 0.2}
	pkgmath.MustValidateSimplex(invalid, pkgmath.DefaultSimplexTolerance)
}

func TestSimplex_100StepBayesianTransitionDrift(t *testing.T) {
	// Enforce AGENTS.md Section 7.2 requirement:
	// "Float64 mathematical calculations must check for accumulation of rounding errors,
	// verifying that probabilities sum to 1.0 \pm 1e-9 after 100 consecutive Bayes' transitions."
	rng := pkgmath.NewRNG(2026)
	dim := 8

	// Initialize uniform belief
	b := make([]float64, dim)
	for i := range b {
		b[i] = 1.0 / float64(dim)
	}

	// 100 sequential Bayes filter transitions: b_{t+1}(i) = b_t(i) * L_t(i) / \sum_j b_t(j) L_t(j)
	for step := 0; step < 100; step++ {
		// Log likelihoods for observation
		logLikelihoods := make([]float64, dim)
		for i := range logLikelihoods {
			// Random likelihood in log-space
			logLikelihoods[i] = -rng.Float64() * 3.0
		}

		// Compute unnormalized log posterior: ln b(i) + ln L(i)
		logPost := make([]float64, dim)
		for i := range logPost {
			logPost[i] = math.Log(b[i]) + logLikelihoods[i]
		}

		// Normalize back to simplex
		updatedB, err := pkgmath.LogNormalize(logPost)
		if err != nil {
			t.Fatalf("step %d: LogNormalize failed: %v", step, err)
		}
		b = updatedB

		// Verify simplex constraint after each Bayes step
		if err := pkgmath.ValidateSimplex(b, 1e-9); err != nil {
			t.Fatalf("step %d: Bayes transition simplex drift violation: %v", step, err)
		}
	}

	// Final strict assertion
	pkgmath.MustValidateSimplex(b, 1e-9)
}

func TestProjectToSimplex(t *testing.T) {
	// Vector outside simplex
	v := []float64{1.5, 0.2, -0.5, 0.8}
	p := pkgmath.ProjectToSimplex(v)

	if err := pkgmath.ValidateSimplex(p, 1e-9); err != nil {
		t.Fatalf("projected vector violates simplex: %v", err)
	}

	// Vector already on simplex should remain identical within precision
	alreadyOnSimplex := []float64{0.25, 0.25, 0.5}
	p2 := pkgmath.ProjectToSimplex(alreadyOnSimplex)
	for i := range alreadyOnSimplex {
		if math.Abs(alreadyOnSimplex[i]-p2[i]) > 1e-14 {
			t.Fatalf("ProjectToSimplex altered vector already on simplex at %d: %v != %v", i, alreadyOnSimplex[i], p2[i])
		}
	}
}
