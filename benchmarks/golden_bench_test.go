package benchmarks_test

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/adapter/legacy"
	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

func getGoldenCoreAIRoot(b *testing.B) string {
	b.Helper()
	candidates := []string{
		os.Getenv("COREAI_ROOT"),
		os.Getenv("COREAI_DATA_DIR"),
		"/Users/jacob/Development/od/coreai",
		"../../../../coreai",
	}
	for _, dir := range candidates {
		if dir != "" {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return dir
			}
		}
	}
	b.Skip("coreai golden fixtures directory not found; skipping golden benchmark")
	return ""
}

// BenchmarkGolden_07TestDispatch benchmarks pure Go optimizer matching on the 07_test_dispatch golden fixture.
func BenchmarkGolden_07TestDispatch(b *testing.B) {
	coreaiRoot := getGoldenCoreAIRoot(b)
	scenarioDir := filepath.Join(coreaiRoot, "engine/smart_tl/worker/tests/07_test_dispatch")
	inputDir := filepath.Join(scenarioDir, "input")

	locFile := filepath.Join(inputDir, "locations.txt")
	driverFile := filepath.Join(inputDir, "drivers.txt")
	loadFile := filepath.Join(inputDir, "loads.txt")

	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 0)
	if err != nil {
		b.Skipf("skipping golden benchmark: LoadCarrierScenario failed: %v", err)
		return
	}

	baseEpoch := time.Date(2022, 5, 11, 2, 0, 0, 0, time.UTC).Unix()
	for i := range drivers {
		drivers[i].CurrentLocation = drivers[i].HomeLocation
		drivers[i].AvailableEpoch = baseEpoch
		drivers[i].Clocks = hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(baseEpoch, 0))
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(baseEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, _ := model.NewState(res, info, belief)

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		action, _, err := cfaPol.Evaluate(ctx, state)
		if err != nil {
			b.Fatalf("Evaluate failed: %v", err)
		}
		if action.MatchCount() != 2 {
			b.Fatalf("expected 2 matches, got %d", action.MatchCount())
		}
	}
}

// BenchmarkGolden_16OptimalTours benchmarks pure Go optimizer matching on the 16_test_dispatch_optimal_tours golden fixture (49 drivers, 362 loads).
func BenchmarkGolden_16OptimalTours(b *testing.B) {
	coreaiRoot := getGoldenCoreAIRoot(b)
	scenarioDir := filepath.Join(coreaiRoot, "engine/smart_tl/worker/tests/16_test_dispatch_optimal_tours")
	inputDir := filepath.Join(scenarioDir, "input")

	locFile := filepath.Join(inputDir, "locations_out.txt")
	driverFile := filepath.Join(inputDir, "drivers.txt")
	loadFile := filepath.Join(inputDir, "loads.txt")

	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 0)
	if err != nil {
		b.Skipf("skipping golden benchmark: LoadCarrierScenario failed: %v", err)
		return
	}

	baseEpoch := time.Date(2022, 8, 16, 12, 0, 0, 0, time.UTC).Unix()
	for i := range drivers {
		if drivers[i].AvailableEpoch == 0 {
			drivers[i].AvailableEpoch = baseEpoch
		}
		if drivers[i].Clocks == nil {
			drivers[i].Clocks = hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(drivers[i].AvailableEpoch, 0))
		}
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(baseEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, _ := model.NewState(res, info, belief)

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		action, _, err := cfaPol.Evaluate(ctx, state)
		if err != nil {
			b.Fatalf("Evaluate failed: %v", err)
		}
		if action.MatchCount() == 0 {
			b.Fatalf("expected matches, got 0")
		}
	}
}

// BenchmarkGolden_13Relays benchmarks Go optimizer matching on the 13_test_relays fixture.
func BenchmarkGolden_13Relays(b *testing.B) {
	coreaiRoot := getGoldenCoreAIRoot(b)
	scenarioDir := filepath.Join(coreaiRoot, "engine/smart_tl/worker/tests/13_test_relays")
	inputDir := filepath.Join(scenarioDir, "input")

	locFile := filepath.Join(inputDir, "locations_WR.txt")
	driverFile := filepath.Join(inputDir, "DRIVERS_NO_OTR_DED_Sampled.txt")
	loadFile := filepath.Join(inputDir, "RC_AND_WR_LOADS_Sampled.txt")

	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 250)
	if err != nil {
		b.Skipf("skipping golden benchmark: LoadCarrierScenario failed: %v", err)
		return
	}

	baseEpoch := int64(1660176000) // Aug 2022
	for i := range drivers {
		drivers[i].CurrentLocation = drivers[i].HomeLocation
		drivers[i].AvailableEpoch = baseEpoch
		drivers[i].Clocks = hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(baseEpoch, 0))
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(baseEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, _ := model.NewState(res, info, belief)

	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 1500.0
	feasCfg.MaxEarlyDwellHours = 120.0
	feasCfg.MaxLateDeliveryHours = 24.0

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		feasCfg,
		nil,
	)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		action, _, err := cfaPol.Evaluate(ctx, state)
		if err != nil {
			b.Fatalf("Evaluate failed: %v", err)
		}
		if action.MatchCount() == 0 {
			b.Fatalf("expected matches, got 0")
		}
	}
}

// BenchmarkGolden_05HomeTime benchmarks Go optimizer matching on the 05_test_home_time fixture.
func BenchmarkGolden_05HomeTime(b *testing.B) {
	coreaiRoot := getGoldenCoreAIRoot(b)
	scenarioDir := filepath.Join(coreaiRoot, "engine/smart_tl/worker/tests/05_test_home_time")
	inputDir := filepath.Join(scenarioDir, "input")

	locFile := filepath.Join(inputDir, "locations_out.txt")
	driverFile := filepath.Join(inputDir, "drivers_1_custom.txt")
	loadFile := filepath.Join(inputDir, "loads.txt")

	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 100)
	if err != nil {
		b.Skipf("skipping golden benchmark: LoadCarrierScenario failed: %v", err)
		return
	}

	minPickupEpoch := int64(math.MaxInt64)
	for _, l := range loads {
		if l.PickupEarliestEpoch > 0 && l.PickupEarliestEpoch < minPickupEpoch {
			minPickupEpoch = l.PickupEarliestEpoch
		}
	}
	baseEpoch := minPickupEpoch - 3600

	for i := range drivers {
		drivers[i].CurrentLocation = drivers[i].HomeLocation
		drivers[i].AvailableEpoch = baseEpoch
		drivers[i].Clocks = hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(baseEpoch, 0))
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(baseEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, _ := model.NewState(res, info, belief)

	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 1500.0
	feasCfg.MaxEarlyDwellHours = 120.0
	feasCfg.MaxLateDeliveryHours = 24.0

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		feasCfg,
		nil,
	)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		action, _, err := cfaPol.Evaluate(ctx, state)
		if err != nil {
			b.Fatalf("Evaluate failed: %v", err)
		}
		if action.MatchCount() == 0 {
			b.Fatalf("expected matches, got 0")
		}
	}
}

// BenchmarkGolden_14PreAssignments benchmarks Go optimizer matching on the 14_test_pre_assignments fixture.
func BenchmarkGolden_14PreAssignments(b *testing.B) {
	coreaiRoot := getGoldenCoreAIRoot(b)
	scenarioDir := filepath.Join(coreaiRoot, "engine/smart_tl/worker/tests/14_test_pre_assignments")
	inputDir := filepath.Join(scenarioDir, "input")

	locFile := filepath.Join(inputDir, "locations.txt")
	driverFile := filepath.Join(inputDir, "drivers.txt")
	loadFile := filepath.Join(inputDir, "loads.txt")

	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 150)
	if err != nil {
		b.Skipf("skipping golden benchmark: LoadCarrierScenario failed: %v", err)
		return
	}

	baseEpoch := int64(1696860000) // Oct 2023
	for i := range drivers {
		drivers[i].CurrentLocation = drivers[i].HomeLocation
		drivers[i].AvailableEpoch = baseEpoch
		drivers[i].Clocks = hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(baseEpoch, 0))
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(baseEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, _ := model.NewState(res, info, belief)

	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 1500.0
	feasCfg.MaxEarlyDwellHours = 120.0
	feasCfg.MaxLateDeliveryHours = 72.0

	costCfg := model.DefaultCostConfig()
	costCfg.EarlyArrivalPerHour = 0.0

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		costCfg,
		feasCfg,
		nil,
	)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		action, _, err := cfaPol.Evaluate(ctx, state)
		if err != nil {
			b.Fatalf("Evaluate failed: %v", err)
		}
		if action.MatchCount() == 0 {
			b.Fatalf("expected matches, got 0")
		}
	}
}

// BenchmarkGolden_17GeoConstraints benchmarks Go optimizer matching on the 17_test_driver_geo_constraints fixture.
func BenchmarkGolden_17GeoConstraints(b *testing.B) {
	coreaiRoot := getGoldenCoreAIRoot(b)
	scenarioDir := filepath.Join(coreaiRoot, "engine/smart_tl/worker/tests/17_test_driver_geo_constraints")
	inputDir := filepath.Join(scenarioDir, "input")

	locFile := filepath.Join(inputDir, "locations.txt")
	driverFile := filepath.Join(inputDir, "drivers.txt")
	loadFile := filepath.Join(inputDir, "loads.txt")

	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 200)
	if err != nil {
		b.Skipf("skipping golden benchmark: LoadCarrierScenario failed: %v", err)
		return
	}

	baseEpoch := int64(1683140000) // May 2023
	for i := range drivers {
		drivers[i].CurrentLocation = drivers[i].HomeLocation
		drivers[i].AvailableEpoch = baseEpoch
		drivers[i].Clocks = hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(baseEpoch, 0))
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(baseEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, _ := model.NewState(res, info, belief)

	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 1500.0
	feasCfg.MaxEarlyDwellHours = 120.0
	feasCfg.MaxLateDeliveryHours = 72.0

	costCfg := model.DefaultCostConfig()
	costCfg.EarlyArrivalPerHour = 0.0

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		costCfg,
		feasCfg,
		nil,
	)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		action, _, err := cfaPol.Evaluate(ctx, state)
		if err != nil {
			b.Fatalf("Evaluate failed: %v", err)
		}
		if action.MatchCount() == 0 {
			b.Fatalf("expected matches, got 0")
		}
	}
}

// BenchmarkGolden_15OnTimeParameters benchmarks Go optimizer matching on the 15_test_on_time_parameters fixture.
func BenchmarkGolden_15OnTimeParameters(b *testing.B) {
	coreaiRoot := getGoldenCoreAIRoot(b)
	scenarioDir := filepath.Join(coreaiRoot, "engine/smart_tl/worker/tests/15_test_on_time_parameters")
	inputDir := filepath.Join(scenarioDir, "input")

	locFile := filepath.Join(inputDir, "locations_out.txt")
	driverFile := filepath.Join(inputDir, "drivers_1_custom.txt")
	loadFile := filepath.Join(inputDir, "loads-new.txt")

	drivers, loads, _, err := legacy.LoadCarrierScenario(locFile, driverFile, loadFile, 150)
	if err != nil {
		b.Skipf("skipping golden benchmark: LoadCarrierScenario failed: %v", err)
		return
	}

	baseEpoch := int64(1679070000) // Mar 2023
	for i := range drivers {
		drivers[i].CurrentLocation = drivers[i].HomeLocation
		drivers[i].AvailableEpoch = baseEpoch
		drivers[i].Clocks = hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(baseEpoch, 0))
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(baseEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, _ := model.NewState(res, info, belief)

	feasCfg := model.DefaultFeasibilityConfig()
	feasCfg.MaxDeadheadMiles = 1500.0
	feasCfg.MaxEarlyDwellHours = 120.0
	feasCfg.MaxLateDeliveryHours = 72.0

	costCfg := model.DefaultCostConfig()
	costCfg.EarlyArrivalPerHour = 0.0

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		costCfg,
		feasCfg,
		nil,
	)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		action, _, err := cfaPol.Evaluate(ctx, state)
		if err != nil {
			b.Fatalf("Evaluate failed: %v", err)
		}
		if action.MatchCount() == 0 {
			b.Fatalf("expected matches, got 0")
		}
	}
}
