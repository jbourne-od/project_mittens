package reposition_test

import (
	"context"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy/reposition"
)

func createRepositioningTestScenario() (*model.ResourceState, *model.RegionManager, int64) {
	startEpoch := time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC).Unix()

	locClt := model.Location{NodeID: "CLT", Lat: 35.2271, Lon: -80.8431} // Charlotte deficit zone
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880} // Atlanta headhaul cluster
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298} // Midwest hub

	regionDefs := []model.SquareRegion{
		{ID: "SE_CLT", MinLat: 34.0, MaxLat: 37.0, MinLon: -82.0, MaxLon: -79.0, Centroid: locClt},
		{ID: "SE_ATL", MinLat: 31.0, MaxLat: 34.0, MinLon: -86.0, MaxLon: -83.0, Centroid: locAtl},
		{ID: "MW_CHI", MinLat: 40.0, MaxLat: 44.0, MinLon: -90.0, MaxLon: -85.0, Centroid: locChi},
	}
	regionMgr := model.NewRegionManager(1.0, regionDefs)

	// In Charlotte (SE_CLT): 2 unassigned drivers, 0 outbound loads (Heavy Deficit / Oversupply)
	drivers := []model.Driver{
		{
			ID:                  "DRV-CLT-01",
			CurrentLocation:     locClt,
			HomeLocation:        locClt,
			AvailableEpoch:      startEpoch,
			DriveHoursRemaining: 11.0,
			DutyHoursRemaining:  14.0,
			Equipment:           model.Equipment{Type: model.EquipDryVan},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
		{
			ID:                  "DRV-CLT-02",
			CurrentLocation:     locClt,
			HomeLocation:        locClt,
			AvailableEpoch:      startEpoch,
			DriveHoursRemaining: 11.0,
			DutyHoursRemaining:  14.0,
			Equipment:           model.Equipment{Type: model.EquipDryVan},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
	}

	// In Atlanta (SE_ATL): 0 drivers, 2 high-yield outbound loads (Heavy Shortage / Surplus Demand)
	loads := []model.Load{
		{
			ID:                    "LOAD-ATL-1",
			Origin:                locAtl,
			Destination:           locChi,
			PickupEarliestEpoch:   startEpoch + 36000,
			PickupLatestEpoch:     startEpoch + 72000,
			DeliveryEarliestEpoch: startEpoch + 72000,
			DeliveryLatestEpoch:   startEpoch + 108000,
			Revenue:               2600.0,
			RequiredEquipment:     model.EquipDryVan,
		},
		{
			ID:                    "LOAD-ATL-2",
			Origin:                locAtl,
			Destination:           locChi,
			PickupEarliestEpoch:   startEpoch + 36000,
			PickupLatestEpoch:     startEpoch + 72000,
			DeliveryEarliestEpoch: startEpoch + 72000,
			DeliveryLatestEpoch:   startEpoch + 108000,
			Revenue:               2700.0,
			RequiredEquipment:     model.EquipDryVan,
		},
	}

	res := model.NewResourceState(drivers, loads)
	return res, regionMgr, startEpoch
}

func TestRegionalBalanceCalculator_Imbalances(t *testing.T) {
	res, regionMgr, _ := createRepositioningTestScenario()
	calc := reposition.NewRegionalBalanceCalculator()

	yields := map[string]float64{
		"SE_CLT": 900.0,
		"SE_ATL": 2200.0,
	}

	balances := calc.ComputeBalance(res, regionMgr, yields)

	snapClt, okClt := balances["SE_CLT"]
	if !okClt {
		t.Fatalf("missing SE_CLT balance snapshot")
	}
	if snapClt.AvailableDrivers != 2 || snapClt.OutboundTenders != 0 {
		t.Errorf("expected 2 drivers and 0 tenders in CLT, got %d and %d", snapClt.AvailableDrivers, snapClt.OutboundTenders)
	}
	if snapClt.Deficit != 2 {
		t.Errorf("expected deficit 2 in CLT, got %d", snapClt.Deficit)
	}
	if snapClt.ShadowPrice >= snapClt.AverageYieldPerTruck {
		t.Errorf("expected depressed shadow price in deficit region CLT, got %f vs %f", snapClt.ShadowPrice, snapClt.AverageYieldPerTruck)
	}

	snapAtl, okAtl := balances["SE_ATL"]
	if !okAtl {
		t.Fatalf("missing SE_ATL balance snapshot")
	}
	if snapAtl.AvailableDrivers != 0 || snapAtl.OutboundTenders != 2 {
		t.Errorf("expected 0 drivers and 2 tenders in ATL, got %d and %d", snapAtl.AvailableDrivers, snapAtl.OutboundTenders)
	}
	if snapAtl.Deficit != -2 {
		t.Errorf("expected deficit -2 in ATL, got %d", snapAtl.Deficit)
	}
	if snapAtl.ShadowPrice <= snapAtl.AverageYieldPerTruck {
		t.Errorf("expected elevated shadow price in shortage region ATL, got %f vs %f", snapAtl.ShadowPrice, snapAtl.AverageYieldPerTruck)
	}
}

func TestRepositioningSynthesizer_Arbitrage(t *testing.T) {
	ctx := context.Background()
	res, regionMgr, _ := createRepositioningTestScenario()
	synth := reposition.NewRepositioningSynthesizer()

	cfg := reposition.DefaultRepositioningConfig()
	cfg.MaxRepositionDistanceMiles = 400.0
	cfg.DefaultRegionalYield = map[string]float64{
		"SE_CLT": 800.0,
		"SE_ATL": 2800.0,
	}
	cfg.MinArbitrageThreshold = 100.0

	unassigned := res.Drivers()
	moves, err := synth.SynthesizeRepositioningMoves(ctx, res, regionMgr, unassigned, cfg)
	if err != nil {
		t.Fatalf("failed synthesizing repositioning moves: %v", err)
	}

	if len(moves) != 2 {
		t.Fatalf("expected 2 repositioning moves for 2 drivers, got %d", len(moves))
	}

	for _, m := range moves {
		if m.TargetRegionID != "SE_ATL" {
			t.Errorf("expected relocation to SE_ATL, got %s", m.TargetRegionID)
		}
		if m.NetRepositioningValue <= 0 {
			t.Errorf("expected positive net repositioning arbitrage, got %f", m.NetRepositioningValue)
		}
		if m.ArrivalEpoch <= m.StartEpoch {
			t.Errorf("arrival epoch %d must be after start epoch %d", m.ArrivalEpoch, m.StartEpoch)
		}
	}

	summary := reposition.SummaryString(moves)
	if len(summary) == 0 {
		t.Errorf("expected non-empty summary string")
	}
}

func TestRepositioningSynthesizer_HOSInfeasibility(t *testing.T) {
	ctx := context.Background()
	res, regionMgr, _ := createRepositioningTestScenario()
	synth := reposition.NewRepositioningSynthesizer()

	cfg := reposition.DefaultRepositioningConfig()
	cfg.MaxRepositionDistanceMiles = 400.0
	cfg.DefaultRegionalYield = map[string]float64{
		"SE_CLT": 800.0,
		"SE_ATL": 2800.0,
	}

	// Restrict drivers to only 1 hour drive time remaining (insufficient for 226 mi trip)
	tiredDrivers := make([]model.Driver, len(res.Drivers()))
	for i, d := range res.Drivers() {
		tiredDrivers[i] = d
		tiredDrivers[i].DriveHoursRemaining = 1.0
	}

	moves, err := synth.SynthesizeRepositioningMoves(ctx, res, regionMgr, tiredDrivers, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(moves) != 0 {
		t.Errorf("expected 0 repositioning moves due to HOS limits, got %d", len(moves))
	}
}
