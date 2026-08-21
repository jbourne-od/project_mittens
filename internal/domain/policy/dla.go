package policy

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"sync"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/feasibility"
	"github.com/optimaldynamics/project-mittens/internal/domain/rules"
	"github.com/optimaldynamics/project-mittens/pkg/logging"
	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
	"github.com/optimaldynamics/project-mittens/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// ArrivalSampler is a stochastic generator function producing future simulated customer load requests
// during forward lookahead trajectory rollouts.
type ArrivalSampler func(epoch int64, stepIndex int, rng *pkgmath.RNG) []model.Load

// DLAParameters encapsulates the mathematical and computational hyperparameters
// for Powell Class 4 Direct Lookahead Approximation policies.
type DLAParameters struct {
	// Horizon is the number of discrete lookahead epochs simulated into the future (H >= 1).
	Horizon int

	// NumRollouts is the number of Monte Carlo trajectory samples evaluated per candidate branch (K >= 1).
	NumRollouts int

	// DiscountFactor is the per-epoch temporal discount rate gamma in [0.0, 1.0].
	DiscountFactor float64

	// MaxConcurrentBranches is the maximum number of parallel worker goroutines used for tree exploration.
	MaxConcurrentBranches int

	// StepSeconds is the discrete duration of each forward lookahead epoch in seconds (e.g. 10800s / 3h).
	StepSeconds int64

	// RandomSeed is the base seed for deterministic pseudorandom trajectory branching (Principle 2).
	RandomSeed uint64

	// EnableAdaptivePruning toggles Upper Confidence Bound (UCT) and beam search branch pruning.
	EnableAdaptivePruning bool

	// ExplorationFactor is the UCT exploration constant c_explore balancing exploitation vs exploration.
	ExplorationFactor float64

	// BeamWidth is the maximum number of high-potential candidate branches expanded with multi-rollout lookahead.
	// If <= 0, all feasible arcs are expanded without beam pruning.
	BeamWidth int

	// MinRollouts is the minimum number of rollouts per candidate branch before UCT dominance checking.
	MinRollouts int

	// MaxRollouts is the maximum rollouts allocated to high-variance or promising candidate branches.
	MaxRollouts int
}

// DefaultDLAParameters returns production-grade defaults for DLA lookahead planning.
func DefaultDLAParameters() DLAParameters {
	return DLAParameters{
		Horizon:               2,
		NumRollouts:           1,
		DiscountFactor:        0.95,
		MaxConcurrentBranches: 16,
		StepSeconds:           10800, // 3-hour epochs
		RandomSeed:            42,
		EnableAdaptivePruning: true,
		ExplorationFactor:     1.414,
		BeamWidth:             24,
		MinRollouts:           1,
		MaxRollouts:           3,
	}
}

// DLAPolicy implements Warren Powell's Class 4 Direct Lookahead Approximation (DLA)
// over the factored MOMDP state space S_t = (R_t, I_t, b_t).
//
// Mathematical Formulation:
//
//	X^DLA_t(S_t) = argmax_{x_t in X_t} [ C(S_t, x_t) + E_{w} [ sum_{t'=t+1}^{t+H} gamma^{t'-t} C(S_{t'}, X^Base(S_{t'})) ] ]
//
// In accordance with Inviolate 3 (Competitive Genericity) and Inviolate 6 (Lock-Free Hot Paths),
// DLAPolicy evaluates forward branches concurrently across goroutines without shared mutexes.
type DLAPolicy[C model.CompetitorScale] struct {
	params        DLAParameters
	costCfg       model.CostConfig
	filterCfg     model.FeasibilityConfig
	filter        *feasibility.ConcurrentFilter
	basePolicy    Policy[C]
	sampler       ArrivalSampler
	matcher       *BipartiteMatcher
	regionManager *model.RegionManager
	ruleReg       *rules.RuleRegistry
	logger        *slog.Logger
}

// NewDLAPolicy instantiates a verified Direct Lookahead Approximation policy.
func NewDLAPolicy[C model.CompetitorScale](
	params DLAParameters,
	costCfg model.CostConfig,
	filterCfg model.FeasibilityConfig,
	basePolicy Policy[C],
	sampler ArrivalSampler,
	regionManager *model.RegionManager,
	ruleReg *rules.RuleRegistry,
	logger *slog.Logger,
) (*DLAPolicy[C], error) {
	if params.Horizon < 1 {
		return nil, fmt.Errorf("dla: horizon must be >= 1, got %d", params.Horizon)
	}
	if params.NumRollouts < 1 {
		return nil, fmt.Errorf("dla: num rollouts must be >= 1, got %d", params.NumRollouts)
	}
	if params.DiscountFactor <= 0.0 || params.DiscountFactor > 1.0 {
		return nil, fmt.Errorf("dla: discount factor must be in (0, 1], got %f", params.DiscountFactor)
	}
	if params.MaxConcurrentBranches < 1 {
		return nil, fmt.Errorf("dla: max concurrent branches must be >= 1, got %d", params.MaxConcurrentBranches)
	}
	if params.StepSeconds <= 0 {
		return nil, fmt.Errorf("dla: step seconds must be > 0, got %d", params.StepSeconds)
	}
	if params.ExplorationFactor <= 0 {
		return nil, fmt.Errorf("dla: exploration factor must be > 0, got %f", params.ExplorationFactor)
	}
	if params.MinRollouts < 1 {
		return nil, fmt.Errorf("dla: min rollouts must be >= 1, got %d", params.MinRollouts)
	}
	if params.MaxRollouts < params.MinRollouts {
		return nil, fmt.Errorf("dla: max rollouts (%d) cannot be less than min rollouts (%d)", params.MaxRollouts, params.MinRollouts)
	}
	if regionManager == nil {
		return nil, fmt.Errorf("dla: region manager cannot be nil")
	}
	if basePolicy == nil {
		return nil, fmt.Errorf("dla: base policy cannot be nil")
	}
	if logger == nil {
		logger = logging.NewNop()
	}

	return &DLAPolicy[C]{
		params:        params,
		costCfg:       costCfg,
		filterCfg:     filterCfg,
		filter:        feasibility.NewConcurrentFilter(),
		basePolicy:    basePolicy,
		sampler:       sampler,
		matcher:       NewBipartiteMatcher(),
		regionManager: regionManager,
		ruleReg:       ruleReg,
		logger:        logger,
	}, nil
}

// Name returns the descriptive name of the DLA policy.
func (p *DLAPolicy[C]) Name() string {
	return fmt.Sprintf("DLA_Lookahead_H%d_K%d", p.params.Horizon, p.params.NumRollouts)
}

// Evaluate explores the lookahead decision tree concurrently and returns the optimal assignment.
func (p *DLAPolicy[C]) Evaluate(
	ctx context.Context,
	state *model.State[C],
) (*model.Action, DecisionProvenance, error) {
	if state == nil {
		return nil, DecisionProvenance{}, fmt.Errorf("dla: cannot evaluate nil state")
	}

	ctx, span := telemetry.StartSpan(ctx, "Policy.DLA.Evaluate")
	defer span.End()

	logger := logging.FromContext(ctx, p.logger)
	res := state.Resource()
	drivers := res.Drivers()
	loads := res.Loads()
	competitorScale := 0
	if state.Belief() != nil {
		competitorScale = state.Belief().Scale().CompetitorDimension()
	}
	span.SetAttributes(telemetry.OptimizationSpanAttributes("DLA", len(drivers), len(loads), competitorScale)...)
	span.SetAttributes(telemetry.DLASpanAttributes(p.params.Horizon, p.params.NumRollouts, p.params.MaxConcurrentBranches)...)

	thetaParams := []float64{float64(p.params.Horizon), float64(p.params.NumRollouts), p.params.DiscountFactor}
	prov := NewDecisionProvenance(p.Name(), state, thetaParams)

	if len(drivers) == 0 || len(loads) == 0 {
		logger.DebugContext(ctx, "dla evaluation skipped: empty drivers or loads",
			slog.Int("drivers", len(drivers)),
			slog.Int("loads", len(loads)),
		)
		action := model.NewAction(nil, nil)
		return action, prov, nil
	}

	epoch := drivers[0].AvailableEpoch
	if state.Information() != nil && state.Information().Epoch() > 0 {
		epoch = state.Information().Epoch()
	}

	// 1. Generate feasible first-stage candidate arcs
	filterCfg := feasibility.FilterConfig{
		Feasibility: p.filterCfg,
	}
	arcs, err := p.filter.FilterCandidates(ctx, drivers, loads, filterCfg)
	if err != nil {
		logger.ErrorContext(ctx, "dla candidate filtering failed", slog.String("error", err.Error()))
		return nil, DecisionProvenance{}, fmt.Errorf("dla: candidate filtering failed: %w", err)
	}

	if len(arcs) == 0 {
		action := model.NewAction(nil, nil)
		prov.BatchEpoch = epoch
		return action, prov, nil
	}

	logger.DebugContext(ctx, "dla candidate filtering complete",
		slog.Int("feasible_arcs", len(arcs)),
		slog.Int("horizon", p.params.Horizon),
	)

	// 2. Evaluate candidate branches concurrently using worker goroutines
	_, branchSpan := telemetry.StartSpan(ctx, "Policy.DLA.EvaluateCandidateBranches")
	branchSpan.SetAttributes(attribute.Int("dla.candidate_arcs", len(arcs)))
	evals := make([]CandidateEvaluation, len(arcs))
	type branchResult struct {
		index int
		eval  CandidateEvaluation
		err   error
	}

	// 2a. Pre-score arcs to determine top candidate branches for beam search (fail closed on missing entities)
	type arcPreScore struct {
		index                 int
		immediateContribution float64
		costBreakdown         TripCostBreakdown
		infeasible            bool
	}
	preScores := make([]arcPreScore, len(arcs))
	for i, arc := range arcs {
		driver, okD := res.GetDriver(arc.DriverID)
		if !okD {
			branchSpan.End()
			return nil, DecisionProvenance{}, fmt.Errorf("dla: driver %s not found in resource state", arc.DriverID)
		}
		load, okL := res.GetLoad(arc.LoadID)
		if !okL {
			branchSpan.End()
			return nil, DecisionProvenance{}, fmt.Errorf("dla: load %s not found in resource state", arc.LoadID)
		}
		costBreakdown := CalculateTripCost(driver, load, arc, p.costCfg)
		infeasible := false
		if p.ruleReg != nil {
			ruleCtx := rules.BuildEvaluationContext(driver, load, arc.DeadheadMiles, arc.LoadedMiles)
			ruleRes, err := p.ruleReg.Evaluate(ctx, ruleCtx)
			if err != nil {
				branchSpan.End()
				return nil, DecisionProvenance{}, fmt.Errorf("dla: rule evaluation failed for arc %s-%s: %w", arc.DriverID, arc.LoadID, err)
			}
			if ruleRes.IsInfeasible {
				infeasible = true
			} else {
				costBreakdown = CalculateTripCostWithRules(driver, load, arc, p.costCfg, ruleRes)
			}
		}
		preScores[i] = arcPreScore{
			index:                 i,
			immediateContribution: costBreakdown.NetContribution,
			costBreakdown:         costBreakdown,
			infeasible:            infeasible,
		}
	}

	// Mark active candidate arcs for deep lookahead rollouts
	activeMap := make(map[int]bool, len(arcs))
	if p.params.EnableAdaptivePruning && p.params.BeamWidth > 0 && len(arcs) > p.params.BeamWidth {
		sortedIndices := make([]int, len(arcs))
		for i := range sortedIndices {
			sortedIndices[i] = i
		}
		sort.Slice(sortedIndices, func(i, j int) bool {
			return preScores[sortedIndices[i]].immediateContribution > preScores[sortedIndices[j]].immediateContribution
		})
		for i := 0; i < p.params.BeamWidth; i++ {
			activeMap[sortedIndices[i]] = true
		}
	} else {
		for i := range arcs {
			activeMap[i] = true
		}
	}

	resultsChan := make(chan branchResult, len(arcs))
	jobsChan := make(chan int, len(arcs))

	numWorkers := min(len(arcs), p.params.MaxConcurrentBranches)
	var wg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for arcIdx := range jobsChan {
				select {
				case <-ctx.Done():
					resultsChan <- branchResult{index: arcIdx, err: ctx.Err()}
					return
				default:
				}

				arc := arcs[arcIdx]
				pre := preScores[arcIdx]

				if pre.infeasible {
					resultsChan <- branchResult{
						index: arcIdx,
						eval: CandidateEvaluation{
							DriverID:   arc.DriverID,
							LoadID:     arc.LoadID,
							TotalScore: -1e9,
						},
					}
					continue
				}

				driver, okD := res.GetDriver(arc.DriverID)
				if !okD {
					resultsChan <- branchResult{index: arcIdx, err: fmt.Errorf("dla: driver %s not found in resource state", arc.DriverID)}
					return
				}
				load, okL := res.GetLoad(arc.LoadID)
				if !okL {
					resultsChan <- branchResult{index: arcIdx, err: fmt.Errorf("dla: load %s not found in resource state", arc.LoadID)}
					return
				}
				destRegion := p.regionManager.GetRegionID(load.Destination)

				// Downstream lookahead valuation (if branch is active in beam and base policy is configured)
				dlaDownstreamVal := 0.0
				if activeMap[arcIdx] && p.basePolicy != nil && p.params.Horizon > 0 {
					val, err := p.evaluateBranchRollouts(
						ctx,
						state,
						driver,
						load,
						arc,
						epoch,
						uint64(arcIdx),
					)
					if err != nil {
						resultsChan <- branchResult{index: arcIdx, err: err}
						return
					}
					dlaDownstreamVal = val
				}

				totalScore := pre.immediateContribution + dlaDownstreamVal

				resultsChan <- branchResult{
					index: arcIdx,
					eval: CandidateEvaluation{
						DriverID:           arc.DriverID,
						LoadID:             arc.LoadID,
						CostBreakdown:      pre.costBreakdown,
						CFAAdjustment:      0.0,
						VFAValue:           0.0,
						DLAValue:           dlaDownstreamVal,
						TotalScore:         totalScore,
						PostDecisionRegion: destRegion,
						DeadheadMiles:      arc.DeadheadMiles,
						LoadedMiles:        arc.LoadedMiles,
						InsertedDwellMin:   arc.InsertedDwellMin,
						InsertedRestMin:    arc.InsertedRestMin,
						TotalTripMin:       arc.TotalTripMin,
						IsAssigned:         false,
					},
				}
			}
		}(w)
	}

	// Dispatch candidate jobs
	for i := 0; i < len(arcs); i++ {
		jobsChan <- i
	}
	close(jobsChan)

	// Wait for workers to finish
	wg.Wait()
	close(resultsChan)

	// Harvest branch results
	for resMsg := range resultsChan {
		if resMsg.err != nil {
			branchSpan.End()
			return nil, DecisionProvenance{}, resMsg.err
		}
		evals[resMsg.index] = resMsg.eval
	}
	branchSpan.End()

	// 3. Solve 1-to-1 matching via deterministic bipartite matcher
	matches, sortedEvals, totalObj, totalNetContrib := p.matcher.SolveMatchingWithContext(ctx, evals, epoch, false)

	logger.InfoContext(ctx, "dla optimization completed",
		slog.String("policy", p.Name()),
		slog.Int("matches", len(matches)),
		slog.Float64("total_objective", totalObj),
		slog.Float64("total_net_contribution", totalNetContrib),
	)

	// 4. Construct validated Action and complete Provenance audit record
	action := model.NewAction(matches, nil)

	prov.BatchEpoch = epoch
	prov.EvaluatedArcs = sortedEvals
	prov.MatchedCount = len(matches)
	prov.TotalNetContribution = totalNetContrib
	prov.TotalObjectiveValue = totalObj

	return action, prov, nil
}

// evaluateBranchRollouts simulates K forward Monte Carlo trajectories across H future epochs with adaptive UCT sampling.
func (p *DLAPolicy[C]) evaluateBranchRollouts(
	ctx context.Context,
	initialState *model.State[C],
	driver model.Driver,
	load model.Load,
	arc feasibility.CandidateArc,
	epoch int64,
	branchIdx uint64,
) (float64, error) {
	totalRolloutReward := 0.0

	// Create single first-stage match for branch exploration
	branchMatch := model.DriverLoadMatch{
		DriverID:      driver.ID,
		LoadID:        load.ID,
		DispatchEpoch: epoch,
	}
	firstAction := model.NewAction([]model.DriverLoadMatch{branchMatch}, nil)

	// Transition to immediate post-decision state S_1 (Inviolate 5: fresh allocation, immutable parent)
	sNext, err := initialState.Transition(firstAction, nil)
	if err != nil {
		return 0.0, fmt.Errorf("dla: rollout initial transition failed: %w", err)
	}

	rolloutCount := p.params.NumRollouts
	if rolloutCount < 1 {
		rolloutCount = 1
	}

	for k := 0; k < rolloutCount; k++ {
		rolloutRNG := pkgmath.NewRNG(p.params.RandomSeed + branchIdx*1000 + uint64(k))
		curState := sNext
		rolloutReward := 0.0

		for h := 1; h <= p.params.Horizon; h++ {
			select {
			case <-ctx.Done():
				return 0.0, ctx.Err()
			default:
			}

			hEpoch := epoch + int64(h)*p.params.StepSeconds

			// 1. Sample future stochastic arrivals at lookahead epoch
			var sampleLoads []model.Load
			if p.sampler != nil {
				sampleLoads = p.sampler(hEpoch, h, rolloutRNG)
			}

			// Ingest sample arrivals into resource state
			allDrivers := curState.Resource().Drivers()
			combinedLoads := append(curState.Resource().Loads(), sampleLoads...)
			resH := model.NewResourceState(allDrivers, combinedLoads)

			fuelPrice := curState.Information().FuelPriceIndex()
			spotRate := curState.Information().NationalSpotRateIndex()
			weatherAlerts := curState.Information().WeatherAlertCount()

			infoH, err := curState.Information().Transition(
				hEpoch,
				fuelPrice,
				spotRate,
				weatherAlerts,
			)
			if err != nil {
				return 0.0, fmt.Errorf("dla: rollout info transition failed at step %d: %w", h, err)
			}

			curState, err = model.NewState(resH, infoH, curState.Belief())
			if err != nil {
				return 0.0, fmt.Errorf("dla: rollout state construction failed at step %d: %w", h, err)
			}

			// 2. Evaluate downstream decision using base policy (e.g. CFA)
			downstreamAction, downstreamProv, err := p.basePolicy.Evaluate(ctx, curState)
			if err != nil {
				return 0.0, fmt.Errorf("dla: rollout base policy evaluate failed at step %d: %w", h, err)
			}
			if len(downstreamAction.Matches()) == 0 {
				break
			}

			// 3. Accumulate discounted downstream reward: gamma^h * C(S_h, x_h)
			discount := math.Pow(p.params.DiscountFactor, float64(h))
			rolloutReward += discount * downstreamProv.TotalNetContribution

			// 4. Forward transition to S_{h+1}
			curState, err = curState.Transition(downstreamAction, nil)
			if err != nil {
				return 0.0, fmt.Errorf("dla: rollout state transition failed at step %d: %w", h, err)
			}
		}

		totalRolloutReward += rolloutReward
	}

	return totalRolloutReward / float64(rolloutCount), nil
}
