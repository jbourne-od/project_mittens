# Invariants: Core System Constraints

In high-performance concurrent and agentic coding, **invariants** represent system constraints that must mathematically hold true at every single state, transition, and evaluation boundary. Under no circumstances may our solver, simulator, or execution agents violate these rules.

---

## 1. Physical Resource Conservation (Mass Conservation)

A physical truck and driver represent finite, discrete, localized assets.

*   **Invariant 1.1: No Multi-Location Allocation**: A single driver/truck asset $r \in R_t$ cannot be assigned to multiple concurrent spatial tasks, routes, or conflicting loads simultaneously.
*   **Invariant 1.2: Flow Conservation and Regulatory Compliance**:
    *   *Spatial Flow*: For any node $i$ in the logistics network, the number of trucks departing $i$ at time $t$ cannot exceed the current local fleet inventory $R_{t,i}$ plus the incoming trucks arriving at that exact interval.
    *   *Hours of Service (HOS)*: Dispatch actions must strictly comply with physical driver limits (e.g., 11-hour daily driving limit, 14-hour on-duty window, and mandatory 10-hour rest cycles). Infeasible assignments must be pruned prior to evaluation.
*   **Invariant 1.3: Temporal Realism**: The transition duration $\Delta t = t' - t$ required for a truck to move between nodes $i$ and $j$ must strictly respect physical spatial distance and maximum legal speed bounds. No lookahead scenario or simulation branch may violate travel-time physics.

---

## 2. Load-Capture Exclusivity (Game-Theoretic Invariants)

Every freight load is a discrete, single-unit resource.

*   **Invariant 2.1: Single-Winner Auction**: For any freight load $l \in L_t$, the joint action capture indicator across all market participants must obey:
    $$\sum_{i=0}^{N} \mathbb{I}(\text{Carrier } i \text{ captures } l) \le 1$$
    where $i=0$ represents our fleet and $i \ge 1$ represent competing carriers.
*   **Invariant 2.2: No Ghost Freight**: A load that has been successfully captured by any player $i$ is immediately removed from the active spot board for all other market participants and can never be re-awarded or processed as available in future lookahead branches.

---

## 3. Non-Anticipativity of Hidden States (Information Boundary)

Information state transitions must strictly respect temporal causality and observation boundaries.

*   **Invariant 3.1: The Opaque Curtain (Non-Anticipativity)**: When simulating future lookahead branches, the solver may sample hidden competitor states $s_h \sim b_h(s_h)$ to evaluate expectations. However, the policy $\pi_0$ cannot make decisions at decision epoch $t$ that condition on future private competitor actions or latent states $s_{h, t+k}$ prior to them being revealed through observations:
    $$\pi_t(a_t \mid H_t, s_{h, t+k}) = \pi_t(a_t \mid H_t) \quad \forall k \ge 1$$
    Conditioning decisions on future unobserved states is strictly forbidden. All decisions must be measurable only with respect to the observable history $H_t$ and the current belief state $b_t$.
*   **Invariant 3.2: No Out-Of-Band Inter-Agent Communication**: In a decentralized game-theoretic POMDP (POSG), agents act individually using only localized or public market information. No private communication channels or state-peeking are permitted between our planning agents and simulated competitors.

---

## 4. Belief Simplex and Mathematical Integrity

All probability and value structures must remain mathematically sound across all numerical operations.

*   **Invariant 4.1: Simplex Preservation**: Any belief state $b_t$ over latent market states $M_t$ must reside strictly on the probability simplex:
    $$\sum_{m \in M} b_t(m) = 1 \quad \text{and} \quad b_t(m) \ge 0, \;\; \forall m \in M$$
    Normalizations must be explicit, and underflow conditions must be handled without producing NaN or Inf values.
*   **Invariant 4.2: Particle Diversity and Depletion Avoidance**: In particle filter representations of the belief state, the effective sample size $N_{\text{eff}} = \frac{1}{\sum_{i=1}^N w_i^2}$ must be tracked after every Bayesian observation update. When $N_{\text{eff}} < \kappa$ (where $\kappa$ is the depletion threshold, typically $0.4 N$), systematic resampling combined with continuous jittering (variance reinvigoration) must be triggered to preserve particle diversity.
*   **Invariant 4.3: Numerical Value Bounding**: During dynamic programming and value backups, value approximations $\mathcal{V}(b)$ and log-space accumulations must remain strictly finite and bounded within domain reward extremes, preventing arithmetic underflow or overflow.

---

## 5. Concurrency, Thread-Safety, and Determinism Invariants

High-throughput parallel tree search and rollout loops must preserve exact numerical and operational semantics.

*   **Invariant 5.1: Zero Race Conditions on Shared Structures**: All shared lookup tables, Value Function Approximation (VFA) caches, and search tree nodes accessed across concurrent goroutines must either be protected by dedicated synchronization primitives (e.g., `sync.RWMutex`, atomic operations) or be strictly partitioned by worker. All concurrent paths must pass tests with Go's `-race` detector enabled.
*   **Invariant 5.2: Bounded Planning Latency and Safe Fallbacks**: Online planning must strictly obey real-time deadline budgets passed via Go's `context.Context`. If the compute budget expires before tree search finishes:
    1. All worker goroutines must terminate cleanly without leaks or orphan background operations.
    2. The solver must immediately return a safe, deterministic, myopic fallback decision rather than failing or blocking.
*   **Invariant 5.3: Parallel Reproducibility via Partitioned Random Streams**: Parallel execution must never share mutable PRNG state across goroutines. Random number generators must be deterministically partitioned (e.g., derived from a root seed and worker/simulation index) so that running with identical seeds reproduces identical planning decisions regardless of `GOMAXPROCS` or OS thread scheduling.
