package benchmarks_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/internal/service"
	"github.com/optimaldynamics/project-mittens/internal/service/dispatch"
)

// BenchmarkRolling_7DaySimulation benchmarks a full 7-day multi-epoch continuous carrier simulation.
func BenchmarkRolling_7DaySimulation(b *testing.B) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locSdf := model.Location{NodeID: "SDF", Lat: 38.2527, Lon: -85.7585}

	startEpoch := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{ID: "D1", CurrentLocation: locChi, HomeLocation: locChi, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}, Clocks: hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0))},
		{ID: "D2", CurrentLocation: locAtl, HomeLocation: locAtl, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}, Clocks: hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0))},
		{ID: "D3", CurrentLocation: locSdf, HomeLocation: locSdf, AvailableEpoch: startEpoch, Equipment: model.Equipment{Type: model.EquipDryVan}, Clocks: hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0))},
	}

	loadStreamMap := make(map[int64][]model.Load)
	for day := 0; day < 7; day++ {
		epoch := startEpoch + int64(day*86400)
		loadStreamMap[epoch] = []model.Load{
			{ID: fmt.Sprintf("L_DAY_%d_A", day), Origin: locChi, Destination: locAtl, RequiredEquipment: model.EquipDryVan, Revenue: 3400.0, PickupEarliestEpoch: epoch, PickupLatestEpoch: epoch + 36000, DeliveryLatestEpoch: epoch + 120000},
			{ID: fmt.Sprintf("L_DAY_%d_B", day), Origin: locAtl, Destination: locChi, RequiredEquipment: model.EquipDryVan, Revenue: 3200.0, PickupEarliestEpoch: epoch, PickupLatestEpoch: epoch + 36000, DeliveryLatestEpoch: epoch + 120000},
		}
	}
	stream := service.NewStaticLoadStream(loadStreamMap)

	res := model.NewResourceState(drivers, nil)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, _ := model.NewState(res, info, belief)

	facilities := []model.Facility{
		{ID: "FAC-SDF-HUB", Location: locSdf, Type: model.FacilityRelayHub, OpenMinutesOfDay: 0, CloseMinutesOfDay: 1440, AverageDwellMinutes: 60},
	}
	facStore := model.NewFacilityStore(facilities)
	relaySynth := policy.NewRelaySynthesizer(model.DefaultCostConfig(), policy.DefaultRelayConfig(), hos.USPolicySpecs(), facStore, nil)
	relayRunner := dispatch.NewRelayDispatchRunner(nil, relaySynth)

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](policy.DefaultCFAParameters(), model.DefaultCostConfig(), model.DefaultFeasibilityConfig(), nil)
	simRunner := service.NewRollingHorizonRunner[model.Monopolistic](nil, relayRunner, nil, nil)

	cfg := service.RollingHorizonConfig{
		RunID:             "BENCH_7D",
		StartEpoch:        startEpoch,
		HorizonDays:       7,
		DecisionStepHours: 24,
		EnableRelays:      true,
		MinRelayHaulMiles: 450.0,
		EnableVFALearning: false,
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		report, _, err := simRunner.Run(ctx, cfg, state, cfaPol, stream)
		if err != nil {
			b.Fatalf("Run failed: %v", err)
		}
		if report.TotalEpochs != 7 {
			b.Fatalf("expected 7 epochs, got %d", report.TotalEpochs)
		}
	}
}
