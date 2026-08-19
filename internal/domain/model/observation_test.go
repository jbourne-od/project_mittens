package model_test

import (
	"testing"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

func TestObservation_CanonicalSortingAndImmutability(t *testing.T) {
	loads := []model.Load{
		{ID: "L-99", Revenue: 1000.0},
		{ID: "L-01", Revenue: 2000.0},
	}
	feedback := []model.BidFeedback{
		{LoadID: "L-99", Won: false, WinningPrice: 950.0},
		{LoadID: "L-01", Won: true, WinningPrice: 2000.0},
	}

	obs := model.NewObservation(1, loads, feedback)

	if obs.Epoch() != 1 {
		t.Fatalf("Epoch = %d, expected 1", obs.Epoch())
	}
	if obs.LoadCount() != 2 || obs.FeedbackCount() != 2 {
		t.Fatalf("Counts mismatch: loads=%d, feedback=%d", obs.LoadCount(), obs.FeedbackCount())
	}

	// Verify canonical sorting
	lSlice := obs.Loads()
	if lSlice[0].ID != "L-01" || lSlice[1].ID != "L-99" {
		t.Fatalf("loads not canonically sorted: %+v", lSlice)
	}

	fSlice := obs.Feedback()
	if fSlice[0].LoadID != "L-01" || fSlice[1].LoadID != "L-99" {
		t.Fatalf("feedback not canonically sorted: %+v", fSlice)
	}

	// Verify Immutability (Inviolate 5)
	lSlice[0].Revenue = 0.0
	if obs.Loads()[0].Revenue != 2000.0 {
		t.Fatalf("mutating returned slice mutated Observation internal state")
	}
}
