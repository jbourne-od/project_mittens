# Powell Equation-to-Code Traceability Matrix

**Author:** Project Mittens Engineering Team  
**Reference Text:** Powell, W. B. (2022). *Reinforcement Learning and Stochastic Optimization: A unified framework for sequential decisions*. John Wiley & Sons.  
**Purpose:** Formal cross-reference linking mathematical equations in Powell (2022) to typed Go implementations in Project Mittens.

---

## 1. State Variables, Post-Decision States & Transitions (Chapters 3 & 4)

| RLSO (2022) Equation | Mathematical Formulation | Mathematical Concept | Project Mittens Go Implementation | Code Location & Primary Symbol |
| :--- | :--- | :--- | :--- | :--- |
| **Eq. (3.1)** | $S_t = (R_t, I_t, B_t)$ | Factored State Variable (Resource, Information, Belief) | [`model.State[C]`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/state.go) | `State.Resource()`, `State.Information()`, `State.Belief()` |
| **Eq. (3.5)** | $R_t = (R_{t, i})_{i \in \mathcal{I}}$ | Multi-dimensional Resource State vector | [`model.ResourceState`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/resource.go) | `ResourceState.Drivers()`, `ResourceState.Loads()` |
| **Eq. (4.3)** | $S_t \xrightarrow{x_t} S_t^x \xrightarrow{W_{t+1}} S_{t+1}$ | Two-Step State Transition via Post-Decision State | [`State.Transition`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/state.go) | `ResourceState.Transition()` in `resource.go` |
| **Eq. (4.7)** | $R_t^x = R_t + \Delta R(x_t)$ | Post-Decision Resource Allocation State | [`model.DriverLoadMatch`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/resource.go) | `Action.Matches()`, `ResourceState.Transition()` |
| **Eq. (4.12)** | $B_{t+1} = B^M(B_t, x_t, W_{t+1})$ | Recursive Bayesian Belief State Update | [`model.CompetitiveBeliefFilter`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/belief_filter.go) | `BeliefFilter.Filter(prior, obs, action)` |

---

## 2. Cost Function Approximations & SPSA Parameter Tuning (Chapter 9)

| RLSO (2022) Equation | Mathematical Formulation | Mathematical Concept | Project Mittens Go Implementation | Code Location & Primary Symbol |
| :--- | :--- | :--- | :--- | :--- |
| **Eq. (9.1)** | $X^{CFA}(S_t \mid \theta) = \arg\max_{x \in \mathcal{X}_t} \left( C(S_t, x) + \sum_f \theta_f \phi_f(S_t, x) \right)$ | Cost Function Approximation Policy | [`policy.CFAPolicy`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/cfa.go) | `CFAPolicy.Evaluate()`, `CFAParameters` |
| **Eq. (9.4)** | $\phi_f(S_t, x) = (\text{EmptyMiles}, \text{HomeDist}, \text{DwellTime}, \text{Risk})$ | Parametric Basis Functions | [`policy.CFAParameters`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/cfa.go) | `ThetaEmpty`, `ThetaHome`, `ThetaDwell`, `ThetaRisk` |
| **Eq. (9.12)** | $\hat{g}_k(\theta_k) = \frac{J(\theta_k + c_k \Delta_k) - J(\theta_k - c_k \Delta_k)}{2 c_k} \Delta_k^{-1}$ | SPSA Simultaneous Perturbation Pseudo-Gradient | [`pkgmath.SPSAOptimizer`](file:///Users/jacob/Development/od/project_mittens/pkg/math/spsa.go) | `SPSAOptimizer.Step()`, `SPSAOptimizer.Optimize()` |
| **Eq. (9.14)** | $a_k = \frac{a}{(k + 1 + A)^\alpha}, \quad c_k = \frac{c}{(k + 1)^\gamma}$ | Harmonic Stepsize & Perturbation Sequences | [`pkgmath.SPSAConfig`](file:///Users/jacob/Development/od/project_mittens/pkg/math/spsa.go) | `StepSizeAlpha`, `PerturbationC`, `HarmonicA` |
| **Eq. (9.16)** | $\theta_{k+1} = \Pi_\Theta \left[ \theta_k + a_k \hat{g}_k(\theta_k) \right]$ | Projected Stochastic Gradient Step | [`pkgmath.SPSAOptimizer`](file:///Users/jacob/Development/od/project_mittens/pkg/math/spsa.go) | `SPSAOptimizer.Step()` (box bounds projection) |

---

## 3. Value Function Approximations & CAVE Concavity (Chapters 10 & 14)

| RLSO (2022) Equation | Mathematical Formulation | Mathematical Concept | Project Mittens Go Implementation | Code Location & Primary Symbol |
| :--- | :--- | :--- | :--- | :--- |
| **Eq. (10.3)** | $X^{VFA}(S_t) = \arg\max_{x \in \mathcal{X}_t} \left( C(S_t, x) + \gamma \bar{V}_t(S_t^x) \right)$ | Post-Decision Value Function Policy | [`policy.PiecewiseVFAPolicy`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/vfa_piecewise.go) | `PiecewiseVFAPolicy.Evaluate()` |
| **Eq. (14.6)** | $\bar{V}_t(R_t^x) = \sum_{r \in \mathcal{R}} \bar{V}_{t, r}(R_{t, r}^x)$ | Separable Spatial Value Function Decomposition | [`policy.PiecewiseLinearVFATable`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/vfa_piecewise.go) | `PiecewiseLinearVFATable.RegionalSlopes` |
| **Eq. (14.9)** | $\bar{v}_r(1) \ge \bar{v}_r(2) \ge \dots \ge \bar{v}_r(K)$ | Concave Non-Increasing Piecewise Marginal Slopes | [`policy.RegionSlopes`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/vfa_piecewise.go) | `RegionSlopes.MarginalValue()`, `NewRegionSlopes()` |
| **Eq. (14.18)** | $\hat{v}_{t, d} = u_d^*$ where $u_d^* = \text{DualMultiplier}(\text{LAP})$ | Optimal LAP Dual Potentials (Subgradients) | [`pkgmath.SolveLAPDual`](file:///Users/jacob/Development/od/project_mittens/pkg/math/lap.go) | `LAPJVResult.U`, `LAPJVResult.V` |
| **Eq. (14.22)** | $\bar{v}_{t, r}^{(n+1)}(k) = (1 - \alpha_n) \bar{v}_{t, r}^{(n)}(k) + \alpha_n \hat{v}_{t, r}^{(n)}$ | CAVE Adaptive Value Smoothing & Leveling | [`service.PiecewiseVFALearner`](file:///Users/jacob/Development/od/project_mittens/internal/service/vfa_learner.go) | `PiecewiseVFALearner.UpdateFromDuals()` |

---

## 4. Direct Lookahead Approximations (DLAs) & Rolling Horizons (Chapter 20)

| RLSO (2022) Equation | Mathematical Formulation | Mathematical Concept | Project Mittens Go Implementation | Code Location & Primary Symbol |
| :--- | :--- | :--- | :--- | :--- |
| **Eq. (20.4)** | $X^{DLA}(S_t) = \arg\max_{x_t \in \mathcal{X}_t} \left( C(S_t, x_t) + \tilde{V}_t(S_t^{x_t}) \right)$ | Direct Lookahead Policy | [`policy.DLAPolicy`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/dla.go) | `DLAPolicy.Evaluate()` |
| **Eq. (20.8)** | $\tilde{V}_t(S_t^{x_t}) = \mathbb{E}_{\tilde{W}} \left[ \sum_{t'=t+1}^{t+H} C(\tilde{S}_{t'}, \tilde{X}(\tilde{S}_{t'})) \right]$ | Truncated-Horizon Monte Carlo Rollout | [`policy.DLAPolicy`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/dla.go) | `DLAPolicy.evaluateCandidateRollout()` |
| **Eq. (20.14)** | $X^{DLA+VFA}(S_t) = \arg\max_{x_t} \left( C(S_t, x_t) + \tilde{C}_{t:t+H} + \gamma^H \bar{V}_{t+H}(\tilde{S}_{t+H}^x) \right)$ | Hybrid Lookahead with Terminal Value Function Tail | [`service.RollingHorizonRunner`](file:///Users/jacob/Development/od/project_mittens/internal/service/rolling_horizon.go) | `RollingHorizonRunner.Run()` |

---

## 5. Correlated Knowledge Gradient for Spatial Learning (Chapter 7)

| RLSO (2022) Equation | Mathematical Formulation | Mathematical Concept | Project Mittens Go Implementation | Code Location & Primary Symbol |
| :--- | :--- | :--- | :--- | :--- |
| **Eq. (7.14)** | $\mu_{t+1} = \mu_t + \frac{\hat{y}_{t+1} - \mu_{t, x}}{\sigma_{\epsilon}^2 + \Sigma_{t, xx}} \Sigma_t e_x$ | Correlated Normal Bayesian Mean Update | [`pkgmath.CorrelatedKnowledgeGradient`](file:///Users/jacob/Development/od/project_mittens/pkg/math/ckg.go) | `CKG.UpdateObservation()` |
| **Eq. (7.15)** | $\Sigma_{t+1} = \Sigma_t - \frac{\Sigma_t e_x (\Sigma_t e_x)^T}{\sigma_{\epsilon}^2 + \Sigma_{t, xx}}$ | Correlated Covariance Posterior Update | [`pkgmath.CorrelatedKnowledgeGradient`](file:///Users/jacob/Development/od/project_mittens/pkg/math/ckg.go) | `CKG.UpdateObservation()` |
| **Eq. (7.18)** | $\nu_t^{KG}(x) = \mathbb{E} \left[ \max_{x'} \mu_{t+1}(x') - \max_{x'} \mu_t(x') \;\middle|\; S_t \right]$ | Correlated Knowledge Gradient Acquisition Value | [`pkgmath.CorrelatedKnowledgeGradient`](file:///Users/jacob/Development/od/project_mittens/pkg/math/ckg.go) | `CKG.KnowledgeGradient()` |

---

## 6. Exact Jonker-Volgenant LAP Solver (Combinatorial Core)

| RLSO (2022) Reference | Mathematical Formulation | Optimization Concept | Project Mittens Go Implementation | Code Location & Primary Symbol |
| :--- | :--- | :--- | :--- | :--- |
| **Section 14.3.2** | $\min \sum_{i, j} c_{ij} x_{ij} \quad \text{s.t.} \quad \sum_j x_{ij} = 1, \sum_i x_{ij} = 1$ | Bipartite Linear Assignment Problem | [`pkgmath.SolveLAP`](file:///Users/jacob/Development/od/project_mittens/pkg/math/lap.go) | `SolveLAP()`, `SolveLAPDual()` |
| **Section 14.3.3** | $u_i + v_j \le c_{ij}, \quad u_i + v_j = c_{ij} \text{ if } x_{ij} = 1$ | Complementary Slackness & Exact Dual Multipliers | [`pkgmath.LAPJVResult`](file:///Users/jacob/Development/od/project_mittens/pkg/math/lap.go) | `LAPJVResult.U`, `LAPJVResult.V` |

---

## 7. MOMDP Latent Competitor Dynamics (The Mittens Generalization)

| Mathematical Domain | Formal Equation | Theoretical Concept | Project Mittens Go Implementation | Code Location & Primary Symbol |
| :--- | :--- | :--- | :--- | :--- |
| **Belief Simplex** | $b_t \in \Delta(\mathcal{H}) = \{ b \in \mathbb{R}_+^K \mid \sum_{k=1}^K b_k = 1.0 \}$ | Belief State over Latent Competitor Postures $\Theta_t$ | [`model.Belief[C]`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/belief.go) | `Belief.Probabilities()`, `Belief.LatentStates()` |
| **Censored Filtering** | $b_{t+1}(\theta') \propto \Pr(O_{t+1} \mid \theta', p_t) \sum_\theta P(\theta, \theta') b_t(\theta)$ | Recursive Bayes Filter under Censored Auction Observations | [`model.CompetitiveBeliefFilter`](file:///Users/jacob/Development/od/project_mittens/internal/domain/model/belief_filter.go) | `CompetitiveBeliefFilter.Filter()` |
| **Joint Policy** | $X^{\text{POMDP}}(S_t) = \arg\max_{x \in \mathcal{X}_t, p \in \mathcal{P}_t} \left[ C(S_t, x, p) + \mathbb{E}_{\Theta}[V(S_t^x \mid b_t)] \right]$ | Joint Fleet Assignment and Endogenous Spot Pricing Policy | [`policy.CompetitivePOMDPPolicy`](file:///Users/jacob/Development/od/project_mittens/internal/domain/policy/competitive.go) | `CompetitivePOMDPPolicy.Evaluate()` |
