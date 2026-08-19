// Package math provides high-performance, deterministic mathematical and optimization algorithms.
//
// In accordance with Project Mittens AGENTS.md §4.1:
//   - Zero application domain imports.
//   - Completely pure, standalone mathematical library.
//   - Invariant 5 (Immutability) and Invariant 6 (Zero Mutexes on hot paths).
package math

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrInvalidKernelParams is returned when spatial covariance hyperparameters are non-positive.
	ErrInvalidKernelParams = errors.New("pkg/math: invalid spatial kernel hyperparameters")
	// ErrInvalidCKGDimension is returned when vector/matrix dimensions in CKG are incompatible.
	ErrInvalidCKGDimension = errors.New("pkg/math: CKG dimension mismatch")
)

// SpatialKernelConfig specifies the hyperparameters for the spatial Gaussian process covariance kernel.
//
// In accordance with Inviolate 0 (Explicit Configuration):
//   - SignalVariance (sigma_f^2): Overall regional value variance.
//   - LengthScaleMiles (ell): Characteristic spatial correlation distance in miles.
//   - NoiseVariance (sigma_n^2): Nugget variance ensuring positive definiteness.
type SpatialKernelConfig struct {
	SignalVariance   float64 // Prior variance sigma_f^2 (> 0)
	LengthScaleMiles float64 // Spatial correlation length-scale ell in statute miles (> 0)
	NoiseVariance    float64 // Nugget variance sigma_n^2 (>= 0)
}

// DefaultSpatialKernelConfig provides standard hyperparameters for US regional freight markets.
func DefaultSpatialKernelConfig() SpatialKernelConfig {
	return SpatialKernelConfig{
		SignalVariance:   100.0, // Standard deviation of $10 / marginal driver
		LengthScaleMiles: 250.0, // 250 miles spatial correlation horizon
		NoiseVariance:    1e-4,  // Numerical nugget for conditioning
	}
}

// Validate checks that the kernel hyperparameters are strictly positive.
func (c SpatialKernelConfig) Validate() error {
	if c.SignalVariance <= 0 || math.IsNaN(c.SignalVariance) || math.IsInf(c.SignalVariance, 0) {
		return fmt.Errorf("%w: SignalVariance must be positive and finite", ErrInvalidKernelParams)
	}
	if c.LengthScaleMiles <= 0 || math.IsNaN(c.LengthScaleMiles) || math.IsInf(c.LengthScaleMiles, 0) {
		return fmt.Errorf("%w: LengthScaleMiles must be positive and finite", ErrInvalidKernelParams)
	}
	if c.NoiseVariance < 0 || math.IsNaN(c.NoiseVariance) || math.IsInf(c.NoiseVariance, 0) {
		return fmt.Errorf("%w: NoiseVariance must be non-negative and finite", ErrInvalidKernelParams)
	}
	return nil
}

// BuildSpatialCovariance constructs an m x m spatial covariance matrix given pairwise distance matrix D.
//
// Formula:
//
//	\Sigma_{ij} = \sigma_f^2 \exp\left( - \frac{D_{ij}^2}{2 \ell^2} \right) + \sigma_n^2 \delta_{ij}
func BuildSpatialCovariance(distances *DenseMatrix, cfg SpatialKernelConfig) (*DenseMatrix, error) {
	if distances == nil {
		return nil, errors.New("pkg/math: distance matrix cannot be nil")
	}
	if distances.rows != distances.cols {
		return nil, ErrNotSquare
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	m := distances.rows
	cov := NewDenseMatrix(m, m)
	twoL2 := 2.0 * cfg.LengthScaleMiles * cfg.LengthScaleMiles

	for i := 0; i < m; i++ {
		for j := 0; j < m; j++ {
			d := distances.At(i, j)
			val := cfg.SignalVariance * math.Exp(-(d*d)/twoL2)
			if i == j {
				val += cfg.NoiseVariance
			}
			cov.Set(i, j, val)
		}
	}

	return cov, nil
}

// CorrelatedKnowledgeGradient models a multivariate Gaussian belief state over m spatial alternative values:
//
//	\theta \sim \mathcal{N}(\mu, \Sigma)
//
// In accordance with Powell (2022) / Frazier et al. (2009):
//   - Performs recursive Bayesian updates upon observing noisy subgradient samples y_k at location k.
//   - Generalizes information across correlated spatial regions via the joint covariance matrix \Sigma.
//   - Fully immutable: all update methods return fresh pointers (Inviolate 5).
type CorrelatedKnowledgeGradient struct {
	dim        int
	mean       []float64
	covariance *DenseMatrix
}

// NewCorrelatedKnowledgeGradient initializes an immutable CKG model.
func NewCorrelatedKnowledgeGradient(priorMean []float64, priorCovariance *DenseMatrix) (*CorrelatedKnowledgeGradient, error) {
	if priorCovariance == nil {
		return nil, errors.New("pkg/math: prior covariance matrix cannot be nil")
	}
	if priorCovariance.rows != priorCovariance.cols {
		return nil, ErrNotSquare
	}
	dim := priorCovariance.rows
	if len(priorMean) != dim {
		return nil, fmt.Errorf("%w: prior mean length %d != covariance dimension %d", ErrInvalidCKGDimension, len(priorMean), dim)
	}

	copiedMean := make([]float64, dim)
	copy(copiedMean, priorMean)

	return &CorrelatedKnowledgeGradient{
		dim:        dim,
		mean:       copiedMean,
		covariance: priorCovariance.Clone(),
	}, nil
}

// Dimension returns the number of correlated alternatives / regions m.
func (ckg *CorrelatedKnowledgeGradient) Dimension() int {
	return ckg.dim
}

// Mean returns an immutable deep copy of the current mean vector \mu_t (Inviolate 5).
func (ckg *CorrelatedKnowledgeGradient) Mean() []float64 {
	copied := make([]float64, ckg.dim)
	copy(copied, ckg.mean)
	return copied
}

// MeanAt returns the current mean value for alternative k (0-indexed).
func (ckg *CorrelatedKnowledgeGradient) MeanAt(k int) float64 {
	if k < 0 || k >= ckg.dim {
		panic(fmt.Sprintf("pkg/math: CKG index %d out of bounds (dim %d)", k, ckg.dim))
	}
	return ckg.mean[k]
}

// Covariance returns an immutable deep copy of the current covariance matrix \Sigma_t (Inviolate 5).
func (ckg *CorrelatedKnowledgeGradient) Covariance() *DenseMatrix {
	return ckg.covariance.Clone()
}

// VarianceAt returns the marginal posterior variance \Sigma_{kk} for alternative k.
func (ckg *CorrelatedKnowledgeGradient) VarianceAt(k int) float64 {
	if k < 0 || k >= ckg.dim {
		panic(fmt.Sprintf("pkg/math: CKG index %d out of bounds (dim %d)", k, ckg.dim))
	}
	return ckg.covariance.At(k, k)
}

// UpdateBayesian performs recursive Kalman-Bayes conditioning upon observing sample y_k at index k.
//
// Observation Model:
//
//	y_k = \theta_k + \epsilon, \quad \epsilon \sim \mathcal{N}(0, \sigma_\epsilon^2)
//
// Equations:
//  1. Observation scalar variance: \gamma_k^2 = \Sigma_{t, kk} + \sigma_\epsilon^2
//  2. Kalman Gain: K = \frac{\Sigma_t[:, k]}{\gamma_k^2}
//  3. Posterior Mean: \mu_{t+1} = \mu_t + K (y_k - \mu_{t, k})
//  4. Posterior Covariance: \Sigma_{t+1} = \Sigma_t - \frac{\Sigma_t[:, k] \Sigma_t[:, k]^T}{\gamma_k^2}
//
// Returns a newly allocated *CorrelatedKnowledgeGradient instance (Inviolate 5).
func (ckg *CorrelatedKnowledgeGradient) UpdateBayesian(k int, observedValue float64, observationVariance float64) (*CorrelatedKnowledgeGradient, error) {
	if k < 0 || k >= ckg.dim {
		return nil, fmt.Errorf("%w: observation index %d out of range (dim %d)", ErrInvalidCKGDimension, k, ckg.dim)
	}
	if observationVariance < 0 || math.IsNaN(observationVariance) || math.IsInf(observationVariance, 0) {
		return nil, errors.New("pkg/math: observation variance must be non-negative and finite")
	}
	if math.IsNaN(observedValue) || math.IsInf(observedValue, 0) {
		return nil, errors.New("pkg/math: observed value must be finite")
	}

	// Extract k-th column of Sigma_t
	sigmaColK := make([]float64, ckg.dim)
	for i := 0; i < ckg.dim; i++ {
		sigmaColK[i] = ckg.covariance.At(i, k)
	}

	// Scalar observation variance: gamma_k^2 = Sigma_{kk} + sigma_eps^2
	gammaSq := sigmaColK[k] + observationVariance
	if gammaSq <= 1e-15 {
		// Zero uncertainty: posterior is identical to prior
		return ckg.Clone(), nil
	}

	// Innovation: y_k - mu_{t, k}
	innovation := observedValue - ckg.mean[k]

	// Compute updated mean: mu_{t+1} = mu_t + (sigmaColK / gammaSq) * innovation
	newMean := make([]float64, ckg.dim)
	for i := 0; i < ckg.dim; i++ {
		gain := sigmaColK[i] / gammaSq
		newMean[i] = ckg.mean[i] + gain*innovation
	}

	// Compute updated covariance: Sigma_{t+1} = Sigma_t - (sigmaColK * sigmaColK^T) / gammaSq
	newCov := NewDenseMatrix(ckg.dim, ckg.dim)
	for i := 0; i < ckg.dim; i++ {
		for j := 0; j < ckg.dim; j++ {
			priorVal := ckg.covariance.At(i, j)
			reduction := (sigmaColK[i] * sigmaColK[j]) / gammaSq
			postVal := priorVal - reduction
			// Enforce non-negative diagonal variance
			if i == j && postVal < 0 {
				postVal = 0.0
			}
			newCov.Set(i, j, postVal)
		}
	}

	return &CorrelatedKnowledgeGradient{
		dim:        ckg.dim,
		mean:       newMean,
		covariance: newCov,
	}, nil
}

// ValueOfInformation computes the standard single-alternative Knowledge Gradient factor:
//
//	\tilde{\sigma}_k = \frac{\Sigma_{t} e_k}{\sqrt{\Sigma_{t, kk} + \sigma_\epsilon^2}}
//
// Returns the change vector \tilde{\sigma}_k \in \mathbb{R}^m representing the magnitude of
// epistemic revision across all alternatives if alternative k is measured.
func (ckg *CorrelatedKnowledgeGradient) ValueOfInformation(k int, observationVariance float64) ([]float64, error) {
	if k < 0 || k >= ckg.dim {
		return nil, fmt.Errorf("%w: index %d out of bounds (dim %d)", ErrInvalidCKGDimension, k, ckg.dim)
	}

	denom := math.Sqrt(ckg.covariance.At(k, k) + observationVariance)
	if denom <= 1e-15 {
		return make([]float64, ckg.dim), nil
	}

	tildeSigma := make([]float64, ckg.dim)
	for i := 0; i < ckg.dim; i++ {
		tildeSigma[i] = ckg.covariance.At(i, k) / denom
	}

	return tildeSigma, nil
}

// Clone returns an exact deep copy of the CKG model (Inviolate 5).
func (ckg *CorrelatedKnowledgeGradient) Clone() *CorrelatedKnowledgeGradient {
	copiedMean := make([]float64, ckg.dim)
	copy(copiedMean, ckg.mean)

	return &CorrelatedKnowledgeGradient{
		dim:        ckg.dim,
		mean:       copiedMean,
		covariance: ckg.covariance.Clone(),
	}
}
