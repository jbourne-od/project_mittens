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
* **Key Talking Point:** Point out how PFA evaluates in $0.09\text{ ms}$ (pure greedy rule), CFA/VFA solve in $1.2\text{ ms}$ via Jonker-Volgenant / MCNF, DLA runs forward lookahead in $25\text{ ms}$, and the Competitive POMDP jointly optimizes dispatch and spot bidding.

### Demo 2: The $2 \times 2$ Factorial Option Value & Value of Information
```bash
./bin/tournament -mode factorial -episodes 50
```
* **What it shows:** Full $2 \times 2$ matrix isolating **pricing flexibility** ($\mathcal{P}^{\text{fixed}}$ vs $\mathcal{P}^{\text{flex}}$) and **market intelligence** ($b_0$ vs $b_t$).
* **Key Talking Point:** Point out the supermodular interaction ($\Delta_{\text{int}} = +\$2,184.40$, $p < 10^{-60}$)—market intelligence has immense economic value *precisely because* the carrier possesses the pricing lever to monetize it.

### Demo 3: Monopolistic Reduction vs. Competitive Superiority
```bash
./bin/tournament -mode pairwise -episodes 20
```
* **What it shows:** Direct head-to-head comparison of $N=0$ monopolistic dispatch vs $N=1$ competitive MOMDP matching.

---

## 3. Key Empirical Findings (N = 1,000 Large-Sample Monte Carlo)

| Empirical Contrast | Mean Effect | 95% Confidence Interval | Two-Sided $p$-Value | Economic Takeaway |
| :--- | :---: | :---: | :---: | :--- |
| $\Delta_{I \mid \text{legacy}} = V_{01} - V_{00}$ | **-$8.51** | $[-\$63.86, +\$46.83]$ | $0.763$ | **Information alone does essentially nothing ($V_{01} \approx V_{00}$)** |
| $\Delta_{A \mid \text{blind}} = V_{10} - V_{00}$ | **-$506.12** | $[-\$723.70, -\$288.55]$ | $5.6 \times 10^{-6}$ | **Pricing lever without information actively hurts** |
| $\Delta_{I \mid \text{comp}} = V_{11} - V_{10}$ | **+$2,175.89** | $[+\$1,997.99, +\$2,353.78]$ | $6.5 \times 10^{-101}$ | **Information is enormously valuable with pricing lever** |
| $\Delta_{A \mid \text{informed}} = V_{11} - V_{01}$ | **+$1,678.28** | $[+\$1,427.70, +\$1,928.86]$ | $1.2 \times 10^{-36}$ | **Pricing lever delivers large lift under informed state** |
| $V_{11} - V_{00}$ | **+$1,669.76** | $[+\$1,419.48, +\$1,920.05]$ | $2.84 \times 10^{-36}$ | **Full Project Mittens beats legacy by +16.44%** |

### Supermodular Complementarity ($\Delta_{\text{int}} = D_i$):
* **Mean Interaction Effect:** $\bar{D} = +\$2,184.40$ ($95\%\text{ CI}: [+\$1,950.12, +\$2,418.68]$, $t = +18.29$, $p = 3.24 \times 10^{-64}$, Cohen's $d_z = 0.5781$).
* **Robustness Across Market Environments $\Delta_{\text{int}} = f(\text{horizon}, \text{market density})$:**
  - **High Load Density (25 loads / 10 drivers):** $\Delta_{\text{int}} = +\$3,922.45$ ($p = 1.42 \times 10^{-19}$, lift = **+23.26%**).
  - **30-Day Long Horizon:** $\Delta_{\text{int}} = +\$2,575.43$ ($p = 1.05 \times 10^{-13}$, Cohen's $d_z = 0.8631$).
  - **1:1 Tight Fleet Capacity (15 drivers / 15 loads):** $\Delta_{\text{int}} = +\$1,871.58$ ($p = 2.27 \times 10^{-16}$, lift = **+13.80%**).

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
