// Package api implements the production HTTP REST API microservice and transport adapter
// for Project Mittens, exposing real-time fleet dispatch optimization and multi-epoch rolling simulation.
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /internal/domain/policy, /internal/service, /internal/service/dispatch
//   - Strict Rule: Clean Architecture infrastructure layer. No domain model or policy code may import this package.
//
// Active Invariants:
//   - Inviolate 0: Explicit ServerConfig structs; zero init() environment mutations.
//   - Inviolate 5: State Immutability preserved during DTO conversion into domain models.
//   - Inviolate 6: Concurrent lock-free execution across parallel HTTP worker goroutines.
//   - Section 19: Low-cardinality Prometheus metrics exposition on /metrics; zero sensitive PII in standard logs.
package api
