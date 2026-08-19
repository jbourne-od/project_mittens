package model_test

import (
	"math"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

func TestModelCornerCases_GeographicExtremes(t *testing.T) {
	// 1. Poles: North Pole (90, 0) to South Pole (-90, 0)
	northPole := model.Location{NodeID: "NORTH_POLE", Lat: 90.0, Lon: 0.0}
	southPole := model.Location{NodeID: "SOUTH_POLE", Lat: -90.0, Lon: 0.0}

	poleDist := northPole.DistanceMiles(southPole)
	expectedPoleDist := math.Pi * 3958.8 // ~12,436 miles (half Earth circumference)
	if math.Abs(poleDist-expectedPoleDist) > 5.0 {
		t.Errorf("Pole distance calculation drift: got %.2f, expected %.2f", poleDist, expectedPoleDist)
	}

	// 2. Antimeridian Crossing: 179.9 deg E to -179.9 deg W at Equator
	eastAntimeridian := model.Location{NodeID: "EAST_AM", Lat: 0.0, Lon: 179.9}
	westAntimeridian := model.Location{NodeID: "WEST_AM", Lat: 0.0, Lon: -179.9}

	antimeridianDist := eastAntimeridian.DistanceMiles(westAntimeridian)
	expectedAntimeridianDist := (0.2 / 360.0) * (2 * math.Pi * 3958.8) // ~13.8 miles
	if math.Abs(antimeridianDist-expectedAntimeridianDist) > 1.0 {
		t.Errorf("Antimeridian crossing distance drift: got %.2f, expected %.2f",
			antimeridianDist, expectedAntimeridianDist)
	}

	// 3. Co-located identical coordinates
	locA := model.Location{NodeID: "LOC_A", Lat: 40.7128, Lon: -74.0060}
	locB := model.Location{NodeID: "LOC_B", Lat: 40.7128, Lon: -74.0060}
	if dist := locA.DistanceMiles(locB); dist != 0.0 {
		t.Errorf("Identical coordinate distance must be exactly 0.0, got %f", dist)
	}
}

func TestModelCornerCases_InvalidFloatsAndFailClosed(t *testing.T) {
	epoch := time.Now().Unix()

	// 1. InformationState with NaN / Inf
	if _, err := model.NewInformationState(epoch, math.NaN(), 2.50, 0); err == nil {
		t.Errorf("expected error constructing InformationState with NaN fuelPriceIndex")
	}
	if _, err := model.NewInformationState(epoch, 1.0, math.Inf(1), 0); err == nil {
		t.Errorf("expected error constructing InformationState with +Inf spotRateIndex")
	}
	if _, err := model.NewInformationState(epoch, -0.5, 2.50, 0); err == nil {
		t.Errorf("expected error constructing InformationState with negative fuelPriceIndex")
	}

	// 2. CostConfig Validation
	costCfg := model.CostConfig{
		FixedCostPerLoad: -10.0, // Invalid negative cost
	}
	if err := costCfg.Validate(); err == nil {
		t.Errorf("expected error validating CostConfig with negative FixedCostPerLoad")
	}

	// 3. FeasibilityConfig Validation
	feasCfg := model.FeasibilityConfig{
		AverageSpeedMPH: -20.0, // Invalid negative speed
	}
	if err := feasCfg.Validate(); err == nil {
		t.Errorf("expected error validating FeasibilityConfig with negative speed")
	}
}

func TestModelCornerCases_FlowConservationAndStateImmutability(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Now().Unix()

	driver := model.Driver{
		ID:                  "D-01",
		CurrentLocation:     locChi,
		HomeLocation:        locChi,
		AvailableEpoch:      startEpoch,
		DriveHoursRemaining: 11.0,
		DutyHoursRemaining:  14.0,
		Equipment:           model.Equipment{Type: model.EquipDryVan},
	}
	load := model.Load{
		ID:                  "L-01",
		Origin:              locChi,
		Destination:         locAtl,
		Revenue:             2500.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 3600,
		DeliveryLatestEpoch: startEpoch + 48*3600,
	}

	res := model.NewResourceState([]model.Driver{driver}, []model.Load{load})
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	state, _ := model.NewState(res, info, model.NewMonopolisticBelief())

	// 1. Dispatch driver on valid action
	action := model.NewAction([]model.DriverLoadMatch{
		{DriverID: "D-01", LoadID: "L-01", DispatchEpoch: startEpoch},
	}, nil)

	sNext, err := state.Transition(action, nil)
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	// Inviolate 5: Parent state must remain completely unaltered
	if state.Resource().Drivers()[0].CurrentLocation.NodeID != "CHI" {
		t.Errorf("Parent state driver location mutated: got %s",
			state.Resource().Drivers()[0].CurrentLocation.NodeID)
	}
	if state.Resource().Drivers()[0].AssignedLoadID != "" {
		t.Errorf("Parent state driver assigned load mutated: got %s",
			state.Resource().Drivers()[0].AssignedLoadID)
	}

	// Transitioned state must reflect new location and assignment
	if sNext.Resource().Drivers()[0].CurrentLocation.NodeID != "ATL" {
		t.Errorf("Next state driver location expected ATL, got %s",
			sNext.Resource().Drivers()[0].CurrentLocation.NodeID)
	}
	if len(sNext.Resource().Loads()) != 0 {
		t.Errorf("Next state loads expected 0 (assigned load cleared), got %d",
			len(sNext.Resource().Loads()))
	}

	// 2. Duplicate dispatch attempt on already-busy driver must fail closed (Inviolate 8)
	duplicateAction := model.NewAction([]model.DriverLoadMatch{
		{DriverID: "D-01", LoadID: "L-01", DispatchEpoch: startEpoch},
	}, nil)
	if _, err := sNext.Transition(duplicateAction, nil); err == nil {
		t.Errorf("expected fail-closed error when attempting to dispatch already-busy driver")
	}
}
