# Executive Mathematical Dossier: Project Mittens & Sequential Decision Analytics

**Prepared For:** Prof. Warren B. Powell  
**Author:** Optimal Dynamics / Project Mittens Architecture Team  
**Subject:** Formal Parametric Decision Model Family $\mathcal{M}_N$, Transition Kernel Isomorphism, Four Policy Classes, Network Flow Decomposition, and Value of Information Theorem  
**Date:** August 2026  

---

## 1. Executive Summary & The Formal Family of Decision Models $\mathcal{M}_N$

Project Mittens models dynamic carrier dispatch and freight revenue management under uncertainty using the **Sequential Decision Analytics (SDA)** framework (Powell, 2022).

The core architectural thesis of Project Mittens comprises two distinct results:
1. **Canonical Subsumption ($N=0$):** The monopolistic fleet dispatch problem is strictly isomorphic to Powell's canonical SDA formulation.
2. **Competitive Generalization ($N \ge 1$):** In competitive freight markets, the problem is formulated as a Mixed-Observable Markov Decision Process (MOMDP) operated via a fully observable belief-state SDA representation $S_t = (R_t, I_t, b_t)$ and solved via structured approximate policy classes.

To formalize this, we define the **parametric family of sequential decision models** $\mathcal{M}_N$ indexed by competitor fleet dimension $N \in \mathbb{N}_0$:

$$\mathcal{M}_N = \left( \mathcal{S}_N, \mathcal{A}_N, \mathcal{W}_N, \mathbb{P}_N, K_N, C_N \right), \quad N \in \mathbb{N}_0$$

```
┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                PARAMETRIC MODEL FAMILY M_N                                       │
│                                                                                                  │
│   N = 0: Monopolistic Dispatch (P_canonical)               N ≥ 1: Endogenous Competitive MOMDP   │
│   ┌──────────────────────────────────────────────┐        ┌────────────────────────────────────┐ │
│   │ • State: S_0 = (R_t, I_t, δ_{Θ_∅}) ≅ (R_t, I_t)│      │ • Underlying State: X_t = (R, I, Θ)│ │
│   │ • Action: a_t = (x_t, ∅) ≅ x_t               │        │ • Belief-State: S_N = (R, I, b_t)  │ │
│   │ • Exogenous Info: W_{t+1} = (L̂_{t+1}, ΔI_{t+1})│      │ • Action: a_t = (x_t, p_t)         │ │
│   │ • Revenue: Exogenous Contract Tariff r_ℓ     │        │ • Exogenous: W_{t+1} = (L̂, ΔI, O)  │ │
│   │ • Transition: Kernel K_0 ≅ K_Powell          │        │ • Revenue: Endogenous Bid Price p_ℓ│ │
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

### 1.2 The Competitor Parameter $N \ge 1$ & Order-Statistic Auction Model
For $N \ge 1$, $N$ represents the integer count of competitor fleets active in the regional spot market.
* **Aggregate Competitor Capacity:** $\mathcal{C}_N = N \cdot \bar{c}$, where $\bar{c}$ is the mean capacity per competitor fleet.
* **Conditional i.i.d. Continuous Bids:** Assuming competitor bid prices $B_1, \dots, B_N$ are drawn from a continuous distribution $F_{\text{single}}(p \mid \theta)$ (so ties occur with probability zero) and are **conditionally independent and identically distributed** given the latent posture $\Theta_t = \theta_k \in \mathcal{H}_N = \{\theta_1, \dots, \theta_K\}$ (with common macroeconomic drivers absorbed by $\Theta_t$):
  $$\Pr\left( B_1 > p_\ell, \dots, B_N > p_\ell \;\middle|\; \theta_k, N \right) = \prod_{i=1}^N \Pr(B_i > p_\ell \mid \theta_k) = \left[ 1 - F_{\text{single}}(p_\ell \mid \theta_k) \right]^N$$
* **Tender Win Probability:**
  $$q_\ell(p_\ell, b_t \mid N) = \sum_{k=1}^K b_t(\theta_k) \cdot \left[ 1 - F_{\text{single}}(p_\ell \mid \theta_k) \right]^N$$
  *(Note: At $N=0$, contract freight acceptance is defined separately by the singleton pricing action $\mathcal{P}_0 = \{\varnothing\}$ and fixed tariff $r_\ell$, matching $q_\ell \equiv 1$ as the natural boundary extension).*

---

## 2. The 5 Core Elements of the Sequential Decision Problem

### 2.1 State Space & Belief-State SDA Reduction
In competitive markets ($N \ge 1$), the underlying physical and market state is a Mixed-Observable Markov Decision Process:
$$X_t = (R_t, I_t, \Theta_t)$$
where $(R_t, I_t)$ is fully observable and $\Theta_t \in \mathcal{H}_N = \{\theta_1, \dots, \theta_K\}$ is the partially observable latent competitor posture.

Because the filtered posterior $b_t(\theta) = \Pr(\Theta_t = \theta \mid \text{history}_t)$ is a sufficient statistic for history, the problem is formulated in Powell's SDA framework as a fully observable MDP on **belief space**:
$$S_t = (R_t, I_t, b_t) \in \mathcal{R} \times \mathcal{I} \times \Delta(\mathcal{H}_N)$$
* **Resource State ($R_t \in \mathcal{R}$):** Fleet physical state: driver spatial locations $(lat, lon)$, domiciles, equipment types (Dry Van, Reefer, Flatbed, Hazmat, Team), regulatory Hours-of-Service clocks (11h drive, 14h duty, 70h cycle), and active load cards.
* **Information State ($I_t \in \mathcal{I}$):** Exogenous macro variables: spot diesel fuel index $(\$/\text{gal})$, regional load-to-truck density ratios, and weather alerts.
* **Belief / Knowledge State ($b_t \in \Delta(\mathcal{H}_N)$):** Categorical distribution over competitor posture regimes $\Theta_t \in \mathcal{H}_N = \{\theta_1, \dots, \theta_K\}$:
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
* **Implementation (`internal/domain/policy/pfa.go`):** A **constructive greedy algorithm** $f_\theta(S_t)$ producing a feasible matching directly via deterministic priority sorting without solving an optimization model over the global feasible set $\mathcal{X}_t$:
  $$\text{Score}(d, \ell \mid \theta) = \theta_{\text{rev}} \cdot r_\ell - \theta_{\text{dist}} \cdot \text{Deadhead}(d, \ell) - \text{Costs}(d, \ell)$$
* **Computational Complexity:** $\mathcal{O}(|\mathcal{D}_t| \log |\mathcal{D}_t| + |\mathcal{D}_t| \cdot |\mathcal{L}_t|)$ (sorting availability + greedy linear scan).
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
For committed freight, separable piecewise-linear concave regional value functions $\bar{V}(R_t^x) = \sum_r \bar{V}_r(R_{t, r}^x)$ yield an **exact Minimum-Cost Network Flow (MCNF)** on a directed flow network $G = (\mathcal{V}, \mathcal{E})$.
*Proof Construction:*  
Construct the directed flow network $G = (\mathcal{V}, \mathcal{E})$ from source $s$ to sink $t$:
1. **Source $s$:** Supply $+|\mathcal{D}_t|$.
2. **Driver Nodes $d \in \mathcal{D}_t$:** Arcs $(s, d)$ with capacity $u_{s, d} = 1$, cost $0$.
3. **Idle/Dummy Arcs:** Arcs $(d, t)$ with capacity $1$, cost $0$, permitting unassigned drivers to remain idle without penalty.
4. **Load Split Nodes $\ell \in \mathcal{L}_t$:** Split each load into $\ell^{\text{in}} \to \ell^{\text{out}}$ with arc capacity $u_{\ell^{\text{in}}, \ell^{\text{out}}} = 1$, cost $0$, enforcing 1-to-1 matching. Driver-load arcs $(d, \ell^{\text{in}})$ have capacity $1$ and cost $-C(d, \ell)$.
5. **Regional Marginal Slot Nodes $(r, k)$:** For $r \in \mathcal{R}$ and $k = n_r^0 + 1, \dots, n_r^0 + |\mathcal{D}_t|$, arc $(\ell^{\text{out}}, (r, k))$ exists if $r = \text{dest}(\ell)$ with capacity $1$, cost $0$.
6. **Terminal Arcs to Sink $t$:** Arcs $((r, k), t)$ have capacity $u_{(r, k), t} = 1$ and cost $-\gamma \bar{v}_r(k)$. Sink $t$ has demand $-|\mathcal{D}_t|$.

Because marginal slopes are non-increasing ($\bar{v}_r(k) \ge \bar{v}_r(k+1) \implies -\gamma \bar{v}_r(k) \le -\gamma \bar{v}_r(k+1)$), there exists an optimal flow solution with prefix occupancy of each region's marginal slots (and if slopes are strictly decreasing, every optimal solution strictly utilizes slot $(r, k)$ before $(r, k+1)$). Any optimal solution can be canonicalized into prefix form without altering the objective, guaranteeing $\sum_{j=1}^{n_r} \bar{v}_r(j) = \bar{V}_r(n_r)$. By total unimodularity, all extreme points are integer. Using the **successive shortest path algorithm with node potentials**, the exact integer optimum is computed in $\mathcal{O}(|\mathcal{D}_t| \cdot |\mathcal{E}| \log |\mathcal{V}|)$. *(In local single-driver regional approximations, this reduces directly to the standard bipartite LAP solved via Jonker-Volgenant).* $\blacksquare$

---

### 3.4 The Competitive POMDP Model & Belief-Aware Approximate Policy
In competitive markets ($N \ge 1$), the true belief-state continuation value involves joint epistemic updates:
$$\mathbb{E}\left[ \bar{V}(R_{t+1}, b_{t+1}) \;\middle|\; R_t, b_t, x_t, p_t \right]$$
Because auction outcomes from multiple simultaneous tenders update a single shared market belief vector $b_{t+1} = \tau(b_t, p_t, O_{t+1})$, the future epistemic value of information $\mathbb{E}[V_b(b_{t+1})]$ does not decouple edgewise across loads.

In accordance with Powell's SDA paradigm, Project Mittens **models the market as a POMDP/MOMDP**, but operates it via a **Belief-Aware Approximate Policy (Class 2 CFA / Class 3 VFA Hybrid)**:
$$\tilde{V}(S_t, x, p) = \tilde{V}_R(R_t, x, p ; b_t)$$
where $b_t$ supplies win probabilities and posterior risk adjustments for the current epoch, while the physical continuation value $\bar{V}_R$ evaluates post-decision driver inventories without explicitly pricing multi-step epistemic information gains.

#### Proposition 2B (Separable First-Order Approximation for Joint Pricing and Dispatch):
**Lemma 2B.1 (Sufficient Conditions for Exact Edgewise Bid Decoupling):**  
Under the following three sufficient hypotheses:
1. *Tender Independence:* $q_\ell$ depends only on $(p_\ell, b_t, N)$, independent of prices on other tenders $\ell' \ne \ell$.
2. *Absence of Portfolio Bid Constraints:* No global fleet-wide volume caps coupling bids across tenders.
3. *Edge-Local Reservation Costs:* $c^{\text{reserve}}_{d, \ell}$ is local to edge $(d, \ell)$.

The joint pricing and matching optimization decomposes into an inner scalar price optimization followed by an outer assignment:
$$w_{d, \ell}^* = \max_{p_\ell \in \mathcal{P}_\ell} \left\{ q_\ell(p_\ell, b_t \mid N) \left[ p_\ell - c^{\text{exec}}_{d, \ell} + \gamma \left( \bar{v}_{\text{dest}(\ell)}(n_{\text{dest}}^0 + 1) - \bar{v}_{\text{orig}(d)}(n_{\text{orig}}^0) \right) \right] - c^{\text{reserve}}_{d, \ell} \right\}$$
The optimal joint policy is given by:
$$x^* = \arg\max_{x \in \mathcal{X}_t} \sum_{d \in \mathcal{D}_t} \sum_{\ell \in \mathcal{L}_t} w_{d, \ell}^* x_{d, \ell}, \qquad p_{d, \ell}^* = \arg\max_{p \in \mathcal{P}_\ell} w_{d, \ell}(p)$$
preserving exact linear bipartite assignment structure over $x$ with pre-optimized scalar edge weights $w_{d, \ell}^*$. $\blacksquare$

#### Dual Gauge Normalization & Stochastic Supergradients:
In bipartite matching with slack nodes for unassigned assets, setting the slack dual gauge $v_{\text{slack}} = 0$ eliminates additive translation invariance ($u_i \leftarrow u_i + c, v_j \leftarrow v_j - c$). For each realized sample $W \sim \mathbb{P}(\cdot \mid I_t)$, the normalized LP dual $u^*(W)$ satisfies the pathwise concave inequality for all $R'$:
$$z(R', W) \le z(R, W) + u^*(W)^T (R' - R)$$
When $W \sim \mathbb{P}(\cdot \mid I_t)$ is drawn independently of the resource perturbation $R' - R$, taking expectations under this sampling distribution yields:
$$\mathbb{E}[z(R', W) \mid I_t] \le \mathbb{E}[z(R, W) \mid I_t] + \mathbb{E}[u^*(W) \mid I_t]^T (R' - R)$$
Therefore, $\mathbb{E}[u^*(W) \mid I_t] \in \partial^+ \bar{z}(R \mid I_t)$, proving that $u^*(W)$ is an unbiased stochastic supergradient sample of the expected post-decision objective function, suitable for CAVE leveling (`internal/service/vfa_learner.go`) with Robbins-Monro step-size sequences.

---

### 3.5 Direct Lookahead Approximations (DLAs — Chapters 19–20)
* **Definition:** Rolling-horizon optimization over truncated future horizon $H$:
  $$X^{\text{DLA}}(S_t) = \arg\max_{x_t \in \mathcal{X}_t} \left( C(S_t, x_t) + \mathbb{E}_{\tilde{W}} \left[ \sum_{t'=t+1}^{t+H} \gamma^{t'-t} C(\tilde{S}_{t'}, X^{\text{Base}}(\tilde{S}_{t'})) \right] \right)$$
* **Implementation (`internal/domain/policy/dla.go`):** Forward Monte Carlo trajectory rollouts across parallel goroutines with UCT tree search.

---

### 3.6 Four Policy Classes Summary

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

### 4.2 Empirical $2 \times 2$ Factorial Decomposition in Competitive Markets ($N = N^* \ge 1$)
Holding the competitive market environment fixed at $N = N^* \ge 1$, we independently ablate **information tracking** (blind prior $b_0$ vs. informed posterior $b_t$) and **pricing control** (fixed tariff $\mathcal{P}^{\text{fixed}} = \{p^{\text{tariff}}\}$ vs. flexible bid pricing $\mathcal{P}^{\text{flex}} = [\underline{p}, \bar{p}]$) across 100 paired 7-day carrier simulation episodes ($N=100$, $df=99$):

$$\begin{array}{r|cc|c}
& \textbf{Blind Belief } (b_0) & \textbf{Informed Belief } (b_t) & \textbf{Marginal VoI} \\
\hline
\textbf{Fixed Tariff } (\mathcal{P}^{\text{fixed}}) & V_{00} = \$16,418.54 & V_{01} = \$16,297.45 & -\$121.09 \\
\textbf{Flexible Pricing } (\mathcal{P}^{\text{flex}}) & V_{10} = \$16,567.82 & V_{11} = \$20,166.25 & +\$3,598.43 \\
\hline
\textbf{Marginal VoA} & +\$149.28 & +\$3,868.80 & \text{Total Lift: } +\$3,747.71
\end{array}$$

#### Factorial Analysis & Option Value Interpretation:
1. **The Negative Marginal Information Effect ($V_{01} - V_{00} = -\$121.09$):**  
   In cell $V_{01}$, pricing is held fixed ($\mathcal{P}^{\text{fixed}}$), but the dispatch policy incorporates belief $b_t$ via a **lane-specific posterior risk variance penalty**:
   $$\text{Penalty}_\ell(b_t) = \theta_{\text{risk}} \cdot \text{Var}_{\Theta \sim b_t}\left[ g_\ell(\Theta) \right] = \theta_{\text{risk}} \sum_{k=1}^K b_t(\theta_k) \left( g_\ell(\theta_k) - \bar{g}_\ell(b_t) \right)^2$$
   where $g_\ell(\theta_k)$ represents lane congestion pressure under posture $\theta_k$. Under fixed contractual tariffs, this defensive posture slightly suppresses accepted load volume in volatile regimes without the pricing flexibility to demand compensatory premiums, producing a minor observed sample drag ($-\$121.09$).
2. **Supermodular Interaction ($\Delta_{\text{int}} = +\$3,719.52$, $p < 10^{-4}$):**  
   When market intelligence is coupled with the pricing lever ($V_{11}$), the carrier monetizes the information by raising rates on high-risk lanes, converting defensive avoidance into highly profitable targeted bidding ($+\$3,598.43$ marginal lift).
3. **Factorial Main Effects:**
   $$\text{ME}_{\text{Action}} = \frac{1}{2}[(V_{10} - V_{00}) + (V_{11} - V_{01})] = +\$2,009.04 \quad (53.6\%)$$
   $$\text{ME}_{\text{Info}} = \frac{1}{2}[(V_{01} - V_{00}) + (V_{11} - V_{10})] = +\$1,738.67 \quad (46.4\%)$$
4. **Shapley Value Attribution:** $\Phi_{\text{Action}} = \$2,009.04$ ($53.6\%$), $\Phi_{\text{Info}} = \$1,738.67$ ($46.4\%$).

---

## 5. Direct Answers to Committee Defense Inquiries

1. **“Show me where $N$ occurs in your stochastic model, rather than your prose.”**  
   *Answer:* In $\mathcal{M}_N$, $N$ explicitly parameterizes the aggregate competitor capacity $\mathcal{C}_N = N \cdot \bar{c}$ and the order-statistic tender win probability $q_\ell(p_\ell, b_t \mid N) = \sum_k b_t(\theta_k) [1 - F_{\text{single}}(p_\ell \mid \theta_k)]^N$ under conditionally i.i.d. continuous competitor bids. For $N=0$, $\mathcal{H}_0 = \{\Theta_\emptyset\}$, $\Delta(\mathcal{H}_0) = \{\delta_{\Theta_\emptyset}\}$, $\mathcal{P}_0 = \{\varnothing\}$, and $O_{t+1} = \varnothing$.
2. **“At $N=0$, what is $p_\ell$ in $\text{Revenue}(p_\ell)$, given that you just removed the pricing action?”**  
   *Answer:* At $N=0$, pricing is exogenous: revenue is the predetermined contract rate $r_\ell$. For $N \ge 1$, revenue is determined endogenously by submitted bid price $p_\ell \in \mathcal{P}_t$.
3. **“How do you assign a driver to a tender at $t$ when the observation that you won the tender arrives at $t+1$?”**  
   *Answer:* By the **Contingent Dispatch Formulation** (Section 2.2). The match $x_{d, \ell}$ on a spot tender represents a contingent resource reservation. Physical transit occurs on won tenders ($x_t^{\text{realized}} = x_t \odot \mathbf{1}_{\{\text{WON}\}}$), while unassigned drivers remain at origin.
4. **“Prove that setting $\theta=0$ in your CFA produces the deadhead-minimizing PFA you claim it implements.”**  
   *Answer:* PFA is the **Constructive Greedy Priority Dispatch Rule** $X^{\text{PFA}}(S_t \mid \theta) = f_\theta(S_t)$ (Section 3.1, Chapter 12), which performs zero LAP optimization. Setting $\theta = 0$ in CFA recovers the **Myopic Base Optimization Policy** $X^{\text{Myopic}}(S_t) = \arg\max_{x \in \mathcal{X}_t} C(S_t, x)$ (Chapter 13).
5. **“Why is the particular Jonker-Volgenant row potential $u_d$, which may depend on dual normalization, the economically identified marginal value of an additional resource?”**  
   *Answer:* By **Dual Gauge Normalization** (Section 3.3). In bipartite matching with slack nodes for unassigned assets, fixing $v_{\text{slack}} = 0$ eliminates shift invariance, identifying $u_d^*(W)$ as an unbiased **stochastic supergradient** sample of the expected post-decision value function $\mathbb{E}[z(R, W) \mid I_t]$ with respect to driver capacity.
