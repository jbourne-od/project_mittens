# Project Mittens: High-Performance Logistics POMDP Planner

**Project Mittens** is a high-performance Go implementation and generalization of Dr. Warren Powell's Princeton logistics optimizer. It bridges classical **Sequential Decision Analytics (SDA)** with **Partially Observable Markov Decision Processes (POMDPs)** to model competitive freight markets with endogenous shipper behavior and competitor dynamics.

---

## Key Capabilities

1. **Sequential Decision Analytics (SDA) Foundation**: Preserves the 5 core elements of sequential decision problems (State $S_t$, Decisions $x_t$, Exogenous Information $W_{t+1}$, Transition Function $S^M$, and Objective Function $C(S_t, x_t)$) with zero-allocation flat struct slices.
2. **Endogenous Market Generalization**: Replaces static exogenous shipper assumptions with a Mixed-Observability POMDP $S_t = (R_t, M_t)$ where latent market parameters (competitor capacity densities, shipper price sensitivities) are tracked via Bayesian particle filtering.
3. **Provable Model Degeneracy**: Formulated such that setting competitor count $N = 0$ mathematically degenerates the POMDP belief space back to the classical exogenous Powell model.
4. **Concurrent Online Solver (PO-UCT / POMCP)**: Real-time Monte Carlo Tree Search executed across bounded Go worker pools with hard context deadlines, deterministic per-worker random stream partitioning, and lock-free/read-mostly VFA caches.

---

## Repository Documentation Index

| Document | Purpose |
| :--- | :--- |
| [`AGENTS.md`](file:///Users/jacob/Development/od/project_mittens/AGENTS.md) | Agent workflow rules, engineering philosophy, numerical standards, package boundaries, and verification checklists. |
| [`INVARIANTS.md`](file:///Users/jacob/Development/od/project_mittens/INVARIANTS.md) | Core system constraints and inviolate mathematical, physical, game-theoretic, and concurrency invariants (Invariants 1.1–5.3). |
| [`antigravity_builder_spec.md`](file:///Users/jacob/Development/od/project_mittens/antigravity_builder_spec.md) | Concrete architectural specification, Go data structures, particle filter engine, POMCP solver templates, and 6-phase development backlog. |
| [`feasibility_research_report.md`](file:///Users/jacob/Development/od/project_mittens/feasibility_research_report.md) | In-depth research report detailing the mathematical proof of degeneracy, i-POMDP vs. Unified POMDP analysis, and Go concurrency architecture. |
| [`agent_bibliography.md`](file:///Users/jacob/Development/od/project_mittens/agent_bibliography.md) | Machine-readable YAML metadata and structured reference table grounding all theoretical and engineering decisions. |

---

## Architectural Layout

```
.
├── AGENTS.md                   # Agent development rules and engineering standards
├── INVARIANTS.md               # Core system invariants (Physical, Information, Simplex, Concurrency)
├── README.md                   # Project overview and documentation index
├── agent_bibliography.md       # Machine-readable citations and bibliography
├── antigravity_builder_spec.md # Technical implementation and architecture specification
├── feasibility_research_report.md # Theoretical research and feasibility report
├── go.mod                      # Go module definition (github.com/optimaldynamics/project-mittens)
├── cmd/
│   └── optimizer/              # CLI entry point, seed initialization, simulation harness
│       └── main.go
└── internal/
    ├── domain/                 # Core domain models (State, Decision, Exogenous, Transition, Objective, Belief)
    ├── usecase/
    │   └── solver/             # Parallel POMCP solver, rollout policy, thread-safe VFA cache
    └── infrastructure/
        └── simulator/          # Market environment generative model
```

---

## Invariant Summary

All components must strictly adhere to the invariants defined in [`INVARIANTS.md`](file:///Users/jacob/Development/od/project_mittens/INVARIANTS.md):

*   **1. Physical Resource Conservation**: No multi-location allocation (1.1), strict regulatory Hours of Service (1.2), and physical speed/transit limits (1.3).
*   **2. Load-Capture Exclusivity**: Single-winner auction constraint (2.1) and immediate ghost-freight elimination (2.2).
*   **3. Non-Anticipativity (Information Boundary)**: Decisions condition strictly on observable history $H_t$ and belief $b_t$, never on future unobserved competitor actions (3.1); no out-of-band communication (3.2).
*   **4. Belief Simplex & Mathematical Integrity**: Belief states reside strictly on the probability simplex (4.1); effective sample size $N_{\text{eff}}$ tracked with continuous jittering on particle depletion (4.2); bounded numerical accumulations (4.3).
*   **5. Concurrency, Thread-Safety & Determinism**: Zero race conditions under concurrent reads/writes (5.1); bounded planning latency with safe deterministic fallback on context deadline expiry (5.2); deterministic per-worker PRNG stream partitioning for parallel reproducibility (5.3).

---

## Verification & Testing

Before declaring any feature or bug fix complete, run the verification suite:

```bash
# Format and vet
gofmt -w .
go vet ./...

# Run unit tests
go test ./...

# Run concurrency tests under race detector
go test -race ./...

# Run targeted benchmarks with memory allocations
go test ./... -run '^$' -bench . -benchmem
```
