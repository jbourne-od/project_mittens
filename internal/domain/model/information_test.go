package model_test

import (
	"testing"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

func TestInformationState_CreationAndValidation(t *testing.T) {
	info, err := model.NewInformationState(1, 1.25, 2.85, 3)
	if err != nil {
		t.Fatalf("failed to create valid InformationState: %v", err)
	}

	if info.Epoch() != 1 {
		t.Fatalf("Epoch = %d; expected 1", info.Epoch())
	}
	if info.FuelPriceIndex() != 1.25 {
		t.Fatalf("FuelPriceIndex = %f; expected 1.25", info.FuelPriceIndex())
	}
	if info.NationalSpotRateIndex() != 2.85 {
		t.Fatalf("NationalSpotRateIndex = %f; expected 2.85", info.NationalSpotRateIndex())
	}
	if info.WeatherAlertCount() != 3 {
		t.Fatalf("WeatherAlertCount = %d; expected 3", info.WeatherAlertCount())
	}

	// Negative epoch
	if _, err := model.NewInformationState(-1, 1.0, 2.0, 0); err == nil {
		t.Fatalf("expected error on negative epoch")
	}

	// Negative fuel price
	if _, err := model.NewInformationState(0, -1.0, 2.0, 0); err == nil {
		t.Fatalf("expected error on negative fuel price")
	}
}

func TestInformationState_Transition(t *testing.T) {
	info0, _ := model.NewInformationState(0, 1.0, 2.5, 1)

	info1, err := info0.Transition(1, 1.05, 2.6, 2)
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	// Verify info0 was not modified (Inviolate 5)
	if info0.Epoch() != 0 || info0.FuelPriceIndex() != 1.0 {
		t.Fatalf("parent info0 was mutated")
	}

	if info1.Epoch() != 1 || info1.FuelPriceIndex() != 1.05 {
		t.Fatalf("info1 state incorrect: epoch=%d, fuel=%f", info1.Epoch(), info1.FuelPriceIndex())
	}

	// Stale / non-advancing epoch transition should fail
	if _, err := info1.Transition(1, 1.1, 2.7, 0); err == nil {
		t.Fatalf("expected error on non-advancing epoch transition")
	}
}
