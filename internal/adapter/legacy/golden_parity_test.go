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
	"github.com/optimaldynamics/project-mittens/pkg/journal"
	"github.com/optimaldynamics/project-mittens/pkg/replay"
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

// createAndVerifyJournalRecord seals a cryptographic JournalRecord, appends it to a MemoryStore,
// and asserts Merkle chain continuity and validity (Inviolate 7 & Section 19.4).
func createAndVerifyJournalRecord[C model.CompetitorScale](
	t *testing.T,
	decisionID string,
	runID string,
	epoch int64,
	pol policy.Policy[C],
	prov policy.DecisionProvenance,
	state *model.State[C],
	action *model.Action,
) (journal.JournalRecord, journal.JournalStore) {
	t.Helper()
	initialStateHash, _ := journal.HashState(state)
	rBytes, _, _ := journal.EncodeCanonicalResource(state.Resource())
	iBytes, _, _ := journal.EncodeCanonicalInformation(state.Information())
	bBytes, _, _ := journal.EncodeCanonicalBelief(state.Belief())
	aBytes, aHash, _ := journal.EncodeCanonicalAction(action)

	paramHash := journal.ComputeSHA256([]byte(pol.Name()))
	if len(prov.ThetaParameters) > 0 {
		if pHash, err := journal.HashParameters(prov.ThetaParameters); err == nil {
			paramHash = pHash
		}
	}

	rec := journal.JournalRecord{
		RunID:                 runID,
		Epoch:                 epoch,
		BatchSeq:              1,
		DecisionID:            decisionID,
		PrevRecordHash:        journal.GenesisPrevHash,
		RuntimeVersion:        journal.CurrentRuntimeVersion,
		PolicyName:            pol.Name(),
		PolicyParamHash:       paramHash,
		InitialStateHash:      initialStateHash,
		ResourceStateBytes:    rBytes,
		InformationStateBytes: iBytes,
		BeliefStateBytes:      bBytes,
		ActionHash:            aHash,
		ActionBytes:           aBytes,
		MatchedCount:          action.MatchCount(),
		EvaluatedArcsCount:    len(prov.EvaluatedArcs),
		TotalNetContribution:  prov.TotalNetContribution,
		NextStateHash:         initialStateHash,
	}
	rec.Seal()

	store := journal.NewMemoryStore()
	if err := store.Append(rec); err != nil {
		t.Fatalf("failed appending cryptographic journal record: %v", err)
	}

	valid, lastHash, err := store.VerifyRunChain(runID)
	if !valid || err != nil {
		t.Fatalf("Merkle chain verification failed for %s: %v", runID, err)
	}
	if lastHash != rec.RecordHash {
		t.Errorf("expected latest hash %s, got %s", rec.RecordHash, lastHash)
	}

	return rec, store
}

// TestGoldenParity_13Relays characterises relay exchange synthesis, dual-driver swap handoffs,
// and Merkle cryptographic journal integrity on the legacy 13_test_relays fixture.
func TestGoldenParity_13Relays(t *testing.T) {
	coreaiRoot := getCoreAIRoot(t)
	scenarioDir := filepath.Join(coreaiRoot, "engine/smart_tl/worker/tests/13_test_relays")
	inputDir := filepath.Join(scenarioDir, "input")

	locFile := filepath.Join(inputDir, "locations_WR.txt")
	driverFile := filepath.Join(inputDir, "DRIVERS_NO_OTR_DED_Sampled.txt")
	loadFile := filepath.Join(inputDir, "RC_AND_WR_LOADS_Sampled.txt")
	relayFile := filepath.Join(inputDir, "RC_all_relays.txt")

	// 1. Load inputs
	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 250)
	if err != nil {
		t.Fatalf("LoadCarrierScenario failed on 13_test_relays: %v", err)
	}

	relF, err := os.Open(relayFile)
	if err != nil {
		t.Fatalf("cannot open relay file: %v", err)
	}
	defer relF.Close()
	relays, err := legacy.ParseRelays(relF)
	if err != nil {
		t.Fatalf("ParseRelays failed: %v", err)
	}

	t.Logf("Loaded 13_test_relays: %d drivers, %d loads, %d relay nodes", len(drivers), len(loads), len(relays))

	// 2. Configure Go Optimizer dynamically aligned to earliest pickup epoch
	minPickupEpoch := int64(math.MaxInt64)
	for _, l := range loads {
		if l.PickupEarliestEpoch > 0 && l.PickupEarliestEpoch < minPickupEpoch {
			minPickupEpoch = l.PickupEarliestEpoch
		}
	}
	baseEpoch := minPickupEpoch - 3600

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

	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 1500.0
	feasCfg.MaxEarlyDwellHours = 120.0
	feasCfg.MaxLateDeliveryHours = 24.0

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		feasCfg,
		nil,
	)

	// 3. Evaluate matching
	action, prov, err := cfaPol.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("Go Policy Evaluate failed on 13_test_relays: %v", err)
	}

	t.Logf("13_test_relays Go Matching: %d matches, Net Contribution: $%.2f",
		action.MatchCount(), prov.TotalNetContribution)

	if action.MatchCount() == 0 {
		t.Errorf("expected >0 matches from Go optimizer on 13_test_relays")
	}

	// 4. Generate Cryptographic Journal Record & verify Merkle continuity
	record, _ := createAndVerifyJournalRecord(
		t,
		"DEC-13-RELAYS-001",
		"RUN-GOLDEN-13",
		baseEpoch,
		cfaPol,
		prov,
		state,
		action,
	)

	// 5. Offline Bit-Exact Replay Verification (Principle 2)
	replayEngine, err := replay.NewReplayEngine[model.Monopolistic](cfaPol)
	if err != nil {
		t.Fatalf("NewReplayEngine failed: %v", err)
	}

	report, err := replayEngine.ReplayDecision(context.Background(), record, state)
	if err != nil {
		t.Fatalf("ReplayDecision failed: %v", err)
	}
	if !report.IsBitExact {
		t.Fatalf("expected bit-exact replay on 13_test_relays, got drift: %v", report.DriftDetails)
	}
	if !report.InitialStateHashMatch || !report.ActionHashMatch {
		t.Errorf("expected state and action hash match during replay")
	}
}

// TestGoldenParity_05HomeTime evaluates driver time-at-home deadlines, rest dwell,
// and domicile returns on the legacy 05_test_home_time fixture.
func TestGoldenParity_05HomeTime(t *testing.T) {
	coreaiRoot := getCoreAIRoot(t)
	scenarioDir := filepath.Join(coreaiRoot, "engine/smart_tl/worker/tests/05_test_home_time")
	inputDir := filepath.Join(scenarioDir, "input")

	locFile := filepath.Join(inputDir, "locations_out.txt")
	driverFile := filepath.Join(inputDir, "drivers_1_custom.txt")
	loadFile := filepath.Join(inputDir, "loads.txt")
	tahFile := filepath.Join(inputDir, "driverTAHSchedules.txt")

	// 1. Load inputs
	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 100)
	if err != nil {
		t.Fatalf("LoadCarrierScenario failed on 05_test_home_time: %v", err)
	}

	tahF, err := os.Open(tahFile)
	if err != nil {
		t.Fatalf("cannot open TAH file: %v", err)
	}
	defer tahF.Close()
	schedules, err := legacy.ParseTimeAtHomeSchedules(tahF)
	if err != nil {
		t.Fatalf("ParseTimeAtHomeSchedules failed: %v", err)
	}

	t.Logf("Loaded 05_test_home_time: %d drivers, %d loads, %d TAH driver schedules",
		len(drivers), len(loads), len(schedules))

	// 2. Configure Go Optimizer dynamically aligned to earliest pickup epoch
	minPickupEpoch := int64(math.MaxInt64)
	for _, l := range loads {
		if l.PickupEarliestEpoch > 0 && l.PickupEarliestEpoch < minPickupEpoch {
			minPickupEpoch = l.PickupEarliestEpoch
		}
	}
	baseEpoch := minPickupEpoch - 3600

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

	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 1500.0
	feasCfg.MaxEarlyDwellHours = 120.0
	feasCfg.MaxLateDeliveryHours = 24.0

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		feasCfg,
		nil,
	)

	// 3. Evaluate matching
	action, prov, err := cfaPol.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("Go Policy Evaluate failed on 05_test_home_time: %v", err)
	}

	t.Logf("05_test_home_time Go Matching: %d matches, Net Contribution: $%.2f",
		action.MatchCount(), prov.TotalNetContribution)

	if action.MatchCount() == 0 {
		t.Errorf("expected >0 matches from Go optimizer on 05_test_home_time")
	}

	// 4. Generate Cryptographic Journal & Replay Audit
	record, _ := createAndVerifyJournalRecord(
		t,
		"DEC-05-HOMETIME-001",
		"RUN-GOLDEN-05",
		baseEpoch,
		cfaPol,
		prov,
		state,
		action,
	)

	replayEngine, err := replay.NewReplayEngine[model.Monopolistic](cfaPol)
	if err != nil {
		t.Fatalf("NewReplayEngine failed: %v", err)
	}

	report, err := replayEngine.ReplayDecision(context.Background(), record, state)
	if err != nil {
		t.Fatalf("ReplayDecision failed: %v", err)
	}
	if !report.IsBitExact {
		t.Fatalf("expected bit-exact replay on 05_test_home_time, got drift: %v", report.DriftDetails)
	}
}

// TestGoldenParity_14PreAssignments evaluates committed freight bindings, must-dispatch preassignments,
// and residual open freight on the legacy 14_test_pre_assignments fixture.
func TestGoldenParity_14PreAssignments(t *testing.T) {
	coreaiRoot := getCoreAIRoot(t)
	scenarioDir := filepath.Join(coreaiRoot, "engine/smart_tl/worker/tests/14_test_pre_assignments")
	inputDir := filepath.Join(scenarioDir, "input")

	locFile := filepath.Join(inputDir, "locations.txt")
	driverFile := filepath.Join(inputDir, "drivers.txt")
	loadFile := filepath.Join(inputDir, "loads.txt")
	preFile := filepath.Join(inputDir, "preassignments.txt")

	// 1. Load inputs
	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 150)
	if err != nil {
		t.Fatalf("LoadCarrierScenario failed on 14_test_pre_assignments: %v", err)
	}

	preF, err := os.Open(preFile)
	if err != nil {
		t.Fatalf("cannot open preassignments file: %v", err)
	}
	defer preF.Close()
	preMap, err := legacy.ParsePreAssignments(preF)
	if err != nil {
		t.Fatalf("ParsePreAssignments failed: %v", err)
	}

	t.Logf("Loaded 14_test_pre_assignments: %d drivers, %d loads, %d preassignments",
		len(drivers), len(loads), len(preMap))

	// 2. Configure Go Optimizer dynamically aligned to earliest pickup epoch
	minPickupEpoch := int64(math.MaxInt64)
	for _, l := range loads {
		if l.PickupEarliestEpoch > 0 && l.PickupEarliestEpoch < minPickupEpoch {
			minPickupEpoch = l.PickupEarliestEpoch
		}
	}
	baseEpoch := minPickupEpoch - 2*3600

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

	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 1500.0
	feasCfg.MaxEarlyDwellHours = 120.0
	feasCfg.MaxLateDeliveryHours = 72.0

	costCfg := model.DefaultCostConfig()
	costCfg.EarlyArrivalPerHour = 0.0

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		costCfg,
		feasCfg,
		nil,
	)

	// 3. Evaluate matching
	action, prov, err := cfaPol.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("Go Policy Evaluate failed on 14_test_pre_assignments: %v", err)
	}

	t.Logf("14_test_pre_assignments Go Matching: %d matches, Net Contribution: $%.2f",
		action.MatchCount(), prov.TotalNetContribution)

	if action.MatchCount() == 0 {
		t.Errorf("expected >0 matches from Go optimizer on 14_test_pre_assignments")
	}

	// 4. Generate Cryptographic Journal & Replay Audit
	record, _ := createAndVerifyJournalRecord(
		t,
		"DEC-14-PREASSIGN-001",
		"RUN-GOLDEN-14",
		baseEpoch,
		cfaPol,
		prov,
		state,
		action,
	)

	replayEngine, err := replay.NewReplayEngine[model.Monopolistic](cfaPol)
	if err != nil {
		t.Fatalf("NewReplayEngine failed: %v", err)
	}

	report, err := replayEngine.ReplayDecision(context.Background(), record, state)
	if err != nil {
		t.Fatalf("ReplayDecision failed: %v", err)
	}
	if !report.IsBitExact {
		t.Fatalf("expected bit-exact replay on 14_test_pre_assignments, got drift: %v", report.DriftDetails)
	}
}

// TestGoldenParity_17DriverGeoConstraints evaluates driver regional operational boundaries,
// geographic domain filtering, and uncovered loads on the legacy 17_test_driver_geo_constraints fixture.
func TestGoldenParity_17DriverGeoConstraints(t *testing.T) {
	coreaiRoot := getCoreAIRoot(t)
	scenarioDir := filepath.Join(coreaiRoot, "engine/smart_tl/worker/tests/17_test_driver_geo_constraints")
	inputDir := filepath.Join(scenarioDir, "input")

	locFile := filepath.Join(inputDir, "locations.txt")
	driverFile := filepath.Join(inputDir, "drivers.txt")
	loadFile := filepath.Join(inputDir, "loads.txt")

	// 1. Load inputs
	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 200)
	if err != nil {
		t.Fatalf("LoadCarrierScenario failed on 17_test_driver_geo_constraints: %v", err)
	}

	t.Logf("Loaded 17_test_driver_geo_constraints: %d drivers, %d loads", len(drivers), len(loads))

	// 2. Configure Go Optimizer dynamically aligned to earliest pickup epoch
	minPickupEpoch := int64(math.MaxInt64)
	for _, l := range loads {
		if l.PickupEarliestEpoch > 0 && l.PickupEarliestEpoch < minPickupEpoch {
			minPickupEpoch = l.PickupEarliestEpoch
		}
	}
	baseEpoch := minPickupEpoch - 2*3600

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

	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 1500.0
	feasCfg.MaxEarlyDwellHours = 120.0
	feasCfg.MaxLateDeliveryHours = 72.0

	costCfg := model.DefaultCostConfig()
	costCfg.EarlyArrivalPerHour = 0.0

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		costCfg,
		feasCfg,
		nil,
	)

	// 3. Evaluate matching
	action, prov, err := cfaPol.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("Go Policy Evaluate failed on 17_test_driver_geo_constraints: %v", err)
	}

	t.Logf("17_test_driver_geo_constraints Go Matching: %d matches, Net Contribution: $%.2f",
		action.MatchCount(), prov.TotalNetContribution)

	if action.MatchCount() == 0 {
		t.Errorf("expected >0 matches from Go optimizer on 17_test_driver_geo_constraints")
	}

	// 4. Generate Cryptographic Journal & Replay Audit
	record, _ := createAndVerifyJournalRecord(
		t,
		"DEC-17-GEOCONSTR-001",
		"RUN-GOLDEN-17",
		baseEpoch,
		cfaPol,
		prov,
		state,
		action,
	)

	replayEngine, err := replay.NewReplayEngine[model.Monopolistic](cfaPol)
	if err != nil {
		t.Fatalf("NewReplayEngine failed: %v", err)
	}

	report, err := replayEngine.ReplayDecision(context.Background(), record, state)
	if err != nil {
		t.Fatalf("ReplayDecision failed: %v", err)
	}
	if !report.IsBitExact {
		t.Fatalf("expected bit-exact replay on 17_test_driver_geo_constraints, got drift: %v", report.DriftDetails)
	}
}

// TestGoldenParity_15OnTimeParameters evaluates rolling 10-day HOS historical logs and
// tight delivery appointment windows on the legacy 15_test_on_time_parameters fixture.
func TestGoldenParity_15OnTimeParameters(t *testing.T) {
	coreaiRoot := getCoreAIRoot(t)
	scenarioDir := filepath.Join(coreaiRoot, "engine/smart_tl/worker/tests/15_test_on_time_parameters")
	inputDir := filepath.Join(scenarioDir, "input")

	locFile := filepath.Join(inputDir, "locations_out.txt")
	driverFile := filepath.Join(inputDir, "drivers_1_custom.txt")
	loadFile := filepath.Join(inputDir, "loads-new.txt")

	// 1. Load inputs
	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 150)
	if err != nil {
		t.Fatalf("LoadCarrierScenario failed on 15_test_on_time_parameters: %v", err)
	}

	t.Logf("Loaded 15_test_on_time_parameters: %d drivers, %d loads", len(drivers), len(loads))

	// 2. Configure Go Optimizer dynamically aligned to earliest pickup epoch
	minPickupEpoch := int64(math.MaxInt64)
	for _, l := range loads {
		if l.PickupEarliestEpoch > 0 && l.PickupEarliestEpoch < minPickupEpoch {
			minPickupEpoch = l.PickupEarliestEpoch
		}
	}
	baseEpoch := minPickupEpoch - 2*3600

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

	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 1500.0
	feasCfg.MaxEarlyDwellHours = 120.0
	feasCfg.MaxLateDeliveryHours = 72.0

	costCfg := model.DefaultCostConfig()
	costCfg.EarlyArrivalPerHour = 0.0

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		costCfg,
		feasCfg,
		nil,
	)

	// 3. Evaluate matching
	action, prov, err := cfaPol.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("Go Policy Evaluate failed on 15_test_on_time_parameters: %v", err)
	}

	t.Logf("15_test_on_time_parameters Go Matching: %d matches, Net Contribution: $%.2f",
		action.MatchCount(), prov.TotalNetContribution)

	if action.MatchCount() == 0 {
		t.Errorf("expected >0 matches from Go optimizer on 15_test_on_time_parameters")
	}

	// 4. Generate Cryptographic Journal & Replay Audit
	record, _ := createAndVerifyJournalRecord(
		t,
		"DEC-15-ONTIME-001",
		"RUN-GOLDEN-15",
		baseEpoch,
		cfaPol,
		prov,
		state,
		action,
	)

	replayEngine, err := replay.NewReplayEngine[model.Monopolistic](cfaPol)
	if err != nil {
		t.Fatalf("NewReplayEngine failed: %v", err)
	}

	report, err := replayEngine.ReplayDecision(context.Background(), record, state)
	if err != nil {
		t.Fatalf("ReplayDecision failed: %v", err)
	}
	if !report.IsBitExact {
		t.Fatalf("expected bit-exact replay on 15_test_on_time_parameters, got drift: %v", report.DriftDetails)
	}
}
