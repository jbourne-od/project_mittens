package telemetry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/pkg/telemetry"
	"go.opentelemetry.io/otel/trace"
)

func TestTelemetry_ProviderLifecycle(t *testing.T) {
	cfg := telemetry.DefaultTelemetryConfig()
	cfg.ServiceName = "test-mittens"
	p, err := telemetry.NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	tracer := p.Tracer("test-tracer")
	if tracer == nil {
		t.Fatalf("expected non-nil tracer")
	}

	meter := p.Meter("test-meter")
	if meter == nil {
		t.Fatalf("expected non-nil meter")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestTelemetry_SpanRecording(t *testing.T) {
	ctx := context.Background()
	_, span := telemetry.StartSpan(ctx, "TestSpan", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	if !span.SpanContext().IsValid() {
		t.Errorf("expected valid span context")
	}

	attrs := telemetry.OptimizationSpanAttributes("CFA", 10, 50, 0)
	span.SetAttributes(attrs...)
}

func TestTelemetry_MetricsAndPrometheusScrape(t *testing.T) {
	p := telemetry.GlobalProvider()
	ctx := context.Background()

	// Record sample metrics
	telemetry.RecordOptimizationDuration(ctx, 0.042, "CFA", "success")
	telemetry.RecordLAPDuration(ctx, 2.5, 50, 100)
	telemetry.RecordMatchesProduced(ctx, 15, "CFA")
	telemetry.RecordInvariantFailure(ctx, "Inviolate_SimplexSum")
	telemetry.RecordSimplexDrift(ctx, 1e-12)

	// Scrape Prometheus endpoint
	handler := p.PrometheusHandler()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from Prometheus handler, got %d", rr.Code)
	}

	body := rr.Body.String()
	t.Logf("Prometheus Scrape Output (%d bytes):\n%s", len(body), body)

	// Verify low-cardinality metric names
	expectedSubstrings := []string{
		"mittens_optimization_duration",
		"mittens_lap_solve_duration",
		"mittens_matches_total",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(body, sub) {
			t.Errorf("expected Prometheus body to contain '%s'", sub)
		}
	}
}

func TestTelemetry_ConcurrentMetricRecording(t *testing.T) {
	ctx := context.Background()
	var wg sync.WaitGroup
	goroutines := 20
	iterations := 100

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				telemetry.RecordOptimizationDuration(ctx, 0.005, "CFA", "success")
				telemetry.RecordLAPDuration(ctx, 0.5, 10, 20)
				telemetry.RecordMatchesProduced(ctx, 1, "CFA")
			}
		}(g)
	}

	wg.Wait()
}
