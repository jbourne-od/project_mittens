# Agent-Readable Bibliography: Project Mittens

This bibliography is optimized for programmatic ingestion, crawling, and semantic parsing by autonomous agents (e.g., `antigravity`). It contains structured metadata blocks (YAML) and easily parsable Markdown tables for quick token extraction and reference linking.

---

## 1. Machine-Readable Metadata (YAML)

```yaml
references:
  - id: powell_sda_book
    title: "Sequential Decision Analytics and Modeling"
    authors: ["Warren B. Powell"]
    category: "SDA"
    role: "Theoretical Foundation for Sequential Decisions"
    canonical_url: "https://castlelab.princeton.edu/sda/"
    github_repo: "https://github.com/wbpowell328/stochastic-optimization"
    relation: "Provides the 5 core elements of the base Java implementation"

  - id: silver_veness_pomcp_2010
    title: "Monte-Carlo Planning in Large POMDPs"
    authors: ["David Silver", "Joel Veness"]
    category: "POMDP_Algorithms"
    role: "Online POMDP Planning Baseline (POMCP)"
    canonical_url: "https://proceedings.neurips.cc/paper/2010/file/53914e56ac2140d749778c13010d0144-Paper.pdf"
    github_repo: "https://github.com/GeorgePik/POMCP"
    relation: "Informs the parallelized online Monte Carlo search tree and rollout loops"

  - id: sarsop_paper
    title: "SARSOP: Efficient Point-Based POMDP Planning by Approximating Optimally Reachable Belief Spaces"
    authors: ["Hanna Kurniawati", "David Hsu", "Wee Sun Lee"]
    category: "POMDP_Algorithms"
    role: "Offline Point-Based Value Iteration Reference"
    canonical_url: "https://www.robotics.org/conferences/rss2008/web/papers/058.pdf"
    github_repo: "https://github.com/AdaCompNUS/sarsop"
    relation: "Value Function Approximation (VFA) reference for backing local step actions"

  - id: go_concurrency_patterns
    title: "Go Concurrency Patterns: Pipelines and cancellation"
    authors: ["Sameer Ajmani", "Rob Pike"]
    category: "Go_Engineering"
    role: "Implementation Guidelines for Goroutines and Channels"
    canonical_url: "https://go.dev/blog/pipelines"
    relation: "Design pattern for buffered channel worker pools and fan-in aggregation"

  - id: go_race_detector
    title: "Data Race Detector"
    authors: ["The Go Authors"]
    category: "Go_Engineering"
    role: "Concurrency Testing and Race Verification"
    canonical_url: "https://go.dev/doc/articles/race_detector"
    relation: "Core validation requirement for thread safety of shared caches"

  - id: go_profiling
    title: "Profiling Go Programs with pprof"
    authors: ["The Go Authors"]
    category: "Go_Engineering"
    role: "Performance Benchmarking and Lock Contention Profiling"
    canonical_url: "https://go.dev/blog/pprof"
    github_repo: "https://github.com/google/pprof"
    relation: "Tools for diagnosing memory allocation bottlenecks and mutex contention"

  - id: julia_pomdps_jl
    title: "POMDPs.jl: A Framework for Sequential Decision Making under Uncertainty"
    authors: ["JuliaPOMDP Team"]
    category: "POMDP_Frameworks"
    role: "Modular POMDP API Reference"
    canonical_url: "https://juliapomdp.github.io/POMDPs.jl/latest/"
    github_repo: "https://github.com/JuliaPOMDP/POMDPs.jl"
    relation: "API structural blueprint for defining clean interfaces in Go"

  - id: hyp_despot_parallel
    title: "HyP-DESPOT: A Hybrid Parallel Algorithm for Online Planning under Uncertainty"
    authors: ["Nan Ye", "Adhiraj Somani", "David Hsu", "Wee Sun Lee"]
    category: "Parallel_POMDP"
    role: "Parallel Search and Rollout Scaling Reference"
    canonical_url: "https://arxiv.org/abs/1709.06649"
    relation: "Illustrates multi-threaded CPU/GPU optimization for online decision tree scaling"

  - id: momdp_shlomo
    title: "Mixed Observability MDPs for Shared Autonomy with Uncertain Human Behaviour"
    authors: ["Sylvie C.W. Ong", "Shao Wei Png", "David Hsu", "Wee Sun Lee"]
    category: "POMDP_Theory"
    role: "Theoretical Observable/Unobservable State Factorization"
    canonical_url: "https://arxiv.org/abs/1109.2145"
    relation: "Mathematical basis for splitting observable truck state from latent market state"
```

---

## 2. Structured Reference Table

| CiteKey | Title | Category | Role | Canonical URL | GitHub / Resource |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `@powell_sda_book` | Sequential Decision Analytics and Modeling | SDA | Theoretical Foundation | [Link](https://castlelab.princeton.edu/sda/) | [wbpowell328/stochastic-optimization](https://github.com/wbpowell328/stochastic-optimization) |
| `@silver_veness_pomcp_2010` | Monte-Carlo Planning in Large POMDPs | POMDP Algorithms | Online Solver Strategy | [NIPS Paper](https://proceedings.neurips.cc/paper/2010/file/53914e56ac2140d749778c13010d0144-Paper.pdf) | [GeorgePik/POMCP](https://github.com/GeorgePik/POMCP) |
| `@sarsop_paper` | SARSOP: Point-Based POMDP Planning | POMDP Algorithms | Offline Approximations | [Paper](https://www.robotics.org/conferences/rss2008/web/papers/058.pdf) | [AdaCompNUS/sarsop](https://github.com/AdaCompNUS/sarsop) |
| `@go_concurrency_patterns` | Go Concurrency Patterns: Pipelines | Go Engineering | Parallelization Patterns | [Go Pipelines Blog](https://go.dev/blog/pipelines) | [sync.Mutex Wiki](https://go.dev/wiki/MutexOrChannel) |
| `@go_race_detector` | Data Race Detector | Go Engineering | Thread Safety Invariants | [Go Article](https://go.dev/doc/articles/race_detector) | N/A |
| `@go_profiling` | Profiling Go Programs with pprof | Go Engineering | Benchmarking & Contention | [Go pprof Guide](https://go.dev/blog/pprof) | [pkg.go.dev/net/http/pprof](https://pkg.go.dev/net/http/pprof) |
| `@julia_pomdps_jl` | POMDPs.jl Framework | Frameworks | Struct/API Modeling | [Docs](https://juliapomdp.github.io/POMDPs.jl/latest/) | [JuliaPOMDP/POMDPs.jl](https://github.com/JuliaPOMDP/POMDPs.jl) |
| `@hyp_despot_parallel` | HyP-DESPOT Parallel Planning | POMDP Algorithms | Parallel Tree Search | [ArXiv Paper](https://arxiv.org/abs/1709.06649) | N/A |
| `@momdp_shlomo` | Mixed Observability MDPs | POMDP Theory | State Factorization | [ArXiv Paper](https://arxiv.org/abs/1109.2145) | N/A |

---

## 3. Agent Execution Guide for Retrieving and Ingesting Resources

This section provides direct parsing rules for the agent (`antigravity`) when processing these references:

1.  **SDA & Stochastic Optimization (`@powell_sda_book`)**:
    *   **Goal**: Extract the mathematical structure of the post-decision state $S_t^x$ and transition model $S^M$.
    *   **Target Codebase**: Parse the Princeton CASTLE Lab repository to identify how they structured multidimensional driver and load resource arrays in Java. Replicate this memory footprint in Go using flat, contiguous slices of structs to minimize pointer indirection.
2.  **POMCP Parallelization (`@silver_veness_pomcp_2010` & `@go_concurrency_patterns`)**:
    *   **Goal**: Implement the PO-UCT algorithm with low locks and bounded worker pools.
    *   **Target Method**: Integrate the Monte Carlo tree rollout logic from `@silver_veness_pomcp_2010` with the concurrent worker pipeline patterns in `@go_concurrency_patterns`. Dispatch rollout simulations across bounded worker goroutines, using deterministic per-worker PRNG streams to trace independent paths through the state-observation tree.
3.  **Belief Engine and Resampling (`@julia_pomdps_jl` & `@momdp_shlomo`)**:
    *   **Goal**: Maintain the split state space $S_t = (R_t, M_t)$ where physical fleet coordinates $R_t$ are fully observable and market variables $M_t$ are evaluated as a distribution of particles.
    *   **Target Method**: Emulate the modular struct interfaces used by `POMDPs.jl` in Go to separate the transition logic from the tracker/updater logic. Utilize systematic resampling with jittering when effective particle count $N_{\text{eff}} < \kappa$ to prevent particle depletion (Invariant 4.2).
4.  **Race Detection and Profiling (`@go_race_detector` & `@go_profiling`)**:
    *   **Goal**: Enforce zero lock contention and zero data races under extreme concurrent loads (Invariant 5.1).
    *   **Execution Rule**: Before compiling production binaries, execute the automated verification test sequence with the `-race` flag enabled to isolate unsafe pointer sharing between simulation threads.
