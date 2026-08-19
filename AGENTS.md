# Project Mittens AGENTS.md

**Status:** Ratified; authoritative guide for AI and human contributors to Project Mittens
**Ratified:** 2026-08-19
**Purpose:** Enforce mathematical, concurrent, and stylistic rigor during the Java-to-Go Optimizer Migration

---

## 1. Purpose and Engineering Philosophy
Project Mittens is a high-performance, concurrent, and mathematically rigorous Go-based optimization engine. Its primary engineering objective is to replace the legacy Java-based carrier-matching optimizer with a modern Go implementation while correcting its most significant limitation: modeling customer load arrivals as an endogenous, partially observable market process.

We govern our execution using five core principles:
1. **Absolute Mathematical Rigor:** Invariants, transitions, and probability matrices must be treated as exact, verifiable laws.
2. **Deterministic Reproducibility:** Given the same initial state, configuration, and random seed, the optimizer must yield bit-wise identical decisions and lookup states.
3. **High-Efficiency Concurrency:** Capitalize on Go's GMP scheduler using lock-free, value-based state transitions rather than heavy, blocking shared-memory synchronization.
4. **Complete Auditability (Provenance):** Every single driver-load match and pricing decision must be generated with a comprehensive mathematical trail of evidence.
5. **Fail-Closed Robustness:** System anomalies, belief-state simplex violations, and data corruption must immediately panic and halt the run rather than allowing invalid physical dispatches.

---

## 2. Authority and Decision Hierarchy
All human developers, subagents, and automated code-generation tools must adhere to the following hierarchy when resolve ambiguities or conflicting requirements:

1. **[Project Mittens Inviolates](mittens-inviolates.md):** The absolute, non-negotiable laws of the repository. No pull request violating an Inviolate may be merged.
2. **[Mittens High-Level Design](mittens-hld.md):** The authoritative architectural, mathematical, and package layout specification.
3. **Explicit API Contracts & Authoritative Tests:** Typed Go interface definitions, JSON/protobuf schemas, and verification suites.
4. **This AGENTS.md Guide:** Rules governing developer style, documentation patterns, multi-agent collaboration, and observability discipline.
5. **Current Task Requirements:** The specific user story, bug ticket, or feature request currently being implemented.
6. **Existing Codebase Patterns:** Historical conventions and structures already merged into `/internal` and `/pkg`.

*If an implementation task exposes a fundamental flaw, ambiguity, or contradiction in the HLD or Inviolates, the developer must explicitly flag the conflict and seek a ratified amendment to the higher-level document before writing code.*

---

## 3. The Go-Python Developer Bridge
Project Mittens operates as a highly collaborative, cross-functional repository. Because many domain experts, simulation specialists, and reviewers on the team are more familiar with Python (and the legacy Java codebase) than Go, we enforce an elevated standard of implementation documentation to bridge this knowledge gap.

### 3.1 Package-Level Declarations
Every nontrivial package in the repository must contain a package-level doc comment (normally in a `doc.go` file or the primary entrypoint file) detailing:
*   **Ownership & Responsibility:** What core mathematical or business domain this package represents.
*   **Dependency Boundaries:** Which packages this package is permitted to import, and which packages are permitted to import it.
*   **Active Invariants:** The state boundaries or mathematical limits maintained by the package.

Example Package Header:
```go
// Package policy implements the four universal classes of Powell policies
// (PFAs, CFAs, VFAs, and DLAs) over the belief-state MDP.
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /pkg/math
//   - Imported By: /internal/service
//   - Strict Rule: This package performs zero I/O, is completely offline,
//     and does not observe wall-clock time or global mutable state.
package policy
```

### 3.2 Exported Declaration Documentation
Every exported type, struct field, interface, method, function, and constant must be fully documented using standard Go doc comments. Explain *what* the entity does, *why* it exists, and any pre- or post-conditions. Avoid redundant narration of syntax (e.g., do not write `// NewOptimizer returns a new optimizer`—explain what parameters it configures and what invariants it initializes).

### 3.3 Near-Code Operational Context
You must document non-obvious engineering decisions directly inline, near the logic implementing them. This applies specifically to:
*   **Determinism & Ordering:** Document why a specific slice is sorted or how a map's keys are canonicalized before iteration to guarantee deterministic execution.
*   **Lifecycle & State Transitions:** Document the allocation boundaries and post-decision state mappings.
*   **Concurrency & Goroutine Boundaries:** Detail why a channel is unbuffered or how a coordinate-select multiplexer operates safely on the hot path.
*   **Fail-Closed & Retry Logic:** Explain the mathematical bounds of a panic block or the convergence criteria of SPSA pseudo-gradients.

### 3.4 Go Idioms for Python-Oriented Readers
Go's safety, performance, and concurrency behaviors differ fundamentally from Python's interpreter model. You must write explicit inline warnings and explanations when implementing the following idiomatic Go patterns:

#### State Immutability via Value-Based Allocation
Unlike Python, where objects are mutable references by default, Project Mittens enforces strict state immutability to prevent concurrent data races during lookahead tree branch evaluations.
*   **Go Idiom:** Methods that return updated resource states $R_{t+1}$ or updated belief vectors $b_{t+1}$ must return a *newly allocated pointer* to a copied value, leaving the prior state unaltered.
*   **Documentation Requirement:** Place an explicit warning explaining that mutations of the inner fields of the returned struct are compiler-impossible due to the pointer reallocation.

#### Slice and Map Boundary Copying
Passing slices and maps in Go passes references to the underlying backing arrays. If a policy mutates a slice passed from the service layer, it will corrupt the parent state.
*   **Go Idiom:** Deep copy slices and maps at package boundaries using the built-in `copy()` function or explicit iteration.
*   **Documentation Requirement:** Document the copy loop explicitly so a Python developer does not remove it as "redundant allocation."

```go
// Deep-copy the active fleet slice to prevent downstream lane-matching
// policies from corrupting the parent service's resource state.
// In Go, slices are reference headers; copying the slice header does not
// copy the backing array.
copiedDrivers := make([]Driver, len(state.Drivers))
copy(copiedDrivers, state.Drivers)
```

#### Goroutine Lifecycle & context.Context Cancellation
In Go, goroutines are not managed by an OS thread pool or a Python event loop; they are scheduled via Go's GMP runtime. If a goroutine blocks on an unbuffered channel write or an infinite loop, it creates a permanent memory leak (zombie goroutine).
*   **Go Idiom:** All concurrent lookahead trees (DLAs) and Monte Carlo sampling paths must accept a `context.Context` and actively select on `ctx.Done()`.
*   **Documentation Requirement:** Explicitly explain the select-multiplexing pattern to ensure reviewers understand how timeouts are safely propagated across the parallel simulation threads.

---

## 4. Package Architecture & Clean Boundaries
To maintain decoupling and protect our mathematical domain from outer-infrastructure leaks (database drivers, network clients, cloud SDKs), Project Mittens strictly enforces **Clean Architecture** boundaries:

```
                  /-----------------------------                  |           /adapter          |
                  |  (I/O, DB, Kafka, Eld API)  |
                  \--------------|--------------/
                                 |
                  /--------------v--------------                  |          /service           |
                  |     (Orchestrates Flow)     |
                  \--------------|--------------/
                                 |
         /-----------------------+-----------------------         |                                               |
/--------v--------\                               /------v--------|     /model      | <---------------------------- |    /policy    |
| (MOMDP States,  |                               |  (PFAs, CFAs, |
| Belief Simplex) |                               |  VFAs, DLAs)  |
\-----------------/                               \-------|-------/
         |                                                |
         \-----------------------+------------------------/
                                 |
                  /--------------v--------------                  |          /pkg/math          |
                  |  (SPSA, Correlated KG, LA)  |
                  \-----------------------------/
```

### 4.1 Permitted Dependency Mappings
1.  **`/internal/domain/model` (The Domain Core):** Factorings for MOMDP states ($R_t$, $I_t$, $b_t$). This package is the highest logical center. It **must not import any other package** within Project Mittens (except `/pkg/math`). It is entirely pure, containing zero SQL, zero network calls, and zero external framework definitions.
2.  **`/internal/domain/policy` (The Policy Layer):** Powell policy formulations. Permitted to import `/internal/domain/model` and `/pkg/math`. It **must not import `/internal/service` or `/internal/adapter`**.
3.  **`/internal/service` (The Orchestration Layer):** Pulls state from repositories, triggers policy evaluation, executes transition functions, and updates beliefs. Permitted to import `domain/model` and `domain/policy`.
4.  **`/internal/adapter` (The Infrastructure Layer):** Handles PostgreSQL database drivers, Samsara/Motive ELD telemetry feeds, and web-boundary adapters. Permitted to import `domain/model` and `/internal/service`. **No domain code may ever import `/internal/adapter`**.
5.  **`/pkg/math` (The Utility Core):** Reusable mathematical kernels (SPSA matrix perturbations, Cholesky decompositions, correlated Knowledge Gradient updates). It has **zero dependencies** on any application-specific models, behaving as a pure standalone mathematical library.

---

## 5. Observability & Section 19 Compliance
Project Mittens manages highly sensitive, proprietary carrier capacity networks and customer load-bidding data. Our observability paradigm must be incredibly expressive to explain the behavior of stochastic, competitive systems, while remaining strictly secure and leak-free.

### 5.1 The Logging Invariant
Ordinary application logs (`stdout`/`stderr`) must be strictly non-sensitive, concise, and bounded in frequency.
*   **Do Not Log:** Customer names, specific lane coordinates, dollar-value bid values, driver identities, or individual load identifiers.
*   **Do Log:** State transitions, optimization phases, processing durations, batch queue delays, and mathematical convergence indicators.
*   **Trace Context:** All structured logs emitted during an optimization batch must carry the `OptimizationRunID` and `BatchID` to allow seamless correlation.
*   **Debug Mode:** Verbose lookahead tree node allocations or SPSA iteration details must be guarded by explicit `Logger.Debug()` gates to prevent log-disk exhaustion during high-scale runs.

### 5.2 The Distributed Tracing Invariant
We use OpenTelemetry to trace the execution path of matching runs. Every optimization execution must establish a parent span that propagates trace context across all parallel goroutines executing policy evaluations and simulations.
*   **Spans must wrap:** Policy function evaluations, lookup table updates, post-decision state reallocations, and Monte Carlo tree sampling paths.
*   **Attributes:** Every span must carry metadata explaining the state characteristics, e.g., `policy.class = CFA`, `model.N = 1`, `belief.simplex_size = 512`, and `execution.goroutine_count = 32`.

### 5.3 The Metrics Invariant
We capture system-level and mathematical performance indicators using bounded, low-cardinality Prometheus-style counters, gauges, and histograms.
*   **Never use high-cardinality values** (e.g., specific DriverIDs, CustomerIDs, or Coordinate Strings) as metric labels. Doing so creates an out-of-memory panic on the Prometheus metric server due to label-space explosion.
*   **Permitted Metrics:**
    *   `mittens_optimization_duration_seconds` (Histogram by Policy Class)
    *   `mittens_belief_simplex_deviation_ratio` (Gauge capturing floating-point drift from $1.0$)
    *   `mittens_spsa_gradient_magnitude` (Gauge tracking convergence of $	heta$ parameters offline)
    *   `mittens_lookahead_tree_nodes_total` (Histogram capturing direct lookahead tree size)
    *   `mittens_invariant_failures_total` (Counter tracking fail-closed boundary hits)

### 5.4 Semantic Journaling (Complete Provenance)
To understand exactly *why* a particular decision was made (e.g., matching Driver A to Load B with a bid of $1200 over a competing bid), we do not rely on standard application logging. We maintain an independent, append-only **Semantic Journal** within database transactions.
Every single allocation must write a serialized provenance record containing:
1.  **State Frame:** The exact resource state $R_t$ (driver capacities, load positions) and information state $I_t$.
2.  **Belief Vector:** The filtered probability distribution $b_t(\Theta_t)$ over competitor postures.
3.  **Policy Parameterization:** The active coefficient vector $	heta = (	heta_1, 	heta_2)$ used by the CFA or VFA.
4.  **Alternatives Matrix:** The evaluated contribution values, post-decision state values, and competitor pricing risk premiums for *every* candidate assignment in the dispatch tree.
5.  **Audit Trail:** The unique `DecisionID` and `OptimizationRunID` pointing to the exact execution binary version.

---

## 6. Concurrency and Thread Safety Invariants
Writing concurrent code in Go requires strict discipline to prevent race conditions and deadlocks, which are notoriously difficult to debug in production optimization runs.

### 6.1 State Modification is Compiler-Impossible
To guarantee thread safety across millions of lookahead evaluations, the memory representing any state factor must be immutable.
*   If a goroutine must evaluate the transition of a driver $d$ to a destination, it must call:
    `nextResourceState := CurrentState.Transition(action, observation)`
    which reallocates and returns a fresh, isolated memory pointer.
*   **No Mutexes on State Structs:** The domain model package contains zero `sync.Mutex` or `sync.RWMutex` fields. If a struct requires lock synchronization, it is a design violation of Inviolate 5.

### 6.2 Lock-Free Communication
Goroutines must communicate via channel-based message passing.
*   **Channel Buffer Sizes:** Channels used to coordinate lookahead parallelizations must have a buffer size of exactly `0` (unbuffered for tight synchronization) or `1` (single-slot non-blocking notification). Larger buffers are generally design errors that mask resource leaks or performance imbalances.
*   **The Select Multiplexer:** Always use a `select` block with a `context.Context` timeout channel when executing channel reads or writes to prevent goroutine blockage.

```go
select {
case resultsChan <- localEvaluation:
    // Write completed successfully
case <-ctx.Done():
    // Timeout or cancellation triggered; abort execution to prevent thread leaks
    return ctx.Err()
}
```

### 6.3 Concurrency Verifications
All concurrency-affecting pull requests must pass the Go Data Race Detector.
*   During local workstation verification and CI pipelines, tests must be run using:
    `go test -race -v ./...`
*   Any detection of a read/write conflict on shared memory must result in an immediate build rejection, regardless of whether the test apparently passed.

---

## 7. Multi-Agent and AI Working Discipline
When deploying AI code generation tools or multiple subagents to modify Project Mittens, agents must operate under a strict, non-negotiable verification loop.

### 7.1 No "Vibe-Based" Coding
Do not repeatedly modify mathematical formulas or write speculative conditional blocks in a brute-force attempt to make a test turn green.
*   Before writing a single line of code, you must locate the exact mathematical formula in the **MOMDP Competitive Latent State Dynamics Paper** or the **Mittens HLD** and trace how it maps to your type definitions.
*   If a test fails, you must identify the precise physical invariant (e.g., conservation of driver flow, belief simplex sum constraint) that was violated, formulate a mathematical hypothesis for the defect, and fix it systematically.

### 7.2 The Test-Driven Mathematical verification (TDM)
All mathematical transition and filtering equations must be accompanied by rigorous unit tests checking:
*   ** пара-extreme boundary conditions:** (e.g., $N_c = 0$, $b_t = \emptyset$, infinite competitor capacity, zero lane volume).
*   **Precision and Drift Checks:** Float64 mathematical calculations must check for accumulation of rounding errors, verifying that probabilities sum to $1.0 \pm 1e-9$ after 100 consecutive Bayes' transitions.
*   **Parity Verification:** The $N_c = 0$ degenerate test suite must evaluate a standard dispatch sequence and assert that the matched driver-load coordinates match the output of the legacy Java optimizer.

### 7.3 Mandatory Agent Execution Sequence
Before claiming that a task is complete, an agent must execute the following sequential command pipeline inside the sandbox:

1.  **Format Code:** Run `gofmt -s -w .` to enforce canonical Go style.
2.  **Lint & Static Analysis:** Run `golangci-lint run` (or individual `go vet ./...` and `staticcheck ./...` checks) to verify static code sanity.
3.  **Vulnerability Scan:** Run `govulncheck ./...` to ensure zero known vulnerabilities in dependencies.
4.  **Core Tests:** Execute `go test -v ./internal/domain/model/...` and `go test -v ./pkg/math/...` to ensure domain and mathematical models pass.
5.  **Race Detector:** Run `go test -race -v ./...` to verify concurrent safety.
6.  **Build Binary:** Execute `go build ./...` (and `go build -o /workspace/scratch/mittens-opt cmd/optimizer/main.go`) to ensure compilation is unbroken.
7.  **Git Diff Inspection:** Run `git diff` to confirm that no unrelated, commented-out, or diagnostic scaffolding remains in the final submission.

### 7.4 The Mandatory Adversarial PR Review Gate
Before any Pull Request is opened or marked ready for human review, the branch must pass an automated adversarial review gate.

*   **Role & Separation of Concerns:** An independent, read-only subagent (`adversarial_reviewer`) is spawned specifically to red-team the diff. The adversarial agent has **zero write permissions**; it cannot edit code or write fixes. Its sole mandate is to find flaws, Inviolate violations, floating-point drift, immutability leaks, and concurrency hazards.
*   **Gate Verdict:** The adversarial reviewer emits a structured report with a mandatory gate verdict:
    - **`REJECT`:** Any Inviolate breach, data race risk, uncompensated floating-point drift, missing boundary tests, or undocumented non-obvious Go idiom halts the PR pipeline. The primary agent must diagnose the root cause, apply fixes, and re-run the gate.
    - **`APPROVE`:** Granted only when the diff demonstrates complete mathematical rigor, clean architecture adherence, and zero race conditions.
*   **No Bypass:** Under no circumstances may an agent open a PR or declare a task complete while an adversarial review verdict remains `REJECT`.

---

## 8. Glossary of Developer Invariants

For quick reference, keep these developer rules in mind during every code edit:

*   **Inviolate 0 (Explicit Config):** No `init()` function may load env variables. All model and simulation configurations are explicit structs.
*   **Inviolate 1 (Parity Baselines):** If you edit physical matching flow, run the $N=0$ test suite and verify identical assignment payouts.
*   **Inviolate 2 (MOMDP Factoring):** Keep $R_t$ fully observable and deterministic; only $\Theta_t$ transitions stochastically.
*   **Inviolate 3 (Competitive Genericity):** Avoid hardcoding 1-dimensional array operations on competitor vectors. Use Go generics parameterized by $N \ge 0$.
*   **Inviolate 5 (Logical Immutability):** Never mutate a struct's internal state. Allocate, re-assign, and return fresh pointers.
*   **Section 19 (Observability Integrity):** No high-cardinality Prometheus labels; no sensitive load coordinates in normal logs; complete mathematical trails written exclusively to the Semantic Journal.
