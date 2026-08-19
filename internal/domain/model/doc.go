// Package model defines the core domain types and mathematical state representations
// for Project Mittens' Mixed-Observability Markov Decision Process (MOMDP).
//
// Ownership & Responsibility:
//   - Formulates the factored state variables: fully observable resource state (R_t),
//     information state (I_t), and the stochastic belief state (b_t).
//   - Formulates the joint decision action a_t = (x_t, p_t) and endogenous observation
//     emission W_{t+1} = (D_{t+1}, Y_{t+1}).
//   - Provides competitive-dimension generic interfaces parameterized by N >= 0.
//
// Dependency Boundaries:
//   - Imports: Standard library, and optionally /pkg/math.
//   - Imported By: /internal/domain/policy, /internal/service, /internal/adapter.
//   - Strict Rule: Zero I/O, zero network calls, zero SQL or database dependencies,
//     zero ambient environment state (Inviolate 0).
//
// Active Invariants:
//   - Inviolate 2 (MOMDP Factoring): Physical resource state R_t transitions deterministically
//     and is separated from stochastic belief filtering b_t.
//   - Inviolate 3 (Competitive Genericity): State and belief representations must not
//     hardcode N=1 and must be parameterizable for arbitrary competitor counts N >= 0.
//   - Inviolate 5 (State Immutability): All state structures are strictly immutable once
//     created. Transitions return freshly allocated pointers. State structures contain
//     zero sync.Mutex or sync.RWMutex fields.
//   - Inviolate 8 (Fail-Closed Verification): Belief states must satisfy simplex mass
//     conservation (\sum b_t(\Theta) = 1.0 \pm \epsilon).
package model
