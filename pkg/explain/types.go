// Package explain implements human-readable decision explainability, causal attribution,
// and counterfactual alternative comparison for Project Mittens optimization batches.
//
// In accordance with Project Mittens AGENTS.md:
//   - Dependency Boundaries: Imports /internal/domain/model, /internal/domain/policy
//   - Section 19 Compliance: Zero PII leaks, structured and sanitized explanations.
//   - Pure Execution: Stateless mathematical evaluations with zero global mutable state.
package explain

// EconomicBreakdown decomposes the financial and policy objective factors of an assignment.
type EconomicBreakdown struct {
	GrossRevenue          float64 `json:"gross_revenue"`
	LoadedDriveCost       float64 `json:"loaded_drive_cost"`
	EmptyDeadheadCost     float64 `json:"empty_deadhead_cost"`
	EmptyToHomeCost       float64 `json:"empty_to_home_cost"`
	InsertedDwellCost     float64 `json:"inserted_dwell_cost"`
	LatePenalty           float64 `json:"late_penalty"`
	DriverBonus           float64 `json:"driver_bonus"`
	ImmediateNetMargin    float64 `json:"immediate_net_margin"`
	CFAAdjustment         float64 `json:"cfa_adjustment,omitempty"`
	DownstreamRegionalVFA float64 `json:"downstream_regional_vfa,omitempty"`
	CompetitorRiskPremium float64 `json:"competitor_risk_premium,omitempty"`
	TotalObjectiveScore   float64 `json:"total_objective_score"`
}

// CounterfactualAlternative details an evaluated candidate arc that was rejected in favor of the winning match.
type CounterfactualAlternative struct {
	LoadID             string            `json:"load_id"`
	ScoreDelta         float64           `json:"score_delta"`          // Winning Score - Alternative Score
	TotalScore         float64           `json:"total_score"`          // Policy objective score of this rejected arc
	EconomicBreakdown  EconomicBreakdown `json:"economic_breakdown"`   // Financial factors of this rejected arc
	RejectionReason    string            `json:"rejection_reason"`     // Human-readable rationale
	PostDecisionRegion string            `json:"post_decision_region"` // Downstream destination region
	DeadheadMiles      float64           `json:"deadhead_miles"`
	LoadedMiles        float64           `json:"loaded_miles"`
}

// MatchExplanation contains the complete causal and economic justification for a single driver-load match.
type MatchExplanation struct {
	DriverID             string                      `json:"driver_id"`
	AssignedLoadID       string                      `json:"assigned_load_id"`
	DispatchEpoch        int64                       `json:"dispatch_epoch"`
	WinningScore         float64                     `json:"winning_score"`
	ImmediateNetMargin   float64                     `json:"immediate_net_margin"`
	EconomicBreakdown    EconomicBreakdown           `json:"economic_breakdown"`
	Summary              string                      `json:"summary"`
	DeadheadMiles        float64                     `json:"deadhead_miles"`
	LoadedMiles          float64                     `json:"loaded_miles"`
	InsertedDwellMinutes int                         `json:"inserted_dwell_minutes"`
	InsertedRestMinutes  int                         `json:"inserted_rest_minutes"`
	PostDecisionRegion   string                      `json:"post_decision_region"`
	RejectedAlternatives []CounterfactualAlternative `json:"rejected_alternatives,omitempty"`
}

// IdleDriverExplanation explains why a specific driver was kept unassigned during an epoch.
type IdleDriverExplanation struct {
	DriverID                 string                     `json:"driver_id"`
	ReasonCode               string                     `json:"reason_code"` // e.g. "NEGATIVE_EXPECTED_MARGIN", "NO_FEASIBLE_LOADS", "HOS_EXHAUSTION"
	Summary                  string                     `json:"summary"`
	EvaluatedCandidatesCount int                        `json:"evaluated_candidates_count"`
	BestCandidateAlternative *CounterfactualAlternative `json:"best_candidate_alternative,omitempty"`
}

// BeliefShiftExplanation captures how market observations updated the competitive posture belief vector.
type BeliefShiftExplanation struct {
	PriorBelief      map[string]float64 `json:"prior_belief"`
	PosteriorBelief  map[string]float64 `json:"posterior_belief"`
	DominantPosture  string             `json:"dominant_posture"`
	PostureShiftType string             `json:"posture_shift_type"` // e.g. "TIGHTENING", "SURPLUS", "STABLE"
	PricingAction    string             `json:"pricing_action"`
}

// DecisionExplanation is the master explainability record for an entire optimization epoch.
type DecisionExplanation struct {
	DecisionID           string                  `json:"decision_id"`
	OptimizationRunID    string                  `json:"optimization_run_id"`
	BatchEpoch           int64                   `json:"batch_epoch"`
	PolicyName           string                  `json:"policy_name"`
	TotalDrivers         int                     `json:"total_drivers"`
	MatchedDriversCount  int                     `json:"matched_drivers_count"`
	IdleDriversCount     int                     `json:"idle_drivers_count"`
	TotalNetContribution float64                 `json:"total_net_contribution"`
	TotalObjectiveValue  float64                 `json:"total_objective_value"`
	ExecutiveSummary     string                  `json:"executive_summary"`
	MatchedExplanations  []MatchExplanation      `json:"matched_explanations"`
	IdleExplanations     []IdleDriverExplanation `json:"idle_explanations"`
	BeliefShift          *BeliefShiftExplanation `json:"belief_shift,omitempty"`
}
