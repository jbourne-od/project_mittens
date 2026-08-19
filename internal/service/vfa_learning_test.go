package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/internal/service"
	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

func TestPiecewiseVFALearner_MultiEpochConvergence(t *testing.T) {
	// Setup 3 regions: CHI, ATL, NYC
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locNyc := model.Location{NodeID: "NYC", Lat: 40.7128, Lon: -74.0060}

	rm := model.NewRegionManager(1.0, nil)
	chiReg := rm.GetRegionID(locChi)
	atlReg := rm.GetRegionID(locAtl)
	nycReg := rm.GetRegionID(locNyc)

	regionIndexMap := map[string]int{
		chiReg: 0,
		atlReg: 1,
		nycReg: 2,
	}

	distMatrix, _ := pkgmath.NewDenseMatrixWithData(3, 3, []float64{
		0, 588, 790,
		588, 0, 748,
		790, 748, 0,
	})

	kernelCfg := pkgmath.SpatialKernelConfig{
		SignalVariance:   10000.0,
		LengthScaleMiles: 400.0,
		NoiseVariance:    10.0,
	}
	cov, err := pkgmath.BuildSpatialCovariance(distMatrix, kernelCfg)
	if err != nil {
		t.Fatalf("BuildSpatialCovariance failed: %v", err)
	}

	priorCKG, _ := pkgmath.NewCorrelatedKnowledgeGradient([]float64{200.0, 200.0, 200.0}, cov)

	learnerCfg := service.VFALearningConfig{
		StepSize:          0.2,
		HarmonicStepSize:  true,
		HarmonicA:         10.0,
		MaxSlopes:         5,
		UseCKG:            true,
		CKGObservationVar: 25.0,
	}

	learner := service.NewPiecewiseVFALearner(nil, priorCKG, rm, regionIndexMap, learnerCfg)

	baseEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	// Simulate 10 sequential optimization epochs with alternating high-value freight patterns
	for epochIdx := 0; epochIdx < 10; epochIdx++ {
		currentEpoch := baseEpoch + int64(epochIdx*86400)

		drivers := []model.Driver{
			{
				ID:              "D-01",
				CurrentLocation: locChi,
				AvailableEpoch:  currentEpoch,
				Equipment:       model.Equipment{Type: model.EquipDryVan},
				Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(currentEpoch, 0)),
			},
			{
				ID:              "D-02",
				CurrentLocation: locAtl,
				AvailableEpoch:  currentEpoch,
				Equipment:       model.Equipment{Type: model.EquipDryVan},
				Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(currentEpoch, 0)),
			},
		}

		loads := []model.Load{
			{
				ID:                  fmt.Sprintf("L-CHI-NYC-%d", epochIdx),
				Origin:              locChi,
				Destination:         locNyc,
				RequiredEquipment:   model.EquipDryVan,
				Revenue:             3200.0,
				PickupEarliestEpoch: currentEpoch,
				PickupLatestEpoch:   currentEpoch + 36000,
				DeliveryLatestEpoch: currentEpoch + 120000,
			},
			{
				ID:                  fmt.Sprintf("L-ATL-CHI-%d", epochIdx),
				Origin:              locAtl,
				Destination:         locChi,
				RequiredEquipment:   model.EquipDryVan,
				Revenue:             2600.0,
				PickupEarliestEpoch: currentEpoch,
				PickupLatestEpoch:   currentEpoch + 36000,
				DeliveryLatestEpoch: currentEpoch + 120000,
			},
		}

		resState := model.NewResourceState(drivers, loads)
		infoState, _ := model.NewInformationState(currentEpoch, 1.0, 2.50, 0)
		belief := model.NewMonopolisticBelief()
		state, err := model.NewState(resState, infoState, belief)
		if err != nil {
			t.Fatalf("epoch %d: NewState failed: %v", epochIdx, err)
		}

		costCfg := model.DefaultCostConfig()
		feasCfg := model.FeasibilityConfig{AverageSpeedMPH: 50.0, HOSPolicySpecs: hos.USPolicySpecs()}

		// Active Piecewise VFA Policy using current learned slopes
		vfaPolicy := policy.NewPiecewiseVFAPolicy[model.Monopolistic](
			learner.Table(),
			learner.CKG(),
			0.95,
			costCfg,
			feasCfg,
			rm,
		)

		action, prov, err := vfaPolicy.Evaluate(context.Background(), state)
		if err != nil {
			t.Fatalf("epoch %d: Evaluate failed: %v", epochIdx, err)
		}

		if action.MatchCount() != 2 {
			t.Fatalf("epoch %d: expected 2 matches, got %d", epochIdx, action.MatchCount())
		}

		// Re-evaluate matching solution with detailed dual potential extraction
		matchingSolution := policy.MatchingSolution{
			Matches:              action.Matches(),
			Evaluations:          prov.EvaluatedArcs,
			TotalObjective:       prov.TotalObjectiveValue,
			TotalNetContribution: prov.TotalNetContribution,
			DriverDualValues: map[string]float64{
				"D-01": 1500.0 + float64(epochIdx)*20.0,
				"D-02": 1100.0 + float64(epochIdx)*15.0,
			},
		}

		// Adaptive learning step
		learner, err = learner.UpdateFromMatching(matchingSolution, drivers, loads)
		if err != nil {
			t.Fatalf("epoch %d: UpdateFromMatching failed: %v", epochIdx, err)
		}

		// Assert that learned slopes in NYC and CHI strictly satisfy concavity
		for _, reg := range []string{nycReg, chiReg} {
			rs, ok := learner.Table().GetRegionSlopes(reg)
			if ok {
				for i := 1; i < len(rs.Slopes); i++ {
					if rs.Slopes[i] > rs.Slopes[i-1]+1e-9 {
						t.Fatalf("epoch %d: concavity violated in region %s: slopes = %v", epochIdx, reg, rs.Slopes)
					}
				}
			}
		}
	}

	nycSlopes, _ := learner.Table().GetRegionSlopes(nycReg)
	t.Logf("Final learned NYC Piecewise Slopes: %v", nycSlopes.Slopes)
	t.Logf("Final CKG Posterior Spatial Means: %v", learner.CKG().Mean())

	if nycSlopes.Slopes[0] <= 200.0 {
		t.Errorf("expected NYC learned slope to increase from prior 200, got %v", nycSlopes.Slopes[0])
	}
}
