// Package service coordinates the optimization workflow, state transitions,
// recursive Bayesian belief filtering, and execution simulation harnesses.
//
// Ownership & Responsibility:
//   - Orchestrates the sequential decision cycle: State -> Policy Evaluation -> Decision
//     -> Exogenous/Endogenous Observation -> Bayesian Belief Filter -> State Transition.
//   - Drives recursive Bayesian updates b_{t+1} = Filter(b_t, a_t, W_{t+1}) on the belief simplex.
//   - Enforces execution boundaries, provenance recording, and distributed tracing spans.
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /internal/domain/policy, /pkg/math
//   - Imported By: /cmd/optimizer, /internal/adapter
//   - Strict Rule: Coordinates domain components without polluting mathematical models with
//     transport or persistence concerns.
//
// Active Invariants:
//   - Inviolate 1 (Monopolistic Degeneracy): When N=0, belief filter collapses to Dirac delta
//     and transitions match legacy Powell baseline with zero numerical drift.
//   - Inviolate 7 (Provenance): Orchestrator ensures every dispatch action generates a complete
//     provenance record (Semantic Journal).
//   - Inviolate 8 (Fail-Closed Robustness): Invariant failures in simplex normalization or physical
//     flow conservation panic immediately.
package service
