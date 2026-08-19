package policy

import (
	"fmt"
	"sort"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

// MatchingAlgorithm defines the optimization method used for bipartite driver-load assignment.
type MatchingAlgorithm int

const (
	// AlgorithmExactLAP uses the polynomial-time Successive Shortest Path (LAPJV) Linear Assignment solver
	// to guarantee mathematical global optimality across competing driver preferences.
	AlgorithmExactLAP MatchingAlgorithm = iota
	// AlgorithmGreedy uses score-sorted greedy assignment.
	AlgorithmGreedy
)

// InfeasibleScoreSentinel defines the large negative score for infeasible or non-evaluated driver-load arcs.
const InfeasibleScoreSentinel = -1e9

// BipartiteMatcher solves the 1-to-1 driver-load assignment subproblem deterministically.
type BipartiteMatcher struct {
	algorithm MatchingAlgorithm
}

// NewBipartiteMatcher returns a new BipartiteMatcher defaulting to AlgorithmExactLAP.
func NewBipartiteMatcher() *BipartiteMatcher {
	return &BipartiteMatcher{
		algorithm: AlgorithmExactLAP,
	}
}

// NewBipartiteMatcherWithAlgorithm returns a BipartiteMatcher configured with the specified algorithm.
func NewBipartiteMatcherWithAlgorithm(alg MatchingAlgorithm) *BipartiteMatcher {
	return &BipartiteMatcher{
		algorithm: alg,
	}
}

// SolveMatching performs a deterministic bipartite assignment over candidate evaluations.
//
// In accordance with Principle 1 (Absolute Mathematical Rigor) and Principle 2 (Deterministic Reproducibility):
//   - When algorithm is AlgorithmExactLAP, solves the exact Linear Assignment Problem (LAP) in O(M^2 N) time.
//   - When algorithm is AlgorithmGreedy, sorts by TotalScore descending with deterministic tie-breaking.
//   - Each driver and load is assigned at most once.
//   - Negative-score matches (worse than holding driver idle) are rejected unless allowNegative is true.
func (m *BipartiteMatcher) SolveMatching(
	evals []CandidateEvaluation,
	epoch int64,
	allowNegative bool,
) ([]model.DriverLoadMatch, []CandidateEvaluation, float64, float64) {
	if len(evals) == 0 {
		return nil, nil, 0, 0
	}

	if m.algorithm == AlgorithmGreedy {
		return m.solveGreedy(evals, epoch, allowNegative)
	}

	return m.solveExactLAP(evals, epoch, allowNegative)
}

// solveGreedy performs score-sorted greedy assignment.
func (m *BipartiteMatcher) solveGreedy(
	evals []CandidateEvaluation,
	epoch int64,
	allowNegative bool,
) ([]model.DriverLoadMatch, []CandidateEvaluation, float64, float64) {
	// Copy evaluations to preserve caller slice
	sorted := make([]CandidateEvaluation, len(evals))
	copy(sorted, evals)

	// Sort deterministically: TotalScore DESC, DriverID ASC, LoadID ASC
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].TotalScore != sorted[j].TotalScore {
			return sorted[i].TotalScore > sorted[j].TotalScore
		}
		if sorted[i].DriverID != sorted[j].DriverID {
			return sorted[i].DriverID < sorted[j].DriverID
		}
		return sorted[i].LoadID < sorted[j].LoadID
	})

	assignedDrivers := make(map[string]bool)
	assignedLoads := make(map[string]bool)
	var matches []model.DriverLoadMatch

	var totalObjective float64
	var totalNetContrib float64

	for i := range sorted {
		if !allowNegative && sorted[i].TotalScore <= 0 {
			continue
		}

		dID := sorted[i].DriverID
		lID := sorted[i].LoadID

		if !assignedDrivers[dID] && !assignedLoads[lID] {
			assignedDrivers[dID] = true
			assignedLoads[lID] = true
			sorted[i].IsAssigned = true

			matches = append(matches, model.DriverLoadMatch{
				DriverID:              dID,
				LoadID:                lID,
				DispatchEpoch:         epoch,
				EstimatedContribution: sorted[i].CostBreakdown.NetContribution,
			})

			totalObjective += sorted[i].TotalScore
			totalNetContrib += sorted[i].CostBreakdown.NetContribution
		}
	}

	return matches, sorted, totalObjective, totalNetContrib
}

// solveExactLAP builds the bipartite payout matrix and solves the exact Linear Assignment Problem.
func (m *BipartiteMatcher) solveExactLAP(
	evals []CandidateEvaluation,
	epoch int64,
	allowNegative bool,
) ([]model.DriverLoadMatch, []CandidateEvaluation, float64, float64) {
	// 1. Collect distinct driver IDs and load IDs canonicalized in sorted order (Principle 2)
	driverSet := make(map[string]bool)
	loadSet := make(map[string]bool)

	for _, ev := range evals {
		driverSet[ev.DriverID] = true
		loadSet[ev.LoadID] = true
	}

	driverIDs := make([]string, 0, len(driverSet))
	for d := range driverSet {
		driverIDs = append(driverIDs, d)
	}
	sort.Strings(driverIDs)

	loadIDs := make([]string, 0, len(loadSet))
	for l := range loadSet {
		loadIDs = append(loadIDs, l)
	}
	sort.Strings(loadIDs)

	numDrivers := len(driverIDs)
	numLoads := len(loadIDs)

	driverIndex := make(map[string]int, numDrivers)
	for i, d := range driverIDs {
		driverIndex[d] = i
	}
	loadIndex := make(map[string]int, numLoads)
	for j, l := range loadIDs {
		loadIndex[l] = j
	}

	// 2. Construct dense M x N payout matrix initialized to InfeasibleScoreSentinel
	matrix := make([][]float64, numDrivers)
	for i := 0; i < numDrivers; i++ {
		matrix[i] = make([]float64, numLoads)
		for j := 0; j < numLoads; j++ {
			matrix[i][j] = InfeasibleScoreSentinel
		}
	}

	evalIndexMap := make(map[int]int, len(evals)) // (i * numLoads + j) -> index in evals
	for idx, ev := range evals {
		i := driverIndex[ev.DriverID]
		j := loadIndex[ev.LoadID]
		if ev.TotalScore > matrix[i][j] {
			matrix[i][j] = ev.TotalScore
			evalIndexMap[i*numLoads+j] = idx
		}
	}

	// 3. Solve exact Linear Assignment
	assignment, err := pkgmath.SolveLAP(matrix, true, allowNegative)
	if err != nil {
		panic(fmt.Sprintf("domain/policy: exact LAP solver failed: %v", err))
	}

	// 4. Map solution back to CandidateEvaluation slice and Match tuples
	copiedEvals := make([]CandidateEvaluation, len(evals))
	copy(copiedEvals, evals)

	var matches []model.DriverLoadMatch
	var totalObjective float64
	var totalNetContrib float64

	for r := 0; r < numDrivers; r++ {
		c := assignment.RowToCol[r]
		if c == -1 {
			continue
		}

		score := matrix[r][c]
		if score <= InfeasibleScoreSentinel+1.0 {
			continue // Infeasible arc
		}
		if !allowNegative && score <= 0.0 {
			continue // Unprofitable arc
		}

		evIdx, exists := evalIndexMap[r*numLoads+c]
		if !exists {
			continue
		}

		copiedEvals[evIdx].IsAssigned = true
		dID := driverIDs[r]
		lID := loadIDs[c]
		netContrib := copiedEvals[evIdx].CostBreakdown.NetContribution

		matches = append(matches, model.DriverLoadMatch{
			DriverID:              dID,
			LoadID:                lID,
			DispatchEpoch:         epoch,
			EstimatedContribution: netContrib,
		})

		totalObjective += score
		totalNetContrib += netContrib
	}

	// Sort matches deterministically by DriverID ASC
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].DriverID < matches[j].DriverID
	})

	return matches, copiedEvals, totalObjective, totalNetContrib
}
