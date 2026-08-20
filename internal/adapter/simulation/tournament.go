package simulation

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

// TournamentConfig specifies parameterization for comparative Monte Carlo tournaments.
type TournamentConfig struct {
	Episodes          int          `json:"episodes"`
	HorizonDays       int          `json:"horizon_days"`
	DecisionStepHours int          `json:"decision_step_hours"`
	DriverCount       int          `json:"driver_count"`
	LoadsPerEpoch     int          `json:"loads_per_epoch"`
	BaseSeed          uint64       `json:"base_seed"`
	Market            MarketConfig `json:"market"`
}

// DefaultTournamentConfig returns standard benchmarking settings for N=0 vs N=1 tournaments.
func DefaultTournamentConfig() TournamentConfig {
	return TournamentConfig{
		Episodes:          30,
		HorizonDays:       5,
		DecisionStepHours: 12,
		DriverCount:       15,
		LoadsPerEpoch:     25,
		BaseSeed:          20260819,
		Market:            DefaultMarketConfig(),
	}
}

// EpisodeScore captures head-to-head metrics for a single twin simulation episode.
type EpisodeScore struct {
	EpisodeIndex               int     `json:"episode_index"`
	Seed                       uint64  `json:"seed"`
	N0_GrossRevenue            float64 `json:"n0_gross_revenue"`
	N0_OperatingCost           float64 `json:"n0_operating_cost"`
	N0_NetContribution         float64 `json:"n0_net_contribution"`
	N0_TotalWonLoads           int     `json:"n0_total_won_loads"`
	N0_TotalLostLoads          int     `json:"n0_total_lost_loads"`
	N0_WinRate                 float64 `json:"n0_win_rate"`
	N1_GrossRevenue            float64 `json:"n1_gross_revenue"`
	N1_OperatingCost           float64 `json:"n1_operating_cost"`
	N1_NetContribution         float64 `json:"n1_net_contribution"`
	N1_TotalWonLoads           int     `json:"n1_total_won_loads"`
	N1_TotalLostLoads          int     `json:"n1_total_lost_loads"`
	N1_WinRate                 float64 `json:"n1_win_rate"`
	NetContributionDelta       float64 `json:"net_contribution_delta"`
	NetContributionLiftPercent float64 `json:"net_contribution_lift_percent"`
}

// TournamentReport encapsulates aggregate statistical findings and hypothesis test results.
type TournamentReport struct {
	Config                   TournamentConfig          `json:"config"`
	Episodes                 []EpisodeScore            `json:"episodes"`
	TTest                    pkgmath.PairedTTestResult `json:"t_test"`
	DegenerateParityVerified bool                      `json:"degenerate_parity_verified"`
	N1SuperiorityVerified    bool                      `json:"n1_superiority_verified"`
	ExecutionDurationSec     float64                   `json:"execution_duration_sec"`
}

// TournamentRunner coordinates side-by-side Monte Carlo tournament rollouts.
type TournamentRunner struct {
	cfg TournamentConfig
}

// NewTournamentRunner initializes a new TournamentRunner.
func NewTournamentRunner(cfg TournamentConfig) *TournamentRunner {
	return &TournamentRunner{cfg: cfg}
}

// Run executes the complete multi-episode tournament comparing N=0 (Myopic) vs N=1 (MOMDP Belief-Filtered).
func (r *TournamentRunner) Run(ctx context.Context) (*TournamentReport, error) {
	startTime := time.Now()
	cfg := r.cfg
	if cfg.Episodes < 2 {
		cfg.Episodes = 2
	}

	episodes := make([]EpisodeScore, cfg.Episodes)
	n0Nets := make([]float64, cfg.Episodes)
	n1Nets := make([]float64, cfg.Episodes)

	for ep := 0; ep < cfg.Episodes; ep++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		epSeed := cfg.BaseSeed + uint64(ep)*7919

		// 1. Run Baseline N=0 (Myopic Monopolistic Policy)
		n0Score, err := r.runEpisodeN0(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("tournament: episode %d N=0 failed: %w", ep, err)
		}

		// 2. Run Candidate N=1 (Competitive MOMDP Belief-Filtered Policy)
		n1Score, err := r.runEpisodeN1(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("tournament: episode %d N=1 failed: %w", ep, err)
		}

		diff := n1Score.NetContribution - n0Score.NetContribution
		lift := 0.0
		if math.Abs(n0Score.NetContribution) > 1e-6 {
			lift = (diff / math.Abs(n0Score.NetContribution)) * 100.0
		}

		epResult := EpisodeScore{
			EpisodeIndex:               ep + 1,
			Seed:                       epSeed,
			N0_GrossRevenue:            n0Score.GrossRevenue,
			N0_OperatingCost:           n0Score.OperatingCost,
			N0_NetContribution:         n0Score.NetContribution,
			N0_TotalWonLoads:           n0Score.WonLoads,
			N0_TotalLostLoads:          n0Score.LostLoads,
			N0_WinRate:                 n0Score.WinRate,
			N1_GrossRevenue:            n1Score.GrossRevenue,
			N1_OperatingCost:           n1Score.OperatingCost,
			N1_NetContribution:         n1Score.NetContribution,
			N1_TotalWonLoads:           n1Score.WonLoads,
			N1_TotalLostLoads:          n1Score.LostLoads,
			N1_WinRate:                 n1Score.WinRate,
			NetContributionDelta:       diff,
			NetContributionLiftPercent: lift,
		}

		episodes[ep] = epResult
		n0Nets[ep] = n0Score.NetContribution
		n1Nets[ep] = n1Score.NetContribution
	}

	// Statistical Hypothesis Testing
	tTestResult, err := pkgmath.ComputePairedTTest(n0Nets, n1Nets)
	if err != nil {
		return nil, fmt.Errorf("tournament: failed computing paired t-test: %w", err)
	}

	// Verification Criteria:
	// 1. Degenerate Parity: Twin N=0 rollouts on identical seeds must yield identical payouts (0.0 delta)
	twinA, errA := r.runEpisodeN0(ctx, cfg.BaseSeed)
	twinB, errB := r.runEpisodeN0(ctx, cfg.BaseSeed)
	degenParity := errA == nil && errB == nil && math.Abs(twinA.NetContribution-twinB.NetContribution) < 1e-6

	// 2. Statistically significant profit superiority (p < 0.01, t > 2.5, and mean lift > 3.0%)
	n1Superiority := tTestResult.PValueOneTailed < 0.01 && tTestResult.PercentLift > 3.0 && tTestResult.TStatistic > 2.5

	return &TournamentReport{
		Config:                   cfg,
		Episodes:                 episodes,
		TTest:                    tTestResult,
		DegenerateParityVerified: degenParity,
		N1SuperiorityVerified:    n1Superiority,
		ExecutionDurationSec:     time.Since(startTime).Seconds(),
	}, nil
}

type simResult struct {
	GrossRevenue    float64
	OperatingCost   float64
	NetContribution float64
	WonLoads        int
	LostLoads       int
	WinRate         float64
}

// runEpisodeN0 runs a multi-day simulation using the naive N=0 myopic policy (no competitor modeling).
func (r *TournamentRunner) runEpisodeN0(ctx context.Context, seed uint64) (simResult, error) {
	env, err := NewMarketEnvironment(r.cfg.Market, seed)
	if err != nil {
		return simResult{}, err
	}

	rng := pkgmath.NewRNG(seed + 1)
	startEpoch := int64(1700000000)
	stepSec := int64(r.cfg.DecisionStepHours * 3600)
	totalEpochs := (r.cfg.HorizonDays * 24) / r.cfg.DecisionStepHours

	initialDrivers := GenerateTestDrivers(r.cfg.DriverCount, rng)
	initialLoads := GenerateStochasticLoads(r.cfg.LoadsPerEpoch, startEpoch, rng)
	resState := model.NewResourceState(initialDrivers, initialLoads)
	infoState, _ := model.NewInformationState(startEpoch, 2.50, 3.85, len(initialLoads))
	beliefState := model.NewMonopolisticBelief()
	state, _ := model.NewState(resState, infoState, beliefState)

	// Baseline CFA Policy with zero competitor risk adjustment
	cfaParams := policy.CFAParameters{
		ThetaEmpty: 1.0,
		ThetaHome:  1.0,
		ThetaDwell: 1.0,
		ThetaRisk:  0.0, // N=0 ignores competitor posture
	}
	costCfg := model.DefaultCostConfig()
	costCfg.EmptyToHomeRate = 0.20
	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 800.0
	cfaPol := policy.NewCFAPolicy[model.Monopolistic](cfaParams, costCfg, feasCfg, nil)

	totRev := 0.0
	totCost := 0.0
	totWon := 0
	totLost := 0

	for step := 0; step < totalEpochs; step++ {
		epoch := startEpoch + int64(step)*stepSec
		nextEpoch := epoch + stepSec

		// 1. Evaluate Policy on current state
		action, prov, err := cfaPol.Evaluate(ctx, state)
		if err != nil {
			return simResult{}, err
		}

		// 2. Step Market Environment (Censored Auction)
		outcome, _, err := env.Step(epoch, action, state.Resource().Loads())
		if err != nil {
			return simResult{}, err
		}

		// 3. Accounting & Filtering Won Matches
		wonLoadIDs := make(map[string]bool, len(outcome.WonLoads))
		for _, l := range outcome.WonLoads {
			wonLoadIDs[l.ID] = true
		}

		wonMatches := make([]model.DriverLoadMatch, 0, len(outcome.WonLoads))
		for _, m := range action.Matches() {
			if wonLoadIDs[m.LoadID] {
				wonMatches = append(wonMatches, m)
			}
		}

		totWon += len(outcome.WonLoads)
		totLost += len(outcome.LostLoads)
		for _, rev := range outcome.CarrierRevenues {
			totRev += rev
		}
		for _, arc := range prov.EvaluatedArcs {
			if arc.IsAssigned && wonLoadIDs[arc.LoadID] {
				totCost += arc.CostBreakdown.TotalCost
			}
		}

		// 4. Ingest new load arrivals and execute physical resource transition
		incomingLoads := GenerateStochasticLoads(r.cfg.LoadsPerEpoch, nextEpoch, rng)
		nextRes, err := state.Resource().Transition(wonMatches, incomingLoads)
		if err != nil {
			return simResult{}, err
		}
		nextInfo, err := state.Information().Transition(nextEpoch, 2.50, 3.85, len(incomingLoads))
		if err != nil {
			return simResult{}, err
		}
		state, err = model.NewState(nextRes, nextInfo, state.Belief())
		if err != nil {
			return simResult{}, err
		}
	}

	winRate := 0.0
	if totWon+totLost > 0 {
		winRate = float64(totWon) / float64(totWon+totLost)
	}

	return simResult{
		GrossRevenue:    totRev,
		OperatingCost:   totCost,
		NetContribution: totRev - totCost,
		WonLoads:        totWon,
		LostLoads:       totLost,
		WinRate:         winRate,
	}, nil
}

// runEpisodeN1 runs a multi-day simulation using the N=1 MOMDP Bayesian belief-filtered policy.
func (r *TournamentRunner) runEpisodeN1(ctx context.Context, seed uint64) (simResult, error) {
	env, err := NewMarketEnvironment(r.cfg.Market, seed)
	if err != nil {
		return simResult{}, err
	}

	rng := pkgmath.NewRNG(seed + 1)
	startEpoch := int64(1700000000)
	stepSec := int64(r.cfg.DecisionStepHours * 3600)
	totalEpochs := (r.cfg.HorizonDays * 24) / r.cfg.DecisionStepHours

	initialDrivers := GenerateTestDrivers(r.cfg.DriverCount, rng)
	initialLoads := GenerateStochasticLoads(r.cfg.LoadsPerEpoch, startEpoch, rng)
	resState := model.NewResourceState(initialDrivers, initialLoads)
	infoState, _ := model.NewInformationState(startEpoch, 2.50, 3.85, len(initialLoads))

	// N=1 Aggregated Market Belief over 3 competitor postures: [Aggressive, Moderate, Passive]
	marketScale := model.AggregatedMarket{LatentStates: []string{"AGGRESSIVE", "MODERATE", "PASSIVE"}}
	initBelief, _ := model.NewBelief[model.AggregatedMarket](
		marketScale,
		[]string{"AGGRESSIVE", "MODERATE", "PASSIVE"},
		[]float64{1.0 / 3.0, 1.0 / 3.0, 1.0 / 3.0},
	)
	state, _ := model.NewState(resState, infoState, initBelief)

	// Construct Transition Matrix and Observation Model for Belief Filter
	tMatrix, _ := model.NewTransitionMatrix(
		[]string{"AGGRESSIVE", "MODERATE", "PASSIVE"},
		r.cfg.Market.TransitionProb,
	)
	loadsMean := float64(r.cfg.LoadsPerEpoch)
	obsModel, _ := model.NewMarketObservationModel(map[string]model.PostureObservationProfile{
		"AGGRESSIVE": {
			ExpectedWinProbability: 0.35,
			ExpectedSpotRateMean:   2.15,
			ExpectedSpotRateStdDev: 0.10,
			ExpectedOffersMean:     loadsMean,
		},
		"MODERATE": {
			ExpectedWinProbability: 0.65,
			ExpectedSpotRateMean:   2.50,
			ExpectedSpotRateStdDev: 0.10,
			ExpectedOffersMean:     loadsMean,
		},
		"PASSIVE": {
			ExpectedWinProbability: 0.85,
			ExpectedSpotRateMean:   2.95,
			ExpectedSpotRateStdDev: 0.10,
			ExpectedOffersMean:     loadsMean,
		},
	})
	beliefFilter, _ := model.NewCompetitiveBeliefFilter[model.AggregatedMarket](
		marketScale,
		tMatrix,
		obsModel,
	)

	// N=1 CFA Policy with active competitor risk adjustment
	cfaParams := policy.CFAParameters{
		ThetaEmpty: 1.0,
		ThetaHome:  1.0,
		ThetaDwell: 1.0,
		ThetaRisk:  0.0,
	}
	costCfg := model.DefaultCostConfig()
	costCfg.EmptyToHomeRate = 0.20
	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 800.0
	cfaPol := policy.NewCFAPolicy[model.AggregatedMarket](cfaParams, costCfg, feasCfg, nil)
	compPol, err := policy.NewCompetitivePOMDPPolicy[model.AggregatedMarket](
		cfaPol,
		policy.DefaultCompetitivePricingConfig(),
	)
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed to initialize competitive policy: %w", err)
	}

	totRev := 0.0
	totCost := 0.0
	totWon := 0
	totLost := 0

	for step := 0; step < totalEpochs; step++ {
		epoch := startEpoch + int64(step)*stepSec
		nextEpoch := epoch + stepSec

		// 1. Evaluate Competitive POMDP Policy (Matching + Endogenous Spot Rate Formulation)
		action, prov, err := compPol.Evaluate(ctx, state)
		if err != nil {
			return simResult{}, err
		}

		// 2. Step Market Environment (Censored Auction)
		outcome, obs, err := env.Step(epoch, action, state.Resource().Loads())
		if err != nil {
			return simResult{}, err
		}

		// 3. Update Bayesian Belief Filter using Censored Observation (Article 8: Fail Closed)
		nextBelief, err := beliefFilter.Filter(state.Belief(), obs, action)
		if err != nil {
			return simResult{}, fmt.Errorf("tournament: belief filter update failed: %w", err)
		}

		// 5. Accounting & Filtering Won Matches
		wonLoadIDs := make(map[string]bool, len(outcome.WonLoads))
		for _, l := range outcome.WonLoads {
			wonLoadIDs[l.ID] = true
		}

		wonMatches := make([]model.DriverLoadMatch, 0, len(outcome.WonLoads))
		for _, m := range action.Matches() {
			if wonLoadIDs[m.LoadID] {
				wonMatches = append(wonMatches, m)
			}
		}

		totWon += len(outcome.WonLoads)
		totLost += len(outcome.LostLoads)
		for _, rev := range outcome.CarrierRevenues {
			totRev += rev
		}
		for _, arc := range prov.EvaluatedArcs {
			if arc.IsAssigned && wonLoadIDs[arc.LoadID] {
				totCost += arc.CostBreakdown.TotalCost
			}
		}

		// 6. Ingest new load arrivals and execute physical resource transition
		incomingLoads := GenerateStochasticLoads(r.cfg.LoadsPerEpoch, nextEpoch, rng)
		nextRes, err := state.Resource().Transition(wonMatches, incomingLoads)
		if err != nil {
			return simResult{}, err
		}
		nextInfo, err := state.Information().Transition(nextEpoch, 2.50, 3.85, len(incomingLoads))
		if err != nil {
			return simResult{}, err
		}
		state, err = model.NewState(nextRes, nextInfo, nextBelief)
		if err != nil {
			return simResult{}, err
		}
	}

	winRate := 0.0
	if totWon+totLost > 0 {
		winRate = float64(totWon) / float64(totWon+totLost)
	}

	return simResult{
		GrossRevenue:    totRev,
		OperatingCost:   totCost,
		NetContribution: totRev - totCost,
		WonLoads:        totWon,
		LostLoads:       totLost,
		WinRate:         winRate,
	}, nil
}

func GenerateTestDrivers(count int, rng *pkgmath.RNG) []model.Driver {
	cities := []model.Location{
		{NodeID: "NYC", Lat: 40.7128, Lon: -74.0060},
		{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298},
		{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880},
		{NodeID: "DAL", Lat: 32.7767, Lon: -96.7970},
		{NodeID: "PHL", Lat: 39.9526, Lon: -75.1652},
	}

	drivers := make([]model.Driver, count)
	for i := 0; i < count; i++ {
		loc := cities[rng.Intn(len(cities))]
		dID := fmt.Sprintf("DRV_%03d", i+1)
		drivers[i] = model.Driver{
			ID:                  dID,
			CurrentLocation:     loc,
			HomeLocation:        loc,
			AvailableEpoch:      1700000000,
			DriveHoursRemaining: 11.0,
			DutyHoursRemaining:  14.0,
			Equipment:           model.Equipment{Type: model.EquipDryVan},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(1700000000, 0)),
		}
	}
	return drivers
}

func GenerateStochasticLoads(count int, baseEpoch int64, rng *pkgmath.RNG) []model.Load {
	cities := []model.Location{
		{NodeID: "NYC", Lat: 40.7128, Lon: -74.0060},
		{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298},
		{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880},
		{NodeID: "DAL", Lat: 32.7767, Lon: -96.7970},
		{NodeID: "PHL", Lat: 39.9526, Lon: -75.1652},
		{NodeID: "DEN", Lat: 39.7392, Lon: -104.9903},
	}

	loads := make([]model.Load, count)
	for i := 0; i < count; i++ {
		origIdx := rng.Intn(len(cities))
		destIdx := (origIdx + 1 + rng.Intn(len(cities)-1)) % len(cities)
		orig := cities[origIdx]
		dest := cities[destIdx]

		miles := orig.DistanceMiles(dest)
		revenue := miles * (2.40 + rng.Float64()*0.40) // $2.40 - $2.80 / mi
		lID := fmt.Sprintf("LOAD_%d_%03d", baseEpoch, i+1)

		loads[i] = model.Load{
			ID:                     lID,
			Origin:                 orig,
			Destination:            dest,
			PickupEarliestEpoch:    baseEpoch,
			PickupLatestEpoch:      baseEpoch + 86400,
			DeliveryEarliestEpoch:  baseEpoch + 7200,
			DeliveryLatestEpoch:    baseEpoch + 259200,
			Revenue:                revenue,
			RequiredEquipment:      model.EquipDryVan,
			EstimatedTransitEpochs: int64(miles / 50.0 * 3600),
		}
	}
	return loads
}
