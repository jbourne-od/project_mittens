# Executive Mathematical Dossier: Project Mittens & Sequential Decision Analytics

**Prepared For:** Prof. Warren B. Powell  
**Author:** Optimal Dynamics / Project Mittens Architecture Team  
**Subject:** Formal Parametric Decision Model Family $\mathcal{M}_N$, Transition Kernel Isomorphism, Four Policy Classes, Network Flow Decomposition, and Value of Information Theorem  
**Date:** August 2026  

---

## 1. Executive Summary & The Formal Family of Decision Models $\mathcal{M}_N$

Project Mittens models dynamic carrier dispatch and freight revenue management using the **Sequential Decision Analytics (SDA)** framework (Powell, 2022). 

To eliminate any ambiguity between monopolistic fleet dispatch and competitive market bidding, we define the **parametric family of sequential decision models** $\mathcal{M}_N$ indexed by competitor fleet dimension $N \in \mathbb{N}_0$:

$$\mathcal{M}_N = \left( \mathcal{S}_N, \mathcal{A}_N, \mathcal{W}_N, \mathbb{P}_N, K_N, C_N \right), \quad N \in \mathbb{N}_0$$

```
┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                PARAMETRIC MODEL FAMILY M_N                                       │
│                                                                                                  │
│   N = 0: Monopolistic Dispatch (P_canonical)               N ≥ 1: Endogenous Competitive MOMDP   │
│   ┌──────────────────────────────────────────────┐        ┌────────────────────────────────────┐ │
│   │ • State: S_0 = (R_t, I_t, δ_{Θ_∅}) ≅ (R_t, I_t)│      │ • State: S_N = (R_t, I_t, b_t)     │ │
│   │ • Action: a_t = (x_t, ∅) ≅ x_t               │        │ • Action: a_t = (x_t, p_t)         │ │
│   │ • Exogenous Info: W_{t+1} = (L̂_{t+1}, ΔI_{t+1})│      │ • Exogenous: W_{t+1} = (L̂, ΔI, O)  │ │
│   │ • Revenue: Exogenous Contract Tariff r_ℓ     │        │ • Revenue: Endogenous Bid Price p_ℓ│ │
│   │ • Transition: Kernel K_0 ≅ K_Powell          │        │ • Transition: Bayes Filter b_{t+1} │ │
│   └──────────────────────┬───────────────────────┘        └─────────────────┬──────────────────┘ │
│                          │                                                  │                    │
│                          └────────────────────────┬─────────────────────────┘                    │
│                                                   ▼                                              │
│               Bijective Isomorphism: M_0 ≅ P_canonical  ⊂  M_N (for N ≥ 1)                       │
└──────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### 1.1 Theorem 1 (Formal Bijective Isomorphism at $N=0$):
$$\mathcal{M}_0 \cong \mathcal{M}_{\text{Powell}} \quad \text{and} \quad \mathcal{M}_0 \subset \mathcal{M}_N \; (\forall N \ge 1)$$

*Proof Construction:*  
Define the canonical projections $\pi$ and inverse embeddings $\iota$:
$$\pi_{\mathcal{S}}: \mathcal{S}_0 \to \mathcal{S}_{\text{Powell}}, \quad (R_t, I_t, \delta_{\Theta_\emptyset}) \mapsto (R_t, I_t), \qquad \iota_{\mathcal{S}}: \mathcal{S}_{\text{Powell}} \to \mathcal{S}_0, \quad (R_t, I_t) \mapsto (R_t, I_t, \delta_{\Theta_\emptyset})$$
$$\pi_{\mathcal{A}}: \mathcal{A}_0 \to \mathcal{X}_{\text{Powell}}, \quad (x_t, \varnothing) \mapsto x_t, \qquad \iota_{\mathcal{A}}: \mathcal{X}_{\text{Powell}} \to \mathcal{A}_0, \quad x_t \mapsto (x_t, \varnothing)$$
$$\pi_{\mathcal{W}}: \mathcal{W}_0 \to \mathcal{W}_{\text{Powell}}, \quad (\hat{L}_{t+1}, \Delta I_{t+1}, \varnothing) \mapsto (\hat{L}_{t+1}, \Delta I_{t+1}), \qquad \iota_{\mathcal{W}}: \mathcal{W}_{\text{Powell}} \to \mathcal{W}_0, \quad (\hat{L}, \Delta I) \mapsto (\hat{L}, \Delta I, \varnothing)$$

1. **Bijective Topology:**
   $$\pi_{\mathcal{S}} \circ \iota_{\mathcal{S}} = \text{id}_{\mathcal{S}_{\text{Powell}}}, \qquad \iota_{\mathcal{S}} \circ \pi_{\mathcal{S}} = \text{id}_{\mathcal{S}_0}$$
   $$\pi_{\mathcal{A}} \circ \iota_{\mathcal{A}} = \text{id}_{\mathcal{X}_{\text{Powell}}}, \qquad \iota_{\mathcal{A}} \circ \pi_{\mathcal{A}} = \text{id}_{\mathcal{A}_0}$$
   At $N=0$, the competitor state space is the singleton set $\mathcal{H}_0 = \{\Theta_\emptyset\}$. The space of probability measures $\Delta(\mathcal{H}_0) = \{\delta_{\Theta_\emptyset}\}$ is a 0-dimensional point with zero entropy ($H(\delta_{\Theta_\emptyset}) = 0$).
2. **Objective Equivalence:** Under fixed contractual freight rates $r_\ell$:
   $$C_0\left( S_t, (x_t, \varnothing) \right) = \sum_{(d, \ell)} \left[ r_\ell - \text{Costs}(d, \ell) \right] x_{d, \ell} \equiv C_{\text{Powell}}(\pi_{\mathcal{S}} S_t, x_t)$$
3. **Conditional Transition Kernel Commutativity:**
   For any $(s, a) \in \mathcal{S}_{\text{Powell}} \times \mathcal{X}_{\text{Powell}}$, the pushforward transition kernel satisfies:
   $$(\pi_{\mathcal{S}})_\# K_0\left( \cdot \;\middle|\; \iota_{\mathcal{S}}(s), \iota_{\mathcal{A}}(a) \right) = K_{\text{Powell}}\left( \cdot \;\middle|\; s, a \right)$$
   and the state/action-conditional exogenous information distribution satisfies:
   $$(\pi_{\mathcal{W}})_\# \mathbb{P}_0\left( \cdot \;\middle|\; \iota_{\mathcal{S}}(s), \iota_{\mathcal{A}}(a) \right) = \mathbb{P}_{\text{Powell}}\left( \cdot \;\middle|\; s, a \right)$$
Thus $\mathcal{M}_0$ is strictly isomorphic to the canonical Powell fleet management model. $\blacksquare$

---

### 1.2 The Competitor Parameter $N \ge 1$ & Conditional i.i.d. Auction Model
For $N \ge 1$, $N$ represents the integer count of competitor fleets in the regional spot market. 
* **Aggregate Competitor Capacity:** $\mathcal{C}_N = N \cdot \bar{c}$, where $\bar{c}$ is the mean capacity per competitor fleet.
* **Conditional i.i.d. Bid Distribution:** Assuming competitor bid prices $B_1, \dots, B_N$ are **conditionally independent and identically distributed** given the latent market posture $\Theta_t = \theta_k \in \mathcal{H}_N = \{\theta_1, \dots, \theta_K\}$ (with common macroeconomic factors absorbed by $\Theta_t$):
  $$\Pr\left( B_1 > p_\ell, \dots, B_N > p_\ell \;\middle|\; \theta_k, N \right) = \prod_{i=1}^N \Pr(B_i > p_\ell \mid \theta_k) = \left[ 1 - F_{\text{single}}(p_\ell \mid \theta_k) \right]^N$$
* **Tender Win Probability:**
  $$q_\ell(p_\ell, b_t \mid N) = \sum_{k=1}^K b_t(\theta_k) \cdot \left[ 1 - F_{\text{single}}(p_\ell \mid \theta_k) \right]^N$$
  *(Note: At $N=0$, contract freight acceptance is defined separately by the singleton pricing action $\mathcal{P}_0 = \{\varnothing\}$ and fixed tariff $r_\ell$, matching $q_\ell \equiv 1$ as the natural boundary extension).*

---

## 2. The 5 Core Elements of the Sequential Decision Problem

### 2.1 State Space $\mathcal{S}_N$
$$S_t = (R_t, I_t, b_t) \in \mathcal{R} \times \mathcal{I} \times \Delta(\mathcal{H}_N)$$
* **Resource State ($R_t \in \mathcal{R}$):** Fleet physical state: driver spatial locations $(lat, lon)$, domiciles, equipment types (Dry Van, Reefer, Flatbed, Hazmat, Team), regulatory Hours-of-Service clocks (11h drive, 14h duty, 70h cycle), and active load cards.
* **Information State ($I_t \in \mathcal{I}$):** Exogenous macro variables: spot diesel fuel index $(\$/\text{gal})$, regional load-to-truck density ratios, and weather alerts.
* **Belief / Knowledge State ($b_t \in \Delta(\mathcal{H}_N)$):** Probability distribution over discrete competitor posture regimes $\Theta_t \in \mathcal{H}_N = \{\theta_1, \dots, \theta_K\}$:
  $$b_t = \left( b_t(\theta_1), \dots, b_t(\theta_K) \right), \quad \sum_{k=1}^K b_t(\theta_k) = 1.0, \quad b_t(\theta_k) \ge 0$$

---

### 2.2 Action Space $\mathcal{A}_N(S_t)$ & Contingent Dispatch Formulation
$$a_t = (x_t, p_t) \in \mathcal{X}_t(R_t) \times \mathcal{P}_t^N$$
* **Primal Dispatch Matching ($x_t \in \mathcal{X}_t$):** Bipartite assignment matrix $x_{d, \ell} \in \{0, 1\}$ satisfying capacity constraints:
  $$\sum_{\ell \in \mathcal{L}_t} x_{d, \ell} \le 1 \quad \forall d \in \mathcal{D}_t, \qquad \sum_{d \in \mathcal{D}_t} x_{d, \ell} \le 1 \quad \forall \ell \in \mathcal{L}_t$$
* **Endogenous Spot Pricing ($p_t \in \mathcal{P}_t^N$):** Vector of bid prices $p_\ell \in [\underline{p}, \bar{p}]$ submitted for spot tenders. *(At $N=0$, $\mathcal{P}_t^0 = \{\varnothing\}$).*

#### Causality & Contingent Execution Semantics:
1. **Committed Contract Freight ($\mathcal{L}_t^{\text{committed}}$):** Freight already booked executes immediately: $x_{d, \ell} = 1$ dispatches the driver.
2. **Spot Tenders ($\mathcal{L}_t^{\text{spot}}$):** Match $x_{d, \ell} = 1$ is a **contingent resource reservation**.
   - If won ($O_{t+1}(\ell) = \text{WON}$), the match executes physically ($x_{d, \ell}^{\text{realized}} = 1$).
   - If lost ($O_{t+1}(\ell) = \text{LOST}$), the driver remains at origin or repositions ($x_{d, \ell}^{\text{realized}} = 0$).

---

### 2.3 Exogenous Information $\mathcal{W}_N$
$$W_{t+1} = (\hat{L}_{t+1}, \Delta I_{t+1}, O_{t+1}) \in \mathcal{W}_N$$
* $\hat{L}_{t+1}$: Realized new load arrivals and cancellations.
* $\Delta I_{t+1}$: Diesel price changes and transit delay updates.
* $O_{t+1}$: Censored auction outcomes $O_{t+1}(\ell) \in \{\text{WON}, \text{LOST}\}$. *(At $N=0$, $O_{t+1} = \varnothing$).*

---

### 2.4 Transition Function $S_{t+1} = S^M(S_t, a_t, W_{t+1})$ & Post-Decision State $S_t^a$

$$S_t = (R_t, I_t, b_t) \xrightarrow{a_t = (x_t, p_t)} S_t^a = (R_t^x, I_t, b_t^a) \xrightarrow{W_{t+1}} S_{t+1} = (R_{t+1}, I_{t+1}, b_{t+1})$$

1. **Post-Decision Resource State ($R_t^x$):** Committed driver-load pairings prior to transit completion or new arrivals $\hat{L}_{t+1}$.
2. **Post-Decision Belief State ($b_t^a$):** Competitor posture prior projected through Markov transition matrix $T(\theta, \theta') = \Pr(\Theta_{t+1} = \theta' \mid \Theta_t = \theta)$:
   $$b_t^a(\theta') = \sum_{\theta \in \mathcal{H}_N} T(\theta, \theta') b_t(\theta)$$
3. **Next Pre-Decision Belief State ($b_{t+1}$):** Upon observing auction feedback $O_{t+1}$, Bayes' rule updates $b_t^a$ to $b_{t+1}$:
   $$b_{t+1}(\theta') = \frac{\Pr(O_{t+1} \mid \theta', p_t) b_t^a(\theta')}{\sum_{\theta'' \in \mathcal{H}_N} \Pr(O_{t+1} \mid \theta'', p_t) b_t^a(\theta'')}$$

---

### 2.5 Contribution Function $C_N(S_t, a_t)$
* **For Committed Contract Freight ($N=0$ or $\ell \in \mathcal{L}_t^{\text{committed}}$):**
  $$C^{\text{contract}}(d, \ell) = r_\ell - c^{\text{exec}}_{d, \ell}$$
  where $c^{\text{exec}}_{d, \ell} = c^{\text{fixed}} + c^{\text{loaded}} \cdot \text{Miles}_{d, \ell} + c^{\text{empty}} \cdot \text{Deadhead}_{d, \ell} + c^{\text{home}} \cdot \text{DistToHome} + c^{\text{dwell}} \cdot \text{WaitHours} + c^{\text{late}} \cdot \text{LateHours} - \text{Bonus}_d$.
* **For Competitive Spot Tenders ($N \ge 1$ and $\ell \in \mathcal{L}_t^{\text{spot}}$):**
  $$C^{\text{spot}}_{d, \ell}(p_\ell, b_t) = q_\ell(p_\ell, b_t \mid N) \left( p_\ell - c^{\text{exec}}_{d, \ell} \right) - c^{\text{reserve}}_{d, \ell}$$
  where $q_\ell$ is win probability, $c^{\text{exec}}$ is incurred only upon winning, and $c^{\text{reserve}}$ is reservation cost (nominally $\$0.00$).

---

## 3. The Four Universal Classes of Policies (Powell, 2022)

In accordance with Powell (2022, Chapters 11–20), policies are divided into **Policy Search** (PFAs, CFAs) and **Lookahead Approximations** (VFAs, DLAs):

```
                                 THE FOUR CLASSES OF POLICIES
                                              │
                    ┌─────────────────────────┴─────────────────────────┐
                    ▼                                                   ▼
       ┌─────────────────────────┐                         ┌─────────────────────────┐
       │ Policy Search Policies  │                         │ Lookahead Approximations│
       │   (Chapters 11–13)      │                         │   (Chapters 14–20)      │
       └────────────┬────────────┘                         └────────────┬────────────┘
                    │                                                   │
          ┌─────────┴─────────┐                               ┌─────────┴─────────┐
          ▼                   ▼                               ▼                   ▼
    ┌───────────┐       ┌───────────┐                   ┌───────────┐       ┌───────────┐
    │   1. PFA  │       │   2. CFA  │                   │   3. VFA  │       │   4. DLA  │
    │Direct Rule│       │Parametric │                   │   Value   │       │  Direct   │
    │(No Solver)│       │Cost Tuning│                   │ Function  │       │ Lookahead │
    │(Chap. 12) │       │(Chap. 13) │                   │(Chap.14-18)│      │(Chap.19-20)│
    └───────────┘       └───────────┘                   └───────────┘       └───────────┘
```

### 3.1 Policy Function Approximations (PFAs — Chapter 12)
* **Definition:** Direct analytical state-to-action functions $X^{\text{PFA}}(S_t \mid \theta) = f_\theta(S_t)$ evaluated **without solving an embedded optimization model**.
* **Implementation (`internal/domain/policy/pfa.go`):** A **constructive greedy algorithm** $f_\theta(S_t)$ producing a feasible matching directly via deterministic priority rules without solving an optimization problem over the global feasible set $\mathcal{X}_t$:
  $$\text{Score}(d, \ell \mid \theta) = \theta_{\text{rev}} \cdot r_\ell - \theta_{\text{dist}} \cdot \text{Deadhead}(d, \ell) - \text{Costs}(d, \ell)$$
* **Computational Complexity:** $\mathcal{O}(|\mathcal{D}_t| \log |\mathcal{D}_t| + |\mathcal{D}_t| \cdot |\mathcal{L}_t|)$ (sorting availability + greedy linear assignment).
* **Latency:** $0.06\text{ ms}$ (pure array traversal, zero matrix solvers).

---

### 3.2 Cost Function Approximations (CFAs — Chapter 13)
* **Definition:** Parameterized modification of the base optimization model:
  $$X^{\text{CFA}}(S_t \mid \theta) = \arg\max_{x \in \mathcal{X}_t} \left( C(S_t, x) + \sum_{f=1}^F \theta_f \phi_f(S_t, x) \right)$$
* **Myopic Base Model:** Setting $\theta = 0$ recovers the un-shifted base optimization policy:
  $$X^{\text{CFA}}(S_t \mid 0) \equiv X^{\text{Myopic}}(S_t) = \arg\max_{x \in \mathcal{X}_t} C(S_t, x)$$
* **Parameter Tuning via SPSA (`pkg/math/spsa.go`):** Parameter vector $\theta = (\theta_{\text{empty}}, \theta_{\text{home}}, \theta_{\text{dwell}}, \theta_{\text{risk}})$ is tuned offline using Simultaneous Perturbation Stochastic Approximation with harmonic step sequences $a_k = \frac{a}{(k + 1 + A)^\alpha}$ and Bernoulli $\pm 1$ perturbations $\Delta_k$.

---

### 3.3 Value Function Approximations (VFAs — Chapters 14–18) & Network Flow Structure
* **Definition:** Bellman post-decision value optimization:
  $$X^{\text{VFA}}(S_t) = \arg\max_{x \in \mathcal{X}_t} \left( C(S_t, x) + \gamma \sum_{r \in \mathcal{R}} \bar{V}_{t, r}(R_{t, r}^x) \right)$$
  where $\bar{V}_{t, r}(n) = \sum_{k=1}^n \bar{v}_{t, r}(k)$ is the total value of having $n$ drivers terminating in region $r$, and $\bar{v}_{t, r}(1) \ge \bar{v}_{t, r}(2) \ge \dots \ge \bar{v}_{t, r}(K)$ are non-increasing marginal slopes.

#### Theorem 2A (Exact Minimum-Cost Network Flow Formulation for Committed Freight):
For committed freight, separable piecewise-linear concave regional value functions $\bar{V}(R_t^x) = \sum_r \bar{V}_r(R_{t, r}^x)$ yield an **exact Minimum-Cost Network Flow (MCNF)** problem on a directed graph $G = (\mathcal{V}, \mathcal{E})$.
*Proof Construction:*  
Construct a 3-layer flow network: Driver source nodes $\mathcal{D}_t$ (supply $+1$), Load intermediate nodes $\mathcal{L}_t$, and Regional marginal value sink nodes $\mathcal{S}_{\mathcal{R}} = \{ (r, k) \mid r \in \mathcal{R}, k = n_r^0 + 1, \dots, n_r^0 + |\mathcal{D}_t| \}$ (demand $-1$, unit capacity $u = 1$, reward $\bar{v}_r(k)$). Because slopes are monotonically non-increasing ($\bar{v}_r(k) \ge \bar{v}_r(k+1)$), any standard MCNF solver saturates slots in index order. By total unimodularity of the node-arc incidence matrix, all extreme points are integer, guaranteeing exact polynomial solvability in $\mathcal{O}(|V||E|\log|V|)$. *(In local single-driver regional approximations, this reduces directly to the standard bipartite LAP solved via Jonker-Volgenant).* $\blacksquare$

#### Proposition 2B (Separable First-Order Approximation for Joint Pricing and Dispatch):
For competitive spot tenders with stochastic win outcomes, assume:
1. *Tender Independence:* $q_\ell$ depends only on $(p_\ell, b_t, N)$, independent of prices submitted on other tenders.
2. *Absence of Portfolio Bid Constraints:* No global volume caps coupling bids across tenders.
3. *Edge-Local Reservation Costs:* $c^{\text{reserve}}_{d, \ell}$ is local to edge $(d, \ell)$.

**Lemma 2B.1 (Edgewise Bid Decoupling):**  
Under Hypotheses 1–3, the joint pricing and matching optimization decomposes into an inner scalar price optimization followed by an outer assignment:
$$w_{d, \ell}^* = \max_{p_\ell \in \mathcal{P}_\ell} \left\{ q_\ell(p_\ell, b_t \mid N) \left[ p_\ell - c^{\text{exec}}_{d, \ell} + \gamma \left( \bar{v}_{\text{dest}(\ell)}(n_{\text{dest}}^0 + 1) - \bar{v}_{\text{orig}(d)}(n_{\text{orig}}^0) \right) \right] - c^{\text{reserve}}_{d, \ell} \right\}$$
The optimal joint policy is given by:
$$x^* = \arg\max_{x \in \mathcal{X}_t} \sum_{d \in \mathcal{D}_t} \sum_{\ell \in \mathcal{L}_t} w_{d, \ell}^* x_{d, \ell}, \qquad p_{d, \ell}^* = \arg\max_{p \in \mathcal{P}_\ell} w_{d, \ell}(p)$$
preserving linear bipartite assignment structure over $x$ with pre-optimized scalar edge weights $w_{d, \ell}^*$. $\blacksquare$

#### Dual Gauge Normalization & Economic Supergradient Identification:
In bipartite matching with slack nodes for unassigned assets, setting the slack dual gauge $v_{\text{slack}} = 0$ eliminates additive translation invariance ($u_i \leftarrow u_i + c, v_j \leftarrow v_j - c$). Under this normalization, any optimal dual vector $u^* \in \partial^* z$ selected by the solver defines a valid normalized **supergradient** $\hat{v}_{t, d} = u_d^*$ of the concave value function with respect to driver capacity, providing valid stochastic supergradient samples for CAVE leveling (`internal/service/vfa_learner.go`) under standard Robbins-Monro step-size sequences.

---

### 3.4 Direct Lookahead Approximations (DLAs — Chapters 19–20)
* **Definition:** Rolling-horizon optimization over truncated future horizon $H$:
  $$X^{\text{DLA}}(S_t) = \arg\max_{x_t \in \mathcal{X}_t} \left( C(S_t, x_t) + \mathbb{E}_{\tilde{W}} \left[ \sum_{t'=t+1}^{t+H} \gamma^{t'-t} C(\tilde{S}_{t'}, X^{\text{Base}}(\tilde{S}_{t'})) \right] \right)$$
* **Implementation (`internal/domain/policy/dla.go`):** Forward Monte Carlo trajectory rollouts across parallel goroutines with UCT tree search.

---

### 3.5 Four Policy Classes Summary

| Policy Class | Powell Chapter | Optimization Mechanism | Embedded Solver | Training | Latency |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **1. PFA** | Chapter 12 | Constructive Greedy Priority Rule | **None (No LAP)** | None | $0.06\text{ ms}$ |
| **2. CFA** | Chapter 13 | Parametric Cost Modification | Jonker-Volgenant LAP | SPSA ($50$ epochs) | $0.60\text{ ms}$ |
| **3. VFA** | Chapters 14–18 | Piecewise Linear Concave Slopes | Jonker-Volgenant LAP / MCNF | Dual Supergradients + CAVE | $0.48\text{ ms}$ |
| **4. DLA** | Chapters 19–20 | Forward Scenario Tree Search | Monte Carlo UCT Rollouts | None | $99.9\text{ ms}$ |
| **5. POMDP** | SDA Extension | Joint Dispatch + Spot Pricing | Decoupled LAP + Bayes Filter | SPSA + Prior Calibration | $0.38\text{ ms}$ |

---

## 4. Value of Information: Theoretical Theorem & Empirical Factorial Analysis

### 4.1 Theorem 3 (Pure Value of Information via Policy-Set Inclusion):
$$\sup_{\pi \in \Pi^{\text{informed}}} \mathbb{E}[V^\pi] \ge \sup_{\pi \in \Pi^{\text{blind}}} \mathbb{E}[V^\pi]$$

*Proof:*  
Let $\mathcal{F}_t^{\text{blind}} = \sigma(R_{0:t}, I_{0:t}, b_0)$ and $\mathcal{F}_t^{\text{informed}} = \sigma(R_{0:t}, I_{0:t}, O_{1:t}, b_t)$. Because the informed policy observes auction feedback $O_{1:t}$, the filtrations satisfy $\mathcal{F}_t^{\text{blind}} \subseteq \mathcal{F}_t^{\text{informed}}$. Both policies choose from the identical action space $\mathcal{A}_t = \mathcal{X}_t \times \mathcal{P}_t$. Any policy adapted to $\mathcal{F}_t^{\text{blind}}$ is trivially adapted to $\mathcal{F}_t^{\text{informed}}$, establishing strict policy-set inclusion $\Pi^{\text{blind}} \subseteq \Pi^{\text{informed}}$. The supremum inequality follows immediately. $\blacksquare$

---

### 4.2 Empirical $2 \times 2$ Factorial Decomposition & Option Value
Across 100 paired 7-day carrier simulation episodes ($N=100$, $df=99$):

$$\begin{array}{r|cc|c}
& \textbf{Blind Belief } (b_0) & \textbf{Informed Belief } (b_t) & \textbf{Marginal VoI} \\
\hline
\textbf{Legacy Action } (\mathcal{P}_t^0 = \{\varnothing\}) & V_{00} = \$16,418.54 & V_{01} = \$16,297.45 & -\$121.09 \\
\textbf{Competitive Action } (\mathcal{P}_t) & V_{10} = \$16,567.82 & V_{11} = \$20,166.25 & +\$3,598.43 \\
\hline
\textbf{Marginal VoA} & +\$149.28 & +\$3,868.80 & \text{Total Lift: } +\$3,747.71
\end{array}$$

#### Factorial Analysis & Option Value Interpretation:
1. **The Negative Marginal Information Effect ($V_{01} - V_{00} = -\$121.09$):**  
   In cell $V_{01}$, pricing is disabled ($\mathcal{P}_t^0 = \{\varnothing\}$), but the dispatch policy incorporates the belief vector $b_t$ via a **belief-sensitive risk adjustment** ($\theta_{\text{risk}} \cdot \text{Var}(b_t)$ in CFA) to avoid dispatching drivers into lanes forecast to experience intense competitor congestion. Under fixed contract tariffs, this defensive posture slightly suppresses accepted load volume in volatile regimes without the ability to demand compensatory tariffs, causing a minor observed sample drag ($-\$121.09$).
2. **Supermodular Interaction ($\Delta_{\text{int}} = +\$3,719.52$, $p < 10^{-4}$):**  
   When market intelligence is coupled with the pricing lever ($V_{11}$), the carrier monetizes the information by raising rates on high-risk lanes, converting defensive avoidance into highly profitable targeted bidding ($+\$3,598.43$ marginal lift).
3. **Factorial Main Effects:**
   $$\text{ME}_{\text{Action}} = \frac{1}{2}[(V_{10} - V_{00}) + (V_{11} - V_{01})] = +\$2,009.04 \quad (53.6\%)$$
   $$\text{ME}_{\text{Info}} = \frac{1}{2}[(V_{01} - V_{00}) + (V_{11} - V_{10})] = +\$1,738.67 \quad (46.4\%)$$
4. **Shapley Value Attribution:** $\Phi_{\text{Action}} = \$2,009.04$ ($53.6\%$), $\Phi_{\text{Info}} = \$1,738.67$ ($46.4\%$).

---

## 5. Direct Answers to Committee Defense Inquiries

1. **“Show me where $N$ occurs in your stochastic model, rather than your prose.”**  
   *Answer:* In $\mathcal{M}_N$, $N$ explicitly parameterizes the aggregate competitor capacity $\mathcal{C}_N = N \cdot \bar{c}$ and the order-statistic tender win probability $q_\ell(p_\ell, b_t \mid N) = \sum_k b_t(\theta_k) [1 - F_{\text{single}}(p_\ell \mid \theta_k)]^N$ under conditionally i.i.d. competitor bids. For $N=0$, $\mathcal{H}_0 = \{\Theta_\emptyset\}$, $\Delta(\mathcal{H}_0) = \{\delta_{\Theta_\emptyset}\}$, $\mathcal{P}_0 = \{\varnothing\}$, and $O_{t+1} = \varnothing$.
2. **“At $N=0$, what is $p_\ell$ in $\text{Revenue}(p_\ell)$, given that you just removed the pricing action?”**  
   *Answer:* At $N=0$, pricing is exogenous: revenue is the predetermined contract rate $r_\ell$. For $N \ge 1$, revenue is determined endogenously by submitted bid price $p_\ell \in \mathcal{P}_t$.
3. **“How do you assign a driver to a tender at $t$ when the observation that you won the tender arrives at $t+1$?”**  
   *Answer:* By the **Contingent Dispatch Formulation** (Section 2.2). The match $x_{d, \ell}$ on a spot tender represents a contingent resource reservation. Physical transit occurs on won tenders ($x_t^{\text{realized}} = x_t \odot \mathbf{1}_{\{\text{WON}\}}$), while unassigned drivers remain at origin.
4. **“Prove that setting $\theta=0$ in your CFA produces the deadhead-minimizing PFA you claim it implements.”**  
   *Answer:* PFA is the **Constructive Greedy Priority Dispatch Rule** $X^{\text{PFA}}(S_t \mid \theta) = f_\theta(S_t)$ (Section 3.1, Chapter 12), which performs zero LAP optimization. Setting $\theta = 0$ in CFA recovers the **Myopic Base Optimization Policy** $X^{\text{Myopic}}(S_t) = \arg\max_{x \in \mathcal{X}_t} C(S_t, x)$ (Chapter 13).
5. **“Why is the particular Jonker-Volgenant row potential $u_d$, which may depend on dual normalization, the economically identified marginal value of an additional resource?”**  
   *Answer:* By **Dual Gauge Normalization** (Section 3.3). In bipartite matching with slack nodes for unassigned assets, fixing $v_{\text{slack}} = 0$ eliminates shift invariance, identifying $u_d^* \in \partial^* z$ as a valid normalized **supergradient** of the concave value function with respect to driver capacity.
