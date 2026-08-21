package policy

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/feasibility"
	"github.com/optimaldynamics/project-mittens/pkg/logging"
	"github.com/optimaldynamics/project-mittens/pkg/telemetry"
)

// CFAParameters holds the tunable parameter vector theta for the Cost Function Approximation policy.
type CFAParameters struct {
	ThetaEmpty float64 // Multiplier shift for empty deadhead cost (e.g. 1.0 = nominal, 1.2 = 20% penalty)
	ThetaHome  float64 // Multiplier shift for empty-to-home distance penalty
	ThetaDwell float64 // Multiplier shift for early dwell / holding duration
	ThetaRisk  float64 // Multiplier for competitor spot price uncertainty risk premium
}

// DefaultCFAParameters returns nominal unshifted CFA parameters (theta = 1.0).
func DefaultCFAParameters() CFAParameters {
	return CFAParameters{
		ThetaEmpty: 1.0,
		ThetaHome:  1.0,
		ThetaDwell: 1.0,
		ThetaRisk:  0.0,
	}
}

// ToSlice converts the parameters to a float64 slice for SPSA gradient optimization.
func (p CFAParameters) ToSlice() []float64 {
	return []float64{p.ThetaEmpty, p.ThetaHome, p.ThetaDwell, p.ThetaRisk}
}

// CFAParametersFromSlice constructs CFAParameters from a parameter vector.
func CFAParametersFromSlice(s []float64) CFAParameters {
	p := DefaultCFAParameters()
	if len(s) > 0 {
		p.ThetaEmpty = s[0]
	}
	if len(s) > 1 {
		p.ThetaHome = s[1]
	}
	if len(s) > 2 {
		p.ThetaDwell = s[2]
	}
	if len(s) > 3 {
		p.ThetaRisk = s[3]
	}
	return p
}

// CFAPolicy implements a parametric Cost Function Approximation (CFA) policy
// over the MOMDP state space S_t = (R_t, I_t, b_t).
//
// In accordance with Powell (2022) Class 2 Policies:
//
//	X^{CFA}_t(S_t | \theta) = \arg\max_{x \in \mathcal{X}_t} \sum_{(d, \ell) \in x} \bar{C}(d, \ell | \theta)
//
// Where \bar{C}(d, \ell | \theta) is the modified parametric contribution function.
type CFAPolicy[C model.CompetitorScale] struct {
	params        CFAParameters
	costCfg       model.CostConfig
	feasCfg       model.FeasibilityConfig
	filter        *feasibility.ConcurrentFilter
	matcher       *BipartiteMatcher
	regionManager *model.RegionManager
	logger        *slog.Logger
}

// NewCFAPolicy constructs a new CFAPolicy.
func NewCFAPolicy[C model.CompetitorScale](
	params CFAParameters,
	costCfg model.CostConfig,
	feasCfg model.FeasibilityConfig,
	rm *model.RegionManager,
) *CFAPolicy[C] {
	if rm == nil {
		rm = model.NewRegionManager(1.0, nil)
	}
	return &CFAPolicy[C]{
		params:        params,
		costCfg:       costCfg,
		feasCfg:       feasCfg,
		filter:        feasibility.NewConcurrentFilter(),
		matcher:       NewBipartiteMatcher(),
		regionManager: rm,
		logger:        logging.NewNop(),
	}
}

// WithLogger sets the structured logger for this policy instance.
func (p *CFAPolicy[C]) WithLogger(logger *slog.Logger) *CFAPolicy[C] {
	if logger != nil {
		p.logger = logger
	}
	return p
}

// Name returns the descriptive name of the CFA policy.
func (p *CFAPolicy[C]) Name() string {
	return "CFA_Parametric"
}

// Parameters returns the active theta parameters.
func (p *CFAPolicy[C]) Parameters() CFAParameters {
	return p.params
}

// SetParameters updates the parametric vector theta (used by offline SPSA optimizer).
func (p *CFAPolicy[C]) SetParameters(params CFAParameters) {
	p.params = params
}

// Evaluate computes the optimal carrier dispatch decision under the active CFA parameters.
func (p *CFAPolicy[C]) Evaluate(
	ctx context.Context,
	state *model.State[C],
) (*model.Action, DecisionProvenance, error) {
	if state == nil {
		return nil, DecisionProvenance{}, fmt.Errorf("cfa: cannot evaluate nil state")
	}

	ctx, span := telemetry.StartSpan(ctx, "Policy.CFA.Evaluate")
	defer span.End()

	logger := logging.FromContext(ctx, p.logger)

	prov := NewDecisionProvenance(p.Name(), state, p.params.ToSlice())

	res := state.Resource()
	drivers := res.Drivers()
	loads := res.Loads()
	competitorScale := 0
	if state.Belief() != nil {
		competitorScale = state.Belief().Scale().CompetitorDimension()
	}
	span.SetAttributes(telemetry.OptimizationSpanAttributes("CFA", len(drivers), len(loads), competitorScale)...)

	if len(drivers) == 0 || len(loads) == 0 {
		logger.DebugContext(ctx, "cfa evaluation skipped: empty drivers or loads",
			slog.Int("driver_count", len(drivers)),
			slog.Int("load_count", len(loads)),
		)
		return model.NewAction(nil, nil), prov, nil
	}

	logger.DebugContext(ctx, "cfa starting candidate filtering",
		slog.Int("driver_count", len(drivers)),
		slog.Int("load_count", len(loads)),
	)

	// 1. Generate feasible candidate arcs concurrently
	filterCfg := feasibility.FilterConfig{
		Feasibility: p.feasCfg,
	}
	arcs, err := p.filter.FilterCandidates(ctx, drivers, loads, filterCfg)
	if err != nil {
		logger.ErrorContext(ctx, "cfa candidate filtering failed", slog.String("error", err.Error()))
		return nil, DecisionProvenance{}, fmt.Errorf("cfa: candidate filtering failed: %w", err)
	}

	logger.DebugContext(ctx, "cfa candidate filtering complete",
		slog.Int("feasible_arcs", len(arcs)),
	)

	// 2. Score all candidate arcs under parametric cost function (fail closed on missing entities)
	_, scoreSpan := telemetry.StartSpan(ctx, "Policy.CFA.ScoreCandidateArcs")
	evals := make([]CandidateEvaluation, len(arcs))
	for i, arc := range arcs {
		driver, okD := res.GetDriver(arc.DriverID)
		if !okD {
			scoreSpan.End()
			return nil, DecisionProvenance{}, fmt.Errorf("cfa: driver %s not found in resource state", arc.DriverID)
		}
		load, okL := res.GetLoad(arc.LoadID)
		if !okL {
			scoreSpan.End()
			return nil, DecisionProvenance{}, fmt.Errorf("cfa: load %s not found in resource state", arc.LoadID)
		}

		costBreakdown := CalculateTripCost(driver, load, arc, p.costCfg)

		// Parametric shifts: (theta - 1.0) * cost component
		cfaAdj := -((p.params.ThetaEmpty-1.0)*costBreakdown.EmptyCost +
			(p.params.ThetaHome-1.0)*costBreakdown.EmptyToHomeCost +
			(p.params.ThetaDwell-1.0)*costBreakdown.DwellCost)

		// Competitor risk premium (if spot price risk parameter is active)
		riskPremium := 0.0
		if p.params.ThetaRisk > 0 {
			// Risk penalty proportional to empty miles in unfamiliar competitor territory
			riskPremium = p.params.ThetaRisk * costBreakdown.EmptyCost * 0.1
		}

		totalScore := costBreakdown.NetContribution + cfaAdj - riskPremium
		destRegion := p.regionManager.GetRegionID(load.Destination)

		evals[i] = CandidateEvaluation{
			DriverID:           arc.DriverID,
			LoadID:             arc.LoadID,
			CostBreakdown:      costBreakdown,
			CFAAdjustment:      cfaAdj,
			VFAValue:           0.0,
			TotalScore:         totalScore,
			PostDecisionRegion: destRegion,
			DeadheadMiles:      arc.DeadheadMiles,
			LoadedMiles:        arc.LoadedMiles,
			InsertedDwellMin:   arc.InsertedDwellMin,
			InsertedRestMin:    arc.InsertedRestMin,
			TotalTripMin:       arc.TotalTripMin,
			IsAssigned:         false,
		}
	}
	scoreSpan.End()

	// 3. Solve 1-to-1 matching via deterministic bipartite matcher
	epoch := drivers[0].AvailableEpoch
	matches, sortedEvals, totalObj, totalNetContrib := p.matcher.SolveMatchingWithContext(ctx, evals, epoch, false)

	logger.InfoContext(ctx, "cfa optimization completed",
		slog.String("policy", p.Name()),
		slog.Int("candidates", len(evals)),
		slog.Int("matches", len(matches)),
		slog.Float64("total_objective", totalObj),
		slog.Float64("net_contribution", totalNetContrib),
	)

	// 4. Construct Action and DecisionProvenance
	action := model.NewAction(matches, nil)

	prov.BatchEpoch = epoch
	prov.EvaluatedArcs = sortedEvals
	prov.MatchedCount = len(matches)
	prov.TotalNetContribution = totalNetContrib
	prov.TotalObjectiveValue = totalObj

	return action, prov, nil
}
