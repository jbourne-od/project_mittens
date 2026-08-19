// Package feasibility implements concurrent physical feasibility checking and candidate arc generation
// for driver-load assignment subproblems in Project Mittens.
//
// Ownership & Responsibility:
//   - Evaluates equipment compatibility, endorsements, deadhead distance limits, and HOS regulations.
//   - Uses modern Go concurrency (worker pools, GMP multiplexing, context cancellation) to evaluate
//     large candidate matrices with high throughput.
//   - Guarantees deterministic, canonically sorted candidate arc lists (Principle 2).
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /internal/domain/model/hos.
//   - Strict Rule: Zero I/O, pure offline computation, zero shared-memory mutexes.
package feasibility
