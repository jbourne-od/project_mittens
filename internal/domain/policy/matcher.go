package policy

import (
	"sort"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

// BipartiteMatcher solves the 1-to-1 driver-load assignment subproblem deterministically.
type BipartiteMatcher struct{}

// NewBipartiteMatcher returns a new BipartiteMatcher.
func NewBipartiteMatcher() *BipartiteMatcher {
	return &BipartiteMatcher{}
}

// SolveMatching performs a deterministic maximum-score bipartite assignment over candidate evaluations.
//
// In accordance with Principle 2 (Deterministic Reproducibility):
//   - Candidate evaluations are sorted by TotalScore descending.
//   - Equal-score ties are broken deterministically by DriverID ascending, then LoadID ascending.
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
