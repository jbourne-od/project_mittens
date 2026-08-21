package dispatch_test

import (
	"context"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/internal/service/dispatch"
)

func TestRelayDispatchRunner_CombinedSynthesis(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locSdf := model.Location{NodeID: "SDF", Lat: 38.2527, Lon: -85.7585}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locBna := model.Location{NodeID: "BNA", Lat: 36.1627, Lon: -86.7816}
	locMke := model.Location{NodeID: "MKE", Lat: 43.0389, Lon: -87.9065}

	startEpoch := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	// 3 drivers:
	// - D-CHI & D-BNA will execute a relay on a long-haul CHI->ATL load via SDF
	// - D-MKE will execute a regional direct tour CHI->MKE
	drivers := []model.Driver{
		{
			ID:              "D-CHI",
			CurrentLocation: locChi,
			HomeLocation:    locChi,
			AvailableEpoch:  startEpoch,
			Equipment:       model.Equipment{Type: model.EquipDryVan},
			Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
		{
			ID:              "D-BNA",
			CurrentLocation: locBna,
			HomeLocation:    locBna,
			AvailableEpoch:  startEpoch,
			Equipment:       model.Equipment{Type: model.EquipDryVan},
			Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
		{
			ID:              "D-MKE",
			CurrentLocation: locChi,
			HomeLocation:    locMke,
			AvailableEpoch:  startEpoch,
			Equipment:       model.Equipment{Type: model.EquipDryVan},
			Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
	}

	loads := []model.Load{
		{
			ID:                  "L-LONG-CHI-ATL",
			Origin:              locChi,
			Destination:         locAtl,
			RequiredEquipment:   model.EquipDryVan,
			Revenue:             3800.0,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 36000,
			DeliveryLatestEpoch: startEpoch + 120000,
		},
		{
			ID:                  "L-REGIONAL-CHI-MKE",
			Origin:              locChi,
			Destination:         locMke,
			RequiredEquipment:   model.EquipDryVan,
			Revenue:             900.0,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 36000,
			DeliveryLatestEpoch: startEpoch + 120000,
		},
	}

	matches := []model.DriverLoadMatch{
		{
			DriverID:              "D-MKE",
			LoadID:                "L-REGIONAL-CHI-MKE",
			DispatchEpoch:         startEpoch,
			EstimatedContribution: 700.0,
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

	runner := dispatch.NewRelayDispatchRunner(nil, synthesizer)

	batch, err := runner.SynthesizeRelayBatch(
		context.Background(),
		startEpoch,
		drivers,
		matches,
		loads,
		450.0,
	)
	if err != nil {
		t.Fatalf("SynthesizeRelayBatch failed: %v", err)
	}

	t.Logf("Relay Dispatch Batch Result:")
	t.Logf("  Direct Tours: %d, Relays: %d", batch.TotalTours, batch.TotalRelays)
	t.Logf("  Total Loaded Miles: %.1f, Total Empty Miles: %.1f, Net: $%.2f",
		batch.TotalLoadedMiles, batch.TotalEmptyMiles, batch.TotalNetContribution)

	if batch.TotalRelays != 1 {
		t.Errorf("expected 1 relay exchange, got %d", batch.TotalRelays)
	}
	if batch.TotalTours != 1 {
		t.Errorf("expected 1 direct tour, got %d", batch.TotalTours)
	}
	if len(batch.AssignedDriverIDs) != 3 {
		t.Errorf("expected 3 assigned drivers, got %d", len(batch.AssignedDriverIDs))
	}
}

func TestRelayDispatchRunner_FailClosedOnDirectBatchError(t *testing.T) {
	startEpoch := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}

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
		Revenue:             1000.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 36000,
		DeliveryLatestEpoch: startEpoch + 120000,
	}

	// Match references a driver "D-UNKNOWN" not in drivers list
	matches := []model.DriverLoadMatch{
		{
			DriverID:              "D-UNKNOWN",
			LoadID:                "L-01",
			DispatchEpoch:         startEpoch,
			EstimatedContribution: 500.0,
		},
	}

	runner := dispatch.NewRelayDispatchRunner(nil, nil)
	_, err := runner.SynthesizeRelayBatch(
		context.Background(),
		startEpoch,
		[]model.Driver{driver},
		matches,
		[]model.Load{load},
		450.0,
	)
	if err == nil {
		t.Fatalf("expected SynthesizeRelayBatch to fail closed on missing driver in match, got nil")
	}
}
