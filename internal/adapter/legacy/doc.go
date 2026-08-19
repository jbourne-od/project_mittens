// Package legacy provides high-fidelity serialization bridges and test harnesses
// for validating Java-to-Go optimizer parity under monopolistic degeneracy (N=0).
//
// Ownership & Responsibility:
//   - Parses historical Java optimizer serialized state dumps and dispatch test cases.
//   - Adapts legacy fleet resource configurations to Project Mittens model types.
//   - Executes dual-run comparison suites asserting exact numerical equivalence.
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /internal/service
//   - Imported By: /cmd/optimizer, test suites
//   - Strict Rule: Domain model and policy packages must never import this package.
//
// Active Invariants:
//   - Inviolate 1 (Parity Baselines): The N=0 degenerate test suite must evaluate standard
//     dispatch sequences and assert exact match with legacy Java outputs within standard
//     floating-point tolerance.
package legacy
