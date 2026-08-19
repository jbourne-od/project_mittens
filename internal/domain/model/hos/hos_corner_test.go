package hos_test

import (
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
)

func TestHOSCornerCases_SingleMinuteDrivingRemaining(t *testing.T) {
	specs := hos.USPolicySpecs()
	startTime := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC)
	sim := hos.NewSimulator()

	// Initialize driver with 659 minutes already driven (480m drive + 30m hygiene break + 179m drive)
	clocks := hos.NewDriverClocks(specs, startTime)
	clocks, err := clocks.ApplyDrive(480, specs)
	if err != nil {
		t.Fatalf("first drive failed: %v", err)
	}
	clocks, err = clocks.ApplyOffDuty(30, false, specs)
	if err != nil {
		t.Fatalf("hygiene break failed: %v", err)
	}
	clocks, err = clocks.ApplyDrive(179, specs)
	if err != nil {
		t.Fatalf("second drive failed: %v", err)
	}

	if clocks.RemainingDrivingMin() > 1 {
		t.Fatalf("expected <= 1 minute drive remaining, got %d mins", clocks.RemainingDrivingMin())
	}

	// 1. Without autoInsertRests, attempting 2 minutes of driving must fail closed
	evs := []hos.Event{
		hos.DriveEvent(2, 1.5, "SHORT_HAUL"),
	}
	if _, err := sim.Simulate(clocks, evs, specs, false); err == nil {
		t.Errorf("expected ErrHOSViolation when exceeding 1 minute drive limit without auto-rest")
	}

	// 2. With autoInsertRests, simulator must insert a 10-hour daily reset and successfully complete
	res, err := sim.Simulate(clocks, evs, specs, true)
	if err != nil {
		t.Fatalf("Simulate with autoInsertRests failed: %v", err)
	}

	if res.TotalRestMin < 600 {
		t.Errorf("expected at least 600 min (10h) rest inserted, got %d", res.TotalRestMin)
	}
	if res.FinalClocks.RemainingDrivingMin() <= 600 {
		t.Errorf("expected full 11h drive clock reset, got %d", res.FinalClocks.RemainingDrivingMin())
	}
}

func TestHOSCornerCases_SleeperBerthSplitPairingPermutations(t *testing.T) {
	specs := hos.USPolicySpecs()
	startTime := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC)

	// Valid 7/3 Split: 7h Sleeper + 3h Off-Duty
	t.Run("Valid73Split", func(t *testing.T) {
		clocks := hos.NewDriverClocks(specs, startTime)
		// Drive 6 hours
		clocks, _ = clocks.ApplyDrive(360, specs)
		// 7h in Sleeper Berth (qualifying anchor)
		clocks, err := clocks.ApplyOffDuty(420, true, specs)
		if err != nil {
			t.Fatalf("7h sleeper failed: %v", err)
		}
		// Drive 4 hours
		clocks, _ = clocks.ApplyDrive(240, specs)
		// 3h Off-Duty (qualifying pairing)
		clocks, err = clocks.ApplyOffDuty(180, false, specs)
		if err != nil {
			t.Fatalf("3h off-duty failed: %v", err)
		}

		// Shift clock must be recalculated excluding the paired qualifying periods
		if clocks.RemainingShiftMin() <= 0 {
			t.Errorf("expected positive shift hours after valid 7/3 split, got %d mins", clocks.RemainingShiftMin())
		}
	})

	// Invalid Split: 6h Sleeper + 4h Off-Duty (Sleeper period < 7h statutory minimum)
	t.Run("Invalid64SplitRejected", func(t *testing.T) {
		clocks := hos.NewDriverClocks(specs, startTime)
		// Drive 6 hours
		clocks, err := clocks.ApplyDrive(360, specs)
		if err != nil {
			t.Fatalf("first drive failed: %v", err)
		}
		// 6h in Sleeper Berth (< 7h requirement -> does not qualify as split anchor or daily reset)
		clocks, err = clocks.ApplyOffDuty(360, true, specs)
		if err != nil {
			t.Fatalf("6h sleeper failed: %v", err)
		}
		// Attempting to drive 6 hours (total 12h driving without 10h reset) must be rejected with ErrHOSViolation
		_, err = clocks.ApplyDrive(360, specs)
		if err == nil {
			t.Errorf("expected ErrHOSViolation when exceeding 11h driving limit across invalid 6h split")
		}
	})
}

func TestHOSCornerCases_70HourRollingCycleDepletion(t *testing.T) {
	specs := hos.USPolicySpecs()
	startTime := time.Date(2026, 8, 1, 6, 0, 0, 0, time.UTC)
	sim := hos.NewSimulator()

	clocks := hos.NewDriverClocks(specs, startTime)

	// Simulate 7 consecutive days of maximum legal driving (10h drive + 10h rest = 20h cycle)
	var events []hos.Event
	for day := 0; day < 7; day++ {
		events = append(events,
			hos.DriveEvent(600, 500.0, "DAY_DRIVE"),
			hos.RestEvent(600, true, "NIGHT_REST"),
		)
	}

	res, err := sim.Simulate(clocks, events, specs, true)
	if err != nil {
		t.Fatalf("7-day simulation failed: %v", err)
	}

	// 7 days * 10h driving = 70 hours total duty
	if res.TotalDriveMin != 4200 {
		t.Errorf("expected 4200 min (70h) total drive, got %d", res.TotalDriveMin)
	}
	if res.TotalRestMin < 4200 {
		t.Errorf("expected at least 4200 min total rest, got %d", res.TotalRestMin)
	}
}
