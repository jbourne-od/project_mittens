package policy_test

import (
	"context"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

func TestRelay_FeasibleCoordinatedExchange(t *testing.T) {
	// Corridor: Chicago (CHI) -> Atlanta (ATL), ~588 miles
	// Intermediate Relay Hub: Louisville (SDF), ~295 miles from CHI, ~320 miles from ATL
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locSdf := model.Location{NodeID: "SDF", Lat: 38.2527, Lon: -85.7585}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locBna := model.Location{NodeID: "BNA", Lat: 36.1627, Lon: -86.7816} // Nashville (~175 miles to Louisville)

	startEpoch := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	// Inbound driver in Chicago
	dIn := model.Driver{
		ID:              "D-IN-CHI",
		CurrentLocation: locChi,
		AvailableEpoch:  startEpoch,
		Equipment:       model.Equipment{Type: model.EquipDryVan},
		Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
	}

	// Outbound driver in Nashville
	dOut := model.Driver{
		ID:              "D-OUT-BNA",
		CurrentLocation: locBna,
		AvailableEpoch:  startEpoch,
		Equipment:       model.Equipment{Type: model.EquipDryVan},
		Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
	}

	// Long-haul load from Chicago to Atlanta
	load := model.Load{
		ID:                  "L-CHI-ATL",
		Origin:              locChi,
		Destination:         locAtl,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             3800.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 36000,
		DeliveryLatestEpoch: startEpoch + 120000,
	}

	relayFac := model.Facility{
		ID:                  "FAC-SDF-HUB",
		Name:                "Louisville Relay Exchange Terminal",
		Location:            locSdf,
		Type:                model.FacilityRelayHub,
		OpenMinutesOfDay:    0,
		CloseMinutesOfDay:   1440, // 24/7
		AverageDwellMinutes: 60,
	}

	costCfg := model.CostConfig{
		FixedCostPerLoad: 50.0,
		LoadedMileRate:   1.50,
		EmptyMileRate:    1.20,
	}
	relayCfg := policy.DefaultRelayConfig()
	specs := hos.USPolicySpecs()

	exchange, err := policy.EvaluateRelayFeasibility(
		dIn, dOut, load, relayFac,
		costCfg, relayCfg, specs,
	)
	if err != nil {
		t.Fatalf("expected feasible relay exchange, got error: %v", err)
	}

	t.Logf("Relay Exchange Synthesized for Load %s:", exchange.LoadID)
	t.Logf("  Inbound Driver: %s (Miles: %.1f loaded, %.1f empty, EndEpoch: %d)",
		exchange.InboundSegment.DriverID, exchange.InboundSegment.LoadedMiles,
		exchange.InboundSegment.DeadheadMiles, exchange.InboundSegment.EndEpoch)
	t.Logf("  Outbound Driver: %s (Miles: %.1f loaded, %.1f empty, EndEpoch: %d)",
		exchange.OutboundSegment.DriverID, exchange.OutboundSegment.LoadedMiles,
		exchange.OutboundSegment.DeadheadMiles, exchange.OutboundSegment.EndEpoch)
	t.Logf("  Economics: Revenue $%.2f, Total Cost $%.2f, Net Contribution $%.2f",
		exchange.TotalRevenue, exchange.TotalCost, exchange.NetContribution)

	if exchange.NetContribution <= 0 {
		t.Errorf("expected positive net contribution, got $%.2f", exchange.NetContribution)
	}

	if exchange.InboundSegment.ResultingClocks.RemainingDrivingMin() < 0 {
		t.Errorf("inbound driver driving remaining negative: %v", exchange.InboundSegment.ResultingClocks.RemainingDrivingMin())
	}
	if exchange.OutboundSegment.ResultingClocks.RemainingDrivingMin() < 0 {
		t.Errorf("outbound driver driving remaining negative: %v", exchange.OutboundSegment.ResultingClocks.RemainingDrivingMin())
	}
}

func TestRelay_FailClosedInfeasibilities(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locSdf := model.Location{NodeID: "SDF", Lat: 38.2527, Lon: -85.7585}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}

	startEpoch := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	dIn := model.Driver{
		ID:              "D-01",
		CurrentLocation: locChi,
		AvailableEpoch:  startEpoch,
		Equipment:       model.Equipment{Type: model.EquipDryVan},
	}
	dOut := model.Driver{
		ID:              "D-02",
		CurrentLocation: locSdf,
		AvailableEpoch:  startEpoch,
		Equipment:       model.Equipment{Type: model.EquipFlatbed}, // Equipment mismatch (Flatbed vs Reefer)
	}

	loadReefer := model.Load{
		ID:                  "L-REEFER",
		Origin:              locChi,
		Destination:         locAtl,
		RequiredEquipment:   model.EquipReefer,
		Revenue:             4000.0,
		PickupEarliestEpoch: startEpoch,
		DeliveryLatestEpoch: startEpoch + 86400,
	}

	relayFac := model.Facility{
		ID:       "FAC-SDF",
		Location: locSdf,
		Type:     model.FacilityRelayHub,
	}

	costCfg := model.DefaultCostConfig()
	relayCfg := policy.DefaultRelayConfig()
	specs := hos.USPolicySpecs()

	// 1. Equipment mismatch test
	_, err := policy.EvaluateRelayFeasibility(dIn, dOut, loadReefer, relayFac, costCfg, relayCfg, specs)
	if err == nil {
		t.Fatalf("expected error on equipment mismatch, got nil")
	}

	// 2. Same driver test
	_, err = policy.EvaluateRelayFeasibility(dIn, dIn, loadReefer, relayFac, costCfg, relayCfg, specs)
	if err == nil {
		t.Fatalf("expected error on same driver, got nil")
	}
}

func TestRelaySynthesizer_FleetOptimization(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locSdf := model.Location{NodeID: "SDF", Lat: 38.2527, Lon: -85.7585}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locBna := model.Location{NodeID: "BNA", Lat: 36.1627, Lon: -86.7816}

	startEpoch := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{
			ID:              "D-CHI",
			CurrentLocation: locChi,
			AvailableEpoch:  startEpoch,
			Equipment:       model.Equipment{Type: model.EquipDryVan},
			Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
		{
			ID:              "D-BNA",
			CurrentLocation: locBna,
			AvailableEpoch:  startEpoch,
			Equipment:       model.Equipment{Type: model.EquipDryVan},
			Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
	}

	loads := []model.Load{
		{
			ID:                  "L-CHI-ATL-LONG",
			Origin:              locChi,
			Destination:         locAtl,
			RequiredEquipment:   model.EquipDryVan,
			Revenue:             3500.0,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 36000,
			DeliveryLatestEpoch: startEpoch + 120000,
		},
	}

	facilities := []model.Facility{
		{
			ID:                  "FAC-SDF-HUB",
			Name:                "Louisville Relay Hub",
			Location:            locSdf,
			Type:                model.FacilityRelayHub,
			OpenMinutesOfDay:    0,
			CloseMinutesOfDay:   1440,
			AverageDwellMinutes: 60,
		},
	}

	facStore := model.NewFacilityStore(facilities)

	synthesizer := policy.NewRelaySynthesizer(
		model.DefaultCostConfig(),
		policy.DefaultRelayConfig(),
		hos.USPolicySpecs(),
		facStore,
		nil,
	)

	exchanges, err := synthesizer.SynthesizeRelays(context.Background(), drivers, loads, 450.0)
	if err != nil {
		t.Fatalf("SynthesizeRelays failed: %v", err)
	}

	if len(exchanges) != 1 {
		t.Fatalf("expected 1 relay exchange synthesized, got %d", len(exchanges))
	}

	ex := exchanges[0]
	if ex.LoadID != "L-CHI-ATL-LONG" {
		t.Errorf("expected load L-CHI-ATL-LONG, got %s", ex.LoadID)
	}
	if ex.InboundSegment.DriverID != "D-CHI" || ex.OutboundSegment.DriverID != "D-BNA" {
		t.Errorf("expected D-CHI -> D-BNA handoff, got %s -> %s",
			ex.InboundSegment.DriverID, ex.OutboundSegment.DriverID)
	}
}
