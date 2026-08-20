package policy_test

import (
	"context"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

func createDLAExpansionTestState() *model.State[model.Monopolistic] {
	startEpoch := time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC).Unix()

	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locDal := model.Location{NodeID: "DAL", Lat: 32.7767, Lon: -96.7970}
	locClt := model.Location{NodeID: "CLT", Lat: 35.2271, Lon: -80.8431}

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
		{
			ID:                  "DRV-02",
			CurrentLocation:     locAtl,
			HomeLocation:        locAtl,
			AvailableEpoch:      startEpoch,
			DriveHoursRemaining: 11.0,
			DutyHoursRemaining:  14.0,
			Equipment:           model.Equipment{Type: model.EquipDryVan},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
		{
			ID:                  "DRV-03",
			CurrentLocation:     locDal,
			HomeLocation:        locDal,
			AvailableEpoch:      startEpoch,
			DriveHoursRemaining: 11.0,
			DutyHoursRemaining:  14.0,
			Equipment:           model.Equipment{Type: model.EquipDryVan},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
	}

	loads := []model.Load{
		{
			ID:                    "LOAD-CHI-ATL",
			Origin:                locChi,
			Destination:           locAtl,
			PickupEarliestEpoch:   startEpoch,
			PickupLatestEpoch:     startEpoch + 36000,
			DeliveryEarliestEpoch: startEpoch + 36000,
			DeliveryLatestEpoch:   startEpoch + 86400,
			Revenue:               2200.0,
			RequiredEquipment:     model.EquipDryVan,
		},
		{
			ID:                    "LOAD-ATL-CLT",
			Origin:                locAtl,
			Destination:           locClt,
			PickupEarliestEpoch:   startEpoch,
			PickupLatestEpoch:     startEpoch + 36000,
			DeliveryEarliestEpoch: startEpoch + 36000,
			DeliveryLatestEpoch:   startEpoch + 86400,
			Revenue:               1800.0,
			RequiredEquipment:     model.EquipDryVan,
		},
		{
			ID:                    "LOAD-DAL-CHI",
			Origin:                locDal,
			Destination:           locChi,
			PickupEarliestEpoch:   startEpoch,
			PickupLatestEpoch:     startEpoch + 36000,
			DeliveryEarliestEpoch: startEpoch + 36000,
			DeliveryLatestEpoch:   startEpoch + 86400,
			Revenue:               2500.0,
			RequiredEquipment:     model.EquipDryVan,
		},
		{
			ID:                    "LOAD-CHI-CLT",
			Origin:                locChi,
			Destination:           locClt,
			PickupEarliestEpoch:   startEpoch,
			PickupLatestEpoch:     startEpoch + 36000,
			DeliveryEarliestEpoch: startEpoch + 36000,
			DeliveryLatestEpoch:   startEpoch + 86400,
			Revenue:               1950.0,
			RequiredEquipment:     model.EquipDryVan,
		},
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, _ := model.NewState(res, info, belief)
	return state
}

func testArrivalSampler(epoch int64, stepIndex int, rng *pkgmath.RNG) []model.Load {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	return []model.Load{
		{
			ID:                    "LOAD-FUTURE-1",
			Origin:                locAtl,
			Destination:           locChi,
			PickupEarliestEpoch:   epoch,
			PickupLatestEpoch:     epoch + 36000,
			DeliveryEarliestEpoch: epoch + 36000,
			DeliveryLatestEpoch:   epoch + 86400,
			Revenue:               2300.0,
			RequiredEquipment:     model.EquipDryVan,
		},
	}
}

func TestDLA_AdaptiveBeamPruning(t *testing.T) {
	ctx := context.Background()
	state := createDLAExpansionTestState()

	basePolicy := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	// 1. Run DLA with Adaptive Beam Pruning enabled (BeamWidth = 2)
	paramsPruned := policy.DefaultDLAParameters()
	paramsPruned.Horizon = 2
	paramsPruned.NumRollouts = 2
	paramsPruned.EnableAdaptivePruning = true
	paramsPruned.BeamWidth = 2

	dlaPruned := policy.NewDLAPolicy[model.Monopolistic](
		paramsPruned,
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		basePolicy,
		testArrivalSampler,
		nil,
		nil,
		nil,
	)

	actionPruned, provPruned, err := dlaPruned.Evaluate(ctx, state)
	if err != nil {
		t.Fatalf("pruned DLA evaluation failed: %v", err)
	}

	if actionPruned.MatchCount() == 0 {
		t.Errorf("expected non-zero matches from pruned DLA")
	}
	if provPruned.TotalNetContribution <= 0 {
		t.Errorf("expected positive net contribution, got %f", provPruned.TotalNetContribution)
	}

	// 2. Run DLA with Brute Force (BeamWidth = 0, no pruning)
	paramsFull := policy.DefaultDLAParameters()
	paramsFull.Horizon = 2
	paramsFull.NumRollouts = 2
	paramsFull.EnableAdaptivePruning = false
	paramsFull.BeamWidth = 0

	dlaFull := policy.NewDLAPolicy[model.Monopolistic](
		paramsFull,
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		basePolicy,
		testArrivalSampler,
		nil,
		nil,
		nil,
	)

	actionFull, provFull, err := dlaFull.Evaluate(ctx, state)
	if err != nil {
		t.Fatalf("full DLA evaluation failed: %v", err)
	}

	if actionFull.MatchCount() != actionPruned.MatchCount() {
		t.Errorf("match count divergence: pruned=%d, full=%d", actionPruned.MatchCount(), actionFull.MatchCount())
	}
	if provPruned.TotalNetContribution < provFull.TotalNetContribution*0.95 {
		t.Errorf("pruned contribution %f is significantly lower than full %f", provPruned.TotalNetContribution, provFull.TotalNetContribution)
	}
}
