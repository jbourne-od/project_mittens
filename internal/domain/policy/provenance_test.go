package policy_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

func TestPolicies_ProvenanceCompletenessAcrossAllBranches(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	sampleDrivers := []model.Driver{
		{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
		{ID: "D-02", CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
	}

	sampleLoads := []model.Load{
		{
			ID:                  "L-01",
			Origin:              locChi,
			Destination:         locAtl,
			RequiredEquipment:   model.EquipDryVan,
			Revenue:             3000.0,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 36000,
			DeliveryLatestEpoch: startEpoch + 120000,
		},
		{
			ID:                  "L-02",
			Origin:              locChi,
			Destination:         locAtl,
			RequiredEquipment:   model.EquipDryVan,
			Revenue:             3200.0,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 36000,
			DeliveryLatestEpoch: startEpoch + 120000,
		},
	}

	scale := model.MultiCompetitor{Count: 2}
	states := []string{"AGGRESSIVE", "PASSIVE"}
	probs := []float64{0.4, 0.6}
	belief, err := model.NewBelief(scale, states, probs)
	if err != nil {
		t.Fatalf("NewBelief failed: %v", err)
	}

	info, err := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	if err != nil {
		t.Fatalf("NewInformationState failed: %v", err)
	}

	rm := model.NewRegionManager(1.0, nil)
	costCfg := model.DefaultCostConfig()
	feasCfg := model.DefaultFeasibilityConfig()

	cfa := policy.NewCFAPolicy[model.MultiCompetitor](
		policy.DefaultCFAParameters(),
		costCfg,
		feasCfg,
		rm,
	)

	vfa, err := policy.NewVFAPolicy[model.MultiCompetitor](
		policy.NewVFATable(nil),
		0.95,
		costCfg,
		feasCfg,
		rm,
	)
	if err != nil {
		t.Fatalf("NewVFAPolicy failed: %v", err)
	}

	piecewiseVFA, err := policy.NewPiecewiseVFAPolicy[model.MultiCompetitor](
		policy.NewPiecewiseLinearVFATable(nil),
		nil,
		0.95,
		costCfg,
		feasCfg,
		rm,
	)
	if err != nil {
		t.Fatalf("NewPiecewiseVFAPolicy failed: %v", err)
	}

	dlaParams := policy.DefaultDLAParameters()
	dlaParams.Horizon = 1
	dlaParams.NumRollouts = 1
	dla, err := policy.NewDLAPolicy[model.MultiCompetitor](
		dlaParams,
		costCfg,
		feasCfg,
		cfa,
		nil,
		rm,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("NewDLAPolicy failed: %v", err)
	}

	compCFA, err := policy.NewCompetitivePOMDPPolicy[model.MultiCompetitor](
		cfa,
		policy.DefaultCompetitivePricingConfig(),
	)
	if err != nil {
		t.Fatalf("NewCompetitivePOMDPPolicy failed: %v", err)
	}

	policies := []struct {
		name         string
		pol          policy.Policy[model.MultiCompetitor]
		expectedName string
		minThetaLen  int
	}{
		{"CFA", cfa, "CFA_Parametric", 4},
		{"VFA", vfa, "VFA_LinearPostDecision", 1},
		{"PiecewiseVFA", piecewiseVFA, "VFA_PiecewiseLinearConcave", 1},
		{"DLA", dla, "DLA_Lookahead_H1_K1", 3},
		{"CompetitivePOMDP", compCFA, "CompetitivePOMDP_CFA_Parametric", 4},
	}

	testCases := []struct {
		name       string
		drivers    []model.Driver
		loads      []model.Load
		expDrivers int
		expLoads   int
	}{
		{"EmptyDrivers", nil, sampleLoads, 0, 2},
		{"EmptyLoads", sampleDrivers, nil, 2, 0},
		{"EmptyBoth", nil, nil, 0, 0},
		{"NormalMatching", sampleDrivers, sampleLoads, 2, 2},
	}

	for _, tc := range testCases {
		res := model.NewResourceState(tc.drivers, tc.loads)
		state, err := model.NewState(res, info, belief)
		if err != nil {
			t.Fatalf("NewState failed for %s: %v", tc.name, err)
		}

		for _, p := range policies {
			t.Run(tc.name+"/"+p.name, func(t *testing.T) {
				action, prov, err := p.pol.Evaluate(context.Background(), state)
				if err != nil {
					t.Fatalf("Evaluate failed: %v", err)
				}
				if action == nil {
					t.Fatalf("expected non-nil action")
				}

				// Check PolicyName
				if prov.PolicyName != p.expectedName {
					t.Errorf("expected policy name %s, got %s", p.expectedName, prov.PolicyName)
				}

				// Check Dimensions
				if prov.DriverCount != tc.expDrivers {
					t.Errorf("expected DriverCount %d, got %d", tc.expDrivers, prov.DriverCount)
				}
				if prov.LoadCount != tc.expLoads {
					t.Errorf("expected LoadCount %d, got %d", tc.expLoads, prov.LoadCount)
				}
				if prov.CompetitorDimension != 2 {
					t.Errorf("expected CompetitorDimension 2, got %d", prov.CompetitorDimension)
				}

				// Check ActiveBelief
				if prov.ActiveBelief == nil {
					t.Errorf("expected non-nil ActiveBelief")
				} else {
					if prov.ActiveBelief["AGGRESSIVE"] != 0.4 || prov.ActiveBelief["PASSIVE"] != 0.6 {
						t.Errorf("unexpected ActiveBelief: %+v", prov.ActiveBelief)
					}
				}

				// Check ThetaParameters
				if len(prov.ThetaParameters) < p.minThetaLen {
					t.Errorf("expected at least %d theta parameters, got %d (%+v)", p.minThetaLen, len(prov.ThetaParameters), prov.ThetaParameters)
				}

				// Check BatchEpoch
				if prov.BatchEpoch != startEpoch {
					t.Errorf("expected BatchEpoch %d, got %d", startEpoch, prov.BatchEpoch)
				}

				// Check PricingVariables for Competitive POMDP
				if p.name == "CompetitivePOMDP" {
					if prov.PricingVariables == nil {
						t.Errorf("expected non-nil PricingVariables for CompetitivePOMDP")
					} else {
						if _, ok := prov.PricingVariables["min_hurdle"]; !ok {
							t.Errorf("missing min_hurdle in PricingVariables")
						}
						if _, ok := prov.PricingVariables["target_rate_per_mile"]; !ok {
							t.Errorf("missing target_rate_per_mile in PricingVariables")
						}
						if _, ok := prov.PricingVariables["aggressive_prob"]; !ok {
							t.Errorf("missing aggressive_prob in PricingVariables")
						}
						if _, ok := prov.PricingVariables["passive_prob"]; !ok {
							t.Errorf("missing passive_prob in PricingVariables")
						}
					}
				}
			})
		}
	}
}

func TestPolicies_FailClosedMissingEntityLookups(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	validDriver := model.Driver{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}}
	validLoad := model.Load{
		ID:                  "L-01",
		Origin:              locChi,
		Destination:         locAtl,
		RequiredEquipment:   model.EquipDryVan,
		Revenue:             3000.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 36000,
		DeliveryLatestEpoch: startEpoch + 120000,
	}

	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()

	rm := model.NewRegionManager(1.0, nil)
	costCfg := model.DefaultCostConfig()
	feasCfg := model.DefaultFeasibilityConfig()

	cfa := policy.NewCFAPolicy[model.Monopolistic](policy.DefaultCFAParameters(), costCfg, feasCfg, rm)
	vfa, err := policy.NewVFAPolicy[model.Monopolistic](policy.NewVFATable(nil), 0.95, costCfg, feasCfg, rm)
	if err != nil {
		t.Fatalf("NewVFAPolicy failed: %v", err)
	}
	piecewiseVFA, err := policy.NewPiecewiseVFAPolicy[model.Monopolistic](policy.NewPiecewiseLinearVFATable(nil), nil, 0.95, costCfg, feasCfg, rm)
	if err != nil {
		t.Fatalf("NewPiecewiseVFAPolicy failed: %v", err)
	}
	dlaParams := policy.DefaultDLAParameters()
	dlaParams.Horizon = 1
	dlaParams.NumRollouts = 1
	dla, err := policy.NewDLAPolicy[model.Monopolistic](dlaParams, costCfg, feasCfg, cfa, nil, rm, nil, nil)
	if err != nil {
		t.Fatalf("NewDLAPolicy failed: %v", err)
	}

	t.Run("CFA_NilState_FailClosed", func(t *testing.T) {
		_, _, err := cfa.Evaluate(context.Background(), nil)
		if err == nil {
			t.Errorf("expected error on nil state")
		}
	})

	t.Run("VFA_NilState_FailClosed", func(t *testing.T) {
		_, _, err := vfa.Evaluate(context.Background(), nil)
		if err == nil {
			t.Errorf("expected error on nil state")
		}
	})

	t.Run("PiecewiseVFA_NilState_FailClosed", func(t *testing.T) {
		_, _, err := piecewiseVFA.Evaluate(context.Background(), nil)
		if err == nil {
			t.Errorf("expected error on nil state")
		}
	})

	t.Run("DLA_NilState_FailClosed", func(t *testing.T) {
		_, _, err := dla.Evaluate(context.Background(), nil)
		if err == nil {
			t.Errorf("expected error on nil state")
		}
	})

	// Verify that state with driver and load successfully evaluates
	resValid := model.NewResourceState([]model.Driver{validDriver}, []model.Load{validLoad})
	stateValid, err := model.NewState(resValid, info, belief)
	if err != nil {
		t.Fatalf("NewState failed: %v", err)
	}

	_, _, err = cfa.Evaluate(context.Background(), stateValid)
	if err != nil {
		t.Fatalf("expected valid state to pass CFA evaluation: %v", err)
	}
	_, _, err = vfa.Evaluate(context.Background(), stateValid)
	if err != nil {
		t.Fatalf("expected valid state to pass VFA evaluation: %v", err)
	}
	_, _, err = piecewiseVFA.Evaluate(context.Background(), stateValid)
	if err != nil {
		t.Fatalf("expected valid state to pass PiecewiseVFA evaluation: %v", err)
	}
	_, _, err = dla.Evaluate(context.Background(), stateValid)
	if err != nil {
		t.Fatalf("expected valid state to pass DLA evaluation: %v", err)
	}
}

func TestCompetitivePOMDPPolicy_NilState_FailClosed(t *testing.T) {
	cfa := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)
	compPol, err := policy.NewCompetitivePOMDPPolicy[model.Monopolistic](
		cfa,
		policy.DefaultCompetitivePricingConfig(),
	)
	if err != nil {
		t.Fatalf("NewCompetitivePOMDPPolicy failed: %v", err)
	}

	_, _, err = compPol.Evaluate(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected error on nil state")
	}
	if !strings.Contains(err.Error(), "cannot evaluate nil state") {
		t.Errorf("unexpected error message: %v", err)
	}
}
