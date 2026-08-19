// Package db implements database persistence and Semantic Journaling for optimization runs.
//
// Ownership & Responsibility:
//   - Manages transactional persistence of optimization batches and solution states.
//   - Implements the append-only Semantic Journal recording full decision provenance
//     (state inputs, active theta, belief vectors, and alternative marginal contributions).
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /internal/service
//   - Imported By: /cmd/optimizer
//   - Strict Rule: Domain model and policy packages must never import this package.
//
// Active Invariants:
//   - Inviolate 7 (Complete Provenance): Decision records must store all evaluated alternatives
//     and belief state context within transaction boundaries.
//   - Section 19 (Observability & Security): Ordinary application logs must remain non-sensitive;
//     sensitive matching details and bid evaluations reside solely in audited journal storage.
package db
