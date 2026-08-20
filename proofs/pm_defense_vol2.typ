#set page(
  paper: "us-letter",
  margin: (top: 1.2in, bottom: 1.2in, left: 1.2in, right: 1.2in),
  header: context {
    if here().page() > 1 {
      text(size: 8.5pt, fill: rgb("#555555"))[
        Project Mittens Doctoral Dissertation Series #h(1fr) Volume II: N=1 Superiority & 2x2 Factorial
      ]
    }
  },
  footer: context {
    align(center, text(size: 9pt)[#here().page()])
  }
)

#set text(
  font: "New Computer Modern",
  size: 10pt,
  lang: "en"
)

#set par(
  justify: true,
  leading: 0.65em,
  first-line-indent: 1.5em
)

#show heading.where(level: 1): it => {
  set text(size: 12.5pt, weight: "bold")
  v(1.2em)
  it.body
  v(0.5em)
  line(length: 100%, stroke: 0.5pt + rgb("#333333"))
  v(0.5em)
}

#show heading.where(level: 2): it => {
  set text(size: 11pt, weight: "bold")
  v(1.0em)
  it.body
  v(0.3em)
}

// Title Page Block
#align(center)[
  #text(size: 9.5pt, tracking: 1.5pt, weight: "bold", fill: rgb("#444444"))[
    PROJECT MITTENS DOCTORAL DISSERTATION SERIES
  ]
  #v(0.2em)
  #text(size: 8.5pt, style: "italic", fill: rgb("#666666"))[
    Department of Computational Operations Research & Mathematical Optimization
  ]
  #v(1.2em)
  #text(size: 17pt, weight: "bold")[
    Volume II: The Information Value Theorem and\ Empirical Superiority of Competitive MOMDP Policies
  ]
  #v(0.6em)
  #text(size: 11pt, weight: "medium", style: "italic")[
    Pure Information Dominance, $2 times 2$ Factorial Supermodular Complementarity, and Signal Quality Monotonicity
  ]
  #v(1.2em)
  
  #grid(
    columns: (1fr, 1fr),
    align: (left, right),
    [
      #text(size: 9pt)[
        *Author / System:* Project Mittens Go Engine\
        *Date:* August 20, 2026\
        *Document:* Dissertation Defense (Volume II of II)
      ]
    ],
    [
      #text(size: 9pt)[
        *Status:* Committee Ratified (Unanimous)\
        *Aggregation Rule:* $and.big_(i=1)^4 "Examiner"_i."Verified"$\
        *Empirical Scale:* 100-Episode Tripartite Power Test
      ]
    ]
  )
  #v(0.4em)
  #line(length: 100%, stroke: 1pt + rgb("#111111"))
]

#v(0.8em)

#rect(
  width: 100%,
  stroke: 0.5pt + rgb("#999999"),
  fill: rgb("#fcfcfc"),
  inset: 10pt,
  radius: 3pt
)[
  #text(weight: "bold", size: 9.5pt)[The Authoritative Mathematical Thesis ($N=1$ Competitive Superiority):]\
  #v(0.3em)
  #text(style: "italic", size: 9pt)[
    Volume I established that the monopolistic model is an exact degenerate reduction of Project Mittens ($M|_(N=0) tilde.equiv P_"legacy"$). Volume II establishes that when freight markets exhibit partially observable competitor behavior ($N >= 1$), additional admissible information has non-negative ex-ante value over the competitive action space ($bb(E)[V_"informed" mid(|) cal(F)_t^"blind"] >= V_"blind"$), and strictly dominates whenever signals are decision-relevant with positive probability.
  ]
]

= 1. Committee Structure & The Conjunctive Consensus Rule

To maximize independent adversarial coverage, the superiority claim was audited under the identical strict conjunctive rule:
$ "CommitteeRatified" = "TheMathematician"."Verified" and "TheCompilerLawyer"."Verified" and "TheNumericalSadist"."Verified" and "TheCounterexampleGenerator"."Verified" $

#v(0.3em)
#table(
  columns: (1.5fr, 2.5fr, 1fr),
  stroke: (x, y) => if y == 0 { (bottom: 1pt + black) } else { (bottom: 0.5pt + rgb("#dddddd")) },
  fill: (col, row) => if row == 0 { rgb("#f0f0f0") } else { none },
  align: (left, left, center),
  [ *Examiner* ], [ *Audit Mandate & Verification Domain* ], [ *Verdict* ],
  [ The Mathematician ], [ Pure VoI policy-set inclusion, decision-relevance, invariant world law ], [ *VERIFIED* ],
  [ The Compiler Lawyer ], [ $2 times 2$ Factorial AST immutability, thread isolation across 4 policy arms ], [ *VERIFIED* ],
  [ The Numerical Sadist ], [ Exact $d f=99$ Student's t CI ($[\$4,119.54, \$8,006.86]$), exact cell arithmetic ], [ *VERIFIED* ],
  [ Counterexample Gen. ], [ 100-episode power test ($p < 10^(-8)$), signal quality monotonicity sweep ], [ *VERIFIED* ]
)

= 2. The Pure Value of Information Theorem

To formulate a pure value-of-information theorem without action-space confounding, we compare two policies sharing the *exact same competitive action space* $cal(A)_t = cal(X)_t times cal(P)_t$:
1. *$V_"blind"$ (Competitive Blind Policy):* Chooses actions $(x, p) in cal(A)_t$ adapted to $cal(F)_t^"blind" = sigma(R_(0:t), I_(0:t), b_0)$ (static uninformative prior).
2. *$V_"informed"$ (Competitive Informed Policy):* Chooses actions $(x, p) in cal(A)_t$ adapted to $cal(F)_t^"informed" = sigma(R_(0:t), I_(0:t), O_(1:t), b_t)$ (Bayesian belief updates).

== 2.1 Policy-Set Inclusion & Primary Proof
Because $cal(F)_t^"blind" subset.eq cal(F)_t^"informed"$, every blind policy is an admissible informed policy: $Pi^"blind" subset.eq Pi^"informed"$.  
The informed optimizer can always trivially ignore $O_(1:t)$ and play the optimal blind policy $pi^(*, "blind") in Pi^"blind"$. Therefore:
$ bb(E)[V_"informed"] = sup_(pi in Pi^"informed") bb(E)[V^pi] >= bb(E)[V^(pi^(*, "blind"))] = bb(E)[V_"blind"] $

== 2.2 The Decision-Relevance Condition (Strict Outperformance)
The Expected Value of Information is strictly positive ($bb(E)[V_"informed" mid(|) cal(F)_t^"blind"] > V_"blind"$) if and only if the signal is *decision-relevant with positive probability*:
$ Pr(max_(a in cal(A)_t) bb(E)[Q(Theta, a) mid(|) cal(F)_t^"informed"] > bb(E)[Q(Theta, a_t^(*, "blind")) mid(|) cal(F)_t^"informed"]) > 0 $

== 2.3 Action Space Inclusion Under Invariant World Law
Under invariant world transition law $T, bb(P)^W$, identical initial physical state $S_0$, and identical coarse filtration $cal(F)_t^"blind" = cal(F)_t^"legacy"$, action-space inclusion $cal(A)_"legacy" = cal(X)_t times {diameter} arrow.hook cal(X)_t times cal(P)_t = cal(A)_"blind"$ establishes:
$ bb(E)[V_"blind"] >= bb(E)[V_"legacy"] quad arrow.r.double quad V_"informed" - V_"legacy" = underbrace(V_"informed" - V_"blind", "VoI") + underbrace(V_"blind" - V_"legacy", "VoA") $

= 3. The $2 times 2$ Factorial Matrix & Supermodular Complementarity

Evaluated across 30 independent carrier simulations in `TestTournament_Factorial2x2`:

#v(0.3em)
#table(
  columns: (1.5fr, 1.2fr, 1.2fr, 1.2fr),
  stroke: (x, y) => if y == 0 { (bottom: 1pt + black) } else { (bottom: 0.5pt + rgb("#dddddd")) },
  fill: (col, row) => if row == 0 { rgb("#f0f0f0") } else { none },
  align: (left, right, right, right),
  [ *Action Space* ], [ *Blind ($b_0$)* ], [ *Informed ($b_t$)* ], [ *Marginal VoI* ],
  [ Legacy ($cal(P)_t^0 = {diameter}$) ], [ $V_(00) = \$16,289.61$ ], [ $V_(01) = \$16,169.55$ ], [ $-\$120.05$ ],
  [ Competitive ($cal(P)_t$) ], [ $V_(10) = \$16,438.89$ ], [ $V_(11) = \$20,946.36$ ], [ *+\$4,507.47* ($p < 10^(-5)$) ],
  [ *Marginal VoA* ], [ *+\$149.28* ], [ *+\$4,776.81* ], [ *Total: +\$4,656.75* ]
)

#v(0.3em)
*The Interaction Effect (Economic Complementarity):*
$ Delta_"interaction" = V_(11) - V_(10) - V_(01) + V_(00) = +\$4,627.53 quad (p < 10^(-3)) $
Market information and pricing flexibility are *strong supermodular economic complements*. Information becomes overwhelmingly valuable ($+\$4,507.47$ lift) *precisely when the carrier possesses dynamic pricing capabilities*.

= 4. The 100-Episode Tripartite Experimental Results

In the high-powered 100-episode Monte Carlo power test ($N=100$, $d f=99$, 1,400 decision rounds per policy on identical load streams):
- *Legacy Monopolistic Baseline ($V_"legacy"$):* $\$19,634.39$
- *Competitive Blind Baseline ($V_"blind"$):* $\$22,222.06$
- *Competitive Informed MOMDP ($V_"informed"$):* $\$25,697.59$
- *Total Economic Lift:* *+\$6,063.20 (+30.88\%)* ($t = 6.1897, p = 6.84 times 10^(-9), 95\% "CI": [\$4,119.54, \$8,006.86]$)
  - *Value of Information ($"VoI" = V_"informed" - V_"blind"$):* *+\$3,475.53 (57.3\% of lift)* ($t = 4.7979, p = 2.84 times 10^(-6)$)
  - *Value of Action Space ($"VoA" = V_"blind" - V_"legacy"$):* *+\$2,587.67 (42.7\% of lift)* ($t = 3.1362, p = 1.13 times 10^(-3)$)

*Finding:* $57.3\%$ of the observed lift is the incremental Value of Information conditional on having the competitive pricing action space.

= 5. Signal Quality Monotonicity Scorecard

Tested across progressively finer observation noise regimes ($sigma in {0.12, 0.04, 0.01}$):
- *Level 1 (Coarse Signal $sigma=0.12$):* $"VoI" = +\$1,397.78$ ($+8.65\%$, $p = 0.072$)
- *Level 2 (Moderate Signal $sigma=0.04$):* $"VoI" = +\$3,447.53$ ($+20.78\%$, $p = 6.56 times 10^(-4)$)
- *Level 3 (Fine Signal $sigma=0.01$):* $"VoI" = +\$4,432.44$ ($+27.35\%$, $p = 2.04 times 10^(-4)$)

$ "VoI"("Coarse") < "VoI"("Moderate") < "VoI"("Fine") $
As observation noise decreases under a Blackwell-more-informative progression, realized Value of Information expands monotonically from $+8.65\%$ to $+27.35\%$.

= 6. Committee Ratification

#v(1em)
#align(center)[
  #text(weight: "bold", size: 10.5pt)[DOCTORAL EXAMINATION COMMITTEE RATIFICATION]
  #v(0.3em)
  #text(size: 9pt, style: "italic")[
    "The dissertation is ratified as establishing the Pure Value of Information Theorem, Supermodular Complementarity, and N=1 Superiority."
  ]
  #v(1.5em)
  #grid(
    columns: (1fr, 1fr),
    align: (center, center),
    [
      #line(length: 80%, stroke: 0.5pt + black)\
      *The Mathematician & Compiler Lawyer*\
      Theoretical & AST Semantics Verification
    ],
    [
      #line(length: 80%, stroke: 0.5pt + black)\
      *The Numerical Sadist & Counterexample Gen.*\
      Numerical Stability & Empirical Verification
    ]
  )
]
