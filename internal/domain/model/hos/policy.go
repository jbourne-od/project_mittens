package hos

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidPolicySpecs is returned when HOS statutory rules contain non-positive or conflicting limits.
	ErrInvalidPolicySpecs = errors.New("domain/model/hos: invalid policy specifications")
)

// PolicySpecs defines the statutory regulatory parameters governing a driver's Hours of Service.
//
// In accordance with Inviolate 0 (Explicit Configuration), all statutory durations,
// reset thresholds, and rolling cycle lengths are strongly-typed configuration parameters.
type PolicySpecs struct {
	Name                          string
	RestBreakMin                  int // Duration of mandatory rest break (e.g. 30 mins)
	DailyResetMin                 int // Duration of full off-duty daily reset (e.g. 600 mins / 10 hrs)
	WeeklyResetMin                int // Duration of 34-hour cycle restart (e.g. 2040 mins / 34 hrs)
	MaxHygieneMin                 int // Maximum continuous driving before a 30-min break (e.g. 480 mins / 8 hrs)
	MaxDrivingMin                 int // Maximum driving minutes allowed per shift (e.g. 660 mins / 11 hrs)
	MaxShiftMin                   int // Maximum on-duty shift window span (e.g. 840 mins / 14 hrs)
	MaxCycleMin                   int // Maximum cumulative on-duty minutes in rolling cycle (e.g. 4200 mins / 70 hrs)
	CycleLengthDays               int // Number of days in the rolling cycle window (e.g. 8 days)
	MinSleeperBerthBreakMin       int // Minimum short break for sleeper berth split provision (e.g. 120 mins / 2 hrs)
	MinSleeperBerthResetMin       int // Minimum long rest for sleeper berth split provision (e.g. 420 mins / 7 hrs)
	AdverseConditionsExtensionMin int // Additional driving minutes granted for adverse driving conditions (e.g. 120 mins / 2 hrs)
}

// Validate checks that the policy specification satisfies physical and mathematical invariants.
func (ps PolicySpecs) Validate() error {
	if ps.RestBreakMin <= 0 || ps.DailyResetMin <= 0 || ps.WeeklyResetMin <= 0 {
		return fmt.Errorf("%w: reset durations must be positive", ErrInvalidPolicySpecs)
	}
	if ps.MaxHygieneMin <= 0 || ps.MaxDrivingMin <= 0 || ps.MaxShiftMin <= 0 || ps.MaxCycleMin <= 0 {
		return fmt.Errorf("%w: maximum duty limits must be positive", ErrInvalidPolicySpecs)
	}
	if ps.CycleLengthDays <= 0 {
		return fmt.Errorf("%w: cycle length must be >= 1 day", ErrInvalidPolicySpecs)
	}
	if ps.MaxDrivingMin > ps.MaxShiftMin {
		return fmt.Errorf("%w: MaxDrivingMin (%d) cannot exceed MaxShiftMin (%d)", ErrInvalidPolicySpecs, ps.MaxDrivingMin, ps.MaxShiftMin)
	}
	return nil
}

// USPolicySpecs returns the canonical US Federal Motor Carrier Safety Administration (FMCSA)
// 70-hour / 8-day rules for property-carrying commercial solo drivers.
//
// Rules Replicated:
//   - 11-hour driving limit following 10 consecutive hours off duty.
//   - 14-hour on-duty shift window.
//   - 30-minute rest break after 8 cumulative hours of driving.
//   - 70-hour / 8-day rolling cycle limit.
//   - 34-hour restart.
//   - 8/2 and 7/3 sleeper berth split provision.
func USPolicySpecs() PolicySpecs {
	return PolicySpecs{
		Name:                          "US_SOLO_70_8",
		RestBreakMin:                  30,   // 30 minutes
		DailyResetMin:                 600,  // 10 hours
		WeeklyResetMin:                2040, // 34 hours
		MaxHygieneMin:                 480,  // 8 hours
		MaxDrivingMin:                 660,  // 11 hours
		MaxShiftMin:                   840,  // 14 hours
		MaxCycleMin:                   4200, // 70 hours
		CycleLengthDays:               8,    // 8 days
		MinSleeperBerthBreakMin:       120,  // 2 hours
		MinSleeperBerthResetMin:       420,  // 7 hours
		AdverseConditionsExtensionMin: 120,  // 2 hours
	}
}

// USTeamPolicySpecs returns the operational HOS specifications for 2-driver team operations in the US.
func USTeamPolicySpecs() PolicySpecs {
	return PolicySpecs{
		Name:                          "US_TEAM_140_8",
		RestBreakMin:                  30,   // 30 minutes
		DailyResetMin:                 120,  // 2 hours (staggered rest)
		WeeklyResetMin:                1560, // 26 hours
		MaxHygieneMin:                 480,  // 8 hours
		MaxDrivingMin:                 1320, // 22 hours
		MaxShiftMin:                   1680, // 28 hours
		MaxCycleMin:                   8400, // 140 hours
		CycleLengthDays:               8,    // 8 days
		MinSleeperBerthBreakMin:       120,
		MinSleeperBerthResetMin:       420,
		AdverseConditionsExtensionMin: 120,
	}
}

// CanadianPolicySpecs returns the Canadian Commercial Vehicle Drivers Hours of Service Regulations (Cycle 1).
//
// Rules Replicated:
//   - 13-hour driving limit following 8 consecutive hours off duty.
//   - 14-hour on-duty limit within a 16-hour shift window.
//   - 70-hour / 7-day rolling cycle limit.
//   - 36-hour restart.
//   - 10 hours total off-duty per day.
func CanadianPolicySpecs() PolicySpecs {
	return PolicySpecs{
		Name:                          "CANADIAN_SOLO_70_7",
		RestBreakMin:                  480,  // 8 hours off-duty
		DailyResetMin:                 480,  // 8 hours daily reset
		WeeklyResetMin:                2160, // 36 hours cycle restart
		MaxHygieneMin:                 840,  // 14 hours on-duty
		MaxDrivingMin:                 780,  // 13 hours driving
		MaxShiftMin:                   960,  // 16 hours window
		MaxCycleMin:                   4200, // 70 hours
		CycleLengthDays:               7,    // 7 days
		MinSleeperBerthBreakMin:       120,
		MinSleeperBerthResetMin:       480,
		AdverseConditionsExtensionMin: 120,
	}
}

// CanadianTeamPolicySpecs returns the Canadian team operations HOS specifications.
func CanadianTeamPolicySpecs() PolicySpecs {
	return PolicySpecs{
		Name:                          "CANADIAN_TEAM_140_7",
		RestBreakMin:                  30,
		DailyResetMin:                 30,
		WeeklyResetMin:                2160, // 36 hours
		MaxHygieneMin:                 480,  // 8 hours
		MaxDrivingMin:                 480,  // 8 hours
		MaxShiftMin:                   480,  // 8 hours
		MaxCycleMin:                   8400, // 140 hours
		CycleLengthDays:               7,
		MinSleeperBerthBreakMin:       120,
		MinSleeperBerthResetMin:       480,
		AdverseConditionsExtensionMin: 120,
	}
}
