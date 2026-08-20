package policy_test

import (
	"context"
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

		action, _, err := compPol.Evaluate(context.Background(), state)
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

		action, _, err := compPol.Evaluate(context.Background(), state)
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
