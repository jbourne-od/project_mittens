package policy_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

func TestCompetitivePricingConfig_Validation(t *testing.T) {
	cfg := policy.DefaultCompetitivePricingConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("DefaultCompetitivePricingConfig should be valid: %v", err)
	}

	invalidBaseline := cfg
	invalidBaseline.BaselineRatePerMile = -1.0
	if err := invalidBaseline.Validate(); err == nil {
		t.Errorf("expected error on negative baseline rate")
	}

	invalidSurplus := cfg
	invalidSurplus.SurplusRatePerMile = 1.0 // < baseline
	if err := invalidSurplus.Validate(); err == nil {
		t.Errorf("expected error on surplus rate < baseline rate")
	}

	invalidThreshold := cfg
	invalidThreshold.PassiveSurplusThreshold = 1.5
	if err := invalidThreshold.Validate(); err == nil {
		t.Errorf("expected error on threshold > 1.0")
	}
}

func TestCompetitivePOMDPPolicy_EvaluateAndPricing(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{
			ID:                  "DRV-01",
			CurrentLocation:     locChi,
			HomeLocation:        locChi,
			AvailableEpoch:      startEpoch,
			DriveHoursRemaining: 11.0,
			DutyHoursRemaining:  14.0,
			Equipment:           model.Equipment{Type: model.EquipDryVan},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
	}

	// 588 miles from CHI to ATL
	loads := []model.Load{
		{
			ID:                    "LOAD-01",
			Origin:                locChi,
			Destination:           locAtl,
			RequiredEquipment:     model.EquipDryVan,
			PickupEarliestEpoch:   startEpoch,
			PickupLatestEpoch:     startEpoch + 36000,
			DeliveryEarliestEpoch: startEpoch + 18000,
			DeliveryLatestEpoch:   startEpoch + 120000,
			Revenue:               3000.0,
		},
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(startEpoch, 2.50, 3.50, 1)

	scale := model.AggregatedMarket{}
	states := []string{"AGGRESSIVE", "MODERATE", "PASSIVE"}

	cfa := policy.NewCFAPolicy[model.AggregatedMarket](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	t.Run("Passive Market Surge Pricing", func(t *testing.T) {
		// Passive probability 0.80 >= 0.65
		belief, _ := model.NewBelief(scale, states, []float64{0.10, 0.10, 0.80})
		state, err := model.NewState(res, info, belief)
		if err != nil {
			t.Fatalf("NewState failed: %v", err)
		}

		compPol, err := policy.NewCompetitivePOMDPPolicy[model.AggregatedMarket](
			cfa,
			policy.DefaultCompetitivePricingConfig(),
		)
		if err != nil {
			t.Fatalf("NewCompetitivePOMDPPolicy failed: %v", err)
		}

		action, prov, err := compPol.Evaluate(context.Background(), state)
		if err != nil {
			t.Fatalf("Evaluate failed: %v", err)
		}

		if action.MatchCount() != 1 {
			t.Fatalf("expected 1 match, got %d", action.MatchCount())
		}
		if len(action.Bids()) != 1 {
			t.Fatalf("expected 1 spot bid, got %d", len(action.Bids()))
		}

		bid := action.Bids()[0]
		if bid.LoadID != "LOAD-01" {
			t.Errorf("expected bid for LOAD-01, got %s", bid.LoadID)
		}

		// Expected rate = SurplusRatePerMile (2.92)
		dist := locChi.DistanceMiles(locAtl)
		expectedPrice := dist * 2.92
		if bid.BidPrice < expectedPrice {
			t.Errorf("expected bid price >= %.2f, got %.2f", expectedPrice, bid.BidPrice)
		}

		// Verify provenance completeness (Inviolate 7)
		if prov.PolicyName != "CompetitivePOMDP_CFA_Parametric" {
			t.Errorf("expected policy name CompetitivePOMDP_CFA_Parametric, got %s", prov.PolicyName)
		}
		if prov.CompetitorDimension != 1 {
			t.Errorf("expected CompetitorDimension 1, got %d", prov.CompetitorDimension)
		}
		if prov.DriverCount != 1 || prov.LoadCount != 1 {
			t.Errorf("expected dimensions (1, 1), got (%d, %d)", prov.DriverCount, prov.LoadCount)
		}
		if prov.PricingVariables == nil || prov.PricingVariables["target_rate_per_mile"] != 2.92 {
			t.Errorf("expected target_rate_per_mile 2.92 in pricing variables, got %+v", prov.PricingVariables)
		}
		if len(prov.ActiveBelief) != 3 {
			t.Errorf("expected 3 states in ActiveBelief, got %d", len(prov.ActiveBelief))
		}
	})

	t.Run("Aggressive Market Hurdle Elevation", func(t *testing.T) {
		// Aggressive probability 0.90
		belief, _ := model.NewBelief(scale, states, []float64{0.90, 0.05, 0.05})
		state, err := model.NewState(res, info, belief)
		if err != nil {
			t.Fatalf("NewState failed: %v", err)
		}

		compPol, _ := policy.NewCompetitivePOMDPPolicy[model.AggregatedMarket](
			cfa,
			policy.DefaultCompetitivePricingConfig(),
		)

		action, prov, err := compPol.Evaluate(context.Background(), state)
		if err != nil {
			t.Fatalf("Evaluate failed: %v", err)
		}

		if len(action.Bids()) != 1 {
			t.Fatalf("expected 1 spot bid, got %d", len(action.Bids()))
		}

		bid := action.Bids()[0]
		// Under aggressive market, baseline rate (2.52) is used, but hurdle = 75 + 125 * 0.90 = $187.50
		dist := locChi.DistanceMiles(locAtl)
		nominalPrice := dist * 2.52
		if bid.BidPrice < nominalPrice {
			t.Errorf("expected bid price >= nominal rate, got %.2f vs %.2f", bid.BidPrice, nominalPrice)
		}

		if prov.PricingVariables["min_hurdle"] != 75.0+125.0*0.90 {
			t.Errorf("expected min_hurdle 187.50, got %.2f", prov.PricingVariables["min_hurdle"])
		}
	})

	t.Run("Concurrent Evaluation Safety", func(t *testing.T) {
		belief, _ := model.NewBelief(scale, states, []float64{0.33, 0.33, 0.34})
		state, _ := model.NewState(res, info, belief)
		compPol, _ := policy.NewCompetitivePOMDPPolicy[model.AggregatedMarket](
			cfa,
			policy.DefaultCompetitivePricingConfig(),
		)

		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				act, _, err := compPol.Evaluate(context.Background(), state)
				if err != nil {
					t.Errorf("concurrent evaluate error: %v", err)
					return
				}
				if act.MatchCount() != 1 || len(act.Bids()) != 1 {
					t.Errorf("unexpected match or bid count")
				}
			}()
		}
		wg.Wait()
	})
}

func TestCompetitivePOMDPPolicy_MonopolisticDegeneracy(t *testing.T) {
	// Inviolate 1: N=0 Monopolistic Degeneracy must collapse cleanly to baseline pricing without competitive drift
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
	}
	loads := []model.Load{
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
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, err := model.NewState(res, info, belief)
	if err != nil {
		t.Fatalf("NewState failed: %v", err)
	}

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

	action, prov, err := compPol.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if action.MatchCount() != 1 {
		t.Fatalf("expected 1 match, got %d", action.MatchCount())
	}
	if len(action.Bids()) != 1 {
		t.Fatalf("expected 1 spot bid, got %d", len(action.Bids()))
	}

	// For N=0, aggressiveProb = 0 and passiveProb = 0
	if prov.PricingVariables["aggressive_prob"] != 0.0 {
		t.Errorf("expected aggressive_prob 0.0 for N=0, got %f", prov.PricingVariables["aggressive_prob"])
	}
	if prov.PricingVariables["passive_prob"] != 0.0 {
		t.Errorf("expected passive_prob 0.0 for N=0, got %f", prov.PricingVariables["passive_prob"])
	}
	if prov.PricingVariables["min_hurdle"] != 75.0 {
		t.Errorf("expected nominal hurdle 75.0 for N=0, got %f", prov.PricingVariables["min_hurdle"])
	}
	if prov.PricingVariables["target_rate_per_mile"] != 2.52 {
		t.Errorf("expected nominal rate 2.52 for N=0, got %f", prov.PricingVariables["target_rate_per_mile"])
	}
	if prov.CompetitorDimension != 0 {
		t.Errorf("expected CompetitorDimension 0, got %d", prov.CompetitorDimension)
	}
	if prov.ActiveBelief[model.MonopolisticSingletonKey] != 1.0 {
		t.Errorf("expected active belief at theta_empty = 1.0, got %f", prov.ActiveBelief[model.MonopolisticSingletonKey])
	}
}

func TestCompetitivePOMDPPolicy_MultiCompetitorGenericity(t *testing.T) {
	// Inviolate 3: Multi-agent markets (N=3) with composite latent state keys
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
	}
	loads := []model.Load{
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
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	scale := model.MultiCompetitor{Count: 3}

	states := []string{"c1_agg:c2_agg:c3_agg", "c1_agg:c2_pas:c3_mod", "c1_pas:c2_pas:c3_pas"}

	cfa := policy.NewCFAPolicy[model.MultiCompetitor](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	t.Run("Multi-Competitor Aggressive Concentration", func(t *testing.T) {
		// 70% chance of all-aggressive, 20% mixed, 10% all-passive
		probs := []float64{0.70, 0.20, 0.10}
		belief, err := model.NewBelief(scale, states, probs)
		if err != nil {
			t.Fatalf("NewBelief failed: %v", err)
		}
		state, err := model.NewState(res, info, belief)
		if err != nil {
			t.Fatalf("NewState failed: %v", err)
		}

		compPol, err := policy.NewCompetitivePOMDPPolicy[model.MultiCompetitor](
			cfa,
			policy.DefaultCompetitivePricingConfig(),
		)
		if err != nil {
			t.Fatalf("NewCompetitivePOMDPPolicy failed: %v", err)
		}

		action, prov, err := compPol.Evaluate(context.Background(), state)
		if err != nil {
			t.Fatalf("Evaluate failed: %v", err)
		}

		if action.MatchCount() != 1 {
			t.Fatalf("expected 1 match, got %d", action.MatchCount())
		}

		if prov.CompetitorDimension != 3 {
			t.Errorf("expected CompetitorDimension 3, got %d", prov.CompetitorDimension)
		}

		// Both c1_agg:c2_agg:c3_agg and c1_agg:c2_pas:c3_mod contain agg -> aggressiveProb >= 0.70
		if prov.PricingVariables["aggressive_prob"] < 0.70 {
			t.Errorf("expected aggressive_prob >= 0.70, got %f", prov.PricingVariables["aggressive_prob"])
		}
		if prov.PricingVariables["min_hurdle"] <= 75.0 {
			t.Errorf("expected min_hurdle to be elevated above 75.0, got %f", prov.PricingVariables["min_hurdle"])
		}
	})

	t.Run("Multi-Competitor Passive Surge", func(t *testing.T) {
		// 80% chance of all-passive market
		probs := []float64{0.10, 0.10, 0.80}
		belief, err := model.NewBelief(scale, states, probs)
		if err != nil {
			t.Fatalf("NewBelief failed: %v", err)
		}
		state, err := model.NewState(res, info, belief)
		if err != nil {
			t.Fatalf("NewState failed: %v", err)
		}

		compPol, err := policy.NewCompetitivePOMDPPolicy[model.MultiCompetitor](
			cfa,
			policy.DefaultCompetitivePricingConfig(),
		)
		if err != nil {
			t.Fatalf("NewCompetitivePOMDPPolicy failed: %v", err)
		}

		action, prov, err := compPol.Evaluate(context.Background(), state)
		if err != nil {
			t.Fatalf("Evaluate failed: %v", err)
		}

		if prov.PricingVariables["target_rate_per_mile"] != 2.92 {
			t.Errorf("expected target_rate_per_mile 2.92 for passive market, got %f", prov.PricingVariables["target_rate_per_mile"])
		}
		if len(action.Bids()) != 1 {
			t.Fatalf("expected 1 spot bid, got %d", len(action.Bids()))
		}
	})
}

func TestCompetitivePOMDPPolicy_CustomClassifiers(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
	}
	loads := []model.Load{
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
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	scale := model.MultiCompetitor{Count: 2}

	states := []string{"predatory_pricing", "balanced_regime", "capacity_shortage"}
	probs := []float64{0.75, 0.15, 0.10}

	belief, err := model.NewBelief(scale, states, probs)
	if err != nil {
		t.Fatalf("NewBelief failed: %v", err)
	}
	state, err := model.NewState(res, info, belief)
	if err != nil {
		t.Fatalf("NewState failed: %v", err)
	}

	cfa := policy.NewCFAPolicy[model.MultiCompetitor](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	cfg := policy.DefaultCompetitivePricingConfig()
	cfg.AggressiveClassifier = func(k string) bool {
		return k == "predatory_pricing"
	}
	cfg.PassiveClassifier = func(k string) bool {
		return k == "capacity_shortage"
	}

	compPol, err := policy.NewCompetitivePOMDPPolicy[model.MultiCompetitor](cfa, cfg)
	if err != nil {
		t.Fatalf("NewCompetitivePOMDPPolicy failed: %v", err)
	}

	_, prov, err := compPol.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if prov.PricingVariables["aggressive_prob"] != 0.75 {
		t.Errorf("expected aggressive_prob 0.75, got %f", prov.PricingVariables["aggressive_prob"])
	}
	if prov.PricingVariables["passive_prob"] != 0.10 {
		t.Errorf("expected passive_prob 0.10, got %f", prov.PricingVariables["passive_prob"])
	}
}

// faultyPolicy is a mock policy that returns matches for nonexistent loads to test fail-closed error handling
type faultyPolicy[C model.CompetitorScale] struct {
	invalidLoadID string
}

func (f *faultyPolicy[C]) Name() string {
	return "FaultyPolicy"
}

func (f *faultyPolicy[C]) Evaluate(ctx context.Context, state *model.State[C]) (*model.Action, policy.DecisionProvenance, error) {
	match := model.DriverLoadMatch{
		DriverID:      "D-01",
		LoadID:        f.invalidLoadID,
		DispatchEpoch: 1000,
	}
	return model.NewAction([]model.DriverLoadMatch{match}, nil), policy.NewDecisionProvenance(f.Name(), state, nil), nil
}

func TestCompetitivePOMDPPolicy_MissingLoadFailClosed(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{ID: "D-01", CurrentLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}},
	}
	res := model.NewResourceState(drivers, nil)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, _ := model.NewState(res, info, belief)

	faulty := &faultyPolicy[model.Monopolistic]{invalidLoadID: "NONEXISTENT-LOAD"}
	compPol, err := policy.NewCompetitivePOMDPPolicy[model.Monopolistic](
		faulty,
		policy.DefaultCompetitivePricingConfig(),
	)
	if err != nil {
		t.Fatalf("NewCompetitivePOMDPPolicy failed: %v", err)
	}

	_, _, err = compPol.Evaluate(context.Background(), state)
	if err == nil {
		t.Fatalf("expected fail-closed error on missing matched load, got nil")
	}
	if !strings.Contains(err.Error(), "matched load NONEXISTENT-LOAD not found in resource state") {
		t.Errorf("unexpected error message: %v", err)
	}
}
