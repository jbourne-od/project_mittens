package benchmarks_test

import (
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
)

// BenchmarkHOS_EvaluateTripFeasibility evaluates single-trip forward timeline projection throughput.
func BenchmarkHOS_EvaluateTripFeasibility(b *testing.B) {
	sim := hos.NewSimulator()
	specs := hos.USPolicySpecs()
	now := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC)
	clocks := hos.NewDriverClocks(specs, now)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		res, err := sim.EvaluateTripFeasibility(
			clocks,
			45.0,  // 45 miles empty deadhead
			480.0, // 480 miles linehaul
			60,    // 60 min loading
			60,    // 60 min unloading
			50.0,  // 50 MPH
			now.Add(2*time.Hour),
			now.Add(12*time.Hour),
			now.Add(10*time.Hour),
			now.Add(36*time.Hour),
			specs,
		)
		if err != nil || !res.IsFeasible {
			b.Fatalf("expected feasible trip: err=%v, res=%+v", err, res)
		}
	}
}

// BenchmarkHOS_MultiLegComplexSimulate evaluates complex multi-leg HOS simulations with automatic statutory rest insertion.
func BenchmarkHOS_MultiLegComplexSimulate(b *testing.B) {
	sim := hos.NewSimulator()
	specs := hos.USPolicySpecs()
	now := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC)
	clocks := hos.NewDriverClocks(specs, now)

	// Long-haul multi-day sequence (1,200 miles with mandatory 10h resets and 30m hygiene breaks)
	events := []hos.Event{
		hos.DriveEvent(300, 250.0, "LEG_1_DEADHEAD"),
		hos.LoadingEvent(90, "FAC_ORIGIN"),
		hos.DriveEvent(400, 330.0, "LEG_2_LINEHAUL_A"),
		hos.DriveEvent(400, 330.0, "LEG_3_LINEHAUL_B"),
		hos.DriveEvent(350, 290.0, "LEG_4_LINEHAUL_C"),
		hos.UnloadingEvent(90, "FAC_DEST"),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		res, err := sim.Simulate(clocks, events, specs, true)
		if err != nil {
			b.Fatalf("Simulate failed: %v", err)
		}
		if res.TotalRestMin == 0 {
			b.Fatalf("expected rest periods inserted in long haul")
		}
	}
}
