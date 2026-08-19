package policy_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

func TestPolicyCornerCases_NegativePayoutRejection(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	// Unprofitable load: $50 revenue on a 587-mile trip (operating cost ~$1,000)
	driver := model.Driver{
		ID:                  "D-01",
		CurrentLocation:     locChi,
		HomeLocation:        locChi,
		AvailableEpoch:      startEpoch,
		DriveHoursRemaining: 11.0,
		DutyHoursRemaining:  14.0,
		Equipment:           model.Equipment{Type: model.EquipDryVan},
	}
	unprofitableLoad := model.Load{
		ID:                  "L-UNPROFITABLE",
		Origin:              locChi,
		Destination:         locAtl,
		Revenue:             50.0,
		RequiredEquipment:   model.EquipDryVan,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 3600,
		DeliveryLatestEpoch: startEpoch + 48*3600,
	}

	res := model.NewResourceState([]model.Driver{driver}, []model.Load{unprofitableLoad})
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	state, _ := model.NewState(res, info, model.NewMonopolisticBelief())

	cfa := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	action, prov, err := cfa.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("CFA evaluate failed: %v", err)
	}

	// Optimizer must reject negative match: 0 matches, 0 cost, 0 net contribution
	if len(action.Matches()) != 0 {
		t.Errorf("expected 0 matches for unprofitable load, got %d (payout: $%.2f)",
			len(action.Matches()), prov.TotalNetContribution)
	}
	if prov.TotalNetContribution != 0.0 {
		t.Errorf("expected 0.0 net contribution when idle, got %f", prov.TotalNetContribution)
	}
}

func TestPolicyCornerCases_SevereShortageGlobalMaxYield(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	// 5 Drivers vs 200 competing Loads
	numDrivers := 5
	numLoads := 200

	drivers := make([]model.Driver, numDrivers)
	for i := 0; i < numDrivers; i++ {
		drivers[i] = model.Driver{
			ID:                  fmt.Sprintf("D-%02d", i),
			CurrentLocation:     locChi,
			HomeLocation:        locChi,
			AvailableEpoch:      startEpoch,
			DriveHoursRemaining: 11.0,
			DutyHoursRemaining:  14.0,
			Equipment:           model.Equipment{Type: model.EquipDryVan},
		}
	}

	loads := make([]model.Load, numLoads)
	for i := 0; i < numLoads; i++ {
		loads[i] = model.Load{
			ID:                  fmt.Sprintf("LOAD-%03d", i),
			Origin:              locChi,
			Destination:         locAtl,
			Revenue:             float64(1500 + i*20), // Strictly increasing revenues
			RequiredEquipment:   model.EquipDryVan,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 3600,
			DeliveryLatestEpoch: startEpoch + 48*3600,
		}
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	state, _ := model.NewState(res, info, model.NewMonopolisticBelief())

	cfa := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	action, prov, err := cfa.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("CFA evaluate failed: %v", err)
	}

	// Must match exactly 5 loads (all drivers utilized)
	if len(action.Matches()) != numDrivers {
		t.Fatalf("expected exactly %d matches, got %d", numDrivers, len(action.Matches()))
	}

	// Must select the top 5 highest-revenue loads: LOAD-199, LOAD-198, LOAD-197, LOAD-196, LOAD-195
	matchedLoadIDs := make(map[string]bool)
	for _, m := range action.Matches() {
		matchedLoadIDs[m.LoadID] = true
	}

	for i := numLoads - numDrivers; i < numLoads; i++ {
		expectedID := fmt.Sprintf("LOAD-%03d", i)
		if !matchedLoadIDs[expectedID] {
			t.Errorf("Optimizer failed to pick highest-yield load %s (picked: %v)", expectedID, matchedLoadIDs)
		}
	}

	t.Logf("Shortage optimization selected top 5 loads with Total Net Contribution: $%.2f",
		prov.TotalNetContribution)
}

func TestPolicyCornerCases_DeterministicSymmetryTieBreaking(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	// 4 Identical drivers and 4 identical loads with identical revenues
	drivers := []model.Driver{
		{ID: "D-03", CurrentLocation: locChi, HomeLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
		{ID: "D-01", CurrentLocation: locChi, HomeLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
		{ID: "D-04", CurrentLocation: locChi, HomeLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
		{ID: "D-02", CurrentLocation: locChi, HomeLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
	}
	loads := []model.Load{
		{ID: "L-04", Origin: locChi, Destination: locAtl, Revenue: 3000.0, RequiredEquipment: model.EquipDryVan, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 3600, DeliveryLatestEpoch: startEpoch + 48*3600},
		{ID: "L-02", Origin: locChi, Destination: locAtl, Revenue: 3000.0, RequiredEquipment: model.EquipDryVan, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 3600, DeliveryLatestEpoch: startEpoch + 48*3600},
		{ID: "L-01", Origin: locChi, Destination: locAtl, Revenue: 3000.0, RequiredEquipment: model.EquipDryVan, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 3600, DeliveryLatestEpoch: startEpoch + 48*3600},
		{ID: "L-03", Origin: locChi, Destination: locAtl, Revenue: 3000.0, RequiredEquipment: model.EquipDryVan, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 3600, DeliveryLatestEpoch: startEpoch + 48*3600},
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	state, _ := model.NewState(res, info, model.NewMonopolisticBelief())

	cfa := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	// Run 20 evaluations and assert bit-wise identical match tuples
	var canonicalMatches []model.DriverLoadMatch
	for run := 0; run < 20; run++ {
		action, _, err := cfa.Evaluate(context.Background(), state)
		if err != nil {
			t.Fatalf("Run %d failed: %v", run, err)
		}

		matches := action.Matches()
		if run == 0 {
			canonicalMatches = matches
			// Verify canonical pairing: D-01->L-01, D-02->L-02, D-03->L-03, D-04->L-04
			for i, m := range matches {
				expectedDriver := fmt.Sprintf("D-%02d", i+1)
				expectedLoad := fmt.Sprintf("L-%02d", i+1)
				if m.DriverID != expectedDriver || m.LoadID != expectedLoad {
					t.Errorf("Canonical pairing mismatch at index %d: got (%s, %s), expected (%s, %s)",
						i, m.DriverID, m.LoadID, expectedDriver, expectedLoad)
				}
			}
		} else {
			if len(matches) != len(canonicalMatches) {
				t.Fatalf("Run %d match count mismatch: %d vs %d", run, len(matches), len(canonicalMatches))
			}
			for i := range matches {
				if matches[i] != canonicalMatches[i] {
					t.Fatalf("Run %d determinism broken: match %d is %v vs %v",
						run, i, matches[i], canonicalMatches[i])
				}
			}
		}
	}
}
