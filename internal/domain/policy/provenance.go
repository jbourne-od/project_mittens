package policy

import (
	"sort"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

// CandidateEvaluation records the full evaluation metrics and alternative scoring
// for a single candidate assignment arc during an optimization run.
type CandidateEvaluation struct {
	DriverID           string            `json:"driver_id"`
	LoadID             string            `json:"load_id"`
	CostBreakdown      TripCostBreakdown `json:"cost_breakdown"`
	CFAAdjustment      float64           `json:"cfa_adjustment"`
	VFAValue           float64           `json:"vfa_value"`
	DLAValue           float64           `json:"dla_value"`
	TotalScore         float64           `json:"total_score"` // Final policy objective score
	PostDecisionRegion string            `json:"post_decision_region"`
	DeadheadMiles      float64           `json:"deadhead_miles"`
	LoadedMiles        float64           `json:"loaded_miles"`
	InsertedDwellMin   int               `json:"inserted_dwell_min"`
	InsertedRestMin    int               `json:"inserted_rest_min"`
	TotalTripMin       int               `json:"total_trip_min"`
	IsAssigned         bool              `json:"is_assigned"`
}

// DecisionProvenance captures the complete audit trail and evidence matrix
// for all decisions made in an optimization batch (Article 7 & Section 19.4).
type DecisionProvenance struct {
	OptimizationRunID    string                `json:"optimization_run_id,omitempty"`
	BatchEpoch           int64                 `json:"batch_epoch"`
	PolicyName           string                `json:"policy_name"`
	ThetaParameters      []float64             `json:"theta_parameters,omitempty"`
	EvaluatedArcs        []CandidateEvaluation `json:"evaluated_arcs"`
	MatchedCount         int                   `json:"matched_count"`
	TotalNetContribution float64               `json:"total_net_contribution"`
	TotalObjectiveValue  float64               `json:"total_objective_value"`
	ActiveBelief         map[string]float64    `json:"active_belief,omitempty"`
	DriverCount          int                   `json:"driver_count"`
	LoadCount            int                   `json:"load_count"`
	CompetitorDimension  int                   `json:"competitor_dimension"`
	PricingVariables     map[string]float64    `json:"pricing_variables,omitempty"`
}

// NewDecisionProvenance constructs and initializes a standardized DecisionProvenance struct
// with state dimensions, batch epoch, policy name, active belief vector, and theta parameters.
func NewDecisionProvenance[C model.CompetitorScale](
	policyName string,
	state *model.State[C],
	theta []float64,
) DecisionProvenance {
	var epoch int64
	var driverCount, loadCount, compDim int
	var beliefMap map[string]float64

	if state != nil {
		if state.Information() != nil {
			epoch = state.Information().Epoch()
		}
		if state.Resource() != nil {
			driverCount = len(state.Resource().Drivers())
			loadCount = len(state.Resource().Loads())
			if driverCount > 0 && epoch == 0 {
				epoch = state.Resource().Drivers()[0].AvailableEpoch
			}
		}
		if state.Belief() != nil {
			compDim = state.Belief().Scale().CompetitorDimension()
			beliefMap = state.Belief().Distribution()
		}
	}

	var copiedTheta []float64
	if theta != nil {
		copiedTheta = make([]float64, len(theta))
		copy(copiedTheta, theta)
	}

	return DecisionProvenance{
		BatchEpoch:          epoch,
		PolicyName:          policyName,
		ThetaParameters:     copiedTheta,
		DriverCount:         driverCount,
		LoadCount:           loadCount,
		CompetitorDimension: compDim,
		ActiveBelief:        beliefMap,
		EvaluatedArcs:       make([]CandidateEvaluation, 0),
		PricingVariables:    nil,
	}
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
