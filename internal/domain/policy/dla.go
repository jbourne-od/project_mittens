package policy

import (
	"context"
	"fmt"
	"log/slog"
	"math"
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
) *DLAPolicy[C] {
	if params.Horizon < 1 {
		params.Horizon = 1
	}
	if params.NumRollouts < 1 {
		params.NumRollouts = 1
	}
	if params.DiscountFactor < 0.0 || params.DiscountFactor > 1.0 {
		params.DiscountFactor = 0.95
	}
	if params.MaxConcurrentBranches < 1 {
		params.MaxConcurrentBranches = 16
	}
	if params.StepSeconds <= 0 {
		params.StepSeconds = 10800
	}
	if regionManager == nil {
		regionManager = model.NewRegionManager(1.0, nil)
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
	}
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

	if len(drivers) == 0 || len(loads) == 0 {
		logger.DebugContext(ctx, "dla evaluation skipped: empty drivers or loads",
			slog.Int("drivers", len(drivers)),
			slog.Int("loads", len(loads)),
		)
		action := model.NewAction(nil, nil)
		return action, DecisionProvenance{
			PolicyName: p.Name(),
		}, nil
	}

	epoch := drivers[0].AvailableEpoch

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
		return action, DecisionProvenance{
			PolicyName: p.Name(),
		}, nil
	}

	logger.DebugContext(ctx, "dla candidate filtering complete",
		slog.Int("feasible_arcs", len(arcs)),
		slog.Int("horizon", p.params.Horizon),
	)

	// 2. Evaluate all candidate branches concurrently using worker goroutines
	_, branchSpan := telemetry.StartSpan(ctx, "Policy.DLA.EvaluateCandidateBranches")
	branchSpan.SetAttributes(attribute.Int("dla.candidate_arcs", len(arcs)))
	evals := make([]CandidateEvaluation, len(arcs))
	type branchResult struct {
		index int
		eval  CandidateEvaluation
		err   error
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
				driver, _ := res.GetDriver(arc.DriverID)
				load, _ := res.GetLoad(arc.LoadID)

				// Calculate immediate first-stage trip cost & rules
				costBreakdown := CalculateTripCost(driver, load, arc, p.costCfg)
				if p.ruleReg != nil {
					ruleCtx := rules.BuildEvaluationContext(driver, load, arc.DeadheadMiles, arc.LoadedMiles)
					ruleRes, _ := p.ruleReg.Evaluate(ctx, ruleCtx)
					if ruleRes.IsInfeasible {
						resultsChan <- branchResult{
							index: arcIdx,
							eval: CandidateEvaluation{
								DriverID:   arc.DriverID,
								LoadID:     arc.LoadID,
								TotalScore: -1e9, // Prune infeasible rule match
							},
						}
						continue
					}
					costBreakdown = CalculateTripCostWithRules(driver, load, arc, p.costCfg, ruleRes)
				}

				immediateContribution := costBreakdown.NetContribution
				destRegion := p.regionManager.GetRegionID(load.Destination)

				// Downstream lookahead valuation (if base policy and horizon are configured)
				dlaDownstreamVal := 0.0
				if p.basePolicy != nil && p.params.Horizon > 0 {
					dlaDownstreamVal = p.evaluateBranchRollouts(
						ctx,
						state,
						driver,
						load,
						arc,
						epoch,
						uint64(arcIdx),
					)
				}

				totalScore := immediateContribution + dlaDownstreamVal

				resultsChan <- branchResult{
					index: arcIdx,
					eval: CandidateEvaluation{
						DriverID:           arc.DriverID,
						LoadID:             arc.LoadID,
						CostBreakdown:      costBreakdown,
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

	provenance := DecisionProvenance{
		BatchEpoch:           epoch,
		PolicyName:           p.Name(),
		ThetaParameters:      []float64{float64(p.params.Horizon), float64(p.params.NumRollouts), p.params.DiscountFactor},
		EvaluatedArcs:        sortedEvals,
		MatchedCount:         len(matches),
		TotalNetContribution: totalNetContrib,
		TotalObjectiveValue:  totalObj,
	}

	return action, provenance, nil
}

// evaluateBranchRollouts simulates K forward Monte Carlo trajectories across H future epochs.
func (p *DLAPolicy[C]) evaluateBranchRollouts(
	ctx context.Context,
	initialState *model.State[C],
	driver model.Driver,
	load model.Load,
	arc feasibility.CandidateArc,
	epoch int64,
	branchIdx uint64,
) float64 {
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
		return 0.0
	}

	for k := 0; k < p.params.NumRollouts; k++ {
		rolloutRNG := pkgmath.NewRNG(p.params.RandomSeed + branchIdx*1000 + uint64(k))
		curState := sNext
		rolloutReward := 0.0

		for h := 1; h <= p.params.Horizon; h++ {
			select {
			case <-ctx.Done():
				return 0.0
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

			infoH, err := curState.Information().Transition(
				hEpoch,
				curState.Information().NationalSpotRateIndex(),
				curState.Information().FuelPriceIndex(),
				len(sampleLoads),
			)
			if err != nil {
				break
			}

			curState, err = model.NewState(resH, infoH, curState.Belief())
			if err != nil {
				break
			}

			// 2. Evaluate downstream decision using base policy (e.g. CFA)
			downstreamAction, downstreamProv, err := p.basePolicy.Evaluate(ctx, curState)
			if err != nil || len(downstreamAction.Matches()) == 0 {
				break
			}

			// 3. Accumulate discounted downstream reward: gamma^h * C(S_h, x_h)
			discount := math.Pow(p.params.DiscountFactor, float64(h))
			rolloutReward += discount * downstreamProv.TotalNetContribution

			// 4. Forward transition to S_{h+1}
			curState, err = curState.Transition(downstreamAction, nil)
			if err != nil {
				break
			}
		}

		totalRolloutReward += rolloutReward
	}

	return totalRolloutReward / float64(p.params.NumRollouts)
}
