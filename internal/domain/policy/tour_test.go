package policy_test

import (
	"context"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

func TestTour_2LegChainedDispatchWithRest(t *testing.T) {
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locClt := model.Location{NodeID: "CLT", Lat: 35.2271, Lon: -80.8431}

	driver := model.Driver{
		ID:              "D-01",
		CurrentLocation: locChi,
		AvailableEpoch:  startEpoch,
		Equipment:       model.Equipment{Type: model.EquipDryVan},
		Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
	}

	load1 := model.Load{
		ID:                  "L-CHI-ATL",
		Origin:              locChi,
		Destination:         locAtl,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             2500.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 7200,
		DeliveryLatestEpoch: startEpoch + 48*3600,
	}

	load2 := model.Load{
		ID:                  "L-ATL-CLT",
		Origin:              locAtl,
		Destination:         locClt,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             1200.0,
		PickupEarliestEpoch: startEpoch + 20*3600,
		PickupLatestEpoch:   startEpoch + 36*3600,
		DeliveryLatestEpoch: startEpoch + 72*3600,
	}

	candidates := []model.Load{load2}

	synthesizer := policy.NewTourSynthesizer(policy.DefaultTourSynthesizerConfig())

	tour, err := synthesizer.SynthesizeTour(context.Background(), driver, load1, candidates)
	if err != nil {
		t.Fatalf("SynthesizeTour failed: %v", err)
	}

	if tour.DriverID() != "D-01" {
		t.Errorf("expected driver D-01, got %s", tour.DriverID())
	}
	if tour.LoadedLegCount() != 2 {
		t.Errorf("expected 2 loaded legs, got %d", tour.LoadedLegCount())
	}

	t.Logf("Synthesized Tour: %d total legs, %.1f loaded miles, %.1f empty miles, Net Contribution: $%.2f",
		tour.LegCount(), tour.TotalLoadedMiles(), tour.TotalEmptyMiles(), tour.NetContribution())

	for i, leg := range tour.Legs() {
		t.Logf("  Leg %d [%s]: %s -> %s (Load: %s, Dist: %.1f mi, Duration: %dm, Cost: $%.2f)",
			i+1, leg.Type, leg.Origin.NodeID, leg.Destination.NodeID, leg.LoadID,
			leg.DistanceMiles, leg.DurationMinutes, leg.CostBreakdown.TotalCost)
	}

	// Verify gross revenue
	expectedRev := load1.Revenue + load2.Revenue
	if tour.GrossRevenue() != expectedRev {
		t.Errorf("expected gross revenue $%.2f, got $%.2f", expectedRev, tour.GrossRevenue())
	}

	// Verify net contribution
	expectedNet := tour.GrossRevenue() - tour.TotalCost()
	if mathAbs(tour.NetContribution()-expectedNet) > 1e-6 {
		t.Errorf("expected net contribution $%.2f, got $%.2f", expectedNet, tour.NetContribution())
	}

	// Verify time continuity
	legs := tour.Legs()
	for i := 1; i < len(legs); i++ {
		if legs[i].StartEpoch < legs[i-1].EndEpoch {
			t.Errorf("time discontinuity between leg %d and %d", i-1, i)
		}
	}
}

func TestTour_DomicileReturnSynthesis(t *testing.T) {
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locInd := model.Location{NodeID: "IND", Lat: 39.7684, Lon: -86.1581}
	locCol := model.Location{NodeID: "CMH", Lat: 39.9612, Lon: -82.9988}

	driver := model.Driver{
		ID:              "D-02",
		CurrentLocation: locChi,
		HomeLocation:    locChi,
		AvailableEpoch:  startEpoch,
		Equipment:       model.Equipment{Type: model.EquipDryVan},
		Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
	}

	load1 := model.Load{
		ID:                  "L-CHI-IND",
		Origin:              locChi,
		Destination:         locInd,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             900.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 7200,
		DeliveryLatestEpoch: startEpoch + 24*3600,
	}

	load2 := model.Load{
		ID:                  "L-IND-CMH",
		Origin:              locInd,
		Destination:         locCol,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             800.0,
		PickupEarliestEpoch: startEpoch + 5*3600,
		PickupLatestEpoch:   startEpoch + 12*3600,
		DeliveryLatestEpoch: startEpoch + 24*3600,
	}

	cfg := policy.DefaultTourSynthesizerConfig()
	cfg.AutoRepositionHome = true // Enable auto-reposition back to CHI domicile

	synthesizer := policy.NewTourSynthesizer(cfg)

	tour, err := synthesizer.SynthesizeTour(context.Background(), driver, load1, []model.Load{load2})
	if err != nil {
		t.Fatalf("SynthesizeTour failed: %v", err)
	}

	if !tour.EndsAtDomicile() {
		t.Errorf("expected tour to end at driver domicile CHI")
	}

	lastLeg := tour.Legs()[len(tour.Legs())-1]
	if lastLeg.Type != policy.LegRepositionHome {
		t.Errorf("expected final leg to be REPOSITION_HOME, got %s", lastLeg.Type)
	}
	if lastLeg.Destination.NodeID != "CHI" {
		t.Errorf("expected final leg destination CHI, got %s", lastLeg.Destination.NodeID)
	}
}

func TestTour_Immutability(t *testing.T) {
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()
	locA := model.Location{NodeID: "A", Lat: 40.0, Lon: -80.0}
	locB := model.Location{NodeID: "B", Lat: 41.0, Lon: -81.0}

	legs := []policy.TourLeg{
		{
			Type:          policy.LegLoaded,
			Origin:        locA,
			Destination:   locB,
			StartEpoch:    startEpoch,
			EndEpoch:      startEpoch + 3600,
			DistanceMiles: 100.0,
			CostBreakdown: policy.TripCostBreakdown{Revenue: 500.0, TotalCost: 200.0, NetContribution: 300.0},
		},
	}

	tour, err := policy.NewDriverTour("D-IMM", legs, nil, nil)
	if err != nil {
		t.Fatalf("NewDriverTour failed: %v", err)
	}

	// Mutate slice returned by Legs()
	retrievedLegs := tour.Legs()
	retrievedLegs[0].DistanceMiles = 9999.0

	// Verify internal tour state remains 100.0
	if tour.Legs()[0].DistanceMiles != 100.0 {
		t.Errorf("mutating returned Legs() slice mutated internal DriverTour state")
	}
}

func mathAbs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
