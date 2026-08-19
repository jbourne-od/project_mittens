package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	// DefaultTracerScope is the default instrumentation scope name for Project Mittens.
	DefaultTracerScope = "github.com/optimaldynamics/project-mittens"
)

// TelemetryConfig configures OpenTelemetry providers and metric exporters.
type TelemetryConfig struct {
	ServiceName    string  `json:"service_name"`
	ServiceVersion string  `json:"service_version"`
	Environment    string  `json:"environment"`
	EnableTracing  bool    `json:"enable_tracing"`
	EnableMetrics  bool    `json:"enable_metrics"`
	SampleRate     float64 `json:"sample_rate"`
}

// DefaultTelemetryConfig returns standard production defaults.
func DefaultTelemetryConfig() TelemetryConfig {
	return TelemetryConfig{
		ServiceName:    "project-mittens",
		ServiceVersion: "1.0.0",
		Environment:    "production",
		EnableTracing:  true,
		EnableMetrics:  true,
		SampleRate:     1.0,
	}
}

// Provider manages the OpenTelemetry TracerProvider and MeterProvider lifecycles.
type Provider struct {
	cfg            TelemetryConfig
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	promHandler    http.Handler
	metrics        *Metrics
	shutdownOnce   sync.Once
}

var (
	globalProvider *Provider
	globalMu       sync.RWMutex
)

// InitGlobalProvider initializes the package-level default OpenTelemetry provider.
func InitGlobalProvider(cfg TelemetryConfig) (*Provider, error) {
	p, err := NewProvider(cfg)
	if err != nil {
		return nil, err
	}

	globalMu.Lock()
	globalProvider = p
	globalMu.Unlock()

	return p, nil
}

// GlobalProvider returns the currently active global telemetry provider, initializing defaults if nil.
func GlobalProvider() *Provider {
	globalMu.RLock()
	p := globalProvider
	globalMu.RUnlock()

	if p != nil {
		return p
	}

	globalMu.Lock()
	defer globalMu.Unlock()

	if globalProvider != nil {
		return globalProvider
	}

	var err error
	globalProvider, err = NewProvider(DefaultTelemetryConfig())
	if err != nil {
		panic(fmt.Sprintf("telemetry: failed initializing default global provider: %v", err))
	}
	return globalProvider
}

// NewProvider initializes OpenTelemetry tracing and metrics according to cfg.
func NewProvider(cfg TelemetryConfig) (*Provider, error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: failed creating resource: %w", err)
	}

	// 1. Tracing Setup
	var sampler sdktrace.Sampler
	if cfg.SampleRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else if cfg.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(tp)

	// 2. Metrics & Prometheus Setup
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("telemetry: failed initializing prometheus exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
	)
	otel.SetMeterProvider(mp)

	p := &Provider{
		cfg:            cfg,
		tracerProvider: tp,
		meterProvider:  mp,
		promHandler:    promhttp.Handler(),
	}

	// Initialize registered domain metrics
	metrics, err := p.initMetrics()
	if err != nil {
		return nil, fmt.Errorf("telemetry: failed registering domain metrics: %w", err)
	}
	p.metrics = metrics

	return p, nil
}

// Metrics returns the registered domain metrics instruments for this provider.
func (p *Provider) Metrics() *Metrics {
	if p == nil {
		return nil
	}
	return p.metrics
}

// Tracer returns a named tracer instance from the provider.
func (p *Provider) Tracer(name string) trace.Tracer {
	if p == nil || p.tracerProvider == nil {
		return otel.GetTracerProvider().Tracer(name)
	}
	return p.tracerProvider.Tracer(name)
}

// Meter returns a named meter instance from the provider.
func (p *Provider) Meter(name string) metric.Meter {
	if p == nil || p.meterProvider == nil {
		return otel.GetMeterProvider().Meter(name)
	}
	return p.meterProvider.Meter(name)
}

// PrometheusHandler returns the HTTP handler exposing Prometheus scrape metrics.
func (p *Provider) PrometheusHandler() http.Handler {
	if p == nil || p.promHandler == nil {
		return promhttp.Handler()
	}
	return p.promHandler
}

// Shutdown gracefully flushes and terminates tracing and metric exporters.
func (p *Provider) Shutdown(ctx context.Context) error {
	var firstErr error
	p.shutdownOnce.Do(func() {
		if p.tracerProvider != nil {
			if err := p.tracerProvider.Shutdown(ctx); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("telemetry: trace shutdown failed: %w", err)
			}
		}
		if p.meterProvider != nil {
			if err := p.meterProvider.Shutdown(ctx); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("telemetry: meter shutdown failed: %w", err)
			}
		}
	})
	return firstErr
}
