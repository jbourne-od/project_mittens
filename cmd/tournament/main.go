// Package main provides the static binary entrypoint for the Project Mittens Tournament Runner.
//
// It executes paired twin-rollout Monte Carlo tournaments comparing the naive monopolistic N=0 baseline
// against the competitive MOMDP N=1 belief-filtered policy, prints rigorous statistical scorecards,
// and exports structured JSON records for Python visual analysis.
//
// In accordance with Inviolate 0 (Explicit Configuration) and Inviolate 4 (Closed Business Logic),
// all parameters, random seeds, and transition models are injected explicitly at runtime.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/adapter/simulation"
	"github.com/optimaldynamics/project-mittens/pkg/logging"
)

func main() {
	episodes := flag.Int("episodes", 25, "Number of paired simulation episodes")
	horizonDays := flag.Int("days", 7, "Simulation horizon per episode in days")
	stepHours := flag.Int("step-hours", 12, "Decision epoch step in hours")
	drivers := flag.Int("drivers", 10, "Fleet driver count per episode")
	loadsPerEpoch := flag.Int("loads-per-epoch", 15, "Synthetic market load arrivals per decision epoch")
	baseSeed := flag.Uint64("seed", 20260819, "Base pseudo-random seed for deterministic reproduction")
	outputJSON := flag.String("output-json", "tournament_results.json", "Path to export structured tournament results JSON")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	flag.Parse()

	// 1. Initialize structured logger (Inviolate 0)
	var lvl logging.Level
	switch *logLevel {
	case "debug":
		lvl = logging.LevelDebug
	case "warn":
		lvl = logging.LevelWarn
	case "error":
		lvl = logging.LevelError
	default:
		lvl = logging.LevelInfo
	}

	logger := logging.New(logging.Config{
		Level:  lvl,
		Format: logging.FormatText,
	})
	slog.SetDefault(logger)

	runID := fmt.Sprintf("TOURNAMENT_%d", time.Now().Unix())
	ctx := logging.WithContextData(context.Background(), logging.ContextData{
		OptimizationRunID: runID,
		PolicyClass:       "TOURNAMENT_N0_VS_N1",
	})

	fmt.Println("================================================================================")
	fmt.Println("             PROJECT MITTENS: COMPETITIVE TOURNAMENT RUNNER                     ")
	fmt.Println("================================================================================")
	fmt.Printf(" Configuration:\n")
	fmt.Printf("   Episodes:           %d\n", *episodes)
	fmt.Printf("   Horizon:            %d days (%d hours/step -> %d steps/episode)\n", *horizonDays, *stepHours, (*horizonDays*24)/(*stepHours))
	fmt.Printf("   Fleet Capacity:     %d drivers\n", *drivers)
	fmt.Printf("   Market Flow:        %d loads/epoch\n", *loadsPerEpoch)
	fmt.Printf("   Base Seed:          %d\n", *baseSeed)
	fmt.Printf("   Output JSON:        %s\n", *outputJSON)
	fmt.Println("================================================================================")
	fmt.Println(" Executing Monte Carlo paired episodes...")

	cfg := simulation.TournamentConfig{
		Episodes:          *episodes,
		HorizonDays:       *horizonDays,
		DecisionStepHours: *stepHours,
		DriverCount:       *drivers,
		LoadsPerEpoch:     *loadsPerEpoch,
		BaseSeed:          *baseSeed,
		Market:            simulation.DefaultMarketConfig(),
	}

	runner := simulation.NewTournamentRunner(cfg)
	report, err := runner.Run(ctx)
	if err != nil {
		slog.Error("Tournament execution failed", "error", err)
		os.Exit(1)
	}

	// Print Formatted Scorecard Table
	fmt.Println("\n================================================================================")
	fmt.Println("                      EPISODE-BY-EPISODE COMPARISON                             ")
	fmt.Println("================================================================================")
	fmt.Printf(" %-4s | %-12s | %-12s | %-12s | %-12s | %-10s\n", "Ep", "N=0 Net ($)", "N=1 Net ($)", "Delta ($)", "Lift (%)", "N1 WinRate")
	fmt.Println("--------------------------------------------------------------------------------")
	for _, ep := range report.Episodes {
		fmt.Printf(" %-4d | $%11.2f | $%11.2f | $%+11.2f | %+11.2f%% | %9.2f%%\n",
			ep.EpisodeIndex,
			ep.N0_NetContribution,
			ep.N1_NetContribution,
			ep.NetContributionDelta,
			ep.NetContributionLiftPercent,
			ep.N1_WinRate*100.0,
		)
	}

	fmt.Println("================================================================================")
	fmt.Println("                    STATISTICAL HYPOTHESIS TEST RESULTS                         ")
	fmt.Println("================================================================================")
	fmt.Println(report.TTest.SummaryString())
	fmt.Println("================================================================================")
	fmt.Printf(" Mean N=0 Contribution:    $%.2f\n", report.TTest.MeanBaseline)
	fmt.Printf(" Mean N=1 Contribution:    $%.2f\n", report.TTest.MeanCandidate)
	fmt.Printf(" Mean Profit Lift:         +$%.2f (+%.2f%%)\n", report.TTest.MeanDifference, report.TTest.PercentLift)
	fmt.Printf(" 95%% Confidence Interval:   [$%.2f, $%.2f]\n", report.TTest.ConfidenceLow95, report.TTest.ConfidenceHigh95)
	fmt.Printf(" Student's t-Statistic:     %.4f (df=%.0f)\n", report.TTest.TStatistic, report.TTest.DegreesOfFreedom)
	fmt.Printf(" One-Tailed p-Value:        %e\n", report.TTest.PValueOneTailed)
	fmt.Printf(" Superiority Verified:      %t\n", report.N1SuperiorityVerified)
	fmt.Printf(" Execution Wall-Clock Time: %.2f seconds\n", report.ExecutionDurationSec)
	fmt.Println("================================================================================")

	// Export structured JSON
	if *outputJSON != "" {
		if dir := filepath.Dir(*outputJSON); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0755)
		}
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			slog.Error("Failed marshaling JSON report", "error", err)
			os.Exit(1)
		}
		if err := os.WriteFile(*outputJSON, data, 0644); err != nil {
			slog.Error("Failed writing output JSON file", "path", *outputJSON, "error", err)
			os.Exit(1)
		}
		fmt.Printf("\n[SUCCESS] Exported structured tournament report to %s\n", *outputJSON)
	}
}
