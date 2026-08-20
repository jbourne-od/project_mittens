// Package policy implements the four universal classes of Powell policies
// (PFAs, CFAs, VFAs, and DLAs) over the MOMDP state space.
//
// In accordance with Project Mittens AGENTS.md:
//   - Dependency Boundaries: Imports /internal/domain/model, /internal/domain/model/feasibility, /pkg/math, /pkg/logging
//   - Zero I/O, offline, zero wall-clock time or global mutable state.
//   - Inviolate 5: State immutability via value-based allocation and fresh pointers.
//   - Inviolate 6: Zero mutexes on hot paths.
package policy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/feasibility"
	"github.com/optimaldynamics/project-mittens/pkg/logging"
	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
	"github.com/optimaldynamics/project-mittens/pkg/telemetry"
)

var (
	// ErrInvalidPiecewiseSlopes is returned when initialized piecewise slopes violate concavity.
	ErrInvalidPiecewiseSlopes = errors.New("domain/policy: piecewise VFA slopes must be non-increasing (concave)")
)

// RegionSlopes represents the discrete, piecewise-linear concave marginal values for a single geographic region.
//
// In accordance with Powell (2022) / Topaloglu & Powell (2006):
//   - Slopes[k] (for k=0..K-1) represents the marginal value of having the (k+1)-th driver in this region.
//   - Invariant: Slopes[0] >= Slopes[1] >= ... >= Slopes[K-1] (Diminishing marginal returns / Concavity).
type RegionSlopes struct {
	RegionID string
	Slopes   []float64
}

// NewRegionSlopes creates and validates a RegionSlopes struct ensuring non-increasing slopes.
func NewRegionSlopes(regionID string, slopes []float64) (RegionSlopes, error) {
	if len(slopes) == 0 {
		return RegionSlopes{RegionID: regionID, Slopes: nil}, nil
	}

	copied := make([]float64, len(slopes))
	copy(copied, slopes)

	for i := 1; i < len(copied); i++ {
		if copied[i] > copied[i-1]+1e-9 {
			return RegionSlopes{}, fmt.Errorf("%w: at index %d (slope %f > previous slope %f)",
				ErrInvalidPiecewiseSlopes, i, copied[i], copied[i-1])
		}
	}

	return RegionSlopes{
		RegionID: regionID,
		Slopes:   copied,
	}, nil
}

// MarginalValue returns the marginal value of adding the (resourceLevel+1)-th driver.
//
// For resourceLevel < len(Slopes), returns Slopes[resourceLevel].
// For resourceLevel >= len(Slopes), returns the asymptotic tail slope Slopes[len-1] or 0.0 if empty.
func (rs RegionSlopes) MarginalValue(resourceLevel int) float64 {
	if len(rs.Slopes) == 0 {
		return 0.0
	}
	if resourceLevel < 0 {
		resourceLevel = 0
	}
	if resourceLevel >= len(rs.Slopes) {
		return rs.Slopes[len(rs.Slopes)-1]
	}
	return rs.Slopes[resourceLevel]
}

// TotalValue computes the cumulative integral value \bar{V}_r(R) = \sum_{j=0}^{R-1} Slopes[j].
func (rs RegionSlopes) TotalValue(resourceCount int) float64 {
	if resourceCount <= 0 || len(rs.Slopes) == 0 {
		return 0.0
	}
	sum := 0.0
	for i := 0; i < resourceCount; i++ {
		sum += rs.MarginalValue(i)
	}
	return sum
}

// PiecewiseLinearVFATable stores an immutable map of RegionSlopes indexed by geographic region ID.
//
// In accordance with Inviolate 5 (Immutability) and Inviolate 6 (Lock-Free Hot Paths),
// all update methods return fresh pointers.
type PiecewiseLinearVFATable struct {
	slopes map[string]RegionSlopes
}

// NewPiecewiseLinearVFATable creates an immutable PiecewiseLinearVFATable.
func NewPiecewiseLinearVFATable(initialSlopes map[string]RegionSlopes) *PiecewiseLinearVFATable {
	copied := make(map[string]RegionSlopes, len(initialSlopes))
	for k, v := range initialSlopes {
		copied[k] = RegionSlopes{
			RegionID: v.RegionID,
			Slopes:   append([]float64(nil), v.Slopes...),
		}
	}
	return &PiecewiseLinearVFATable{
		slopes: copied,
	}
}

// GetMarginalValue returns the marginal slope for adding the (resourceCount+1)-th driver in regionID.
func (t *PiecewiseLinearVFATable) GetMarginalValue(regionID string, resourceCount int) float64 {
	if t == nil {
		return 0.0
	}
	rs, ok := t.slopes[regionID]
	if !ok {
		return 0.0
	}
	return rs.MarginalValue(resourceCount)
}

// GetRegionSlopes returns a deep copy of the slopes for a region.
func (t *PiecewiseLinearVFATable) GetRegionSlopes(regionID string) (RegionSlopes, bool) {
	if t == nil {
		return RegionSlopes{}, false
	}
	rs, ok := t.slopes[regionID]
	if !ok {
		return RegionSlopes{}, false
	}
	return RegionSlopes{
		RegionID: rs.RegionID,
		Slopes:   append([]float64(nil), rs.Slopes...),
	}, true
}

// UpdateCAVE applies the Concave Adaptive Value Estimation (CAVE) level-clearing projection
// to update the slope at resourceLevel with observed subgradient sample observedDual.
//
// Mathematical Form (Powell 2022 / Topaloglu 2006):
//  1. Smoothed update: \tilde{v}_{k} = (1 - \alpha) v_{k} + \alpha \hat{v}
//  2. Forward pass: For j = k+1..K-1, if v_j > v_{j-1}, set v_j = v_{j-1}
//  3. Backward pass: For j = k-1..0, if v_j < v_{j+1}, set v_j = v_{j+1}
//
// Guarantees strict mathematical concavity preservation (Inviolate 1 & Principle 1).
// Returns a newly allocated *PiecewiseLinearVFATable (Inviolate 5).
func (t *PiecewiseLinearVFATable) UpdateCAVE(
	regionID string,
	resourceLevel int,
	observedDual float64,
	stepSize float64,
	maxSlopes int,
) *PiecewiseLinearVFATable {
	if stepSize <= 0 {
		stepSize = 0.1
	}
	if stepSize > 1.0 {
		stepSize = 1.0
	}
	if maxSlopes <= 0 {
		maxSlopes = 10
	}
	if resourceLevel < 0 {
		resourceLevel = 0
	}
	if resourceLevel >= maxSlopes {
		resourceLevel = maxSlopes - 1
	}

	// Copy existing slopes map
	copied := make(map[string]RegionSlopes, len(t.slopes)+1)
	for k, v := range t.slopes {
		copied[k] = RegionSlopes{
			RegionID: v.RegionID,
			Slopes:   append([]float64(nil), v.Slopes...),
		}
	}

	// Get or initialize region slopes
	curRS, exists := copied[regionID]
	var currentSlopes []float64
	if exists && len(curRS.Slopes) > 0 {
		currentSlopes = append([]float64(nil), curRS.Slopes...)
	} else {
		// Default diminishing slopes initialized from observed sample
		currentSlopes = make([]float64, maxSlopes)
		for i := 0; i < maxSlopes; i++ {
			currentSlopes[i] = observedDual * math.Pow(0.9, float64(i))
		}
	}

	// Ensure slice length is at least maxSlopes
	for len(currentSlopes) < maxSlopes {
		lastVal := 0.0
		if len(currentSlopes) > 0 {
			lastVal = currentSlopes[len(currentSlopes)-1] * 0.9
		}
		currentSlopes = append(currentSlopes, lastVal)
	}

	// 1. Smooth the target slope
	currentSlopes[resourceLevel] = (1.0-stepSize)*currentSlopes[resourceLevel] + stepSize*observedDual

	// 2. Forward pass: ensure monotonicity to the right (Slopes[j] <= Slopes[j-1])
	for j := resourceLevel + 1; j < len(currentSlopes); j++ {
		if currentSlopes[j] > currentSlopes[j-1] {
			currentSlopes[j] = currentSlopes[j-1]
		}
	}

	// 3. Backward pass: ensure monotonicity to the left (Slopes[j] >= Slopes[j+1])
	for j := resourceLevel - 1; j >= 0; j-- {
		if currentSlopes[j] < currentSlopes[j+1] {
			currentSlopes[j] = currentSlopes[j+1]
		}
	}

	copied[regionID] = RegionSlopes{
		RegionID: regionID,
		Slopes:   currentSlopes,
	}

	return &PiecewiseLinearVFATable{
		slopes: copied,
	}
}

// PiecewiseVFAPolicy implements a Value Function Approximation (VFA) policy using
// piecewise-linear concave marginal values over the post-decision resource state.
//
// In accordance with Powell (2022) Class 3:
//
//	X^{VFA}_t(S_t) = \arg\max_{x \in \mathcal{X}_t} \left( C(S_t, x) + \gamma \Delta \bar{V}_t(S^x_t(S_t, x)) \right)
type PiecewiseVFAPolicy[C model.CompetitorScale] struct {
	table         *PiecewiseLinearVFATable
	ckg           *pkgmath.CorrelatedKnowledgeGradient
	discount      float64
	costCfg       model.CostConfig
	feasCfg       model.FeasibilityConfig
	filter        *feasibility.ConcurrentFilter
	matcher       *BipartiteMatcher
	regionManager *model.RegionManager
	logger        *slog.Logger
}

// NewPiecewiseVFAPolicy constructs a new PiecewiseVFAPolicy.
func NewPiecewiseVFAPolicy[C model.CompetitorScale](
	table *PiecewiseLinearVFATable,
	ckg *pkgmath.CorrelatedKnowledgeGradient,
	discount float64,
	costCfg model.CostConfig,
	feasCfg model.FeasibilityConfig,
	rm *model.RegionManager,
) *PiecewiseVFAPolicy[C] {
	if table == nil {
		table = NewPiecewiseLinearVFATable(nil)
	}
	if discount <= 0 || discount > 1.0 {
		discount = 0.95
	}
	if rm == nil {
		rm = model.NewRegionManager(1.0, nil)
	}
	return &PiecewiseVFAPolicy[C]{
		table:         table,
		ckg:           ckg,
		discount:      discount,
		costCfg:       costCfg,
		feasCfg:       feasCfg,
		filter:        feasibility.NewConcurrentFilter(),
		matcher:       NewBipartiteMatcher(),
		regionManager: rm,
		logger:        logging.NewNop(),
	}
}

// WithLogger sets the structured logger for this policy instance.
func (p *PiecewiseVFAPolicy[C]) WithLogger(logger *slog.Logger) *PiecewiseVFAPolicy[C] {
	if logger != nil {
		p.logger = logger
	}
	return p
}

// Name returns the descriptive name of the Piecewise VFA policy.
func (p *PiecewiseVFAPolicy[C]) Name() string {
	return "VFA_PiecewiseLinearConcave"
}

// Table returns the active PiecewiseLinearVFATable.
func (p *PiecewiseVFAPolicy[C]) Table() *PiecewiseLinearVFATable {
	return p.table
}

// SetTable updates the active immutable PiecewiseLinearVFATable pointer.
func (p *PiecewiseVFAPolicy[C]) SetTable(table *PiecewiseLinearVFATable) {
	if table != nil {
		p.table = table
	}
}

// Evaluate computes the optimal carrier dispatch decision under piecewise-linear concave post-decision marginal value functions.
func (p *PiecewiseVFAPolicy[C]) Evaluate(
	ctx context.Context,
	state *model.State[C],
) (*model.Action, DecisionProvenance, error) {
	if state == nil {
		return nil, DecisionProvenance{}, fmt.Errorf("vfa_piecewise: cannot evaluate nil state")
	}

	ctx, span := telemetry.StartSpan(ctx, "Policy.PiecewiseVFA.Evaluate")
	defer span.End()

	logger := logging.FromContext(ctx, p.logger)

	res := state.Resource()
	drivers := res.Drivers()
	loads := res.Loads()
	competitorScale := 0
	if state.Belief() != nil {
		competitorScale = state.Belief().Scale().CompetitorDimension()
	}
	span.SetAttributes(telemetry.OptimizationSpanAttributes("VFA_Piecewise", len(drivers), len(loads), competitorScale)...)

	if len(drivers) == 0 || len(loads) == 0 {
		logger.DebugContext(ctx, "piecewise vfa evaluation skipped: empty drivers or loads",
			slog.Int("driver_count", len(drivers)),
			slog.Int("load_count", len(loads)),
		)
		return model.NewAction(nil, nil), DecisionProvenance{
			PolicyName: p.Name(),
		}, nil
	}

	// 1. Generate feasible candidate arcs concurrently
	filterCfg := feasibility.FilterConfig{
		Feasibility: p.feasCfg,
	}
	arcs, err := p.filter.FilterCandidates(ctx, drivers, loads, filterCfg)
	if err != nil {
		logger.ErrorContext(ctx, "piecewise vfa candidate filtering failed", slog.String("error", err.Error()))
		return nil, DecisionProvenance{}, fmt.Errorf("vfa_piecewise: candidate filtering failed: %w", err)
	}

	// Count existing drivers currently in each region (anti-bunching factor)
	regionDriverCounts := make(map[string]int)
	for _, d := range drivers {
		destRegion := p.regionManager.GetRegionID(d.CurrentLocation)
		regionDriverCounts[destRegion]++
	}

	// 2. Score candidate arcs with immediate contribution + gamma * marginal VFA slope
	_, scoreSpan := telemetry.StartSpan(ctx, "Policy.PiecewiseVFA.ScoreCandidateArcs")
	evals := make([]CandidateEvaluation, len(arcs))
	for i, arc := range arcs {
		driver, _ := res.GetDriver(arc.DriverID)
		load, _ := res.GetLoad(arc.LoadID)

		costBreakdown := CalculateTripCost(driver, load, arc, p.costCfg)
		destRegion := p.regionManager.GetRegionID(load.Destination)
		currentCount := regionDriverCounts[destRegion]

		marginalValue := p.table.GetMarginalValue(destRegion, currentCount)
		vfaVal := p.discount * marginalValue
		totalScore := costBreakdown.NetContribution + vfaVal

		evals[i] = CandidateEvaluation{
			DriverID:           arc.DriverID,
			LoadID:             arc.LoadID,
			CostBreakdown:      costBreakdown,
			CFAAdjustment:      0.0,
			VFAValue:           vfaVal,
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

	// 3. Solve exact bipartite matching
	epoch := drivers[0].AvailableEpoch
	sol := p.matcher.SolveMatchingDetailedWithContext(ctx, evals, epoch, false)

	logger.InfoContext(ctx, "piecewise vfa optimization completed",
		slog.String("policy", p.Name()),
		slog.Int("candidates", len(evals)),
		slog.Int("matches", len(sol.Matches)),
		slog.Float64("total_objective", sol.TotalObjective),
		slog.Float64("net_contribution", sol.TotalNetContribution),
	)

	// 4. Construct Action and DecisionProvenance
	action := model.NewAction(sol.Matches, nil)
	provenance := DecisionProvenance{
		PolicyName:           p.Name(),
		BatchEpoch:           epoch,
		EvaluatedArcs:        sol.Evaluations,
		MatchedCount:         len(sol.Matches),
		TotalNetContribution: sol.TotalNetContribution,
		TotalObjectiveValue:  sol.TotalObjective,
	}

	return action, provenance, nil
}
