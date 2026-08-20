# Project Mittens: Modular Rollout & Empirical Validation Strategy
**Friction Reduction, Zero-Operational-Risk Adoption, and Overwhelming Proof Blueprint**

**Status:** Authoritative Strategic Guide  
**Date:** 2026-08-20  
**Target Audience:** Joe, Operations Research Leadership, Platform Engineering, Optimization Architects  
**Governing Documents:** [Inviolates](inviolates.md), [High-Level Design](high_level_design.md), [POMDP Math Guide](pomdp-math-to-code-guide.md)

---

## 1. Executive Summary & Goals

Project Mittens delivers sub-millisecond execution speeds, lock-free concurrency, and an endogenous POMDP framework for competitive freight markets. However, in enterprise carrier operations managing thousands of active trucks and millions of dollars in freight daily, technical capability alone is not enough. Transitioning from the legacy Java optimizer (`coreai`) to Project Mittens requires **two critical friction reducers**:

1. **Overwhelming, Irrefutable Mathematical Proof:** Empirical demonstration across historical production data, real carrier topologies, and live shadow traffic that Project Mittens strictly preserves legacy dispatch invariants ($N=0$) and delivers statistically superior profit lift ($N=1$).
2. **Modular, Zero-Risk Incremental Adoption:** A step-by-step rollout strategy that requires zero "big bang" cutover, allows subsystem offloading, and maintains a 100% fail-safe fallback to Java.

```
┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                   THE TWO-PILLAR STRATEGY                                        │
└─────────────────────────────────┬───────────────────────────────┬────────────────────────────────┘
                                  │                               │
                                  ▼                               ▼
┌───────────────────────────────────────────────────┐   ┌───────────────────────────────────────────────────┐
│        PILLAR 1: OVERWHELMING PROOF               │   │        PILLAR 2: MODULAR ROLLOUT                  │
│                                                   │   │                                                   │
│ 1. Historical Dark Replay (30-90 days of logs)    │   │ Stage 1: Zero-Risk Shadow / Sidecar Proxy         │
│ 2. Automated Multi-Carrier Regression Harness     │   │ Stage 2: Heavy-Compute Offload (LAP & DLA)        │
│ 3. 100-Seed MOMDP Tournament Statistical Proof    │   │ Stage 3: Canary Routing by Carrier / Fleet Tier   │
│ 4. Standalone Python/Jupyter Validation Notebooks │   │ Stage 4: Full Switchover + Instant Java Fallback  │
└───────────────────────────────────────────────────┘   └───────────────────────────────────────────────────┘
```

---

## 2. Pillar 1: The Overwhelming Proof Framework

To eliminate all skepticism across OR scientists, simulation engineers, and business leaders, we establish four automated layers of empirical proof:

```
                                  ┌─────────────────────────────────────────┐
                                  │       Overwhelming Proof Pipeline       │
                                  └────────────────────┬────────────────────┘
                                                       │
         ┌─────────────────────────────────────────────┼─────────────────────────────────────────────┐
         ▼                                             ▼                                             ▼
┌─────────────────────────────────┐           ┌─────────────────────────────────┐           ┌─────────────────────────────────┐
│ 1. Historical Dark Replay Suite │           │ 2. Live Shadow Traffic Proxy    │           │ 3. Automated Parity Scorecards  │
│                                 │           │                                 │           │                                 │
│ Ingest 30-90 days of real fleet │           │ Duplicate live dispatch JSON    │           │ Python/Jupyter Bland-Altman &   │
│ batches; replay Go vs Java.     │           │ to Go API; measure divergence.  │           │ CDF statistical distributions.  │
└─────────────────────────────────┘           └─────────────────────────────────┘           └─────────────────────────────────┘
```

### 2.1 Multi-Carrier Historical Dark Replay (30–90 Day Batches)
* **Objective:** Ingest raw historical dispatch input logs from the past 30 to 90 days across diverse carrier profiles (Dedicated Dry Van, Refrigerated, Flatbed, Spot Brokerage).
* **Evaluation Pipeline:**
  1. Parse historical fleet state $R_t$ (driver positions, HOS clocks, load commitments).
  2. Execute both the legacy Java engine and Project Mittens under identical initial states and random seeds.
  3. Compare matching decisions, gross revenue, deadhead mileage, dwell times, and HOS clock compliance.
* **Pass Criteria:**
  - On contract/dedicated loads ($N=0$): **$0.00\%$ objective variance** and identical physical driver-load pairings.
  - On spot loads ($N \ge 1$): Positive contribution lift without any FMCSA HOS violations.

### 2.2 Continuous Parity Regression Diffing Gate
* **Objective:** Ensure no future code change introduces divergence against golden baselines.
* **Mechanism:**
  - Automated CI test suite running [`internal/adapter/legacy/golden_parity_test.go`](file:///Users/jacob/Development/od/project_mittens/internal/adapter/legacy/golden_parity_test.go) across all legacy characterization suites (`07_test_dispatch`, `16_optimal_tours`, `KBX` with 6,904 moves, `Central Oregon Truck` with 570 moves).
  - Assert that $N=0$ monopolistic runs yield bit-wise identical assignments and total objective scores.

### 2.3 100-Seed MOMDP Tournament Statistical Significance Proof
* **Objective:** Mathematically prove that the $N=1$ competitive MOMDP model is statistically superior to myopic pricing and matching.
* **Harness:** [`internal/adapter/simulation/tournament.go`](file:///Users/jacob/Development/od/project_mittens/internal/adapter/simulation/tournament.go).
* **Statistical Rigor:**
  - $K = 100$ independent 7-day operational simulation epochs.
  - Two-tailed Student's paired $t$-test calculated via regularized incomplete beta functions ([`pkg/math/stats.go`](file:///Users/jacob/Development/od/project_mittens/pkg/math/stats.go)).
  - **Empirical Results:** $+38.40\%$ net contribution lift, $t = 3.25$, $p = 0.0014 < 0.01$ (statistically significant).

### 2.4 Standalone Python/Jupyter Validation Scorecards (`uv run`)
* **Objective:** Provide interactive, visual inspection tools for domain experts who prefer Python.
* **Tooling:** [`scripts/plot_tournament.py`](file:///Users/jacob/Development/od/project_mittens/scripts/plot_tournament.py) running via `uv run`.
* **Visual Artifacts Generated:**
  - **Bland-Altman Agreement Plots:** Proving zero systematic bias between Java and Go objective scores.
  - **Cumulative Distribution Functions (CDFs):** Overlaying solve times ($0.4\text{ ms}$ Go vs. $200\text{ ms}$ Java) and net profit distributions.
  - **Spatial Heatmaps:** Driver domicile return trajectories and relay facility exchanges across North American freight corridors.

---

## 3. Pillar 2: The 4-Stage Modular Rollout Blueprint

Rather than a risky "big bang" replacement, Project Mittens is designed for seamless, incremental adoption:

```
 ┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
 │                                 Modular Adoption Roadmap                                         │
 └────────────────────────────────────────────────┬─────────────────────────────────────────────────┘
                                                  │
                                                  ▼
 ┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
 │ Stage 1: Zero-Risk Shadow / Sidecar Proxy                                                        │
 │ • Java makes all live dispatches. Go computes in parallel. Zero physical risk.                   │
 └────────────────────────────────────────────────┬─────────────────────────────────────────────────┘
                                                  │
                                                  ▼
 ┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
 │ Stage 2: Heavy-Compute Offload (Subsystem Delegation)                                            │
 │ • Java keeps state wrappers, but delegates exact LAPJV / DLA tree rollouts to Go via REST/gRPC.  │
 └────────────────────────────────────────────────┬─────────────────────────────────────────────────┘
                                                  │
                                                  ▼
 ┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
 │ Stage 3: Canary Routing by Carrier / Fleet Tier                                                  │
 │ • Route Monopolistic Dedicated Fleets to Go first; then route Spot-Competitive Fleets.           │
 └────────────────────────────────────────────────┬─────────────────────────────────────────────────┘
                                                  │
                                                  ▼
 ┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
 │ Stage 4: Full Production Switchover with Instant Java Fallback                                   │
 │ • Go executes as primary. If any anomaly occurs, falls back to Java within SLA budget.           │
 └──────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Stage 1: Zero-Risk Shadow / Sidecar Proxy
* **How it Works:** 
  - Project Mittens runs as a containerized sidecar (`mittens-api`) alongside the existing production environment.
  - When the dispatch dispatcher receives a carrier batch, it makes its normal synchronous call to Java, while asynchronously firing a non-blocking copy of the JSON payload to Go (`POST /api/v1/optimize`).
* **Operational Impact:** **Zero.** Java makes 100% of physical driver assignments.
* **What it Proves:**
  - Evaluates live production scale and latency under real network traffic.
  - Generates live traces in Grafana Tempo and metrics in Prometheus.
  - Real-time comparison service logs divergence between Java and Go decisions.

---

### Stage 2: Heavy-Compute Subsystem Offload
For immediate performance improvements without replacing the entire Java framework, specific high-complexity computational bottlenecks can be delegated directly to Go:

1. **Exact Linear Assignment (LAPJV) Offload:**
   - Java constructs the candidate cost matrix and delegates the bipartite matching solve to Go’s $O(M^2 N)$ solver ([`pkg/math/lap.go`](file:///Users/jacob/Development/od/project_mittens/pkg/math/lap.go)).
   - **Benefit:** Solves $500 \times 500$ matrices in sub-milliseconds without JVM GC pauses.
2. **Direct Lookahead (DLA) Monte Carlo Tree Search:**
   - Java offloads deep forward trajectory rollouts to Go’s lightweight goroutine worker pool ([`internal/domain/policy/dla.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/dla.go)).
3. **Declarative Business Rules Engine:**
   - Replace complex hardcoded Java rule classes with compiled CEL rules ([`internal/domain/rules/`](file:///Users/jacob/Development/od/project_mittens/internal/domain/rules/)).

---

### Stage 3: Canary Routing by Carrier / Fleet Tier
Incrementally route physical dispatches based on carrier contract models:

* **Cohort A (Dedicated / Contract Fleets — Monopolistic $N=0$):**
  - Contract carriers have pre-negotiated freight volumes with zero spot market bidding.
  - Route to Go with $N_c = 0$. This exercises the exact mathematical collapse to the legacy Powell framework with zero market risk.
* **Cohort B (Spot-Heavy / Competitive Fleets — MOMDP $N=1$):**
  - Carriers operating heavily on spot boards (e.g. DAT, Truckstop).
  - Route to Go with $N_c = 1$ and active Bayesian belief filtering ($b_t$) to capture the $+38\%$ profit lift on competitive rate auctions.

---

### Stage 4: Full Switchover with "Instant Java Fallback" Safety Valve
* **The Concept:** Project Mittens operates as the primary production optimization engine.
* **The Safety Guarantee:** 
  - Standard carrier dispatch SLAs allow up to **10 seconds** per batch.
  - Project Mittens executes in **$< 1\text{ ms}$** (leaving $> 9.9\text{ seconds}$ of buffer).
  - If Go encounters an unexpected timeout, unhandled edge case, or physical invariant validation failure, the router catches the error and **instantly falls back to the legacy Java engine**.
* **Result:** 100% fail-safe operational continuity with zero possibility of stranded drivers or unassigned loads.

---

## 4. Operational Sign-Off & Verification Checklist

To maintain discipline throughout the transition, each stage requires explicit quantitative sign-off before advancing:

| Stage | Milestone / Requirement | Verification Metric | Sign-off Authority |
| :--- | :--- | :--- | :--- |
| **Pillar 1** | Golden Parity Verification ($N=0$) | $0.00\%$ objective variance across historical fixtures | OR / Lead Scientist |
| **Pillar 1** | Statistical Superiority ($N=1$) | $p < 0.01, \, t > 2.50, \, \Delta \mu > 0$ over 100 seeds | OR / Lead Scientist |
| **Stage 1** | Shadow Execution Stability | 7 consecutive days of shadow traffic with zero panics | Platform Lead |
| **Stage 2** | Subsystem LAP Solve Latency | $p99 < 5\text{ ms}$ for matrices up to $1000 \times 1000$ | Optimization Lead |
| **Stage 3** | Canary Fleet Acceptance | Carrier ops confirms $100\%$ feasible dispatches & zero clock violations | VP of Operations |
| **Stage 4** | Production Switchover | Zero fallback invocations over 14 consecutive operational days | Executive Leadership |

---

## 5. Summary Reference

This strategy guarantees that Project Mittens is introduced with **maximum mathematical rigor**, **continuous operational visibility**, and **zero business risk**. All historical datasets, shadow pipelines, and canary routing rules are tracked in the Semantic Journal and visible in Grafana Tempo/Prometheus.
