// Package service implements carrier optimization orchestration, multi-day simulation,
// and adaptive value function approximation learning loops.
//
// In accordance with Project Mittens AGENTS.md:
//   - Dependency Boundaries: Imports /internal/domain/model, /internal/domain/policy, /internal/service/dispatch, /pkg/logging
//   - Inviolate 5: State immutability via value-based allocation.
//   - Inviolate 6: Lock-free concurrency on hot paths.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/internal/service/dispatch"
	"github.com/optimaldynamics/project-mittens/pkg/logging"
	"github.com/optimaldynamics/project-mittens/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// RollingHorizonConfig encapsulates settings for multi-day rolling horizon carrier operations.
type RollingHorizonConfig struct {
	RunID             string  `json:"run_id"`
	StartEpoch        int64   `json:"start_epoch"`
	HorizonDays       int     `json:"horizon_days"`
	DecisionStepHours int     `json:"decision_step_hours"`
	EnableRelays      bool    `json:"enable_relays"`
	MinRelayHaulMiles float64 `json:"min_relay_haul_miles"`
	EnableVFALearning bool    `json:"enable_vfa_learning"`
}

// DefaultRollingHorizonConfig returns standard settings for a 7-day daily rolling horizon run.
func DefaultRollingHorizonConfig() RollingHorizonConfig {
	return RollingHorizonConfig{
		RunID:             "ROLLING_HORIZON_7D",
		StartEpoch:        time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix(),
		HorizonDays:       7,
		DecisionStepHours: 24,
		EnableRelays:      true,
		MinRelayHaulMiles: 450.0,
		EnableVFALearning: true,
	}
}

// Validate checks that rolling horizon parameters are physically sound.
func (cfg RollingHorizonConfig) Validate() error {
	if cfg.StartEpoch <= 0 {
		return fmt.Errorf("service: StartEpoch must be positive, got %d", cfg.StartEpoch)
	}
	if cfg.HorizonDays <= 0 {
		return fmt.Errorf("service: HorizonDays must be >= 1, got %d", cfg.HorizonDays)
	}
	if cfg.DecisionStepHours <= 0 {
		return fmt.Errorf("service: DecisionStepHours must be >= 1, got %d", cfg.DecisionStepHours)
	}
	return nil
}

// DailyKPISnapshot records fleet operational performance for an individual day.
type DailyKPISnapshot struct {
	DayIndex             int     `json:"day_index"`
	Epoch                int64   `json:"epoch"`
	ActiveDrivers        int     `json:"active_drivers"`
	AvailableLoads       int     `json:"available_loads"`
	MatchedLoads         int     `json:"matched_loads"`
	DirectToursCount     int     `json:"direct_tours_count"`
	RelayExchangesCount  int     `json:"relay_exchanges_count"`
	TotalLoadedMiles     float64 `json:"total_loaded_miles"`
	TotalEmptyMiles      float64 `json:"total_empty_miles"`
	EmptyRatio           float64 `json:"empty_ratio"`
	TotalGrossRevenue    float64 `json:"total_gross_revenue"`
	TotalOperatingCost   float64 `json:"total_operating_cost"`
	TotalNetContribution float64 `json:"total_net_contribution"`
}

// RollingHorizonReport encapsulates the complete audit of a multi-day carrier execution run.
type RollingHorizonReport struct {
	RunID                string             `json:"run_id"`
	StartEpoch           int64              `json:"start_epoch"`
	EndEpoch             int64              `json:"end_epoch"`
	TotalDays            int                `json:"total_days"`
	TotalEpochs          int                `json:"total_epochs"`
	DailySnapshots       []DailyKPISnapshot `json:"daily_snapshots"`
	GlobalKPIs           KPIReport          `json:"global_kpis"`
	TotalDirectTours     int                `json:"total_direct_tours"`
	TotalRelayExchanges  int                `json:"total_relay_exchanges"`
	TotalLoadedMiles     float64            `json:"total_loaded_miles"`
	TotalEmptyMiles      float64            `json:"total_empty_miles"`
	GlobalEmptyRatio     float64            `json:"global_empty_ratio"`
	TotalGrossRevenue    float64            `json:"total_gross_revenue"`
	TotalOperatingCost   float64            `json:"total_operating_cost"`
	TotalNetContribution float64            `json:"total_net_contribution"`
}

// RollingHorizonRunner orchestrates end-to-end multi-day rolling horizon optimization simulations.
type RollingHorizonRunner[C model.CompetitorScale] struct {
	optService  *OptimizationService[C]
	relayRunner *dispatch.RelayDispatchRunner
	vfaLearner  *PiecewiseVFALearner
	logger      *slog.Logger
}

// NewRollingHorizonRunner constructs a new RollingHorizonRunner.
func NewRollingHorizonRunner[C model.CompetitorScale](
	optService *OptimizationService[C],
	relayRunner *dispatch.RelayDispatchRunner,
	vfaLearner *PiecewiseVFALearner,
	logger *slog.Logger,
) *RollingHorizonRunner[C] {
	if optService == nil {
		optService = NewOptimizationService[C](nil, logger)
	}
	if relayRunner == nil {
		relayRunner = dispatch.NewRelayDispatchRunner(nil, nil)
	}
	if logger == nil {
		logger = logging.NewNop()
	}
	return &RollingHorizonRunner[C]{
		optService:  optService,
		relayRunner: relayRunner,
		vfaLearner:  vfaLearner,
		logger:      logger,
	}
}

// Run executes the complete multi-day rolling horizon simulation.
func (r *RollingHorizonRunner[C]) Run(
	ctx context.Context,
	cfg RollingHorizonConfig,
	initialState *model.State[C],
	pol policy.Policy[C],
	stream LoadStream,
) (*RollingHorizonReport, *model.State[C], error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	if initialState == nil {
		return nil, nil, fmt.Errorf("service: initialState cannot be nil")
	}
	if pol == nil {
		return nil, nil, fmt.Errorf("service: policy cannot be nil")
	}

	stepSec := int64(cfg.DecisionStepHours * 3600)
	totalSec := int64(cfg.HorizonDays * 86400)
	endEpoch := cfg.StartEpoch + totalSec

	stats := NewStatisticCalculator()
	currentState := initialState
	dailySnapshots := make([]DailyKPISnapshot, 0, cfg.HorizonDays)

	totalDirectTours := 0
	totalRelays := 0
	totalLoadedMiles := 0.0
	totalEmptyMiles := 0.0
	totalRevenue := 0.0
	totalCost := 0.0
	totalNet := 0.0

	ctx, span := telemetry.StartSpan(ctx, "RollingHorizon.Run")
	defer span.End()
	span.SetAttributes(telemetry.SimulationSpanAttributes(cfg.RunID, cfg.HorizonDays, 0)...)

	logger := logging.FromContext(ctx, r.logger)
	logger.InfoContext(ctx, "starting rolling horizon simulation",
		slog.String("run_id", cfg.RunID),
		slog.Int("horizon_days", cfg.HorizonDays),
		slog.Int("decision_step_hours", cfg.DecisionStepHours),
		slog.String("policy", pol.Name()),
	)

	currentLearner := r.vfaLearner
	epochCount := 0

	for epoch := cfg.StartEpoch; epoch < endEpoch; epoch += stepSec {
		select {
		case <-ctx.Done():
			return nil, currentState, ctx.Err()
		default:
		}

		dayIndex := int((epoch - cfg.StartEpoch) / 86400)
		nextEpoch := epoch + stepSec

		// 1. Ingest new load arrivals for current epoch
		var newLoads []model.Load
		if stream != nil {
			newLoads = stream.GetLoadsForEpoch(epoch)
		}
		stats.RecordLoadOffers(len(newLoads))

		availableDrivers := currentState.Resource().Drivers()
		availableLoads := currentState.Resource().Loads()

		// 2. Evaluate Policy & Solve Matching
		action, prov, nextState, err := r.optService.OptimizeEpoch(
			ctx,
			currentState,
			pol,
			nextEpoch,
			newLoads,
		)
		if err != nil {
			return nil, currentState, fmt.Errorf("service: epoch %d optimization failed: %w", epoch, err)
		}

		// 3. Adaptive VFA Learning update if enabled
		if cfg.EnableVFALearning && currentLearner != nil {
			matchingSol := policy.MatchingSolution{
				Matches:              action.Matches(),
				Evaluations:          prov.EvaluatedArcs,
				TotalObjective:       prov.TotalObjectiveValue,
				TotalNetContribution: prov.TotalNetContribution,
				DriverDualValues:     make(map[string]float64),
			}
			// Extract duals from matching solution evaluations
			for _, eval := range prov.EvaluatedArcs {
				if eval.IsAssigned {
					matchingSol.DriverDualValues[eval.DriverID] = eval.CostBreakdown.NetContribution
				}
			}

			updatedLearner, err := currentLearner.UpdateFromMatching(matchingSol, availableDrivers, availableLoads)
			if err == nil {
				currentLearner = updatedLearner
			}
		}

		// 4. Synthesize Multi-Leg Tours & Relays
		var dispatchBatch *dispatch.RelayDispatchBatch
		if cfg.EnableRelays {
			batch, err := r.relayRunner.SynthesizeRelayBatch(
				ctx,
				epoch,
				availableDrivers,
				action.Matches(),
				availableLoads,
				cfg.MinRelayHaulMiles,
			)
			if err == nil {
				dispatchBatch = batch
			}
		}

		// 5. Daily metrics snapshot
		dayLoaded := 0.0
		dayEmpty := 0.0
		dayRev := 0.0
		dayCost := 0.0
		dayNet := 0.0
		directToursInEpoch := 0
		relaysInEpoch := 0

		if dispatchBatch != nil {
			dayLoaded = dispatchBatch.TotalLoadedMiles
			dayEmpty = dispatchBatch.TotalEmptyMiles
			dayRev = dispatchBatch.TotalGrossRevenue
			dayCost = dispatchBatch.TotalOperatingCost
			dayNet = dispatchBatch.TotalNetContribution
			directToursInEpoch = dispatchBatch.TotalTours
			relaysInEpoch = dispatchBatch.TotalRelays
		} else {
			dayRev = prov.TotalNetContribution
			dayNet = prov.TotalNetContribution
		}

		totalDirectTours += directToursInEpoch
		totalRelays += relaysInEpoch
		totalLoadedMiles += dayLoaded
		totalEmptyMiles += dayEmpty
		totalRevenue += dayRev
		totalCost += dayCost
		totalNet += dayNet

		dayDistance := dayLoaded + dayEmpty
		dayEmptyRatio := 0.0
		if dayDistance > 0 {
			dayEmptyRatio = dayEmpty / dayDistance
		}

		snapshot := DailyKPISnapshot{
			DayIndex:             dayIndex,
			Epoch:                epoch,
			ActiveDrivers:        len(availableDrivers),
			AvailableLoads:       len(availableLoads),
			MatchedLoads:         action.MatchCount(),
			DirectToursCount:     directToursInEpoch,
			RelayExchangesCount:  relaysInEpoch,
			TotalLoadedMiles:     dayLoaded,
			TotalEmptyMiles:      dayEmpty,
			TotalGrossRevenue:    dayRev,
			TotalOperatingCost:   dayCost,
			TotalNetContribution: dayNet,
			EmptyRatio:           dayEmptyRatio,
		}
		dailySnapshots = append(dailySnapshots, snapshot)

		currentState = nextState
		epochCount++
	}

	globalTotalDist := totalLoadedMiles + totalEmptyMiles
	globalEmptyRatio := 0.0
	if globalTotalDist > 0 {
		globalEmptyRatio = totalEmptyMiles / globalTotalDist
	}

	report := &RollingHorizonReport{
		RunID:                cfg.RunID,
		StartEpoch:           cfg.StartEpoch,
		EndEpoch:             endEpoch,
		TotalDays:            cfg.HorizonDays,
		TotalEpochs:          epochCount,
		DailySnapshots:       dailySnapshots,
		GlobalKPIs:           stats.Snapshot(),
		TotalDirectTours:     totalDirectTours,
		TotalRelayExchanges:  totalRelays,
		TotalLoadedMiles:     totalLoadedMiles,
		TotalEmptyMiles:      totalEmptyMiles,
		GlobalEmptyRatio:     globalEmptyRatio,
		TotalGrossRevenue:    totalRevenue,
		TotalOperatingCost:   totalCost,
		TotalNetContribution: totalNet,
	}

	span.SetAttributes(attribute.Int("simulation.epoch_count", epochCount))

	logger.InfoContext(ctx, "rolling horizon simulation complete",
		slog.String("run_id", cfg.RunID),
		slog.Int("total_days", cfg.HorizonDays),
		slog.Int("total_tours", totalDirectTours),
		slog.Int("total_relays", totalRelays),
		slog.Float64("total_net", totalNet),
		slog.Float64("empty_ratio", globalEmptyRatio),
	)

	return report, currentState, nil
}
