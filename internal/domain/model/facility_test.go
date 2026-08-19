package model_test

import (
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

func TestFacilityStore_LookupAndNearest(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locMil := model.Location{NodeID: "MIL", Lat: 43.0389, Lon: -87.9065}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}

	facilities := []model.Facility{
		{
			ID:                  "FAC-CHI",
			Name:                "Chicago Main Terminal",
			Location:            locChi,
			Type:                model.FacilityTerminal,
			OpenMinutesOfDay:    480,  // 8:00 AM
			CloseMinutesOfDay:   1020, // 5:00 PM
			AverageDwellMinutes: 45,
		},
		{
			ID:                  "FAC-MIL",
			Name:                "Milwaukee Shipper",
			Location:            locMil,
			Type:                model.FacilityShipper,
			OpenMinutesOfDay:    0,
			CloseMinutesOfDay:   1440, // 24/7
			AverageDwellMinutes: 60,
		},
	}

	store := model.NewFacilityStore(facilities)

	if store.Count() != 2 {
		t.Fatalf("expected 2 facilities, got %d", store.Count())
	}

	// 1. Get by ID
	fChi, err := store.GetFacility("FAC-CHI")
	if err != nil || fChi.ID != "FAC-CHI" {
		t.Fatalf("GetFacility failed: %v", err)
	}

	// 2. Open hours check
	timeAt9AM := time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)
	timeAt11PM := time.Date(2026, 8, 19, 23, 0, 0, 0, time.UTC)

	if !fChi.IsOpenAt(timeAt9AM) {
		t.Fatalf("FAC-CHI should be open at 9:00 AM")
	}
	if fChi.IsOpenAt(timeAt11PM) {
		t.Fatalf("FAC-CHI should be closed at 11:00 PM")
	}

	// 3. Find Nearest
	nearest, dist, err := store.FindNearest(locAtl, model.FacilityTerminal)
	if err != nil {
		t.Fatalf("FindNearest failed: %v", err)
	}
	if nearest.ID != "FAC-CHI" {
		t.Fatalf("expected nearest terminal to ATL to be FAC-CHI, got %s", nearest.ID)
	}
	if dist <= 0 {
		t.Fatalf("expected positive distance, got %f", dist)
	}
}
