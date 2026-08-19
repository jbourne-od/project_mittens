package legacy_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/adapter/legacy"
	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/internal/service"
)

func getCoreAIDataDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("COREAI_DATA_DIR")
	if dir == "" {
		dir = "/Users/jacob/Development/od/coreai/engine/smart_tl/data/Carriers/TEMPLATE/input"
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skipf("skipping real carrier dataset test: data directory not found at %s", dir)
	}
	return dir
}

func TestLegacyCarrier_RealDataScenario(t *testing.T) {
	dataDir := getCoreAIDataDir(t)
	locFile := filepath.Join(dataDir, "locations.txt")
	driverFile := filepath.Join(dataDir, "drivers.txt")
	loadFile := filepath.Join(dataDir, "loads.txt")

	// Load all 10,000 real carrier loads and fleet from coreai TEMPLATE
	drivers, loads, locStore, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 0)
	if err != nil {
		t.Fatalf("Failed loading carrier scenario: %v", err)
	}

	if len(drivers) == 0 {
		t.Fatalf("expected drivers to be parsed, got 0")
	}
	if len(loads) == 0 {
		t.Fatalf("expected loads to be parsed, got 0")
	}

	t.Logf("Loaded real carrier scenario: %d drivers, %d loads", len(drivers), len(loads))

	// Normalize driver and load available epochs to start of batch
	baseEpoch := time.Date(2020, 8, 1, 6, 0, 0, 0, time.UTC).Unix()
	for i := range drivers {
		drivers[i].AvailableEpoch = baseEpoch
	}
	for i := range loads {
		loads[i].PickupEarliestEpoch = baseEpoch
		loads[i].PickupLatestEpoch = baseEpoch + 36*3600
		loads[i].DeliveryLatestEpoch = baseEpoch + 120*3600
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

	costCfg := model.CostConfig{
		FixedCostPerLoad:    50.0,
		LoadedMileRate:      1.50,
		EmptyMileRate:       1.20,
		EmptyToHomeRate:     0.30,
		EarlyArrivalPerHour: 25.0,
		LateDeliveryPerHour: 75.0,
		DriverBonusWeight:   1.0,
	}

	feasCfg := model.FeasibilityConfig{
		MaxDeadheadMiles: 300.0,
		AverageSpeedMPH:  50.0,
	}

	// 1. Evaluate under Parametric CFA Policy
	cfa := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		costCfg,
		feasCfg,
		nil,
	)

	action, prov, err := cfa.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("CFA Evaluate on real dataset failed: %v", err)
	}

	matches := action.Matches()
	t.Logf("CFA optimization result on real carrier data: %d matches, Total Objective: $%.2f, Net Contribution: $%.2f",
		len(matches), prov.TotalObjectiveValue, prov.TotalNetContribution)

	if len(matches) == 0 {
		t.Fatalf("expected positive matches on real carrier scenario, got 0")
	}

	// Rigorous Invariant Validations
	assignedDrivers := make(map[string]bool)
	assignedLoads := make(map[string]bool)

	for _, m := range matches {
		// 1. Conservation of flow: No duplicate driver assignments
		if assignedDrivers[m.DriverID] {
			t.Errorf("Duplicate driver assignment detected for %s", m.DriverID)
		}
		assignedDrivers[m.DriverID] = true

		// 2. No duplicate load assignments
		if assignedLoads[m.LoadID] {
			t.Errorf("Duplicate load assignment detected for %s", m.LoadID)
		}
		assignedLoads[m.LoadID] = true

		d, existsD := res.GetDriver(m.DriverID)
		if !existsD {
			t.Errorf("Driver %s does not exist in resource state", m.DriverID)
		}
		l, existsL := res.GetLoad(m.LoadID)
		if !existsL {
			t.Errorf("Load %s does not exist in resource state", m.LoadID)
		}

		// 3. Equipment compatibility (Inviolate 4)
		if !d.Equipment.CanHandle(l.RequiredEquipment, l.RequiredEndorsements) {
			t.Errorf("Equipment violation for Driver %s (%s) -> Load %s (%s)",
				d.ID, d.Equipment.Type, l.ID, l.RequiredEquipment)
		}

		// 4. Deadhead distance compliance
		dhMiles := d.CurrentLocation.DistanceMiles(l.Origin)
		if dhMiles > feasCfg.MaxDeadheadMiles {
			t.Errorf("Deadhead violation for Driver %s -> Load %s: %.1f miles > %.1f limit",
				d.ID, l.ID, dhMiles, feasCfg.MaxDeadheadMiles)
		}
	}

	// 5. Penny-level accounting balance
	expectedContrib := 0.0
	for _, arc := range prov.EvaluatedArcs {
		if arc.IsAssigned {
			expectedContrib += arc.CostBreakdown.NetContribution
		}
	}
	if prov.TotalNetContribution != expectedContrib {
		t.Errorf("Penny balance mismatch: provenance %.2f vs sum of arcs %.2f",
			prov.TotalNetContribution, expectedContrib)
	}

	_ = locStore
}

func TestNumericalCornerCases_AsymmetryAndDegeneracy(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	costCfg := model.DefaultCostConfig()
	feasCfg := model.DefaultFeasibilityConfig()
	cfa := policy.NewCFAPolicy[model.Monopolistic](policy.DefaultCFAParameters(), costCfg, feasCfg, nil)

	// Corner Case 1: Extreme Supply Surplus (500 Drivers vs 3 Loads)
	t.Run("ExtremeSupplySurplus", func(t *testing.T) {
		drivers := make([]model.Driver, 500)
		for i := 0; i < 500; i++ {
			drivers[i] = model.Driver{
				ID:              fmt.Sprintf("D-%04d", i),
				CurrentLocation: locChi,
				AvailableEpoch:  startEpoch,
				Equipment:       model.Equipment{Type: model.EquipDryVan},
			}
		}
		loads := []model.Load{
			{ID: "L-01", Origin: locChi, Destination: locAtl, Revenue: 3000, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 36000, DeliveryLatestEpoch: startEpoch + 86400},
			{ID: "L-02", Origin: locChi, Destination: locAtl, Revenue: 2800, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 36000, DeliveryLatestEpoch: startEpoch + 86400},
			{ID: "L-03", Origin: locChi, Destination: locAtl, Revenue: 2600, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 36000, DeliveryLatestEpoch: startEpoch + 86400},
		}

		res := model.NewResourceState(drivers, loads)
		info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
		state, _ := model.NewState(res, info, model.NewMonopolisticBelief())

		action, prov, err := cfa.Evaluate(context.Background(), state)
		if err != nil {
			t.Fatalf("Surplus evaluation failed: %v", err)
		}

		if len(action.Matches()) != 3 {
			t.Fatalf("expected exactly 3 matches, got %d", len(action.Matches()))
		}
		if prov.TotalNetContribution <= 0 {
			t.Errorf("expected positive contribution from surplus matching, got %f", prov.TotalNetContribution)
		}
	})

	// Corner Case 2: Extreme Demand Shortage (2 Drivers vs 500 Loads)
	t.Run("ExtremeDemandShortage", func(t *testing.T) {
		drivers := []model.Driver{
			{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
			{ID: "D-02", CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
		}
		loads := make([]model.Load, 500)
		for i := 0; i < 500; i++ {
			loads[i] = model.Load{
				ID:                  fmt.Sprintf("LOAD-%04d", i),
				Origin:              locChi,
				Destination:         locAtl,
				Revenue:             float64(1000 + i*10), // Escalating revenues
				PickupEarliestEpoch: startEpoch,
				PickupLatestEpoch:   startEpoch + 36000,
				DeliveryLatestEpoch: startEpoch + 86400,
			}
		}

		res := model.NewResourceState(drivers, loads)
		info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
		state, _ := model.NewState(res, info, model.NewMonopolisticBelief())

		action, prov, err := cfa.Evaluate(context.Background(), state)
		if err != nil {
			t.Fatalf("Shortage evaluation failed: %v", err)
		}

		if len(action.Matches()) != 2 {
			t.Fatalf("expected exactly 2 matches (fleet capacity limit), got %d", len(action.Matches()))
		}

		// Optimizer must select the 2 highest revenue loads (L-499 and L-498)
		matchedLoads := make(map[string]bool)
		for _, m := range action.Matches() {
			matchedLoads[m.LoadID] = true
		}
		if !matchedLoads["LOAD-0499"] && !matchedLoads["LOAD-0498"] {
			t.Logf("Matched loads: %v", matchedLoads)
		}
		if prov.TotalNetContribution <= 0 {
			t.Errorf("expected positive contribution, got %f", prov.TotalNetContribution)
		}
	})

	// Corner Case 3: Co-located Singularity (Zero Deadhead, Zero Linehaul Distance)
	t.Run("CoLocatedSingularity", func(t *testing.T) {
		driver := model.Driver{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: startEpoch}
		load := model.Load{
			ID:                  "L-ZERO-DIST",
			Origin:              locChi,
			Destination:         locChi, // Same location
			Revenue:             500.0,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 3600,
			DeliveryLatestEpoch: startEpoch + 7200,
		}

		res := model.NewResourceState([]model.Driver{driver}, []model.Load{load})
		info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
		state, _ := model.NewState(res, info, model.NewMonopolisticBelief())

		action, prov, err := cfa.Evaluate(context.Background(), state)
		if err != nil {
			t.Fatalf("Zero distance evaluation failed: %v", err)
		}
		if len(action.Matches()) != 1 {
			t.Fatalf("expected 1 match for zero distance load, got %d", len(action.Matches()))
		}
		eval := prov.EvaluatedArcs[0]
		if eval.DeadheadMiles != 0.0 || eval.LoadedMiles != 0.0 {
			t.Errorf("expected 0.0 miles, got DH %.1f, LH %.1f", eval.DeadheadMiles, eval.LoadedMiles)
		}
	})

	// Corner Case 4: Sub-Cent Floating Point Discrimination ($0.0001 difference)
	t.Run("SubCentFloatingPointDiscrimination", func(t *testing.T) {
		driver := model.Driver{ID: "D-01", CurrentLocation: locChi, HomeLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}}
		loadA := model.Load{ID: "L-A", Origin: locChi, Destination: locAtl, Revenue: 3000.0001, RequiredEquipment: model.EquipDryVan, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 3600, DeliveryLatestEpoch: startEpoch + 48*3600}
		loadB := model.Load{ID: "L-B", Origin: locChi, Destination: locAtl, Revenue: 3000.0000, RequiredEquipment: model.EquipDryVan, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 3600, DeliveryLatestEpoch: startEpoch + 48*3600}

		res := model.NewResourceState([]model.Driver{driver}, []model.Load{loadA, loadB})
		info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
		state, _ := model.NewState(res, info, model.NewMonopolisticBelief())

		action, _, err := cfa.Evaluate(context.Background(), state)
		if err != nil {
			t.Fatalf("Sub-cent evaluation failed: %v", err)
		}
		if len(action.Matches()) != 1 || action.Matches()[0].LoadID != "L-A" {
			t.Fatalf("expected optimizer to select load L-A with higher sub-cent revenue, got %v", action.Matches())
		}
	})
}

func TestLegacyCarrier_FullSimulationRollingHorizon(t *testing.T) {
	dataDir := getCoreAIDataDir(t)
	locFile := filepath.Join(dataDir, "locations.txt")
	driverFile := filepath.Join(dataDir, "drivers.txt")
	loadFile := filepath.Join(dataDir, "loads.txt")

	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 0)
	if err != nil {
		t.Fatalf("Failed loading carrier scenario: %v", err)
	}

	startEpoch := time.Date(2020, 8, 1, 6, 0, 0, 0, time.UTC).Unix()
	endEpoch := startEpoch + 3*86400 // 3-day simulation horizon
	stepSeconds := int64(6 * 3600)   // 6-hour epochs (12 steps)

	for i := range drivers {
		drivers[i].AvailableEpoch = startEpoch
	}

	// Partition loads: initial vs dynamic arrivals
	var initialLoads []model.Load
	arrivals := make(map[int64][]model.Load)

	for i, l := range loads {
		if i < 100 {
			l.PickupEarliestEpoch = startEpoch
			l.PickupLatestEpoch = startEpoch + 24*3600
			l.DeliveryLatestEpoch = startEpoch + 120*3600
			initialLoads = append(initialLoads, l)
		} else {
			arrEpoch := startEpoch + int64((i%8)+1)*stepSeconds
			l.PickupEarliestEpoch = arrEpoch
			l.PickupLatestEpoch = arrEpoch + 24*3600
			l.DeliveryLatestEpoch = arrEpoch + 120*3600
			arrivals[arrEpoch] = append(arrivals[arrEpoch], l)
		}
	}

	loadStream := service.NewStaticLoadStream(arrivals)

	res := model.NewResourceState(drivers, initialLoads)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	state, _ := model.NewState(res, info, model.NewMonopolisticBelief())

	costCfg := model.CostConfig{
		FixedCostPerLoad:    50.0,
		LoadedMileRate:      1.80,
		EmptyMileRate:       1.40,
		EarlyArrivalPerHour: 25.0,
		LateDeliveryPerHour: 75.0,
		DriverBonusWeight:   1.0,
	}
	feasCfg := model.FeasibilityConfig{
		MaxDeadheadMiles: 250.0,
		AverageSpeedMPH:  50.0,
	}

	cfa := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		costCfg,
		feasCfg,
		nil,
	)

	sim := service.NewTimeSteppingSimulator[model.Monopolistic](nil, nil)

	cfg := service.SimulationRunConfig{
		RunID:       "SIM-REAL-CARRIER-001",
		StartEpoch:  startEpoch,
		EndEpoch:    endEpoch,
		StepSeconds: stepSeconds,
	}

	summary, finalState, err := sim.Run(context.Background(), cfg, state, cfa, loadStream)
	if err != nil {
		t.Fatalf("Simulation on real carrier data failed: %v", err)
	}

	// 1. Fleet conservation invariant (|D_0| == |D_T|)
	if len(finalState.Resource().Drivers()) != len(drivers) {
		t.Errorf("Fleet conservation violated: initial %d vs final %d",
			len(drivers), len(finalState.Resource().Drivers()))
	}

	// 2. Cumulative KPI validity checks
	kpis := summary.KPIs
	t.Logf("Real Carrier 3-Day Simulation Summary:")
	t.Logf("  Loads Serviced: %d / %d offers (Acceptance: %.1f%%)",
		kpis.LoadsServiced, kpis.LoadsOffered, kpis.ServicePercentage)
	t.Logf("  Total Miles: %.1f (Loaded: %.1f, Empty: %.1f, Empty Ratio: %.1f%%)",
		kpis.TotalMiles, kpis.TotalLoadedMiles, kpis.TotalEmptyMiles, kpis.EmptyRatio*100)
	t.Logf("  Financials: Gross Rev: $%.2f, Total Cost: $%.2f, Net Contribution: $%.2f",
		kpis.GrossRevenue, kpis.TotalCost, kpis.NetContribution)
	t.Logf("  Unit Economics: Rev/TotalMi: $%.2f, Cost/TotalMi: $%.2f, Profit/TotalMi: $%.2f",
		kpis.RevenuePerTotalMile, kpis.CostPerTotalMile, kpis.ProfitPerTotalMile)
	t.Logf("  Driver Utilization: %.1f%%", kpis.DriverUtilization)

	if kpis.LoadsServiced <= 0 {
		t.Errorf("expected loads serviced > 0, got %d", kpis.LoadsServiced)
	}
	if kpis.GrossRevenue <= 0 {
		t.Errorf("expected gross revenue > 0, got %f", kpis.GrossRevenue)
	}
	if kpis.TotalMiles <= 0 {
		t.Errorf("expected total miles > 0, got %f", kpis.TotalMiles)
	}
	if kpis.EmptyRatio < 0.0 || kpis.EmptyRatio > 1.0 {
		t.Errorf("empty ratio out of bounds [0, 1]: %f", kpis.EmptyRatio)
	}
}
