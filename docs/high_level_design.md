# Project Mittens High-Level Design

**Status:** Approved design for the current documentation phase  
**Date:** 2026-08-19  
**Audience:** Platform engineering, operations research, optimization engineering, and technical leadership  
**Highest repository authority:** [Ratified Project Mittens Inviolates](mittens-inviolates.md)  

---

## 1. Executive Summary

Project Mittens is the next-generation sequential decision and optimization engine for Optimal Dynamics, written in Go. Its core purpose is to replace the legacy Java-based optimizer with a modern, high-concurrency, lock-free execution environment while simultaneously correcting its biggest structural limitation: the assumption of static, purely exogenous customer demand ("loads") [1, 2].

Traditionally, incoming load arrivals $W_{t+1}$ are treated as independent, stationary exogenous stochastic processes, completely decoupled from the carrier's asset allocation and pricing decisions. In realistic competitive logistics corridors, however, the probability of receiving load offers and winning spot rate bids is highly endogenous—depending dynamically on competitor lane concentration, shipper booking preferences, and pricing postures [11, 12].

Project Mittens extends Professor Warren B. Powell's canonical five-element sequential decision analytics (SDA) framework by formulating a competitive **Partially Observable Markov Decision Process (POMDP)** model [1, 3]. By collapsing the unobservable actions of $N_c$ competitors into a single, aggregate latent market state $\Theta_t$, we preserve mathematical tractability and avoid the nested belief hierarchies that render Interactive POMDPs (i-POMDPs) computationally intractable [3, 6]. We factor this model into a **Mixed-Observability MDP (MOMDP)**, keeping physical vehicle allocation states $R_t$ fully observable and deterministic, while restricting stochastic Bayesian filtering strictly to the lower-dimensional latent competitive belief $b_t$ [5, 6].

Crucially, the Mittens engine is guided by three non-negotiable architectural requirements:
1. **Monopolistic Degeneracy ($N_c = 0$):** When the competitor count is set to zero, the latent state space collapses to a singleton, and the transition and observation dynamics decouple, exactly recovering the legacy monopolistic Powell framework [1, 11].
2. **Legacy Parity:** The Go-based implementation must achieve complete functional parity and numerical equivalence with the legacy Java optimizer in the $N_c = 0$ degenerate case, serving as an uncompromised baseline for physical fleet transitions [1, 2].
3. **Competitive Genericity ($N_c \ge 0$):** Core interfaces for state representations, observation channels, and lookahead expectation operators must utilize generic type structures parameterizable by competitor dimension $N_c$, protecting the codebase from rewrites if data eventually permits scaling to multi-agent $N_c > 1$ scenarios [6, 10].

---

## 2. Problem Statement

The legacy Java-based optimization engine has driven massive operational value, managing real-world truckload matching and driver dispatches across thousands of active assets [11, 12]. However, as carriers operate in highly volatile, spot-market-dominated shipping networks, several structural and software engineering limitations have emerged:

1. **The Monopolistic Demand Fallacy:** Real-world load offers are endogenous. Shippers select carriers based on bid price $p_t$ relative to competitor prices [13]. By treating load arrivals as purely exogenous, the legacy engine cannot capture competitive feedback loops (e.g., aggressively dispatching trucks to a specific lane might stochastically displace competitor capacity, shifting lane concentration in the next epoch) [11].
2. **Computational Overhead of the JVM:** The legacy optimizer experiences high memory footprints, garbage collection pauses, and multi-threaded synchronization overhead during deep lookahead planning (DLAs) [1]. Lookahead tree search (e.g., UCT or sparse tree search) requires massive parallel branch evaluation, where Java's heavy OS thread-mapping model limits search depth and throughput [1, 3, 5].
3. **The i-POMDP / Dec-POMDP Paradox:** Traditional multi-agent formulations, such as Interactive POMDPs, require tracking infinite-dimensional hierarchies of beliefs (beliefs about competitor beliefs about our beliefs) [6, 10]. From an engineering perspective, this is impossible to deploy: carriers do not have access to competitors' private fleet distributions, pricing utility functions, or scheduled loads [11]. We require a model that captures competitive dynamics using only observable transaction data (e.g., bid win/loss feedback and historical booking rates) [11, 12].

---

## 3. Core HLD Goals

Project Mittens will establish and enforce the following properties:

*   **Exogenous Generalization:** Generalize the exogenous load arrival process by dynamically coupling load availability and bidding probabilities to the unobserved aggregate competitor state $\Theta_t$ [11, 13].
*   **Legacy Parity and Monopolistic Collapse:** Ensure that setting $N_c = 0$ mathematically collapses the POMDP state space, recursive belief filtering, and transition logic back to the classical fully observable Powell framework [1, 2].
*   **MOMDP Structural Decoupling:** Prevent the curse of dimensionality by factoring states into observable, deterministic physical resources $R_t$ and filtered competitive beliefs $b_t$, isolating the physical fleet transitions from stochastic filtering [5, 6].
*   **Competitive-Dimension Genericity:** Leverage Go generics to define compile-time parameterizable interfaces for any competitor scale $N_c \ge 0$, maintaining a single, unified mathematical core [8, 10].
*   **High-Throughput Go Concurrency:** Capitalize on Go's lightweight GMP scheduler and lock-free channels to execute parallel lookahead tree search and SPSA parameter optimization loops [3, 8].
*   **Strict State Immutability:** Enforce immutable state blocks to guarantee that concurrent exploratory branch evaluations during direct lookaheads cannot corrupt shared memory or introduce data races [3, 5].
*   **Auditable Provenance Logging:** Generate comprehensive execution journals recording all state inputs, parameter vectors $\theta$, active beliefs $b_t$, and marginal matching valuations for complete operational auditability [7, 8].

---

## 4. Non-Goals

This high-level design phase does not commit to, or cover, the following:
*   A final database persistence schema (e.g., PostgreSQL, DynamoDB) or cold-storage layout.
*   A specific external network protocol (e.g., gRPC, REST, gRPC-Web) for carrier-to-shipper communication.
*   An active multi-agent ($N_c > 1$) simulation or decentralized game-theoretic solver implementation.
*   Real-time automated pricing feedback or dynamic rate-setting rules in the core dispatch loop.
*   Automated translation of legacy Java optimization code to Go.

---

## 5. Architectural Decisions

### 5.1 Mixed-Observability Factoring (MOMDP)
To prevent exponential state-space explosion, Project Mittens strictly factors the state variable:
*   **Resource State ($R_t$):** Physical assets (trucks, driver locations, scheduled loads) are 100% observable and transition deterministically given dispatch action $x_t$ and realized loads $D_{t+1}$ [11].
*   **Belief State ($b_t$):** The stochastic probability distribution over the latent competitive market state $\Theta_t$ [3, 11].

```
                     ┌────────────────────────────────────────┐
                     │          Joint Action at = (xt, pt)    │
                     └─────────────┬────────────────────┬─────┘
                                   │                    │
                                   ▼                    ▼
   ┌───────────────────────────────────┐    ┌──────────────────────────────────┐
   │ Deterministic Fleet Transition    │    │  Bayesian Belief Filter          │
   │ Rt+1 = fR(Rt, xt, Dt+1)           │    │  bt+1 = Filter(bt, at, Wt+1)     │
   └───────────────────────────────────┘    └──────────────────────────────────┘
```

This MOMDP factoring ensures that the physical network transition logic remains identical to the legacy framework, isolating the POMDP stochastic filtering strictly to the lower-dimensional competitive subspace $\mathcal{H}$ [5, 6].

### 5.2 Competitor Collapsing
We bypass the nested-belief bottleneck of i-POMDP models by collapsing the $N_c$ competitors into a single latent market state $\Theta_t \in \mathcal{H}$ via an aggregation mapping:
$$ \Theta_t = g\left(R^1_t, a^1_t, R^2_t, a^2_t, \dots, R^{N_c}_t, a^{N_c}_t\right) $$
This collapses the multi-agent game-theoretic complexity into a single-agent partially observable environment, converting the competitive POMDP into a fully observable belief-state MDP over the expanded state variable $S^{ext}_t = (R_t, I_t, b_t)$ [1, 3, 6].

### 5.3 Immutable State Transitions
To scale concurrent tree-search planning on multi-core processors, all domain state variables are strictly immutable. Any action transition or Bayesian filtering update allocates and returns a new state pointer:
$$ (S^{ext}_{t+1}, \Delta_t) = S^M(S^{ext}_t, a_t, W_{t+1}) $$
This allows parallel goroutines evaluating distinct lookahead branches in Direct Lookahead Approximations (DLAs) to run concurrently without shared-memory mutexes, preventing data races and synchronization bottlenecks on the hot execution paths [3, 8].

---

## 6. Mathematical State and Transition Model

The mathematical core of Project Mittens defines the state, actions, transition dynamics, and observation loops under MOMDP factoring.

### 6.1 State Factoring
The complete state of the system is factored as:
$$ s_t = (R_t, \Theta_t) \in \mathcal{S} \times \mathcal{H} $$
Where:
*   $R_t \in \mathcal{S}$ is the fully observable resource state representing driver availability, hours-of-service, and current load assignments [11].
*   $\Theta_t \in \mathcal{H}$ is the latent competitor lane concentration and pricing posture across the logistics network [11, 13].

Since $\Theta_t$ is hidden, the carrier maintains and plans over the expanded belief-state MDP state variable:
$$ S^{ext}_t = (R_t, I_t, b_t) $$
Where $b_t \in \Delta(\mathcal{H})$ is the competitive belief vector [3, 11].

### 6.2 Decision Joint Action
At each decision epoch, the optimizer selects a joint action:
$$ a_t = (x_t, p_t) \in \mathcal{X}(R_t) \times \mathcal{P} $$
Where:
*   $x_t$ is the physical matching decision matching drivers to realized load offers [11].
*   $p_t$ is the spot bidding price vector submitted to the market [11, 13].

### 6.3 Mixed Transition Dynamics
1. **Deterministic Resource Transition:**
$$ R_{t+1} = f_R(R_t, x_t, D_{t+1}) $$
Where $D_{t+1}$ represents realized customer load offers [11].
2. **Stochastic Latent State Transition:**
$$ T_\Gamma(\Theta_{t+1} \mid \Theta_t, a_t) = P(\Theta_{t+1} \mid \Theta_t, x_t, p_t) $$
Models how the carrier's previous decisions and pricing stochastically influence future competitor spatial density [11, 13].
3. **Generalized Exogenous Load Observation Model:**
Rather than pure, independent exogenous information, the system receives an observation representing load offers and bidding outcomes:
$$ o_{t+1} = W_{t+1} = (D_{t+1}, Y_{t+1}) $$
$$ Z(o_{t+1} \mid s_{t+1}, a_t) = P(D_{t+1}, Y_{t+1} \mid R_{t+1}, \Theta_{t+1}, x_t, p_t) $$
Where $Y_{t+1}$ is the win/loss signal for submitted bids. The arrival of new customer load offers $D_{t+1}$ is dynamically coupled to the competitor concentration $\Theta_{t+1}$ [11, 13].

### 6.4 Parity Degeneracy ($N_c = 0$)
When $N_c = 0$, there are no competitors. The latent state space collapses to a singleton representing competitor absence:
$$ \mathcal{H} = \{\Theta_\emptyset\} $$
The competitive transition probability becomes deterministic:
$$ T_\Gamma(\Theta_\emptyset \mid \Theta_\emptyset, a_t) = 1, \quad \forall a_t \in \mathcal{A} $$
The carrier's belief state collapses to a static Dirac delta distribution centered at $\Theta_\emptyset$:
$$ b_t(\Theta) = \delta(\Theta - \Theta_\emptyset) = \begin{cases} 1 & \text{if } \Theta = \Theta_\emptyset \\ 0 & \text{otherwise} \end{cases} $$
The observation model simplifies, decoupling load arrivals from competitor dynamics and collapsing the bidding feedback variable $Y_{t+1}$:
$$ Z(o_{t+1} \mid s_{t+1}, a_t) = P(D_{t+1} \mid R_{t+1}, \Theta_\emptyset, x_t, p_t) = P(D_{t+1} \mid I_{t+1}) = P(W_{t+1} \mid S_t) $$
The expanded state variable collapses:
$$ S^{ext}_t = (R_t, I_t, b_t = \delta(\Theta_t - \Theta_\emptyset)) \equiv (R_t, I_t) = S_t $$
This mathematically and numerically recovers the classical, fully observable monopolistic Powell framework, satisfying the degenerate equivalence constraint [1, 2].

---

## 7. Package Architecture

Project Mittens enforces strict clean-architecture boundaries. Dependencies point inward toward deterministic domain packages, preventing import cycles and isolating mathematical optimization from I/O and operational infrastructure [8, 10]:

```
mittens/
├── cmd/
│   └── optimizer/                # Single static binary entry point (dependency injection only)
│       └── main.go
├── internal/
│   ├── domain/
│   │   ├── model/                # Core domain types: State, Action, Observation, Belief (No I/O)
│   │   │   ├── state.go
│   │   │   ├── action.go
│   │   │   └── belief.go
│   │   └── policy/               # Closed policy classes: PFA, CFA, VFA, DLA (Math-only)
│   │       ├── pfa.go
│   │       ├── cfa.go
│   │       ├── vfa.go
│   │       └── dla.go
│   ├── service/                  # Coordination layers, filtering loops, simulation runs
│   │   ├── engine.go
│   │   └── filter.go
│   └── adapter/                  # External adapters and legacy Java porting adapters
│       ├── legacy/               # High-fidelity legacy Java compatibility port (N_c = 0)
│       ├── eld/                  # ELD integration
│       └── db/                   # Persistent storage adapters
└── pkg/
    └── math/                     # Shared math: linear algebra, SPSA optimizer, solvers
```

### Architectural Package Rules:
1. **Domain Isolation:** Package `domain` contains the pure mathematical model of the sequential decision process. It is completely isolated and cannot perform any I/O, observe wall-clock time, access environment variables, or import external frameworks [1, 8].
2. **Dependencies Point Inward:** Packages in `adapter` and `service` may import `domain`. No package in `domain` may import `service` or `adapter` [8].
3. **Generics Implementation:** The model package utilizes Go generics to parameterize competitor count $N$, allowing arbitrary competitive scales to be checked at compile time [8, 10]:
```go
package model

// Generic Belief representation supporting N competitor factors
type Belief[N any] struct {
    Distribution map[string]float64
    Metadata     N
}
```

---

## 8. Legacy Compatibility and Verification Path

To verify Java-to-Go parity and mathematical correctness, Project Mittens defines a dual-run shadow execution and verification pathway:

1. **High-Fidelity Serialization Bridge:** The `legacy` adapter package parses historical, serialized Java state structures and maps them into Go model types.
2. **The Degeneracy Parity Check:** Under the test configuration $N_c = 0$, the Go engine is executed against historical driver-to-load dispatch runs. The matching output, allocated driver routes, and expected contributions must match the legacy Java engine's outputs, bounded only by standard JVM-to-Go floating-point precision differences [1, 11].
3. **Fail-Closed Verification Invariants:** Active runtime assertions check that the belief vector satisfies the probability simplex constraint:
$$ \sum_{\Theta \in \mathcal{H}} b_t(\Theta) = 1.0 \pm 10^{-7} $$
Any particle depletion or numerical instability outside this boundary triggers an immediate panic, halting operations to prevent corrupt dispatches [3, 8].

---

## 9. Current Deliverable Boundary

This high-level design phase produces the following deliverables:
*   This normative High-Level Design (HLD) document defining the mathematical, architectural, and concurrent foundations of Project Mittens [1, 8].
*   The Project Constitution (`mittens-inviolates.md`) establishing the non-negotiable legal boundaries for development [8].
*   Draft Go interface specifications for states, actions, policies, and belief filters.
*   A test specification suite for validating monopolistic $N_c = 0$ degeneracy.
*   No database schemas, API wire protocols, or production deployment configurations are committed in this phase.
