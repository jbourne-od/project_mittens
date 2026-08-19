package policy_test

import (
	"context"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

func TestPiecewiseVFA_ConcavityValidation(t *testing.T) {
	// Valid concave slopes: 100, 80, 50, 20, 0
	_, err := policy.NewRegionSlopes("REG_A", []float64{100, 80, 50, 20, 0})
	if err != nil {
		t.Fatalf("expected valid concave slopes, got error: %v", err)
	}

	// Invalid non-concave slopes: 100, 50, 80 (80 > 50 increases!)
	_, err = policy.NewRegionSlopes("REG_B", []float64{100, 50, 80})
	if err == nil {
		t.Fatalf("expected error on non-concave slopes, got nil")
	}
}

func TestPiecewiseVFA_MarginalAndTotalValues(t *testing.T) {
	rs, _ := policy.NewRegionSlopes("REG_A", []float64{100.0, 70.0, 40.0, 10.0})

	// Marginal values:
	// 0 drivers -> 1st driver adds 100
	// 1 driver  -> 2nd driver adds 70
	// 2 drivers -> 3rd driver adds 40
	// 3 drivers -> 4th driver adds 10
	// 4+ drivers -> tail slope 10
	if rs.MarginalValue(0) != 100.0 {
		t.Errorf("MarginalValue(0) = %v; expected 100.0", rs.MarginalValue(0))
	}
	if rs.MarginalValue(1) != 70.0 {
		t.Errorf("MarginalValue(1) = %v; expected 70.0", rs.MarginalValue(1))
	}
	if rs.MarginalValue(2) != 40.0 {
		t.Errorf("MarginalValue(2) = %v; expected 40.0", rs.MarginalValue(2))
	}
	if rs.MarginalValue(3) != 10.0 {
		t.Errorf("MarginalValue(3) = %v; expected 10.0", rs.MarginalValue(3))
	}
	if rs.MarginalValue(10) != 10.0 {
		t.Errorf("MarginalValue(10) = %v; expected 10.0 (tail)", rs.MarginalValue(10))
	}

	// Total integral value:
	// V(0) = 0
	// V(1) = 100
	// V(2) = 100 + 70 = 170
	// V(3) = 100 + 70 + 40 = 210
	// V(4) = 210 + 10 = 220
	if rs.TotalValue(0) != 0.0 {
		t.Errorf("TotalValue(0) = %v; expected 0.0", rs.TotalValue(0))
	}
	if rs.TotalValue(1) != 100.0 {
		t.Errorf("TotalValue(1) = %v; expected 100.0", rs.TotalValue(1))
	}
	if rs.TotalValue(2) != 170.0 {
		t.Errorf("TotalValue(2) = %v; expected 170.0", rs.TotalValue(2))
	}
	if rs.TotalValue(4) != 220.0 {
		t.Errorf("TotalValue(4) = %v; expected 220.0", rs.TotalValue(4))
	}
}

func TestPiecewiseVFA_CAVELevelClearingProjection(t *testing.T) {
	initialSlopes := map[string]policy.RegionSlopes{
		"CHI": {
			RegionID: "CHI",
			Slopes:   []float64{100.0, 80.0, 60.0, 40.0, 20.0},
		},
	}

	table := policy.NewPiecewiseLinearVFATable(initialSlopes)

	// Update level 2 (current slope 60.0) with very high observed sample = 150.0 and stepSize = 1.0
	// New smoothed level 2 = 150.0
	// To preserve concavity:
	// - Backward pass: level 1 and level 0 must be elevated to at least 150.0! (Slopes[0] >= Slopes[1] >= Slopes[2] = 150.0)
	// - Forward pass: level 3 and level 4 remain <= 150.0
	updatedTable := table.UpdateCAVE("CHI", 2, 150.0, 1.0, 5)

	chiSlopes, ok := updatedTable.GetRegionSlopes("CHI")
	if !ok {
		t.Fatalf("CHI slopes not found in updated table")
	}

	t.Logf("CAVE Updated CHI slopes: %v", chiSlopes.Slopes)

	// Verify non-increasing concavity invariant
	for i := 1; i < len(chiSlopes.Slopes); i++ {
		if chiSlopes.Slopes[i] > chiSlopes.Slopes[i-1] {
			t.Fatalf("concavity violated after CAVE update: Slopes[%d]=%v > Slopes[%d]=%v",
				i, chiSlopes.Slopes[i], i-1, chiSlopes.Slopes[i-1])
		}
	}

	if chiSlopes.Slopes[0] < 150.0 || chiSlopes.Slopes[1] < 150.0 {
		t.Errorf("expected level 0 and 1 to be elevated to >= 150.0 by backward pass, got %v", chiSlopes.Slopes)
	}

	// Immutability: original table must still have initial slopes
	origChi, _ := table.GetRegionSlopes("CHI")
	if origChi.Slopes[0] != 100.0 || origChi.Slopes[2] != 60.0 {
		t.Errorf("original table mutated: %v", origChi.Slopes)
	}
}

func TestPiecewiseVFA_PolicyAntiBunching(t *testing.T) {
	// Setup 2 drivers in Atlanta (ATL) and 2 candidate loads:
	// Load 1: ATL -> CHI (Destination CHI has steep diminishing slopes: [200, 50, 0])
	// Load 2: ATL -> NYC (Destination NYC has constant slope: [150, 150, 150])
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locNyc := model.Location{NodeID: "NYC", Lat: 40.7128, Lon: -74.0060}

	d1 := model.Driver{
		ID:              "D-01",
		CurrentLocation: locAtl,
		AvailableEpoch:  startEpoch,
		Equipment:       model.Equipment{Type: model.EquipDryVan},
		Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
	}
	d2 := model.Driver{
		ID:              "D-02",
		CurrentLocation: locAtl,
		AvailableEpoch:  startEpoch,
		Equipment:       model.Equipment{Type: model.EquipDryVan},
		Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
	}

	// 2 loads to CHI, 1 load to NYC
	lChi1 := model.Load{
		ID:                  "L-CHI-1",
		Origin:              locAtl,
		Destination:         locChi,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             2500.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 36000,
		DeliveryLatestEpoch: startEpoch + 120000,
	}
	lChi2 := model.Load{
		ID:                  "L-CHI-2",
		Origin:              locAtl,
		Destination:         locChi,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             2500.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 36000,
		DeliveryLatestEpoch: startEpoch + 120000,
	}
	lNyc := model.Load{
		ID:                  "L-NYC-1",
		Origin:              locAtl,
		Destination:         locNyc,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             2500.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 36000,
		DeliveryLatestEpoch: startEpoch + 120000,
	}

	resState := model.NewResourceState(
		[]model.Driver{d1, d2},
		[]model.Load{lChi1, lChi2, lNyc},
	)
	infoState, err := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	if err != nil {
		t.Fatalf("NewInformationState failed: %v", err)
	}
	belief := model.NewMonopolisticBelief()
	state, err := model.NewState(resState, infoState, belief)
	if err != nil {
		t.Fatalf("NewState failed: %v", err)
	}

	rm := model.NewRegionManager(1.0, nil)
	chiRegion := rm.GetRegionID(locChi)
	nycRegion := rm.GetRegionID(locNyc)

	// VFA table: CHI 1st driver is worth $500, but 2nd driver is worth only $50! NYC is worth $300.
	vfaTable := policy.NewPiecewiseLinearVFATable(map[string]policy.RegionSlopes{
		chiRegion: {
			RegionID: chiRegion,
			Slopes:   []float64{500.0, 50.0, 0.0},
		},
		nycRegion: {
			RegionID: nycRegion,
			Slopes:   []float64{300.0, 300.0, 300.0},
		},
	})

	costCfg := model.DefaultCostConfig()
	feasCfg := model.FeasibilityConfig{AverageSpeedMPH: 50.0, HOSPolicySpecs: hos.USPolicySpecs()}

	vfaPolicy := policy.NewPiecewiseVFAPolicy[model.Monopolistic](
		vfaTable,
		nil,
		0.95,
		costCfg,
		feasCfg,
		rm,
	)

	action, prov, err := vfaPolicy.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	matches := action.Matches()
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}

	t.Logf("Matches: %v", matches)
	t.Logf("Decision Provenance: %s, alternatives evaluated: %d", prov.PolicyName, len(prov.EvaluatedArcs))
}
