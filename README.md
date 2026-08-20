# Project Mittens

> **The Next-Generation Concurrent MOMDP Carrier Optimization Engine in Go**  
> *Optimal Dynamics Platform Core*

[![Go Version](https://img.shields.io/badge/go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-Proprietary-red.svg)](LICENSE)
[![Architecture](https://img.shields.io/badge/architecture-MOMDP%20%2F%20Clean-blue.svg)](docs/high_level_design.md)
[![Concurrency](https://img.shields.io/badge/concurrency-Lock--Free%20GMP-green.svg)](AGENTS.md)
[![Parity](https://img.shields.io/badge/parity-N%3D0%20Verified-brightgreen.svg)](docs/java_parity_migration_plan.md)

---

## 1. Executive Overview

**Project Mittens** is a high-performance, concurrent, and mathematically rigorous optimization engine written in Go. Its primary engineering objective is to replace the legacy Java-based carrier-matching optimizer (`coreai`) with a modern, cloud-native implementation while correcting its most significant theoretical limitation: **modeling customer load arrivals as an endogenous, partially observable competitive market process**.

Traditionally, freight load offers $W_{t+1}$ are modeled as independent, stationary exogenous stochastic processes. In reality, shippers award freight based dynamically on carrier bid prices, lane capacity density, and competing carrier postures. Project Mittens extends Professor Warren B. Powell's canonical **Sequential Decision Analytics (SDA)** framework into a **Mixed-Observability Markov Decision Process (MOMDP)**:
- **Observable Physical Resource State ($R_t$):** Trucks, driver locations, and statutory regulatory clocks evolve deterministically under dispatch assignments $x_t$.
- **Stochastic Competitive Belief State ($b_t$):** Probability distribution over unobserved competitor postures $\Theta_t$ is filtered recursively via Bayesian observation channels using bid win/loss feedback and market clearing rates.

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

---

## 2. Core Engineering Principles

Project Mittens enforces strict engineering invariants ratified in the [Project Mittens Constitution](docs/inviolates.md) and [AGENTS.md](AGENTS.md):

1. **Absolute Mathematical Rigor:** Invariants, transitions, and probability matrices are treated as exact physical laws. Floating-point drift is neutralized using Neumaier compensated summation and log-space projection.
2. **Deterministic Reproducibility:** Given the identical initial state, configuration, and random seed, the engine produces bit-wise identical matchings and decision traces across runs.
3. **High-Efficiency Concurrency:** Capitalizes on Go's GMP scheduler using lock-free, value-based state transitions (`Inviolate 5 & 6`). Zero mutexes on state structs or hot dispatch paths.
4. **Complete Auditability (Provenance):** Every dispatch assignment and pricing decision records a full mathematical trail in the append-only **Semantic Journal** (`Inviolate 7`).
5. **Fail-Closed Robustness:** Infeasible spatial/temporal transitions, HOS breaches, and belief simplex violations trigger fail-closed halts rather than permitting invalid physical dispatches (`Inviolate 8`).
6. **Explicit Configuration:** Zero `init()` functions or hidden global mutable state. All hyperparameters and regulatory rules are strongly typed structs (`Inviolate 0`).
7. **Monopolistic Degeneracy ($N=0$ Parity):** Setting competitor count $N=0$ mathematically collapses the POMDP belief filter to a Dirac delta in $O(1)$, guaranteeing bit-for-bit numerical equivalence with legacy baselines (`Inviolate 1 & 3`).

---

## 3. System Architecture & Clean Boundaries

Project Mittens strictly enforces **Clean Architecture** dependency boundaries to isolate pure mathematical kernels from orchestration and infrastructure:

```
                  ┌───────────────────────────────────────────┐
                  │           /internal/adapter               │
                  │  (PostgreSQL, Legacy Scenario Parser)     │
                  └─────────────────────┬─────────────────────┘
                                        │
                  ┌─────────────────────▼─────────────────────┐
                  │           /internal/service               │
                  │  (Orchestrator, Simulator, Dispatch, VFA) │
                  └─────────────────────┬─────────────────────┘
                                        │
         ┌──────────────────────────────┴──────────────────────────────┐
         ▼                                                             ▼
┌─────────────────────────────────┐                   ┌─────────────────────────────────┐
│     /internal/domain/model      │                   │     /internal/domain/policy     │
│ (MOMDP States, Simplex, HOS)    │◄──────────────────│ (PFAs, CFAs, VFAs, DLAs, LAP)   │
└────────────────┬────────────────┘                   └────────────────┬────────────────┘
                 │                                                     │
                 │         ┌─────────────────────────────────┐         │
                 └────────►│     /internal/domain/rules      │◄────────┘
                           │ (CEL Engine: google/cel-go)     │
                           └────────────────┬────────────────┘
                                            │
                                  ┌─────────▼─────────┐
                                  │     /pkg/math     │
                                  │ (LAP, SPSA, CKG)  │
                                  └───────────────────┘
```

### Dependency Rules:
- **`pkg/math`:** Standalone mathematical library (zero external domain imports). Contains exact Linear Assignment Problem (LAP) solvers, SPSA stochastic optimization, Simplex projections, and Gaussian Process spatial covariance kernels.
- **`internal/domain/model`:** Pure domain core ($R_t$, $I_t$, $b_t$, HOS regulatory clocks, equipment, geography). Zero I/O, zero network, zero SQL.
- **`internal/domain/rules`:** Google CEL business rules engine for conditional rate, bonus, and feasibility adjustments.
- **`internal/domain/policy`:** Powell's four universal policy classes. Pure offline computation.
- **`internal/service`:** Orchestrates simulation rolling horizons, multi-leg tour synthesis batches, and adaptive VFA learning loops.
- **`internal/adapter`:** Ingests legacy carrier scenarios (10,000 loads, 28 drivers) for continuous golden parity benchmarking.

---

## 4. Key Subsystems & Mathematical Models

### 4.1 Exact Linear Assignment (LAP) Solver (`pkg/math/lap.go`)
Implements the polynomial-time **Successive Shortest Path (SSP / LAPJV)** algorithm with Dijkstra dual potentials:
- Guarantees global optimality for dense, rectangular bipartite driver-load networks.
- Resolves greedy **Assignment Paradoxes** (yielding $+85\%$ surplus over greedy heuristics).
- Extracts exact optimal dual shadow prices ($u_d^*, v_\ell^*$) for downstream subgradient learning.

### 4.2 High-Fidelity Hours-of-Service Engine (`internal/domain/model/hos`)
Full regulatory simulation of US FMCSA 70-hour / 8-day rules for property-carrying commercial drivers:
- 11-hour driving ceiling following 10 consecutive hours off duty.
- 14-hour on-duty shift window.
- Mandatory 30-minute hygiene rest breaks after 8 cumulative driving hours.
- 34-hour rolling cycle restarts.
- Split Sleeper Berth provisions ($8/2$ and $7/3$ pairings).

### 4.3 Multi-Leg Tour Synthesis & Chained Dispatch (`internal/domain/policy/tour*.go`)
Sequences multi-day continuous driver journeys across chained freight loads:
- Forward-projects sequential HOS clocks across intermediate pickup, dwell, transit, and unloading events.
- Synthesizes empty reposition legs returning drivers to their home domiciles within scheduled home-time windows.
- Concurrently synthesizes whole-fleet work order batches in `internal/service/dispatch`.

### 4.4 Piecewise-Linear Concave VFA & Correlated Knowledge Gradient (`internal/domain/policy/vfa*.go`, `pkg/math/ckg.go`)
Implements Powell's Class 3 (VFA) policies with spatial learning:
- **Diminishing Marginal Slopes:** Models regional driver value curves $\bar{V}_r(R_r)$ with concave slopes ($v_{r, 0} \ge v_{r, 1} \ge \dots \ge v_{r, K-1}$).
- **CAVE (Concave Adaptive Value Estimation):** Level-clearing projection sweeps that restore strict concavity upon applying noisy subgradient updates.
- **Correlated Knowledge Gradient (CKG):** Spatial Gaussian Process kernel updating neighboring regional valuations via Haversine distance covariance:
  $$\Sigma_{ij} = \sigma_f^2 \exp\left(-\frac{D_{ij}^2}{2\ell^2}\right) + \sigma_n^2 \delta_{ij}$$
- **Anti-Bunching:** Naturally prevents driver clustering by lowering marginal dispatch values for subsequent drivers directed into the same destination region.

### 4.5 Competitive MOMDP Dynamic Bayesian Belief Filter (`internal/domain/model/belief_filter.go`)
Maintains posterior probability distributions over latent competitor postures $\Theta_t \in \{\text{TightCapacity}, \text{SurplusCapacity}\}$:
- **Chapman-Kolmogorov Forward Prediction:** Row-stochastic Markov transition updates with Neumaier compensated summation.
- **Log-Space Bayes' Rule Correction:** Evaluates joint log-likelihoods across Binomial bid outcomes, Gaussian spot clearing rates, and Poisson offer counts using Ramanujan log-factorial expansions.
- Validated over 10,000 consecutive Bayesian updates with total simplex drift bounded to $|\sum b_i - 1.0| < 10^{-11}$.

---

## 5. Repository Layout

```
project_mittens/
├── cmd/
│   └── optimizer/                # Main executable entrypoint
├── docs/
│   ├── stochflow_journaling_audit_and_explainability.md # Lossless replay, audit & decision explainability
│   ├── modular_rollout_and_validation_strategy.md # Friction reduction, proof & modular rollout blueprint
│   ├── pomdp-math-to-code-guide.md # Mathematical MOMDP spec & Python-to-Go code map
│   ├── high_level_design.md      # Authoritative architectural design
│   ├── inviolates.md             # Non-negotiable repository laws
│   ├── java_parity_migration_plan.md # 11-phase migration roadmap
│   └── glossary.md               # Mathematical & domain terminology
├── internal/
│   ├── domain/
│   │   ├── model/                # MOMDP state factoring, entities, HOS engine
│   │   │   └── hos/              # FMCSA regulatory clocks and timeline simulator
│   │   ├── policy/               # Powell policy formulations (CFA, VFA, DLA, LAP, Tours)
│   │   └── rules/                # CEL-based business rule engine (google/cel-go)
│   ├── service/                  # Optimization orchestrator, simulator, dispatch runner
│   │   └── dispatch/             # Multi-leg batch tour synthesis runner
│   └── adapter/
│       ├── api/                  # OpenAPI 3.1 REST API server & Chi route handlers
│       ├── legacy/               # Real carrier scenario parser & golden parity suites
│       └── simulation/           # Ground-truth competitive market & tournament harness
├── pkg/
│   ├── logging/                  # Zero-alloc structured slog wrapper
│   ├── math/                     # Pure math kernels: LAP, SPSA, Simplex, CKG, Stats (t-test)
│   └── telemetry/                # OpenTelemetry distributed tracing & Prometheus metrics
├── deploy/
│   ├── grafana/                  # Auto-provisioned Grafana datasources (Prometheus, Tempo) & dashboards
│   ├── prometheus/               # Prometheus scraping configuration
│   └── tempo/                    # Grafana Tempo distributed tracing configuration
├── docker-compose.yml            # Complete containerized production observability stack
├── Dockerfile                    # Hardened multi-stage non-root distroless container image
├── AGENTS.md                     # Contributor and subagent development guide
└── go.mod                        # Go module dependencies
```

---

## 6. Production Containerization & Observability Stack

Project Mittens provides a turn-key containerized environment built on minimal, hardened, non-root Google Distroless images (`gcr.io/distroless/static-debian12:nonroot`).

### Launching the Complete Stack
To start the optimizer API, Grafana Tempo distributed tracing backend, Prometheus metrics server, and pre-configured Grafana dashboards:

```bash
docker compose up -d
```

### Stack Endpoints

| Service | Port | Description |
| :--- | :--- | :--- |
| **Mittens REST API** | `http://localhost:8080` | OpenAPI 3.1 dispatch optimization (`/api/v1/optimize`), simulation (`/api/v1/simulate`), `/healthz`, and `/metrics` |
| **Grafana UI** | `http://localhost:3000` | Unified executive dashboards & Tempo trace explorer (Anonymous Admin) |
| **Grafana Tempo** | `http://localhost:3201` | High-scale distributed tracing backend (OTLP gRPC on `4317`) |
| **Prometheus Server** | `http://localhost:9091` | Time-series metrics engine scraping 5-second intervals |

---

## 7. Verification & Quality Standards

Project Mittens enforces a mandatory seven-step verification pipeline required before any code may be merged:

```bash
# 1. Format code to canonical Go standard
gofmt -s -w .

# 2. Static analysis and compiler vet
go vet ./...
staticcheck ./...

# 3. Security vulnerability scan
govulncheck ./...

# 4. Execute full test suite under the Go Data Race Detector
go test -race -v -count=1 ./...

# 5. Compile binary targets
go build ./...

# 6. Run legacy carrier 10,000-load parity benchmark
go test -race -v -count=1 ./internal/adapter/legacy/...
```

### The Adversarial PR Review Gate
Before any Pull Request is merged into `main`, an independent, read-only subagent (`adversarial_reviewer`) red-teams the diff. The gate strictly audits for:
- Breaches of any Project Mittens Inviolate.
- State mutability leaks (slices/maps shallow copied across package boundaries).
- Concurrency hazards or missing `ctx.Done()` selects.
- Uncompensated floating-point drift or numerical instability.
- Undocumented Go idioms for cross-functional Python/Java reviewers.

---

## 7. Mathematical References & Foundations

1. **Powell, W. B. (2022).** *Reinforcement Learning and Stochastic Optimization: A Unified Framework.* John Wiley & Sons.
2. **Topaloglu, H., & Powell, W. B. (2006).** Dynamic-programming approximations for managing fleets of vehicles with random travel times. *Operations Research*, 54(4), 638-654.
3. **Frazier, P. I., Powell, W. B., & Dayanik, S. (2009).** The Knowledge-Gradient policy for correlated normal beliefs. *INFORMS Journal on Computing*, 21(4), 599-613.
4. **Jonker, R., & Volgenant, A. (1987).** A shortest augmenting path algorithm for dense and sparse linear assignment problems. *Computing*, 38(4), 325-340.
5. **Ong, S. C., Png, S. W., Hsu, D., & Lee, W. S. (2010).** POMDPs for robotic tasks with mixed observability. *Robotics: Science and Systems VI*.

---

*Project Mittens is developed and maintained by the Optimal Dynamics platform engineering team.*
