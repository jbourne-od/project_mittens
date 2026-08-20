// Package simulation provides ground-truth competitive market simulation and tournament evaluation harnesses
// comparing monopolistic baselines (N=0) against competitive belief-filtered MOMDP policies (N=1).
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /internal/domain/policy, /internal/service, /pkg/math, /pkg/telemetry, /pkg/logging
//   - Imported By: /cmd/tournament, external evaluation suites
//   - Strict Rule: Clean architecture adapter layer. Contains ground-truth simulation environments with zero information leaks to policies.
//
// Active Invariants:
//   - Inviolate 0: Explicit TournamentConfig and MarketConfig structs; zero init() state.
//   - Inviolate 1: Exact numerical parity in degenerate N=0 monopolistic environments.
//   - Inviolate 2 & 5: MOMDP state factoring and complete state immutability across simulation epochs.
//   - Inviolate 3: Generic competitor scale dimension support.
//   - Inviolate 6: Thread-safe, lock-free parallel tournament episode execution.
//   - Inviolate 7 & Section 19: Complete audit trails in Semantic Journal; low-cardinality telemetry.
package simulation
