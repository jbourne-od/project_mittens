package feasibility_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/feasibility"
)

func TestConcurrentFilter_BasicFilteringAndSorting(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locMil := model.Location{NodeID: "MIL", Lat: 43.0389, Lon: -87.9065} // ~90 miles from Chicago
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880} // ~587 miles from Chicago
	locDal := model.Location{NodeID: "DAL", Lat: 32.7767, Lon: -96.7970}

	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{ID: "D-02", CurrentLocation: locChi, AvailableEpoch: startEpoch},
		{ID: "D-01", CurrentLocation: locMil, AvailableEpoch: startEpoch},
	}

	loads := []model.Load{
		{
			ID:                  "L-01",
			Origin:              locChi,
			Destination:         locAtl,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 14400, // 4 hours window
			Revenue:             2500.0,
		},
		{
			ID:                  "L-02",
			Origin:              locMil,
			Destination:         locDal,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 14400,
			Revenue:             2800.0,
		},
	}

	cfg := feasibility.FilterConfig{
		Feasibility: model.FeasibilityConfig{
			MaxDeadheadMiles: 150.0, // Prunes long deadheads
			AverageSpeedMPH:  50.0,
		},
		WorkerCount: 4,
	}

	filter := feasibility.NewConcurrentFilter()
	ctx := context.Background()

	arcs, err := filter.FilterCandidates(ctx, drivers, loads, cfg)
	if err != nil {
		t.Fatalf("FilterCandidates failed: %v", err)
	}

	// Verify deterministic canonical sorting by DriverID, then LoadID
	for i := 1; i < len(arcs); i++ {
		prev := arcs[i-1]
		curr := arcs[i]
		if prev.DriverID > curr.DriverID || (prev.DriverID == curr.DriverID && prev.LoadID >= curr.LoadID) {
			t.Fatalf("candidate arcs not canonically sorted: [%s, %s] after [%s, %s]", curr.DriverID, curr.LoadID, prev.DriverID, prev.LoadID)
		}
	}
}

func TestConcurrentFilter_EquipmentAndEndorsements(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	// Dry van driver (no hazmat)
	dryDriver := model.Driver{
		ID:              "D-DRY",
		CurrentLocation: locChi,
		AvailableEpoch:  startEpoch,
		Equipment:       model.Equipment{Type: model.EquipDryVan},
	}
	// Reefer driver with hazmat
	hazmatReeferDriver := model.Driver{
		ID:              "D-REEFER-HAZ",
		CurrentLocation: locChi,
		AvailableEpoch:  startEpoch,
		Equipment: model.Equipment{
			Type:         model.EquipReefer,
			Endorsements: []model.Endorsement{model.EndorsementHazmat},
		},
	}

	loads := []model.Load{
		{
			ID:                   "L-HAZMAT",
			Origin:               locChi,
			Destination:          locAtl,
			RequiredEquipment:    model.EquipDryVan,
			RequiredEndorsements: []model.Endorsement{model.EndorsementHazmat},
			PickupEarliestEpoch:  startEpoch,
			PickupLatestEpoch:    startEpoch + 36000,
			DeliveryLatestEpoch:  startEpoch + 120000,
		},
		{
			ID:                  "L-REEFER",
			Origin:              locChi,
			Destination:         locAtl,
			RequiredEquipment:   model.EquipReefer,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 36000,
			DeliveryLatestEpoch: startEpoch + 120000,
		},
	}

	cfg := feasibility.FilterConfig{
		Feasibility: model.DefaultFeasibilityConfig(),
		WorkerCount: 2,
	}

	filter := feasibility.NewConcurrentFilter()
	arcs, err := filter.FilterCandidates(context.Background(), []model.Driver{dryDriver, hazmatReeferDriver}, loads, cfg)
	if err != nil {
		t.Fatalf("FilterCandidates failed: %v", err)
	}

	// Dry driver should match 0 loads (L-HAZMAT requires Hazmat, L-REEFER requires Reefer)
	// Hazmat reefer driver should match BOTH loads (reefer can haul dry hazmat and reefer)
	if len(arcs) != 2 {
		t.Fatalf("expected exactly 2 feasible arcs for D-REEFER-HAZ, got %d: %+v", len(arcs), arcs)
	}

	for _, arc := range arcs {
		if arc.DriverID != "D-REEFER-HAZ" {
			t.Fatalf("dry driver unexpectedly matched incompatible load: %+v", arc)
		}
	}
}

func TestConcurrentFilter_DeadheadPruning(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locMIA := model.Location{NodeID: "MIA", Lat: 25.7617, Lon: -80.1918} // >1100 miles from Chicago
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}

	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: startEpoch},
	}

	loads := []model.Load{
		{
			ID:                  "L-MIA",
			Origin:              locMIA, // Too far for Chicago driver
			Destination:         locAtl,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 86400,
		},
	}

	cfg := feasibility.FilterConfig{
		Feasibility: model.FeasibilityConfig{
			MaxDeadheadMiles: 200.0, // Max 200 miles deadhead
			AverageSpeedMPH:  50.0,
		},
		WorkerCount: 2,
	}

	filter := feasibility.NewConcurrentFilter()
	arcs, err := filter.FilterCandidates(context.Background(), drivers, loads, cfg)
	if err != nil {
		t.Fatalf("FilterCandidates failed: %v", err)
	}

	if len(arcs) != 0 {
		t.Fatalf("expected 0 arcs due to deadhead pruning, got %d", len(arcs))
	}
}

func TestConcurrentFilter_ContextCancellation(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}

	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	// Large batch
	drivers := make([]model.Driver, 100)
	for i := 0; i < 100; i++ {
		drivers[i] = model.Driver{
			ID:              fmt.Sprintf("D-%03d", i),
			CurrentLocation: locChi,
			AvailableEpoch:  startEpoch,
		}
	}
	loads := make([]model.Load, 50)
	for j := 0; j < 50; j++ {
		loads[j] = model.Load{
			ID:                  fmt.Sprintf("L-%03d", j),
			Origin:              locChi,
			Destination:         locAtl,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 86400,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cfg := feasibility.FilterConfig{
		Feasibility: model.DefaultFeasibilityConfig(),
		WorkerCount: 8,
	}

	filter := feasibility.NewConcurrentFilter()
	_, err := filter.FilterCandidates(ctx, drivers, loads, cfg)
	if err == nil {
		t.Fatalf("expected error on cancelled context, got nil")
	}
}

func TestConcurrentFilter_HighConcurrencyRaceDetector(t *testing.T) {
	// Verify parallel evaluation of 50 drivers x 50 loads (2500 pairs) under -race
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}

	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	const numDrivers = 50
	const numLoads = 50

	drivers := make([]model.Driver, numDrivers)
	for i := 0; i < numDrivers; i++ {
		drivers[i] = model.Driver{
			ID:              fmt.Sprintf("D-%03d", i),
			CurrentLocation: locChi,
			AvailableEpoch:  startEpoch,
		}
	}

	loads := make([]model.Load, numLoads)
	for j := 0; j < numLoads; j++ {
		loads[j] = model.Load{
			ID:                  fmt.Sprintf("L-%03d", j),
			Origin:              locChi,
			Destination:         locAtl,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 36000,
			DeliveryLatestEpoch: startEpoch + 120000, // ~33h window, allowing 11.75h drive + 10h sleeper reset + dwell
		}
	}

	cfg := feasibility.FilterConfig{
		Feasibility: model.DefaultFeasibilityConfig(),
		WorkerCount: 16,
	}

	filter := feasibility.NewConcurrentFilter()
	arcs, err := filter.FilterCandidates(context.Background(), drivers, loads, cfg)
	if err != nil {
		t.Fatalf("FilterCandidates failed under concurrency: %v", err)
	}

	// Since all 50 drivers and 50 loads start at Chicago and go to Atlanta with open windows, all 2500 pairs should be feasible
	if len(arcs) != numDrivers*numLoads {
		t.Fatalf("expected %d feasible arcs, got %d", numDrivers*numLoads, len(arcs))
	}
}
