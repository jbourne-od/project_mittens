package model

import (
	"errors"
	"fmt"
	"sort"

	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

var (
	// ErrBeliefDimensionMismatch is returned when the number of state keys does not match probability values.
	ErrBeliefDimensionMismatch = errors.New("domain/model: belief state keys count != probabilities count")
	// ErrBeliefEmpty is returned when an empty belief state is constructed for N >= 1.
	ErrBeliefEmpty = errors.New("domain/model: belief state must have at least one latent state")
	// ErrDuplicateStateKey is returned when duplicate state keys are provided in a belief configuration.
	ErrDuplicateStateKey = errors.New("domain/model: duplicate latent state key in belief distribution")
)

// MonopolisticSingletonKey defines the canonical state key for the degenerate competitor absence state (\Theta_\emptyset).
const MonopolisticSingletonKey = "theta_empty"

// CompetitorScale defines the generic type constraint for competitor dimensions (N >= 0)
// satisfying Inviolate 3 (Competitive Genericity).
type CompetitorScale interface {
	CompetitorDimension() int
}

// Monopolistic represents the degenerate single-agent market state with zero competitors (N=0).
//
// In accordance with Inviolate 1 (Monopolistic Degeneracy), this zero-allocation singleton
// guarantees that belief filtering collapses to an exact O(1) Dirac delta distribution.
type Monopolistic struct{}

// CompetitorDimension returns 0.
func (Monopolistic) CompetitorDimension() int {
	return 0
}

// AggregatedMarket represents a collapsed market with N=1 aggregate competitor posture.
type AggregatedMarket struct {
	LatentStates []string
}

// CompetitorDimension returns 1.
func (AggregatedMarket) CompetitorDimension() int {
	return 1
}

// MultiCompetitor represents multi-agent markets with explicit competitor factor count N > 1.
type MultiCompetitor struct {
	Count int
}

// CompetitorDimension returns the explicit competitor count N.
func (mc MultiCompetitor) CompetitorDimension() int {
	return mc.Count
}

// Belief represents a probability distribution b_t over the latent competitor state space \mathcal{H},
// parameterized generically by the competitor dimension C (Inviolate 3).
//
// In accordance with Inviolate 5 (State Immutability) and Inviolate 8 (Fail-Closed Simplex Constraints):
//   - All state blocks are immutable once allocated.
//   - Probabilities are guaranteed to sum to 1.0 \pm 1e-9 on the unit simplex.
type Belief[C CompetitorScale] struct {
	scale     C
	stateKeys []string
	probs     []float64
	keyIndex  map[string]int
}

// NewMonopolisticBelief returns the canonical Dirac delta distribution centered at \Theta_\emptyset (N=0).
//
// Mathematical Formulation (Inviolate 1 & HLD Section 6.4):
//
//	b_t(\Theta) = \delta(\Theta - \Theta_\emptyset) = 1.0 if \Theta = \Theta_\emptyset, else 0.0.
func NewMonopolisticBelief() *Belief[Monopolistic] {
	return &Belief[Monopolistic]{
		scale:     Monopolistic{},
		stateKeys: []string{MonopolisticSingletonKey},
		probs:     []float64{1.0},
		keyIndex:  map[string]int{MonopolisticSingletonKey: 0},
	}
}

// NewBelief creates and validates a generic Belief distribution over arbitrary latent state spaces.
//
// In accordance with Principle 2 (Deterministic Reproducibility), state keys are sorted canonically
// so that probability lookups and array iterations remain bit-wise identical across runs.
func NewBelief[C CompetitorScale](scale C, stateKeys []string, probs []float64) (*Belief[C], error) {
	if len(stateKeys) == 0 {
		return nil, ErrBeliefEmpty
	}
	if len(stateKeys) != len(probs) {
		return nil, fmt.Errorf("%w: %d keys != %d probs", ErrBeliefDimensionMismatch, len(stateKeys), len(probs))
	}

	// Validate simplex mass conservation (Inviolate 8)
	if err := pkgmath.ValidateSimplex(probs, pkgmath.DefaultSimplexTolerance); err != nil {
		return nil, fmt.Errorf("domain/model: invalid belief simplex: %w", err)
	}

	// Canonical sorting of (key, prob) pairs to guarantee deterministic ordering
	type keyProb struct {
		key  string
		prob float64
	}
	pairs := make([]keyProb, len(stateKeys))
	for i := range stateKeys {
		pairs[i] = keyProb{key: stateKeys[i], prob: probs[i]}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].key < pairs[j].key
	})

	sortedKeys := make([]string, len(pairs))
	sortedProbs := make([]float64, len(pairs))
	kIndex := make(map[string]int, len(pairs))

	for i, p := range pairs {
		if _, exists := kIndex[p.key]; exists {
			return nil, fmt.Errorf("%w: key %s", ErrDuplicateStateKey, p.key)
		}
		sortedKeys[i] = p.key
		sortedProbs[i] = p.prob
		kIndex[p.key] = i
	}

	return &Belief[C]{
		scale:     scale,
		stateKeys: sortedKeys,
		probs:     sortedProbs,
		keyIndex:  kIndex,
	}, nil
}

// Probability returns the probability mass b_t(\Theta = stateKey) in O(1) time.
// Returns 0.0 if the stateKey does not exist in the distribution.
func (b *Belief[C]) Probability(stateKey string) float64 {
	idx, ok := b.keyIndex[stateKey]
	if !ok {
		return 0.0
	}
	return b.probs[idx]
}

// StateKeys returns a deep copy of the canonically sorted latent state keys.
func (b *Belief[C]) StateKeys() []string {
	out := make([]string, len(b.stateKeys))
	copy(out, b.stateKeys)
	return out
}

// Probabilities returns a deep copy of the probability masses on the simplex.
func (b *Belief[C]) Probabilities() []float64 {
	out := make([]float64, len(b.probs))
	copy(out, b.probs)
	return out
}

// Distribution returns a map representation of the belief state (StateKey -> Probability).
func (b *Belief[C]) Distribution() map[string]float64 {
	m := make(map[string]float64, len(b.stateKeys))
	for i, k := range b.stateKeys {
		m[k] = b.probs[i]
	}
	return m
}

// Dimension returns the number of discrete latent states in the belief support.
func (b *Belief[C]) Dimension() int {
	return len(b.probs)
}

// Scale returns the active competitor scale metadata.
func (b *Belief[C]) Scale() C {
	return b.scale
}

// Clone returns an exact, deep-copied duplicate of the Belief struct (Inviolate 5).
func (b *Belief[C]) Clone() *Belief[C] {
	copiedKeys := make([]string, len(b.stateKeys))
	copy(copiedKeys, b.stateKeys)

	copiedProbs := make([]float64, len(b.probs))
	copy(copiedProbs, b.probs)

	copiedIndex := make(map[string]int, len(b.keyIndex))
	for k, v := range b.keyIndex {
		copiedIndex[k] = v
	}

	return &Belief[C]{
		scale:     b.scale,
		stateKeys: copiedKeys,
		probs:     copiedProbs,
		keyIndex:  copiedIndex,
	}
}

// Validate asserts that the belief distribution satisfies the probability simplex invariant.
func (b *Belief[C]) Validate() error {
	return pkgmath.ValidateSimplex(b.probs, pkgmath.DefaultSimplexTolerance)
}
