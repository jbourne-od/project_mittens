// Package policy implements the four universal classes of Powell policies
// (PFAs, CFAs, VFAs, and DLAs) over the belief-state MDP.
package policy

import (
	"context"
	"fmt"
	"sort"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/feasibility"
)

// PFAParameters encapsulates the tunable parameter vector theta for Policy Function Approximations.
//
// In accordance with Powell (2022, Chapter 12), PFAs use analytical functions or lookup/sorting rules
// that map state directly to action without solving an embedded argmax optimization problem.
type PFAParameters struct {
	// MaxDeadheadMiles defines the physical reachability threshold for greedy candidate evaluation.
	MaxDeadheadMiles float64
	// DeadheadWeight scales the penalty of empty transit in the greedy dispatch scoring function.
	DeadheadWeight float64
	// DwellWeight scales the penalty of driver dwell time before pickup.
	DwellWeight float64
	// RevenueWeight scales the preference for higher gross revenue loads.
	RevenueWeight float64
}

// DefaultPFAParameters returns standard parameters for greedy nearest-available dispatch rules.
func DefaultPFAParameters() PFAParameters {
	return PFAParameters{
		MaxDeadheadMiles: 500.0,
		DeadheadWeight:   1.0,
		DwellWeight:      0.5,
		RevenueWeight:    1.0,
	}
}

// PFAPolicy implements Powell (2022) Class 1 Policy Function Approximations (PFAs).
//
// Unlike CFAs, VFAs, and DLAs, a PFA evaluates decisions analytically via sequential greedy priority
// rules with deterministic conflict resolution, performing zero matrix inversion or LAP optimization.
//
// Mathematical Formulation:
//
//	X^{PFA}(S_t | \theta) = f_\theta(S_t)
type PFAPolicy[C model.CompetitorScale] struct {
	params    PFAParameters
	costCfg   model.CostConfig
	feasCfg   model.FeasibilityConfig
	filter    *feasibility.ConcurrentFilter
}

// NewPFAPolicy constructs a validated Policy Function Approximation policy.
func NewPFAPolicy[C model.CompetitorScale](
	params PFAParameters,
	costCfg model.CostConfig,
	feasCfg model.FeasibilityConfig,
) *PFAPolicy[C] {
	return &PFAPolicy[C]{
		params:  params,
		costCfg: costCfg,
		feasCfg: feasCfg,
		filter:  feasibility.NewConcurrentFilter(),
	}
}

// Name returns the descriptive name of the PFA policy.
func (p *PFAPolicy[C]) Name() string {
	return "PFA_GreedyPriorityRule"
}

// Evaluate applies the direct analytical dispatch rule f_theta(S_t) to generate a feasible matching.
func (p *PFAPolicy[C]) Evaluate(
	ctx context.Context,
	state *model.State[C],
) (*model.Action, DecisionProvenance, error) {
	if state == nil {
		return nil, DecisionProvenance{}, fmt.Errorf("pfa: state cannot be nil")
	}

	drivers := state.Resource().Drivers()
	loads := state.Resource().Loads()
	theta := []float64{p.params.MaxDeadheadMiles, p.params.DeadheadWeight, p.params.DwellWeight, p.params.RevenueWeight}
	prov := NewDecisionProvenance(p.Name(), state, theta)

	if len(drivers) == 0 || len(loads) == 0 {
		return model.NewAction(nil, nil), prov, nil
	}

	// 1. Sort drivers deterministically by available epoch (earliest availability first)
	sortedDrivers := make([]model.Driver, len(drivers))
	copy(sortedDrivers, drivers)
	sort.SliceStable(sortedDrivers, func(i, j int) bool {
		if sortedDrivers[i].AvailableEpoch != sortedDrivers[j].AvailableEpoch {
			return sortedDrivers[i].AvailableEpoch < sortedDrivers[j].AvailableEpoch
		}
		return sortedDrivers[i].ID < sortedDrivers[j].ID
	})

	// 2. Direct Sequential Greedy Dispatch (Analytical PFA rule without LAP solver)
	assignedLoads := make(map[string]bool, len(loads))
	matches := make([]model.DriverLoadMatch, 0, len(drivers))
	evaluatedArcs := make([]CandidateEvaluation, 0, len(drivers)*len(loads))
	totalNetContrib := 0.0

	for _, d := range sortedDrivers {
		type candidateOption struct {
			load          model.Load
			score         float64
			arc           feasibility.CandidateArc
			costBreakdown TripCostBreakdown
		}

		var bestOption *candidateOption

		for _, l := range loads {
			if assignedLoads[l.ID] {
				continue
			}

			// Feasibility check
			if !d.Equipment.CanHandle(l.RequiredEquipment, l.RequiredEndorsements) {
				continue
			}

			deadheadMiles := d.CurrentLocation.DistanceMiles(l.Origin)
			if deadheadMiles > p.params.MaxDeadheadMiles || deadheadMiles > p.feasCfg.MaxDeadheadMiles {
				continue
			}

			loadedMiles := l.Origin.DistanceMiles(l.Destination)
			arc := feasibility.CandidateArc{
				DriverID:      d.ID,
				LoadID:        l.ID,
				DeadheadMiles: deadheadMiles,
				LoadedMiles:   loadedMiles,
			}
			costBreakdown := CalculateTripCost(d, l, arc, p.costCfg)

			// Analytical PFA scoring rule: f_theta(driver, load)
			score := (p.params.RevenueWeight * l.Revenue) -
				(p.params.DeadheadWeight * costBreakdown.EmptyCost) -
				(p.params.DwellWeight * costBreakdown.DwellCost) -
				(costBreakdown.FixedCost + costBreakdown.LoadedCost + costBreakdown.EmptyToHomeCost)

			eval := CandidateEvaluation{
				DriverID:      d.ID,
				LoadID:        l.ID,
				CostBreakdown: costBreakdown,
				TotalScore:    score,
				DeadheadMiles: deadheadMiles,
				LoadedMiles:   loadedMiles,
				IsAssigned:    false,
			}
			evaluatedArcs = append(evaluatedArcs, eval)

			if score > 0 && (bestOption == nil || score > bestOption.score) {
				bestOption = &candidateOption{
					load:          l,
					score:         score,
					arc:           arc,
					costBreakdown: costBreakdown,
				}
			}
		}

		// If a profitable, feasible load is found, assign greedily
		if bestOption != nil {
			assignedLoads[bestOption.load.ID] = true
			m := model.DriverLoadMatch{
				DriverID:      d.ID,
				LoadID:        bestOption.load.ID,
				DispatchEpoch: d.AvailableEpoch,
			}
			matches = append(matches, m)
			totalNetContrib += bestOption.costBreakdown.NetContribution

			// Mark evaluation as assigned
			for idx := range evaluatedArcs {
				if evaluatedArcs[idx].DriverID == d.ID && evaluatedArcs[idx].LoadID == bestOption.load.ID {
					evaluatedArcs[idx].IsAssigned = true
					break
				}
			}
		}
	}

	prov.EvaluatedArcs = evaluatedArcs
	prov.MatchedCount = len(matches)
	prov.TotalNetContribution = totalNetContrib
	prov.TotalObjectiveValue = totalNetContrib

	return model.NewAction(matches, nil), prov, nil
}
