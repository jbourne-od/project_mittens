package stream_test

import (
	"sync"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/adapter/stream"
	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
)

func TestStreamBuffer_Validation(t *testing.T) {
	buf := stream.NewStreamBuffer()

	// Empty driver ID
	err := buf.IngestDriverPing(stream.ELDDriverPingDTO{
		DriverID: "",
		Lat:      40.0,
		Lon:      -80.0,
	})
	if err == nil {
		t.Errorf("expected error on empty driver ID")
	}

	// Invalid Lat
	err = buf.IngestDriverPing(stream.ELDDriverPingDTO{
		DriverID: "DRV-1",
		Lat:      95.0,
		Lon:      -80.0,
	})
	if err == nil {
		t.Errorf("expected error on out-of-bounds latitude")
	}

	// Invalid HOS
	err = buf.IngestDriverPing(stream.ELDDriverPingDTO{
		DriverID:            "DRV-1",
		Lat:                 40.0,
		Lon:                 -80.0,
		DriveHoursRemaining: -1.0,
	})
	if err == nil {
		t.Errorf("expected error on negative drive hours")
	}

	// Empty load ID
	err = buf.IngestLoadTender(stream.TMSLoadTenderDTO{
		LoadID:  "",
		Revenue: 1000.0,
	})
	if err == nil {
		t.Errorf("expected error on empty load ID")
	}

	// Negative revenue
	err = buf.IngestLoadTender(stream.TMSLoadTenderDTO{
		LoadID:    "LOAD-1",
		OriginLat: 40.0,
		OriginLon: -80.0,
		DestLat:   41.0,
		DestLon:   -81.0,
		Revenue:   -100.0,
	})
	if err == nil {
		t.Errorf("expected error on negative revenue")
	}
}

func TestStreamBuffer_TimestampOrdering(t *testing.T) {
	buf := stream.NewStreamBuffer()

	// Ingest newer ping first
	p1 := stream.ELDDriverPingDTO{
		DriverID:            "DRV-1",
		Timestamp:           2000,
		Lat:                 41.8781,
		Lon:                 -87.6298,
		DriveHoursRemaining: 8.0,
		DutyHoursRemaining:  10.0,
	}
	if err := buf.IngestDriverPing(p1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ingest older out-of-order ping
	p0 := stream.ELDDriverPingDTO{
		DriverID:            "DRV-1",
		Timestamp:           1000,
		Lat:                 33.7490,
		Lon:                 -84.3880,
		DriveHoursRemaining: 11.0,
		DutyHoursRemaining:  14.0,
	}
	if err := buf.IngestDriverPing(p0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pings, _, _ := buf.Snapshot()
	if len(pings) != 1 {
		t.Fatalf("expected 1 ping, got %d", len(pings))
	}
	if pings["DRV-1"].Timestamp != 2000 {
		t.Errorf("expected timestamp 2000 to be preserved, got %d", pings["DRV-1"].Timestamp)
	}
	if pings["DRV-1"].Lat != 41.8781 {
		t.Errorf("expected lat 41.8781, got %f", pings["DRV-1"].Lat)
	}
}

func TestStreamBuffer_ConcurrentAccess(t *testing.T) {
	buf := stream.NewStreamBuffer()
	var wg sync.WaitGroup

	// 20 concurrent driver telemetry producers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(drvIdx int) {
			defer wg.Done()
			for step := 0; step < 50; step++ {
				_ = buf.IngestDriverPing(stream.ELDDriverPingDTO{
					DriverID:            "DRV-" + string(rune('A'+drvIdx)),
					Timestamp:           int64(1000 + step),
					Lat:                 40.0 + float64(step)*0.01,
					Lon:                 -80.0,
					DriveHoursRemaining: 10.0,
					DutyHoursRemaining:  12.0,
				})
			}
		}(i)
	}

	// 10 concurrent load tender producers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(loadIdx int) {
			defer wg.Done()
			for step := 0; step < 20; step++ {
				_ = buf.IngestLoadTender(stream.TMSLoadTenderDTO{
					LoadID:    "LOAD-" + string(rune('A'+loadIdx)),
					Timestamp: int64(1000 + step),
					OriginLat: 35.0,
					OriginLon: -85.0,
					DestLat:   36.0,
					DestLon:   -86.0,
					Revenue:   1500.0,
				})
			}
		}(i)
	}

	// 5 concurrent status/snapshot readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for step := 0; step < 30; step++ {
				_ = buf.Status()
				_, _, _ = buf.Snapshot()
			}
		}()
	}

	wg.Wait()
	status := buf.Status()
	if status.TotalPingsIngested != 1000 {
		t.Errorf("expected 1000 total pings ingested, got %d", status.TotalPingsIngested)
	}
	if status.TotalTendersIngested != 200 {
		t.Errorf("expected 200 total tenders ingested, got %d", status.TotalTendersIngested)
	}
}

func TestStateSynchronizer_MergeAndImmutability(t *testing.T) {
	buf := stream.NewStreamBuffer()
	syncEngine := stream.NewStateSynchronizer(buf)

	startEpoch := time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC).Unix()

	// Initial resource state with 1 driver and 2 loads
	initialDrivers := []model.Driver{
		{
			ID:                  "DRV-01",
			CurrentLocation:     model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298},
			HomeLocation:        model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298},
			AvailableEpoch:      startEpoch,
			DriveHoursRemaining: 11.0,
			DutyHoursRemaining:  14.0,
			Equipment:           model.Equipment{Type: model.EquipDryVan},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
	}

	initialLoads := []model.Load{
		{
			ID:                    "LOAD-KEEP",
			Origin:                model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298},
			Destination:           model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880},
			PickupEarliestEpoch:   startEpoch,
			PickupLatestEpoch:     startEpoch + 3600,
			DeliveryEarliestEpoch: startEpoch + 7200,
			DeliveryLatestEpoch:   startEpoch + 14400,
			Revenue:               1800.0,
			RequiredEquipment:     model.EquipDryVan,
		},
		{
			ID:                    "LOAD-CANCEL",
			Origin:                model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298},
			Destination:           model.Location{NodeID: "CLT", Lat: 35.2271, Lon: -80.8431},
			PickupEarliestEpoch:   startEpoch,
			PickupLatestEpoch:     startEpoch + 3600,
			DeliveryEarliestEpoch: startEpoch + 7200,
			DeliveryLatestEpoch:   startEpoch + 14400,
			Revenue:               1600.0,
			RequiredEquipment:     model.EquipDryVan,
		},
	}

	baseResource := model.NewResourceState(initialDrivers, initialLoads)

	// Stream updates:
	// 1. Update DRV-01 location to Indianapolis, drive hours to 8.5
	_ = buf.IngestDriverPing(stream.ELDDriverPingDTO{
		DriverID:            "DRV-01",
		Timestamp:           startEpoch + 1800,
		Lat:                 39.7684,
		Lon:                 -86.1581,
		NodeID:              "IND",
		DriveHoursRemaining: 8.5,
		DutyHoursRemaining:  11.5,
	})

	// 2. Discover brand new driver DRV-02 in Atlanta
	_ = buf.IngestDriverPing(stream.ELDDriverPingDTO{
		DriverID:            "DRV-02",
		Timestamp:           startEpoch + 1800,
		Lat:                 33.7490,
		Lon:                 -84.3880,
		NodeID:              "ATL",
		DriveHoursRemaining: 10.0,
		DutyHoursRemaining:  13.0,
		EquipmentType:       "REEFER",
	})

	// 3. Cancel LOAD-CANCEL
	_ = buf.CancelTender(stream.TenderCancelDTO{
		LoadID:    "LOAD-CANCEL",
		Reason:    "Customer canceled shipper order",
		Timestamp: startEpoch + 1800,
	})

	// 4. Ingest new load tender LOAD-NEW
	_ = buf.IngestLoadTender(stream.TMSLoadTenderDTO{
		LoadID:                "LOAD-NEW",
		Timestamp:             startEpoch + 1800,
		OriginNodeID:          "IND",
		OriginLat:             39.7684,
		OriginLon:             -86.1581,
		DestinationNodeID:     "ATL",
		DestLat:               33.7490,
		DestLon:               -84.3880,
		PickupEarliestEpoch:   startEpoch + 3600,
		PickupLatestEpoch:     startEpoch + 7200,
		DeliveryEarliestEpoch: startEpoch + 10800,
		DeliveryLatestEpoch:   startEpoch + 18000,
		Revenue:               2100.0,
		RequiredEquipment:     "DRY_VAN",
	})

	// Perform Synchronization
	nextResource, report, err := syncEngine.Synchronize(baseResource, startEpoch+1800)
	if err != nil {
		t.Fatalf("synchronization failed: %v", err)
	}

	if report.DriversUpdated != 1 {
		t.Errorf("expected 1 driver updated, got %d", report.DriversUpdated)
	}
	if report.DriversAdded != 1 {
		t.Errorf("expected 1 driver added, got %d", report.DriversAdded)
	}
	if report.LoadsCanceled != 1 {
		t.Errorf("expected 1 load canceled, got %d", report.LoadsCanceled)
	}
	if report.LoadsAdded != 1 {
		t.Errorf("expected 1 load added, got %d", report.LoadsAdded)
	}
	if report.TotalDrivers != 2 {
		t.Errorf("expected 2 total drivers, got %d", report.TotalDrivers)
	}
	if report.TotalLoads != 2 {
		t.Errorf("expected 2 total loads, got %d", report.TotalLoads)
	}

	// Verify DRV-01 was updated in nextResource
	drv1, ok := nextResource.GetDriver("DRV-01")
	if !ok {
		t.Fatalf("DRV-01 not found in synced state")
	}
	if drv1.CurrentLocation.NodeID != "IND" {
		t.Errorf("expected DRV-01 location IND, got %s", drv1.CurrentLocation.NodeID)
	}
	if drv1.DriveHoursRemaining != 8.5 {
		t.Errorf("expected DRV-01 drive hours 8.5, got %f", drv1.DriveHoursRemaining)
	}

	// Inviolate 5: Verify baseResource remains completely unmutated
	origDrv1, _ := baseResource.GetDriver("DRV-01")
	if origDrv1.CurrentLocation.NodeID != "CHI" {
		t.Errorf("Inviolate 5 violation: baseResource driver location was mutated to %s", origDrv1.CurrentLocation.NodeID)
	}
	if origDrv1.DriveHoursRemaining != 11.0 {
		t.Errorf("Inviolate 5 violation: baseResource driver drive hours were mutated to %f", origDrv1.DriveHoursRemaining)
	}
	if len(baseResource.Loads()) != 2 {
		t.Errorf("Inviolate 5 violation: baseResource loads count was mutated")
	}
}
