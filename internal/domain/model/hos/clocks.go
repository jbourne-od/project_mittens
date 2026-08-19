package hos

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrHOSViolation is returned when a requested duty duration exceeds statutory HOS limits.
	ErrHOSViolation = errors.New("domain/model/hos: statutory HOS limit exceeded")
)

// DutyStatus represents the standard FMCSA driver duty status.
type DutyStatus int

const (
	// StatusOffDuty indicates the driver is completely relieved from work and all responsibility.
	StatusOffDuty DutyStatus = iota
	// StatusSleeperBerth indicates the driver is resting in an approved commercial sleeper berth compartment.
	StatusSleeperBerth
	// StatusDriving indicates the driver is operating the commercial motor vehicle controls.
	StatusDriving
	// StatusOnDutyNotDriving indicates the driver is performing non-driving work (loading, unloading, inspection, fueling).
	StatusOnDutyNotDriving
)

func (s DutyStatus) String() string {
	switch s {
	case StatusOffDuty:
		return "OFF_DUTY"
	case StatusSleeperBerth:
		return "SLEEPER_BERTH"
	case StatusDriving:
		return "DRIVING"
	case StatusOnDutyNotDriving:
		return "ON_DUTY_NOT_DRIVING"
	default:
		return "UNKNOWN"
	}
}

// DriverClocks models the complete, authoritative regulatory clocks tracking a driver's duty limits.
//
// In accordance with Inviolate 5 (State Immutability) and Inviolate 6 (Lock-Free Hot Paths),
// DriverClocks instances are immutable. All transition operations allocate and return new DriverClocks pointers.
type DriverClocks struct {
	now                     time.Time
	remainingHygieneMin     int // Continuous drive time remaining before mandatory 30-min break
	remainingDrivingMin     int // Daily driving time remaining in current shift
	remainingShiftMin       int // Elapsed on-duty shift window time remaining
	remainingCycleMin       int // Cumulative duty time remaining in rolling multi-day window
	currentDutyStatus       DutyStatus
	consecutiveOffDutyMin   int   // Unbroken minutes spent in OffDuty or SleeperBerth
	consecutiveDrivingMin   int   // Cumulative driving minutes since last >=30-min break
	dutyMinutesByDay        []int // Rolling daily duty history (minutes per day)
	currentDayIndex         int   // Index of current day in rolling cycle
	sleeperSplitActive      bool  // True if driver completed a valid first qualifying sleeper split break
	lastSleeperSplitMin     int   // Duration of the first qualifying split break
	hasAdverseConditions    bool
	drivingCeilingExtension int
}

// NewDriverClocks initializes a fresh, full set of regulatory clocks for a driver ready for dispatch.
func NewDriverClocks(specs PolicySpecs, startTime time.Time) *DriverClocks {
	cycleDays := specs.CycleLengthDays
	if cycleDays <= 0 {
		cycleDays = 8
	}

	return &DriverClocks{
		now:                   startTime,
		remainingHygieneMin:   specs.MaxHygieneMin,
		remainingDrivingMin:   specs.MaxDrivingMin,
		remainingShiftMin:     specs.MaxShiftMin,
		remainingCycleMin:     specs.MaxCycleMin,
		currentDutyStatus:     StatusOffDuty,
		consecutiveOffDutyMin: specs.DailyResetMin, // Driver starts fully rested
		consecutiveDrivingMin: 0,
		dutyMinutesByDay:      make([]int, cycleDays),
		currentDayIndex:       0,
	}
}

// Now returns the clock's current reference timestamp.
func (c *DriverClocks) Now() time.Time {
	return c.now
}

// RemainingHygieneMin returns the remaining continuous driving minutes before a 30-min rest is required.
func (c *DriverClocks) RemainingHygieneMin() int {
	return c.remainingHygieneMin
}

// RemainingDrivingMin returns the remaining daily driving minutes in the active shift.
func (c *DriverClocks) RemainingDrivingMin() int {
	return c.remainingDrivingMin
}

// RemainingShiftMin returns the remaining minutes in the 14-hour on-duty shift window.
func (c *DriverClocks) RemainingShiftMin() int {
	return c.remainingShiftMin
}

// RemainingCycleMin returns the remaining minutes in the 70-hour rolling cycle.
func (c *DriverClocks) RemainingCycleMin() int {
	return c.remainingCycleMin
}

// DutyStatus returns the current duty status of the driver.
func (c *DriverClocks) DutyStatus() DutyStatus {
	return c.currentDutyStatus
}

// MaxImmediateDriveMin returns the maximum uninterrupted driving duration (in minutes)
// the driver can execute immediately before hitting ANY statutory barrier (hygiene, driving, shift, cycle).
func (c *DriverClocks) MaxImmediateDriveMin() int {
	m := c.remainingHygieneMin
	if c.remainingDrivingMin < m {
		m = c.remainingDrivingMin
	}
	if c.remainingShiftMin < m {
		m = c.remainingShiftMin
	}
	if c.remainingCycleMin < m {
		m = c.remainingCycleMin
	}
	if m < 0 {
		return 0
	}
	return m
}

// Clone creates an exact deep copy of the DriverClocks structure (Inviolate 5).
func (c *DriverClocks) Clone() *DriverClocks {
	copiedDays := make([]int, len(c.dutyMinutesByDay))
	copy(copiedDays, c.dutyMinutesByDay)

	return &DriverClocks{
		now:                     c.now,
		remainingHygieneMin:     c.remainingHygieneMin,
		remainingDrivingMin:     c.remainingDrivingMin,
		remainingShiftMin:       c.remainingShiftMin,
		remainingCycleMin:       c.remainingCycleMin,
		currentDutyStatus:       c.currentDutyStatus,
		consecutiveOffDutyMin:   c.consecutiveOffDutyMin,
		consecutiveDrivingMin:   c.consecutiveDrivingMin,
		dutyMinutesByDay:        copiedDays,
		currentDayIndex:         c.currentDayIndex,
		sleeperSplitActive:      c.sleeperSplitActive,
		lastSleeperSplitMin:     c.lastSleeperSplitMin,
		hasAdverseConditions:    c.hasAdverseConditions,
		drivingCeilingExtension: c.drivingCeilingExtension,
	}
}

// ApplyDrive advances the clocks by durationMin of continuous driving.
//
// In accordance with Inviolate 5, returns a newly allocated *DriverClocks instance.
// Returns an error if durationMin exceeds any active driving or shift limit.
func (c *DriverClocks) ApplyDrive(durationMin int, specs PolicySpecs) (*DriverClocks, error) {
	if durationMin <= 0 {
		return c.Clone(), nil
	}

	maxDrive := c.MaxImmediateDriveMin()
	if durationMin > maxDrive {
		return nil, fmt.Errorf("%w: requested drive %d mins exceeds available immediate drive limit %d mins", ErrHOSViolation, durationMin, maxDrive)
	}

	next := c.Clone()
	next.now = next.now.Add(time.Duration(durationMin) * time.Minute)
	next.currentDutyStatus = StatusDriving
	next.consecutiveOffDutyMin = 0
	next.consecutiveDrivingMin += durationMin

	next.remainingHygieneMin -= durationMin
	next.remainingDrivingMin -= durationMin
	next.remainingShiftMin -= durationMin
	next.remainingCycleMin -= durationMin

	next.dutyMinutesByDay[next.currentDayIndex] += durationMin

	return next, nil
}

// ApplyOnDutyNotDriving advances the clocks by durationMin of on-duty work (e.g. loading, unloading, inspection).
//
// On-duty non-driving time consumes shift and rolling cycle clocks, but does NOT consume driving time.
func (c *DriverClocks) ApplyOnDutyNotDriving(durationMin int, specs PolicySpecs) (*DriverClocks, error) {
	if durationMin <= 0 {
		return c.Clone(), nil
	}

	maxDuty := c.remainingShiftMin
	if c.remainingCycleMin < maxDuty {
		maxDuty = c.remainingCycleMin
	}
	if durationMin > maxDuty {
		return nil, fmt.Errorf("%w: requested duty %d mins exceeds available duty limit %d mins", ErrHOSViolation, durationMin, maxDuty)
	}

	next := c.Clone()
	next.now = next.now.Add(time.Duration(durationMin) * time.Minute)
	next.currentDutyStatus = StatusOnDutyNotDriving
	next.consecutiveOffDutyMin = 0

	// Does NOT consume remainingDriving or remainingHygiene directly, but consumes shift window and cycle
	next.remainingShiftMin -= durationMin
	next.remainingCycleMin -= durationMin

	next.dutyMinutesByDay[next.currentDayIndex] += durationMin

	return next, nil
}

// ApplyOffDuty advances the clocks by durationMin of rest / off-duty / sleeper berth time,
// checking and applying statutory resets (30-min hygiene, 10-hr daily reset, 34-hr cycle restart, sleeper split).
func (c *DriverClocks) ApplyOffDuty(durationMin int, isSleeper bool, specs PolicySpecs) (*DriverClocks, error) {
	if durationMin <= 0 {
		return c.Clone(), nil
	}

	next := c.Clone()
	next.now = next.now.Add(time.Duration(durationMin) * time.Minute)
	if isSleeper {
		next.currentDutyStatus = StatusSleeperBerth
	} else {
		next.currentDutyStatus = StatusOffDuty
	}
	next.consecutiveOffDutyMin += durationMin

	// 1. Weekly 34-Hour Restart (resets cycle clock to full 70 hours)
	if next.consecutiveOffDutyMin >= specs.WeeklyResetMin {
		next.remainingCycleMin = specs.MaxCycleMin
		next.remainingDrivingMin = specs.MaxDrivingMin + next.drivingCeilingExtension
		next.remainingShiftMin = specs.MaxShiftMin
		next.remainingHygieneMin = specs.MaxHygieneMin
		next.consecutiveDrivingMin = 0
		next.sleeperSplitActive = false
		return next, nil
	}

	// 2. Full 10-Hour Daily Reset (resets daily driving to 11h, shift to 14h, hygiene to 8h)
	if next.consecutiveOffDutyMin >= specs.DailyResetMin {
		next.remainingDrivingMin = specs.MaxDrivingMin + next.drivingCeilingExtension
		next.remainingShiftMin = specs.MaxShiftMin
		next.remainingHygieneMin = specs.MaxHygieneMin
		next.consecutiveDrivingMin = 0
		next.sleeperSplitActive = false
		return next, nil
	}

	// 3. Sleeper Berth Split Provision (8/2 or 7/3 split)
	// Qualifying short break: >= 2 hours (120 mins)
	// Qualifying long rest: >= 7 hours in sleeper berth (420 mins)
	if isSleeper && durationMin >= specs.MinSleeperBerthResetMin {
		// Long sleeper split break
		if next.sleeperSplitActive {
			// Second qualifying period completes the split reset!
			next.remainingDrivingMin = specs.MaxDrivingMin + next.drivingCeilingExtension
			next.remainingShiftMin = specs.MaxShiftMin
			next.remainingHygieneMin = specs.MaxHygieneMin
			next.consecutiveDrivingMin = 0
			next.sleeperSplitActive = false
		} else {
			// First qualifying split period
			next.sleeperSplitActive = true
			next.lastSleeperSplitMin = durationMin
			next.remainingHygieneMin = specs.MaxHygieneMin
			next.consecutiveDrivingMin = 0
		}
		return next, nil
	} else if durationMin >= specs.MinSleeperBerthBreakMin {
		// Short split break (>= 2 hours)
		if next.sleeperSplitActive {
			// Completes pair with previous long sleeper rest
			next.remainingDrivingMin = specs.MaxDrivingMin + next.drivingCeilingExtension
			next.remainingShiftMin = specs.MaxShiftMin
			next.remainingHygieneMin = specs.MaxHygieneMin
			next.consecutiveDrivingMin = 0
			next.sleeperSplitActive = false
		} else {
			next.sleeperSplitActive = true
			next.lastSleeperSplitMin = durationMin
			next.remainingHygieneMin = specs.MaxHygieneMin
			next.consecutiveDrivingMin = 0
		}
		return next, nil
	}

	// 4. Short 30-Minute Rest Break (resets 8-hour continuous drive hygiene clock)
	if durationMin >= specs.RestBreakMin {
		next.remainingHygieneMin = specs.MaxHygieneMin
		next.consecutiveDrivingMin = 0
	}

	// Off-duty time pauses the shift countdown only if it was a qualifying split break;
	// otherwise ordinary off-duty time continues to count against the 14-hour consecutive window
	if !next.sleeperSplitActive {
		next.remainingShiftMin = max(0, next.remainingShiftMin-durationMin)
	}

	return next, nil
}
