package policy

import (
	"sort"
)

// CandidateEvaluation records the full evaluation metrics and alternative scoring
// for a single candidate assignment arc during an optimization run.
type CandidateEvaluation struct {
	DriverID           string
	LoadID             string
	CostBreakdown      TripCostBreakdown
	CFAAdjustment      float64
	VFAValue           float64
	DLAValue           float64
	TotalScore         float64 // Final policy objective score
	PostDecisionRegion string
	DeadheadMiles      float64
	LoadedMiles        float64
	InsertedDwellMin   int
	InsertedRestMin    int
	TotalTripMin       int
	IsAssigned         bool
}

// DecisionProvenance captures the complete audit trail and evidence matrix
// for all decisions made in an optimization batch (Section 19.4 & Inviolate 4).
type DecisionProvenance struct {
	OptimizationRunID    string
	BatchEpoch           int64
	PolicyName           string
	ThetaParameters      []float64
	EvaluatedArcs        []CandidateEvaluation
	MatchedCount         int
	TotalNetContribution float64
	TotalObjectiveValue  float64
}

// Canonicalize sorts the evaluated arcs deterministically by (DriverID, LoadID).
func (dp *DecisionProvenance) Canonicalize() {
	sort.Slice(dp.EvaluatedArcs, func(i, j int) bool {
		if dp.EvaluatedArcs[i].DriverID != dp.EvaluatedArcs[j].DriverID {
			return dp.EvaluatedArcs[i].DriverID < dp.EvaluatedArcs[j].DriverID
		}
		return dp.EvaluatedArcs[i].LoadID < dp.EvaluatedArcs[j].LoadID
	})
}
