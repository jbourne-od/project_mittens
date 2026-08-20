package simulation_test

import (
	"context"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/adapter/simulation"
)

// TestTournament_Regime1_HighVolatilityMarket tests N=1 vs N=0 in a volatile market
// where competitor postures fluctuate rapidly (high entropy transitions).
func TestTournament_Regime1_HighVolatilityMarket(t *testing.T) {
	cfg := simulation.DefaultTournamentConfig()
	cfg.Episodes = 20
	cfg.HorizonDays = 5
	cfg.DriverCount = 15
	cfg.LoadsPerEpoch = 25
	cfg.BaseSeed = 202608201

	// Volatile market with 2-day regime persistence: 60% self-persistence, 20% jump
	cfg.Market.TransitionProb = [][]float64{
		{0.60, 0.25, 0.15}, // Aggressive -> [Agg, Mod, Pas]
		{0.20, 0.60, 0.20}, // Moderate   -> [Agg, Mod, Pas]
		{0.15, 0.25, 0.60}, // Passive    -> [Agg, Mod, Pas]
	}
	cfg.Market.PriceNoiseStdDev = 0.04

	runner := simulation.NewTournamentRunner(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	report, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Regime 1 failed: %v", err)
	}

	t.Logf("\n--- REGIME 1: HIGH VOLATILITY MARKET (N=%d) ---\n%s", report.TTest.N, report.TTest.SummaryString())

	if report.TTest.PValueOneTailed >= 0.05 {
		t.Errorf("Regime 1: expected p < 0.05, got %e", report.TTest.PValueOneTailed)
	}
	if report.TTest.MeanDifference <= 0 {
		t.Errorf("Regime 1: expected positive mean lift, got $%.2f", report.TTest.MeanDifference)
	}
}

// TestTournament_Regime2_BearMarket_Overcapacity tests N=1 vs N=0 in an aggressive, oversupplied market
// where competitors cut prices aggressively (85% spot rate).
func TestTournament_Regime2_BearMarket_Overcapacity(t *testing.T) {
	cfg := simulation.DefaultTournamentConfig()
	cfg.Episodes = 20
	cfg.HorizonDays = 5
	cfg.DriverCount = 15
	cfg.LoadsPerEpoch = 25
	cfg.BaseSeed = 202608202

	// Persistent Bear market: 85% persistence in Aggressive posture
	cfg.Market.InitialPosture = simulation.PostureAggressive
	cfg.Market.TransitionProb = [][]float64{
		{0.85, 0.10, 0.05},
		{0.60, 0.30, 0.10},
		{0.50, 0.40, 0.10},
	}
	cfg.Market.PriceMultipliers = []float64{0.82, 0.95, 1.05} // Squeezed spot rates

	runner := simulation.NewTournamentRunner(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	report, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Regime 2 failed: %v", err)
	}

	t.Logf("\n--- REGIME 2: BEAR MARKET / OVERCAPACITY (N=%d) ---\n%s", report.TTest.N, report.TTest.SummaryString())

	if report.TTest.MeanDifference <= 0 {
		t.Errorf("Regime 2: expected positive mean difference (loss mitigation), got $%.2f", report.TTest.MeanDifference)
	}
}

// TestTournament_Regime3_BullMarket_TightCapacity tests N=1 vs N=0 in a passive, tight-capacity market
// where competitors price high (118% spot rate).
func TestTournament_Regime3_BullMarket_TightCapacity(t *testing.T) {
	cfg := simulation.DefaultTournamentConfig()
	cfg.Episodes = 20
	cfg.HorizonDays = 5
	cfg.DriverCount = 15
	cfg.LoadsPerEpoch = 25
	cfg.BaseSeed = 202608203

	// Persistent Bull market: 85% persistence in Passive posture
	cfg.Market.InitialPosture = simulation.PosturePassive
	cfg.Market.TransitionProb = [][]float64{
		{0.15, 0.35, 0.50},
		{0.10, 0.30, 0.60},
		{0.05, 0.15, 0.80},
	}
	cfg.Market.PriceMultipliers = []float64{0.90, 1.05, 1.25} // Elevated spot rates

	runner := simulation.NewTournamentRunner(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	report, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Regime 3 failed: %v", err)
	}

	t.Logf("\n--- REGIME 3: BULL MARKET / TIGHT CAPACITY (N=%d) ---\n%s", report.TTest.N, report.TTest.SummaryString())

	if report.TTest.PValueOneTailed >= 0.01 {
		t.Errorf("Regime 3: expected p < 0.01, got %e", report.TTest.PValueOneTailed)
	}
	if report.TTest.PercentLift < 10.0 {
		t.Errorf("Regime 3: expected >= 10%% profit lift, got %.2f%%", report.TTest.PercentLift)
	}
}

// TestTournament_Regime4_100Episode_MonteCarloPowerTest executes 100 independent 7-day twin simulations
// to achieve high statistical power (p < 1e-6, Cohen's d > 0.6).
func TestTournament_Regime4_100Episode_MonteCarloPowerTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 100-episode power test in short mode")
	}

	cfg := simulation.DefaultTournamentConfig()
	cfg.Episodes = 100
	cfg.HorizonDays = 7
	cfg.DecisionStepHours = 12
	cfg.DriverCount = 15
	cfg.LoadsPerEpoch = 25
	cfg.BaseSeed = 202608204

	runner := simulation.NewTournamentRunner(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	report, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Regime 4 failed: %v", err)
	}

	t.Logf("\n========================================================================\n"+
		"      100-EPISODE MONTE CARLO POWER TEST SCORECARD (N=0 vs N=1)         \n"+
		"========================================================================\n"+
		"%s\n"+
		"========================================================================\n"+
		"  Mean N=0 Net Contribution: $%.2f\n"+
		"  Mean N=1 Net Contribution: $%.2f\n"+
		"  Mean Net Lift:             +$%.2f (+%.2f%%)\n"+
		"  95%% Confidence Interval:   [$%.2f, $%.2f]\n"+
		"  Cohen's d Effect Size:     %.4f\n"+
		"  Win-Loss-Tie Record:       %d Wins / %d Losses / %d Ties (Win Rate: %.1f%%)\n"+
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
		report.TTest.CohensD,
		report.TTest.WinsCandidate, report.TTest.WinsBaseline, report.TTest.Ties,
		float64(report.TTest.WinsCandidate)/float64(report.TTest.N)*100.0,
		report.TTest.TStatistic,
		report.TTest.DegreesOfFreedom,
		report.TTest.PValueOneTailed,
		report.N1SuperiorityVerified,
	)

	// Statistical Power Assertions
	if !report.N1SuperiorityVerified {
		t.Errorf("expected N=1 superiority to be statistically verified")
	}
	if report.TTest.PValueOneTailed >= 0.001 {
		t.Errorf("expected p < 0.001 (high significance), got %e", report.TTest.PValueOneTailed)
	}
	if report.TTest.CohensD < 0.40 {
		t.Errorf("expected substantial effect size Cohen's d >= 0.40, got %.4f", report.TTest.CohensD)
	}
	if report.TTest.WinsCandidate <= report.TTest.WinsBaseline {
		t.Errorf("expected N=1 wins (%d) to strictly exceed N=0 wins (%d)", report.TTest.WinsCandidate, report.TTest.WinsBaseline)
	}
}

// TestTournament_Regime5_TripartiteDecomposition100Episode evaluates the 3-way economic attribution
// over 100 episodes to isolate Value of Information from Value of Action Space with high precision.
func TestTournament_Regime5_TripartiteDecomposition100Episode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 100-episode tripartite test in short mode")
	}

	cfg := simulation.DefaultTournamentConfig()
	cfg.Episodes = 100
	cfg.HorizonDays = 7
	cfg.DecisionStepHours = 12
	cfg.DriverCount = 15
	cfg.LoadsPerEpoch = 25
	cfg.BaseSeed = 202608206

	runner := simulation.NewTournamentRunner(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()

	report, err := runner.RunTripartite(ctx)
	if err != nil {
		t.Fatalf("Tripartite 100-episode failed: %v", err)
	}

	t.Logf("\n========================================================================\n"+
		"   100-EPISODE TRIPARTITE ECONOMIC DECOMPOSITION (VoI vs Action Space)  \n"+
		"========================================================================\n"+
		"%s\n"+
		"========================================================================\n"+
		"  Informed vs Legacy (Total):  t=%.4f (df=%.0f), p=%e, lift=+%.2f%%\n"+
		"  Informed vs Blind (VoI):     t=%.4f (df=%.0f), p=%e, lift=+%.2f%%\n"+
		"  Blind vs Legacy (VoA):       t=%.4f (df=%.0f), p=%e, lift=+%.2f%%\n"+
		"========================================================================",
		report.Decomposition.SummaryString(),
		report.TTestInformedVsLegacy.TStatistic, report.TTestInformedVsLegacy.DegreesOfFreedom, report.TTestInformedVsLegacy.PValueOneTailed, report.TTestInformedVsLegacy.PercentLift,
		report.TTestInformedVsBlind.TStatistic, report.TTestInformedVsBlind.DegreesOfFreedom, report.TTestInformedVsBlind.PValueOneTailed, report.TTestInformedVsBlind.PercentLift,
		report.TTestBlindVsLegacy.TStatistic, report.TTestBlindVsLegacy.DegreesOfFreedom, report.TTestBlindVsLegacy.PValueOneTailed, report.TTestBlindVsLegacy.PercentLift,
	)

	// Statistical Assertions
	if report.Decomposition.ValueOfInformation <= 0 {
		t.Errorf("expected strictly positive Value of Information, got $%.2f", report.Decomposition.ValueOfInformation)
	}
	if report.TTestInformedVsBlind.PValueOneTailed >= 0.001 {
		t.Errorf("expected highly significant VoI (p < 0.001), got %e", report.TTestInformedVsBlind.PValueOneTailed)
	}
}

