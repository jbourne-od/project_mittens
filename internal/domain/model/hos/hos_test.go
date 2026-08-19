package hos_test

import (
	"sync"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
)

func TestHOS_BasicDrivingConsumption(t *testing.T) {
	specs := hos.USPolicySpecs()
	start := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC)
	clocks := hos.NewDriverClocks(specs, start)

	// Drive 4 hours (240 mins)
	next, err := clocks.ApplyDrive(240, specs)
	if err != nil {
		t.Fatalf("ApplyDrive failed: %v", err)
	}

	if next.RemainingHygieneMin() != 240 {
		t.Fatalf("RemainingHygieneMin = %d, expected 240", next.RemainingHygieneMin())
	}
	if next.RemainingDrivingMin() != 420 { // 660 - 240
		t.Fatalf("RemainingDrivingMin = %d, expected 420", next.RemainingDrivingMin())
	}
	if next.RemainingShiftMin() != 600 { // 840 - 240
		t.Fatalf("RemainingShiftMin = %d, expected 600", next.RemainingShiftMin())
	}
	if next.RemainingCycleMin() != 3960 { // 4200 - 240
		t.Fatalf("RemainingCycleMin = %d, expected 3960", next.RemainingCycleMin())
	}
}

func TestHOS_Auto30MinHygieneRestInsertion(t *testing.T) {
	specs := hos.USPolicySpecs()
	start := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC)
	clocks := hos.NewDriverClocks(specs, start)
	sim := hos.NewSimulator()

	// Request 9 hours driving (540 mins)
	// Expect: 8 hrs drive (480m) + 30m rest break + 1 hr drive (60m)
	events := []hos.Event{
		hos.DriveEvent(540, 450.0, "ROUTE_A"),
	}

	res, err := sim.Simulate(clocks, events, specs, true)
	if err != nil {
		t.Fatalf("Simulate failed: %v", err)
	}

	if res.TotalDriveMin != 540 {
		t.Fatalf("TotalDriveMin = %d, expected 540", res.TotalDriveMin)
	}
	if res.TotalRestMin != 30 {
		t.Fatalf("TotalRestMin = %d, expected 30", res.TotalRestMin)
	}
	if res.TotalDurationMin != 570 {
		t.Fatalf("TotalDurationMin = %d, expected 570", res.TotalDurationMin)
	}
	if len(res.Timeline) != 3 {
		t.Fatalf("Timeline length = %d, expected 3 (drive 480m -> rest 30m -> drive 60m)", len(res.Timeline))
	}
	if res.Timeline[1].Event.Type != hos.EventRest || res.Timeline[1].Event.DurationMin != 30 {
		t.Fatalf("Second timeline entry is not 30m rest break: %+v", res.Timeline[1])
	}
}

func TestHOS_11HourDailyLimitAnd10HourReset(t *testing.T) {
	specs := hos.USPolicySpecs()
	start := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC)
	clocks := hos.NewDriverClocks(specs, start)
	sim := hos.NewSimulator()

	// Request 15 hours driving (900 mins)
	// Day 1: 8h drive (480m) + 30m rest + 3h drive (180m) -> 11h daily max reached!
	// Daily Reset: 10h sleeper reset (600m)
	// Day 2: remaining 4h drive (240m)
	events := []hos.Event{
		hos.DriveEvent(900, 750.0, "LONG_HAUL"),
	}

	res, err := sim.Simulate(clocks, events, specs, true)
	if err != nil {
		t.Fatalf("Simulate failed: %v", err)
	}

	if res.TotalDriveMin != 900 {
		t.Fatalf("TotalDriveMin = %d, expected 900", res.TotalDriveMin)
	}
	// Rest should be 30m break + 600m daily reset = 630m
	if res.TotalRestMin != 630 {
		t.Fatalf("TotalRestMin = %d, expected 630 (30m + 600m)", res.TotalRestMin)
	}
	if res.TotalDurationMin != 1530 { // 900 + 630
		t.Fatalf("TotalDurationMin = %d, expected 1530", res.TotalDurationMin)
	}
}

func TestHOS_SleeperBerthSplitProvision(t *testing.T) {
	specs := hos.USPolicySpecs()
	start := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC)
	clocks := hos.NewDriverClocks(specs, start)

	// Drive 6 hours (360 mins)
	clocks, err := clocks.ApplyDrive(360, specs)
	if err != nil {
		t.Fatalf("drive failed: %v", err)
	}

	// First qualifying split rest: 7 hours in sleeper berth (420 mins)
	clocks, err = clocks.ApplyOffDuty(420, true, specs)
	if err != nil {
		t.Fatalf("sleeper rest failed: %v", err)
	}

	// Drive another 5 hours (300 mins)
	clocks, err = clocks.ApplyDrive(300, specs)
	if err != nil {
		t.Fatalf("second drive failed: %v", err)
	}

	// Second qualifying split break: 3 hours off-duty (180 mins)
	clocks, err = clocks.ApplyOffDuty(180, false, specs)
	if err != nil {
		t.Fatalf("split break failed: %v", err)
	}

	// Completing the valid 7/3 pair (total 10h) resets the 11-hour driving and 14-hour shift clocks
	if clocks.RemainingDrivingMin() != specs.MaxDrivingMin {
		t.Fatalf("RemainingDrivingMin = %d, expected full reset to %d", clocks.RemainingDrivingMin(), specs.MaxDrivingMin)
	}
}

func TestHOS_SleeperBerthSplit_IllegalShortBreaksRejected(t *testing.T) {
	// FMCSA 49 CFR § 395.1(g): Two 2-hour breaks (4 hours total) MUST NOT trigger a daily clock reset
	specs := hos.USPolicySpecs()
	start := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC)
	clocks := hos.NewDriverClocks(specs, start)

	// Drive 5 hours (300 mins)
	clocks, err := clocks.ApplyDrive(300, specs)
	if err != nil {
		t.Fatalf("drive failed: %v", err)
	}

	// First short break: 2 hours off-duty (120 mins)
	clocks, err = clocks.ApplyOffDuty(120, false, specs)
	if err != nil {
		t.Fatalf("first short break failed: %v", err)
	}

	// Drive another 4 hours (240 mins)
	clocks, err = clocks.ApplyDrive(240, specs)
	if err != nil {
		t.Fatalf("second drive failed: %v", err)
	}

	// Second short break: 2 hours off-duty (120 mins)
	clocks, err = clocks.ApplyOffDuty(120, false, specs)
	if err != nil {
		t.Fatalf("second short break failed: %v", err)
	}

	// Two 2-hour breaks do NOT total 10h and lack a >=7h sleeper period -> NO full reset!
	if clocks.RemainingDrivingMin() >= specs.MaxDrivingMin {
		t.Fatalf("Illegal reset! RemainingDrivingMin = %d was fully reset after only two 2-hour breaks", clocks.RemainingDrivingMin())
	}
}

func TestHOS_TripFeasibilityEvaluation(t *testing.T) {
	specs := hos.USPolicySpecs()
	start := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC)
	clocks := hos.NewDriverClocks(specs, start)
	sim := hos.NewSimulator()

	// Trip: 50 miles deadhead (1 hour), 60 min loading, 250 miles linehaul (5 hours), 60 min unloading
	// Total drive = 6 hours (360m), total duty = 8 hours (480m), no statutory rests needed!
	pickupLatest := start.Add(3 * time.Hour)    // 11:00 AM (driver arrives at 9:00 AM)
	deliveryLatest := start.Add(10 * time.Hour) // 6:00 PM (driver arrives at 3:00 PM)

	res, err := sim.EvaluateTripFeasibility(
		clocks,
		50.0, 250.0,
		60, 60,
		50.0,
		start, pickupLatest,
		start, deliveryLatest,
		specs,
	)
	if err != nil {
		t.Fatalf("EvaluateTripFeasibility failed: %v", err)
	}

	if !res.IsFeasible {
		t.Fatalf("expected trip to be feasible, got infeasible: %s", res.InfeasibilityReason)
	}
	if res.DeadheadDriveMin != 60 || res.LoadedDriveMin != 300 {
		t.Fatalf("drive durations: deadhead=%d, loaded=%d", res.DeadheadDriveMin, res.LoadedDriveMin)
	}
	if res.InsertedRestMin != 0 {
		t.Fatalf("InsertedRestMin = %d, expected 0", res.InsertedRestMin)
	}

	// Test Infeasible pickup window (driver arrives at 9:00 AM, but latest pickup was 8:30 AM)
	infeasiblePickup := start.Add(30 * time.Minute)
	resInf, err := sim.EvaluateTripFeasibility(
		clocks,
		50.0, 250.0,
		60, 60,
		50.0,
		start, infeasiblePickup,
		start, deliveryLatest,
		specs,
	)
	if err != nil {
		t.Fatalf("EvaluateTripFeasibility failed: %v", err)
	}
	if resInf.IsFeasible {
		t.Fatalf("expected trip to be infeasible due to tight pickup window")
	}
}

func TestHOS_TripFeasibilityEvaluation_EarlyArrivalDwell(t *testing.T) {
	specs := hos.USPolicySpecs()
	start := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC)
	clocks := hos.NewDriverClocks(specs, start)
	sim := hos.NewSimulator()

	// Deadhead takes 1 hour (driver arrives at 9:00 AM)
	// pickupEarliest is 10:00 AM -> must model 1 hour (60 min) early arrival dwell!
	pickupEarliest := start.Add(2 * time.Hour)   // 10:00 AM
	pickupLatest := start.Add(4 * time.Hour)     // 12:00 PM
	deliveryEarliest := start.Add(6 * time.Hour) // 2:00 PM
	deliveryLatest := start.Add(12 * time.Hour)  // 8:00 PM

	res, err := sim.EvaluateTripFeasibility(
		clocks,
		50.0, 250.0,
		60, 60,
		50.0,
		pickupEarliest, pickupLatest,
		deliveryEarliest, deliveryLatest,
		specs,
	)
	if err != nil {
		t.Fatalf("EvaluateTripFeasibility failed: %v", err)
	}

	if !res.IsFeasible {
		t.Fatalf("expected trip to be feasible: %s", res.InfeasibilityReason)
	}
	if res.InsertedDwellMin != 60 {
		t.Fatalf("InsertedDwellMin = %d, expected 60 min early arrival dwell", res.InsertedDwellMin)
	}
	// Loading must start at 10:00 AM (after dwell) and end at 11:00 AM (60 min loading)
	expectedLoadingEnd := start.Add(3 * time.Hour)
	if !res.LoadingEndTime.Equal(expectedLoadingEnd) {
		t.Fatalf("LoadingEndTime = %v, expected %v", res.LoadingEndTime, expectedLoadingEnd)
	}
}

func TestHOS_ConcurrentSimulatorEvaluation(t *testing.T) {
	// Verify zero race conditions when evaluating multiple driver lookaheads in parallel goroutines (Inviolate 6)
	specs := hos.USPolicySpecs()
	start := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC)
	sim := hos.NewSimulator()

	const numGoroutines = 32
	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			driverClocks := hos.NewDriverClocks(specs, start)
			events := []hos.Event{
				hos.DriveEvent(300+id*10, float64(250+id*10), "ORIGIN"),
				hos.LoadingEvent(60, "FACILITY"),
				hos.DriveEvent(200, 160.0, "DEST"),
			}
			res, err := sim.Simulate(driverClocks, events, specs, true)
			if err != nil {
				t.Errorf("goroutine %d simulation failed: %v", id, err)
			}
			if res.TotalDurationMin <= 0 {
				t.Errorf("goroutine %d total duration is non-positive", id)
			}
		}(g)
	}

	wg.Wait()
}
