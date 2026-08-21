# Powell Equation-to-Code Traceability Matrix

**Author:** Project Mittens Engineering Team  
**Reference Text:** Powell, W. B. (2022). *Reinforcement Learning and Stochastic Optimization: A unified framework for sequential decisions*. John Wiley & Sons.  
**Purpose:** Audit-grade cross-reference linking canonical equations in Powell (2022) to their exact mathematical instantiation and typed Go symbols in Project Mittens.

---

## 1. State Variables, Post-Decision States & Transitions (Chapters 3 & 4)

| Powell (2022) Equation | Canonical SDA Formulation | Project Mittens Domain Instantiation | Repository Go Type & Method | File Location |
| :--- | :--- | :--- | :--- | :--- |
| **Eq. (3.1)** | $S_t = (R_t, I_t, B_t)$ | $S_t = (R_t, I_t, b_t) \in \mathcal{R} \times \mathcal{I} \times \Delta(\mathcal{H}_N)$ | `model.State[C]`<br>`State.Resource()`, `State.Information()`, `State.Belief()` | [`internal/domain/model/state.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/state.go) |
| **Eq. (3.5)** | $R_t = (R_{t, i})_{i \in \mathcal{I}}$ | $R_t = (\text{Drivers}_t, \text{Loads}_t)$ with HOS clocks | `model.ResourceState`<br>`ResourceState.Drivers()`, `ResourceState.Loads()` | [`internal/domain/model/resource.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/resource.go) |
| **Eq. (4.3)** | $S_t \xrightarrow{x_t} S_t^x \xrightarrow{W_{t+1}} S_{t+1}$ | $S_t \xrightarrow{a_t} S_t^a = (R_t^x, I_t, b_t^a) \xrightarrow{W_{t+1}} S_{t+1}$ | `State.Transition(action, newLoads)`<br>`ResourceState.Transition()` | [`internal/domain/model/state.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/state.go)<br>[`internal/domain/model/resource.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/resource.go) |
| **Eq. (4.7)** | $R_t^x = R_t + \Delta R(x_t)$ | Driver state advance after assignment committing | `model.DriverLoadMatch`<br>`Action.Matches()` | [`internal/domain/model/action.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/action.go) |
| **Eq. (4.12)** | $B_{t+1} = B^M(B_t, x_t, W_{t+1})$ | $b_{t+1}(\theta') \propto \Pr(O_{t+1} \mid \theta', p_t) \sum_\theta T(\theta, \theta') b_t(\theta)$ | `model.CompetitiveBeliefFilter[C]`<br>`BeliefFilter.Filter(prior, obs, action)` | [`internal/domain/model/belief_filter.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/belief_filter.go) |

---

## 2. Policy Function Approximations (Chapter 12)

| Powell (2022) Equation | Canonical SDA Formulation | Project Mittens Domain Instantiation | Repository Go Type & Method | File Location |
| :--- | :--- | :--- | :--- | :--- |
| **Eq. (12.1)** | $X^{\text{PFA}}(S_t \mid \theta) = f_\theta(S_t)$ | Constructive Greedy Priority Dispatch Rule (No embedded LAP) | `policy.PFAPolicy[C]`<br>`PFAPolicy.Evaluate()` | [`internal/domain/policy/pfa.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/pfa.go) |
| **Eq. (12.3)** | $\theta = (\theta_1, \dots, \theta_P)$ | $\theta = (\text{MaxDeadhead}, \text{DeadheadWeight}, \text{DwellWeight}, \text{RevWeight})$ | `policy.PFAParameters`<br>`DefaultPFAParameters()` | [`internal/domain/policy/pfa.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/pfa.go) |

---

## 3. Cost Function Approximations & SPSA Tuning (Chapter 13)

| Powell (2022) Equation | Canonical SDA Formulation | Project Mittens Domain Instantiation | Repository Go Type & Method | File Location |
| :--- | :--- | :--- | :--- | :--- |
| **Eq. (13.1)** | $X^{\text{CFA}}(S_t \mid \theta) = \arg\max_{x \in \mathcal{X}_t} \left( C(S_t, x) + \sum_f \theta_f \phi_f(S_t, x) \right)$ | Parametric Cost Modification Matching | `policy.CFAPolicy[C]`<br>`CFAPolicy.Evaluate()` | [`internal/domain/policy/cfa.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/cfa.go) |
| **Eq. (13.4)** | $\phi(S_t, x) = (f_1(S_t, x), \dots, f_F(S_t, x))$ | $\phi = (\text{EmptyMiles}, \text{DistToHome}, \text{DwellHours}, \text{RiskPenalty})$ | `policy.CFAParameters`<br>`ThetaEmpty`, `ThetaHome`, `ThetaDwell`, `ThetaRisk` | [`internal/domain/policy/cfa.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/cfa.go) |
| **Eq. (13.7)** | $X^{\text{Myopic}}(S_t) = \arg\max_{x \in \mathcal{X}_t} C(S_t, x)$ | Myopic Base Model ($X^{\text{CFA}}(S_t \mid 0)$) | `policy.CFAPolicy[C]` (with $\theta = 0$) | [`internal/domain/policy/cfa.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/cfa.go) |
| **Eq. (13.12)** | $\hat{g}_k(\theta_k) = \frac{J(\theta_k + c_k \Delta_k) - J(\theta_k - c_k \Delta_k)}{2 c_k} \Delta_k^{-1}$ | SPSA Simultaneous Perturbation Pseudo-Gradient | `pkgmath.SPSAOptimizer`<br>`SPSAOptimizer.Step()`, `SPSAOptimizer.Optimize()` | [`pkg/math/spsa.go`](file:///Users/jacob/Development/od/project_mittens/pkg/math/spsa.go) |
| **Eq. (13.14)** | $a_k = \frac{a}{(k + 1 + A)^\alpha}, \quad c_k = \frac{c}{(k + 1)^\gamma}$ | Harmonic Stepsize & Perturbation Sequences | `pkgmath.SPSAConfig`<br>`StepSizeAlpha`, `PerturbationC`, `HarmonicA` | [`pkg/math/spsa.go`](file:///Users/jacob/Development/od/project_mittens/pkg/math/spsa.go) |

---

## 4. Value Function Approximations & Concave Adaptive Estimation (Chapters 14–18)

| Powell (2022) Equation | Canonical SDA Formulation | Project Mittens Domain Instantiation | Repository Go Type & Method | File Location |
| :--- | :--- | :--- | :--- | :--- |
| **Eq. (14.3)** | $X^{\text{VFA}}(S_t) = \arg\max_{x \in \mathcal{X}_t} \left( C(S_t, x) + \gamma \bar{V}_t(S_t^x) \right)$ | Post-Decision Value Function Matching | `policy.PiecewiseVFAPolicy`<br>`PiecewiseVFAPolicy.Evaluate()` | [`internal/domain/policy/vfa_piecewise.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/vfa_piecewise.go) |
| **Eq. (14.6)** | $\bar{V}_t(R_t^x) = \sum_{r \in \mathcal{R}} \bar{V}_{t, r}(R_{t, r}^x)$ | Spatially Separable Regional Value Decomposition | `policy.PiecewiseLinearVFATable`<br>`PiecewiseLinearVFATable.RegionalSlopes` | [`internal/domain/policy/vfa_piecewise.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/vfa_piecewise.go) |
| **Eq. (14.7)** | $\bar{V}_{t, r}(n) = \sum_{k=1}^n \bar{v}_{t, r}(k)$ | Total Regional Value from Accumulated Slopes | `policy.RegionSlopes`<br>`RegionSlopes.TotalValue(n)` | [`internal/domain/policy/vfa_piecewise.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/vfa_piecewise.go) |
| **Eq. (14.9)** | $\bar{v}_r(1) \ge \bar{v}_r(2) \ge \dots \ge \bar{v}_r(K)$ | Concave Non-Increasing Piecewise Marginal Slopes | `policy.RegionSlopes`<br>`RegionSlopes.MarginalValue(k)`, `NewRegionSlopes()` | [`internal/domain/policy/vfa_piecewise.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/vfa_piecewise.go) |
| **Eq. (14.18)** | $\hat{v}_{t, d} = u_d^*$ where $v_{\text{slack}} = 0$ | Normalized Supergradient Dual Potentials | `pkgmath.SolveLAPDual`<br>`LAPJVResult.U`, `LAPJVResult.V` | [`pkg/math/lap.go`](file:///Users/jacob/Development/od/project_mittens/pkg/math/lap.go) |
| **Eq. (14.22)** | $\bar{v}_{t, r}^{(n+1)}(k) = (1 - \alpha_n) \bar{v}_{t, r}^{(n)}(k) + \alpha_n \hat{v}_{t, r}^{(n)}$ | CAVE Adaptive Value Smoothing & Leveling | `service.PiecewiseVFALearner`<br>`PiecewiseVFALearner.UpdateFromDuals()` | [`internal/service/vfa_learner.go`](file:///Users/jacob/Development/od/project_mittens/internal/service/vfa_learner.go) |

---

## 5. Direct Lookahead Approximations & Rolling Horizons (Chapters 19 & 20)

| Powell (2022) Equation | Canonical SDA Formulation | Project Mittens Domain Instantiation | Repository Go Type & Method | File Location |
| :--- | :--- | :--- | :--- | :--- |
| **Eq. (19.4)** | $X^{\text{DLA}}(S_t) = \arg\max_{x_t \in \mathcal{X}_t} \left( C(S_t, x_t) + \tilde{V}_t(S_t^{x_t}) \right)$ | Truncated-Horizon Direct Lookahead Policy | `policy.DLAPolicy`<br>`DLAPolicy.Evaluate()` | [`internal/domain/policy/dla.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/dla.go) |
| **Eq. (20.8)** | $\tilde{V}_t(S_t^{x_t}) = \mathbb{E}_{\tilde{W}} \left[ \sum_{t'=t+1}^{t+H} \gamma^{t'-t} C(\tilde{S}_{t'}, \tilde{X}(\tilde{S}_{t'})) \right]$ | Forward Monte Carlo Trajectory Rollouts | `policy.DLAPolicy`<br>`DLAPolicy.evaluateBranchRollouts()` | [`internal/domain/policy/dla.go`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/dla.go) |
| **Eq. (20.14)** | $X^{\text{DLA+VFA}}(S_t) = \arg\max_{x_t} \left( C(S_t, x_t) + \tilde{C}_{t:t+H} + \gamma^H \bar{V}_{t+H}(\tilde{S}_{t+H}^x) \right)$ | Hybrid Lookahead with Terminal Boundary Value | `service.RollingHorizonRunner`<br>`RollingHorizonRunner.Run()` | [`internal/service/rolling_horizon.go`](file:///Users/jacob/Development/od/project_mittens/internal/service/rolling_horizon.go) |

---

## 6. Correlated Knowledge Gradient for Spatial Learning (Chapter 7)

| Powell (2022) Equation | Canonical SDA Formulation | Project Mittens Domain Instantiation | Repository Go Type & Method | File Location |
| :--- | :--- | :--- | :--- | :--- |
| **Eq. (7.14)** | $\mu_{t+1} = \mu_t + \frac{\hat{y}_{t+1} - \mu_{t, x}}{\sigma_{\epsilon}^2 + \Sigma_{t, xx}} \Sigma_t e_x$ | Correlated Normal Bayesian Mean Update | `pkgmath.CorrelatedKnowledgeGradient`<br>`CKG.UpdateObservation()` | [`pkg/math/ckg.go`](file:///Users/jacob/Development/od/project_mittens/pkg/math/ckg.go) |
| **Eq. (7.15)** | $\Sigma_{t+1} = \Sigma_t - \frac{\Sigma_t e_x (\Sigma_t e_x)^T}{\sigma_{\epsilon}^2 + \Sigma_{t, xx}}$ | Correlated Covariance Posterior Update | `pkgmath.CorrelatedKnowledgeGradient`<br>`CKG.UpdateObservation()` | [`pkg/math/ckg.go`](file:///Users/jacob/Development/od/project_mittens/pkg/math/ckg.go) |
| **Eq. (7.18)** | $\nu_t^{\text{KG}}(x) = \mathbb{E} \left[ \max_{x'} \mu_{t+1}(x') - \max_{x'} \mu_t(x') \;\middle|\; S_t \right]$ | Correlated Knowledge Gradient Acquisition Value | `pkgmath.CorrelatedKnowledgeGradient`<br>`CKG.KnowledgeGradient()` | [`pkg/math/ckg.go`](file:///Users/jacob/Development/od/project_mittens/pkg/math/ckg.go) |

---

## 7. Execution Verification & Benchmark Commands

To audit or reproduce any equation mapping:

```bash
# 1. Run full 4-way policy benchmark (PFA vs CFA vs VFA vs DLA vs Competitive POMDP)
go run ./cmd/tournament/main.go -mode 4way -episodes 15 -days 7

# 2. Run 2x2 factorial decomposition (V00, V01, V10, V11)
go run ./cmd/tournament/main.go -mode factorial -episodes 25 -days 7

# 3. Verify concurrent safety (the Go race detector reported no data races on the executed test suite)
go test -race -short ./pkg/math/... ./internal/domain/policy/... ./internal/service/...
```
