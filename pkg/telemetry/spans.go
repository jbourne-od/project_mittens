package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// StartSpan creates a new OpenTelemetry span in the default Mittens tracer scope.
func StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	p := GlobalProvider()
	tracer := p.Tracer(DefaultTracerScope)
	return tracer.Start(ctx, spanName, opts...)
}

// OptimizationSpanAttributes creates standardized low-cardinality attributes for optimization spans.
func OptimizationSpanAttributes(policyClass string, driverCount, loadCount int, competitorScale int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("policy.class", policyClass),
		attribute.Int("fleet.driver_count", driverCount),
		attribute.Int("fleet.load_count", loadCount),
		attribute.Int("model.competitor_scale", competitorScale),
	}
}

// FeasibilitySpanAttributes creates attributes for candidate feasibility filtering spans.
func FeasibilitySpanAttributes(evaluatedPairs, feasiblePairs int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int("feasibility.evaluated_pairs", evaluatedPairs),
		attribute.Int("feasibility.feasible_pairs", feasiblePairs),
	}
}

// SimulationSpanAttributes creates attributes for multi-epoch simulation runs.
func SimulationSpanAttributes(runID string, horizonDays, epochCount int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("simulation.run_id", runID),
		attribute.Int("simulation.horizon_days", horizonDays),
		attribute.Int("simulation.epoch_count", epochCount),
	}
}

// LAPSpanAttributes creates attributes for exact linear assignment network flow spans.
func LAPSpanAttributes(rows, cols int, algorithm string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int("lap.matrix_rows", rows),
		attribute.Int("lap.matrix_cols", cols),
		attribute.String("lap.algorithm", algorithm),
		attribute.Bool("lap.dual_shadow_prices_extracted", true),
	}
}

// DLASpanAttributes creates attributes for direct lookahead approximation spans.
func DLASpanAttributes(horizon, rollouts, branches int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int("dla.horizon_depth", horizon),
		attribute.Int("dla.monte_carlo_rollouts", rollouts),
		attribute.Int("dla.concurrent_branches", branches),
	}
}

// BeliefSpanAttributes creates attributes for Bayesian belief filter updates.
func BeliefSpanAttributes(simplexSize int, observationType string, drift float64) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int("belief.simplex_size", simplexSize),
		attribute.String("belief.observation_type", observationType),
		attribute.Float64("belief.simplex_drift", drift),
	}
}

// TourSpanAttributes creates attributes for multi-leg tour synthesis spans.
func TourSpanAttributes(maxLegs, synthesizedCount, domicileReturns int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int("tour.max_legs", maxLegs),
		attribute.Int("tour.synthesized_count", synthesizedCount),
		attribute.Int("tour.domicile_returns", domicileReturns),
	}
}

// RelaySpanAttributes creates attributes for relay facility exchange evaluation spans.
func RelaySpanAttributes(evaluated, feasible int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int("relay.evaluated_pairs", evaluated),
		attribute.Int("relay.feasible_relays", feasible),
	}
}
