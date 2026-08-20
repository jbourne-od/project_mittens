package simulation_test

import (
	"context"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/adapter/simulation"
	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

func TestTournament_SingleEpochDiagnostic(t *testing.T) {
	cfg := simulation.DefaultTournamentConfig()
	cfg.DriverCount = 10
	cfg.LoadsPerEpoch = 15
	seed := cfg.BaseSeed + 3 // Episode 3 seed

	env, _ := simulation.NewMarketEnvironment(cfg.Market, seed)
	rng := pkgmath.NewRNG(seed + 1)
	startEpoch := int64(1700000000)
	stepSec := int64(12 * 3600)

	initialDrivers := simulation.GenerateTestDrivers(10, rng)
	initialLoads := simulation.GenerateStochasticLoads(15, startEpoch, rng)
	resState := model.NewResourceState(initialDrivers, initialLoads)
	infoState, _ := model.NewInformationState(startEpoch, 2.50, 3.85, 15)

	marketScale := model.AggregatedMarket{LatentStates: []string{"AGGRESSIVE", "MODERATE", "PASSIVE"}}
	initBelief, _ := model.NewBelief[model.AggregatedMarket](
		marketScale,
		[]string{"AGGRESSIVE", "MODERATE", "PASSIVE"},
		[]float64{1.0 / 3.0, 1.0 / 3.0, 1.0 / 3.0},
	)
	state, _ := model.NewState(resState, infoState, initBelief)

	tMatrix, _ := model.NewTransitionMatrix(
		[]string{"AGGRESSIVE", "MODERATE", "PASSIVE"},
		cfg.Market.TransitionProb,
	)
	loadsMean := float64(15)
	obsModel, _ := model.NewMarketObservationModel(map[string]model.PostureObservationProfile{
		"AGGRESSIVE": {ExpectedWinProbability: 0.35, ExpectedSpotRateMean: 2.15, ExpectedSpotRateStdDev: 0.20, ExpectedOffersMean: loadsMean},
		"MODERATE":   {ExpectedWinProbability: 0.65, ExpectedSpotRateMean: 2.50, ExpectedSpotRateStdDev: 0.20, ExpectedOffersMean: loadsMean},
		"PASSIVE":    {ExpectedWinProbability: 0.85, ExpectedSpotRateMean: 2.95, ExpectedSpotRateStdDev: 0.20, ExpectedOffersMean: loadsMean},
	})
	beliefFilter, _ := model.NewCompetitiveBeliefFilter[model.AggregatedMarket](marketScale, tMatrix, obsModel)

	costCfg := model.DefaultCostConfig()
	costCfg.EmptyToHomeRate = 0.20
	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 800.0
	cfaPol := policy.NewCFAPolicy[model.AggregatedMarket](policy.DefaultCFAParameters(), costCfg, feasCfg, nil)

	for step := 0; step < 5; step++ {
		epoch := startEpoch + int64(step)*stepSec
		action, _, err := cfaPol.Evaluate(context.Background(), state)
		if err != nil {
			t.Fatalf("cfaPol.Evaluate failed: %v", err)
		}
		outcome, obs, err := env.Step(epoch, action, state.Resource().Loads())
		if err != nil {
			t.Fatalf("env.Step failed: %v", err)
		}
		nextBelief, err := beliefFilter.Filter(state.Belief(), obs, action)
		if err != nil {
			t.Fatalf("beliefFilter.Filter failed: %v", err)
		}

		t.Logf("Step %d: TrueState=%s | Posture: Agg=%.3f, Mod=%.3f, Pas=%.3f | Won=%d, Lost=%d",
			step, outcome.TrueCompetitorState,
			nextBelief.Probability("AGGRESSIVE"), nextBelief.Probability("MODERATE"), nextBelief.Probability("PASSIVE"),
			len(outcome.WonLoads), len(outcome.LostLoads))

		incomingLoads := simulation.GenerateStochasticLoads(15, epoch+stepSec, rng)
		nextRes, err := state.Resource().Transition(action.Matches(), incomingLoads)
		if err != nil {
			t.Fatalf("Resource.Transition failed: %v", err)
		}
		nextInfo, err := state.Information().Transition(epoch+stepSec, 2.50, 3.85, 15)
		if err != nil {
			t.Fatalf("Information.Transition failed: %v", err)
		}
		state, err = model.NewState(nextRes, nextInfo, nextBelief)
		if err != nil {
			t.Fatalf("NewState failed: %v", err)
		}
	}
}

func TestTournament_DegenerateN0_Parity(t *testing.T) {
	cfg := simulation.DefaultTournamentConfig()
	cfg.Episodes = 3
	cfg.HorizonDays = 2
	cfg.DriverCount = 10
	cfg.LoadsPerEpoch = 15

	runner := simulation.NewTournamentRunner(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	report, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Run tournament failed: %v", err)
	}

	if len(report.Episodes) != 3 {
		t.Fatalf("expected 3 episodes, got %d", len(report.Episodes))
	}

	if !report.DegenerateParityVerified {
		t.Errorf("expected DegenerateParityVerified to be true (identical twin N=0 rollouts)")
	}

	for _, ep := range report.Episodes {
		if ep.N0_GrossRevenue <= 0 {
			t.Errorf("expected positive revenue for episode %d, got $%.2f", ep.EpisodeIndex, ep.N0_GrossRevenue)
		}
		if ep.N0_TotalWonLoads <= 0 {
			t.Errorf("expected positive won loads for episode %d, got %d", ep.EpisodeIndex, ep.N0_TotalWonLoads)
		}
	}
}

func TestTournament_N1_Superiority_StatisticalSignificance(t *testing.T) {
	cfg := simulation.DefaultTournamentConfig()
	cfg.Episodes = 25
	cfg.HorizonDays = 5
	cfg.DecisionStepHours = 12
	cfg.DriverCount = 15
	cfg.LoadsPerEpoch = 25
	cfg.BaseSeed = 20260819

	runner := simulation.NewTournamentRunner(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	report, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Run tournament failed: %v", err)
	}

	for i, ep := range report.Episodes {
		t.Logf("Ep %2d: N0 net=$%9.2f (rev=$%9.2f won=%d lost=%d) | N1 net=$%9.2f (rev=$%9.2f won=%d lost=%d) | diff=$%+9.2f",
			i+1, ep.N0_NetContribution, ep.N0_GrossRevenue, ep.N0_TotalWonLoads, ep.N0_TotalLostLoads,
			ep.N1_NetContribution, ep.N1_GrossRevenue, ep.N1_TotalWonLoads, ep.N1_TotalLostLoads, ep.NetContributionDelta)
	}

	t.Logf("\n========================================================================\n"+
		"           TOURNAMENT SCORECARD: N=0 (BASELINE) vs N=1 (MOMDP)          \n"+
		"========================================================================\n"+
		"%s\n"+
		"========================================================================\n"+
		"  Mean N=0 Net Contribution: $%.2f\n"+
		"  Mean N=1 Net Contribution: $%.2f\n"+
		"  Mean Net Contribution Lift: +$%.2f (+%.2f%%)\n"+
		"  95%% Confidence Interval:   [$%.2f, $%.2f]\n"+
		"  Student's t-Statistic:     %.4f (df=%.0f)\n"+
		"  One-Tailed p-Value:        %e\n"+
		"  N=1 Superiority Verified:  %t\n"+
		"========================================================================",
		report.TTest.SummaryString(),
		report.TTest.MeanBaseline,
		report.TTest.MeanCandidate,
		report.TTest.MeanDifference,
		report.TTest.PercentLift,
		report.TTest.ConfidenceLow95,
		report.TTest.ConfidenceHigh95,
		report.TTest.TStatistic,
		report.TTest.DegreesOfFreedom,
		report.TTest.PValueOneTailed,
		report.N1SuperiorityVerified,
	)

	// Statistical Assertions
	if !report.N1SuperiorityVerified {
		t.Errorf("expected N=1 superiority to be statistically verified (p < 0.01, t > 2.5, lift > 3%%)")
	}
	if report.TTest.PValueOneTailed >= 0.01 {
		t.Errorf("expected p-value < 0.01 (reject null hypothesis H0), got %e", report.TTest.PValueOneTailed)
	}
	if report.TTest.MeanDifference <= 0 {
		t.Errorf("expected positive profit lift for N=1, got $%.2f", report.TTest.MeanDifference)
	}
	if report.TTest.ConfidenceLow95 <= 0 {
		t.Errorf("expected lower 95%% CI > 0, got $%.2f", report.TTest.ConfidenceLow95)
	}
}

// TestTournament_TripartiteDecomposition evaluates the 3-way economic attribution decomposition:
// V_informed - V_legacy = (V_informed - V_blind) [VoI] + (V_blind - V_legacy) [VoA]
func TestTournament_TripartiteDecomposition(t *testing.T) {
	cfg := simulation.DefaultTournamentConfig()
	cfg.Episodes = 25
	cfg.HorizonDays = 5
	cfg.DecisionStepHours = 12
	cfg.DriverCount = 15
	cfg.LoadsPerEpoch = 25
	cfg.BaseSeed = 202608205

	runner := simulation.NewTournamentRunner(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	report, err := runner.RunTripartite(ctx)
	if err != nil {
		t.Fatalf("RunTripartite failed: %v", err)
	}

	t.Logf("\n========================================================================\n"+
		"           TRIPARTITE ECONOMIC DECOMPOSITION (N=25 EPISODES)            \n"+
		"========================================================================\n"+
		"%s\n"+
		"========================================================================\n"+
		"  Informed vs Legacy: t=%.4f, p=%e, lift=+%.2f%%\n"+
		"  Informed vs Blind (VoI): t=%.4f, p=%e, lift=+%.2f%%\n"+
		"  Blind vs Legacy (VoA):   t=%.4f, p=%e, lift=+%.2f%%\n"+
		"========================================================================",
		report.Decomposition.SummaryString(),
		report.TTestInformedVsLegacy.TStatistic, report.TTestInformedVsLegacy.PValueOneTailed, report.TTestInformedVsLegacy.PercentLift,
		report.TTestInformedVsBlind.TStatistic, report.TTestInformedVsBlind.PValueOneTailed, report.TTestInformedVsBlind.PercentLift,
		report.TTestBlindVsLegacy.TStatistic, report.TTestBlindVsLegacy.PValueOneTailed, report.TTestBlindVsLegacy.PercentLift,
	)

	// Assert that Value of Information is strictly positive
	if report.Decomposition.ValueOfInformation <= 0 {
		t.Errorf("expected positive Value of Information, got $%.2f", report.Decomposition.ValueOfInformation)
	}
	if report.TTestInformedVsBlind.PValueOneTailed >= 0.05 {
		t.Errorf("expected VoI to be statistically significant (p < 0.05), got %e", report.TTestInformedVsBlind.PValueOneTailed)
	}
}
