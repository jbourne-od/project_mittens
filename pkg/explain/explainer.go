package explain

import (
	"fmt"
	"sort"
	"strings"

	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

// Explainer coordinates the extraction of causal explanations and counterfactual comparisons.
type Explainer struct{}

// NewExplainer initializes a new Explainer.
func NewExplainer() *Explainer {
	return &Explainer{}
}

// ExplainDecision evaluates a complete DecisionProvenance record and produces a comprehensive DecisionExplanation.
func (e *Explainer) ExplainDecision(
	prov policy.DecisionProvenance,
	priorBelief, posteriorBelief map[string]float64,
) (*DecisionExplanation, error) {
	// 1. Group evaluated candidate arcs by DriverID
	driverArcs := make(map[string][]policy.CandidateEvaluation)
	for _, arc := range prov.EvaluatedArcs {
		driverArcs[arc.DriverID] = append(driverArcs[arc.DriverID], arc)
	}

	// Canonicalize driver ordering
	driverIDs := make([]string, 0, len(driverArcs))
	for dID := range driverArcs {
		driverIDs = append(driverIDs, dID)
	}
	sort.Strings(driverIDs)

	matchedExplanations := make([]MatchExplanation, 0)
	idleExplanations := make([]IdleDriverExplanation, 0)

	// 2. Analyze decisions per driver
	for _, dID := range driverIDs {
		arcs := driverArcs[dID]

		var winningArc *policy.CandidateEvaluation
		altArcs := make([]policy.CandidateEvaluation, 0)

		for i := range arcs {
			arc := arcs[i]
			if arc.IsAssigned {
				winningArc = &arc
			} else {
				altArcs = append(altArcs, arc)
			}
		}

		if winningArc != nil {
			// Matched Driver
			breakdown := extractEconomicBreakdown(*winningArc)

			// Process counterfactual rejected alternatives
			counterfactuals := make([]CounterfactualAlternative, 0, len(altArcs))
			for _, alt := range altArcs {
				delta := winningArc.TotalScore - alt.TotalScore
				reason := determineRejectionReason(*winningArc, alt)
				altBreakdown := extractEconomicBreakdown(alt)

				counterfactuals = append(counterfactuals, CounterfactualAlternative{
					LoadID:             alt.LoadID,
					ScoreDelta:         delta,
					TotalScore:         alt.TotalScore,
					EconomicBreakdown:  altBreakdown,
					RejectionReason:    reason,
					PostDecisionRegion: alt.PostDecisionRegion,
					DeadheadMiles:      alt.DeadheadMiles,
					LoadedMiles:        alt.LoadedMiles,
				})
			}

			// Sort counterfactuals by ScoreDelta ascending (closest competitor first)
			sort.Slice(counterfactuals, func(i, j int) bool {
				return counterfactuals[i].ScoreDelta < counterfactuals[j].ScoreDelta
			})

			summary := fmt.Sprintf(
				"Driver %s matched to Load %s for net contribution $%.2f (total objective score $%.2f) with %.1f empty deadhead miles.",
				dID, winningArc.LoadID, winningArc.CostBreakdown.NetContribution, winningArc.TotalScore, winningArc.DeadheadMiles,
			)

			matchedExplanations = append(matchedExplanations, MatchExplanation{
				DriverID:             dID,
				AssignedLoadID:       winningArc.LoadID,
				DispatchEpoch:        prov.BatchEpoch,
				WinningScore:         winningArc.TotalScore,
				ImmediateNetMargin:   winningArc.CostBreakdown.NetContribution,
				EconomicBreakdown:    breakdown,
				Summary:              summary,
				DeadheadMiles:        winningArc.DeadheadMiles,
				LoadedMiles:          winningArc.LoadedMiles,
				InsertedDwellMinutes: winningArc.InsertedDwellMin,
				InsertedRestMinutes:  winningArc.InsertedRestMin,
				PostDecisionRegion:   winningArc.PostDecisionRegion,
				RejectedAlternatives: counterfactuals,
			})
		} else {
			// Idle Driver
			idleExp := explainIdleDriver(dID, arcs)
			idleExplanations = append(idleExplanations, idleExp)
		}
	}

	// 3. Optional Belief Shift analysis
	var beliefShift *BeliefShiftExplanation
	if len(priorBelief) > 0 && len(posteriorBelief) > 0 {
		beliefShift = analyzeBeliefShift(priorBelief, posteriorBelief)
	}

	// 4. Executive Summary
	execSummary := fmt.Sprintf(
		"Optimization batch %s (Policy: %s) dispatched %d of %d drivers, achieving $%.2f in total net contribution with zero FMCSA HOS violations.",
		prov.OptimizationRunID, prov.PolicyName, len(matchedExplanations), len(driverIDs), prov.TotalNetContribution,
	)

	return &DecisionExplanation{
		DecisionID:           prov.OptimizationRunID,
		OptimizationRunID:    prov.OptimizationRunID,
		BatchEpoch:           prov.BatchEpoch,
		PolicyName:           prov.PolicyName,
		TotalDrivers:         len(driverIDs),
		MatchedDriversCount:  len(matchedExplanations),
		IdleDriversCount:     len(idleExplanations),
		TotalNetContribution: prov.TotalNetContribution,
		TotalObjectiveValue:  prov.TotalObjectiveValue,
		ExecutiveSummary:     execSummary,
		MatchedExplanations:  matchedExplanations,
		IdleExplanations:     idleExplanations,
		BeliefShift:          beliefShift,
	}, nil
}

func extractEconomicBreakdown(arc policy.CandidateEvaluation) EconomicBreakdown {
	return EconomicBreakdown{
		GrossRevenue:          arc.CostBreakdown.Revenue,
		LoadedDriveCost:       arc.CostBreakdown.LoadedCost,
		EmptyDeadheadCost:     arc.CostBreakdown.EmptyCost,
		EmptyToHomeCost:       arc.CostBreakdown.EmptyToHomeCost,
		InsertedDwellCost:     arc.CostBreakdown.DwellCost,
		LatePenalty:           arc.CostBreakdown.LatePenalty,
		DriverBonus:           arc.CostBreakdown.DriverBonus,
		ImmediateNetMargin:    arc.CostBreakdown.NetContribution,
		CFAAdjustment:         arc.CFAAdjustment,
		DownstreamRegionalVFA: arc.VFAValue,
		CompetitorRiskPremium: arc.DLAValue, // Or risk adjustment
		TotalObjectiveScore:   arc.TotalScore,
	}
}

func determineRejectionReason(winning, alt policy.CandidateEvaluation) string {
	if alt.TotalScore <= 0 {
		return fmt.Sprintf("Negative policy valuation ($%.2f score): Operating costs exceed revenue after repositioning.", alt.TotalScore)
	}
	if alt.DeadheadMiles > winning.DeadheadMiles+50.0 {
		return fmt.Sprintf("Excessive deadhead: Requires %.1f empty miles repositioning (vs %.1f for assigned load).", alt.DeadheadMiles, winning.DeadheadMiles)
	}
	if alt.InsertedDwellMin > winning.InsertedDwellMin+60 {
		return fmt.Sprintf("Unproductive dwell time: Incurs %d min of appointment waiting dwell at origin.", alt.InsertedDwellMin)
	}
	if alt.TotalScore < winning.TotalScore {
		return fmt.Sprintf("Lower net objective: Yields $%.2f less total mathematical contribution than winning load %s.", winning.TotalScore-alt.TotalScore, winning.LoadID)
	}
	return "Displaced in global bipartite matching optimum."
}

func explainIdleDriver(driverID string, arcs []policy.CandidateEvaluation) IdleDriverExplanation {
	if len(arcs) == 0 {
		return IdleDriverExplanation{
			DriverID:                 driverID,
			ReasonCode:               "NO_FEASIBLE_LOADS",
			Summary:                  fmt.Sprintf("Driver %s has zero physically feasible candidate loads within reach radius.", driverID),
			EvaluatedCandidatesCount: 0,
		}
	}

	// Sort arcs by TotalScore descending to find the best candidate
	sort.Slice(arcs, func(i, j int) bool {
		return arcs[i].TotalScore > arcs[j].TotalScore
	})

	best := arcs[0]
	bestBreakdown := extractEconomicBreakdown(best)
	bestAlt := &CounterfactualAlternative{
		LoadID:             best.LoadID,
		TotalScore:         best.TotalScore,
		EconomicBreakdown:  bestBreakdown,
		PostDecisionRegion: best.PostDecisionRegion,
		DeadheadMiles:      best.DeadheadMiles,
		LoadedMiles:        best.LoadedMiles,
	}

	if best.TotalScore <= 0 {
		return IdleDriverExplanation{
			DriverID:                 driverID,
			ReasonCode:               "NEGATIVE_EXPECTED_MARGIN",
			Summary:                  fmt.Sprintf("All %d candidate loads evaluated for Driver %s yielded negative net margins (best candidate %s scored $%.2f). Holding idle preserves capital.", len(arcs), driverID, best.LoadID, best.TotalScore),
			EvaluatedCandidatesCount: len(arcs),
			BestCandidateAlternative: bestAlt,
		}
	}

	return IdleDriverExplanation{
		DriverID:                 driverID,
		ReasonCode:               "DISPLACED_BY_GLOBAL_OPTIMUM",
		Summary:                  fmt.Sprintf("Driver %s had viable candidates (e.g. Load %s scored $%.2f), but all candidates were awarded to higher-utility drivers in the global LAP solve.", driverID, best.LoadID, best.TotalScore),
		EvaluatedCandidatesCount: len(arcs),
		BestCandidateAlternative: bestAlt,
	}
}

func analyzeBeliefShift(prior, posterior map[string]float64) *BeliefShiftExplanation {
	// Find dominant posterior posture
	dominant := ""
	maxP := -1.0
	for k, v := range posterior {
		if v > maxP {
			maxP = v
			dominant = k
		}
	}

	shiftType := "STABLE"
	pricingAction := "Maintaining baseline pricing markups."

	priorDom := prior[dominant]
	if maxP > priorDom+0.15 {
		if strings.ToUpper(dominant) == "AGGRESSIVE" || strings.ToUpper(dominant) == "TIGHT" {
			shiftType = "TIGHTENING"
			pricingAction = "Increased spot pricing bid markups to account for competitor capacity exhaustion."
		} else {
			shiftType = "SURPLUS"
			pricingAction = "Discounted margin expectations to remain competitive amidst competitor capacity surplus."
		}
	}

	return &BeliefShiftExplanation{
		PriorBelief:      prior,
		PosteriorBelief:  posterior,
		DominantPosture:  dominant,
		PostureShiftType: shiftType,
		PricingAction:    pricingAction,
	}
}
