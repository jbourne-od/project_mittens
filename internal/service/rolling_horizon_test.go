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
	"github.com/optimaldynamics/project-mittens/internal/service/dispatch"
)

func TestRollingHorizonRunner_7DayMultiEpoch(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locSdf := model.Location{NodeID: "SDF", Lat: 38.2527, Lon: -85.7585}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locBna := model.Location{NodeID: "BNA", Lat: 36.1627, Lon: -86.7816}
	locMke := model.Location{NodeID: "MKE", Lat: 43.0389, Lon: -87.9065}

	startEpoch := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	// Initial fleet of 3 drivers
	drivers := []model.Driver{
		{
			ID:              "D-CHI",
			CurrentLocation: locChi,
			HomeLocation:    locChi,
			AvailableEpoch:  startEpoch,
			Equipment:       model.Equipment{Type: model.EquipDryVan},
			Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
		{
			ID:              "D-BNA",
			CurrentLocation: locBna,
			HomeLocation:    locBna,
			AvailableEpoch:  startEpoch,
			Equipment:       model.Equipment{Type: model.EquipDryVan},
			Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
		{
			ID:              "D-MKE",
			CurrentLocation: locChi,
			HomeLocation:    locMke,
			AvailableEpoch:  startEpoch,
			Equipment:       model.Equipment{Type: model.EquipDryVan},
			Clocks:          hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
	}

	// Dynamic load stream generating daily freight arrivals across the 7-day horizon
	loadStreamMap := make(map[int64][]model.Load)
	for day := 0; day < 7; day++ {
		epoch := startEpoch + int64(day*86400)
		loadStreamMap[epoch] = []model.Load{
			{
				ID:                  fmt.Sprintf("L-DAY-%d-LONG", day),
				Origin:              locChi,
				Destination:         locAtl,
				RequiredEquipment:   model.EquipDryVan,
				Revenue:             3600.0,
				PickupEarliestEpoch: epoch,
				PickupLatestEpoch:   epoch + 36000,
				DeliveryLatestEpoch: epoch + 120000,
			},
			{
				ID:                  fmt.Sprintf("L-DAY-%d-REG", day),
				Origin:              locChi,
				Destination:         locMke,
				RequiredEquipment:   model.EquipDryVan,
				Revenue:             1100.0,
				PickupEarliestEpoch: epoch,
				PickupLatestEpoch:   epoch + 36000,
				DeliveryLatestEpoch: epoch + 120000,
			},
		}
	}
	stream := service.NewStaticLoadStream(loadStreamMap)

	res := model.NewResourceState(drivers, nil)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, err := model.NewState(res, info, belief)
	if err != nil {
		t.Fatalf("NewState failed: %v", err)
	}

	facilities := []model.Facility{
		{
			ID:                  "FAC-SDF-HUB",
			Name:                "Louisville Relay Hub",
			Location:            locSdf,
			Type:                model.FacilityRelayHub,
			OpenMinutesOfDay:    0,
			CloseMinutesOfDay:   1440,
			AverageDwellMinutes: 60,
		},
	}
	facStore := model.NewFacilityStore(facilities)

	relaySynth := policy.NewRelaySynthesizer(
		model.DefaultCostConfig(),
		policy.DefaultRelayConfig(),
		hos.USPolicySpecs(),
		facStore,
		nil,
	)
	relayRunner := dispatch.NewRelayDispatchRunner(nil, relaySynth)

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	runner := service.NewRollingHorizonRunner[model.Monopolistic](
		nil,
		relayRunner,
		nil,
		nil,
	)

	cfg := service.RollingHorizonConfig{
		RunID:             "TEST_7D_ROLLING",
		StartEpoch:        startEpoch,
		HorizonDays:       7,
		DecisionStepHours: 24,
		EnableRelays:      true,
		MinRelayHaulMiles: 450.0,
		EnableVFALearning: false,
	}

	report, finalState, err := runner.Run(context.Background(), cfg, state, cfaPol, stream)
	if err != nil {
		t.Fatalf("RollingHorizonRunner.Run failed: %v", err)
	}

	t.Logf("7-Day Rolling Horizon Simulation Report:")
	t.Logf("  Total Days: %d, Total Epochs: %d", report.TotalDays, report.TotalEpochs)
	t.Logf("  Direct Tours: %d, Relays: %d", report.TotalDirectTours, report.TotalRelayExchanges)
	t.Logf("  Total Loaded Miles: %.1f, Total Empty Miles: %.1f (Empty Ratio: %.1f%%)",
		report.TotalLoadedMiles, report.TotalEmptyMiles, report.GlobalEmptyRatio*100.0)
	t.Logf("  Total Gross Revenue: $%.2f, Total Net Contribution: $%.2f",
		report.TotalGrossRevenue, report.TotalNetContribution)

	if len(report.DailySnapshots) != 7 {
		t.Errorf("expected 7 daily snapshots, got %d", len(report.DailySnapshots))
	}
	if report.TotalEpochs != 7 {
		t.Errorf("expected 7 epochs, got %d", report.TotalEpochs)
	}
	if finalState == nil {
		t.Errorf("expected non-nil final state")
	}
}
