package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds the registered domain metric instruments.
type Metrics struct {
	OptimizationDuration   metric.Float64Histogram
	LAPSolveDuration       metric.Float64Histogram
	HOSEvalDuration        metric.Float64Histogram
	MatchesTotal           metric.Int64Counter
	InvariantFailuresTotal metric.Int64Counter
	SimplexDrift           metric.Float64Gauge
}

func (p *Provider) initMetrics() (*Metrics, error) {
	m := p.Meter(DefaultTracerScope)

	optDuration, err := m.Float64Histogram(
		"mittens.optimization.duration",
		metric.WithDescription("Duration of single-epoch optimization runs in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.010, 0.025, 0.050, 0.100, 0.250, 0.500, 1.0, 2.5, 5.0),
	)
	if err != nil {
		return nil, fmt.Errorf("failed creating optimization duration histogram: %w", err)
	}

	lapDuration, err := m.Float64Histogram(
		"mittens.lap.solve.duration",
		metric.WithDescription("Duration of Linear Assignment Problem (LAP) solving in milliseconds"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 25.0, 50.0),
	)
	if err != nil {
		return nil, fmt.Errorf("failed creating lap duration histogram: %w", err)
	}

	hosDuration, err := m.Float64Histogram(
		"mittens.hos.eval.duration",
		metric.WithDescription("Duration of HOS forward timeline simulations in milliseconds"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0),
	)
	if err != nil {
		return nil, fmt.Errorf("failed creating hos duration histogram: %w", err)
	}

	matchesTotal, err := m.Int64Counter(
		"mittens.matches.total",
		metric.WithDescription("Total number of driver-to-load assignments produced"),
		metric.WithUnit("{match}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed creating matches counter: %w", err)
	}

	failuresTotal, err := m.Int64Counter(
		"mittens.invariant.failures.total",
		metric.WithDescription("Total number of fail-closed invariant boundary violations detected"),
		metric.WithUnit("{failure}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed creating invariant failures counter: %w", err)
	}

	simplexDrift, err := m.Float64Gauge(
		"mittens.belief.simplex.drift",
		metric.WithDescription("Absolute deviation of belief distribution sum from 1.0"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed creating simplex drift gauge: %w", err)
	}

	return &Metrics{
		OptimizationDuration:   optDuration,
		LAPSolveDuration:       lapDuration,
		HOSEvalDuration:        hosDuration,
		MatchesTotal:           matchesTotal,
		InvariantFailuresTotal: failuresTotal,
		SimplexDrift:           simplexDrift,
	}, nil
}

// GlobalMetrics returns the registered global metrics instruments.
func GlobalMetrics() *Metrics {
	return GlobalProvider().Metrics()
}

// RecordOptimizationDuration records the wall-clock execution time of an optimization run.
func RecordOptimizationDuration(ctx context.Context, durationSec float64, policyClass string, status string) {
	if m := GlobalMetrics(); m != nil && m.OptimizationDuration != nil {
		attrs := []attribute.KeyValue{
			attribute.String("policy.class", policyClass),
			attribute.String("status", status),
		}
		m.OptimizationDuration.Record(ctx, durationSec, metric.WithAttributes(attrs...))
	}
}

// RecordLAPDuration records the solve duration of an assignment problem.
func RecordLAPDuration(ctx context.Context, durationMs float64, rows, cols int) {
	if m := GlobalMetrics(); m != nil && m.LAPSolveDuration != nil {
		dimBucket := "small"
		maxDim := rows
		if cols > maxDim {
			maxDim = cols
		}
		if maxDim > 200 {
			dimBucket = "large"
		} else if maxDim > 50 {
			dimBucket = "medium"
		}

		attrs := []attribute.KeyValue{
			attribute.String("matrix.bucket", dimBucket),
		}
		m.LAPSolveDuration.Record(ctx, durationMs, metric.WithAttributes(attrs...))
	}
}

// RecordHOSEvalDuration records the duration of an HOS forward timeline evaluation.
func RecordHOSEvalDuration(ctx context.Context, durationMs float64) {
	if m := GlobalMetrics(); m != nil && m.HOSEvalDuration != nil {
		m.HOSEvalDuration.Record(ctx, durationMs)
	}
}

// RecordMatchesProduced increments the matched assignments counter.
func RecordMatchesProduced(ctx context.Context, count int64, policyClass string) {
	if m := GlobalMetrics(); m != nil && m.MatchesTotal != nil && count > 0 {
		attrs := []attribute.KeyValue{
			attribute.String("policy.class", policyClass),
		}
		m.MatchesTotal.Add(ctx, count, metric.WithAttributes(attrs...))
	}
}

// RecordInvariantFailure increments the invariant boundary failure counter.
func RecordInvariantFailure(ctx context.Context, invariantName string) {
	if m := GlobalMetrics(); m != nil && m.InvariantFailuresTotal != nil {
		attrs := []attribute.KeyValue{
			attribute.String("invariant.name", invariantName),
		}
		m.InvariantFailuresTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordSimplexDrift sets the active simplex precision drift value.
func RecordSimplexDrift(ctx context.Context, drift float64) {
	if m := GlobalMetrics(); m != nil && m.SimplexDrift != nil {
		m.SimplexDrift.Record(ctx, drift)
	}
}
