package policy_test

import (
	"context"
	"testing"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

func TestPFAPolicy_GreedyAnalyticalEvaluation(t *testing.T) {
	rng := pkgmath.NewRNG(20260821)
	cities := []model.Location{
		{NodeID: "NYC", Lat: 40.7128, Lon: -74.0060},
		{NodeID: "PHL", Lat: 39.9526, Lon: -75.1652},
		{NodeID: "BOS", Lat: 42.3601, Lon: -71.0589},
	}

	drivers := []model.Driver{
		{
			ID:              "DRV_01",
			CurrentLocation: cities[0],
			HomeLocation:    cities[0],
			AvailableEpoch:  1700000000,
			Equipment:       model.Equipment{Type: model.EquipDryVan},
		},
		{
			ID:              "DRV_02",
			CurrentLocation: cities[1],
			HomeLocation:    cities[1],
			AvailableEpoch:  1700003600,
			Equipment:       model.Equipment{Type: model.EquipDryVan},
		},
	}

	loads := []model.Load{
		{
			ID:                "LOAD_01",
			Origin:            cities[0],
			Destination:       cities[2],
			Revenue:           1200.0,
			RequiredEquipment: model.EquipDryVan,
		},
		{
			ID:                "LOAD_02",
			Origin:            cities[1],
			Destination:       cities[0],
			Revenue:           800.0,
			RequiredEquipment: model.EquipDryVan,
		},
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(1700000000, 2.50, 3.85, len(loads))
	state, _ := model.NewState(res, info, model.NewMonopolisticBelief())

	pfaParams := policy.DefaultPFAParameters()
	costCfg := model.DefaultCostConfig()
	feasCfg := model.DefaultFeasibilityConfig()

	pfa := policy.NewPFAPolicy[model.Monopolistic](pfaParams, costCfg, feasCfg)

	if pfa.Name() != "PFA_GreedyPriorityRule" {
		t.Errorf("unexpected name: %s", pfa.Name())
	}

	action, prov, err := pfa.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("PFA evaluation failed: %v", err)
	}

	if len(action.Matches()) != 2 {
		t.Errorf("expected 2 matches, got %d", len(action.Matches()))
	}
	if prov.MatchedCount != 2 {
		t.Errorf("expected prov.MatchedCount = 2, got %d", prov.MatchedCount)
	}
	if prov.TotalNetContribution <= 0 {
		t.Errorf("expected positive contribution, got %f", prov.TotalNetContribution)
	}
	_ = rng
}
