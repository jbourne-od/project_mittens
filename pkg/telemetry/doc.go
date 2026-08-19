// Package telemetry provides the OpenTelemetry distributed tracing and Prometheus metrics infrastructure
// for Project Mittens, implementing strict Section 19 and AGENTS.md observability compliance.
//
// Dependency Boundaries:
//   - Imports: go.opentelemetry.io/otel, go.opentelemetry.io/otel/trace, go.opentelemetry.io/otel/metric,
//     go.opentelemetry.io/otel/sdk, go.opentelemetry.io/otel/exporters/prometheus
//   - Imported By: /internal/service, /internal/domain/policy, /internal/adapter/api, /cmd/optimizer
//   - Strict Rule: Pure standalone utility package. Does not import /internal/... domain models.
//
// Active Invariants:
//   - Inviolate 0: Explicit TelemetryConfig structs; zero init() environment mutations.
//   - Inviolate 6: Lock-free metrics accumulation using atomic counters and asynchronous OTel instruments.
//   - Section 19 Compliance: Low-cardinality label space; zero customer names, specific driver IDs,
//     dollar bids, or GPS coordinates in metric labels or span names.
package telemetry
