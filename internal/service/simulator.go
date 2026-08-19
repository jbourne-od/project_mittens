package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/pkg/logging"
)

// LoadStream supplies newly realized customer load offers D_{t+1} as simulation time advances.
type LoadStream interface {
	GetLoadsForEpoch(epoch int64) []model.Load
}

// StaticLoadStream provides a deterministic in-memory mapping of arrival epochs to customer loads.
type StaticLoadStream struct {
	arrivals map[int64][]model.Load
}

// NewStaticLoadStream creates a StaticLoadStream from an arrival schedule map.
func NewStaticLoadStream(arrivals map[int64][]model.Load) *StaticLoadStream {
	copied := make(map[int64][]model.Load, len(arrivals))
	for k, v := range arrivals {
		slice := make([]model.Load, len(v))
		for i, l := range v {
			slice[i] = l.Clone()
		}
		copied[k] = slice
	}
	return &StaticLoadStream{arrivals: copied}
}

// GetLoadsForEpoch returns all customer loads appearing at the specified epoch.
func (s *StaticLoadStream) GetLoadsForEpoch(epoch int64) []model.Load {
	if s == nil || s.arrivals == nil {
		return nil
	}
	loads, exists := s.arrivals[epoch]
	if !exists {
		return nil
	}
	out := make([]model.Load, len(loads))
	for i, l := range loads {
		out[i] = l.Clone()
	}
	return out
}

// SimulationRunConfig encapsulates time horizons and execution settings for a multi-step simulation.
type SimulationRunConfig struct {
	RunID       string `json:"run_id"`
	StartEpoch  int64  `json:"start_epoch"`
	EndEpoch    int64  `json:"end_epoch"`
	StepSeconds int64  `json:"step_seconds"`
}

// Validate checks that simulation parameters are physically valid.
func (cfg SimulationRunConfig) Validate() error {
	if cfg.StartEpoch <= 0 {
		return fmt.Errorf("simulator: StartEpoch must be > 0, got %d", cfg.StartEpoch)
	}
	if cfg.EndEpoch < cfg.StartEpoch {
		return fmt.Errorf("simulator: EndEpoch %d < StartEpoch %d", cfg.EndEpoch, cfg.StartEpoch)
	}
	if cfg.StepSeconds <= 0 {
		return fmt.Errorf("simulator: StepSeconds must be > 0, got %d", cfg.StepSeconds)
	}
	return nil
}

// SimulationSummary encapsulates the end-to-end results of a completed time-stepping simulation.
type SimulationSummary struct {
	RunID          string         `json:"run_id"`
	StartEpoch     int64          `json:"start_epoch"`
	EndEpoch       int64          `json:"end_epoch"`
	TotalEpochs    int            `json:"total_epochs"`
	KPIs           KPIReport      `json:"kpis"`
	JournalEntries []JournalEntry `json:"journal_entries"`
}

// TimeSteppingSimulator executes rolling-horizon discrete event simulations over MOMDP carrier networks.
type TimeSteppingSimulator[C model.CompetitorScale] struct {
	service *OptimizationService[C]
	logger  *slog.Logger
}

// NewTimeSteppingSimulator initializes a TimeSteppingSimulator.
func NewTimeSteppingSimulator[C model.CompetitorScale](
	service *OptimizationService[C],
	logger *slog.Logger,
) *TimeSteppingSimulator[C] {
	if service == nil {
		service = NewOptimizationService[C](nil, logger)
	}
	if logger == nil {
		logger = logging.NewNop()
	}
	return &TimeSteppingSimulator[C]{
		service: service,
		logger:  logger,
	}
}

// Run executes a multi-step time simulation from StartEpoch to EndEpoch in discrete StepSeconds intervals.
func (sim *TimeSteppingSimulator[C]) Run(
	ctx context.Context,
	cfg SimulationRunConfig,
	initialState *model.State[C],
	pol policy.Policy[C],
	stream LoadStream,
) (*SimulationSummary, *model.State[C], error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	if initialState == nil {
		return nil, nil, fmt.Errorf("simulator: initialState cannot be nil")
	}
	if pol == nil {
		return nil, nil, fmt.Errorf("simulator: policy cannot be nil")
	}

	stats := NewStatisticCalculator()
	currentState := initialState
	totalEpochs := 0

	runCtx := logging.WithContextData(ctx, logging.ContextData{
		OptimizationRunID: cfg.RunID,
		PolicyClass:       pol.Name(),
	})
	logger := logging.FromContext(runCtx, sim.logger)

	logger.InfoContext(runCtx, "starting multi-epoch simulation run",
		slog.String("run_id", cfg.RunID),
		slog.Int64("start_epoch", cfg.StartEpoch),
		slog.Int64("end_epoch", cfg.EndEpoch),
		slog.Int64("step_seconds", cfg.StepSeconds),
		slog.String("policy", pol.Name()),
	)

	for epoch := cfg.StartEpoch; epoch <= cfg.EndEpoch; epoch += cfg.StepSeconds {
		select {
		case <-ctx.Done():
			logger.WarnContext(runCtx, "simulation cancelled by context", slog.String("error", ctx.Err().Error()))
			return nil, currentState, ctx.Err()
		default:
		}

		nextEpoch := epoch + cfg.StepSeconds
		newLoads := []model.Load{}
		if stream != nil {
			newLoads = stream.GetLoadsForEpoch(epoch)
		}

		stats.RecordLoadOffers(len(newLoads))
		drivers := currentState.Resource().Drivers()
		availableHours := float64(len(drivers)) * (float64(cfg.StepSeconds) / 3600.0)
		stats.RecordDriverHours(0.0, availableHours)

		action, prov, nextState, err := sim.service.OptimizeEpoch(
			runCtx,
			currentState,
			pol,
			nextEpoch,
			newLoads,
		)
		if err != nil {
			return nil, currentState, fmt.Errorf("simulator: epoch %d optimization failed: %w", epoch, err)
		}

		// Accumulate metrics from evaluated provenance
		for _, eval := range prov.EvaluatedArcs {
			if eval.IsAssigned {
				var deadheadMiles float64
				var loadedMiles float64
				if d, ok := currentState.Resource().GetDriver(eval.DriverID); ok {
					if l, ok := currentState.Resource().GetLoad(eval.LoadID); ok {
						deadheadMiles = d.CurrentLocation.DistanceMiles(l.Origin)
						loadedMiles = l.Origin.DistanceMiles(l.Destination)
					}
				}
				dwellMin := int(eval.CostBreakdown.DwellCost)
				stats.RecordDispatch(eval.CostBreakdown, loadedMiles, deadheadMiles, dwellMin, 0, true, true)
			}
		}

		_ = action

		currentState = nextState
		totalEpochs++
	}

	kpis := stats.Snapshot()
	entries := sim.service.Journal().GetEntries()

	logger.InfoContext(runCtx, "simulation run completed",
		slog.String("run_id", cfg.RunID),
		slog.Int("total_epochs", totalEpochs),
		slog.Int("total_dispatches", kpis.LoadsServiced),
		slog.Float64("gross_revenue", kpis.GrossRevenue),
		slog.Float64("net_profit", kpis.NetContribution),
		slog.Float64("empty_ratio", kpis.EmptyRatio),
	)

	summary := &SimulationSummary{
		RunID:          cfg.RunID,
		StartEpoch:     cfg.StartEpoch,
		EndEpoch:       cfg.EndEpoch,
		TotalEpochs:    totalEpochs,
		KPIs:           kpis,
		JournalEntries: entries,
	}

	return summary, currentState, nil
}
