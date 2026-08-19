package model_test

import (
	"testing"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

func TestAction_FeasibilityAndSorting(t *testing.T) {
	drivers := []model.Driver{
		{ID: "D-02", AvailableEpoch: 0},
		{ID: "D-01", AvailableEpoch: 0},
	}
	loads := []model.Load{
		{ID: "L-02", Revenue: 1000.0},
		{ID: "L-01", Revenue: 1200.0},
	}
	rs := model.NewResourceState(drivers, loads)

	matches := []model.DriverLoadMatch{
		{DriverID: "D-02", LoadID: "L-02", EstimatedContribution: 500.0},
		{DriverID: "D-01", LoadID: "L-01", EstimatedContribution: 600.0},
	}
	bids := []model.SpotPriceBid{
		{LoadID: "L-02", BidPrice: 1100.0},
		{LoadID: "L-01", BidPrice: 1300.0},
	}

	act := model.NewAction(matches, bids)

	// Verify canonical sorting of matches: [D-01, D-02]
	mSlice := act.Matches()
	if mSlice[0].DriverID != "D-01" || mSlice[1].DriverID != "D-02" {
		t.Fatalf("matches not canonically sorted: %+v", mSlice)
	}

	// Verify canonical sorting of bids: [L-01, L-02]
	bSlice := act.Bids()
	if bSlice[0].LoadID != "L-01" || bSlice[1].LoadID != "L-02" {
		t.Fatalf("bids not canonically sorted: %+v", bSlice)
	}

	// Feasibility should pass
	if err := act.ValidateFeasibility(rs); err != nil {
		t.Fatalf("expected action to be feasible, got: %v", err)
	}
}

func TestAction_FeasibilityViolations(t *testing.T) {
	drivers := []model.Driver{
		{ID: "D-01", AssignedLoadID: "L-BUSY"}, // Busy driver
	}
	loads := []model.Load{
		{ID: "L-01", Revenue: 1000.0},
	}
	rs := model.NewResourceState(drivers, loads)

	// Busy driver match
	busyAct := model.NewAction([]model.DriverLoadMatch{{DriverID: "D-01", LoadID: "L-01"}}, nil)
	if err := busyAct.ValidateFeasibility(rs); err == nil {
		t.Fatalf("expected error on busy driver match")
	}

	// Non-positive bid price
	invalidBidAct := model.NewAction(nil, []model.SpotPriceBid{{LoadID: "L-01", BidPrice: -100.0}})
	if err := invalidBidAct.ValidateFeasibility(rs); err == nil {
		t.Fatalf("expected error on negative bid price")
	}
}
