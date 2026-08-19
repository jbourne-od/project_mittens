package model_test

import (
	"math"
	"testing"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

func TestMonopolisticBelief_DegenerateCollapse(t *testing.T) {
	// Inviolate 1: Monopolistic Degeneracy (N=0) must collapse to Dirac delta distribution at \Theta_\emptyset
	b := model.NewMonopolisticBelief()

	if b.Scale().CompetitorDimension() != 0 {
		t.Fatalf("expected competitor dimension 0, got %d", b.Scale().CompetitorDimension())
	}
	if b.Dimension() != 1 {
		t.Fatalf("expected dimension 1, got %d", b.Dimension())
	}
	if b.Probability(model.MonopolisticSingletonKey) != 1.0 {
		t.Fatalf("expected probability 1.0 at %s, got %f", model.MonopolisticSingletonKey, b.Probability(model.MonopolisticSingletonKey))
	}
	if b.Probability("other_state") != 0.0 {
		t.Fatalf("expected probability 0.0 for non-existent state, got %f", b.Probability("other_state"))
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("Monopolistic belief failed validation: %v", err)
	}
}

func TestBelief_AggregatedMarketAndCanonicalSorting(t *testing.T) {
	scale := model.AggregatedMarket{LatentStates: []string{"high_capacity", "low_capacity", "balanced"}}
	keys := []string{"high_capacity", "low_capacity", "balanced"}
	probs := []float64{0.2, 0.5, 0.3}

	b, err := model.NewBelief(scale, keys, probs)
	if err != nil {
		t.Fatalf("NewBelief failed: %v", err)
	}

	if b.Scale().CompetitorDimension() != 1 {
		t.Fatalf("expected dimension 1, got %d", b.Scale().CompetitorDimension())
	}

	// Verify canonical sorting of keys: ["balanced", "high_capacity", "low_capacity"]
	sortedKeys := b.StateKeys()
	if sortedKeys[0] != "balanced" || sortedKeys[1] != "high_capacity" || sortedKeys[2] != "low_capacity" {
		t.Fatalf("keys not canonically sorted: %v", sortedKeys)
	}

	// Verify correct matching probabilities after sorting
	if math.Abs(b.Probability("balanced")-0.3) > 1e-12 {
		t.Fatalf("P(balanced) = %f; expected 0.3", b.Probability("balanced"))
	}
	if math.Abs(b.Probability("high_capacity")-0.2) > 1e-12 {
		t.Fatalf("P(high_capacity) = %f; expected 0.2", b.Probability("high_capacity"))
	}
	if math.Abs(b.Probability("low_capacity")-0.5) > 1e-12 {
		t.Fatalf("P(low_capacity) = %f; expected 0.5", b.Probability("low_capacity"))
	}
}

func TestBelief_MultiCompetitorGenericity(t *testing.T) {
	// Inviolate 3: Genericity for arbitrary N >= 0
	scale := model.MultiCompetitor{Count: 5}
	keys := []string{"posture_A", "posture_B"}
	probs := []float64{0.4, 0.6}

	b, err := model.NewBelief(scale, keys, probs)
	if err != nil {
		t.Fatalf("NewBelief failed: %v", err)
	}

	if b.Scale().CompetitorDimension() != 5 {
		t.Fatalf("expected CompetitorDimension 5, got %d", b.Scale().CompetitorDimension())
	}
}

func TestBelief_Immutability(t *testing.T) {
	scale := model.AggregatedMarket{}
	keys := []string{"state_1", "state_2"}
	probs := []float64{0.7, 0.3}

	b, _ := model.NewBelief(scale, keys, probs)

	// Mutating returned probabilities slice should not alter internal state (Inviolate 5)
	pSlice := b.Probabilities()
	pSlice[0] = 0.0
	if b.Probability("state_1") != 0.7 {
		t.Fatalf("mutating returned probability slice mutated internal Belief state")
	}

	// Mutating returned distribution map should not alter internal state
	dist := b.Distribution()
	dist["state_1"] = 0.0
	if b.Probability("state_1") != 0.7 {
		t.Fatalf("mutating returned distribution map mutated internal Belief state")
	}
}

func TestBelief_ValidationFailures(t *testing.T) {
	scale := model.AggregatedMarket{}

	// Non-summing probabilities (0.5 + 0.3 = 0.8 != 1.0)
	_, err := model.NewBelief(scale, []string{"s1", "s2"}, []float64{0.5, 0.3})
	if err == nil {
		t.Fatalf("expected error on simplex sum mismatch, got nil")
	}

	// Dimension mismatch
	_, err = model.NewBelief(scale, []string{"s1", "s2"}, []float64{1.0})
	if err == nil {
		t.Fatalf("expected error on dimension mismatch, got nil")
	}

	// Duplicate state key
	_, err = model.NewBelief(scale, []string{"s1", "s1"}, []float64{0.5, 0.5})
	if err == nil {
		t.Fatalf("expected error on duplicate state key, got nil")
	}

	// Empty keys
	_, err = model.NewBelief(scale, nil, nil)
	if err == nil {
		t.Fatalf("expected error on empty keys, got nil")
	}
}
