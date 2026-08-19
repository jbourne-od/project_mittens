package policy_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

func TestDLAPolicy_LookaheadAvoidsDeadheadTrap(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locGary := model.Location{NodeID: "GARY", Lat: 41.5934, Lon: -87.3464}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locDal := model.Location{NodeID: "DAL", Lat: 32.7767, Lon: -96.7970}

	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	driver := model.Driver{
		ID:              "D-01",
		CurrentLocation: locChi,
		AvailableEpoch:  startEpoch,
		Equipment:       model.Equipment{Type: model.EquipDryVan},
	}

	// Load 1 (Deadhead Trap): Short haul with high immediate profit, but ends in Gary (no return loads)
	loadTrap := model.Load{
		ID:                  "L-TRAP",
		Origin:              locChi,
		Destination:         locGary,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             1500.0, // High revenue for short trip (~30 miles)
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 36000,
		DeliveryLatestEpoch: startEpoch + 86400,
	}

	// Load 2 (Gateway): Linehaul to Atlanta with moderate immediate profit
	loadGateway := model.Load{
		ID:                  "L-GATEWAY",
		Origin:              locChi,
		Destination:         locAtl,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             1800.0, // (~587 miles, net profit ~$600)
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 36000,
		DeliveryLatestEpoch: startEpoch + 86400,
	}

	res := model.NewResourceState([]model.Driver{driver}, []model.Load{loadTrap, loadGateway})
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, err := model.NewState(res, info, belief)
	if err != nil {
		t.Fatalf("NewState failed: %v", err)
	}

	costCfg := model.CostConfig{
		FixedCostPerLoad: 50.0,
		LoadedMileRate:   2.00,
		EmptyMileRate:    1.50,
	}

	feasCfg := model.DefaultFeasibilityConfig()

	// Base policy: CFA
	cfa := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		costCfg,
		feasCfg,
		nil,
	)

	// First verify that myopic CFA prefers the trap load
	cfaAction, _, err := cfa.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("CFA evaluate failed: %v", err)
	}
	if len(cfaAction.Matches()) != 1 || cfaAction.Matches()[0].LoadID != "L-TRAP" {
		t.Fatalf("expected myopic CFA to pick L-TRAP, got %v", cfaAction.Matches())
	}

	// Dynamic Arrival Sampler: Atlanta gets a lucrative $4000 load to Dallas in lookahead step 1; Gary gets nothing
	sampler := func(epoch int64, stepIndex int, rng *pkgmath.RNG) []model.Load {
		if stepIndex == 1 {
			return []model.Load{
				{
					ID:                  "L-DOWNSTREAM-ATL-DAL",
					Origin:              locAtl,
					Destination:         locDal,
					RequiredEquipment:   model.EquipDryVan,
					Revenue:             4000.0,
					PickupEarliestEpoch: epoch,
					PickupLatestEpoch:   epoch + 36000,
					DeliveryLatestEpoch: epoch + 2*86400,
				},
			}
		}
		return nil
	}

	dlaParams := policy.DLAParameters{
		Horizon:               1,
		NumRollouts:           1,
		DiscountFactor:        0.95,
		MaxConcurrentBranches: 4,
		StepSeconds:           10800,
		RandomSeed:            42,
	}

	dla := policy.NewDLAPolicy[model.Monopolistic](
		dlaParams,
		costCfg,
		feasCfg,
		cfa,
		sampler,
		nil,
		nil,
		nil,
	)

	// DLA should recognize the massive downstream value in Atlanta and pick L-GATEWAY!
	dlaAction, dlaProv, err := dla.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("DLA evaluate failed: %v", err)
	}

	if len(dlaAction.Matches()) != 1 {
		t.Fatalf("expected 1 match, got %d", len(dlaAction.Matches()))
	}
	if dlaAction.Matches()[0].LoadID != "L-GATEWAY" {
		t.Errorf("expected DLA to choose L-GATEWAY over L-TRAP due to lookahead valuation, got %s", dlaAction.Matches()[0].LoadID)
	}

	// Verify provenance captured DLA value
	for _, arc := range dlaProv.EvaluatedArcs {
		if arc.LoadID == "L-GATEWAY" {
			if arc.DLAValue <= 0 {
				t.Errorf("expected positive DLA value for gateway arc, got %f", arc.DLAValue)
			}
		}
	}
}

func TestDLAPolicy_DeterministicReproducibility(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
		{ID: "D-02", CurrentLocation: locAtl, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
	}
	loads := []model.Load{
		{ID: "L-01", Origin: locChi, Destination: locAtl, Revenue: 2000.0, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 36000, DeliveryLatestEpoch: startEpoch + 86400},
		{ID: "L-02", Origin: locAtl, Destination: locChi, Revenue: 2200.0, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 36000, DeliveryLatestEpoch: startEpoch + 86400},
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	state, _ := model.NewState(res, info, model.NewMonopolisticBelief())

	costCfg := model.DefaultCostConfig()
	feasCfg := model.DefaultFeasibilityConfig()
	cfa := policy.NewCFAPolicy[model.Monopolistic](policy.DefaultCFAParameters(), costCfg, feasCfg, nil)

	dla := policy.NewDLAPolicy[model.Monopolistic](
		policy.DefaultDLAParameters(),
		costCfg,
		feasCfg,
		cfa,
		nil,
		nil,
		nil,
		nil,
	)

	// Run 1
	action1, prov1, err1 := dla.Evaluate(context.Background(), state)
	if err1 != nil {
		t.Fatalf("Run 1 failed: %v", err1)
	}

	// Run 2
	action2, prov2, err2 := dla.Evaluate(context.Background(), state)
	if err2 != nil {
		t.Fatalf("Run 2 failed: %v", err2)
	}

	if len(action1.Matches()) != len(action2.Matches()) {
		t.Fatalf("match count mismatch: %d vs %d", len(action1.Matches()), len(action2.Matches()))
	}
	for i := range action1.Matches() {
		if action1.Matches()[i] != action2.Matches()[i] {
			t.Errorf("match %d mismatch: %+v vs %+v", i, action1.Matches()[i], action2.Matches()[i])
		}
	}
	if prov1.TotalObjectiveValue != prov2.TotalObjectiveValue {
		t.Errorf("objective mismatch: %f vs %f", prov1.TotalObjectiveValue, prov2.TotalObjectiveValue)
	}
}

func TestDLAPolicy_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	driver := model.Driver{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: 1000}
	load := model.Load{ID: "L-01", Origin: locChi, Destination: locChi, Revenue: 1000, PickupEarliestEpoch: 1000, PickupLatestEpoch: 2000, DeliveryLatestEpoch: 3000}

	res := model.NewResourceState([]model.Driver{driver}, []model.Load{load})
	info, _ := model.NewInformationState(1000, 1.0, 2.50, 0)
	state, _ := model.NewState(res, info, model.NewMonopolisticBelief())

	dla := policy.NewDLAPolicy[model.Monopolistic](
		policy.DefaultDLAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	_, _, err := dla.Evaluate(ctx, state)
	if err == nil {
		t.Errorf("expected context cancellation error, got nil")
	}
}

func TestDLAPolicy_ConcurrentParallelEvaluations(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
		{ID: "D-02", CurrentLocation: locAtl, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
	}
	loads := []model.Load{
		{ID: "L-01", Origin: locChi, Destination: locAtl, Revenue: 2000.0, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 36000, DeliveryLatestEpoch: startEpoch + 86400},
		{ID: "L-02", Origin: locAtl, Destination: locChi, Revenue: 2200.0, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 36000, DeliveryLatestEpoch: startEpoch + 86400},
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	state, _ := model.NewState(res, info, model.NewMonopolisticBelief())

	costCfg := model.DefaultCostConfig()
	feasCfg := model.DefaultFeasibilityConfig()
	cfa := policy.NewCFAPolicy[model.Monopolistic](policy.DefaultCFAParameters(), costCfg, feasCfg, nil)

	dla := policy.NewDLAPolicy[model.Monopolistic](
		policy.DefaultDLAParameters(),
		costCfg,
		feasCfg,
		cfa,
		nil,
		nil,
		nil,
		nil,
	)

	const goroutines = 16
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			action, prov, err := dla.Evaluate(context.Background(), state)
			if err != nil {
				t.Errorf("goroutine %d failed: %v", gID, err)
				return
			}
			if len(action.Matches()) != 2 {
				t.Errorf("goroutine %d expected 2 matches, got %d", gID, len(action.Matches()))
			}
			_ = prov
		}(g)
	}

	wg.Wait()
}
