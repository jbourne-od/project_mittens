# Montana Meeting Quick Reference & Pocket Guide

**Prepared For:** Jacob (Montana Briefing with Prof. Warren B. Powell)  
**Subject:** High-Level Talking Points, Terminal Demo Cheatsheet, and Rapid Defense Guide  
**Date:** August 2026  

---

## 1. The 30-Second Elevator Pitch

> *"Warren, we didn't just migrate our carrier optimizer from Java to Go for speed; we unified the entire system under your **Sequential Decision Analytics (SDA)** framework."*
>
> *"We proved mathematically that the classical fleet dispatch problem is an exact degenerate reduction of our model at $N=0$ ($\mathcal{M}_0 \cong \mathcal{M}_{\text{Powell}}$), and we generalized it to competitive spot markets ($N \ge 1$) as a Mixed-Observable MDP operated on a fully observable belief-state representation $S_t = (R_t, I_t, b_t)$."*
>
> *"We implemented all **Four Universal Classes of Policies** (PFAs, CFAs, VFAs, DLAs) and benchmarked them head-to-head on identical stochastic problem instances. Everything is running live in compiled, race-free Go on this laptop."*

---

## 2. The 3 Live Terminal Demonstrations (100% Offline)

From the repository root (`/Users/jacob/Development/od/project_mittens`):

### Demo 1: The 4-Policy Benchmark Tournament (PFA vs CFA vs VFA vs DLA vs Competitive POMDP)
```bash
./bin/tournament -mode 4way -episodes 10 -days 5
```
* **What it shows:** Side-by-side scorecard of all 4 policy classes plus the competitive POMDP.
* **Key Talking Point:** Point out how PFA evaluates in $0.06\text{ ms}$ (pure greedy rule), CFA/VFA solve in $0.5\text{ ms}$ via Jonker-Volgenant / MCNF, DLA runs forward Monte Carlo rollouts in $100\text{ ms}$, and the Competitive POMDP delivers substantial net contribution lift by jointly optimizing dispatch and spot bidding.

### Demo 2: The $2 \times 2$ Factorial Option Value & Value of Information ($N = 1,000$ Baseline)
```bash
./bin/tournament -mode factorial -episodes 50
```
* **What it shows:** Full $2 \times 2$ matrix isolating **pricing flexibility** ($\mathcal{P}^{\text{fixed}}$ vs $\mathcal{P}^{\text{flex}}$) and **market intelligence** ($b_0$ vs $b_t$).
* **Key Talking Point:** Point out the supermodular interaction ($\Delta_{\text{int}} = +\$2,184.40$, $t = +18.29, p = 3.24 \times 10^{-64}$)—market intelligence has immense economic value *precisely because* the carrier possesses the pricing lever to monetize it. Under the legacy policy, standalone information effect is essentially zero ($-\$8.51$, $p = 0.763$, $84.3\%$ exact ties).

### Demo 3: Second-Order Response Surfaces & Scarcity Sweeps
```bash
./bin/tournament -mode curves -episodes 50
```
* **What it shows:** Continuous sweeps across **Tender Flow** ($\lambda \in [10, 30]$), **Fleet Capacity** ($K \in [6, 20]$), and **Horizon** ($H \in [3, 30]$ days) with paired finite differences.
* **Key Talking Point:** Point out **Empirical Proposition 4**: Complementarity amplifies under high tender density (interaction jumps to $+\$4,054$ and lift reaches $+28.1\%$ at $2.5 : 1$ load-to-truck ratio), while horizon rollouts reveal per-day information attenuation ($\$311/\text{day} \to \$81/\text{day}$) against constant blind mispricing bleeding ($-\$72/\text{day}$).

### Demo 4: Monopolistic Reduction vs. Competitive Superiority
```bash
./bin/tournament -mode pairwise
```
* **What it shows:** Direct head-to-head comparison of $N=0$ monopolistic dispatch vs $N=1$ competitive MOMDP matching.

---

## 3. Rapid Defense: The 5 Tough Questions & 10-Second Answers

| Potential Question from Warren | 10-Second Winning Answer | Reference Section |
| :--- | :--- | :--- |
| **1. "Where does $N$ occur in your stochastic model, rather than your prose?"** | *"In $\mathcal{M}_N$, $N$ parameterizes aggregate competitor capacity $\mathcal{C}_N = N\bar{c}$ and the order-statistic tender win probability $q_\ell = \sum_k b_t(k)[1 - F_{\text{single}}(p_\ell \mid \theta_k)]^N$. At $N=0$, $\mathcal{H}_0 = \{\Theta_\emptyset\}$, belief collapses to $\delta_{\Theta_\emptyset}$, and pricing is inactive."* | Dossier §1.2 |
| **2. "At $N=0$, what is revenue if pricing is removed?"** | *"At $N=0$, pricing is exogenous: revenue is the predetermined contractual rate $r_\ell$ (fixed tariff). For $N \ge 1$, revenue is determined endogenously by the submitted bid price $p_\ell$."* | Dossier §2.5 |
| **3. "How do you assign drivers before knowing if you won the tender?"** | *"Via our **Contingent Dispatch Formulation**: the match $x_{d, \ell}$ is a contingent resource reservation. Physical transit executes only on won tenders ($x_t^{\text{realized}} = x_t \odot \mathbf{1}_{\{\text{WON}\}}$), while unassigned drivers remain at origin."* | Dossier §2.2 |
| **4. "Is $\arg\max_x C(S, x)$ a PFA in my taxonomy?"** | *"No. Following your Chapter 11–13 definitions, $\arg\max C(S, x)$ is the **Myopic Base Optimization Model** (CFA with $\theta=0$). Our PFA is an authentic direct constructive greedy priority dispatch rule $f_\theta(S_t)$ with zero embedded LAP solvers."* | Dossier §3.1 |
| **5. "How does piecewise-linear concave VFA preserve integer network flow?"** | *"We construct unit-capacity regional marginal value slots $(r, k)$ with rewards $\bar{v}_r(k)$. Non-increasing slopes ($\bar{v}_r(k) \ge \bar{v}_r(k+1)$) guarantee prefix slot saturation, maintaining total unimodularity and integer extreme points solved via Successive Shortest Paths in $\mathcal{O}(\vert\mathcal{D}_t\vert \vert\mathcal{E}\vert \log \vert\mathcal{V}\vert)$."* | Dossier §3.3 |

---

## 4. Key Reference Documents in the Repository

1. **Executive Mathematical Dossier:** [`docs/powell_mathematical_dossier.md`](file:///Users/jacob/Development/od/project_mittens/docs/powell_mathematical_dossier.md)  
   *(Full formal proofs of Theorem 1 Isomorphism, Theorem 2A MCNF Decomposition, Lemma 2B.1 Edgewise Decoupling, and Theorem 3 Value of Information).*
2. **Equation Traceability Matrix:** [`docs/powell_equation_traceability_matrix.md`](file:///Users/jacob/Development/od/project_mittens/docs/powell_equation_traceability_matrix.md)  
   *(1-to-1 cross-reference linking RLSO 2022 equations to typed Go structs and methods).*
3. **Doctoral Defense Proofs:** [`proofs/powell_subsumption_defense.md`](file:///Users/jacob/Development/od/project_mittens/proofs/powell_subsumption_defense.md) and [`proofs/n1_competitive_superiority_defense.md`](file:///Users/jacob/Development/od/project_mittens/proofs/n1_competitive_superiority_defense.md).

Have a great flight to Montana!
