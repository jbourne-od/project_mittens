# Project Mittens Inviolates

**Status:** Ratified; highest repository-level architectural authority for Project Mittens
**Ratified:** 2026-08-19
**Project Context:** Java-to-Go Optimizer Migration & Exogenous Load Modeling Framework

---

## Purpose

This document intentionally uses **Inviolate**, not *invariant*.

An **Inviolate** is a project law: a boundary that Project Mittens' design, implementation, operation, and review process may not cross. An **invariant** is a formal property checked by the compiler, static analysis, unit tests, or runtime guardrails. Invariants may enforce Inviolates, but the terms are not interchangeable.

This document is the highest repository-level architectural authority for Project Mittens. Its ratification establishes the following amendment discipline:
*   Inviolate numbers never move and are never reused.
*   A retired Inviolate remains as a numbered tombstone that identifies its replacement or explains why it no longer applies.
*   Changing the meaning of an Inviolate requires explicit approval and a deliberate amendment. Affected designs, contracts, tests, and authority references change with it; implementation cannot silently reinterpret it.
*   Operational difficulty is evidence for a proposed amendment, not permission to route around an Inviolate.
*   A demonstrated Inviolate violation blocks code approval and repository merge, regardless of passing tests or apparent operational success.

---

## Articles of the Mittens Constitution

### 0. Run-affecting configuration is explicit state
All operational configurations, hyperparameters, simulation parameters, and competitor pricing/capacity matrices must be passed as explicit, strongly-typed parameters in the state or the action context. Semantic code representing the sequential decision process may not discover parameters from ambient environment variables, untyped files, global maps, or package-level initializers.
*   **Architectural Justification:** Ensures exact deterministic replayability, isolating execution from local operational environments.
*   **Authority Reference:** *Sequential Decision Analytics and Modeling: Modeling with Python* (Powell, 2022). Original Link: [http://tinyurl.com/sdaexamplesprint](http://tinyurl.com/sdaexamplesprint).
*   **Go Implementation standard:** *Uber Go Style Guide: Avoid Init* (GitHub). Original Link: [https://github.com/uber-go/guide/blob/master/style.md](https://github.com/uber-go/guide/blob/master/style.md#avoid-init).

### 1. Monopolistic Degeneracy ($N=0$) must be numerically equivalent to the legacy Powell framework
When the competitive dimension is set to $N=0$ (representing monopolistic operations or competitor absence), the state variable, transition functions, and observation probability distributions must mathematically and numerically collapse into the standard, fully observable Powell framework. No competitive logic, latent-factor filtering, or belief-state matrix operations may introduce numerical drift, precision divergence, or runtime performance overhead compared to a native monopolistic implementation.
*   **Architectural Justification:** Establishes the legacy Java optimizer as a strict baseline, validating core physical state logic before adding competitive complexity.
*   **Authority Reference:** *Reinforcement Learning and Stochastic Optimization: A Unified Framework for Sequential Decisions* (Powell, 2022). Original Link: [http://tinyurl.com/RLandSO/](http://tinyurl.com/RLandSO/).
*   **Mathematical Proof Reference:** *Theorem 2 (Market Collapse to the Powell Degenerate Case)* of the Partially Observable Competitive Latent State Dynamics Paper (Sandoval-Segura et al., 2026). Original Link: [https://arxiv.org/abs/2201.00258](https://arxiv.org/abs/2201.00258).

### 2. State spaces must enforce Mixed Observability (MOMDP separation)
The system state must be factored into a fully observable state component $R_t$ (physical resource states such as driver locations, truck capacities, and scheduled loads) and a partially observable belief state component $b_t$ (representing the probability distribution over latent competitor postures $\Theta_t$). The transition dynamics of physical resources $R_t$ must remain strictly deterministic and separated from the stochastic Bayesian filtering of $b_t$.
*   **Architectural Justification:** Avoids exponential explosion in value-iteration or tree-search complexity by exploiting the observable resource structure.
*   **Authority Reference:** *A Closer Look at MOMDPs* (Araya-López et al., 2010). Original Link: [https://inria.hal.science/inria-00535559v1](https://inria.hal.science/inria-00535559v1).
*   **Application Reference:** *Mixed Observability MDPs for Shared Autonomy with Uncertain Human Behaviour* (Zilberstein, 2018). Original Link: [https://arxiv.org/abs/1301.3836](https://arxiv.org/abs/1301.3836).

### 3. API interfaces must be competitive-dimension generic ($N \ge 0$)
All state representing competitors, observation emissions, transition maps, and lookahead expectation operators must utilize generic interfaces parameterizable by the competitor count $N \ge 0$. Under no circumstances may $N=1$ (collapsed competitive market state) be hardcoded into the mathematical core. The Go type system must support arbitrary competitor dimensions, ensuring that moving to $N > 1$ (Interactive POMDPs or Dec-POMDPs) does not require changing core interface definitions.
*   **Architectural Justification:** Bypasses current data limits while preserving the codebase's long-term extension paths.
*   **Authority Reference:** *The Complexity of Decentralized Control of Markov Decision Processes* (Bernstein et al., 2002). Original Link: [https://arxiv.org/abs/1301.3836](https://arxiv.org/abs/1301.3836).
*   **Library Pattern Reference:** *POMDPs.jl: A Framework for Sequential Decision Making under Uncertainty* (JuliaPOMDP). Original Link: [https://github.com/JuliaPOMDP/POMDPs.jl](https://github.com/JuliaPOMDP/POMDPs.jl).

### 4. There is one closed, typed source of business logic (No Ambient Code)
All dispatching, pricing, and matching logic must be defined via explicit Powell policy classes (PFAs, CFAs, VFAs, or DLAs) that implement strict Go interfaces. Arbitrary inline functions, database stored procedures, dynamic SQL generation, or runtime-discovered Go source files are barred from the decision-making path. The core optimizer must compile into a single static binary.
*   **Architectural Justification:** Guarantees that the running optimizer is always 100% auditable and prevents out-of-band behavioral changes.
*   **Authority Reference:** *Go Project Structure 2026: Clean Architecture & Best Practices* (Reintech). Original Link: [https://reintech.io/blog/go-project-structure-2026-clean-architecture](https://reintech.io/blog/go-project-structure-2026-clean-architecture).
*   **Style Reference:** *Effective Go: Program Structure* (Golang). Original Link: [https://go.dev/doc/effective_go](https://go.dev/doc/effective_go).

### 5. Semantic state blocks are strictly immutable
Once created, state blocks (representing resource states $R_t$, information states $I_t$, or belief states $b_t$) cannot be mutated. Any policy evaluation, transition function $S^M$, or Bayesian update must return a newly allocated, immutable state pointer. 
*   **Architectural Justification:** Prevents data races in Go's concurrent execution environment and allows branch-level rollbacks during lookahead searches without state corruption.
*   **Authority Reference:** *Effective Go: Pointers vs. Values* (Golang). Original Link: [https://go.dev/doc/effective_go#pointers_vs_values](https://go.dev/doc/effective_go#pointers_vs_values).
*   **Concurrency Reference:** *Go Concurrency Patterns: Share Memory by Communicating* (Golang). Original Link: [https://go.dev/talks/2012/concurrency.slide](https://go.dev/talks/2012/concurrency.slide).

### 6. Concurrent execution must utilize lock-free message passing on hot paths
Evaluations of complex lookahead trees (DLAs) or randomized simulations (such as Monte Carlo tree rolls) must utilize Go's lightweight goroutines and channel-based communication rather than shared-memory mutexes on the hot decision paths. Goroutines on planning paths must respect standard `context.Context` cancellation to prevent zombie execution threads on timeout.
*   **Architectural Justification:** Capitalizes on Go's GMP scheduler to maximize thread efficiency during real-time load matching.
*   **Authority Reference:** *Understanding Real-World Concurrency Bugs in Go* (Tu et al., 2019). Original Link: [http://dx.doi.org/10.1145/3297858.3304069](http://dx.doi.org/10.1145/3297858.3304069).
*   **Code Pattern Reference:** *Go select Statement: Multiplexing Channels* (Golang Tutorial). Original Link: [https://go.dev/ref/spec#Select_statements](https://go.dev/ref/spec#Select_statements).

### 7. Decisions must generate complete, auditable provenance
Any assignment or pricing action output by the optimizer must pack a serialized provenance structure detailing the exact state inputs, the chosen policy class, the parameter vectors (such as CFA coefficients $	heta$), the active competitive belief vector $b_t$, and the evaluated marginal contributions for all alternatives. Deciding paths without recording these variables is a violation of certified execution.
*   **Architectural Justification:** Allows operations teams to diagnose "why did the machine match driver $d$ to load $\ell$?" with absolute scientific certainty.
*   **Authority Reference:** *The Parametric Cost Function Approximation: A new approach for multistage stochastic programming* (Powell & Ghadimi, 2022). Original Link: [https://arxiv.org/abs/2201.00258](https://arxiv.org/abs/2201.00258).
*   **Learning Reference:** *Optimal Learning* (Powell, 2011). Original Link: [http://tinyurl.com/optimallearning/](http://tinyurl.com/optimallearning/).

### 8. Validation and model checking fail closed
Before any execution run, the model parameters (transition matrices, price elasticity models, network geography, driver availability) must pass static integrity checks. During execution, state boundaries and probability mass sums (beliefs must sum to $1.0 \pm \epsilon$) must be monitored by active invariants. Any verification failure must immediately panic and halt execution rather than accepting data on a best-effort basis.
*   **Architectural Justification:** Prevents corrupted beliefs or impossible resource assignments from translating into physical truck dispatches, protecting real-world carriers from operational losses.
*   **Authority Reference:** *Automated Dynamic Concurrency Analysis for Go* (Lange et al., 2017). Original Link: [https://doi.org/10.1145/3009837.3009847](https://doi.org/10.1145/3009837.3009847).
*   **Robustness Reference:** *Handling Particle Depletion · ParticleFilters.jl* (JuliaPOMDP). Original Link: [https://github.com/JuliaPOMDP/ParticleFilters.jl](https://github.com/JuliaPOMDP/ParticleFilters.jl).