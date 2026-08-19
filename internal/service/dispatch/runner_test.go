package dispatch_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/internal/service/dispatch"
)

func TestDispatchRunner_BatchSynthesis(t *testing.T) {
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locClt := model.Location{NodeID: "CLT", Lat: 35.2271, Lon: -80.8431}
	locNyc := model.Location{NodeID: "NYC", Lat: 40.7128, Lon: -74.0060}

	drivers := []model.Driver{
		{
			ID:              "D-01",
			CurrentLocation: locChi,
			AvailableEpoch:  startEpoch,
			Equipment:       model.Equipment{Type: model.EquipDryVan},
			Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
		{
			ID:              "D-02",
			CurrentLocation: locAtl,
			AvailableEpoch:  startEpoch,
			Equipment:       model.Equipment{Type: model.EquipDryVan},
			Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
	}

	load1 := model.Load{
		ID:                  "L-CHI-ATL",
		Origin:              locChi,
		Destination:         locAtl,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             2200.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 7200,
		DeliveryLatestEpoch: startEpoch + 48*3600,
	}

	load2 := model.Load{
		ID:                  "L-ATL-CLT",
		Origin:              locAtl,
		Destination:         locClt,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             1100.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 7200,
		DeliveryLatestEpoch: startEpoch + 24*3600,
	}

	load3 := model.Load{
		ID:                  "L-CLT-NYC",
		Origin:              locClt,
		Destination:         locNyc,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             1800.0,
		PickupEarliestEpoch: startEpoch + 15*3600,
		PickupLatestEpoch:   startEpoch + 36*3600,
		DeliveryLatestEpoch: startEpoch + 72*3600,
	}

	matches := []model.DriverLoadMatch{
		{DriverID: "D-01", LoadID: load1.ID},
		{DriverID: "D-02", LoadID: load2.ID},
	}

	allLoads := []model.Load{load1, load2, load3}

	runner := dispatch.NewDispatchRunner(nil)

	batch, err := runner.SynthesizeBatch(context.Background(), startEpoch, drivers, matches, allLoads)
	if err != nil {
		t.Fatalf("SynthesizeBatch failed: %v", err)
	}

	if batch.TotalTours != 2 {
		t.Errorf("expected 2 tours, got %d", batch.TotalTours)
	}

	t.Logf("Dispatch Batch Result: %d tours, %.1f loaded miles, %.1f empty miles, Total Net Contribution: $%.2f",
		batch.TotalTours, batch.TotalLoadedMiles, batch.TotalEmptyMiles, batch.TotalNetContribution)

	for _, tour := range batch.Tours {
		t.Logf("  Tour %s: %d legs, Revenue: $%.2f, Net: $%.2f",
			tour.DriverID(), tour.LegCount(), tour.GrossRevenue(), tour.NetContribution())
	}

	// Verify that D-02 chained into Load 3 (CLT -> NYC)
	var tourD02 *policy.DriverTour
	for _, tour := range batch.Tours {
		if tour.DriverID() == "D-02" {
			tourD02 = tour
			break
		}
	}

	if tourD02 == nil {
		t.Fatalf("Tour for D-02 not found")
	}

	if tourD02.LoadedLegCount() != 2 {
		t.Errorf("expected D-02 to chain 2 loaded legs (ATL-CLT and CLT-NYC), got %d", tourD02.LoadedLegCount())
	}
}

func TestDispatchRunner_ConcurrentRaceDetector(t *testing.T) {
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()
	locA := model.Location{NodeID: "A", Lat: 35.0, Lon: -85.0}
	locB := model.Location{NodeID: "B", Lat: 36.0, Lon: -86.0}

	const goroutines = 16
	var wg sync.WaitGroup

	runner := dispatch.NewDispatchRunner(nil)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()

			drivers := []model.Driver{
				{
					ID:              fmt.Sprintf("D-%02d", gID),
					CurrentLocation: locA,
					AvailableEpoch:  startEpoch,
					Equipment:       model.Equipment{Type: model.EquipDryVan},
					Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
				},
			}

			load := model.Load{
				ID:                  fmt.Sprintf("L-%02d", gID),
				Origin:              locA,
				Destination:         locB,
				RequiredEquipment:   model.EquipDryVan,
				Revenue:             1000.0,
				PickupEarliestEpoch: startEpoch,
				PickupLatestEpoch:   startEpoch + 7200,
				DeliveryLatestEpoch: startEpoch + 24*3600,
			}

			matches := []model.DriverLoadMatch{
				{DriverID: drivers[0].ID, LoadID: load.ID},
			}

			batch, err := runner.SynthesizeBatch(context.Background(), startEpoch, drivers, matches, []model.Load{load})
			if err != nil {
				t.Errorf("goroutine %d failed: %v", gID, err)
				return
			}
			if batch.TotalTours != 1 {
				t.Errorf("goroutine %d expected 1 tour, got %d", gID, batch.TotalTours)
			}
		}(g)
	}

	wg.Wait()
}
