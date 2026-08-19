package policy_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/feasibility"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

func TestCalculateTripCost(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}

	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	driver := model.Driver{
		ID:              "D-01",
		CurrentLocation: locChi,
		AvailableEpoch:  startEpoch,
	}

	load := model.Load{
		ID:                  "L-01",
		Origin:              locChi,
		Destination:         locAtl,
		Revenue:             2000.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 14400,
		DeliveryLatestEpoch: startEpoch + 86400,
	}

	arc := feasibility.CandidateArc{
		DriverID:            "D-01",
		LoadID:              "L-01",
		DeadheadMiles:       20.0,
		LoadedMiles:         587.0,
		InsertedDwellMin:    120,                                  // 2 hours dwell
		DeliveryArrivalTime: time.Unix(startEpoch+90000, 0).UTC(), // 1 hour late
	}

	costCfg := model.CostConfig{
		FixedCostPerLoad:    50.0,
		LoadedMileRate:      2.00,
		EmptyMileRate:       1.50,
		EmptyToHomeRate:     1.00,
		EarlyArrivalPerHour: 20.0,
		LateDeliveryPerHour: 100.0,
		DriverBonusWeight:   1.0,
	}

	cost := policy.CalculateTripCost(driver, load, arc, costCfg)

	if cost.Revenue != 2000.0 {
		t.Fatalf("expected revenue 2000, got %f", cost.Revenue)
	}
	if cost.FixedCost != 50.0 {
		t.Fatalf("expected fixed cost 50, got %f", cost.FixedCost)
	}
	if cost.LoadedCost != 587.0*2.00 {
		t.Fatalf("expected loaded cost 1174, got %f", cost.LoadedCost)
	}
	if cost.EmptyCost != 20.0*1.50 {
		t.Fatalf("expected empty cost 30, got %f", cost.EmptyCost)
	}
	if cost.DwellCost != 2.0*20.0 { // 2 hours * 20 $/hr
		t.Fatalf("expected dwell cost 40, got %f", cost.DwellCost)
	}
	if cost.LatePenalty != 1.0*100.0 { // 1 hour late * 100 $/hr
		t.Fatalf("expected late penalty 100, got %f", cost.LatePenalty)
	}
	if cost.DriverBonus != 25.0 { // low deadhead bonus
		t.Fatalf("expected bonus 25, got %f", cost.DriverBonus)
	}

	expectedTotalCost := 50.0 + 1174.0 + 30.0 + cost.EmptyToHomeCost + 40.0 + 100.0 - 25.0
	if cost.TotalCost != expectedTotalCost {
		t.Fatalf("expected total cost %f, got %f", expectedTotalCost, cost.TotalCost)
	}
	if cost.NetContribution != 2000.0-expectedTotalCost {
		t.Fatalf("expected net contribution %f, got %f", 2000.0-expectedTotalCost, cost.NetContribution)
	}
}

func TestBipartiteMatcher_DeterministicMatching(t *testing.T) {
	evals := CandidateEvaluationFixture(
		// (DriverID, LoadID, TotalScore)
		[]struct {
			dID   string
			lID   string
			score float64
		}{
			{"D-01", "L-01", 500.0},
			{"D-01", "L-02", 700.0}, // D-01 prefers L-02
			{"D-02", "L-02", 800.0}, // D-02 wins L-02 (higher score)
			{"D-02", "L-01", 400.0},
			{"D-03", "L-03", -50.0}, // Negative score (should be pruned)
		},
	)

	matcher := policy.NewBipartiteMatcher()
	matches, _, totalObj, _ := matcher.SolveMatching(evals, 1000, false)

	// D-02 gets L-02 (score 800), D-01 gets L-01 (score 500), D-03 gets none
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].DriverID != "D-02" || matches[0].LoadID != "L-02" {
		t.Fatalf("expected D-02 -> L-02, got %s -> %s", matches[0].DriverID, matches[0].LoadID)
	}
	if matches[1].DriverID != "D-01" || matches[1].LoadID != "L-01" {
		t.Fatalf("expected D-01 -> L-01, got %s -> %s", matches[1].DriverID, matches[1].LoadID)
	}
	if totalObj != 1300.0 {
		t.Fatalf("expected total objective 1300, got %f", totalObj)
	}
}

func TestCFAPolicy_MonopolisticEvaluation(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locMil := model.Location{NodeID: "MIL", Lat: 43.0389, Lon: -87.9065}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}

	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
		{ID: "D-02", CurrentLocation: locMil, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
	}

	loads := []model.Load{
		{
			ID:                  "L-01",
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
		t.Fatalf("NewInformationState failed: %v", err)
	}
	belief := model.NewMonopolisticBelief()
	state, err := model.NewState(res, info, belief)
	if err != nil {
		t.Fatalf("NewState failed: %v", err)
	}

	costCfg := model.DefaultCostConfig()
	feasCfg := model.DefaultFeasibilityConfig()

	cfa := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		costCfg,
		feasCfg,
		nil,
	)

	action, provenance, err := cfa.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("CFAPolicy.Evaluate failed: %v", err)
	}

	if action.MatchCount() != 1 {
		t.Fatalf("expected 1 match, got %d", action.MatchCount())
	}

	// Chicago driver D-01 should win L-01 because deadhead is 0 vs Milwaukee D-02 deadhead ~90 miles
	match := action.Matches()[0]
	if match.DriverID != "D-01" || match.LoadID != "L-01" {
		t.Fatalf("expected D-01 -> L-01, got %s -> %s", match.DriverID, match.LoadID)
	}

	if provenance.MatchedCount != 1 || provenance.TotalNetContribution <= 0 {
		t.Fatalf("invalid provenance record: %+v", provenance)
	}
}

func TestVFAPolicy_RegionalBiasing(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locDal := model.Location{NodeID: "DAL", Lat: 32.7767, Lon: -96.7970}

	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
	}

	// Two identical revenue/distance loads, one to Atlanta, one to Dallas
	loads := []model.Load{
		{
			ID:                  "L-ATL",
			Origin:              locChi,
			Destination:         locAtl,
			RequiredEquipment:   model.EquipDryVan,
			Revenue:             3500.0,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 36000,
			DeliveryLatestEpoch: startEpoch + 120000,
		},
		{
			ID:                  "L-DAL",
			Origin:              locChi,
			Destination:         locDal,
			RequiredEquipment:   model.EquipDryVan,
			Revenue:             3500.0,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 36000,
			DeliveryLatestEpoch: startEpoch + 120000,
		},
	}

	rm := model.NewRegionManager(1.0, nil)
	atlRegion := rm.GetRegionID(locAtl)
	dalRegion := rm.GetRegionID(locDal)

	// VFA table gives Dallas destination a +$1500 bonus to overcome mileage difference
	vfaTable := policy.NewVFATable(map[string]float64{
		atlRegion: 50.0,
		dalRegion: 1500.0,
	})

	res := model.NewResourceState(drivers, loads)
	info, err := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	if err != nil {
		t.Fatalf("NewInformationState failed: %v", err)
	}
	belief := model.NewMonopolisticBelief()
	state, err := model.NewState(res, info, belief)
	if err != nil {
		t.Fatalf("NewState failed: %v", err)
	}

	vfaPolicy := policy.NewVFAPolicy[model.Monopolistic](
		vfaTable,
		1.0,
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		rm,
	)

	action, _, err := vfaPolicy.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("VFAPolicy.Evaluate failed: %v", err)
	}

	if action.MatchCount() != 1 {
		t.Fatalf("expected 1 match, got %d", action.MatchCount())
	}

	// L-DAL should be selected due to higher post-decision state value
	if match := action.Matches()[0]; match.LoadID != "L-DAL" {
		t.Fatalf("expected VFA to choose L-DAL, got %s", match.LoadID)
	}
}

func TestVFATable_Immutability(t *testing.T) {
	initial := map[string]float64{"REG_A": 100.0}
	table1 := policy.NewVFATable(initial)

	table2 := table1.WithUpdatedValue("REG_B", 200.0)

	if table1.GetValue("REG_B") != 0.0 {
		t.Fatalf("table1 was mutated by table2 update")
	}
	if table2.GetValue("REG_B") != 200.0 {
		t.Fatalf("table2 did not retain updated value")
	}
}

func TestPolicy_ConcurrentParallelEvaluations(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	const numDrivers = 20
	const numLoads = 20

	drivers := make([]model.Driver, numDrivers)
	for i := 0; i < numDrivers; i++ {
		drivers[i] = model.Driver{
			ID:              fmt.Sprintf("D-%02d", i),
			CurrentLocation: locChi,
			AvailableEpoch:  startEpoch,
			Equipment:       model.Equipment{Type: model.EquipDryVan},
		}
	}

	loads := make([]model.Load, numLoads)
	for j := 0; j < numLoads; j++ {
		loads[j] = model.Load{
			ID:                  fmt.Sprintf("L-%02d", j),
			Origin:              locChi,
			Destination:         locAtl,
			RequiredEquipment:   model.EquipDryVan,
			Revenue:             3000.0,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 36000,
			DeliveryLatestEpoch: startEpoch + 120000,
		}
	}

	res := model.NewResourceState(drivers, loads)
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

	// Run 32 concurrent goroutines evaluating the policy simultaneously under -race
	var wg sync.WaitGroup
	const goroutines = 32

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			action, _, err := cfa.Evaluate(context.Background(), state)
			if err != nil {
				t.Errorf("goroutine %d evaluation failed: %v", gID, err)
				return
			}
			if action.MatchCount() != numDrivers {
				t.Errorf("goroutine %d: expected %d matches, got %d", gID, numDrivers, action.MatchCount())
			}
		}(g)
	}

	wg.Wait()
}

func CandidateEvaluationFixture(fixtures []struct {
	dID   string
	lID   string
	score float64
}) []policy.CandidateEvaluation {
	evals := make([]policy.CandidateEvaluation, len(fixtures))
	for i, f := range fixtures {
		evals[i] = policy.CandidateEvaluation{
			DriverID: f.dID,
			LoadID:   f.lID,
			CostBreakdown: policy.TripCostBreakdown{
				NetContribution: f.score,
			},
			TotalScore: f.score,
		}
	}
	return evals
}
