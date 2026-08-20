package legacy_test

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/adapter/legacy"
	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

// getCoreAIRoot locates the legacy Java coreai test fixtures root directory.
func getCoreAIRoot(t *testing.T) string {
	t.Helper()
	candidates := []string{
		os.Getenv("COREAI_ROOT"),
		os.Getenv("COREAI_DATA_DIR"),
		"/Users/jacob/Development/od/coreai",
		"../../../../coreai",
	}
	for _, dir := range candidates {
		if dir != "" {
			if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
				return dir
			}
		}
	}
	t.Skip("coreai golden test fixtures directory not accessible or not found; skipping legacy golden parity test")
	return ""
}

// TestGoldenParity_07TestDispatch validates end-to-end matching parity against Java's 07_test_dispatch fixture.
func TestGoldenParity_07TestDispatch(t *testing.T) {
	coreaiRoot := getCoreAIRoot(t)
	scenarioDir := filepath.Join(coreaiRoot, "engine/smart_tl/worker/tests/07_test_dispatch")
	inputDir := filepath.Join(scenarioDir, "input")
	expectedDriversFile := filepath.Join(scenarioDir, "expected/test_rankedListsForDrivers.txt")

	locFile := filepath.Join(inputDir, "locations.txt")
	driverFile := filepath.Join(inputDir, "drivers.txt")
	loadFile := filepath.Join(inputDir, "loads.txt")

	// 1. Load inputs
	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 0)
	if err != nil {
		t.Fatalf("LoadCarrierScenario failed: %v", err)
	}

	t.Logf("Loaded 07_test_dispatch: %d drivers, %d loads", len(drivers), len(loads))

	// 2. Parse Golden Expected Drivers
	goldenMoves, err := legacy.ParseGoldenRankedDriversFile(expectedDriversFile)
	if err != nil {
		t.Fatalf("ParseGoldenRankedDriversFile failed: %v", err)
	}

	// Filter Golden Rank 1 Assignments
	goldenRank1Map := make(map[string]legacy.GoldenDriverMove)
	for _, gm := range goldenMoves {
		if gm.Rank1 == 1 && gm.Rank2 == 1 {
			goldenRank1Map[gm.DriverID] = gm
		}
	}

	t.Logf("Golden Rank 1 Assignments: %d drivers", len(goldenRank1Map))
	for dID, gm := range goldenRank1Map {
		t.Logf("  Golden Driver %s: Type=%s, LoadID=%s, Empty=%.1f, Loaded=%.1f, Rev=$%.2f",
			dID, gm.MovementType, gm.LoadID, gm.EmptyMiles, gm.LoadedMiles, gm.Revenue)
	}

	// 3. Configure Go Optimizer (Monopolistic N=0 at epoch 2022-05-11 02:00:00 UTC)
	baseEpoch := time.Date(2022, 5, 11, 2, 0, 0, 0, time.UTC).Unix()

	for i := range drivers {
		drivers[i].CurrentLocation = drivers[i].HomeLocation
		drivers[i].AvailableEpoch = baseEpoch
		drivers[i].Clocks = hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(baseEpoch, 0))
	}

	res := model.NewResourceState(drivers, loads)
	info, err := model.NewInformationState(baseEpoch, 1.0, 2.50, 0)
	if err != nil {
		t.Fatalf("NewInformationState failed: %v", err)
	}
	belief := model.NewMonopolisticBelief()
	state, err := model.NewState(res, info, belief)
	if err != nil {
		t.Fatalf("NewState failed: %v", err)
	}

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	// 4. Evaluate Go Matching
	action, prov, err := cfaPol.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("Policy Evaluate failed: %v", err)
	}

	t.Logf("Go Optimizer Result: %d matches, Total Net Contribution: $%.2f",
		action.MatchCount(), prov.TotalNetContribution)

	goMatchMap := make(map[string]string)
	for _, m := range action.Matches() {
		goMatchMap[m.DriverID] = m.LoadID
		t.Logf("  Go Matched: Driver %s -> Load %s (Contrib: $%.2f)",
			m.DriverID, m.LoadID, m.EstimatedContribution)
	}

	// 5. Parity Assertions
	// In Java 07_test_dispatch:
	// - SONRW2 is assigned to 270391 (closest driver, 32.5 empty miles)
	// - QUIRC4 is assigned to 270392 (65.0 empty miles)
	// - SMIVA2 remains unassigned (Hold)
	if len(action.Matches()) != 2 {
		t.Errorf("expected exactly 2 matches, got %d", len(action.Matches()))
	}

	for dID, gm := range goldenRank1Map {
		if gm.IsLoaded {
			goLoadID, matched := goMatchMap[dID]
			if !matched {
				t.Errorf("Driver %s was assigned to load %s in golden, but unassigned in Go", dID, gm.LoadID)
			} else if goLoadID != gm.LoadID {
				t.Errorf("Driver %s matched load %s in Go; expected golden assignment %s", dID, goLoadID, gm.LoadID)
			}
		}
	}
}

// TestGoldenCharacterization_KBX tests 6,904 operational carrier driver records from KBX.
func TestGoldenCharacterization_KBX(t *testing.T) {
	coreaiRoot := getCoreAIRoot(t)
	goldenFile := filepath.Join(coreaiRoot, "engine/smart_tl/worker/src/test/resources/fleetmanager/dispatching/kbx_rankedListsForDrivers.txt")

	records, err := legacy.ParseGoldenRankedDriversFile(goldenFile)
	if err != nil {
		t.Fatalf("Failed to parse KBX golden file: %v", err)
	}

	t.Logf("Parsed %d KBX golden moves across all ranks", len(records))

	sim := hos.NewSimulator()
	specs := hos.USPolicySpecs()
	validMoves := 0
	loadedMoves := 0
	holdMoves := 0
	homeMoves := 0
	totalLoadedMiles := 0.0
	totalEmptyMiles := 0.0

	for _, rec := range records {
		if rec.IsLoaded {
			loadedMoves++
			totalLoadedMiles += rec.LoadedMiles
			totalEmptyMiles += rec.EmptyMiles
		} else if rec.IsHold {
			holdMoves++
		} else if rec.IsEmptyToHome {
			homeMoves++
			totalEmptyMiles += rec.EmptyMiles
		}

		// Validate that loaded miles are non-negative and finite (IEEE 754 checks)
		if math.IsNaN(rec.LoadedMiles) || math.IsInf(rec.LoadedMiles, 0) || rec.LoadedMiles < 0 {
			t.Errorf("Driver %s has invalid loaded miles: %f", rec.DriverID, rec.LoadedMiles)
		}
		if math.IsNaN(rec.EmptyMiles) || math.IsInf(rec.EmptyMiles, 0) || rec.EmptyMiles < 0 {
			t.Errorf("Driver %s has invalid empty miles: %f", rec.DriverID, rec.EmptyMiles)
		}

		// Characterize HOS simulation integrity
		if rec.IsLoaded && rec.LoadedMiles > 0 {
			now := time.Date(2026, 1, 9, 7, 0, 0, 0, time.UTC)
			clocks := hos.NewDriverClocks(specs, now)

			tripRes, err := sim.EvaluateTripFeasibility(
				clocks,
				rec.EmptyMiles,
				rec.LoadedMiles,
				60,
				60,
				50.0,
				now,
				now.Add(24*time.Hour),
				now,
				now.Add(48*time.Hour),
				specs,
			)

			if err == nil && tripRes.IsFeasible {
				validMoves++
			}
		}
	}

	t.Logf("KBX Characterization Summary:")
	t.Logf("  Total Records: %d", len(records))
	t.Logf("  Loaded Hauls: %d (Total Miles: %.1f, Empty Miles: %.1f)", loadedMoves, totalLoadedMiles, totalEmptyMiles)
	t.Logf("  Driver Holds: %d, Empty-To-Home: %d", holdMoves, homeMoves)
	t.Logf("  Go HOS Feasibility Validated: %d moves", validMoves)

	if len(records) < 6000 {
		t.Errorf("expected >6000 records, got %d", len(records))
	}
}

// TestGoldenCharacterization_CentralOregonTruck tests 570 specialized flatbed driver records.
func TestGoldenCharacterization_CentralOregonTruck(t *testing.T) {
	coreaiRoot := getCoreAIRoot(t)
	goldenFile := filepath.Join(coreaiRoot, "engine/smart_tl/worker/src/test/resources/fleetmanager/dispatching/centraloregontruck_rankedListsForDrivers.txt")

	records, err := legacy.ParseGoldenRankedDriversFile(goldenFile)
	if err != nil {
		t.Fatalf("Failed to parse Central Oregon Truck golden file: %v", err)
	}

	t.Logf("Parsed %d Central Oregon Truck golden moves", len(records))

	loadedCount := 0
	homeCount := 0
	holdCount := 0

	for _, rec := range records {
		if rec.IsLoaded {
			loadedCount++
			if math.IsNaN(rec.LoadedMiles) || math.IsInf(rec.LoadedMiles, 0) || rec.LoadedMiles < 0 {
				t.Errorf("Driver %s has invalid loaded miles: %f", rec.DriverID, rec.LoadedMiles)
			}
		} else if rec.IsEmptyToHome {
			homeCount++
		} else if rec.IsHold {
			holdCount++
		}
	}

	t.Logf("Central Oregon Truck Breakdown: Loaded=%d, Holds=%d, Home=%d",
		loadedCount, holdCount, homeCount)

	if len(records) != 570 {
		t.Errorf("expected 570 records, got %d", len(records))
	}
}

// TestGoldenParity_16OptimalTours validates multi-leg optimal tours fixture parity and Go optimizer execution.
func TestGoldenParity_16OptimalTours(t *testing.T) {
	coreaiRoot := getCoreAIRoot(t)
	scenarioDir := filepath.Join(coreaiRoot, "engine/smart_tl/worker/tests/16_test_dispatch_optimal_tours")
	inputDir := filepath.Join(scenarioDir, "input")
	expectedDriversFile := filepath.Join(scenarioDir, "expected/test_rankedListsForDrivers.txt")

	locFile := filepath.Join(inputDir, "locations_out.txt")
	driverFile := filepath.Join(inputDir, "drivers.txt")
	loadFile := filepath.Join(inputDir, "loads.txt")

	// 1. Load inputs and execute Go Optimizer
	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 0)
	if err != nil {
		t.Fatalf("LoadCarrierScenario failed: %v", err)
	}

	t.Logf("Loaded 16_test_dispatch_optimal_tours inputs: %d drivers, %d loads", len(drivers), len(loads))

	baseEpoch := time.Date(2022, 8, 16, 12, 0, 0, 0, time.UTC).Unix()
	for i := range drivers {
		if drivers[i].AvailableEpoch == 0 {
			drivers[i].AvailableEpoch = baseEpoch
		}
		if drivers[i].Clocks == nil {
			drivers[i].Clocks = hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(drivers[i].AvailableEpoch, 0))
		}
	}

	res := model.NewResourceState(drivers, loads)
	info, err := model.NewInformationState(baseEpoch, 1.0, 2.50, 0)
	if err != nil {
		t.Fatalf("NewInformationState failed: %v", err)
	}
	belief := model.NewMonopolisticBelief()
	state, err := model.NewState(res, info, belief)
	if err != nil {
		t.Fatalf("NewState failed: %v", err)
	}

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	action, prov, err := cfaPol.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("Go Policy Evaluate failed on 16_test_dispatch_optimal_tours: %v", err)
	}

	t.Logf("Go Optimizer Result on 16_test_dispatch_optimal_tours: %d matches, Net Contribution: $%.2f",
		action.MatchCount(), prov.TotalNetContribution)

	if action.MatchCount() == 0 {
		t.Errorf("expected >0 matches from Go optimizer on 16_test_dispatch_optimal_tours")
	}

	// 2. Parse and verify Golden expected multi-leg tours
	records, err := legacy.ParseGoldenRankedDriversFile(expectedDriversFile)
	if err != nil {
		t.Fatalf("Failed to parse 16_test_dispatch_optimal_tours expected file: %v", err)
	}

	t.Logf("Parsed %d optimal tour moves from 16_test_dispatch_optimal_tours", len(records))

	// Group legs by DriverID and Rank1
	type TourKey struct {
		DriverID string
		Rank1    int
	}

	tourLegsMap := make(map[TourKey][]legacy.GoldenDriverMove)
	for _, rec := range records {
		key := TourKey{DriverID: rec.DriverID, Rank1: rec.Rank1}
		tourLegsMap[key] = append(tourLegsMap[key], rec)
	}

	multiLegTourCount := 0
	singleLegCount := 0

	for key, legs := range tourLegsMap {
		if len(legs) > 1 {
			multiLegTourCount++
			// Verify spatial flow conservation between consecutive legs
			for i := 0; i < len(legs)-1; i++ {
				currLeg := legs[i]
				nextLeg := legs[i+1]
				if currLeg.DropoffLoc != "" && nextLeg.PickupLoc != "" {
					if currLeg.DropoffLoc != nextLeg.PickupLoc && nextLeg.EmptyMiles == 0 {
						t.Errorf("Driver %s Tour %d leg %d->%d discontinuous: %s != %s",
							key.DriverID, key.Rank1, i, i+1, currLeg.DropoffLoc, nextLeg.PickupLoc)
					}
				}
			}
		} else {
			singleLegCount++
		}
	}

	t.Logf("16_test_dispatch_optimal_tours Analysis:")
	t.Logf("  Total Evaluated Tours: %d", len(tourLegsMap))
	t.Logf("  Multi-Leg Chained Tours: %d", multiLegTourCount)
	t.Logf("  Single-Leg Moves: %d", singleLegCount)

	if multiLegTourCount == 0 {
		t.Errorf("expected multi-leg chained tours in 16_test_dispatch_optimal_tours")
	}
}
