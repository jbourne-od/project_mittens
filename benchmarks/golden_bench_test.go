package benchmarks_test

import (
	"context"
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
