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
