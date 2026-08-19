package benchmarks_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/internal/service/dispatch"
)

// BenchmarkRelay_Synthesizer evaluates multi-driver relay exchange discovery on 50 drivers and 100 long-haul loads.
func BenchmarkRelay_Synthesizer(b *testing.B) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locSdf := model.Location{NodeID: "SDF", Lat: 38.2527, Lon: -85.7585}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locDal := model.Location{NodeID: "DAL", Lat: 32.7767, Lon: -96.7970}
	locMem := model.Location{NodeID: "MEM", Lat: 35.1495, Lon: -90.0490}

	startEpoch := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

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
		{
			ID:                  "FAC-MEM-HUB",
			Name:                "Memphis Freight Exchange",
			Location:            locMem,
			Type:                model.FacilityRelayHub,
			OpenMinutesOfDay:    0,
			CloseMinutesOfDay:   1440,
			AverageDwellMinutes: 60,
		},
	}
	facStore := model.NewFacilityStore(facilities)

	rng := rand.New(rand.NewSource(555))
	cities := []model.Location{locChi, locAtl, locDal, locSdf, locMem}

	numDrivers := 50
	drivers := make([]model.Driver, numDrivers)
	for i := 0; i < numDrivers; i++ {
		loc := cities[rng.Intn(len(cities))]
		drivers[i] = model.Driver{
			ID:                  fmt.Sprintf("DRV_RELAY_%03d", i),
			CurrentLocation:     loc,
			HomeLocation:        loc,
			AvailableEpoch:      startEpoch,
			DriveHoursRemaining: 11.0,
			DutyHoursRemaining:  14.0,
			Equipment:           model.Equipment{Type: model.EquipDryVan},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		}
	}

	numLoads := 100
	loads := make([]model.Load, numLoads)
	for j := 0; j < numLoads; j++ {
		orig := locChi
		dest := locAtl
		if j%2 == 0 {
			orig = locChi
			dest = locDal
		}
		loads[j] = model.Load{
			ID:                  fmt.Sprintf("LOAD_RELAY_%04d", j),
			Origin:              orig,
			Destination:         dest,
			RequiredEquipment:   model.EquipDryVan,
			Revenue:             3800.0,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 36000,
			DeliveryLatestEpoch: startEpoch + 172800,
		}
	}

	synth := policy.NewRelaySynthesizer(
		model.DefaultCostConfig(),
		policy.DefaultRelayConfig(),
		hos.USPolicySpecs(),
		facStore,
		nil,
	)

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		exchanges, err := synth.SynthesizeRelays(ctx, drivers, loads, 400.0)
		if err != nil {
			b.Fatalf("SynthesizeRelays failed: %v", err)
		}
		_ = exchanges
	}
}

// BenchmarkRelay_CombinedDispatchRunner benchmarks combined relay & multi-leg tour synthesis.
func BenchmarkRelay_CombinedDispatchRunner(b *testing.B) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locSdf := model.Location{NodeID: "SDF", Lat: 38.2527, Lon: -85.7585}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}

	startEpoch := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	facilities := []model.Facility{
		{
			ID:                  "FAC-SDF-HUB",
			Location:            locSdf,
			Type:                model.FacilityRelayHub,
			OpenMinutesOfDay:    0,
			CloseMinutesOfDay:   1440,
			AverageDwellMinutes: 60,
		},
	}
	facStore := model.NewFacilityStore(facilities)

	synth := policy.NewRelaySynthesizer(
		model.DefaultCostConfig(),
		policy.DefaultRelayConfig(),
		hos.USPolicySpecs(),
		facStore,
		nil,
	)
	runner := dispatch.NewRelayDispatchRunner(nil, synth)

	drivers := []model.Driver{
		{ID: "D1", CurrentLocation: locChi, HomeLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}, Clocks: hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0))},
		{ID: "D2", CurrentLocation: locSdf, HomeLocation: locSdf, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}, Clocks: hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0))},
		{ID: "D3", CurrentLocation: locChi, HomeLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}, Clocks: hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0))},
	}

	loads := []model.Load{
		{ID: "L-LONG", Origin: locChi, Destination: locAtl, RequiredEquipment: model.EquipDryVan, Revenue: 3500.0, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 36000, DeliveryLatestEpoch: startEpoch + 172800},
		{ID: "L-SHORT", Origin: locChi, Destination: locSdf, RequiredEquipment: model.EquipDryVan, Revenue: 900.0, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 36000, DeliveryLatestEpoch: startEpoch + 172800},
	}

	matches := []model.DriverLoadMatch{
		{DriverID: "D3", LoadID: "L-SHORT", DispatchEpoch: startEpoch, EstimatedContribution: 650.0},
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		batch, err := runner.SynthesizeRelayBatch(ctx, startEpoch, drivers, matches, loads, 450.0)
		if err != nil {
			b.Fatalf("SynthesizeRelayBatch failed: %v", err)
		}
		if batch.TotalRelays == 0 && batch.TotalTours == 0 {
			b.Fatalf("expected dispatch executions")
		}
	}
}
