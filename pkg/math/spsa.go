package math

import (
	"context"
	"errors"
	"fmt"
	"math"
)

var (
	// ErrInvalidSPSAConfig is returned when SPSA hyperparameters violate mathematical constraints.
	ErrInvalidSPSAConfig = errors.New("pkg/math: invalid SPSA configuration")
	// ErrNilLossFunction is returned when a nil objective loss function is provided.
	ErrNilLossFunction = errors.New("pkg/math: loss function cannot be nil")
)

// SPSAConfig encapsulates all explicit hyperparameters required for Simultaneous
// Perturbation Stochastic Approximation (Spall 1992, 1998).
//
// In accordance with Inviolate 0 (Explicit Configuration), all algorithm parameters,
// step decay coefficients, and random seeds are passed explicitly without ambient discovery.
type SPSAConfig struct {
	// MaxIterations is the total number of optimization steps to execute.
	MaxIterations int
	// A is the stability constant in the numerator denominator of the step size sequence (typically ~0.1 * MaxIterations).
	A float64
	// a is the gradient step size numerator scaling factor.
	a float64
	// c is the perturbation step size numerator scaling factor.
	c float64
	// Alpha is the decay exponent for gradient step size a_k (standard Spall value = 0.602).
	Alpha float64
	// Gamma is the decay exponent for perturbation step size c_k (standard Spall value = 0.101).
	Gamma float64
	// LowerBounds optionally defines element-wise lower bounds for projected SPSA.
	LowerBounds []float64
	// UpperBounds optionally defines element-wise upper bounds for projected SPSA.
	UpperBounds []float64
	// Seed is the deterministic 64-bit random seed used to initialize simultaneous perturbations.
	Seed uint64
}

// DefaultSPSAConfig returns a canonical SPSA configuration with standard Spall exponents
// (Alpha=0.602, Gamma=0.101) for a given number of iterations.
func DefaultSPSAConfig(maxIterations int, a, c float64, seed uint64) SPSAConfig {
	return SPSAConfig{
		MaxIterations: maxIterations,
		A:             float64(maxIterations) * 0.1,
		a:             a,
		c:             c,
		Alpha:         0.602,
		Gamma:         0.101,
		Seed:          seed,
	}
}

// Validate checks that SPSA hyperparameters are mathematically sound.
func (cfg *SPSAConfig) Validate(dim int) error {
	if cfg.MaxIterations <= 0 {
		return fmt.Errorf("%w: MaxIterations must be > 0, got %d", ErrInvalidSPSAConfig, cfg.MaxIterations)
	}
	if cfg.a <= 0 {
		return fmt.Errorf("%w: step size a must be > 0, got %f", ErrInvalidSPSAConfig, cfg.a)
	}
	if cfg.c <= 0 {
		return fmt.Errorf("%w: perturbation c must be > 0, got %f", ErrInvalidSPSAConfig, cfg.c)
	}
	if cfg.Alpha <= 0 || cfg.Alpha > 1.0 {
		return fmt.Errorf("%w: Alpha must be in (0, 1], got %f", ErrInvalidSPSAConfig, cfg.Alpha)
	}
	if cfg.Gamma <= 0 || cfg.Gamma > 1.0 {
		return fmt.Errorf("%w: Gamma must be in (0, 1], got %f", ErrInvalidSPSAConfig, cfg.Gamma)
	}
	if len(cfg.LowerBounds) > 0 && len(cfg.LowerBounds) != dim {
		return fmt.Errorf("%w: LowerBounds dimension %d != parameter dimension %d", ErrInvalidSPSAConfig, len(cfg.LowerBounds), dim)
	}
	if len(cfg.UpperBounds) > 0 && len(cfg.UpperBounds) != dim {
		return fmt.Errorf("%w: UpperBounds dimension %d != parameter dimension %d", ErrInvalidSPSAConfig, len(cfg.UpperBounds), dim)
	}
	if len(cfg.LowerBounds) > 0 && len(cfg.UpperBounds) > 0 {
		for i := 0; i < dim; i++ {
			if cfg.LowerBounds[i] > cfg.UpperBounds[i] {
				return fmt.Errorf("%w: LowerBounds[%d]=%f > UpperBounds[%d]=%f", ErrInvalidSPSAConfig, i, cfg.LowerBounds[i], i, cfg.UpperBounds[i])
			}
		}
	}
	return nil
}

// LossFunc represents a black-box scalar objective function to be minimized over parameter vector theta.
type LossFunc func(ctx context.Context, theta []float64) (float64, error)

// OptimizeSPSA minimizes the provided objective loss function using projected Simultaneous
// Perturbation Stochastic Approximation.
//
// Concurrency & Cancellation Context:
// In accordance with Inviolate 6, OptimizeSPSA actively selects on ctx.Done() at each iteration,
// terminating early without thread leakage if the parent optimization context is cancelled.
//
// In accordance with Inviolate 5, the returned optimal theta is a newly allocated slice.
func OptimizeSPSA(ctx context.Context, cfg SPSAConfig, initialTheta []float64, loss LossFunc) ([]float64, float64, error) {
	dim := len(initialTheta)
	if dim == 0 {
		return nil, 0, ErrEmptySlice
	}
	if loss == nil {
		return nil, 0, ErrNilLossFunction
	}
	if err := cfg.Validate(dim); err != nil {
		return nil, 0, err
	}

	// Allocate and project initial point
	theta := make([]float64, dim)
	copy(theta, initialTheta)
	projectTheta(theta, cfg.LowerBounds, cfg.UpperBounds)

	rng := NewRNG(cfg.Seed)
	thetaPlus := make([]float64, dim)
	thetaMinus := make([]float64, dim)
	delta := make([]float64, dim)

	bestTheta := make([]float64, dim)
	copy(bestTheta, theta)
	bestLoss := math.Inf(1)

	for k := 0; k < cfg.MaxIterations; k++ {
		select {
		case <-ctx.Done():
			return bestTheta, bestLoss, ctx.Err()
		default:
		}

		// Calculate step sizes for iteration k
		ak := cfg.a / math.Pow(cfg.A+float64(k)+1.0, cfg.Alpha)
		ck := cfg.c / math.Pow(float64(k)+1.0, cfg.Gamma)

		// Generate Bernoulli +/- 1 simultaneous perturbation vector
		for i := 0; i < dim; i++ {
			delta[i] = rng.Rademacher()
			thetaPlus[i] = theta[i] + ck*delta[i]
			thetaMinus[i] = theta[i] - ck*delta[i]
		}

		// Project perturbed points if bounds are active
		projectTheta(thetaPlus, cfg.LowerBounds, cfg.UpperBounds)
		projectTheta(thetaMinus, cfg.LowerBounds, cfg.UpperBounds)

		// Evaluate objective at perturbed points
		yPlus, err := loss(ctx, thetaPlus)
		if err != nil {
			return bestTheta, bestLoss, fmt.Errorf("pkg/math: SPSA evaluation failed at step %d (+): %w", k, err)
		}
		yMinus, err := loss(ctx, thetaMinus)
		if err != nil {
			return bestTheta, bestLoss, fmt.Errorf("pkg/math: SPSA evaluation failed at step %d (-): %w", k, err)
		}

		// Update best observed loss tracking
		if yPlus < bestLoss {
			bestLoss = yPlus
			copy(bestTheta, thetaPlus)
		}
		if yMinus < bestLoss {
			bestLoss = yMinus
			copy(bestTheta, thetaMinus)
		}

		// Simultaneous pseudo-gradient calculation: g_i = (yPlus - yMinus) / (2 * ck * delta_i)
		diffY := yPlus - yMinus
		for i := 0; i < dim; i++ {
			// Since delta[i] in {-1, +1}, division by delta[i] is identical to multiplication by delta[i]
			ghat_i := diffY / (2.0 * ck) * delta[i]
			theta[i] -= ak * ghat_i
		}

		// Project updated parameter vector into feasible domain
		projectTheta(theta, cfg.LowerBounds, cfg.UpperBounds)
	}

	return bestTheta, bestLoss, nil
}

func projectTheta(theta, lower, upper []float64) {
	for i := range theta {
		if len(lower) > 0 && theta[i] < lower[i] {
			theta[i] = lower[i]
		}
		if len(upper) > 0 && theta[i] > upper[i] {
			theta[i] = upper[i]
		}
	}
}
