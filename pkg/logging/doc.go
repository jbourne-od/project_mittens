// Package logging provides structured, high-performance, and Section 19 compliant logging
// built on Go's standard log/slog library for Project Mittens.
//
// Ownership & Responsibility:
//   - Configures structured JSON and Text handlers with fine-grained log-level gating.
//   - Manages trace and execution context propagation (OptimizationRunID, BatchEpoch).
//   - Enforces Section 5.1 & Section 19 observability invariants (no raw coordinate PII in application logs).
//   - Provides zero-allocation Nop loggers for high-throughput Monte Carlo rollouts.
//
// Dependency Boundaries:
//   - Imports: standard library only (log/slog, context, io, os).
//   - Imported By: /internal/*, /cmd/*, /pkg/*
package logging
