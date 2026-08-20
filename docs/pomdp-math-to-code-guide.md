# Project Mittens: Mathematical MOMDP Specification & Code Map
**A Python/OR-Oriented Companion to `docs/exogenous-load-pomdp-v2.pdf`**

**Status:** Authoritative Mathematical Architecture & Implementation Guide  
**Date:** 2026-08-19  
**Target Audience:** Operations Research (OR) Scientists, Simulation Specialists, Python Engineers, and Optimization Architects  

---

## 1. Executive Overview & Purpose

The goal of this document is to bridge the gap between theoretical Operations Research mathematics (as formulated in `docs/exogenous-load-pomdp-v2.pdf` and Powell’s Sequential Decision Analytics) and the production Go implementation of **Project Mittens**.

If you are an OR scientist or Python developer comfortable with NumPy, SciPy, and mathematical notations ($S_t, a_t, W_{t+1}, b_t, \Theta_t$), this guide provides a direct **1-to-1 mapping** from every formula and lemma in the paper to the exact Go structs, methods, algorithms, and lines of code in this repository.

```
┌───────────────────────────────────────────────────────────────────────────────────┐
│                      MOMDP Mathematical Formulation                               │
│              Paper: docs/exogenous-load-pomdp-v2.pdf                              │
└────────────────────────────────────────┬──────────────────────────────────────────┘
                                         │
                                         ▼
┌───────────────────────────────────────────────────────────────────────────────────┐
│                           Project Mittens Go Core                                 │
│  Domain Models (internal/domain/model) ──► Policies (internal/domain/policy)      │
│  Orchestration (internal/service)      ──► Math Kernels (pkg/math)                │
└───────────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. The Python-to-Go Rosetta Stone

Project Mittens is engineered in Go for extreme concurrent throughput, sub-millisecond solve latencies, and zero-allocation memory guarantees. Here is how core Python/NumPy concepts map to Go:

| Mathematical / Python Concept | Python Idiom (`numpy` / `scipy` / `dataclasses`) | Project Mittens Go Implementation | Code Location |
| :--- | :--- | :--- | :--- |
| **State Immutability** | `@dataclass(frozen=True)` or `copy.deepcopy()` | Value-based structs returning newly allocated pointers on `.Transition()` | [`internal/domain/model/state.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/state.go) |
| **Linear Assignment (LAP)** | `scipy.optimize.linear_sum_assignment(cost_mat)` | Exact $O(M^2 N)$ Successive Shortest Path (LAPJV) with dual shadow price extraction | [`pkg/math/lap.go`](file:///Users/jacob/Development/od/project_mittens/pkg/math/lap.go) |
| **Bayesian Belief Filter** | `b_next = (P_obs * (T @ b_prior)) / norm` | Log-space Chapman-Kolmogorov forward step + LogNormalize projection | [`internal/domain/model/belief_filter.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/belief_filter.go) |
| **Matrix Algebra & CKG** | `np.linalg.cholesky(K)` & `np.linalg.solve(L, y)` | Pure Go positive-definite Cholesky decomposition & forward/back substitution | [`pkg/math/matrix.go`](file:///Users/jacob/Development/od/project_mittens/pkg/math/matrix.go) |
| **Concurrent Tree Search** | `concurrent.futures.ProcessPoolExecutor()` | Go GMP scheduler, lightweight goroutines (`go func()`), lock-free channels, and `context.Context` | [`internal/domain/policy/dla.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/dla.go) |
| **Statistical Testing** | `scipy.stats.ttest_rel(sample_a, sample_b)` | Exact Student's paired $t$-test with Regularized Incomplete Beta function | [`pkg/math/stats.go`](file:///Users/jacob/Development/od/project_mittens/pkg/math/stats.go) |
| **Parameter Types** | `TypeVar('C', bound=CompetitorScale)` | Go Generics `[C model.CompetitorScale]` with compile-time $N=0$ monopolistic collapse | [`internal/domain/model/belief.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/belief.go) |

---

## 3. Powell’s 5 Elements of Sequential Decision Analytics: Math to Code

### 3.1 The State Variable $S_t = (R_t, I_t, b_t)$

Under Mixed-Observability MDP (MOMDP) factoring, the global state is decomposed into three distinct sub-states:

$$\mathcal{S}_t^{\text{ext}} = \Big( R_t, \, I_t, \, b_t \Big)$$

```
                        ┌─────────────────────────────────────┐
                        │      Extended State S_t^ext         │
                        │    (internal/domain/model/state.go) │
                        └───────┬──────────┬──────────┬───────┘
                                │          │          │
        ┌───────────────────────┘          │          └───────────────────────┐
        ▼                                  ▼                                  ▼
┌─────────────────────────┐    ┌─────────────────────────┐    ┌─────────────────────────┐
│   Resource State R_t    │    │  Information State I_t  │    │    Belief State b_t     │
│ (Fully Observable Fleet)│    │ (Macro Indices/Weather) │    │  (Competitor Postures)  │
│  - Active Drivers       │    │  - Current Epoch        │    │  - Aggressive: 0.15     │
│  - Unassigned Loads     │    │  - Spot Rate ($/mile)   │    │  - Moderate:   0.65     │
│  - Driver HOS Clocks    │    │  - Fuel Price ($/gal)   │    │  - Passive:    0.20     │
└─────────────────────────┘    └─────────────────────────┘    └─────────────────────────┘
```

1. **Resource State $R_t \in \mathcal{R}$ (Physical Fleet Assets):**
   - **Math:** $R_t = \Big( \{d_i\}_{i=1}^{M}, \{\ell_j\}_{j=1}^{N} \Big)$ tracking driver physical locations, domicile nodes, equipment endorsements, and FMCSA Hours-of-Service (HOS) duty clocks.
   - **Go Struct:** [`model.ResourceState`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/resource.go#L18) and [`model.Driver`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/resource.go#L36).
   - **FMCSA Regulatory Tracking:** [`hos.DriverClocks`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/hos/clocks.go#L18) tracking 11h driving, 14h on-duty, 70h/8-day multi-day cycles, and split sleeper berth credits (8h/2h or 7h/3h).

2. **Information State $I_t \in \mathcal{I}$ (Exogenous Macro Environment):**
   - **Math:** $I_t = (\tau_t, \bar{r}_t, \bar{f}_t)$ capturing decision epoch $\tau_t$, national spot rate index $\bar{r}_t$, and fuel price index $\bar{f}_t$.
   - **Go Struct:** [`model.InformationState`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/information.go#L15).

3. **Belief State $b_t \in \Delta(\mathcal{H})$ (Latent Market Postures):**
   - **Math:** Probability simplex over unobserved competitor pricing posture $\Theta_t \in \{\text{AGGRESSIVE}, \text{MODERATE}, \text{PASSIVE}\}$ where $\sum_k b_t(\Theta_k) = 1.0$.
   - **Go Struct:** [`model.Belief[C]`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/belief.go#L20).
   - **Monopolistic Collapse ($N_c = 0$):** When $C = \text{Monopolistic}$, $b_t$ collapses to an $O(1)$ Dirac delta $\delta(\Theta_\emptyset)$ via [`model.NewMonopolisticBelief()`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/belief.go#L42).

---

### 3.2 The Decision / Action Variable $a_t = (x_t, p_t)$

At every decision epoch, the optimizer computes a joint action:

$$a_t = \Big( x_t, \, p_t \Big) \in \mathcal{X}(R_t) \times \mathcal{P}$$

```
                                  ┌─────────────────────────────┐
                                  │      Joint Action a_t       │
                                  │ (internal/domain/model)     │
                                  └───────┬─────────────┬───────┘
                                          │             │
                    ┌─────────────────────┘             └─────────────────────┐
                    ▼                                                         ▼
    ┌───────────────────────────────┐                         ┌───────────────────────────────┐
    │  Matching Decisions x_t       │                         │    Spot Pricing Bids p_t      │
    │  x_{d, \ell} \in {0, 1}       │                         │    p_\ell \in \mathbb{R}^+    │
    │  - Driver A -> Load 101       │                         │    - Bid $2,450 on Load 101   │
    │  - Driver B -> Hold Idle      │                         │    - Bid $1,800 on Load 102   │
    └───────────────────────────────┘                         └───────────────────────────────┘
```

- **Matching Action $x_t$:** Binary assignment matrix $x_{d, \ell} \in \{0, 1\}$ subject to 1-to-1 matching constraints:
  $$\sum_{\ell \in \mathcal{L}_t} x_{d, \ell} \le 1, \quad \forall d \in \mathcal{D}_t \qquad \text{and} \qquad \sum_{d \in \mathcal{D}_t} x_{d, \ell} \le 1, \quad \forall \ell \in \mathcal{L}_t$$
- **Pricing Action $p_t$:** Carrier rate bid vector submitted to spot freight auctions.
- **Go Struct:** [`model.Action`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/action.go#L16), [`model.DriverLoadMatch`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/action.go#L23), and [`model.BidQuote`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/action.go#L33).

---

### 3.3 The Exogenous Information / Observation Process $W_{t+1} = (D_{t+1}, Y_{t+1})$

Between decision epochs $t$ and $t+1$, the environment emits feedback:

$$W_{t+1} = o_{t+1} = \Big( D_{t+1}, \, Y_{t+1} \Big)$$

- **New Load Arrivals $D_{t+1}$:** Realized freight demand offers broadcasted to the network.
- **Auction Feedback $Y_{t+1}$:** Win/loss outcome signals on submitted bids:
  $$Y_{t+1}(\ell) = \begin{cases} 1 & \text{if } p_t(\ell) \le p_{\text{competitor}}(\ell) \times 1.02 \quad \text{(Win)} \\ 0 & \text{if } p_t(\ell) > p_{\text{competitor}}(\ell) \times 1.02 \quad \text{(Loss)} \end{cases}$$
- **Go Implementation:** [`model.Observation`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/observation.go#L15) and [`simulation.MarketEnvironment`](file:///Users/jacob/Development/od/project_mittens/internal/adapter/simulation/market_env.go#L45).

---

### 3.4 The Transition Function $S_{t+1} = S^M(S_t, a_t, W_{t+1})$

Because of MOMDP factoring, the transition function cleanly decouples into deterministic physical movement and stochastic Bayesian belief filtering:

```
                            ┌──────────────────────────────────┐
                            │    Current State S_t = (R, I, b) │
                            └─────────────────┬────────────────┘
                                              │ Action a_t = (x_t, p_t)
                                              ▼
                ┌───────────────────────────────────────────────────────────┐
                │          Transition Function S^M(S_t, a_t, W_{t+1})       │
                └─────────────┬───────────────────────────────┬─────────────┘
                              │                               │
        Deterministic Fleet Flow                              Stochastic Bayesian Filter
        R_{t+1} = f_R(R_t, x_t, D_{t+1})                      b_{t+1} = Filter(b_t, o_{t+1})
                              │                               │
                              ▼                               ▼
                ┌───────────────────────────┐   ┌───────────────────────────┐
                │  Next Fleet State R_{t+1} │   │ Next Belief Vector b_{t+1}│
                │ (Driver positions & HOS)  │   │  (Posterior distribution) │
                └─────────────┬─────────────┘   └─────────────┬─────────────┘
                              │                               │
                              └───────────────┬───────────────┘
                                              ▼
                            ┌──────────────────────────────────┐
                            │ Next Extended State S_{t+1}      │
                            └──────────────────────────────────┘
```

#### 1. Deterministic Resource Transition $R_{t+1} = f_R(R_t, x_t, D_{t+1})$
- **Math:** Drivers paired with loads move along designated arcs:
  $$\text{Location}(d) \leftarrow \text{Destination}(\ell), \quad \text{AvailableTime}(d) \leftarrow t_{\text{unload\_end}}, \quad \text{Clocks}(d) \leftarrow \text{ForwardHOS}(d, \ell)$$
- **Go Method:** [`model.ResourceState.Transition(matches, newLoads)`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/resource.go#L110).

#### 2. Stochastic Bayesian Belief Filter $b_{t+1} = \text{Filter}(b_t, o_{t+1}, a_t)$
- **Math (Chapman-Kolmogorov + Log-Space Bayes' Rule):**
  $$b_{t+1}(\Theta_j) = \frac{P(o_{t+1} \mid \Theta_j, a_t) \sum_i b_t(\Theta_i) T_{i,j}(a_t)}{\sum_k P(o_{t+1} \mid \Theta_k, a_t) \sum_i b_t(\Theta_i) T_{i,k}(a_t)}$$
- **Log-Space Numerical Stability:** Evaluates $\ln \text{posterior}_j = \ln \bar{b}_{t+1}(j) + \ln P(o_{t+1} \mid \Theta_j)$ and applies `pkgmath.LogNormalize(logProbs)` to prevent floating-point underflow.
- **Go Method:** [`model.BeliefFilter.Filter(prior, obs, action)`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/belief_filter.go#L70).

---

## 4. Powell’s 4 Universal Policy Classes in Project Mittens

Project Mittens provides complete, mathematically verified implementations of all four Powell policy classes:

```
                                  ┌───────────────────────────────┐
                                  │   Powell's 4 Policy Classes   │
                                  └───────────────┬───────────────┘
                                                  │
         ┌───────────────────┬────────────────────┴───────────────────┬───────────────────┐
         ▼                   ▼                                        ▼                   ▼
┌─────────────────┐ ┌─────────────────┐                      ┌─────────────────┐ ┌─────────────────┐
│   Class 1: PFA  │ │   Class 2: CFA  │                      │   Class 3: VFA  │ │   Class 4: DLA  │
│  Policy Function│ │  Cost Function  │                      │  Value Function │ │ Direct Lookahead│
│  Approximation  │ │  Approximation  │                      │  Approximation  │ │  Approximation  │
│                 │ │                 │                      │                 │ │                 │
│ Declarative CEL │ │ Parametric shift│                      │ Piecewise CAVE  │ │ Multi-horizon   │
│ business rules  │ │ theta on LAP    │                      │ convex slopes   │ │ Monte Carlo tree│
└─────────────────┘ └─────────────────┘                      └─────────────────┘ └─────────────────┘
```

### 4.1 Class 1: Policy Function Approximations (PFAs) — Rule Engine
- **Math:** $X^{\text{PFA}}(S_t) = \text{MatchIf}(S_t \text{ satisfies } \text{Rules})$.
- **Implementation:** Google Common Expression Language (CEL) declarative business rules evaluated over compiled AST programs.
- **Code:** [`internal/domain/rules/registry.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/rules/registry.go) and [`context.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/rules/context.go).

---

### 4.2 Class 2: Cost Function Approximations (CFAs) — Parametric Shifts
- **Math:** Modifies the single-period matching contribution with a parameter vector $\theta = (\theta_{\text{empty}}, \theta_{\text{home}}, \theta_{\text{dwell}}, \theta_{\text{risk}})$:
  $$X^{\text{CFA}}_t(S_t \mid \theta) = \arg\max_{x \in \mathcal{X}_t} \sum_{(d, \ell) \in x} \bar{C}(d, \ell \mid \theta)$$
  $$\bar{C}(d, \ell \mid \theta) = C(d, \ell) - (\theta_{\text{empty}}-1) C_{\text{empty}} - (\theta_{\text{home}}-1) C_{\text{home}} - (\theta_{\text{dwell}}-1) C_{\text{dwell}} - \theta_{\text{risk}} \cdot \text{RiskPremium}(b_t)$$
- **Offline Optimization:** Parameter vector $\theta$ is tuned offline using **Simultaneous Perturbation Stochastic Approximation (SPSA)** via [`pkgmath.OptimizeSPSA`](file:///Users/jacob/Development/od/project_mittens/pkg/math/spsa.go#L37).
- **Code:** [`internal/domain/policy/cfa.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/cfa.go#L55).

---

### 4.3 Class 3: Value Function Approximations (VFAs) — Piecewise Concave & CKG
- **Math:** Augments immediate dispatch contribution with the discounted downstream marginal value of depositing a driver in post-decision region $r$:
  $$X^{\text{VFA}}_t(S_t) = \arg\max_{x \in \mathcal{X}_t} \sum_{(d, \ell) \in x} \Big[ C(d, \ell) + \gamma \cdot \bar{V}_t\big(R^d_{t+1}(\text{Region}(\ell))\big) \Big]$$
- **CAVE Concavity Preservation:** Enforces non-increasing marginal slopes $v_{r, 1} \ge v_{r, 2} \ge \dots \ge v_{r, K}$ via forward/backward leveling passes.
- **Correlated Knowledge Gradient (CKG):** Uses spatial Gaussian Process covariance matrices $\Sigma$ and Cholesky updates to spread observed shadow price samples across adjacent geographic regions.
- **Code:** [`internal/domain/policy/vfa_piecewise.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/vfa_piecewise.go#L237) and [`pkg/math/ckg.go`](file:///Users/jacob/Development/od/project_mittens/pkg/math/ckg.go#L20).

---

### 4.4 Class 4: Direct Lookahead Approximations (DLAs) — Tree Rollouts
- **Math:** Evaluates candidate first-stage assignments by rolling forward $K$ Monte Carlo trajectories over horizon $H$:
  $$X^{\text{DLA}}_t(S_t) = \arg\max_{x_t \in \mathcal{X}_t} \left[ C(S_t, x_t) + \mathbb{E}_{\tilde{W}} \left[ \sum_{t'=t+1}^{t+H} \gamma^{t'-t} C\big( \tilde{S}_{t'}, X^{\text{base}}(\tilde{S}_{t'}) \big) \right] \right]$$
- **Concurrent Lock-Free Parallelization:** Work is partitioned across worker goroutines via channels selecting on `ctx.Done()`, ensuring zero mutex contention and zero zombie thread leaks.
- **Code:** [`internal/domain/policy/dla.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/dla.go#L64).

---

## 5. Network Flow & Advanced Optimization Kernels

### 5.1 Exact LAPJV Linear Assignment Solver (`pkg/math/lap.go`)
- **Algorithm:** Successive Shortest Path (SSP) with Dijkstra dual potentials, guaranteeing global bipartite matching optimality in $O(\min(M, N)^2 \max(M, N))$ time.
- **Dual Multiplier Extraction:** Returns driver opportunity costs $u_d$ and load shadow prices $v_\ell$ for VFA learning and market clearing rate estimation.
- **Code:** [`pkgmath.SolveLAP(matrix, maximize, allowNegative)`](file:///Users/jacob/Development/od/project_mittens/pkg/math/lap.go#L46).

### 5.2 Multi-Leg Tour & Relay Facility Synthesis (`internal/domain/policy/`)
- **Multi-Leg Tour Synthesizer:** Chains sequential loads for a driver across multi-day planning horizons ($H \le 72\text{h}$) while forward-simulating HOS clocks and enforcing domicile returns:
  [`policy.TourSynthesizer.SynthesizeTour`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/tour_synthesizer.go#L79).
- **Dual-Driver Relay Synthesizer:** Discovers two-driver split exchanges across mid-corridor relay hubs, matching inbound/outbound legs and synchronizing trailer handoffs:
  [`policy.RelaySynthesizer.SynthesizeRelays`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/relay_synthesizer.go#L55).

---

## 6. Complete Mathematical Cross-Reference Directory

| Paper Concept / Formula | Exact Mathematical Notation | Go Type / Struct | Go File & Identifier |
| :--- | :--- | :--- | :--- |
| **Factored MOMDP State** | $S_t^{\text{ext}} = (R_t, I_t, b_t)$ | `model.State[C]` | [`internal/domain/model/state.go:18`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/state.go#L18) |
| **Physical Resource State** | $R_t \in \mathcal{R}$ | `model.ResourceState` | [`internal/domain/model/resource.go:18`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/resource.go#L18) |
| **HOS Duty Clocks** | $\text{HOS}_d = (\tau_{\text{drive}}, \tau_{\text{duty}}, \tau_{\text{cycle}})$ | `hos.DriverClocks` | [`internal/domain/model/hos/clocks.go:18`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/hos/clocks.go#L18) |
| **HOS Forward Simulator** | $\text{SimulateTrip}(d, \ell, \text{clocks})$ | `hos.Simulator` | [`internal/domain/model/hos/simulator.go:37`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/hos/simulator.go#L37) |
| **Information State** | $I_t = (\tau_t, \bar{r}_t, \bar{f}_t)$ | `model.InformationState` | [`internal/domain/model/information.go:15`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/information.go#L15) |
| **Competitive Belief Vector** | $b_t \in \Delta(\mathcal{H})$ | `model.Belief[C]` | [`internal/domain/model/belief.go:20`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/belief.go#L20) |
| **Monopolistic Collapse ($N=0$)** | $b_t = \delta(\Theta_\emptyset)$ | `model.Monopolistic` | [`internal/domain/model/belief.go:42`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/belief.go#L42) |
| **Transition Dynamics Matrix** | $T_\Gamma(\Theta_{t+1} \mid \Theta_t)$ | `model.TransitionMatrix` | [`internal/domain/model/transition_matrix.go:18`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/transition_matrix.go#L18) |
| **Bayesian Belief Filter** | $b_{t+1} = \text{Bayes}(b_t, T_\Gamma, o_{t+1})$ | `model.BeliefFilter[C]` | [`internal/domain/model/belief_filter.go:24`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/belief_filter.go#L24) |
| **Candidate Arc Filtering** | $\mathcal{A}_{\text{feas}} \subseteq \mathcal{D}_t \times \mathcal{L}_t$ | `feasibility.ConcurrentFilter` | [`internal/domain/model/feasibility/filter.go:47`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/feasibility/filter.go#L47) |
| **Trip Cost Function** | $C(d, \ell) = \text{Rev} - \text{Costs}$ | `policy.CalculateTripCost` | [`internal/domain/policy/cost.go:50`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/cost.go#L50) |
| **Exact Bipartite Matcher** | $\max \sum C(d, \ell) x_{d, \ell}$ | `policy.BipartiteMatcher` | [`internal/domain/policy/matcher.go:26`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/matcher.go#L26) |
| **Linear Assignment (LAPJV)** | $O(M^2 N)$ Network Flow | `pkgmath.SolveLAP` | [`pkg/math/lap.go:46`](file:///Users/jacob/Development/od/project_mittens/pkg/math/lap.go#L46) |
| **CFA Policy** | $\bar{C}(d, \ell \mid \theta) = C + \theta^T \phi$ | `policy.CFAPolicy[C]` | [`internal/domain/policy/cfa.go:63`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/cfa.go#L63) |
| **SPSA Gradient Optimizer** | $\hat{g}_k(\theta) = \frac{J(\theta+c\Delta) - J(\theta-c\Delta)}{2 c \Delta}$ | `pkgmath.OptimizeSPSA` | [`pkg/math/spsa.go:37`](file:///Users/jacob/Development/od/project_mittens/pkg/math/spsa.go#L37) |
| **Piecewise VFA (CAVE)** | $v_{r, k} = (1-\alpha)v_k + \alpha \hat{v}$ | `policy.PiecewiseLinearVFATable` | [`internal/domain/policy/vfa_piecewise.go:32`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/vfa_piecewise.go#L32) |
| **Correlated Knowledge Gradient** | $\mathbb{E}[\Delta V \mid \Sigma]$ | `pkgmath.CorrelatedKnowledgeGradient` | [`pkg/math/ckg.go:20`](file:///Users/jacob/Development/od/project_mittens/pkg/math/ckg.go#L20) |
| **Direct Lookahead (DLA)** | $C(S_t, x_t) + \sum \gamma^k C(S_{t+k})$ | `policy.DLAPolicy[C]` | [`internal/domain/policy/dla.go:64`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/dla.go#L64) |
| **Multi-Leg Tour Synthesis** | Chained tours $L_1 \to L_2 \to \text{Home}$ | `policy.TourSynthesizer` | [`internal/domain/policy/tour_synthesizer.go:51`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/tour_synthesizer.go#L51) |
| **Relay Facility Synthesis** | Dual-driver hub exchange | `policy.RelaySynthesizer` | [`internal/domain/policy/relay_synthesizer.go:23`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/relay_synthesizer.go#L23) |
| **Service Orchestrator** | Epoch optimization & Journaling | `service.OptimizationService[C]` | [`internal/service/orchestrator.go:19`](file:///Users/jacob/Development/od/project_mittens/internal/service/orchestrator.go#L19) |
| **Rolling Horizon Simulation** | Multi-day simulation loop | `service.RollingHorizonRunner[C]` | [`internal/service/rolling_horizon.go:99`](file:///Users/jacob/Development/od/project_mittens/internal/service/rolling_horizon.go#L99) |
| **Decision Audit Provenance** | Complete mathematical trail | `policy.DecisionProvenance` | [`internal/domain/policy/provenance.go:16`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/provenance.go#L16) |
| **Student's t-test** | $t = \frac{\bar{d} \sqrt{n}}{s_d}, \, p = I_x(a, b)$ | `pkgmath.PairedTTest` | [`pkg/math/stats.go:22`](file:///Users/jacob/Development/od/project_mittens/pkg/math/stats.go#L22) |
