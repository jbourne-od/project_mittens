package model

import (
	"errors"
	"fmt"
	"math"

	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

var (
	// ErrFilterDegenerateObservation is returned when observation likelihoods are zero across all states.
	ErrFilterDegenerateObservation = errors.New("domain/model: observation has zero likelihood across all latent states")
)

// BeliefFilter performs recursive Bayesian state estimation over latent competitor postures \Theta_t.
//
// In accordance with MOMDP factoring (HLD Section 5.1 & 6.3):
//
//	b_{t+1}(\Theta_j) = \frac{P(o_{t+1} \mid \Theta_j, a_t) \sum_i b_t(\Theta_i) T_{i,j}(a_t)}{\sum_k P(o_{t+1} \mid \Theta_k, a_t) \sum_i b_t(\Theta_i) T_{i,k}(a_t)}
//
// In accordance with Inviolate 1 (Monopolistic Degeneracy), when C is Monopolistic (N=0),
// the filter collapses into an exact O(1) identity returning the canonical Dirac delta distribution.
type BeliefFilter[C CompetitorScale] struct {
	scale            C
	transitionMatrix *TransitionMatrix
	observationModel *MarketObservationModel
}

// NewMonopolisticFilter constructs an O(1) zero-allocation filter for the N=0 degenerate monopolistic baseline.
func NewMonopolisticFilter() *BeliefFilter[Monopolistic] {
	return &BeliefFilter[Monopolistic]{
		scale: Monopolistic{},
	}
}

// NewCompetitiveBeliefFilter constructs a Bayesian belief filter parameterized by transition dynamics and observation likelihoods.
//
// Returns an error if transition matrix keys do not match observation model profiles.
func NewCompetitiveBeliefFilter[C CompetitorScale](
	scale C,
	tm *TransitionMatrix,
	om *MarketObservationModel,
) (*BeliefFilter[C], error) {
	if tm == nil {
		return nil, errors.New("domain/model: transition matrix cannot be nil")
	}
	if om == nil {
		return nil, errors.New("domain/model: observation model cannot be nil")
	}

	// Verify that observation model contains profiles for all transition matrix states
	for _, key := range tm.StateKeys() {
		if _, ok := om.profiles[key]; !ok {
			return nil, fmt.Errorf("domain/model: observation model missing profile for latent state %s", key)
		}
	}

	return &BeliefFilter[C]{
		scale:            scale,
		transitionMatrix: tm,
		observationModel: om,
	}, nil
}

// Filter updates the competitive belief distribution b_t upon observing market feedback o_{t+1}.
//
// In accordance with Inviolate 5 (State Immutability), Filter leaves prior unaltered and returns
// a newly allocated *Belief[C] pointer.
func (f *BeliefFilter[C]) Filter(
	prior *Belief[C],
	obs *Observation,
	action *Action,
) (*Belief[C], error) {
	if prior == nil {
		return nil, errors.New("domain/model: prior belief cannot be nil")
	}

	// Inviolate 1: Monopolistic N=0 collapse
	if f.scale.CompetitorDimension() == 0 {
		return any(NewMonopolisticBelief()).(*Belief[C]), nil
	}

	if obs == nil {
		return nil, errors.New("domain/model: observation cannot be nil")
	}

	if f.transitionMatrix == nil || f.observationModel == nil {
		return nil, errors.New("domain/model: competitive filter requires transition matrix and observation model")
	}

	stateKeys := f.transitionMatrix.StateKeys()
	dim := f.transitionMatrix.Dimension()

	// 1. Prediction Step (Chapman-Kolmogorov forward transition):
	// \bar{b}_{t+1}(j) = \sum_{i=1}^K b_t(i) T_{i,j}
	priorProbs := prior.Probabilities()
	predictedPrior := f.transitionMatrix.Predict(priorProbs)

	// 2. Correction Step (Log-Space Bayes' Rule):
	// \ln \text{posterior}_j = \ln \bar{b}_{t+1}(j) + \ln P(o_{t+1} \mid \Theta_j)
	logUnnorm := make([]float64, dim)
	allNegativeInf := true

	for j, key := range stateKeys {
		if predictedPrior[j] <= 0.0 {
			logUnnorm[j] = math.Inf(-1)
			continue
		}

		logPriorProb := math.Log(predictedPrior[j])
		logLikelihood, err := f.observationModel.LogLikelihood(obs, key)
		if err != nil {
			return nil, fmt.Errorf("domain/model: failed computing likelihood for state %s: %w", key, err)
		}

		logUnnorm[j] = logPriorProb + logLikelihood
		if !math.IsInf(logUnnorm[j], -1) {
			allNegativeInf = false
		}
	}

	if allNegativeInf {
		return nil, ErrFilterDegenerateObservation
	}

	// 3. Log-Space Simplex Normalization
	posteriorProbs, err := pkgmath.LogNormalize(logUnnorm)
	if err != nil {
		return nil, fmt.Errorf("domain/model: LogNormalize failed: %w", err)
	}

	// Project to exact simplex to eliminate minor floating-point exponentiation jitter (Inviolate 8)
	posteriorProbs = pkgmath.ProjectToSimplex(posteriorProbs)

	// Construct and return immutable posterior belief
	return NewBelief(f.scale, stateKeys, posteriorProbs)
}

// TransitionMatrix returns the active transition matrix.
func (f *BeliefFilter[C]) TransitionMatrix() *TransitionMatrix {
	return f.transitionMatrix
}

// ObservationModel returns the active observation model.
func (f *BeliefFilter[C]) ObservationModel() *MarketObservationModel {
	return f.observationModel
}
