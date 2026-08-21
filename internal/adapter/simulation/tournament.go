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

// TripartiteDecomposition decomposes total economic lift into Value of Action Space vs Value of Information.
type TripartiteDecomposition struct {
	MeanLegacy            float64 `json:"mean_legacy"`
	MeanBlind             float64 `json:"mean_blind"`
	MeanInformed          float64 `json:"mean_informed"`
	TotalLiftDollars      float64 `json:"total_lift_dollars"`        // V_informed - V_legacy
	TotalLiftPercent      float64 `json:"total_lift_percent"`        // (TotalLift / MeanLegacy) * 100
	ValueOfActionSpace    float64 `json:"value_of_action_space"`     // V_blind - V_legacy
	ValueOfActionSpacePct float64 `json:"value_of_action_space_pct"` // (VoA / TotalLift) * 100
	ValueOfInformation    float64 `json:"value_of_information"`      // V_informed - V_blind
	ValueOfInformationPct float64 `json:"value_of_information_pct"`  // (VoI / TotalLift) * 100
}

// SummaryString formats the economic attribution decomposition.
func (d TripartiteDecomposition) SummaryString() string {
	return fmt.Sprintf(
		"Tripartite Economic Decomposition:\n"+
			"  1. Legacy Monopolistic Mean:       $%.2f\n"+
			"  2. Competitive Blind Mean:         $%.2f\n"+
			"  3. Competitive Informed Mean:      $%.2f\n"+
			"  --------------------------------------------------\n"+
			"  Total Economic Lift:               +$%.2f (+%.2f%%)\n"+
			"    ├── Value of Action Space:       +$%.2f (%.1f%% of lift)\n"+
			"    └── Value of Information (VoI):  +$%.2f (%.1f%% of lift)",
		d.MeanLegacy, d.MeanBlind, d.MeanInformed,
		d.TotalLiftDollars, d.TotalLiftPercent,
		d.ValueOfActionSpace, d.ValueOfActionSpacePct,
		d.ValueOfInformation, d.ValueOfInformationPct,
	)
}

// FactorialDecomposition2x2 encapsulates the full 2x2 factorial experimental evaluation.
type FactorialDecomposition2x2 struct {
	V00_LegacyBlind           float64 `json:"v00_legacy_blind"`             // Legacy Action Space + Blind Belief
	V01_LegacyInformed        float64 `json:"v01_legacy_informed"`          // Legacy Action Space + Informed Belief
	V10_CompetitiveBlind      float64 `json:"v10_competitive_blind"`        // Competitive Action Space + Blind Belief
	V11_CompetitiveInformed   float64 `json:"v11_competitive_informed"`     // Competitive Action Space + Informed Belief
	MainEffectActionSpace     float64 `json:"main_effect_action_space"`     // 0.5 * [(V10 - V00) + (V11 - V01)]
	MainEffectInformation     float64 `json:"main_effect_information"`      // 0.5 * [(V01 - V00) + (V11 - V10)]
	InteractionEffect         float64 `json:"interaction_effect"`           // V11 - V10 - V01 + V00 (Complementarity)
	TotalLift                 float64 `json:"total_lift"`                   // V11 - V00
	ConditionalVoIUnderComp   float64 `json:"conditional_voi_under_comp"`   // V11 - V10
	ConditionalVoIUnderLegacy float64 `json:"conditional_voi_under_legacy"` // V01 - V00
}

// SummaryString formats the 2x2 factorial analysis.
func (f FactorialDecomposition2x2) SummaryString() string {
	return fmt.Sprintf(
		"2x2 Factorial Economic Matrix:\n"+
			"                   | Blind Belief (b0) | Informed Belief (bt) | Marginal VoI\n"+
			"  -----------------+-------------------+----------------------+-------------\n"+
			"  Legacy Action    | V00 = $%9.2f  | V01 = $%9.2f     | $%+9.2f\n"+
			"  Competitive Act. | V10 = $%9.2f  | V11 = $%9.2f     | $%+9.2f\n"+
			"  -----------------+-------------------+----------------------+-------------\n"+
			"  Marginal VoA     | $%+9.2f       | $%+9.2f          | Total: $%+9.2f\n\n"+
			"  Main Effect of Action Space (VoA):  +$%9.2f\n"+
			"  Main Effect of Information (VoI):   +$%9.2f\n"+
			"  Interaction Effect (Complement):   +$%9.2f",
		f.V00_LegacyBlind, f.V01_LegacyInformed, f.ConditionalVoIUnderLegacy,
		f.V10_CompetitiveBlind, f.V11_CompetitiveInformed, f.ConditionalVoIUnderComp,
		f.V10_CompetitiveBlind-f.V00_LegacyBlind, f.V11_CompetitiveInformed-f.V01_LegacyInformed, f.TotalLift,
		f.MainEffectActionSpace, f.MainEffectInformation, f.InteractionEffect,
	)
}

// FactorialReport2x2 encapsulates the full 2x2 factorial experimental run.
type FactorialReport2x2 struct {
	Config               TournamentConfig          `json:"config"`
	Factorial            FactorialDecomposition2x2 `json:"factorial"`
	TTestV11VsV00        pkgmath.PairedTTestResult `json:"t_test_v11_vs_v00"`
	TTestV11VsV10        pkgmath.PairedTTestResult `json:"t_test_v11_vs_v10"`
	TTestV10VsV00        pkgmath.PairedTTestResult `json:"t_test_v10_vs_v00"`
	TTestV01VsV00        pkgmath.PairedTTestResult `json:"t_test_v01_vs_v00"`
	ExecutionDurationSec float64                   `json:"execution_duration_sec"`
}

// TripartiteReport encapsulates the findings of a 3-way head-to-head tournament.
type TripartiteReport struct {
	Config                TournamentConfig          `json:"config"`
	Decomposition         TripartiteDecomposition   `json:"decomposition"`
	TTestInformedVsLegacy pkgmath.PairedTTestResult `json:"t_test_informed_vs_legacy"`
	TTestInformedVsBlind  pkgmath.PairedTTestResult `json:"t_test_informed_vs_blind"`
	TTestBlindVsLegacy    pkgmath.PairedTTestResult `json:"t_test_blind_vs_legacy"`
	ExecutionDurationSec  float64                   `json:"execution_duration_sec"`
}

// FourWayPolicyMetric captures average metrics for a single policy class across tournament episodes.
type FourWayPolicyMetric struct {
	PolicyClass         string  `json:"policy_class"`
	Description         string  `json:"description"`
	MeanNetContribution float64 `json:"mean_net_contribution"`
	MeanGrossRevenue    float64 `json:"mean_gross_revenue"`
	MeanOperatingCost   float64 `json:"mean_operating_cost"`
	MeanWinRate         float64 `json:"mean_win_rate"`
	MeanLatencyMs       float64 `json:"mean_latency_ms"`
	PercentLiftOverPFA  float64 `json:"percent_lift_over_pfa"`
}

// FourWayReport encapsulates head-to-head empirical comparison across the four universal policy classes.
type FourWayReport struct {
	Config               TournamentConfig          `json:"config"`
	Policies             []FourWayPolicyMetric     `json:"policies"`
	TTestCFAvsPFA        pkgmath.PairedTTestResult `json:"t_test_cfa_vs_pfa"`
	TTestVFAvsPFA        pkgmath.PairedTTestResult `json:"t_test_vfa_vs_pfa"`
	TTestDLAvsPFA        pkgmath.PairedTTestResult `json:"t_test_dla_vs_pfa"`
	TTestPOMDPvsCFA      pkgmath.PairedTTestResult `json:"t_test_pomdp_vs_cfa"`
	ExecutionDurationSec float64                   `json:"execution_duration_sec"`
}

// SummaryString formats the 4-policy benchmark scorecard.
func (r FourWayReport) SummaryString() string {
	out := "================================================================================\n" +
		"       POWELL 4-POLICY BENCHMARK: PFA vs CFA vs VFA vs DLA vs POMDP             \n" +
		"================================================================================\n" +
		fmt.Sprintf(" %-16s | %-12s | %-12s | %-10s | %-10s | %-18s\n",
			"Policy Class", "Net Margin", "Lift vs PFA", "Win Rate", "Latency", "Description") +
		"--------------------------------------------------------------------------------\n"
	for _, p := range r.Policies {
		out += fmt.Sprintf(" %-16s | $%11.2f | %+11.2f%% | %9.2f%% | %7.2f ms | %-18s\n",
			p.PolicyClass,
			p.MeanNetContribution,
			p.PercentLiftOverPFA,
			p.MeanWinRate*100.0,
			p.MeanLatencyMs,
			p.Description,
		)
	}
	out += "================================================================================\n"
	return out
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
	infoState, err := model.NewInformationState(startEpoch, 2.50, 3.85, len(initialLoads))
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating info state: %w", err)
	}
	beliefState := model.NewMonopolisticBelief()
	state, err := model.NewState(resState, infoState, beliefState)
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating state: %w", err)
	}

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
	infoState, err := model.NewInformationState(startEpoch, 2.50, 3.85, len(initialLoads))
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating info state: %w", err)
	}

	// N=1 Aggregated Market Belief over 3 competitor postures: [Aggressive, Moderate, Passive]
	marketScale := model.AggregatedMarket{LatentStates: []string{"AGGRESSIVE", "MODERATE", "PASSIVE"}}
	initBelief, err := model.NewBelief[model.AggregatedMarket](
		marketScale,
		[]string{"AGGRESSIVE", "MODERATE", "PASSIVE"},
		[]float64{1.0 / 3.0, 1.0 / 3.0, 1.0 / 3.0},
	)
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating initial belief: %w", err)
	}
	state, err := model.NewState(resState, infoState, initBelief)
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating state: %w", err)
	}

	// Construct Transition Matrix and Observation Model for Belief Filter
	tMatrix, err := model.NewTransitionMatrix(
		[]string{"AGGRESSIVE", "MODERATE", "PASSIVE"},
		r.cfg.Market.TransitionProb,
	)
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating transition matrix: %w", err)
	}
	loadsMean := float64(r.cfg.LoadsPerEpoch)
	obsModel, err := model.NewMarketObservationModel(map[string]model.PostureObservationProfile{
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
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating observation model: %w", err)
	}
	beliefFilter, err := model.NewCompetitiveBeliefFilter[model.AggregatedMarket](
		marketScale,
		tMatrix,
		obsModel,
	)
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating belief filter: %w", err)
	}

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

// runEpisodeN1Blind runs a multi-day simulation using a competitive policy that has access to the
// spot pricing action space, but maintains a fixed uninformative belief prior (zero Bayesian updates).
func (r *TournamentRunner) runEpisodeN1Blind(ctx context.Context, seed uint64) (simResult, error) {
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
	infoState, err := model.NewInformationState(startEpoch, 2.50, 3.85, len(initialLoads))
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating info state: %w", err)
	}

	marketScale := model.AggregatedMarket{LatentStates: []string{"AGGRESSIVE", "MODERATE", "PASSIVE"}}
	initBelief, err := model.NewBelief[model.AggregatedMarket](
		marketScale,
		[]string{"AGGRESSIVE", "MODERATE", "PASSIVE"},
		[]float64{1.0 / 3.0, 1.0 / 3.0, 1.0 / 3.0},
	)
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating initial belief: %w", err)
	}
	state, err := model.NewState(resState, infoState, initBelief)
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating state: %w", err)
	}

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
		return simResult{}, fmt.Errorf("tournament: failed to initialize competitive blind policy: %w", err)
	}

	totRev := 0.0
	totCost := 0.0
	totWon := 0
	totLost := 0

	for step := 0; step < totalEpochs; step++ {
		epoch := startEpoch + int64(step)*stepSec
		nextEpoch := epoch + stepSec

		// 1. Evaluate Competitive POMDP Policy (with static prior belief)
		action, prov, err := compPol.Evaluate(ctx, state)
		if err != nil {
			return simResult{}, err
		}

		// 2. Step Market Environment (Censored Auction)
		outcome, _, err := env.Step(epoch, action, state.Resource().Loads())
		if err != nil {
			return simResult{}, err
		}

		// 3. Blind baseline DOES NOT update belief: remains initBelief

		// 4. Accounting & Filtering Won Matches
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

		// 5. Ingest new load arrivals and execute physical resource transition
		incomingLoads := GenerateStochasticLoads(r.cfg.LoadsPerEpoch, nextEpoch, rng)
		nextRes, err := state.Resource().Transition(wonMatches, incomingLoads)
		if err != nil {
			return simResult{}, err
		}
		nextInfo, err := state.Information().Transition(nextEpoch, 2.50, 3.85, len(incomingLoads))
		if err != nil {
			return simResult{}, err
		}
		// Belief remains static uninformative prior
		state, err = model.NewState(nextRes, nextInfo, initBelief)
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

// RunTripartite executes a 3-way tournament evaluating Legacy (N=0), Blind (N=1 static prior),
// and Informed (N=1 belief-filtered) policies to isolate the Value of Information from the Value of Action Space.
func (r *TournamentRunner) RunTripartite(ctx context.Context) (*TripartiteReport, error) {
	startTime := time.Now()
	cfg := r.cfg
	if cfg.Episodes < 2 {
		cfg.Episodes = 2
	}

	nLegacy := make([]float64, cfg.Episodes)
	nBlind := make([]float64, cfg.Episodes)
	nInformed := make([]float64, cfg.Episodes)

	for ep := 0; ep < cfg.Episodes; ep++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		epSeed := cfg.BaseSeed + uint64(ep)*7919

		// 1. Run Legacy Monopolistic (N=0)
		scoreLegacy, err := r.runEpisodeN0(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("tripartite: episode %d legacy failed: %w", ep, err)
		}

		// 2. Run Competitive Blind (N=1 static prior)
		scoreBlind, err := r.runEpisodeN1Blind(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("tripartite: episode %d blind failed: %w", ep, err)
		}

		// 3. Run Competitive Informed (N=1 belief-filtered)
		scoreInformed, err := r.runEpisodeN1(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("tripartite: episode %d informed failed: %w", ep, err)
		}

		nLegacy[ep] = scoreLegacy.NetContribution
		nBlind[ep] = scoreBlind.NetContribution
		nInformed[ep] = scoreInformed.NetContribution
	}

	tInformedVsLegacy, err := pkgmath.ComputePairedTTest(nLegacy, nInformed)
	if err != nil {
		return nil, fmt.Errorf("tripartite: t-test informed vs legacy failed: %w", err)
	}
	tInformedVsBlind, err := pkgmath.ComputePairedTTest(nBlind, nInformed)
	if err != nil {
		return nil, fmt.Errorf("tripartite: t-test informed vs blind failed: %w", err)
	}
	tBlindVsLegacy, err := pkgmath.ComputePairedTTest(nLegacy, nBlind)
	if err != nil {
		return nil, fmt.Errorf("tripartite: t-test blind vs legacy failed: %w", err)
	}

	totalLift := tInformedVsLegacy.MeanDifference
	voa := tBlindVsLegacy.MeanDifference
	voi := tInformedVsBlind.MeanDifference

	voaPct := 0.0
	voiPct := 0.0
	if math.Abs(totalLift) > 1e-6 {
		voaPct = (voa / totalLift) * 100.0
		voiPct = (voi / totalLift) * 100.0
	}

	decomp := TripartiteDecomposition{
		MeanLegacy:            tInformedVsLegacy.MeanBaseline,
		MeanBlind:             tInformedVsBlind.MeanBaseline,
		MeanInformed:          tInformedVsLegacy.MeanCandidate,
		TotalLiftDollars:      totalLift,
		TotalLiftPercent:      tInformedVsLegacy.PercentLift,
		ValueOfActionSpace:    voa,
		ValueOfActionSpacePct: voaPct,
		ValueOfInformation:    voi,
		ValueOfInformationPct: voiPct,
	}

	return &TripartiteReport{
		Config:                cfg,
		Decomposition:         decomp,
		TTestInformedVsLegacy: tInformedVsLegacy,
		TTestInformedVsBlind:  tInformedVsBlind,
		TTestBlindVsLegacy:    tBlindVsLegacy,
		ExecutionDurationSec:  time.Since(startTime).Seconds(),
	}, nil
}

// runEpisodeN0Informed runs a multi-day simulation using the Legacy Action Space (no spot pricing),
// but updating the Bayesian belief state to evaluate driver risk and dispatch defensively (V01).
func (r *TournamentRunner) runEpisodeN0Informed(ctx context.Context, seed uint64) (simResult, error) {
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
	infoState, err := model.NewInformationState(startEpoch, 2.50, 3.85, len(initialLoads))
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating info state: %w", err)
	}

	marketScale := model.AggregatedMarket{LatentStates: []string{"AGGRESSIVE", "MODERATE", "PASSIVE"}}
	initBelief, err := model.NewBelief[model.AggregatedMarket](
		marketScale,
		[]string{"AGGRESSIVE", "MODERATE", "PASSIVE"},
		[]float64{1.0 / 3.0, 1.0 / 3.0, 1.0 / 3.0},
	)
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating initial belief: %w", err)
	}
	state, err := model.NewState(resState, infoState, initBelief)
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating state: %w", err)
	}

	tMatrix, err := model.NewTransitionMatrix(
		[]string{"AGGRESSIVE", "MODERATE", "PASSIVE"},
		r.cfg.Market.TransitionProb,
	)
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating transition matrix: %w", err)
	}
	loadsMean := float64(r.cfg.LoadsPerEpoch)
	obsModel, err := model.NewMarketObservationModel(map[string]model.PostureObservationProfile{
		"AGGRESSIVE": {ExpectedWinProbability: 0.35, ExpectedSpotRateMean: 2.15, ExpectedSpotRateStdDev: 0.10, ExpectedOffersMean: loadsMean},
		"MODERATE":   {ExpectedWinProbability: 0.65, ExpectedSpotRateMean: 2.50, ExpectedSpotRateStdDev: 0.10, ExpectedOffersMean: loadsMean},
		"PASSIVE":    {ExpectedWinProbability: 0.85, ExpectedSpotRateMean: 2.95, ExpectedSpotRateStdDev: 0.10, ExpectedOffersMean: loadsMean},
	})
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating observation model: %w", err)
	}
	beliefFilter, err := model.NewCompetitiveBeliefFilter[model.AggregatedMarket](marketScale, tMatrix, obsModel)
	if err != nil {
		return simResult{}, fmt.Errorf("tournament: failed creating belief filter: %w", err)
	}

	// CFA Policy with posture-informed risk adjustment but NO spot pricing wrapper (P_legacy, informed)
	cfaParams := policy.CFAParameters{
		ThetaEmpty: 1.0,
		ThetaHome:  1.0,
		ThetaDwell: 1.0,
		ThetaRisk:  0.25, // Informed risk adjustment on dispatch
	}
	costCfg := model.DefaultCostConfig()
	costCfg.EmptyToHomeRate = 0.20
	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 800.0
	cfaPol := policy.NewCFAPolicy[model.AggregatedMarket](cfaParams, costCfg, feasCfg, nil)

	totRev := 0.0
	totCost := 0.0
	totWon := 0
	totLost := 0

	for step := 0; step < totalEpochs; step++ {
		epoch := startEpoch + int64(step)*stepSec
		nextEpoch := epoch + stepSec

		// 1. Evaluate CFA Policy directly (Legacy Action Space: Fixed Tariff / Monopolistic Matching)
		action, prov, err := cfaPol.Evaluate(ctx, state)
		if err != nil {
			return simResult{}, err
		}

		// 2. Step Market Environment
		outcome, obs, err := env.Step(epoch, action, state.Resource().Loads())
		if err != nil {
			return simResult{}, err
		}

		// 3. Update Bayesian Belief Filter
		nextBelief, err := beliefFilter.Filter(state.Belief(), obs, action)
		if err != nil {
			return simResult{}, fmt.Errorf("factorial: belief filter update failed: %w", err)
		}

		// 4. Accounting & Filtering Won Matches
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

		// 5. Ingest new load arrivals and execute physical resource transition
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

// RunFactorial2x2 executes the complete 2x2 factorial experiment evaluating V00, V01, V10, V11
// across common random number streams to compute main effects and interaction (complementarity).
func (r *TournamentRunner) RunFactorial2x2(ctx context.Context) (*FactorialReport2x2, error) {
	startTime := time.Now()
	cfg := r.cfg
	if cfg.Episodes < 2 {
		cfg.Episodes = 2
	}

	v00 := make([]float64, cfg.Episodes)
	v01 := make([]float64, cfg.Episodes)
	v10 := make([]float64, cfg.Episodes)
	v11 := make([]float64, cfg.Episodes)

	for ep := 0; ep < cfg.Episodes; ep++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		epSeed := cfg.BaseSeed + uint64(ep)*7919

		// 1. Run V00 (Legacy Action + Blind)
		s00, err := r.runEpisodeN0(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("factorial: episode %d V00 failed: %w", ep, err)
		}
		// 2. Run V01 (Legacy Action + Informed)
		s01, err := r.runEpisodeN0Informed(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("factorial: episode %d V01 failed: %w", ep, err)
		}
		// 3. Run V10 (Competitive Action + Blind)
		s10, err := r.runEpisodeN1Blind(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("factorial: episode %d V10 failed: %w", ep, err)
		}
		// 4. Run V11 (Competitive Action + Informed)
		s11, err := r.runEpisodeN1(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("factorial: episode %d V11 failed: %w", ep, err)
		}

		v00[ep] = s00.NetContribution
		v01[ep] = s01.NetContribution
		v10[ep] = s10.NetContribution
		v11[ep] = s11.NetContribution
	}

	tV11VsV00, err := pkgmath.ComputePairedTTest(v00, v11)
	if err != nil {
		return nil, fmt.Errorf("factorial: t-test v11 vs v00 failed: %w", err)
	}
	tV11VsV10, err := pkgmath.ComputePairedTTest(v10, v11)
	if err != nil {
		return nil, fmt.Errorf("factorial: t-test v11 vs v10 failed: %w", err)
	}
	tV10VsV00, err := pkgmath.ComputePairedTTest(v00, v10)
	if err != nil {
		return nil, fmt.Errorf("factorial: t-test v10 vs v00 failed: %w", err)
	}
	tV01VsV00, err := pkgmath.ComputePairedTTest(v00, v01)
	if err != nil {
		return nil, fmt.Errorf("factorial: t-test v01 vs v00 failed: %w", err)
	}

	mean00 := tV11VsV00.MeanBaseline
	mean01 := tV01VsV00.MeanCandidate
	mean10 := tV10VsV00.MeanCandidate
	mean11 := tV11VsV00.MeanCandidate

	mainAction := 0.5 * ((mean10 - mean00) + (mean11 - mean01))
	mainInfo := 0.5 * ((mean01 - mean00) + (mean11 - mean10))
	interaction := mean11 - mean10 - mean01 + mean00

	factorial := FactorialDecomposition2x2{
		V00_LegacyBlind:           mean00,
		V01_LegacyInformed:        mean01,
		V10_CompetitiveBlind:      mean10,
		V11_CompetitiveInformed:   mean11,
		MainEffectActionSpace:     mainAction,
		MainEffectInformation:     mainInfo,
		InteractionEffect:         interaction,
		TotalLift:                 mean11 - mean00,
		ConditionalVoIUnderComp:   mean11 - mean10,
		ConditionalVoIUnderLegacy: mean01 - mean00,
	}

	return &FactorialReport2x2{
		Config:               cfg,
		Factorial:            factorial,
		TTestV11VsV00:        tV11VsV00,
		TTestV11VsV10:        tV11VsV10,
		TTestV10VsV00:        tV10VsV00,
		TTestV01VsV00:        tV01VsV00,
		ExecutionDurationSec: time.Since(startTime).Seconds(),
	}, nil
}

// Run4Way executes a comprehensive 5-way benchmark evaluating PFA, CFA, PiecewiseVFA, DLA, and Competitive POMDP
// across identical stochastic problem instances to produce the canonical Powell policy comparison.
func (r *TournamentRunner) Run4Way(ctx context.Context) (*FourWayReport, error) {
	startTime := time.Now()
	cfg := r.cfg
	if cfg.Episodes < 2 {
		cfg.Episodes = 2
	}

	pfaScores := make([]simResult, cfg.Episodes)
	cfaScores := make([]simResult, cfg.Episodes)
	vfaScores := make([]simResult, cfg.Episodes)
	dlaScores := make([]simResult, cfg.Episodes)
	pomdpScores := make([]simResult, cfg.Episodes)

	pfaNets := make([]float64, cfg.Episodes)
	cfaNets := make([]float64, cfg.Episodes)
	vfaNets := make([]float64, cfg.Episodes)
	dlaNets := make([]float64, cfg.Episodes)
	pomdpNets := make([]float64, cfg.Episodes)

	pfaLatencies := make([]float64, cfg.Episodes)
	cfaLatencies := make([]float64, cfg.Episodes)
	vfaLatencies := make([]float64, cfg.Episodes)
	dlaLatencies := make([]float64, cfg.Episodes)
	pomdpLatencies := make([]float64, cfg.Episodes)

	for ep := 0; ep < cfg.Episodes; ep++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		epSeed := cfg.BaseSeed + uint64(ep)*7919

		// 1. PFA (Greedy Direct Contribution)
		t0 := time.Now()
		sPFA, err := r.runEpisodePFA(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("4way: episode %d PFA failed: %w", ep, err)
		}
		pfaLatencies[ep] = float64(time.Since(t0).Microseconds()) / 1000.0 / float64((r.cfg.HorizonDays*24)/r.cfg.DecisionStepHours)
		pfaScores[ep] = sPFA
		pfaNets[ep] = sPFA.NetContribution

		// 2. CFA (Parametric Cost Function Approximation)
		t0 = time.Now()
		sCFA, err := r.runEpisodeCFA(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("4way: episode %d CFA failed: %w", ep, err)
		}
		cfaLatencies[ep] = float64(time.Since(t0).Microseconds()) / 1000.0 / float64((r.cfg.HorizonDays*24)/r.cfg.DecisionStepHours)
		cfaScores[ep] = sCFA
		cfaNets[ep] = sCFA.NetContribution

		// 3. Piecewise VFA (Downstream Marginal Slopes with CAVE)
		t0 = time.Now()
		sVFA, err := r.runEpisodeVFA(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("4way: episode %d VFA failed: %w", ep, err)
		}
		vfaLatencies[ep] = float64(time.Since(t0).Microseconds()) / 1000.0 / float64((r.cfg.HorizonDays*24)/r.cfg.DecisionStepHours)
		vfaScores[ep] = sVFA
		vfaNets[ep] = sVFA.NetContribution

		// 4. DLA (Direct Lookahead Approximation)
		t0 = time.Now()
		sDLA, err := r.runEpisodeDLA(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("4way: episode %d DLA failed: %w", ep, err)
		}
		dlaLatencies[ep] = float64(time.Since(t0).Microseconds()) / 1000.0 / float64((r.cfg.HorizonDays*24)/r.cfg.DecisionStepHours)
		dlaScores[ep] = sDLA
		dlaNets[ep] = sDLA.NetContribution

		// 5. Competitive POMDP (Bayesian Belief Simplex + Dynamic Pricing)
		t0 = time.Now()
		sPOMDP, err := r.runEpisodeN1(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("4way: episode %d POMDP failed: %w", ep, err)
		}
		pomdpLatencies[ep] = float64(time.Since(t0).Microseconds()) / 1000.0 / float64((r.cfg.HorizonDays*24)/r.cfg.DecisionStepHours)
		pomdpScores[ep] = sPOMDP
		pomdpNets[ep] = sPOMDP.NetContribution
	}

	tCFAvsPFA, _ := pkgmath.ComputePairedTTest(pfaNets, cfaNets)
	tVFAvsPFA, _ := pkgmath.ComputePairedTTest(pfaNets, vfaNets)
	tDLAvsPFA, _ := pkgmath.ComputePairedTTest(pfaNets, dlaNets)
	tPOMDPvsCFA, _ := pkgmath.ComputePairedTTest(cfaNets, pomdpNets)

	meanPFA := meanResult(pfaScores)
	meanCFA := meanResult(cfaScores)
	meanVFA := meanResult(vfaScores)
	meanDLA := meanResult(dlaScores)
	meanPOMDP := meanResult(pomdpScores)

	pfaMeanLat := meanSlice(pfaLatencies)
	cfaMeanLat := meanSlice(cfaLatencies)
	vfaMeanLat := meanSlice(vfaLatencies)
	dlaMeanLat := meanSlice(dlaLatencies)
	pomdpMeanLat := meanSlice(pomdpLatencies)

	policies := []FourWayPolicyMetric{
		{
			PolicyClass:         "1. PFA",
			Description:         "Myopic Direct Contribution",
			MeanNetContribution: meanPFA.NetContribution,
			MeanGrossRevenue:    meanPFA.GrossRevenue,
			MeanOperatingCost:   meanPFA.OperatingCost,
			MeanWinRate:         meanPFA.WinRate,
			MeanLatencyMs:       pfaMeanLat,
			PercentLiftOverPFA:  0.0,
		},
		{
			PolicyClass:         "2. CFA",
			Description:         "Parametric Cost Tuning (SPSA)",
			MeanNetContribution: meanCFA.NetContribution,
			MeanGrossRevenue:    meanCFA.GrossRevenue,
			MeanOperatingCost:   meanCFA.OperatingCost,
			MeanWinRate:         meanCFA.WinRate,
			MeanLatencyMs:       cfaMeanLat,
			PercentLiftOverPFA:  ((meanCFA.NetContribution - meanPFA.NetContribution) / math.Abs(meanPFA.NetContribution)) * 100.0,
		},
		{
			PolicyClass:         "3. VFA",
			Description:         "Piecewise Linear Concave Slopes",
			MeanNetContribution: meanVFA.NetContribution,
			MeanGrossRevenue:    meanVFA.GrossRevenue,
			MeanOperatingCost:   meanVFA.OperatingCost,
			MeanWinRate:         meanVFA.WinRate,
			MeanLatencyMs:       vfaMeanLat,
			PercentLiftOverPFA:  ((meanVFA.NetContribution - meanPFA.NetContribution) / math.Abs(meanPFA.NetContribution)) * 100.0,
		},
		{
			PolicyClass:         "4. DLA",
			Description:         "Direct Lookahead (2-Epoch Horizon)",
			MeanNetContribution: meanDLA.NetContribution,
			MeanGrossRevenue:    meanDLA.GrossRevenue,
			MeanOperatingCost:   meanDLA.OperatingCost,
			MeanWinRate:         meanDLA.WinRate,
			MeanLatencyMs:       dlaMeanLat,
			PercentLiftOverPFA:  ((meanDLA.NetContribution - meanPFA.NetContribution) / math.Abs(meanPFA.NetContribution)) * 100.0,
		},
		{
			PolicyClass:         "5. Competitive",
			Description:         "MOMDP Bayesian Belief Simplex",
			MeanNetContribution: meanPOMDP.NetContribution,
			MeanGrossRevenue:    meanPOMDP.GrossRevenue,
			MeanOperatingCost:   meanPOMDP.OperatingCost,
			MeanWinRate:         meanPOMDP.WinRate,
			MeanLatencyMs:       pomdpMeanLat,
			PercentLiftOverPFA:  ((meanPOMDP.NetContribution - meanPFA.NetContribution) / math.Abs(meanPFA.NetContribution)) * 100.0,
		},
	}

	return &FourWayReport{
		Config:               cfg,
		Policies:             policies,
		TTestCFAvsPFA:        tCFAvsPFA,
		TTestVFAvsPFA:        tVFAvsPFA,
		TTestDLAvsPFA:        tDLAvsPFA,
		TTestPOMDPvsCFA:      tPOMDPvsCFA,
		ExecutionDurationSec: time.Since(startTime).Seconds(),
	}, nil
}

func meanSlice(vals []float64) float64 {
	if len(vals) == 0 {
		return 0.0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func meanResult(scores []simResult) simResult {
	if len(scores) == 0 {
		return simResult{}
	}
	var rev, cost, net, win float64
	for _, s := range scores {
		rev += s.GrossRevenue
		cost += s.OperatingCost
		net += s.NetContribution
		win += s.WinRate
	}
	n := float64(len(scores))
	return simResult{
		GrossRevenue:    rev / n,
		OperatingCost:   cost / n,
		NetContribution: net / n,
		WinRate:         win / n,
	}
}

// runEpisodePFA runs an episode using a pure greedy PFA heuristic (theta = 0).
func (r *TournamentRunner) runEpisodePFA(ctx context.Context, seed uint64) (simResult, error) {
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
	infoState, err := model.NewInformationState(startEpoch, 2.50, 3.85, len(initialLoads))
	if err != nil {
		return simResult{}, fmt.Errorf("pfa: failed creating info state: %w", err)
	}
	beliefState := model.NewMonopolisticBelief()
	state, err := model.NewState(resState, infoState, beliefState)
	if err != nil {
		return simResult{}, fmt.Errorf("pfa: failed creating state: %w", err)
	}

	cfaParams := policy.CFAParameters{
		ThetaEmpty: 0.0,
		ThetaHome:  0.0,
		ThetaDwell: 0.0,
		ThetaRisk:  0.0,
	}
	costCfg := model.DefaultCostConfig()
	costCfg.EmptyToHomeRate = 0.0
	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 800.0
	pfaPol := policy.NewCFAPolicy[model.Monopolistic](cfaParams, costCfg, feasCfg, nil)

	return r.executeSimulationLoop(ctx, env, state, pfaPol, rng, startEpoch, stepSec, totalEpochs)
}

// runEpisodeCFA runs an episode using tuned CFA parameters.
func (r *TournamentRunner) runEpisodeCFA(ctx context.Context, seed uint64) (simResult, error) {
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
	infoState, err := model.NewInformationState(startEpoch, 2.50, 3.85, len(initialLoads))
	if err != nil {
		return simResult{}, fmt.Errorf("cfa: failed creating info state: %w", err)
	}
	beliefState := model.NewMonopolisticBelief()
	state, err := model.NewState(resState, infoState, beliefState)
	if err != nil {
		return simResult{}, fmt.Errorf("cfa: failed creating state: %w", err)
	}

	cfaParams := policy.CFAParameters{
		ThetaEmpty: 1.15,
		ThetaHome:  0.20,
		ThetaDwell: 0.85,
		ThetaRisk:  0.0,
	}
	costCfg := model.DefaultCostConfig()
	costCfg.EmptyToHomeRate = 0.20
	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 800.0
	cfaPol := policy.NewCFAPolicy[model.Monopolistic](cfaParams, costCfg, feasCfg, nil)

	return r.executeSimulationLoop(ctx, env, state, cfaPol, rng, startEpoch, stepSec, totalEpochs)
}

// runEpisodeVFA runs an episode using Piecewise Linear Concave VFA.
func (r *TournamentRunner) runEpisodeVFA(ctx context.Context, seed uint64) (simResult, error) {
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
	infoState, err := model.NewInformationState(startEpoch, 2.50, 3.85, len(initialLoads))
	if err != nil {
		return simResult{}, fmt.Errorf("vfa: failed creating info state: %w", err)
	}
	beliefState := model.NewMonopolisticBelief()
	state, err := model.NewState(resState, infoState, beliefState)
	if err != nil {
		return simResult{}, fmt.Errorf("vfa: failed creating state: %w", err)
	}

	costCfg := model.DefaultCostConfig()
	costCfg.EmptyToHomeRate = 0.20
	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 800.0

	slopesA, _ := policy.NewRegionSlopes("REG_NYC", []float64{150, 100, 50, 20, 0})
	slopesB, _ := policy.NewRegionSlopes("REG_CHI", []float64{180, 120, 70, 30, 0})
	slopesC, _ := policy.NewRegionSlopes("REG_ATL", []float64{140, 90, 40, 10, 0})
	slopesD, _ := policy.NewRegionSlopes("REG_DAL", []float64{160, 110, 60, 20, 0})
	slopesE, _ := policy.NewRegionSlopes("REG_PHL", []float64{130, 80, 30, 10, 0})
	slopesF, _ := policy.NewRegionSlopes("REG_DEN", []float64{170, 115, 65, 25, 0})
	vfaTable := policy.NewPiecewiseLinearVFATable(map[string]policy.RegionSlopes{
		"REG_NYC": slopesA,
		"REG_CHI": slopesB,
		"REG_ATL": slopesC,
		"REG_DAL": slopesD,
		"REG_PHL": slopesE,
		"REG_DEN": slopesF,
	})
	rm := model.NewRegionManager(1.0, nil)
	vfaPol, err := policy.NewPiecewiseVFAPolicy[model.Monopolistic](vfaTable, nil, 0.95, costCfg, feasCfg, rm)
	if err != nil {
		return simResult{}, fmt.Errorf("vfa: failed init policy: %w", err)
	}

	return r.executeSimulationLoop(ctx, env, state, vfaPol, rng, startEpoch, stepSec, totalEpochs)
}

// runEpisodeDLA runs an episode using Direct Lookahead policy.
func (r *TournamentRunner) runEpisodeDLA(ctx context.Context, seed uint64) (simResult, error) {
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
	infoState, err := model.NewInformationState(startEpoch, 2.50, 3.85, len(initialLoads))
	if err != nil {
		return simResult{}, fmt.Errorf("dla: failed creating info state: %w", err)
	}
	beliefState := model.NewMonopolisticBelief()
	state, err := model.NewState(resState, infoState, beliefState)
	if err != nil {
		return simResult{}, fmt.Errorf("dla: failed creating state: %w", err)
	}

	costCfg := model.DefaultCostConfig()
	costCfg.EmptyToHomeRate = 0.20
	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 800.0

	cfaBase := policy.NewCFAPolicy[model.Monopolistic](policy.DefaultCFAParameters(), costCfg, feasCfg, nil)
	dlaParams := policy.DefaultDLAParameters()
	dlaParams.Horizon = 2
	dlaParams.NumRollouts = 1
	dlaParams.DiscountFactor = 0.95
	dlaParams.StepSeconds = stepSec
	dlaParams.RandomSeed = seed + 101
	dlaParams.EnableAdaptivePruning = false
	rm := model.NewRegionManager(1.0, nil)
	dlaPol, err := policy.NewDLAPolicy[model.Monopolistic](dlaParams, costCfg, feasCfg, cfaBase, nil, rm, nil, nil)
	if err != nil {
		return simResult{}, fmt.Errorf("dla: failed init policy: %w", err)
	}

	return r.executeSimulationLoop(ctx, env, state, dlaPol, rng, startEpoch, stepSec, totalEpochs)
}

func (r *TournamentRunner) executeSimulationLoop(
	ctx context.Context,
	env *MarketEnvironment,
	initialState *model.State[model.Monopolistic],
	pol policy.Policy[model.Monopolistic],
	rng *pkgmath.RNG,
	startEpoch int64,
	stepSec int64,
	totalEpochs int,
) (simResult, error) {
	state := initialState
	totRev := 0.0
	totCost := 0.0
	totWon := 0
	totLost := 0

	for step := 0; step < totalEpochs; step++ {
		epoch := startEpoch + int64(step)*stepSec
		nextEpoch := epoch + stepSec

		action, prov, err := pol.Evaluate(ctx, state)
		if err != nil {
			return simResult{}, err
		}

		outcome, _, err := env.Step(epoch, action, state.Resource().Loads())
		if err != nil {
			return simResult{}, err
		}

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
