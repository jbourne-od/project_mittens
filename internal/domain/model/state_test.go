package model_test

import (
	"testing"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

func TestState_MonopolisticState(t *testing.T) {
	drivers := []model.Driver{{ID: "D-01"}}
	loads := []model.Load{{ID: "L-01"}}
	rs := model.NewResourceState(drivers, loads)

	info, err := model.NewInformationState(0, 1.0, 2.5, 0)
	if err != nil {
		t.Fatalf("NewInformationState failed: %v", err)
	}

	state, err := model.NewMonopolisticState(rs, info)
	if err != nil {
		t.Fatalf("NewMonopolisticState failed: %v", err)
	}

	// Verify MOMDP factoring components
	if state.Resource().DriverCount() != 1 {
		t.Fatalf("Resource DriverCount = %d, expected 1", state.Resource().DriverCount())
	}
	if state.Information().Epoch() != 0 {
		t.Fatalf("Information Epoch = %d, expected 0", state.Information().Epoch())
	}
	if state.Belief().Scale().CompetitorDimension() != 0 {
		t.Fatalf("Belief CompetitorDimension = %d, expected 0", state.Belief().Scale().CompetitorDimension())
	}
	if state.Belief().Probability(model.MonopolisticSingletonKey) != 1.0 {
		t.Fatalf("Belief Dirac delta = %f, expected 1.0", state.Belief().Probability(model.MonopolisticSingletonKey))
	}
}

func TestState_AggregatedMarketState(t *testing.T) {
	rs := model.NewResourceState(nil, nil)
	info, _ := model.NewInformationState(5, 1.1, 3.0, 2)
	b, _ := model.NewBelief(model.AggregatedMarket{}, []string{"c_high", "c_low"}, []float64{0.7, 0.3})

	state, err := model.NewState(rs, info, b)
	if err != nil {
		t.Fatalf("NewState failed: %v", err)
	}

	if state.Belief().Scale().CompetitorDimension() != 1 {
		t.Fatalf("CompetitorDimension = %d, expected 1", state.Belief().Scale().CompetitorDimension())
	}
	if state.Belief().Probability("c_high") != 0.7 {
		t.Fatalf("P(c_high) = %f, expected 0.7", state.Belief().Probability("c_high"))
	}
}

func TestState_NilValidation(t *testing.T) {
	rs := model.NewResourceState(nil, nil)
	info, _ := model.NewInformationState(0, 1.0, 2.0, 0)
	b := model.NewMonopolisticBelief()

	if _, err := model.NewState(nil, info, b); err == nil {
		t.Fatalf("expected error on nil ResourceState")
	}
	if _, err := model.NewState(rs, nil, b); err == nil {
		t.Fatalf("expected error on nil InformationState")
	}
	if _, err := model.NewState[model.Monopolistic](rs, info, nil); err == nil {
		t.Fatalf("expected error on nil Belief")
	}
}
