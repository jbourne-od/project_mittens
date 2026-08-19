package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
)

var (
	// ErrInvalidConfig is returned when configuration parameters fail validation.
	ErrInvalidConfig = errors.New("domain/model: invalid configuration parameters")
)

// AllowedEarlyNeverWaitSentinelMinutes is the sentinel constant (99999 minutes)
// representing "never wait" early loading/unloading start semantics matching legacy Java FleetManager.
const AllowedEarlyNeverWaitSentinelMinutes = 99999

// SimulationConfig specifies the temporal execution parameters for a simulation run.
type SimulationConfig struct {
	StartTime          time.Time
	EndTime            time.Time
	TimeStepDuration   time.Duration
	PlanningHorizon    time.Duration
	RandomSeed         uint64
	EnableCompetitors  bool
	CompetitorPostures int
}

// Validate verifies the simulation parameters satisfy physical constraints.
func (c SimulationConfig) Validate() error {
	if c.StartTime.IsZero() || c.EndTime.IsZero() {
		return fmt.Errorf("%w: StartTime and EndTime must be set", ErrInvalidConfig)
	}
	if !c.EndTime.After(c.StartTime) {
		return fmt.Errorf("%w: EndTime (%v) must be after StartTime (%v)", ErrInvalidConfig, c.EndTime, c.StartTime)
	}
	if c.TimeStepDuration <= 0 {
		return fmt.Errorf("%w: TimeStepDuration must be positive", ErrInvalidConfig)
	}
	if c.PlanningHorizon <= 0 {
		return fmt.Errorf("%w: PlanningHorizon must be positive", ErrInvalidConfig)
	}
	return nil
}

// CostConfig specifies the economic valuation hyperparameters for the objective function.
//
// In accordance with legacy CostFunctions.java:
// TotalCost = FixedCost + (LoadedRate * LoadedMiles) + (EmptyRate * EmptyMiles) + (EmptyToHomeRate * EmptyToHomeMiles) - Bonus
type CostConfig struct {
	FixedCostPerLoad     float64 // Fixed dispatch cost ($/load)
	LoadedMileRate       float64 // Loaded transit cost ($/mile)
	EmptyMileRate        float64 // Empty deadhead repositioning cost ($/mile)
	EmptyToHomeRate      float64 // Empty-to-home repositioning penalty ($/mile)
	LateDeliveryPerHour  float64 // Penalty per hour past latest delivery appointment window ($/hr)
	EarlyArrivalPerHour  float64 // Dwell cost per hour before earliest appointment window ($/hr)
	DriverBonusWeight    float64 // Multiplier for driver retention / preferred lane bonuses
	OpportunityCostRatio float64 // Multiplier for rejected load opportunity valuation
}

// DefaultCostConfig provides standard commercial truckload cost parameters.
func DefaultCostConfig() CostConfig {
	return CostConfig{
		FixedCostPerLoad:     50.0,
		LoadedMileRate:       1.85,
		EmptyMileRate:        1.45,
		EmptyToHomeRate:      1.65,
		LateDeliveryPerHour:  100.0,
		EarlyArrivalPerHour:  25.0,
		DriverBonusWeight:    1.0,
		OpportunityCostRatio: 0.15,
	}
}

// Validate verifies that cost parameters are non-negative and finite.
func (c CostConfig) Validate() error {
	if c.FixedCostPerLoad < 0 || c.LoadedMileRate < 0 || c.EmptyMileRate < 0 || c.EmptyToHomeRate < 0 {
		return fmt.Errorf("%w: cost rates must be non-negative", ErrInvalidConfig)
	}
	if c.LateDeliveryPerHour < 0 || c.EarlyArrivalPerHour < 0 {
		return fmt.Errorf("%w: penalty rates must be non-negative", ErrInvalidConfig)
	}
	return nil
}

// FeasibilityConfig governs physical candidate arc generation and pruning thresholds.
type FeasibilityConfig struct {
	MaxDeadheadMiles        float64
	MaxEarlyDwellHours      float64
	MaxLateDeliveryHours    float64
	AverageSpeedMPH         float64
	EarlyArrivalSentinelMin int
	HOSPolicySpecs          hos.PolicySpecs
}

// Validate verifies that feasibility thresholds are positive and valid.
func (f FeasibilityConfig) Validate() error {
	if f.AverageSpeedMPH <= 0 {
		return fmt.Errorf("%w: AverageSpeedMPH must be positive", ErrInvalidConfig)
	}
	if f.MaxDeadheadMiles < 0 {
		return fmt.Errorf("%w: MaxDeadheadMiles must be non-negative", ErrInvalidConfig)
	}
	return nil
}

// DefaultFeasibilityConfig provides standard candidate generation thresholds.
func DefaultFeasibilityConfig() FeasibilityConfig {
	return FeasibilityConfig{
		MaxDeadheadMiles:        250.0,
		MaxEarlyDwellHours:      12.0,
		MaxLateDeliveryHours:    2.0,
		AverageSpeedMPH:         50.0,
		EarlyArrivalSentinelMin: AllowedEarlyNeverWaitSentinelMinutes,
		HOSPolicySpecs:          hos.USPolicySpecs(),
	}
}
