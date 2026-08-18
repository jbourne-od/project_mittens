# AGENTS.md

## Purpose

This repository contains numerical applications written in Go. The primary application is the **Vectorized Online POMDP Planner**, but the repository is intentionally structured to support additional planners, simulators, optimizers, services, and numerical tools without turning the codebase into a single application-shaped dependency graph.

The governing priorities are:

1. **Correctness**
2. **Reproducibility**
3. **Clarity**
4. **Measured performance**
5. **Operational simplicity**

Performance matters, sometimes a great deal, but a fast numerical answer that is subtly wrong is merely an efficient way to manufacture bad decisions.

---

## Engineering Philosophy

### Prefer boring, explicit Go

Write idiomatic Go. Prefer small packages, concrete data structures, explicit ownership, composition, and narrow interfaces over framework-heavy abstractions.

In particular:

- Prefer composition over elaborate abstraction hierarchies.
- Define interfaces where they are consumed, not where implementations happen to live.
- Accept interfaces and return concrete types when an interface is genuinely useful.
- Do not create an interface merely to make a single implementation look architecturally important.
- Keep interfaces out of hot numerical loops unless profiling shows the indirection is irrelevant.
- Prefer plain structs and functions to dependency-injection frameworks.
- Prefer the standard library unless a dependency buys us substantial correctness, performance, or maintainability.
- Keep `main` packages thin.
- Make dependencies flow inward toward stable domain and numerical packages.

### Correctness before cleverness

For numerical work, retain the simplest obviously-correct implementation whenever practical, even after adding an optimized implementation.

A scalar reference implementation is often more valuable than another paragraph of documentation. It gives optimized and vectorized code something executable to be correct *against*.

### Performance must be earned with measurements

Do not optimize based on aesthetic instincts.

Before a meaningful optimization:

1. Establish a representative benchmark.
2. Record the baseline.
3. Identify the actual bottleneck.
4. Make the smallest useful change.
5. Verify numerical equivalence.
6. Re-run the benchmark with allocation counts.
7. Keep the optimization only if the trade-off is worthwhile.

“No allocations” is not automatically a virtue. Neither is “vectorized.” The target is lower total cost for the actual workload while preserving correctness and maintainability.

---

# Agent Workflow

## Plan before changing non-trivial behavior

For anything beyond a small, local edit:

1. Read the relevant code, tests, package docs, and design notes first.
2. State the invariant or behavior being changed.
3. Identify the smallest package boundary that should own the change.
4. Write a short implementation plan.
5. Identify how correctness will be tested before implementation.
6. Identify whether performance must be benchmarked.
7. Implement in small steps.
8. Verify before declaring completion.

Use the Superpowers workflows when available, especially:

- brainstorming for non-trivial design work,
- writing-plans before multi-step implementation,
- test-driven-development for features and bug fixes,
- systematic-debugging for unexpected behavior,
- using-git-worktrees for substantial isolated work,
- verification-before-completion before claiming a task is done.

Do not use “the change is obvious” as a substitute for inspecting the surrounding code. Many numerical bugs are obvious in retrospect too.

## Test-driven development

For behavioral changes, prefer:

1. Write or identify the failing test.
2. Run it and observe the expected failure.
3. Implement the minimum change that makes it pass.
4. Refactor only while tests remain green.
5. Run the relevant broader verification suite.

For numerical optimizations, the first test should usually compare the optimized path to a trusted reference implementation.

## Keep changes scoped

Do not combine a requested feature with unrelated cleanup.

Small adjacent improvements are acceptable when they directly reduce risk or make the requested implementation materially clearer. Broad “while I am here” refactors are not.

## Do not silently weaken tests

Never make a numerical test pass by casually:

- widening a tolerance,
- reducing sample size,
- fixing a random seed to an unusually favorable case,
- deleting an edge case,
- removing a benchmark,
- clipping NaNs or infinities,
- changing an assertion from exact to approximate without a mathematical reason.

If a tolerance must change, explain why the previous tolerance was mathematically or numerically inappropriate.

---

# Repository Structure

Prefer the following shape unless the existing repository gives a strong reason not to:

```text
.
├── AGENTS.md
├── README.md
├── go.mod
├── go.sum
├── cmd/
│   ├── vop/                     # Vectorized Online POMDP Planner binary
│   │   └── main.go
│   └── <other-app>/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── vop/                 # VOP application orchestration
│   │   └── <other-app>/
│   ├── pomdp/                   # Core POMDP concepts and contracts
│   ├── planner/                 # Planning algorithms and common planner machinery
│   ├── belief/                  # Belief representations and updates
│   ├── model/                   # Generative / transition / observation model adapters
│   ├── rollout/                 # Rollout/default-policy machinery
│   ├── vector/                  # Vectorized kernels and batch-oriented primitives
│   ├── numeric/                 # Stable numerical primitives and tolerances
│   ├── rng/                     # Deterministic random stream construction
│   ├── search/                  # Generic search/tree structures when truly reusable
│   ├── config/                  # Typed configuration and validation
│   ├── telemetry/               # Metrics/tracing adapters
│   └── testutil/                # Shared test helpers, not production abstractions
├── benchmarks/
│   ├── data/                    # Small representative benchmark fixtures
│   └── README.md
├── docs/
│   ├── design/
│   └── superpowers/
│       ├── specs/
│       └── plans/
└── scripts/                     # Small repo automation only
```

This is a direction, not a package-generation quota. Do not create empty packages in anticipation of some glorious future architecture.

## Package boundaries

### `cmd/*`

Each binary should contain little more than:

- configuration/flag parsing,
- dependency construction,
- lifecycle management,
- calling the application package,
- process exit behavior.

Business logic, planning logic, and numerical kernels do not belong in `main.go`.

### `internal/app/*`

Application orchestration belongs here.

An application package may compose models, planners, telemetry, persistence, and configuration, but should not become the dumping ground for mathematical logic.

### `internal/pomdp`

Own the vocabulary and stable contracts of the problem domain.

Possible concepts include:

- state,
- action,
- observation,
- belief,
- reward,
- discount,
- horizon,
- generative model,
- policy,
- planner result.

Keep the contracts minimal. Do not encode assumptions specific to one planner into the general POMDP layer.

### `internal/planner`

Own planning algorithms and planner-level orchestration.

Planner-specific structures should remain planner-specific until a second real use case proves they are reusable.

Do not create a “universal search framework” because two structs both happen to contain children.

### `internal/vector`

Own vectorized/batched kernels whose purpose is performance.

Every important optimized kernel should have one or more of:

- a simple reference implementation,
- differential tests against a reference implementation,
- targeted benchmarks,
- documented aliasing/mutation behavior.

Do not put generic helpers here merely because they operate on `[]float64`.

### `internal/numeric`

Own genuinely reusable numerical primitives such as:

- stable normalization,
- log-domain operations,
- compensated or pairwise reductions when warranted,
- finite-value validation,
- tolerance helpers,
- probability validation,
- sampling primitives when independent of POMDP semantics.

This package should remain mathematically coherent. It is not `utils`.

### `internal/rng`

Own deterministic random-stream construction and seed derivation.

Randomness is an input to an experiment, not ambient weather.

---

# Numerical Programming Rules

## Use `float64` by default

Use `float64` unless there is a demonstrated reason to use something else.

A move to `float32`, fixed-point arithmetic, SIMD-specific storage, or another representation requires:

- a clear motivation,
- accuracy analysis,
- differential tests,
- benchmarks representative of the intended workload.

Memory bandwidth can absolutely matter. So can being wrong.

## Never compare floating-point values casually

Avoid exact equality for computed floating-point quantities unless exactness is a deliberate invariant.

Use a comparison based on the scale and semantics of the quantity, generally combining absolute and relative error:

```text
|a-b| <= atol + rtol * max(|a|, |b|)
```

Do not create one magical repository-wide epsilon and apply it to rewards, normalized probabilities, log-likelihoods, and accumulated values as if dimensions were an administrative inconvenience.

Tolerances should live near the mathematical contract they represent.

## Treat NaN and infinity as signals

Do not silently convert NaN or infinity into zero, a boundary value, or “best effort” output.

At API and model boundaries:

- validate inputs where invalid values are plausible,
- return contextual errors for invalid numerical states,
- include enough information to reproduce the failure.

Inside performance-critical kernels, avoid redundant checks only when callers clearly own the invariant and tests enforce it.

## Prefer numerically stable formulations

Use stable formulations when values can span meaningful dynamic ranges.

Examples include:

- log probabilities instead of repeated multiplication of tiny probabilities,
- log-sum-exp for probability aggregation in log space,
- stable normalization,
- pairwise or compensated summation when accumulation error is material,
- avoiding subtractive cancellation where a better formulation exists.

Do not introduce a sophisticated technique merely because it exists. Use it when the expected error justifies the complexity.

## Probability invariants

Where a value is semantically a probability distribution:

- values must be finite,
- values must satisfy the model’s non-negativity requirement,
- normalization must be explicit,
- zero-mass cases must have defined behavior,
- underflow behavior must be tested,
- normalization failure must not be hidden.

For belief states, tests should explicitly cover normalization and impossible-observation cases.

## Mutation and aliasing must be obvious

Hot numerical code often benefits from buffer reuse. That is acceptable, but ownership must be explicit.

Prefer APIs whose names or documentation make it clear whether they:

- allocate a new result,
- mutate a receiver,
- write into a destination buffer,
- permit source/destination aliasing.

For example, APIs shaped like:

```go
func Add(dst, x, y []float64)
```

must document whether `dst == x` or `dst == y` is valid.

Accidental aliasing bugs are particularly charming because the output often looks almost correct.

## Data layout is part of performance design

For hot paths, consider:

- contiguous memory access,
- structure-of-arrays versus array-of-structures,
- cache locality,
- branch predictability,
- batch size,
- memory bandwidth,
- allocation rate.

Choose based on benchmarks and access patterns, not on what looks most “vectorized” in a design document.

## Keep a portable baseline

Prefer a correct pure-Go implementation first.

Architecture-specific SIMD, assembly, cgo, GPU execution, or specialized native libraries may be added when measurements justify them, but isolate them behind narrow internal boundaries and retain a portable reference path where feasible.

Do not make the repository impossible to test on a developer laptop to save 4% in a benchmark nobody runs in production.

---

# POMDP-Specific Design Rules

## Keep the model separate from the planner

The planner should depend on the smallest model contract it needs.

Do not allow planner logic to leak into:

- transition dynamics,
- observation generation,
- reward calculation,
- belief representation,
- environment simulation.

Likewise, do not make the model aware of search-tree internals.

A generative model and a real environment may share behavior, but they are not automatically the same abstraction.

## Make planner assumptions explicit

A planner implementation must document the assumptions that affect correctness or behavior, including as applicable:

- finite or continuing horizon,
- discounting,
- action availability,
- observation representation,
- belief representation,
- rollout/default policy,
- terminal-state semantics,
- reward bounds,
- stochastic-model assumptions,
- pruning,
- batching/vectorization semantics.

Do not quietly smuggle assumptions from POMCP, MCTS, particle filtering, or another algorithm into the generic POMDP interfaces.

## Belief is a first-class type

Avoid passing around anonymous `[]float64` values whose semantics depend on which comment the reader last saw.

Belief representations should encode or clearly document:

- what each element represents,
- whether the representation is normalized,
- whether it is dense, sparse, particle-based, factored, or otherwise,
- ownership/mutation rules,
- how impossible observations are handled.

If several belief representations exist, do not force them behind one interface unless the planners genuinely benefit from that abstraction.

## Separate decision quality from compute budget

Online planning usually has an explicit resource constraint.

Represent the stopping or budget mechanism clearly, such as:

- number of simulations,
- wall-clock deadline,
- node expansions,
- depth,
- memory budget,
- convergence or confidence rule.

Avoid burying compute-budget logic inside unrelated tree operations.

A planner result should make it possible to understand *why* planning stopped.

## Vectorization must preserve planner semantics

Batching simulations, actions, particles, observations, or belief updates must not silently change algorithmic behavior.

For vectorized paths, test:

- equivalence to the scalar/reference path,
- deterministic behavior under the same random-stream specification,
- edge batch sizes such as 0, 1, and non-multiples of internal block sizes,
- terminal states inside a batch,
- mixed valid/invalid or active/inactive lanes where applicable,
- stable ordering when ordering is semantically meaningful.

If vectorization changes random-number consumption, that must be intentional and documented.

## Distinguish algorithmic approximation from numerical error

Approximate planning is expected. Uncontrolled floating-point or implementation error is not.

Tests and metrics should distinguish:

- Monte Carlo variance,
- approximation error from finite search,
- model approximation,
- numerical error,
- implementation error.

Do not widen floating-point tolerances because a stochastic algorithm has variance. Those are different sources of uncertainty.

---

# Randomness and Reproducibility

## No ambient randomness

Do not use package-level random state in planner or numerical code.

Seeds and streams should be explicit inputs.

Prefer constructors or APIs that receive an RNG or deterministic stream factory rather than reaching into shared global state.

## Reproducible by default

A planner run used for testing, benchmarking, or debugging should be reproducible from captured inputs that include at least:

- seed,
- model/configuration,
- planner parameters,
- stopping budget,
- relevant version/build information where available.

If the production system intentionally uses nondeterministic seeds, log or return the realized seed when operationally appropriate.

## Parallel randomness

Concurrent execution must not create accidental nondeterminism through shared RNG access.

Prefer deterministic stream partitioning, for example by deriving independent streams from:

- root seed,
- worker index,
- simulation index,
- batch index,
- episode index.

The derivation scheme must be stable and tested.

Changing `GOMAXPROCS` should not silently change numerical results unless nondeterministic execution is an explicit design choice.

---

# Concurrency

Use concurrency when the workload benefits from it, not because goroutines are the local wildlife.

## Rules

- Prefer coarse-grained parallelism over millions of tiny goroutines.
- Bound worker counts.
- Avoid oversubscribing CPUs when a lower-level numerical library already uses threads.
- Partition mutable buffers by worker where possible.
- Do not rely on concurrent map or slice writes without explicit ownership.
- Propagate `context.Context` cancellation at orchestration boundaries.
- Do not thread `context.Context` through pure numerical functions that do not perform blocking or cancellable work.
- Keep deterministic and parallel concerns separate enough to test independently.

Run the race detector for concurrency changes.

---

# Error Handling

## Libraries return errors; applications decide policy

Deep packages should not:

- call `os.Exit`,
- log and continue,
- panic for recoverable input/model errors.

Return contextual errors and let the application layer determine policy.

## Panics are for broken invariants

A panic may be appropriate for an impossible internal state that indicates a programming error, especially in code that cannot meaningfully recover.

Do not panic because an external model returned malformed data or a configuration value was invalid.

## Add context without destroying identity

Wrap errors with `%w` when callers may need to inspect the cause.

Error messages should explain the failed operation and relevant identifiers or dimensions without dumping massive vectors or sensitive payloads.

---

# API and Application Boundaries

## CLI

For small command-line applications, prefer the standard library `flag` package.

Use a CLI framework only when command structure, help generation, configuration layering, or UX complexity genuinely warrants one.

Flags should map into typed configuration, then be validated once before execution.

## HTTP

Do not add an HTTP API unless an application needs one.

If a service interface is required, prefer idiomatic `net/http`; `chi` is a good default router for a small explicit API. Use Gin when existing project conventions or a concrete feature make it the better fit.

Handlers should be thin:

```text
HTTP request
    -> parse/validate
    -> application service
    -> translate result/error
    -> HTTP response
```

Planner and numerical packages must not depend on HTTP concepts.

## Cloud Run

If an application is deployed to Cloud Run:

- keep the process stateless unless external state is explicitly part of the design,
- respect request cancellation and shutdown,
- make CPU and memory assumptions explicit,
- bound concurrent expensive planner requests,
- expose appropriate health behavior,
- avoid initialization work on every request,
- validate whether request latency limits are compatible with the planning budget.

Do not let an HTTP concurrency default accidentally become a numerical admission-control policy.

---

# Configuration

Configuration should be typed and validated.

Prefer:

```go
type Config struct {
    Seed        uint64
    Horizon     int
    Discount    float64
    Simulations int
}
```

over maps of strings or unstructured blobs inside core code.

Validation belongs near construction/application boundaries.

Configuration should make important algorithmic choices explicit. Avoid hidden defaults for parameters that materially change planner behavior.

Defaults are acceptable when they are:

- safe,
- documented,
- mathematically sensible,
- stable enough not to make experiments irreproducible.

---

# Testing Strategy

Testing a numerical planner requires more than line coverage.

## Test layers

### 1. Unit tests

Use for:

- mathematical primitives,
- model contracts,
- belief updates,
- tree operations,
- deterministic seed derivation,
- configuration validation,
- edge cases.

### 2. Differential/reference tests

Optimized/vectorized implementations should be compared against trusted simpler implementations.

Use multiple shapes, random-but-seeded inputs, and pathological edge cases.

### 3. Property tests

Useful properties include:

- normalized beliefs sum to one within the contract tolerance,
- probability values remain finite and valid,
- deterministic seeds reproduce results,
- equivalent scalar/vector paths agree,
- permutation invariance where mathematically required,
- monotonicity or bounds where the model guarantees them,
- terminal states remain terminal,
- impossible actions are never selected.

Go fuzzing is appropriate when properties can be cheaply checked over a wide input space.

### 4. Statistical tests

Use statistical assertions only when deterministic tests cannot express the behavior.

Statistical tests must:

- use explicit seeds,
- have enough samples to justify their thresholds,
- avoid flaky pass/fail boundaries,
- explain the statistic being tested.

Do not use a stochastic test to avoid writing a deterministic test.

### 5. Integration tests

Exercise real composition across model, belief, planner, and application boundaries.

Keep them fewer and more representative than unit tests.

### 6. Benchmarks

Benchmark hot numerical paths and representative planner workloads.

Use `-benchmem`.

Track at least the dimensions that matter for the workload, such as:

- actions,
- states/particles,
- observations,
- horizon/depth,
- batch size,
- simulations,
- worker count.

Avoid microbenchmarks whose data fits perfectly in cache if production does not.

## Edge cases worth testing explicitly

At minimum, consider:

- empty inputs where permitted,
- single-element vectors,
- zero-probability events,
- all-zero unnormalized weights,
- very small probabilities,
- very large/small rewards,
- horizon zero and one,
- discount zero and one if valid,
- no legal actions,
- terminal root state,
- repeated observations,
- batch size one,
- uneven final batch,
- NaN/Inf rejection,
- cancellation/deadline,
- fixed seed across serial and parallel modes.

---

# Benchmarking and Profiling

## Representative benchmarks first

Before changing a hot path, create or use a benchmark that resembles the actual workload.

Run:

```bash
go test ./... -run '^$' -bench . -benchmem
```

For focused work, benchmark the affected package instead of the entire repository.

## Compare statistically

When performance matters enough to influence design, prefer multiple benchmark samples and `benchstat` rather than comparing two single noisy runs.

Do not celebrate a 3% win measured once on a laptop performing seventeen unrelated acts of modern computing.

## Profile before large optimization work

Use Go profiling tools as appropriate:

- CPU profiles,
- heap/allocation profiles,
- execution traces,
- mutex/block profiles for concurrency problems.

Optimization proposals should name the observed bottleneck.

---

# Observability

The planner should expose enough telemetry to diagnose quality and cost without coupling core algorithms to a specific telemetry vendor.

Potential metrics include:

- planning latency,
- simulations completed,
- expansions,
- maximum/average search depth,
- batch sizes,
- action counts,
- model calls,
- belief-update counts,
- cache hit rates,
- stop reason,
- allocation or memory high-water information where operationally useful.

Algorithm-specific metrics should remain algorithm-specific.

Avoid high-cardinality labels derived from state vectors, observations, seeds, or raw IDs.

Tracing belongs at meaningful application/planner boundaries, not inside every vector add.

---

# Dependencies

Every new dependency should answer:

1. What does it provide that the standard library or existing dependencies do not?
2. Is it maintained?
3. Is the API stable?
4. What does it add to the dependency/security surface?
5. Does it appear in a hot path?
6. Can it preserve reproducibility?
7. Is the performance claim measured in our workload?

For numerical work, a mature library such as Gonum may be preferable to hand-implementing sophisticated algorithms. Conversely, do not import a large numerical stack merely to compute a dot product.

Avoid framework dependencies that impose architecture unrelated to the problem.

---

# Security and Supply Chain

This is a numerical repository, not a security product, but ordinary software hygiene still applies.

- Do not commit credentials or tokens.
- Do not log secrets.
- Validate untrusted serialized inputs.
- Bound allocations driven by external dimensions.
- Avoid unsafe code unless a measured hot path justifies it.
- Treat `unsafe`, assembly, cgo, and architecture-specific code as higher-review areas.
- Pin and review dependencies through `go.mod`/`go.sum`.

If cryptographic code is ever introduced, isolate it in a dedicated package and apply substantially stronger review and testing. Do not invent cryptographic primitives.

---

# Go Style

## Naming

Use short names when scope is short and meaning is obvious.

Use domain names over implementation names:

- `belief`, not `floatSlice2`,
- `reward`, not `val`,
- `simulations`, not `n` at application boundaries.

Inside a five-line numerical loop, `i`, `j`, `x`, and `n` are perfectly respectable.

## Functions

Prefer functions small enough that their invariants fit in working memory.

Extract helpers when they create a real concept, reusable invariant, or test seam. Do not decompose a readable numerical kernel into twelve one-line functions in the name of cleanliness.

## Comments

Comments should explain:

- mathematical intent,
- invariants,
- non-obvious numerical choices,
- aliasing constraints,
- performance trade-offs,
- why a strange-looking implementation is necessary.

Do not narrate syntax.

For nontrivial formulas, include the mathematical definition or a reference to the relevant design document/paper.

## Generics

Use generics when the abstraction is genuinely type-parametric and improves the code.

Do not turn numerical code into a miniature template metaprogramming language to avoid writing two clear functions.

For performance-sensitive generic code, benchmark the instantiated path.

---

# Documentation

Each significant planner or algorithm should document:

- the problem it solves,
- algorithmic assumptions,
- inputs and outputs,
- complexity or dominant cost,
- approximation behavior,
- reproducibility behavior,
- important references,
- known failure modes.

For substantial changes, keep design notes under `docs/design/` or the Superpowers spec/plan directories.

When implementing from a paper, do not rely on variable names alone. Map the paper’s notation to code concepts explicitly.

---

# Verification Commands

Before claiming a change is complete, run the smallest relevant set first, then the broader suite.

Typical repository verification:

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
staticcheck ./...
govulncheck ./...
```

For performance-sensitive changes:

```bash
go test ./path/to/package -run '^$' -bench . -benchmem
```

For repeated benchmark comparison, use `benchstat` when available.

Not every tiny documentation-only change requires every command. Behavioral code changes do require tests. Concurrency changes require the race detector. Numerical hot-path changes require correctness tests and benchmarks.

If a verification command cannot be run, state exactly what was not run and why.

---

# Review Checklist

Before finishing a change, verify:

## Architecture

- Does the change live in the correct package?
- Did application-specific behavior leak into generic numerical/domain code?
- Is a new interface actually necessary?
- Is the dependency direction still clean?

## Numerical correctness

- Are floating-point comparisons mathematically justified?
- Can inputs produce NaN or Inf?
- Are probability/belief invariants preserved?
- Is the implementation stable at extreme values?
- Is approximation error being confused with floating-point error?

## Reproducibility

- Is randomness explicit?
- Can the failing/run scenario be reproduced from a seed and config?
- Does concurrency alter random streams or results unintentionally?

## Performance

- Is this actually a hot path?
- Is there a representative benchmark?
- Did allocations improve or worsen?
- Did the optimization preserve the scalar/reference behavior?
- Is complexity worth the measured gain?

## Go quality

- Are interfaces small and consumer-owned?
- Are errors contextual and wrapped appropriately?
- Is mutation/ownership obvious?
- Is concurrency bounded?
- Are goroutine lifetimes clear?
- Is `context.Context` used only where appropriate?

## Testing

- Was the behavior test written or identified before the implementation?
- Are edge cases represented?
- Is there a differential test for optimized numerical code?
- Is any test made artificially weaker?
- Was the race detector run for concurrency changes?

---

# Things Agents Must Not Do

Do not:

- create a generic `utils` package,
- add abstractions for hypothetical future applications,
- introduce an interface for every struct,
- use global mutable RNG state,
- hide numerical failures by clipping values,
- change tolerances merely to make tests pass,
- optimize without a benchmark,
- remove the readable reference implementation solely because the optimized path exists,
- parallelize code without considering reproducibility,
- use map iteration where deterministic numerical ordering matters,
- add a framework to solve a problem the standard library already solves cleanly,
- put planner logic in HTTP handlers or CLI parsing,
- put HTTP/CLI concepts into planner or numerical packages,
- log from low-level numerical code,
- panic on ordinary input/model errors,
- use `unsafe` because it looks faster,
- claim a performance win without measurement,
- claim completion without verification.

---

# Default Decision Rule

When several implementations are plausible, prefer the one that:

1. makes the mathematical invariant easiest to state,
2. makes correctness easiest to test,
3. keeps ownership and mutation explicit,
4. preserves reproducibility,
5. has acceptable measured performance,
6. leaves the fewest unnecessary abstractions behind.

The planner exists to reason under uncertainty. The code implementing it should introduce as little additional uncertainty as possible.
