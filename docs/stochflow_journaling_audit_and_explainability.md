# Project Mittens: Stochflow-Grade Semantic Journaling, Lossless Replay & Explainability Architecture

**Status:** Authoritative Architectural Design  
**Date:** 2026-08-20  
**Target Audience:** Operations Research Scientists, Platform Architects, Core Developers  
**Governing Inviolates:** [Inviolate 2 (MOMDP Separation)](inviolates.md), [Inviolate 5 (Immutability)](inviolates.md), [Inviolate 7 (Complete Provenance)](inviolates.md), [Section 19 (Observability & PII Integrity)](../AGENTS.md)  
**Cross-Repository Alignment:** Inspired by `stochflow` (Deterministic Replay & Economic Stopping) and `maiden_lane` (Progressive Semantic Spine)

---

## 1. Executive Purpose

Stochastic optimization engines are often criticized as "black boxes." When an optimizer dispatches Driver A to Load 101, holds Driver B idle in Dallas, or shifts spot pricing bids in response to competitor postures, human planners and domain scientists need two foundational capabilities:

1. **End-User Decision Explainability:** Clear, causal, and mathematically justified explanations answering *"Why did the system make this choice over the alternatives?"*
2. **Lossless Record–Replay for Regression & Auditing:** Byte-exact reproducibility of every stochastic transition, observation arrival, and network flow solve, enabling instant regression testing and historical replay.

Project Mittens adopts the dual-track architecture pioneered in `stochflow`: separating **sanitized operational audit trails** (for humans, UIs, and compliance) from **lossless cryptographic journals** (for byte-exact replay, deterministic verification, and automated regression).

```
┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                DUAL-TRACK DURABILITY ARCHITECTURE                                │
└─────────────────────────────────┬───────────────────────────────┬────────────────────────────────┘
                                  │                               │
                                  ▼                               ▼
┌───────────────────────────────────────────────────┐   ┌───────────────────────────────────────────────────┐
│        TRACK 1: LOSSLESS JOURNAL                  │   │        TRACK 2: SANITIZED AUDIT & EXPLAIN         │
│             (pkg/journal)                         │   │             (pkg/audit & pkg/explain)             │
│                                                   │   │                                                   │
│ • Full-fidelity payload bytes & state hashes      │   │ • Lossy, sanitized codes-not-values              │
│ • Cryptographic SHA-256 state & input pins        │   │ • Zero PII (Section 19 compliance)                │
│ • Deterministic logical clock (Run/Epoch/Seq)     │   │ • Human-readable causal explanations              │
│ • Bit-exact replay & automated regression testing │   │ • Counterfactual alternative comparisons          │
└───────────────────────────────────────────────────┘   └───────────────────────────────────────────────────┘
```

---

## 2. Track 1: Lossless Cryptographic Journal (`pkg/journal`)

The **Lossless Journal** is an append-only ledger designed for bit-exact reproducibility. It records every input, state snapshot, solver matrix, and external observation using deterministic logical clocks.

### 2.1 Logical Time vs. Wall-Clock
In accordance with Inviolate 5 and `stochflow` principles, replay execution **never references wall-clock time or host randomness**. All events are keyed by logical coordinates:

$$\text{EventKey} = \Big( \text{OptimizationRunID}, \, \text{LogicalEpoch}, \, \text{BatchSeq}, \, \text{DecisionID} \Big)$$

### 2.2 Journal Entry Specification
Every optimization batch generates an immutable journal record:

```go
// JournalRecord captures full-fidelity state snapshots and cryptographic hashes
// for byte-exact deterministic replay and regression testing.
type JournalRecord struct {
    // Logical Coordinate
    RunID       string `json:"run_id"`
    Epoch       int64  `json:"epoch"`        // 0-based simulation/batch epoch
    BatchSeq    int    `json:"batch_seq"`     // Sequence within epoch
    DecisionID  string `json:"decision_id"`   // Deterministic ID (e.g., "DEC-CFA-1700000000-0001")

    // Semantic Pins & Versioning (Refuses replay on binary/schema mismatch)
    RuntimeVersion   string `json:"runtime_version"`
    PolicyName       string `json:"policy_name"`
    PolicyParamHash  string `json:"policy_param_hash"` // SHA-256 of theta / VFA slopes

    // State Hashes & Payloads (Oracle Reads)
    InitialStateHash string `json:"initial_state_hash"` // SHA-256 of S_t = (R_t, I_t, b_t)
    ResourceState    []byte `json:"resource_state_bytes"`
    InformationState []byte `json:"information_state_bytes"`
    BeliefState      []byte `json:"belief_state_bytes"`

    // Solver Inputs & Outputs
    MatrixDimensionRows int      `json:"matrix_rows"`
    MatrixDimensionCols int      `json:"matrix_cols"`
    CostMatrixHash      string   `json:"cost_matrix_hash"`
    MatchedAssignments  []byte   `json:"matched_assignments_bytes"`
    DualShadowPrices    []byte   `json:"dual_shadow_prices_bytes"` // u_d and v_ell vectors

    // External Effects & Observations
    RealizedObservations []byte `json:"realized_observations_bytes"` // W_{t+1} = (D_{t+1}, Y_{t+1})

    // Output State Hash
    NextStateHash string `json:"next_state_hash"` // SHA-256 of S_{t+1}
}
```

### 2.3 Deterministic Replay Engine (`pkg/replay`)
The replay engine takes a historical `JournalRecord` stream and re-executes the optimization logic:
1. **Oracle Read:** Supplies recorded states and observations without contacting live databases or external APIs.
2. **Re-Execution:** Evaluates feasibility filtering, parametric cost construction, and LAPJV matching.
3. **Assertion Verification:** Compares the newly computed objective and state hashes against the journal:
   $$\text{Assert}\Big(\text{Hash}(S_{t+1}^{\text{recomputed}}) == \text{JournalRecord}.\text{NextStateHash}\Big)$$
4. **Drift Detection:** If an algorithm change produces different matches, the replay harness highlights the exact driver, load, or matrix cost that changed.

---

## 3. Track 2: Sanitized Operational Audit (`pkg/audit`)

While the journal is lossless and binary-oriented, the **Operational Audit Log** is designed for human operators, web UIs, and compliance audits.

### 3.1 Section 19 Compliance (Zero PII & Low Cardinality)
In accordance with Section 19 of `AGENTS.md`:
* **Masked Identifiers:** Driver IDs and customer account numbers are hashed or pseudonymized (`DRV-8492` $\to$ `D-***42`).
* **Sanitized Codes:** Uses enumerated outcome codes rather than raw coordinates or dollar values.
* **Low-Cardinality Logging:** Operational logs record state transitions, invariant checks, and summary indicators.

### 3.2 Audit Event Types

```go
type AuditEventType string

const (
    // Operational Dispatch Events
    EventDispatchMatched       AuditEventType = "DISPATCH_MATCHED"
    EventDriverHeldIdle        AuditEventType = "DRIVER_HELD_IDLE"
    EventTourSynthesized       AuditEventType = "TOUR_SYNTHESIZED"
    EventRelayExchangeCreated  AuditEventType = "RELAY_EXCHANGE_CREATED"

    // Market & POMDP Belief Events
    EventBeliefPostureShift    AuditEventType = "BELIEF_POSTURE_SHIFT"
    EventAuctionBidSubmitted   AuditEventType = "AUCTION_BID_SUBMITTED"
    EventAuctionBidResult      AuditEventType = "AUCTION_BID_RESULT"

    // Invariant & Regulatory Checks
    EventHOSRestInserted       AuditEventType = "HOS_REST_INSERTED"
    EventFeasibilityPruned     AuditEventType = "FEASIBILITY_PRUNED"
    EventSimplexCheckPassed    AuditEventType = "SIMPLEX_CHECK_PASSED"
)
```

---

## 4. The Decision Explainability Engine (`pkg/explain`)

The Explainability Engine transforms mathematical optimization outputs into transparent, intuitive explanations for dispatchers, planners, and carrier leadership.

```
┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                 DECISION EXPLAINABILITY PIPELINE                                 │
└─────────────────────────────────┬───────────────────────────────┬────────────────────────────────┘
                                  │                               │
                                  ▼                               ▼
┌───────────────────────────────────────────────────┐   ┌───────────────────────────────────────────────────┐
│        1. COUNTERFACTUAL MATRIX COMPARISON        │   │        2. CAUSAL ATTRIBUTION DECOMPOSITION        │
│                                                   │   │                                                   │
│ Compares the selected match against all rejected  │   │ Decomposes the score into:                        │
│ candidate arcs for that driver or load.           │   │ • Immediate Net Margin ($)                        │
│                                                   │   │ • Empty Deadhead Penalty ($)                      │
│ Matches: Driver 14 -> Load 102 (Score: +$1,840)   │   │ • HOS Dwell / Rest Insertion ($)                  │
│ Rejected: Driver 14 -> Load 105 (Score: +$1,120)  │   │ • Downstream Regional Valuation VFA ($)           │
│ Rejected: Driver 14 -> Hold Idle (Score: $0)      │   │ • Competitive Risk Premium MOMDP ($)              │
└───────────────────────────────────────────────────┘   └───────────────────────────────────────────────────┘
```

### 4.1 Why Was Driver $d$ Matched to Load $\ell_1$ Instead of $\ell_2$?

The explainability engine evaluates the **counterfactual delta**:

$$\Delta(d, \ell_1, \ell_2) = \bar{C}(d, \ell_1 \mid \theta) - \bar{C}(d, \ell_2 \mid \theta)$$

It produces an explainability record:

```json
{
  "explanation_type": "MATCH_DECISION",
  "driver_pseudonym": "D-***14",
  "assigned_load_id": "L-102",
  "winning_score": 1840.50,
  "economic_breakdown": {
    "gross_revenue": 2400.00,
    "loaded_drive_cost": 420.00,
    "empty_deadhead_cost": 45.00,
    "inserted_rest_dwell_cost": 0.00,
    "immediate_net_margin": 1935.00,
    "downstream_regional_vfa": 120.50,
    "competitor_risk_discount": -215.00
  },
  "top_rejected_alternatives": [
    {
      "load_id": "L-105",
      "score": 1120.00,
      "rejection_reason": "Excessive deadhead: 142 miles empty reposition required to pickup node."
    },
    {
      "load_id": "L-108",
      "score": -350.00,
      "rejection_reason": "HOS Infeasible: Exceeds 11-hour driving window without mandatory 10-hour sleeper break."
    }
  ]
}
```

### 4.2 Why Did the Engine Hold Driver $d$ Idle?

When a driver is left unassigned, the engine explains the economic and regulatory rationale:

```json
{
  "explanation_type": "DRIVER_HELD_IDLE",
  "driver_pseudonym": "D-***28",
  "location": "Dallas, TX",
  "reason_code": "NEGATIVE_EXPECTED_CONTRIBUTION",
  "summary": "All 4 candidate loads available within a 150-mile radius yielded negative net margins after factoring deadhead costs back to domicile.",
  "evaluated_candidates_count": 4,
  "best_negative_candidate": {
    "load_id": "L-204",
    "net_contribution": -142.50,
    "deadhead_miles": 185.0
  }
}
```

### 4.3 Why Did the System Shift Spot Bidding Posture?

In competitive MOMDP mode ($N=1$), the system explains pricing adjustments driven by Bayesian belief updates:

```json
{
  "explanation_type": "BELIEF_POSTURE_UPDATE",
  "corridor": "CHI -> ATL",
  "prior_belief": { "AGGRESSIVE": 0.15, "MODERATE": 0.65, "PASSIVE": 0.20 },
  "observed_signal": "LOST_3_BIDS_BELOW_BENCHMARK",
  "posterior_belief": { "AGGRESSIVE": 0.52, "MODERATE": 0.38, "PASSIVE": 0.10 },
  "action_taken": "Increased pricing bid premium from +2.0% to +8.5% to avoid winner's curse under aggressive competitor capacity concentration."
}
```

---

## 5. Implementation Roadmap & Package Boundaries

```
project_mittens/
├── pkg/
│   ├── journal/               # Lossless append-only record-replay store
│   │   ├── record.go          # JournalRecord, StateHash, Oracle reads
│   │   ├── store.go           # Memory, File, and Postgres store interfaces
│   │   └── replay.go          # Deterministic re-execution & verification
│   ├── audit/                 # Sanitized Section 19 operational event stream
│   │   ├── event.go           # AuditEvent, EventType, Masking
│   │   └── sink.go            # Structured JSON / OpenTelemetry log bridge
│   └── explain/               # Causal explanation & counterfactual engine
│       ├── counterfactual.go  # Compares alternative assignment scores
│       └── format.go          # JSON/Text explainability modal generation
```

### Dependency Boundaries:
* `pkg/journal`, `pkg/audit`, and `pkg/explain` are pure mathematical and data logging packages.
* They import `/internal/domain/model` and `/pkg/math`.
* They do not perform ambient I/O or access global wall-clock time during replay.

---

## 6. Summary for Joe & The Team

By adopting Stochflow's dual-track architecture:
1. **Zero Black-Box Complaints:** Every match, idle hold, and bid adjustment is accompanied by a transparent counterfactual breakdown.
2. **Instant Regression Testing:** Historical runs can be replayed in milliseconds with byte-exact verification across Go builds.
3. **Production Safety:** Operational audits remain 100% sanitized and compliant with Section 19 data privacy standards.
