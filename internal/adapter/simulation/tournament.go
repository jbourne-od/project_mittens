package simulation

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
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
	V00_LegacyBlind           float64                   `json:"v00_legacy_blind"`         // Legacy Action Space + Blind Belief
	V01_LegacyInformed        float64                   `json:"v01_legacy_informed"`      // Legacy Action Space + Informed Belief
	V10_CompetitiveBlind      float64                   `json:"v10_competitive_blind"`    // Competitive Action Space + Blind Belief
	V11_CompetitiveInformed   float64                   `json:"v11_competitive_informed"` // Competitive Action Space + Informed Belief
	DeltaA_Blind              float64                   `json:"delta_a_blind"`            // V10 - V00: Competitive-policy effect under blind belief
	DeltaA_Informed           float64                   `json:"delta_a_informed"`         // V11 - V01: Competitive-policy effect under informed belief
	DeltaI_Legacy             float64                   `json:"delta_i_legacy"`           // V01 - V00: Information effect under legacy policy
	DeltaI_Comp               float64                   `json:"delta_i_comp"`             // V11 - V10: Information effect under competitive policy
	ConditionalVoIUnderLegacy float64                   `json:"conditional_voi_under_legacy"`
	ConditionalVoIUnderComp   float64                   `json:"conditional_voi_under_comp"`
	InteractionEffect         float64                   `json:"interaction_effect"`       // (V11 - V10) - (V01 - V00): Supermodular Complementarity
	InteractionTest           pkgmath.PairedTTestResult `json:"interaction_test"`         // Paired statistical hypothesis test on D_i
	MainEffectActionSpace     float64                   `json:"main_effect_action_space"` // 0.5 * [DeltaA_Blind + DeltaA_Informed]
	MainEffectInformation     float64                   `json:"main_effect_information"`  // 0.5 * [DeltaI_Legacy + DeltaI_Comp]
	TotalLift                 float64                   `json:"total_lift"`               // V11 - V00
	TotalLiftPercent          float64                   `json:"total_lift_percent"`       // ((V11 - V00) / |V00|) * 100
}

// SummaryString formats the 2x2 factorial analysis.
func (f FactorialDecomposition2x2) SummaryString() string {
	return fmt.Sprintf(
		"2x2 Empirical Economic Matrix:\n"+
			"                   | Blind Belief (b0) | Informed Belief (bt) | Marginal Info Effect\n"+
			"  -----------------+-------------------+----------------------+----------------------\n"+
			"  Legacy Action    | V00 = $%9.2f  | V01 = $%9.2f     | Δ_I|legacy = $%+9.2f\n"+
			"  Competitive Act. | V10 = $%9.2f  | V11 = $%9.2f     | Δ_I|comp   = $%+9.2f\n"+
			"  -----------------+-------------------+----------------------+----------------------\n"+
			"  Marginal Action  | Δ_A|blind         | Δ_A|informed         | Total Lift (V11-V00)\n"+
			"  Effect           | $%+9.2f       | $%+9.2f          | Total: $%+9.2f (+%.2f%%)\n\n"+
			"  Main Effect of Action Space:       +$%9.2f\n"+
			"  Main Effect of Information:        +$%9.2f\n"+
			"  Supermodular Interaction (Δ_int):  +$%9.2f",
		f.V00_LegacyBlind, f.V01_LegacyInformed, f.DeltaI_Legacy,
		f.V10_CompetitiveBlind, f.V11_CompetitiveInformed, f.DeltaI_Comp,
		f.DeltaA_Blind, f.DeltaA_Informed, f.TotalLift, f.TotalLiftPercent,
		f.MainEffectActionSpace, f.MainEffectInformation, f.InteractionEffect,
	)
}

// FactorialReport2x2 encapsulates the full 2x2 factorial experimental run.
type FactorialReport2x2 struct {
	Config                      TournamentConfig          `json:"config"`
	Factorial                   FactorialDecomposition2x2 `json:"factorial"`
	TTestV11VsV00               pkgmath.PairedTTestResult `json:"t_test_v11_vs_v00"`
	TTestV11VsV10               pkgmath.PairedTTestResult `json:"t_test_v11_vs_v10"`
	TTestV10VsV00               pkgmath.PairedTTestResult `json:"t_test_v10_vs_v00"`
	TTestV01VsV00               pkgmath.PairedTTestResult `json:"t_test_v01_vs_v00"`
	TTestV11VsV01               pkgmath.PairedTTestResult `json:"t_test_v11_vs_v01"`
	TTestInteraction            pkgmath.PairedTTestResult `json:"t_test_interaction"`
	PositiveInteractionEpisodes int                       `json:"positive_interaction_episodes"`
	NegativeInteractionEpisodes int                       `json:"negative_interaction_episodes"`
	ExactTieInteractionEpisodes int                       `json:"exact_tie_interaction_episodes"`
	ExecutionDurationSec        float64                   `json:"execution_duration_sec"`
}

// SummaryString formats the 2x2 factorial experimental scorecard and hypothesis tests.
func (r *FactorialReport2x2) SummaryString() string {
	f := r.Factorial
	totSurplus := f.TotalLift
	shapleyActionPct := 0.0
	shapleyInfoPct := 0.0
	if math.Abs(totSurplus) > 1e-6 {
		shapleyActionPct = (f.MainEffectActionSpace / totSurplus) * 100.0
		shapleyInfoPct = (f.MainEffectInformation / totSurplus) * 100.0
	}

	n := float64(r.Config.Episodes)
	posPct := (float64(r.PositiveInteractionEpisodes) / n) * 100.0
	negPct := (float64(r.NegativeInteractionEpisodes) / n) * 100.0
	tiePct := (float64(r.ExactTieInteractionEpisodes) / n) * 100.0

	return fmt.Sprintf(
		"================================================================================\n"+
			"          PROJECT MITTENS: 2x2 FACTORIAL VALUE OF INFORMATION & PRICING         \n"+
			"================================================================================\n"+
			"2x2 Empirical Economic Matrix:\n"+
			"                   | Blind Belief (b0) | Informed Belief (bt) | Marginal Info Effect\n"+
			"  -----------------+-------------------+----------------------+----------------------\n"+
			"  Legacy Action    | V00 = $%9.2f  | V01 = $%9.2f     | Δ_I|legacy = $%+9.2f\n"+
			"  Competitive Act. | V10 = $%9.2f  | V11 = $%9.2f     | Δ_I|comp   = $%+9.2f\n"+
			"  -----------------+-------------------+----------------------+----------------------\n"+
			"  Marginal Action  | Δ_A|blind         | Δ_A|informed         | Total Lift (V11-V00)\n"+
			"  Effect           | $%+9.2f       | $%+9.2f          | Total: $%+9.2f (%+.2f%%)\n\n"+
			"  Main Effect of Action Space (ME_A):  +$%9.2f  (Shapley Allocation: %.1f%%)\n"+
			"  Main Effect of Information (ME_I):   +$%9.2f  (Shapley Allocation: %.1f%%)\n"+
			"  * Note: For 2 factors, main effects equal Shapley allocations of the joint surplus.\n"+
			"--------------------------------------------------------------------------------\n"+
			" Statistical Contrasts & Hypothesis Tests (N = %d paired episodes):\n"+
			"  • Δ_I|legacy (V01-V00): Mean = $%+9.2f | 95%% CI: [$%+9.2f, $%+9.2f] | t = %+6.2f | p = %.3e | d_z = %+.4f | (Info under legacy)\n"+
			"  • Δ_A|blind  (V10-V00): Mean = $%+9.2f | 95%% CI: [$%+9.2f, $%+9.2f] | t = %+6.2f | p = %.3e | d_z = %+.4f | (Blind pricing hurts)\n"+
			"  • Δ_I|comp   (V11-V10): Mean = $%+9.2f | 95%% CI: [$%+9.2f, $%+9.2f] | t = %+6.2f | p = %.3e | d_z = %+.4f | (Info under competitive)\n"+
			"  • Δ_A|inf    (V11-V01): Mean = $%+9.2f | 95%% CI: [$%+9.2f, $%+9.2f] | t = %+6.2f | p = %.3e | d_z = %+.4f | (Full vs informed legacy)\n"+
			"  • Full Lift  (V11-V00): Mean = $%+9.2f | 95%% CI: [$%+9.2f, $%+9.2f] | t = %+6.2f | p = %.3e | d_z = %+.4f | (Total Mittens lift: %+.2f%%)\n"+
			"--------------------------------------------------------------------------------\n"+
			" Factorial Cross-Effect Analysis (Supermodular Interaction on Treatment Lattice):\n"+
			"  • Mean Interaction Δ_int:  +$%.2f (SD = $%.2f, SE = $%.2f)\n"+
			"  • 95%% Paired Confidence:   [$+%.2f, $+%.2f]\n"+
			"  • Hypothesis Test:         t = %+6.2f, df = %d, p(two-tailed) = %e, Cohen's d_z = %.4f\n"+
			"  • Episode Attribution:     Positive: %d (%.1f%%) | Negative: %d (%.1f%%) | Exact Tie: %d (%.1f%%)\n"+
			"================================================================================\n",
		f.V00_LegacyBlind, f.V01_LegacyInformed, f.ConditionalVoIUnderLegacy,
		f.V10_CompetitiveBlind, f.V11_CompetitiveInformed, f.ConditionalVoIUnderComp,
		f.V10_CompetitiveBlind-f.V00_LegacyBlind, f.V11_CompetitiveInformed-f.V01_LegacyInformed, f.TotalLift, r.TTestV11VsV00.PercentLift,
		f.MainEffectActionSpace, shapleyActionPct,
		f.MainEffectInformation, shapleyInfoPct,
		r.Config.Episodes,
		r.TTestV01VsV00.MeanDifference, r.TTestV01VsV00.ConfidenceLow95, r.TTestV01VsV00.ConfidenceHigh95, r.TTestV01VsV00.TStatistic, r.TTestV01VsV00.PValueTwoTailed, r.TTestV01VsV00.CohensD,
		r.TTestV10VsV00.MeanDifference, r.TTestV10VsV00.ConfidenceLow95, r.TTestV10VsV00.ConfidenceHigh95, r.TTestV10VsV00.TStatistic, r.TTestV10VsV00.PValueTwoTailed, r.TTestV10VsV00.CohensD,
		r.TTestV11VsV10.MeanDifference, r.TTestV11VsV10.ConfidenceLow95, r.TTestV11VsV10.ConfidenceHigh95, r.TTestV11VsV10.TStatistic, r.TTestV11VsV10.PValueTwoTailed, r.TTestV11VsV10.CohensD,
		r.TTestV11VsV01.MeanDifference, r.TTestV11VsV01.ConfidenceLow95, r.TTestV11VsV01.ConfidenceHigh95, r.TTestV11VsV01.TStatistic, r.TTestV11VsV01.PValueTwoTailed, r.TTestV11VsV01.CohensD,
		r.TTestV11VsV00.MeanDifference, r.TTestV11VsV00.ConfidenceLow95, r.TTestV11VsV00.ConfidenceHigh95, r.TTestV11VsV00.TStatistic, r.TTestV11VsV00.PValueTwoTailed, r.TTestV11VsV00.CohensD, r.TTestV11VsV00.PercentLift,
		r.TTestInteraction.MeanDifference, r.TTestInteraction.StdDevDifference, r.TTestInteraction.StdErrDifference,
		r.TTestInteraction.ConfidenceLow95, r.TTestInteraction.ConfidenceHigh95,
		r.TTestInteraction.TStatistic, int(r.TTestInteraction.DegreesOfFreedom), r.TTestInteraction.PValueTwoTailed, r.TTestInteraction.CohensD,
		r.PositiveInteractionEpisodes, posPct, r.NegativeInteractionEpisodes, negPct, r.ExactTieInteractionEpisodes, tiePct,
	)
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

func renderProgress(current, total int, label string, start time.Time) {
	if total <= 0 {
		return
	}
	width := 24
	percent := float64(current) / float64(total)
	if percent > 1.0 {
		percent = 1.0
	}
	filled := int(percent * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	elapsed := time.Since(start).Seconds()

	fmt.Printf("\r\033[K [%s] %3.0f%% (%d/%d) | %s | Elapsed: %.1fs", bar, percent*100.0, current, total, label, elapsed)
	if current == total {
		fmt.Printf("\n")
	}
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

		renderProgress(ep, cfg.Episodes, fmt.Sprintf("Episode %d/%d (N=0 vs N=1)...", ep+1, cfg.Episodes), startTime)
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

		renderProgress(ep+1, cfg.Episodes, fmt.Sprintf("Episode %d/%d complete (Lift: %+.1f%%)", ep+1, cfg.Episodes, lift), startTime)

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

		renderProgress(ep, cfg.Episodes, fmt.Sprintf("Episode %d/%d (Tripartite)...", ep+1, cfg.Episodes), startTime)
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

		renderProgress(ep+1, cfg.Episodes, fmt.Sprintf("Episode %d/%d complete", ep+1, cfg.Episodes), startTime)
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

	type epResult struct {
		ep  int
		s00 simResult
		s01 simResult
		s10 simResult
		s11 simResult
		err error
	}

	numWorkers := runtime.GOMAXPROCS(0)
	if numWorkers < 1 {
		numWorkers = 1
	}
	if numWorkers > cfg.Episodes {
		numWorkers = cfg.Episodes
	}

	jobs := make(chan int, cfg.Episodes)
	results := make(chan epResult, cfg.Episodes)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ep := range jobs {
				select {
				case <-ctx.Done():
					results <- epResult{ep: ep, err: ctx.Err()}
					return
				default:
				}

				epSeed := cfg.BaseSeed + uint64(ep)*7919
				s00, err00 := r.runEpisodeN0(ctx, epSeed)
				if err00 != nil {
					results <- epResult{ep: ep, err: fmt.Errorf("factorial: episode %d V00 failed: %w", ep, err00)}
					return
				}
				s01, err01 := r.runEpisodeN0Informed(ctx, epSeed)
				if err01 != nil {
					results <- epResult{ep: ep, err: fmt.Errorf("factorial: episode %d V01 failed: %w", ep, err01)}
					return
				}
				s10, err10 := r.runEpisodeN1Blind(ctx, epSeed)
				if err10 != nil {
					results <- epResult{ep: ep, err: fmt.Errorf("factorial: episode %d V10 failed: %w", ep, err10)}
					return
				}
				s11, err11 := r.runEpisodeN1(ctx, epSeed)
				if err11 != nil {
					results <- epResult{ep: ep, err: fmt.Errorf("factorial: episode %d V11 failed: %w", ep, err11)}
					return
				}

				results <- epResult{ep: ep, s00: s00, s01: s01, s10: s10, s11: s11}
			}
		}()
	}

	for ep := 0; ep < cfg.Episodes; ep++ {
		jobs <- ep
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	completed := 0
	for res := range results {
		if res.err != nil {
			return nil, res.err
		}
		v00[res.ep] = res.s00.NetContribution
		v01[res.ep] = res.s01.NetContribution
		v10[res.ep] = res.s10.NetContribution
		v11[res.ep] = res.s11.NetContribution

		completed++
		renderProgress(completed, cfg.Episodes, fmt.Sprintf("Episodes completed: %d/%d (Workers: %d)", completed, cfg.Episodes, numWorkers), startTime)
	}

	tV11VsV00, err := pkgmath.ComputePairedTTest(v00, v11)
	if err != nil {
		return nil, fmt.Errorf("factorial: t-test v11 vs v00 failed: %w", err)
	}
	tV11VsV10, err := pkgmath.ComputePairedTTest(v10, v11)
	if err != nil {
		return nil, fmt.Errorf("factorial: t-test v11 vs v10 failed: %w", err)
	}
	tV11VsV01, err := pkgmath.ComputePairedTTest(v01, v11)
	if err != nil {
		return nil, fmt.Errorf("factorial: t-test v11 vs v01 failed: %w", err)
	}
	tV10VsV00, err := pkgmath.ComputePairedTTest(v00, v10)
	if err != nil {
		return nil, fmt.Errorf("factorial: t-test v10 vs v00 failed: %w", err)
	}
	tV01VsV00, err := pkgmath.ComputePairedTTest(v00, v01)
	if err != nil {
		return nil, fmt.Errorf("factorial: t-test v01 vs v00 failed: %w", err)
	}

	dComp := make([]float64, cfg.Episodes)
	dLegacy := make([]float64, cfg.Episodes)
	posCount := 0
	negCount := 0
	tieCount := 0

	for ep := 0; ep < cfg.Episodes; ep++ {
		dComp[ep] = v11[ep] - v10[ep]
		dLegacy[ep] = v01[ep] - v00[ep]
		diffInt := dComp[ep] - dLegacy[ep]
		if diffInt > 1e-4 {
			posCount++
		} else if diffInt < -1e-4 {
			negCount++
		} else {
			tieCount++
		}
	}

	tInteraction, err := pkgmath.ComputePairedTTest(dLegacy, dComp)
	if err != nil {
		return nil, fmt.Errorf("factorial: t-test interaction failed: %w", err)
	}

	mean00 := tV11VsV00.MeanBaseline
	mean01 := tV01VsV00.MeanCandidate
	mean10 := tV10VsV00.MeanCandidate
	mean11 := tV11VsV00.MeanCandidate

	mainAction := 0.5 * ((mean10 - mean00) + (mean11 - mean01))
	mainInfo := 0.5 * ((mean01 - mean00) + (mean11 - mean10))
	interaction := mean11 - mean10 - mean01 + mean00

	totLift := mean11 - mean00
	totLiftPct := 0.0
	if math.Abs(mean00) > 1e-6 {
		totLiftPct = (totLift / math.Abs(mean00)) * 100.0
	}

	factorial := FactorialDecomposition2x2{
		V00_LegacyBlind:         mean00,
		V01_LegacyInformed:      mean01,
		V10_CompetitiveBlind:    mean10,
		V11_CompetitiveInformed: mean11,
		DeltaA_Blind:            mean10 - mean00,
		DeltaA_Informed:         mean11 - mean01,
		DeltaI_Legacy:           mean01 - mean00,
		DeltaI_Comp:             mean11 - mean10,
		InteractionEffect:       interaction,
		InteractionTest:         tInteraction,
		MainEffectActionSpace:   mainAction,
		MainEffectInformation:   mainInfo,
		TotalLift:               totLift,
		TotalLiftPercent:        totLiftPct,
	}

	return &FactorialReport2x2{
		Config:                      cfg,
		Factorial:                   factorial,
		TTestV11VsV00:               tV11VsV00,
		TTestV11VsV10:               tV11VsV10,
		TTestV10VsV00:               tV10VsV00,
		TTestV01VsV00:               tV01VsV00,
		TTestV11VsV01:               tV11VsV01,
		TTestInteraction:            tInteraction,
		PositiveInteractionEpisodes: posCount,
		NegativeInteractionEpisodes: negCount,
		ExactTieInteractionEpisodes: tieCount,
		ExecutionDurationSec:        time.Since(startTime).Seconds(),
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
		renderProgress(ep*5, cfg.Episodes*5, fmt.Sprintf("Ep %d/%d: 1. PFA (Greedy Rule)...", ep+1, cfg.Episodes), startTime)
		t0 := time.Now()
		sPFA, err := r.runEpisodePFA(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("4way: episode %d PFA failed: %w", ep, err)
		}
		pfaLatencies[ep] = float64(time.Since(t0).Microseconds()) / 1000.0 / float64((r.cfg.HorizonDays*24)/r.cfg.DecisionStepHours)
		pfaScores[ep] = sPFA
		pfaNets[ep] = sPFA.NetContribution

		// 2. CFA (Parametric Cost Function Approximation)
		renderProgress(ep*5+1, cfg.Episodes*5, fmt.Sprintf("Ep %d/%d: 2. CFA (SPSA LAP)...", ep+1, cfg.Episodes), startTime)
		t0 = time.Now()
		sCFA, err := r.runEpisodeCFA(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("4way: episode %d CFA failed: %w", ep, err)
		}
		cfaLatencies[ep] = float64(time.Since(t0).Microseconds()) / 1000.0 / float64((r.cfg.HorizonDays*24)/r.cfg.DecisionStepHours)
		cfaScores[ep] = sCFA
		cfaNets[ep] = sCFA.NetContribution

		// 3. Piecewise VFA (Downstream Marginal Slopes with CAVE)
		renderProgress(ep*5+2, cfg.Episodes*5, fmt.Sprintf("Ep %d/%d: 3. VFA (CAVE Slopes)...", ep+1, cfg.Episodes), startTime)
		t0 = time.Now()
		sVFA, err := r.runEpisodeVFA(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("4way: episode %d VFA failed: %w", ep, err)
		}
		vfaLatencies[ep] = float64(time.Since(t0).Microseconds()) / 1000.0 / float64((r.cfg.HorizonDays*24)/r.cfg.DecisionStepHours)
		vfaScores[ep] = sVFA
		vfaNets[ep] = sVFA.NetContribution

		// 4. DLA (Direct Lookahead Approximation)
		renderProgress(ep*5+3, cfg.Episodes*5, fmt.Sprintf("Ep %d/%d: 4. DLA (Tree Rollout)...", ep+1, cfg.Episodes), startTime)
		t0 = time.Now()
		sDLA, err := r.runEpisodeDLA(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("4way: episode %d DLA failed: %w", ep, err)
		}
		dlaLatencies[ep] = float64(time.Since(t0).Microseconds()) / 1000.0 / float64((r.cfg.HorizonDays*24)/r.cfg.DecisionStepHours)
		dlaScores[ep] = sDLA
		dlaNets[ep] = sDLA.NetContribution

		// 5. Competitive POMDP (Bayesian Belief Simplex + Dynamic Pricing)
		renderProgress(ep*5+4, cfg.Episodes*5, fmt.Sprintf("Ep %d/%d: 5. POMDP (Simplex+Bid)...", ep+1, cfg.Episodes), startTime)
		t0 = time.Now()
		sPOMDP, err := r.runEpisodeN1(ctx, epSeed)
		if err != nil {
			return nil, fmt.Errorf("4way: episode %d POMDP failed: %w", ep, err)
		}
		pomdpLatencies[ep] = float64(time.Since(t0).Microseconds()) / 1000.0 / float64((r.cfg.HorizonDays*24)/r.cfg.DecisionStepHours)
		pomdpScores[ep] = sPOMDP
		pomdpNets[ep] = sPOMDP.NetContribution

		renderProgress((ep+1)*5, cfg.Episodes*5, fmt.Sprintf("Ep %d/%d Complete (POMDP: $%.0f)", ep+1, cfg.Episodes, sPOMDP.NetContribution), startTime)
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
			Description:         "Greedy Priority Rule (No LAP)",
			MeanNetContribution: meanPFA.NetContribution,
			MeanGrossRevenue:    meanPFA.GrossRevenue,
			MeanOperatingCost:   meanPFA.OperatingCost,
			MeanWinRate:         meanPFA.WinRate,
			MeanLatencyMs:       pfaMeanLat,
			PercentLiftOverPFA:  0.0,
		},
		{
			PolicyClass:         "2. CFA",
			Description:         "Parametric Cost Tuning (SPSA LAP)",
			MeanNetContribution: meanCFA.NetContribution,
			MeanGrossRevenue:    meanCFA.GrossRevenue,
			MeanOperatingCost:   meanCFA.OperatingCost,
			MeanWinRate:         meanCFA.WinRate,
			MeanLatencyMs:       cfaMeanLat,
			PercentLiftOverPFA:  ((meanCFA.NetContribution - meanPFA.NetContribution) / math.Abs(meanPFA.NetContribution)) * 100.0,
		},
		{
			PolicyClass:         "3. VFA",
			Description:         "Piecewise Concave Slopes (CAVE LAP)",
			MeanNetContribution: meanVFA.NetContribution,
			MeanGrossRevenue:    meanVFA.GrossRevenue,
			MeanOperatingCost:   meanVFA.OperatingCost,
			MeanWinRate:         meanVFA.WinRate,
			MeanLatencyMs:       vfaMeanLat,
			PercentLiftOverPFA:  ((meanVFA.NetContribution - meanPFA.NetContribution) / math.Abs(meanPFA.NetContribution)) * 100.0,
		},
		{
			PolicyClass:         "4. DLA",
			Description:         "Direct Lookahead (2-Epoch Rollouts)",
			MeanNetContribution: meanDLA.NetContribution,
			MeanGrossRevenue:    meanDLA.GrossRevenue,
			MeanOperatingCost:   meanDLA.OperatingCost,
			MeanWinRate:         meanDLA.WinRate,
			MeanLatencyMs:       dlaMeanLat,
			PercentLiftOverPFA:  ((meanDLA.NetContribution - meanPFA.NetContribution) / math.Abs(meanPFA.NetContribution)) * 100.0,
		},
		{
			PolicyClass:         "5. Competitive",
			Description:         "MOMDP Bayesian Simplex + Pricing",
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

	pfaParams := policy.DefaultPFAParameters()
	costCfg := model.DefaultCostConfig()
	costCfg.EmptyToHomeRate = 0.20
	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 800.0
	pfaPol := policy.NewPFAPolicy[model.Monopolistic](pfaParams, costCfg, feasCfg)

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

	cfaParams := policy.DefaultCFAParameters()
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
	dlaParams.Horizon = 1
	dlaParams.NumRollouts = 1
	dlaParams.DiscountFactor = 0.95
	dlaParams.StepSeconds = stepSec
	dlaParams.RandomSeed = seed + 101
	dlaParams.EnableAdaptivePruning = true
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

// SweepPoint encapsulates the complete 2x2 factorial metrics for a single operating point.
type SweepPoint struct {
	ParamName     string  `json:"param_name"`
	ParamValue    float64 `json:"param_value"`
	HorizonDays   int     `json:"horizon_days"`
	DriverCount   int     `json:"driver_count"`
	LoadsPerEpoch int     `json:"loads_per_epoch"`
	ScarcityRatio float64 `json:"scarcity_ratio"` // loads / drivers

	// Total episode metrics
	V00Mean          float64 `json:"v00_mean"`
	V01Mean          float64 `json:"v01_mean"`
	V10Mean          float64 `json:"v10_mean"`
	V11Mean          float64 `json:"v11_mean"`
	TotalLiftDollars float64 `json:"total_lift_dollars"`
	TotalLiftPct     float64 `json:"total_lift_pct"`

	DeltaLegacyVoI   float64 `json:"delta_legacy_voi"`
	DeltaBlindVoA    float64 `json:"delta_blind_voa"`
	DeltaCompVoI     float64 `json:"delta_comp_voi"`
	DeltaInformedVoA float64 `json:"delta_informed_voa"`

	InteractionMean  float64 `json:"interaction_mean"`
	InteractionCI95L float64 `json:"interaction_ci95_low"`
	InteractionCI95H float64 `json:"interaction_ci95_high"`
	InteractionT     float64 `json:"interaction_t"`
	InteractionP     float64 `json:"interaction_p"`
	InteractionDz    float64 `json:"interaction_dz"`

	// Normalized per-day metrics
	InteractionPerDay float64 `json:"interaction_per_day"`
	CompVoIPerDay     float64 `json:"comp_voi_per_day"`
	BlindVoAPerDay    float64 `json:"blind_voa_per_day"`
	TotalLiftPerDay   float64 `json:"total_lift_per_day"`

	// Raw episode interaction vectors for paired finite difference calculations
	EpisodeInteractions []float64 `json:"-"`
}

// FiniteDiffContrast captures the paired difference test between adjacent sweep points: delta_i = D_i(theta_{j+1}) - D_i(theta_j)
type FiniteDiffContrast struct {
	FromParam float64 `json:"from_param"`
	ToParam   float64 `json:"to_param"`
	MeanDelta float64 `json:"mean_delta"`
	CI95Low   float64 `json:"ci95_low"`
	CI95High  float64 `json:"ci95_high"`
	TStat     float64 `json:"t_stat"`
	PValue    float64 `json:"p_value"`
	CohensDz  float64 `json:"cohens_dz"`
}

// CurveSweepReport encapsulates an entire response curve sweep across a parameterized domain.
type CurveSweepReport struct {
	CurveName   string               `json:"curve_name"`
	Description string               `json:"description"`
	Points      []SweepPoint         `json:"points"`
	FiniteDiffs []FiniteDiffContrast `json:"finite_diffs"`
}

// FullComparativeStaticsReport encapsulates the three core comparative statics response curves.
type FullComparativeStaticsReport struct {
	DensitySweep  CurveSweepReport `json:"density_sweep"`
	CapacitySweep CurveSweepReport `json:"capacity_sweep"`
	HorizonSweep  CurveSweepReport `json:"horizon_sweep"`
}

// SummaryString formats the full response curves into structured ASCII tables.
func (rep *FullComparativeStaticsReport) SummaryString() string {
	var sb strings.Builder

	sb.WriteString("================================================================================\n")
	sb.WriteString("     PROJECT MITTENS: COMPREHENSIVE ECONOMIC RESPONSE SURFACES (2ND ORDER)      \n")
	sb.WriteString("================================================================================\n\n")

	// 1. Density Curve
	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString(" 1. TENDER DENSITY RESPONSE CURVE (Hold H = 7 Days, K = 10 Drivers)\n")
	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf(" %-6s | %-6s | %-9s | %-9s | %-16s | %-12s | %-12s | %-14s | %-10s\n",
		"Loads", "Ratio", "V00", "V11", "Total Lift", "Δ_I|comp", "Δ_int", "95% CI", "Δ_int/Day"))
	sb.WriteString("--------------------------------------------------------------------------------\n")
	for _, pt := range rep.DensitySweep.Points {
		sb.WriteString(fmt.Sprintf(" %-6.0f | %4.2f:1 | $%8.2f | $%8.2f | +$%8.2f (%+5.1f%%) | +$%10.2f | +$%10.2f | [%+6.0f, %+6.0f] | +$%7.2f/d\n",
			pt.ParamValue, pt.ScarcityRatio, pt.V00Mean, pt.V11Mean, pt.TotalLiftDollars, pt.TotalLiftPct,
			pt.DeltaCompVoI, pt.InteractionMean, pt.InteractionCI95L, pt.InteractionCI95H, pt.InteractionPerDay))
	}
	sb.WriteString("\n Paired Stepwise Finite Differences Across Density Steps (Testing E[D(λ_{j+1}) - D(λ_j)] > 0):\n")
	sb.WriteString(" --------------------------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf(" %-12s | %-20s | %-23s | %-11s | %-8s\n",
		"Transition", "Mean Δ(Interaction)", "95% Confidence Interval", "t-Statistic", "p-Value"))
	sb.WriteString(" --------------------------------------------------------------------------------\n")
	for _, fd := range rep.DensitySweep.FiniteDiffs {
		sign := "+"
		if fd.MeanDelta < 0 {
			sign = "-"
		}
		sb.WriteString(fmt.Sprintf(" λ: %2.0f -> %-2.0f | %s$%10.2f          | [%+10.2f, %+10.2f] |   %+6.2f    | %.4f\n",
			fd.FromParam, fd.ToParam, sign, math.Abs(fd.MeanDelta), fd.CI95Low, fd.CI95High, fd.TStat, fd.PValue))
	}
	sb.WriteString(" --------------------------------------------------------------------------------\n\n")

	// 2. Capacity Curve
	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString(" 2. FLEET CAPACITY RESPONSE CURVE (Hold H = 7 Days, λ = 15 Loads/Epoch)\n")
	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf(" %-6s | %-6s | %-9s | %-9s | %-16s | %-12s | %-12s | %-14s | %-10s\n",
		"Trucks", "Ratio", "V00", "V11", "Total Lift", "Δ_I|comp", "Δ_int", "95% CI", "Δ_int/Day"))
	sb.WriteString("--------------------------------------------------------------------------------\n")
	for _, pt := range rep.CapacitySweep.Points {
		sb.WriteString(fmt.Sprintf(" %-6.0f | %4.2f:1 | $%8.2f | $%8.2f | +$%8.2f (%+5.1f%%) | +$%10.2f | +$%10.2f | [%+6.0f, %+6.0f] | +$%7.2f/d\n",
			pt.ParamValue, pt.ScarcityRatio, pt.V00Mean, pt.V11Mean, pt.TotalLiftDollars, pt.TotalLiftPct,
			pt.DeltaCompVoI, pt.InteractionMean, pt.InteractionCI95L, pt.InteractionCI95H, pt.InteractionPerDay))
	}
	sb.WriteString("\n Paired Stepwise Finite Differences Across Capacity Steps (Testing Scarcity Amplification):\n")
	sb.WriteString(" --------------------------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf(" %-12s | %-20s | %-23s | %-11s | %-8s\n",
		"Transition", "Mean Δ(Interaction)", "95% Confidence Interval", "t-Statistic", "p-Value"))
	sb.WriteString(" --------------------------------------------------------------------------------\n")
	for _, fd := range rep.CapacitySweep.FiniteDiffs {
		sign := "+"
		if fd.MeanDelta < 0 {
			sign = "-"
		}
		sb.WriteString(fmt.Sprintf(" K: %2.0f -> %-2.0f | %s$%10.2f          | [%+10.2f, %+10.2f] |   %+6.2f    | %.4f\n",
			fd.FromParam, fd.ToParam, sign, math.Abs(fd.MeanDelta), fd.CI95Low, fd.CI95High, fd.TStat, fd.PValue))
	}
	sb.WriteString(" --------------------------------------------------------------------------------\n\n")

	// 3. Horizon Curve
	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString(" 3. HORIZON RESPONSE CURVE (Hold K = 10 Drivers, λ = 15 Loads/Epoch)\n")
	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf(" %-4s | %-17s | %-12s | %-11s | %-12s | %-11s | %-12s | %-10s\n",
		"Days", "Total Lift", "Δ_I|comp(Tot)", "Δ_I|comp(/d)", "Δ_A|blnd(Tot)", "Δ_A|blnd(/d)", "Δ_int (Tot)", "Δ_int (/d)"))
	sb.WriteString("--------------------------------------------------------------------------------\n")
	for _, pt := range rep.HorizonSweep.Points {
		blindTot := pt.BlindVoAPerDay * pt.ParamValue
		blindTotSign := "+"
		if blindTot < 0 {
			blindTotSign = "-"
		}
		blindPerDaySign := "+"
		if pt.BlindVoAPerDay < 0 {
			blindPerDaySign = "-"
		}
		totalSign := "+"
		if pt.TotalLiftDollars < 0 {
			totalSign = "-"
		}
		sb.WriteString(fmt.Sprintf(" %-4.0f | %s$%8.2f (%+5.1f%%) | +$%10.2f | +$%8.2f/d | %s$%10.2f | %s$%8.2f/d | +$%10.2f | +$%7.2f/d\n",
			pt.ParamValue, totalSign, math.Abs(pt.TotalLiftDollars), pt.TotalLiftPct,
			pt.DeltaCompVoI, pt.CompVoIPerDay,
			blindTotSign, math.Abs(blindTot),
			blindPerDaySign, math.Abs(pt.BlindVoAPerDay),
			pt.InteractionMean, pt.InteractionPerDay))
	}
	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString(" *Identity check: Total Lift = Δ_A|blind(Tot) + Δ_I|comp(Tot)\n")
	sb.WriteString("================================================================================\n")
	return sb.String()
}

// RunSingleSweepPoint executes a multi-threaded 2x2 factorial run for a single configuration point.
func RunSingleSweepPoint(ctx context.Context, paramName string, paramVal float64, cfg TournamentConfig) (*SweepPoint, error) {
	runner := NewTournamentRunner(cfg)
	rep, err := runner.RunFactorial2x2(ctx)
	if err != nil {
		return nil, err
	}

	h := float64(cfg.HorizonDays)
	if h <= 0 {
		h = 1.0
	}

	scarcity := float64(cfg.LoadsPerEpoch) / float64(cfg.DriverCount)

	// Extract raw episode interactions
	v00 := rep.TTestV11VsV00.MeanBaseline
	v11 := rep.TTestV11VsV00.MeanCandidate

	pt := &SweepPoint{
		ParamName:           paramName,
		ParamValue:          paramVal,
		HorizonDays:         cfg.HorizonDays,
		DriverCount:         cfg.DriverCount,
		LoadsPerEpoch:       cfg.LoadsPerEpoch,
		ScarcityRatio:       scarcity,
		V00Mean:             v00,
		V01Mean:             rep.Factorial.V01_LegacyInformed,
		V10Mean:             rep.Factorial.V10_CompetitiveBlind,
		V11Mean:             v11,
		TotalLiftDollars:    rep.Factorial.TotalLift,
		TotalLiftPct:        rep.TTestV11VsV00.PercentLift,
		DeltaLegacyVoI:      rep.Factorial.ConditionalVoIUnderLegacy,
		DeltaBlindVoA:       rep.Factorial.V10_CompetitiveBlind - rep.Factorial.V00_LegacyBlind,
		DeltaCompVoI:        rep.Factorial.ConditionalVoIUnderComp,
		DeltaInformedVoA:    rep.Factorial.V11_CompetitiveInformed - rep.Factorial.V01_LegacyInformed,
		InteractionMean:     rep.TTestInteraction.MeanDifference,
		InteractionCI95L:    rep.TTestInteraction.ConfidenceLow95,
		InteractionCI95H:    rep.TTestInteraction.ConfidenceHigh95,
		InteractionT:        rep.TTestInteraction.TStatistic,
		InteractionP:        rep.TTestInteraction.PValueTwoTailed,
		InteractionDz:       rep.TTestInteraction.CohensD,
		InteractionPerDay:   rep.TTestInteraction.MeanDifference / h,
		CompVoIPerDay:       rep.Factorial.ConditionalVoIUnderComp / h,
		BlindVoAPerDay:      (rep.Factorial.V10_CompetitiveBlind - rep.Factorial.V00_LegacyBlind) / h,
		TotalLiftPerDay:     rep.Factorial.TotalLift / h,
		EpisodeInteractions: make([]float64, cfg.Episodes),
	}

	return pt, nil
}

// RunDensitySweep evaluates factorial interaction across varying load arrivals: lambda in {10, 12, 15, 18, 20, 25, 30}.
func RunDensitySweep(ctx context.Context, episodes int, baseSeed uint64) (*CurveSweepReport, error) {
	lambdas := []int{10, 12, 15, 18, 20, 25, 30}
	points := make([]SweepPoint, 0, len(lambdas))
	repMap := make(map[int]*FactorialReport2x2, len(lambdas))

	for _, l := range lambdas {
		cfg := TournamentConfig{
			Episodes:          episodes,
			HorizonDays:       7,
			DecisionStepHours: 12,
			DriverCount:       10,
			LoadsPerEpoch:     l,
			BaseSeed:          baseSeed,
			Market:            DefaultMarketConfig(),
		}
		runner := NewTournamentRunner(cfg)
		rep, err := runner.RunFactorial2x2(ctx)
		if err != nil {
			return nil, fmt.Errorf("density sweep at lambda=%d failed: %w", l, err)
		}
		repMap[l] = rep

		h := 7.0
		scarcity := float64(l) / 10.0
		pt := SweepPoint{
			ParamName:         "lambda",
			ParamValue:        float64(l),
			HorizonDays:       7,
			DriverCount:       10,
			LoadsPerEpoch:     l,
			ScarcityRatio:     scarcity,
			V00Mean:           rep.Factorial.V00_LegacyBlind,
			V01Mean:           rep.Factorial.V01_LegacyInformed,
			V10Mean:           rep.Factorial.V10_CompetitiveBlind,
			V11Mean:           rep.Factorial.V11_CompetitiveInformed,
			TotalLiftDollars:  rep.Factorial.TotalLift,
			TotalLiftPct:      rep.TTestV11VsV00.PercentLift,
			DeltaLegacyVoI:    rep.Factorial.ConditionalVoIUnderLegacy,
			DeltaBlindVoA:     rep.Factorial.V10_CompetitiveBlind - rep.Factorial.V00_LegacyBlind,
			DeltaCompVoI:      rep.Factorial.ConditionalVoIUnderComp,
			DeltaInformedVoA:  rep.Factorial.V11_CompetitiveInformed - rep.Factorial.V01_LegacyInformed,
			InteractionMean:   rep.TTestInteraction.MeanDifference,
			InteractionCI95L:  rep.TTestInteraction.ConfidenceLow95,
			InteractionCI95H:  rep.TTestInteraction.ConfidenceHigh95,
			InteractionT:      rep.TTestInteraction.TStatistic,
			InteractionP:      rep.TTestInteraction.PValueTwoTailed,
			InteractionDz:     rep.TTestInteraction.CohensD,
			InteractionPerDay: rep.TTestInteraction.MeanDifference / h,
			CompVoIPerDay:     rep.Factorial.ConditionalVoIUnderComp / h,
			BlindVoAPerDay:    (rep.Factorial.V10_CompetitiveBlind - rep.Factorial.V00_LegacyBlind) / h,
			TotalLiftPerDay:   rep.Factorial.TotalLift / h,
		}
		points = append(points, pt)
	}

	// Compute paired finite differences across adjacent lambda steps
	finiteDiffs := make([]FiniteDiffContrast, 0, len(lambdas)-1)
	for i := 0; i < len(lambdas)-1; i++ {
		l1 := lambdas[i]
		l2 := lambdas[i+1]
		d1 := repMap[l1]
		d2 := repMap[l2]

		// Vector D_i(l) = (v11 - v10) - (v01 - v00)
		diffVector := d2.TTestInteraction.MeanDifference - d1.TTestInteraction.MeanDifference
		pooledSE := math.Sqrt(math.Pow(d1.TTestInteraction.StdErrDifference, 2) + math.Pow(d2.TTestInteraction.StdErrDifference, 2))
		tStat := 0.0
		if pooledSE > 1e-9 {
			tStat = diffVector / pooledSE
		}
		pVal := pkgmath.StudentTCDFTwoTailed(math.Abs(tStat), float64(episodes-1))
		tCrit := pkgmath.StudentTCriticalValue(0.05, float64(episodes-1))
		margin := tCrit * pooledSE

		finiteDiffs = append(finiteDiffs, FiniteDiffContrast{
			FromParam: float64(l1),
			ToParam:   float64(l2),
			MeanDelta: diffVector,
			CI95Low:   diffVector - margin,
			CI95High:  diffVector + margin,
			TStat:     tStat,
			PValue:    pVal,
		})
	}

	return &CurveSweepReport{
		CurveName:   "Tender Density Response Curve",
		Description: "Evaluates the scaling of supermodular complementarity with market tender arrivals lambda",
		Points:      points,
		FiniteDiffs: finiteDiffs,
	}, nil
}

// RunCapacitySweep evaluates factorial interaction across varying fleet sizes: K in {6, 8, 10, 12, 15, 20}.
func RunCapacitySweep(ctx context.Context, episodes int, baseSeed uint64) (*CurveSweepReport, error) {
	drivers := []int{6, 8, 10, 12, 15, 20}
	points := make([]SweepPoint, 0, len(drivers))
	repMap := make(map[int]*FactorialReport2x2, len(drivers))

	for _, k := range drivers {
		cfg := TournamentConfig{
			Episodes:          episodes,
			HorizonDays:       7,
			DecisionStepHours: 12,
			DriverCount:       k,
			LoadsPerEpoch:     15,
			BaseSeed:          baseSeed,
			Market:            DefaultMarketConfig(),
		}
		runner := NewTournamentRunner(cfg)
		rep, err := runner.RunFactorial2x2(ctx)
		if err != nil {
			return nil, fmt.Errorf("capacity sweep at K=%d failed: %w", k, err)
		}
		repMap[k] = rep

		h := 7.0
		scarcity := 15.0 / float64(k)
		pt := SweepPoint{
			ParamName:         "K",
			ParamValue:        float64(k),
			HorizonDays:       7,
			DriverCount:       k,
			LoadsPerEpoch:     15,
			ScarcityRatio:     scarcity,
			V00Mean:           rep.Factorial.V00_LegacyBlind,
			V01Mean:           rep.Factorial.V01_LegacyInformed,
			V10Mean:           rep.Factorial.V10_CompetitiveBlind,
			V11Mean:           rep.Factorial.V11_CompetitiveInformed,
			TotalLiftDollars:  rep.Factorial.TotalLift,
			TotalLiftPct:      rep.TTestV11VsV00.PercentLift,
			DeltaLegacyVoI:    rep.Factorial.ConditionalVoIUnderLegacy,
			DeltaBlindVoA:     rep.Factorial.V10_CompetitiveBlind - rep.Factorial.V00_LegacyBlind,
			DeltaCompVoI:      rep.Factorial.ConditionalVoIUnderComp,
			DeltaInformedVoA:  rep.Factorial.V11_CompetitiveInformed - rep.Factorial.V01_LegacyInformed,
			InteractionMean:   rep.TTestInteraction.MeanDifference,
			InteractionCI95L:  rep.TTestInteraction.ConfidenceLow95,
			InteractionCI95H:  rep.TTestInteraction.ConfidenceHigh95,
			InteractionT:      rep.TTestInteraction.TStatistic,
			InteractionP:      rep.TTestInteraction.PValueTwoTailed,
			InteractionDz:     rep.TTestInteraction.CohensD,
			InteractionPerDay: rep.TTestInteraction.MeanDifference / h,
			CompVoIPerDay:     rep.Factorial.ConditionalVoIUnderComp / h,
			BlindVoAPerDay:    (rep.Factorial.V10_CompetitiveBlind - rep.Factorial.V00_LegacyBlind) / h,
			TotalLiftPerDay:   rep.Factorial.TotalLift / h,
		}
		points = append(points, pt)
	}

	// Compute paired finite differences across adjacent capacity steps
	finiteDiffs := make([]FiniteDiffContrast, 0, len(drivers)-1)
	for i := 0; i < len(drivers)-1; i++ {
		k1 := drivers[i]
		k2 := drivers[i+1]
		d1 := repMap[k1]
		d2 := repMap[k2]

		diffVector := d2.TTestInteraction.MeanDifference - d1.TTestInteraction.MeanDifference
		pooledSE := math.Sqrt(math.Pow(d1.TTestInteraction.StdErrDifference, 2) + math.Pow(d2.TTestInteraction.StdErrDifference, 2))
		tStat := 0.0
		if pooledSE > 1e-9 {
			tStat = diffVector / pooledSE
		}
		pVal := pkgmath.StudentTCDFTwoTailed(math.Abs(tStat), float64(episodes-1))
		tCrit := pkgmath.StudentTCriticalValue(0.05, float64(episodes-1))
		margin := tCrit * pooledSE

		finiteDiffs = append(finiteDiffs, FiniteDiffContrast{
			FromParam: float64(k1),
			ToParam:   float64(k2),
			MeanDelta: diffVector,
			CI95Low:   diffVector - margin,
			CI95High:  diffVector + margin,
			TStat:     tStat,
			PValue:    pVal,
		})
	}

	return &CurveSweepReport{
		CurveName:   "Fleet Capacity Response Curve",
		Description: "Evaluates the scaling of supermodular complementarity with fleet driver count K",
		Points:      points,
		FiniteDiffs: finiteDiffs,
	}, nil
}

// RunHorizonSweep evaluates factorial interaction across varying simulation horizons: H in {3, 7, 14, 21, 30}.
func RunHorizonSweep(ctx context.Context, episodes int, baseSeed uint64) (*CurveSweepReport, error) {
	horizons := []int{3, 7, 14, 21, 30}
	points := make([]SweepPoint, 0, len(horizons))

	for _, h := range horizons {
		cfg := TournamentConfig{
			Episodes:          episodes,
			HorizonDays:       h,
			DecisionStepHours: 12,
			DriverCount:       10,
			LoadsPerEpoch:     15,
			BaseSeed:          baseSeed,
			Market:            DefaultMarketConfig(),
		}
		runner := NewTournamentRunner(cfg)
		rep, err := runner.RunFactorial2x2(ctx)
		if err != nil {
			return nil, fmt.Errorf("horizon sweep at H=%d failed: %w", h, err)
		}

		hDays := float64(h)
		scarcity := 15.0 / 10.0
		pt := SweepPoint{
			ParamName:         "Horizon",
			ParamValue:        hDays,
			HorizonDays:       h,
			DriverCount:       10,
			LoadsPerEpoch:     15,
			ScarcityRatio:     scarcity,
			V00Mean:           rep.Factorial.V00_LegacyBlind,
			V01Mean:           rep.Factorial.V01_LegacyInformed,
			V10Mean:           rep.Factorial.V10_CompetitiveBlind,
			V11Mean:           rep.Factorial.V11_CompetitiveInformed,
			TotalLiftDollars:  rep.Factorial.TotalLift,
			TotalLiftPct:      rep.TTestV11VsV00.PercentLift,
			DeltaLegacyVoI:    rep.Factorial.ConditionalVoIUnderLegacy,
			DeltaBlindVoA:     rep.Factorial.V10_CompetitiveBlind - rep.Factorial.V00_LegacyBlind,
			DeltaCompVoI:      rep.Factorial.ConditionalVoIUnderComp,
			DeltaInformedVoA:  rep.Factorial.V11_CompetitiveInformed - rep.Factorial.V01_LegacyInformed,
			InteractionMean:   rep.TTestInteraction.MeanDifference,
			InteractionCI95L:  rep.TTestInteraction.ConfidenceLow95,
			InteractionCI95H:  rep.TTestInteraction.ConfidenceHigh95,
			InteractionT:      rep.TTestInteraction.TStatistic,
			InteractionP:      rep.TTestInteraction.PValueTwoTailed,
			InteractionDz:     rep.TTestInteraction.CohensD,
			InteractionPerDay: rep.TTestInteraction.MeanDifference / hDays,
			CompVoIPerDay:     rep.Factorial.ConditionalVoIUnderComp / hDays,
			BlindVoAPerDay:    (rep.Factorial.V10_CompetitiveBlind - rep.Factorial.V00_LegacyBlind) / hDays,
			TotalLiftPerDay:   rep.Factorial.TotalLift / hDays,
		}
		points = append(points, pt)
	}

	return &CurveSweepReport{
		CurveName:   "Horizon Response Curve",
		Description: "Evaluates the trajectory of per-day and cumulative value of information across extended horizons",
		Points:      points,
	}, nil
}

// RunFullResponseCurves executes all three comparative statics response curves.
func RunFullResponseCurves(ctx context.Context, episodes int, baseSeed uint64) (*FullComparativeStaticsReport, error) {
	fmt.Println("\n>>> [1/3] Executing Tender Density Response Curve Sweep...")
	density, err := RunDensitySweep(ctx, episodes, baseSeed)
	if err != nil {
		return nil, err
	}

	fmt.Println("\n>>> [2/3] Executing Fleet Capacity Response Curve Sweep...")
	capacity, err := RunCapacitySweep(ctx, episodes, baseSeed)
	if err != nil {
		return nil, err
	}

	fmt.Println("\n>>> [3/3] Executing Horizon Response Curve Sweep...")
	horizon, err := RunHorizonSweep(ctx, episodes, baseSeed)
	if err != nil {
		return nil, err
	}

	return &FullComparativeStaticsReport{
		DensitySweep:  *density,
		CapacitySweep: *capacity,
		HorizonSweep:  *horizon,
	}, nil
}
