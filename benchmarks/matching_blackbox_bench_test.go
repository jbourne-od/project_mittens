package benchmarks_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

// generateBenchmarkState creates a synthetic carrier state with numDrivers and numLoads.
func generateBenchmarkState(numDrivers, numLoads int, seed int64) *model.State[model.Monopolistic] {
	rng := rand.New(rand.NewSource(seed))
	startEpoch := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	cities := []model.Location{
		{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298},
		{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880},
		{NodeID: "DAL", Lat: 32.7767, Lon: -96.7970},
		{NodeID: "LAX", Lat: 34.0522, Lon: -118.2437},
		{NodeID: "NYC", Lat: 40.7128, Lon: -74.0060},
		{NodeID: "DEN", Lat: 39.7392, Lon: -104.9903},
		{NodeID: "SEA", Lat: 47.6062, Lon: -122.3321},
		{NodeID: "MIA", Lat: 25.7617, Lon: -80.1918},
	}

	drivers := make([]model.Driver, numDrivers)
	for i := 0; i < numDrivers; i++ {
		loc := cities[rng.Intn(len(cities))]
		drivers[i] = model.Driver{
			ID:                  fmt.Sprintf("DRV_%04d", i),
			CurrentLocation:     loc,
			HomeLocation:        loc,
			AvailableEpoch:      startEpoch,
			DriveHoursRemaining: 11.0,
			DutyHoursRemaining:  14.0,
			Equipment:           model.Equipment{Type: model.EquipDryVan},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		}
	}

	loads := make([]model.Load, numLoads)
	for j := 0; j < numLoads; j++ {
		orig := cities[rng.Intn(len(cities))]
		dest := cities[rng.Intn(len(cities))]
		for dest.NodeID == orig.NodeID {
			dest = cities[rng.Intn(len(cities))]
		}

		rev := 800.0 + rng.Float64()*2500.0
		loads[j] = model.Load{
			ID:                    fmt.Sprintf("LOAD_%05d", j),
			Origin:                orig,
			Destination:           dest,
			PickupEarliestEpoch:   startEpoch,
			PickupLatestEpoch:     startEpoch + 43200,
			DeliveryEarliestEpoch: startEpoch + 21600,
			DeliveryLatestEpoch:   startEpoch + 172800,
			Revenue:               rev,
			RequiredEquipment:     model.EquipDryVan,
		}
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, _ := model.NewState(res, info, belief)
	return state
}

// BenchmarkMatching_CFAPolicy benchmarks end-to-end parametric CFA matching.
func BenchmarkMatching_CFAPolicy(b *testing.B) {
	fleetConfigs := []struct {
		drivers int
		loads   int
	}{
		{drivers: 25, loads: 100},
		{drivers: 50, loads: 250},
		{drivers: 100, loads: 500},
		{drivers: 250, loads: 1000},
	}

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	for _, cfg := range fleetConfigs {
		b.Run(fmt.Sprintf("%dDrivers_%dLoads", cfg.drivers, cfg.loads), func(b *testing.B) {
			state := generateBenchmarkState(cfg.drivers, cfg.loads, 42)
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				action, prov, err := cfaPol.Evaluate(ctx, state)
				if err != nil {
					b.Fatalf("Evaluate failed: %v", err)
				}
				if action.MatchCount() == 0 {
					b.Fatalf("expected matches, got 0")
				}
				_ = prov
			}
		})
	}
}

// BenchmarkMatching_PiecewiseVFAPolicy benchmarks Piecewise Linear Concave VFA matching.
func BenchmarkMatching_PiecewiseVFAPolicy(b *testing.B) {
	slopes := map[string]policy.RegionSlopes{
		"CHI": {RegionID: "CHI", Slopes: []float64{600.0, 450.0, 300.0, 150.0, 50.0}},
		"ATL": {RegionID: "ATL", Slopes: []float64{700.0, 500.0, 350.0, 200.0, 80.0}},
		"DAL": {RegionID: "DAL", Slopes: []float64{550.0, 400.0, 280.0, 120.0, 40.0}},
		"LAX": {RegionID: "LAX", Slopes: []float64{800.0, 600.0, 400.0, 250.0, 100.0}},
		"NYC": {RegionID: "NYC", Slopes: []float64{750.0, 550.0, 380.0, 220.0, 90.0}},
	}
	table := policy.NewPiecewiseLinearVFATable(slopes)
	rm := model.NewRegionManager(1.0, nil)

	pvfaPol := policy.NewPiecewiseVFAPolicy[model.Monopolistic](
		table,
		nil,
		0.95,
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		rm,
	)

	state := generateBenchmarkState(50, 250, 99)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		action, prov, err := pvfaPol.Evaluate(ctx, state)
		if err != nil {
			b.Fatalf("Evaluate failed: %v", err)
		}
		if action.MatchCount() == 0 {
			b.Fatalf("expected matches, got 0")
		}
		_ = prov
	}
}
