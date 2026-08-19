package service_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/internal/service"
)

func TestStatisticCalculator_AccumulationAndKPIs(t *testing.T) {
	sc := service.NewStatisticCalculator()

	sc.RecordLoadOffers(10)
	sc.RecordDriverHours(80.0, 100.0)

	cost1 := policy.TripCostBreakdown{
		Revenue:         3000.0,
		FixedCost:       50.0,
		LoadedCost:      1000.0,
		EmptyCost:       100.0,
		EmptyToHomeCost: 200.0,
		DwellCost:       50.0,
		LatePenalty:     0.0,
		DriverBonus:     100.0,
		TotalCost:       1400.0,
		NetContribution: 1700.0, // 3000 - 1400 + 100
	}
	sc.RecordDispatch(cost1, 600.0, 60.0, 60, 0, true, true)

	cost2 := policy.TripCostBreakdown{
		Revenue:         2000.0,
		FixedCost:       50.0,
		LoadedCost:      700.0,
		EmptyCost:       150.0,
		EmptyToHomeCost: 100.0,
		DwellCost:       0.0,
		LatePenalty:     100.0,
		DriverBonus:     0.0,
		TotalCost:       1100.0,
		NetContribution: 900.0, // 2000 - 1100
	}
	sc.RecordDispatch(cost2, 400.0, 90.0, 0, 0, true, false)

	kpi := sc.Snapshot()

	if kpi.LoadsServiced != 2 {
		t.Errorf("expected 2 loads serviced, got %d", kpi.LoadsServiced)
	}
	if kpi.LoadsOffered != 10 {
		t.Errorf("expected 10 loads offered, got %d", kpi.LoadsOffered)
	}
	if kpi.ServicePercentage != 20.0 {
		t.Errorf("expected 20.0%% service percentage, got %f", kpi.ServicePercentage)
	}
	if kpi.OnTimeDeliveryPercent != 50.0 {
		t.Errorf("expected 50.0%% on time delivery, got %f", kpi.OnTimeDeliveryPercent)
	}
	if kpi.TotalLoadedMiles != 1000.0 {
		t.Errorf("expected 1000.0 loaded miles, got %f", kpi.TotalLoadedMiles)
	}
	if kpi.TotalEmptyMiles != 150.0 {
		t.Errorf("expected 150.0 empty miles, got %f", kpi.TotalEmptyMiles)
	}
	if kpi.TotalMiles != 1150.0 {
		t.Errorf("expected 1150.0 total miles, got %f", kpi.TotalMiles)
	}
	expectedEmptyRatio := 150.0 / 1150.0
	if mathAbs(kpi.EmptyRatio-expectedEmptyRatio) > 1e-4 {
		t.Errorf("expected empty ratio %f, got %f", expectedEmptyRatio, kpi.EmptyRatio)
	}
	if kpi.GrossRevenue != 5000.0 {
		t.Errorf("expected 5000.0 gross revenue, got %f", kpi.GrossRevenue)
	}
	if kpi.NetContribution != 2600.0 {
		t.Errorf("expected 2600.0 net contribution, got %f", kpi.NetContribution)
	}
	if kpi.DriverUtilization != 80.0 {
		t.Errorf("expected 80.0%% driver utilization, got %f", kpi.DriverUtilization)
	}
}

func TestMemoryJournal_ImmutabilityAndConcurrency(t *testing.T) {
	journal := service.NewMemoryJournal()
	const goroutines = 32
	const entriesPerGoroutine = 50

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < entriesPerGoroutine; i++ {
				entry := service.JournalEntry{
					DecisionID:           fmt.Sprintf("DEC-%d-%d", gID, i),
					BatchEpoch:           int64(i * 100),
					PolicyName:           "CFA",
					MatchedCount:         1,
					TotalNetContribution: float64(i * 10),
				}
				if err := journal.Record(context.Background(), entry); err != nil {
					t.Errorf("Record failed: %v", err)
				}
			}
		}(g)
	}

	wg.Wait()

	expectedTotal := goroutines * entriesPerGoroutine
	if journal.Count() != expectedTotal {
		t.Fatalf("expected %d journal entries, got %d", expectedTotal, journal.Count())
	}
	entries := journal.GetEntries()
	if len(entries) != expectedTotal {
		t.Fatalf("expected %d snapshotted entries, got %d", expectedTotal, len(entries))
	}
}

func TestOptimizationService_SingleEpochFlow(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	driver := model.Driver{
		ID:              "D-01",
		CurrentLocation: locChi,
		AvailableEpoch:  startEpoch,
		Equipment:       model.Equipment{Type: model.EquipDryVan},
	}
	load := model.Load{
		ID:                  "L-01",
		Origin:              locChi,
		Destination:         locAtl,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             3000.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 36000,
		DeliveryLatestEpoch: startEpoch + 120000,
	}

	res := model.NewResourceState([]model.Driver{driver}, []model.Load{load})
	info, err := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	if err != nil {
		t.Fatalf("NewInformationState failed: %v", err)
	}
	belief := model.NewMonopolisticBelief()
	state, err := model.NewState(res, info, belief)
	if err != nil {
		t.Fatalf("NewState failed: %v", err)
	}

	cfa := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	journal := service.NewMemoryJournal()
	svc := service.NewOptimizationService[model.Monopolistic](journal, nil)

	action, prov, nextState, err := svc.OptimizeEpoch(
		context.Background(),
		state,
		cfa,
		startEpoch+3600,
		nil,
	)
	if err != nil {
		t.Fatalf("OptimizeEpoch failed: %v", err)
	}

	if action.MatchCount() != 1 {
		t.Errorf("expected 1 match, got %d", action.MatchCount())
	}
	if prov.MatchedCount != 1 {
		t.Errorf("expected 1 matched count in provenance, got %d", prov.MatchedCount)
	}
	if journal.Count() != 1 {
		t.Errorf("expected 1 journal entry, got %d", journal.Count())
	}

	// Verify nextState transitions
	if len(nextState.Resource().Loads()) != 0 {
		t.Errorf("expected 0 remaining loads, got %d", len(nextState.Resource().Loads()))
	}
	nextDriver, _ := nextState.Resource().GetDriver("D-01")
	if nextDriver.CurrentLocation.NodeID != "ATL" {
		t.Errorf("expected driver location to transition to ATL, got %s", nextDriver.CurrentLocation.NodeID)
	}
}

func TestTimeSteppingSimulator_MultiDayHorizon(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locDal := model.Location{NodeID: "DAL", Lat: 32.7767, Lon: -96.7970}

	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()
	endEpoch := startEpoch + 7*24*3600 // 7-day simulation horizon
	stepSeconds := int64(3 * 3600)     // 3-hour epochs (56 steps)

	drivers := []model.Driver{
		{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
		{ID: "D-02", CurrentLocation: locAtl, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
	}
	initialLoads := []model.Load{
		{
			ID:                  "L-INIT-01",
			Origin:              locChi,
			Destination:         locAtl,
			RequiredEquipment:   model.EquipDryVan,
			Revenue:             3000.0,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 36000,
			DeliveryLatestEpoch: startEpoch + 120000,
		},
	}

	// Dynamic arrivals stream over the 7 days
	arrivals := map[int64][]model.Load{
		startEpoch + 12*3600: {
			{
				ID:                  "L-D1-01",
				Origin:              locAtl,
				Destination:         locDal,
				RequiredEquipment:   model.EquipDryVan,
				Revenue:             3500.0,
				PickupEarliestEpoch: startEpoch + 12*3600,
				PickupLatestEpoch:   startEpoch + 24*3600,
				DeliveryLatestEpoch: startEpoch + 3*86400,
			},
		},
		startEpoch + 36*3600: {
			{
				ID:                  "L-D2-01",
				Origin:              locDal,
				Destination:         locChi,
				RequiredEquipment:   model.EquipDryVan,
				Revenue:             4000.0,
				PickupEarliestEpoch: startEpoch + 36*3600,
				PickupLatestEpoch:   startEpoch + 48*3600,
				DeliveryLatestEpoch: startEpoch + 5*86400,
			},
		},
	}

	loadStream := service.NewStaticLoadStream(arrivals)

	res := model.NewResourceState(drivers, initialLoads)
	info, err := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	if err != nil {
		t.Fatalf("NewInformationState failed: %v", err)
	}
	belief := model.NewMonopolisticBelief()
	state, err := model.NewState(res, info, belief)
	if err != nil {
		t.Fatalf("NewState failed: %v", err)
	}

	cfa := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	sim := service.NewTimeSteppingSimulator[model.Monopolistic](nil, nil)

	cfg := service.SimulationRunConfig{
		RunID:       "SIM-7DAY-001",
		StartEpoch:  startEpoch,
		EndEpoch:    endEpoch,
		StepSeconds: stepSeconds,
	}

	summary, finalState, err := sim.Run(context.Background(), cfg, state, cfa, loadStream)
	if err != nil {
		t.Fatalf("Simulation Run failed: %v", err)
	}

	expectedEpochs := int((endEpoch-startEpoch)/stepSeconds) + 1
	if summary.TotalEpochs != expectedEpochs {
		t.Errorf("expected %d total epochs, got %d", expectedEpochs, summary.TotalEpochs)
	}
	if summary.KPIs.LoadsServiced < 2 {
		t.Errorf("expected at least 2 loads serviced over 7 days, got %d", summary.KPIs.LoadsServiced)
	}
	if summary.KPIs.GrossRevenue <= 0 {
		t.Errorf("expected positive gross revenue, got %f", summary.KPIs.GrossRevenue)
	}
	if summary.KPIs.NetContribution <= 0 {
		t.Errorf("expected positive net contribution, got %f", summary.KPIs.NetContribution)
	}

	// Verify fleet conservation: exactly 2 drivers remain in fleet
	if len(finalState.Resource().Drivers()) != 2 {
		t.Errorf("expected 2 drivers in final state, got %d", len(finalState.Resource().Drivers()))
	}
}

func TestTimeSteppingSimulator_ContextCancellation(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	res := model.NewResourceState(nil, nil)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, _ := model.NewState(res, info, belief)

	cfa := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	sim := service.NewTimeSteppingSimulator[model.Monopolistic](nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cfg := service.SimulationRunConfig{
		RunID:       "SIM-CANCEL",
		StartEpoch:  startEpoch,
		EndEpoch:    startEpoch + 100000,
		StepSeconds: 3600,
	}

	_, _, err := sim.Run(ctx, cfg, state, cfa, nil)
	if err == nil {
		t.Fatal("expected simulation error on cancelled context, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	_ = locChi
}

func TestTimeSteppingSimulator_ConcurrentParallelRuns(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	const numSimulations = 16
	var wg sync.WaitGroup

	for i := 0; i < numSimulations; i++ {
		wg.Add(1)
		go func(simID int) {
			defer wg.Done()

			drivers := []model.Driver{
				{ID: fmt.Sprintf("D-%d-01", simID), CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
			}
			loads := []model.Load{
				{
					ID:                  fmt.Sprintf("L-%d-01", simID),
					Origin:              locChi,
					Destination:         locAtl,
					RequiredEquipment:   model.EquipDryVan,
					Revenue:             3000.0,
					PickupEarliestEpoch: startEpoch,
					PickupLatestEpoch:   startEpoch + 36000,
					DeliveryLatestEpoch: startEpoch + 120000,
				},
			}

			res := model.NewResourceState(drivers, loads)
			info, err := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
			if err != nil {
				t.Errorf("sim %d: NewInformationState failed: %v", simID, err)
				return
			}
			belief := model.NewMonopolisticBelief()
			state, err := model.NewState(res, info, belief)
			if err != nil {
				t.Errorf("sim %d: NewState failed: %v", simID, err)
				return
			}

			cfa := policy.NewCFAPolicy[model.Monopolistic](
				policy.DefaultCFAParameters(),
				model.DefaultCostConfig(),
				model.DefaultFeasibilityConfig(),
				nil,
			)

			sim := service.NewTimeSteppingSimulator[model.Monopolistic](nil, nil)

			cfg := service.SimulationRunConfig{
				RunID:       fmt.Sprintf("MC-ROLLOUT-%02d", simID),
				StartEpoch:  startEpoch,
				EndEpoch:    startEpoch + 24*3600,
				StepSeconds: 4 * 3600,
			}

			summary, _, err := sim.Run(context.Background(), cfg, state, cfa, nil)
			if err != nil {
				t.Errorf("sim %d failed: %v", simID, err)
				return
			}
			if summary.KPIs.LoadsServiced != 1 {
				t.Errorf("sim %d: expected 1 load serviced, got %d", simID, summary.KPIs.LoadsServiced)
			}
		}(i)
	}

	wg.Wait()
}

func TestOptimizationService_CompetitiveBeliefUpdating(t *testing.T) {
	states := []string{"TightCapacity", "SurplusCapacity"}
	tm := model.NewIdentityTransitionMatrix(states)
	profiles := map[string]model.PostureObservationProfile{
		"TightCapacity": {
			ExpectedWinProbability: 0.20,
			ExpectedSpotRateMean:   3.00,
			ExpectedSpotRateStdDev: 0.20,
			ExpectedOffersMean:     10.0,
		},
		"SurplusCapacity": {
			ExpectedWinProbability: 0.80,
			ExpectedSpotRateMean:   1.80,
			ExpectedSpotRateStdDev: 0.20,
			ExpectedOffersMean:     2.0,
		},
	}
	om, err := model.NewMarketObservationModel(profiles)
	if err != nil {
		t.Fatalf("NewMarketObservationModel failed: %v", err)
	}

	scale := model.AggregatedMarket{LatentStates: states}
	filter, err := model.NewCompetitiveBeliefFilter(scale, tm, om)
	if err != nil {
		t.Fatalf("NewCompetitiveBeliefFilter failed: %v", err)
	}

	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}

	driver := model.Driver{
		ID:              "D-01",
		CurrentLocation: locChi,
		AvailableEpoch:  startEpoch,
		Equipment:       model.Equipment{Type: model.EquipDryVan},
	}
	res := model.NewResourceState([]model.Driver{driver}, nil)
	info, _ := model.NewInformationState(startEpoch, 2.50, 3.50, 0)
	priorBelief, _ := model.NewBelief(scale, states, []float64{0.5, 0.5})

	state, err := model.NewState(res, info, priorBelief)
	if err != nil {
		t.Fatalf("NewState failed: %v", err)
	}

	cfa := policy.NewCFAPolicy[model.AggregatedMarket](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	svc := service.NewOptimizationService[model.AggregatedMarket](nil, nil).WithBeliefFilter(filter)

	// Epoch 1: Introduce 10 new loads (aligns strongly with TightCapacity mean of 10 offers)
	var newLoads []model.Load
	for i := 0; i < 10; i++ {
		newLoads = append(newLoads, model.Load{
			ID:                  fmt.Sprintf("L-%02d", i),
			Origin:              locChi,
			Destination:         locAtl,
			RequiredEquipment:   model.EquipDryVan,
			PickupEarliestEpoch: startEpoch + 1800,
			PickupLatestEpoch:   startEpoch + 7200,
			DeliveryLatestEpoch: startEpoch + 28800,
			Revenue:             3.00 * 150.0,
		})
	}

	_, _, nextState, err := svc.OptimizeEpoch(context.Background(), state, cfa, startEpoch+3600, newLoads)
	if err != nil {
		t.Fatalf("OptimizeEpoch failed: %v", err)
	}

	// Verify that posterior belief on TightCapacity increased
	pTight := nextState.Belief().Probability("TightCapacity")
	t.Logf("Posterior Belief after TightCapacity observation: TightCapacity=%.4f, SurplusCapacity=%.4f",
		pTight, nextState.Belief().Probability("SurplusCapacity"))

	if pTight <= 0.5 {
		t.Errorf("expected belief on TightCapacity to increase > 0.5, got %f", pTight)
	}
}

func mathAbs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
