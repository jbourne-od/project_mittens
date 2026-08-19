package model_test

import (
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

func TestSimulationConfig_Validate(t *testing.T) {
	now := time.Now().UTC()

	// Valid config
	validCfg := model.SimulationConfig{
		StartTime:        now,
		EndTime:          now.Add(24 * time.Hour),
		TimeStepDuration: 15 * time.Minute,
		PlanningHorizon:  48 * time.Hour,
	}
	if err := validCfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}

	// Invalid: EndTime before StartTime
	invalidEnd := validCfg
	invalidEnd.EndTime = now.Add(-1 * time.Hour)
	if err := invalidEnd.Validate(); err == nil {
		t.Fatalf("expected error when EndTime is before StartTime")
	}

	// Invalid: Zero duration
	invalidStep := validCfg
	invalidStep.TimeStepDuration = 0
	if err := invalidStep.Validate(); err == nil {
		t.Fatalf("expected error for zero TimeStepDuration")
	}
}

func TestDefaultConfigs(t *testing.T) {
	costCfg := model.DefaultCostConfig()
	if costCfg.FixedCostPerLoad <= 0 || costCfg.LoadedMileRate <= 0 || costCfg.EmptyMileRate <= 0 {
		t.Fatalf("DefaultCostConfig has invalid non-positive cost parameters: %+v", costCfg)
	}

	feasCfg := model.DefaultFeasibilityConfig()
	if feasCfg.MaxDeadheadMiles <= 0 || feasCfg.AverageSpeedMPH <= 0 {
		t.Fatalf("DefaultFeasibilityConfig has invalid parameters: %+v", feasCfg)
	}
	if feasCfg.EarlyArrivalSentinelMin != model.AllowedEarlyNeverWaitSentinelMinutes {
		t.Fatalf("expected sentinel = 99999, got %d", feasCfg.EarlyArrivalSentinelMin)
	}
}
