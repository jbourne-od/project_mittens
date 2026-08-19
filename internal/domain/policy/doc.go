// Package policy implements the four universal classes of Powell policies
// (PFAs, CFAs, VFAs, and DLAs) over the belief-state MDP.
//
// Ownership & Responsibility:
//   - Implements Policy Function Approximations (PFAs) for direct state-to-decision mapping.
//   - Implements Cost Function Approximations (CFAs) with parametric shifts for network balance
//     and competitive risk premiums.
//   - Implements Value Function Approximations (VFAs) evaluating post-decision states.
//   - Implements Direct Lookahead Approximations (DLAs) evaluating expected outcomes over
//     sampled scenarios from the active belief vector b_t.
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /pkg/math
//   - Imported By: /internal/service
//   - Strict Rule: This package performs zero I/O, is completely offline,
//     and does not observe wall-clock time or global mutable state (Inviolate 0, 4).
//
// Active Invariants:
//   - Inviolate 4 (Closed Business Logic): All decision-making policies must implement typed Go
//     interfaces defined statically at compile time.
//   - Inviolate 5 (State Immutability): Policy evaluation must not alter parent state blocks.
//   - Inviolate 6 (Lock-Free Hot Paths): DLAs and lookahead tree search evaluate branches using
//     goroutines and lock-free channels with context.Context cancellation.
package policy
