// Package reposition implements autonomous multi-region network balance calculation
// and proactive empty tractor relocation for Project Mittens.
//
// Ownership & Responsibility:
//   - Quantifies regional supply/demand imbalances across freight market clusters (headhaul surplus vs. backhaul deficit).
//   - Computes dual shadow price gradients representing the marginal future value of unassigned carrier capacity.
//   - Synthesizes optimal, HOS-compliant empty tractor repositioning moves (x_{d -> r}^{repo}) to balance network capacity.
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /internal/domain/model/hos, /pkg/logging, /pkg/telemetry
//   - Imported By: /internal/service, /internal/adapter/api, /cmd/optimizer
//   - Strict Rule: Pure domain policy execution, zero direct database access, zero network I/O.
//
// Active Invariants:
//   - Inviolate 0 (Explicit Config): Repositioning thresholds, max reposition distances, and mile costs are explicit structs.
//   - Inviolate 2 (Physical Resource Determinism): Repositioning moves preserve driver location and HOS timeline causality.
//   - Inviolate 5 (Immutability): State evaluation does not mutate parent resource states.
//   - Inviolate 6 (Lock-Free Concurrency): Parallel regional evaluations operate without mutex contention.
package reposition
