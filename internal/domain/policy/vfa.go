package policy

import (
	"context"
	"fmt"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/feasibility"
)

// VFATable stores an immutable map of post-decision state marginal values indexed by geographic region.
//
// In accordance with Inviolate 5 (Immutability) and Inviolate 6 (Lock-Free Hot Paths),
// VFATable is lock-free and returns newly allocated structures on update.
type VFATable struct {
	regionValues map[string]float64
}

// NewVFATable creates an immutable VFATable.
func NewVFATable(initialValues map[string]float64) *VFATable {
	copied := make(map[string]float64, len(initialValues))
	for k, v := range initialValues {
		copied[k] = v
	}
	return &VFATable{
		regionValues: copied,
	}
}

// GetValue returns the post-decision marginal value for a region in O(1) lock-free time.
func (t *VFATable) GetValue(regionID string) float64 {
	if t == nil {
		return 0.0
	}
	return t.regionValues[regionID]
}

// WithUpdatedValue returns a newly allocated VFATable with the updated regional value (Inviolate 5).
func (t *VFATable) WithUpdatedValue(regionID string, val float64) *VFATable {
	copied := make(map[string]float64, len(t.regionValues)+1)
	for k, v := range t.regionValues {
		copied[k] = v
	}
	copied[regionID] = val
	return &VFATable{
		regionValues: copied,
	}
}

// Snapshot returns an immutable deep copy of the regional values.
func (t *VFATable) Snapshot() map[string]float64 {
	if t == nil {
		return nil
	}
	copied := make(map[string]float64, len(t.regionValues))
	for k, v := range t.regionValues {
		copied[k] = v
	}
	return copied
}

// VFAPolicy implements a Value Function Approximation (VFA) policy
// over the MOMDP post-decision state space S^x_t.
//
// In accordance with Powell (2022) Class 3 Policies:
//
//	X^{VFA}_t(S_t) = \arg\max_{x \in \mathcal{X}_t} \left( C(S_t, x) + \gamma \bar{V}_t(S^x_t(S_t, x)) \right)
type VFAPolicy[C model.CompetitorScale] struct {
	table         *VFATable
	discount      float64
	costCfg       model.CostConfig
	feasCfg       model.FeasibilityConfig
	filter        *feasibility.ConcurrentFilter
	matcher       *BipartiteMatcher
	regionManager *model.RegionManager
}

// NewVFAPolicy constructs a new VFAPolicy.
func NewVFAPolicy[C model.CompetitorScale](
	table *VFATable,
	discount float64,
	costCfg model.CostConfig,
	feasCfg model.FeasibilityConfig,
	rm *model.RegionManager,
) *VFAPolicy[C] {
	if table == nil {
		table = NewVFATable(nil)
	}
	if discount <= 0 || discount > 1.0 {
		discount = 0.95
	}
	if rm == nil {
		rm = model.NewRegionManager(1.0, nil)
	}
	return &VFAPolicy[C]{
		table:         table,
		discount:      discount,
		costCfg:       costCfg,
		feasCfg:       feasCfg,
		filter:        feasibility.NewConcurrentFilter(),
		matcher:       NewBipartiteMatcher(),
		regionManager: rm,
	}
}

// Name returns the descriptive name of the VFA policy.
func (p *VFAPolicy[C]) Name() string {
	return "VFA_LinearPostDecision"
}

// Table returns the active VFATable.
func (p *VFAPolicy[C]) Table() *VFATable {
	return p.table
}

// SetTable updates the active immutable VFATable pointer.
func (p *VFAPolicy[C]) SetTable(table *VFATable) {
	if table != nil {
		p.table = table
	}
}

// Evaluate computes the optimal carrier dispatch decision under post-decision state value approximations.
func (p *VFAPolicy[C]) Evaluate(
	ctx context.Context,
	state *model.State[C],
) (*model.Action, DecisionProvenance, error) {
	if state == nil {
		return nil, DecisionProvenance{}, fmt.Errorf("vfa: cannot evaluate nil state")
	}

	res := state.Resource()
	drivers := res.Drivers()
	loads := res.Loads()

	if len(drivers) == 0 || len(loads) == 0 {
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
		return nil, DecisionProvenance{}, fmt.Errorf("vfa: candidate filtering failed: %w", err)
	}

	// 2. Score all candidate arcs using contribution + post-decision state value
	evals := make([]CandidateEvaluation, len(arcs))
	for i, arc := range arcs {
		driver, _ := res.GetDriver(arc.DriverID)
		load, _ := res.GetLoad(arc.LoadID)

		costBreakdown := CalculateTripCost(driver, load, arc, p.costCfg)
		destRegion := p.regionManager.GetRegionID(load.Destination)

		vfaVal := p.discount * p.table.GetValue(destRegion)
		totalScore := costBreakdown.NetContribution + vfaVal

		evals[i] = CandidateEvaluation{
			DriverID:           arc.DriverID,
			LoadID:             arc.LoadID,
			CostBreakdown:      costBreakdown,
			CFAAdjustment:      0.0,
			VFAValue:           vfaVal,
			TotalScore:         totalScore,
			PostDecisionRegion: destRegion,
			IsAssigned:         false,
		}
	}

	// 3. Solve 1-to-1 matching via deterministic bipartite matcher
	epoch := drivers[0].AvailableEpoch
	matches, sortedEvals, totalObj, totalNetContrib := p.matcher.SolveMatching(evals, epoch, false)

	// 4. Construct Action and DecisionProvenance
	action := model.NewAction(matches, nil)

	provenance := DecisionProvenance{
		PolicyName:           p.Name(),
		BatchEpoch:           epoch,
		EvaluatedArcs:        sortedEvals,
		MatchedCount:         len(matches),
		TotalNetContribution: totalNetContrib,
		TotalObjectiveValue:  totalObj,
	}

	return action, provenance, nil
}
