package model_test

import (
	"testing"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

func TestResourceState_ImmutabilityAndCanonicalSorting(t *testing.T) {
	drivers := []model.Driver{
		{ID: "D-03", DutyHoursRemaining: 10.0, DriveHoursRemaining: 8.0},
		{ID: "D-01", DutyHoursRemaining: 12.0, DriveHoursRemaining: 9.0},
		{ID: "D-02", DutyHoursRemaining: 14.0, DriveHoursRemaining: 10.0},
	}
	loads := []model.Load{
		{ID: "L-99", Revenue: 1500.0},
		{ID: "L-10", Revenue: 2200.0},
	}

	rs := model.NewResourceState(drivers, loads)

	// Verify canonical sorting
	dSlice := rs.Drivers()
	if len(dSlice) != 3 || dSlice[0].ID != "D-01" || dSlice[1].ID != "D-02" || dSlice[2].ID != "D-03" {
		t.Fatalf("expected drivers canonically sorted [D-01, D-02, D-03], got: %+v", dSlice)
	}

	lSlice := rs.Loads()
	if len(lSlice) != 2 || lSlice[0].ID != "L-10" || lSlice[1].ID != "L-99" {
		t.Fatalf("expected loads canonically sorted [L-10, L-99], got: %+v", lSlice)
	}

	// Verify Immutability: Mutating returned slice should NOT alter ResourceState internal state (Inviolate 5)
	dSlice[0].DutyHoursRemaining = 0.0
	fetchedD1, _ := rs.GetDriver("D-01")
	if fetchedD1.DutyHoursRemaining != 12.0 {
		t.Fatalf("mutating returned driver slice mutated internal ResourceState (Inviolate 5 violation)")
	}
}

func TestResourceState_Transition(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locDal := model.Location{NodeID: "DAL", Lat: 32.7767, Lon: -96.7970}

	drivers := []model.Driver{
		{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: 0, DriveHoursRemaining: 10.0, DutyHoursRemaining: 12.0},
		{ID: "D-02", CurrentLocation: locAtl, AvailableEpoch: 0, DriveHoursRemaining: 11.0, DutyHoursRemaining: 14.0},
	}
	loads := []model.Load{
		{ID: "L-01", Origin: locChi, Destination: locAtl, Revenue: 2000.0, EstimatedTransitEpochs: 4},
		{ID: "L-02", Origin: locAtl, Destination: locDal, Revenue: 1800.0, EstimatedTransitEpochs: 3},
	}

	rs0 := model.NewResourceState(drivers, loads)

	// Match D-01 to L-01
	matches := []model.DriverLoadMatch{
		{DriverID: "D-01", LoadID: "L-01", DispatchEpoch: 0, EstimatedContribution: 1200.0},
	}
	newLoads := []model.Load{
		{ID: "L-03", Origin: locDal, Destination: locChi, Revenue: 2500.0, EstimatedTransitEpochs: 5},
	}

	rs1, err := rs0.Transition(matches, newLoads)
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	// Verify rs0 was unaltered (Inviolate 5)
	if rs0.LoadCount() != 2 {
		t.Fatalf("parent rs0 LoadCount modified: %d != 2", rs0.LoadCount())
	}
	parentD1, _ := rs0.GetDriver("D-01")
	if parentD1.CurrentLocation.NodeID != "CHI" {
		t.Fatalf("parent rs0 D-01 location was mutated")
	}

	// Verify rs1 transitioned state
	if rs1.LoadCount() != 2 { // L-02 remaining + L-03 new
		t.Fatalf("rs1 LoadCount = %d, expected 2", rs1.LoadCount())
	}
	d1, ok := rs1.GetDriver("D-01")
	if !ok || d1.CurrentLocation.NodeID != "ATL" {
		t.Fatalf("D-01 in rs1 not at destination ATL: %+v", d1)
	}
	if d1.AvailableEpoch != 4 {
		t.Fatalf("D-01 AvailableEpoch = %d, expected 4", d1.AvailableEpoch)
	}
	if d1.DriveHoursRemaining != 6.0 {
		t.Fatalf("D-01 DriveHoursRemaining = %f, expected 6.0", d1.DriveHoursRemaining)
	}

	// Verify matched load L-01 is removed, and L-03 was added
	if _, exists := rs1.GetLoad("L-01"); exists {
		t.Fatalf("matched load L-01 still present in rs1")
	}
	if _, exists := rs1.GetLoad("L-03"); !exists {
		t.Fatalf("new load L-03 not found in rs1")
	}
}

func TestResourceState_TransitionErrors(t *testing.T) {
	drivers := []model.Driver{
		{ID: "D-01", AvailableEpoch: 0},
	}
	loads := []model.Load{
		{ID: "L-01", Revenue: 1000.0},
	}
	rs := model.NewResourceState(drivers, loads)

	// Non-existent driver
	_, err := rs.Transition([]model.DriverLoadMatch{{DriverID: "D-UNKNOWN", LoadID: "L-01"}}, nil)
	if err == nil {
		t.Fatalf("expected error for non-existent driver")
	}

	// Non-existent load
	_, err = rs.Transition([]model.DriverLoadMatch{{DriverID: "D-01", LoadID: "L-UNKNOWN"}}, nil)
	if err == nil {
		t.Fatalf("expected error for non-existent load")
	}

	// Duplicate match
	_, err = rs.Transition([]model.DriverLoadMatch{
		{DriverID: "D-01", LoadID: "L-01"},
		{DriverID: "D-01", LoadID: "L-01"},
	}, nil)
	if err == nil {
		t.Fatalf("expected error on duplicate match")
	}
}
