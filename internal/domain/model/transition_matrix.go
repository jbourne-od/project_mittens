package model

import (
	"errors"
	"fmt"
	"sort"

	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

var (
	// ErrTransitionMatrixDimensionMismatch is returned when matrix dimensions do not match state keys.
	ErrTransitionMatrixDimensionMismatch = errors.New("domain/model: transition matrix dimension mismatch")
	// ErrTransitionMatrixRowSimplex is returned when a transition matrix row violates the probability simplex.
	ErrTransitionMatrixRowSimplex = errors.New("domain/model: transition matrix row does not sum to 1.0 within tolerance")
)

// TransitionMatrix represents an immutable, row-stochastic transition probability matrix
// governing latent competitive posture dynamics:
//
//	T_{i,j} = P(\Theta_{t+1} = \Theta_j \mid \Theta_t = \Theta_i, a_t)
//
// In accordance with Inviolate 5 (State Immutability) and Inviolate 8 (Fail-Closed Simplex Stability):
//   - All rows are verified to sum to 1.0 \pm 1e-9 on the unit simplex.
//   - Internal storage is deep-copied and immutable once allocated.
type TransitionMatrix struct {
	stateKeys []string
	keyIndex  map[string]int
	matrix    [][]float64 // matrix[i][j] where i=prior state index, j=next state index
	dim       int
}

// NewTransitionMatrix constructs and validates a row-stochastic transition matrix for the given state keys.
//
// In accordance with Principle 2 (Deterministic Reproducibility), state keys are sorted canonically.
func NewTransitionMatrix(stateKeys []string, data [][]float64) (*TransitionMatrix, error) {
	k := len(stateKeys)
	if k == 0 {
		return nil, ErrBeliefEmpty
	}
	if len(data) != k {
		return nil, fmt.Errorf("%w: expected %d rows, got %d", ErrTransitionMatrixDimensionMismatch, k, len(data))
	}

	// Canonical sorting of keys
	sortedKeys := make([]string, k)
	copy(sortedKeys, stateKeys)
	sort.Strings(sortedKeys)

	// Map original key order to canonical key order
	origIndex := make(map[string]int, k)
	for i, key := range stateKeys {
		if _, exists := origIndex[key]; exists {
			return nil, fmt.Errorf("%w: duplicate key %s", ErrDuplicateStateKey, key)
		}
		origIndex[key] = i
	}

	canonicalIndex := make(map[string]int, k)
	for i, key := range sortedKeys {
		canonicalIndex[key] = i
	}

	// Reorder rows and columns into canonical sorted space
	canonicalMatrix := make([][]float64, k)
	for i := 0; i < k; i++ {
		origRowIdx := origIndex[sortedKeys[i]]
		origRow := data[origRowIdx]
		if len(origRow) != k {
			return nil, fmt.Errorf("%w: row %d expected %d columns, got %d",
				ErrTransitionMatrixDimensionMismatch, i, k, len(origRow))
		}

		canonicalRow := make([]float64, k)
		for j := 0; j < k; j++ {
			origColIdx := origIndex[sortedKeys[j]]
			canonicalRow[j] = origRow[origColIdx]
		}

		// Validate that row lies on the probability simplex (Inviolate 8)
		if err := pkgmath.ValidateSimplex(canonicalRow, pkgmath.DefaultSimplexTolerance); err != nil {
			return nil, fmt.Errorf("%w: row %s: %v", ErrTransitionMatrixRowSimplex, sortedKeys[i], err)
		}

		canonicalMatrix[i] = canonicalRow
	}

	return &TransitionMatrix{
		stateKeys: sortedKeys,
		keyIndex:  canonicalIndex,
		matrix:    canonicalMatrix,
		dim:       k,
	}, nil
}

// NewIdentityTransitionMatrix constructs a static persistence transition matrix (T = I)
// where each latent state persists with probability 1.0.
func NewIdentityTransitionMatrix(stateKeys []string) *TransitionMatrix {
	k := len(stateKeys)
	sortedKeys := make([]string, k)
	copy(sortedKeys, stateKeys)
	sort.Strings(sortedKeys)

	matrix := make([][]float64, k)
	keyIndex := make(map[string]int, k)

	for i := 0; i < k; i++ {
		keyIndex[sortedKeys[i]] = i
		matrix[i] = make([]float64, k)
		matrix[i][i] = 1.0
	}

	return &TransitionMatrix{
		stateKeys: sortedKeys,
		keyIndex:  keyIndex,
		matrix:    matrix,
		dim:       k,
	}
}

// Predict executes the Chapman-Kolmogorov forward prediction step:
//
//	\bar{b}_{t+1}(j) = \sum_{i=1}^K b_t(i) \cdot T_{i,j}
//
// Parameters:
//   - priorProbs: Canonical prior probability slice b_t of length K.
//
// Returns:
//   - Predicted prior probability slice \bar{b}_{t+1} of length K.
func (tm *TransitionMatrix) Predict(priorProbs []float64) []float64 {
	if len(priorProbs) != tm.dim {
		panic(fmt.Sprintf("domain/model: Predict called with priorProbs length %d != matrix dim %d",
			len(priorProbs), tm.dim))
	}

	predicted := make([]float64, tm.dim)
	for j := 0; j < tm.dim; j++ {
		colTerms := make([]float64, tm.dim)
		for i := 0; i < tm.dim; i++ {
			colTerms[i] = priorProbs[i] * tm.matrix[i][j]
		}
		predicted[j] = pkgmath.CompensatedSum(colTerms)
	}

	return predicted
}

// Dimension returns the number of discrete states K in the transition matrix.
func (tm *TransitionMatrix) Dimension() int {
	return tm.dim
}

// StateKeys returns a deep copy of the canonically sorted state keys.
func (tm *TransitionMatrix) StateKeys() []string {
	out := make([]string, tm.dim)
	copy(out, tm.stateKeys)
	return out
}

// At returns the transition probability P(\Theta_{next} = toKey \mid \Theta_{prior} = fromKey).
func (tm *TransitionMatrix) At(fromKey, toKey string) (float64, bool) {
	fromIdx, ok1 := tm.keyIndex[fromKey]
	toIdx, ok2 := tm.keyIndex[toKey]
	if !ok1 || !ok2 {
		return 0.0, false
	}
	return tm.matrix[fromIdx][toIdx], true
}
