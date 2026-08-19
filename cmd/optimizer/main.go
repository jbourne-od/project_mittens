// Package main provides the static binary entrypoint for the Project Mittens optimizer.
//
// In accordance with Inviolate 0 (Explicit Configuration) and Inviolate 4 (Closed Business Logic),
// all parameters, dependencies, and execution contexts are injected explicitly at runtime.
// Package-level init() functions and ambient environment variable discovery are strictly prohibited.
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

	"github.com/optimaldynamics/project-mittens/internal/adapter/legacy"
	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/internal/service"
	"github.com/optimaldynamics/project-mittens/internal/service/dispatch"
	"github.com/optimaldynamics/project-mittens/pkg/logging"
)

func main() {
	scenarioDir := flag.String("scenario", "", "Path to carrier scenario input directory (containing locations.txt, drivers.txt, loads.txt)")
	policyName := flag.String("policy", "cfa", "Optimization policy: cfa, vfa, piecewise-vfa, dla")
	horizonDays := flag.Int("days", 7, "Simulation horizon in days")
	stepHours := flag.Int("step-hours", 24, "Decision batch step in hours")
	enableRelays := flag.Bool("relays", true, "Enable multi-driver relay exchange synthesis")
	outputJSON := flag.String("output-json", "", "Optional path to export JSON execution summary")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	flag.Parse()

	// 1. Initialize structured logging explicitly (Inviolate 0)
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

	runID := fmt.Sprintf("OPT_RUN_%d", time.Now().Unix())
	ctx := logging.WithContextData(context.Background(), logging.ContextData{
		OptimizationRunID: runID,
		PolicyClass:       *policyName,
	})

	logger.InfoContext(ctx, "Project Mittens Optimization Engine Starting",
		slog.String("run_id", runID),
		slog.String("policy", *policyName),
		slog.Int("horizon_days", *horizonDays),
		slog.Bool("enable_relays", *enableRelays),
	)

	startEpoch := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	// 2. Load or construct fleet scenario
	var drivers []model.Driver
	var loads []model.Load

	if *scenarioDir != "" {
		locFile := filepath.Join(*scenarioDir, "locations.txt")
		driverFile := filepath.Join(*scenarioDir, "drivers.txt")
		loadFile := filepath.Join(*scenarioDir, "loads.txt")

		dList, lList, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 0)
		if err != nil {
			logger.ErrorContext(ctx, "failed loading carrier scenario", slog.String("error", err.Error()))
			os.Exit(1)
		}
		drivers = dList
		loads = lList
		logger.InfoContext(ctx, "loaded external carrier scenario",
			slog.Int("drivers", len(drivers)),
			slog.Int("loads", len(loads)),
		)
	} else {
		// Default synthetic enterprise scenario
		drivers, loads = generateSyntheticScenario(startEpoch)
		logger.InfoContext(ctx, "initialized synthetic carrier network",
			slog.Int("drivers", len(drivers)),
			slog.Int("loads", len(loads)),
		)
	}

	// Normalize initial available epochs
	for i := range drivers {
		drivers[i].AvailableEpoch = startEpoch
		if drivers[i].Clocks == nil {
			drivers[i].Clocks = hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0))
		}
	}

	// Group loads into dynamic stream across horizon
	loadStreamMap := make(map[int64][]model.Load)
	loadsPerDay := len(loads) / *horizonDays
	if loadsPerDay == 0 {
		loadsPerDay = len(loads)
	}

	for day := 0; day < *horizonDays; day++ {
		epoch := startEpoch + int64(day*86400)
		startIdx := day * loadsPerDay
		endIdx := startIdx + loadsPerDay
		if endIdx > len(loads) || day == *horizonDays-1 {
			endIdx = len(loads)
		}
		if startIdx < len(loads) {
			dayLoads := make([]model.Load, endIdx-startIdx)
			copy(dayLoads, loads[startIdx:endIdx])
			for i := range dayLoads {
				dayLoads[i].PickupEarliestEpoch = epoch
				dayLoads[i].PickupLatestEpoch = epoch + 36000
				dayLoads[i].DeliveryLatestEpoch = epoch + 120000
			}
			loadStreamMap[epoch] = dayLoads
		}
	}
	stream := service.NewStaticLoadStream(loadStreamMap)

	res := model.NewResourceState(drivers, nil)
	info, err := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	if err != nil {
		logger.ErrorContext(ctx, "failed creating information state", slog.String("error", err.Error()))
		os.Exit(1)
	}
	belief := model.NewMonopolisticBelief()
	state, err := model.NewState(res, info, belief)
	if err != nil {
		logger.ErrorContext(ctx, "failed creating initial state", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// 3. Build facility store
	facilities := []model.Facility{
		{
			ID:                  "FAC-SDF-HUB",
			Name:                "Louisville Relay Hub",
			Location:            model.Location{NodeID: "SDF", Lat: 38.2527, Lon: -85.7585},
			Type:                model.FacilityRelayHub,
			OpenMinutesOfDay:    0,
			CloseMinutesOfDay:   1440,
			AverageDwellMinutes: 60,
		},
		{
			ID:                  "FAC-IND-HUB",
			Name:                "Indianapolis Transfer Terminal",
			Location:            model.Location{NodeID: "IND", Lat: 39.7684, Lon: -86.1581},
			Type:                model.FacilityTerminal,
			OpenMinutesOfDay:    0,
			CloseMinutesOfDay:   1440,
			AverageDwellMinutes: 60,
		},
	}
	facStore := model.NewFacilityStore(facilities)

	// 4. Construct selected Powell Policy
	costCfg := model.DefaultCostConfig()
	feasCfg := model.DefaultFeasibilityConfig()
	rm := model.NewRegionManager(1.0, nil)

	var selectedPolicy policy.Policy[model.Monopolistic]
	switch *policyName {
	case "vfa":
		vfaTable := policy.NewVFATable(map[string]float64{
			"CHI": 200.0,
			"ATL": 250.0,
			"DAL": 300.0,
		})
		selectedPolicy = policy.NewVFAPolicy[model.Monopolistic](vfaTable, 0.95, costCfg, feasCfg, rm)
	case "piecewise-vfa":
		slopes := map[string]policy.RegionSlopes{
			"CHI": {RegionID: "CHI", Slopes: []float64{500.0, 300.0, 100.0, 20.0}},
			"ATL": {RegionID: "ATL", Slopes: []float64{600.0, 350.0, 150.0, 50.0}},
		}
		pvfaTable := policy.NewPiecewiseLinearVFATable(slopes)
		selectedPolicy = policy.NewPiecewiseVFAPolicy[model.Monopolistic](pvfaTable, nil, 0.95, costCfg, feasCfg, rm)
	case "dla":
		dlaParams := policy.DefaultDLAParameters()
		dlaParams.Horizon = 2
		dlaParams.NumRollouts = 4
		basePol := policy.NewCFAPolicy[model.Monopolistic](policy.DefaultCFAParameters(), costCfg, feasCfg, nil)
		selectedPolicy = policy.NewDLAPolicy[model.Monopolistic](dlaParams, costCfg, feasCfg, basePol, nil, rm, nil, logger)
	default: // "cfa"
		selectedPolicy = policy.NewCFAPolicy[model.Monopolistic](
			policy.DefaultCFAParameters(),
			costCfg,
			feasCfg,
			nil,
		)
	}

	// 5. Construct Relay Dispatch Runner
	var relayRunner *dispatch.RelayDispatchRunner
	if *enableRelays {
		relaySynth := policy.NewRelaySynthesizer(
			costCfg,
			policy.DefaultRelayConfig(),
			hos.USPolicySpecs(),
			facStore,
			logger,
		)
		relayRunner = dispatch.NewRelayDispatchRunner(nil, relaySynth)
	}

	// 6. Execute Multi-Day Rolling Horizon Simulation
	simRunner := service.NewRollingHorizonRunner[model.Monopolistic](
		nil,
		relayRunner,
		nil,
		logger,
	)

	simCfg := service.RollingHorizonConfig{
		RunID:             runID,
		StartEpoch:        startEpoch,
		HorizonDays:       *horizonDays,
		DecisionStepHours: *stepHours,
		EnableRelays:      *enableRelays,
		MinRelayHaulMiles: 450.0,
		EnableVFALearning: true,
	}

	report, _, err := simRunner.Run(ctx, simCfg, state, selectedPolicy, stream)
	if err != nil {
		logger.ErrorContext(ctx, "simulation execution failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// 7. Output Summary & Optional JSON Export
	fmt.Printf("\n=======================================================\n")
	fmt.Printf("           PROJECT MITTENS OPTIMIZATION REPORT          \n")
	fmt.Printf("=======================================================\n")
	fmt.Printf("  Run ID:                %s\n", report.RunID)
	fmt.Printf("  Policy:                %s\n", selectedPolicy.Name())
	fmt.Printf("  Horizon Days:          %d (Epochs: %d)\n", report.TotalDays, report.TotalEpochs)
	fmt.Printf("  Total Direct Tours:    %d\n", report.TotalDirectTours)
	fmt.Printf("  Total Relay Exchanges: %d\n", report.TotalRelayExchanges)
	fmt.Printf("  Loaded Miles:          %.1f\n", report.TotalLoadedMiles)
	fmt.Printf("  Empty Miles:           %.1f (%.1f%%)\n", report.TotalEmptyMiles, report.GlobalEmptyRatio*100.0)
	fmt.Printf("  Gross Revenue:         $%.2f\n", report.TotalGrossRevenue)
	fmt.Printf("  Operating Cost:        $%.2f\n", report.TotalOperatingCost)
	fmt.Printf("  Net Contribution:      $%.2f\n", report.TotalNetContribution)
	fmt.Printf("=======================================================\n\n")

	if *outputJSON != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err == nil {
			if writeErr := os.WriteFile(*outputJSON, data, 0644); writeErr == nil {
				logger.InfoContext(ctx, "exported JSON report", slog.String("path", *outputJSON))
			}
		}
	}
}

func generateSyntheticScenario(startEpoch int64) ([]model.Driver, []model.Load) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locBna := model.Location{NodeID: "BNA", Lat: 36.1627, Lon: -86.7816}
	locDal := model.Location{NodeID: "DAL", Lat: 32.7767, Lon: -96.7970}
	locMke := model.Location{NodeID: "MKE", Lat: 43.0389, Lon: -87.9065}

	drivers := []model.Driver{
		{ID: "D-CHI-01", CurrentLocation: locChi, HomeLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
		{ID: "D-CHI-02", CurrentLocation: locChi, HomeLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
		{ID: "D-BNA-01", CurrentLocation: locBna, HomeLocation: locBna, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
		{ID: "D-ATL-01", CurrentLocation: locAtl, HomeLocation: locAtl, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
		{ID: "D-DAL-01", CurrentLocation: locDal, HomeLocation: locDal, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
	}

	loads := []model.Load{
		{ID: "L-01", Origin: locChi, Destination: locAtl, RequiredEquipment: model.EquipDryVan, Revenue: 3600.0},
		{ID: "L-02", Origin: locChi, Destination: locMke, RequiredEquipment: model.EquipDryVan, Revenue: 1100.0},
		{ID: "L-03", Origin: locAtl, Destination: locDal, RequiredEquipment: model.EquipDryVan, Revenue: 3200.0},
		{ID: "L-04", Origin: locDal, Destination: locChi, RequiredEquipment: model.EquipDryVan, Revenue: 3900.0},
		{ID: "L-05", Origin: locBna, Destination: locAtl, RequiredEquipment: model.EquipDryVan, Revenue: 1200.0},
		{ID: "L-06", Origin: locChi, Destination: locAtl, RequiredEquipment: model.EquipDryVan, Revenue: 3700.0},
		{ID: "L-07", Origin: locMke, Destination: locChi, RequiredEquipment: model.EquipDryVan, Revenue: 1050.0},
	}

	return drivers, loads
}
