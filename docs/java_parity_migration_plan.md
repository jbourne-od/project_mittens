# Project Mittens: Legacy Java Optimizer (coreai) Parity & Migration Blueprint

**Status:** Authoritative Architectural Mapping & Multi-Month Migration Roadmap  
**Date:** 2026-08-19  
**Source Repository:** `coreai/engine/smart_tl` (Read-Only Authority)  
**Governing Inviolates:** [Inviolate 0 (Explicit Config)](inviolates.md), [Inviolate 1 (Monopolistic Degeneracy & Parity)](inviolates.md), [Inviolate 2 (MOMDP Separation)](inviolates.md), [Inviolate 5 (Immutability)](inviolates.md)

---

## 1. Executive Assessment & Codebase Scale

A detailed inspection of the legacy Java optimization engine in `coreai/engine/smart_tl` reveals an enterprise-grade mathematical and operational optimization system developed over more than two decades (originating from Princeton University's CASTLE Laboratory and expanded into Optimal Dynamics' production engine).

### Codebase Scale
* **Worker Source Files:** 841 Java classes in `engine/smart_tl/worker/src/main/java`.
* **Worker Test Files:** 419 Java test suites in `engine/smart_tl/worker/src/test/java`.
* **HOS Simulation Library:** 74 Java classes in `engine/smart_tl/library/hos-simulator`.
* **Total Java Codebase:** ~1,300+ files and ~250,000+ lines of domain logic, optimization formulations, business rules, and simulation mechanics.

Replicating this system in Go with absolute numerical and functional parity ($N=0$) is a multi-month engineering program. No component can be approximated or simplified without violating Inviolate 1.

---

## 2. Functional Architecture & Module Decomposition

To achieve 100% functional replication, the Java codebase is decomposed into 11 distinct functional subsystems:

```
┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                       ORCHESTRATION & SIMULATION                                 │
│  FleetManager (8.7k LOC)  │  Simulator (Time-Stepping Loop)  │  StatisticCalculator (KPI Engine) │
└───────────────────────────────────────────────┬──────────────────────────────────────────────────┘
                                                │
         ┌──────────────────────────────────────┼──────────────────────────────────────┐
         │                                      │                                      │
┌────────▼────────┐                   ┌─────────▼─────────┐                  ┌─────────▼────────┐
│  SUBPROBLEMS &  │                   │    FEASIBILITY    │                  │  COST FUNCTIONS  │
│   OPTIMIZATION  │                   │   & ARC ENGINE    │                  │   & OBJECTIVES   │
│                 │                   │                   │                  │                  │
│ DriverDecision- │                   │ ArcInfeasibility  │                  │ CostFunctions    │
│  Subproblem     │                   │ FeasibilityChecker│                  │ RevenuePerTime   │
│ LocationTime-   │                   │ DwellTimeCalc     │                  │ LinearVFA / Vhat │
│  Subproblem     │                   │ ForkArcGenerator  │                  │ DiscountFactors  │
│ OrTools / MIP   │                   │ RelayEligibility  │                  │ BonusCalculators │
└────────┬────────┘                   └─────────┬─────────┘                  └─────────┬────────┘
         │                                      │                                      │
         └──────────────────────────────────────┼──────────────────────────────────────┘
                                                │
┌───────────────────────────────────────────────▼──────────────────────────────────────────────────┐
│                                         DOMAIN ATTRIBUTES                                        │
│  DriverAV (1.9k LOC)  │  LoadAV (5.2k LOC)  │  EquipmentAV  │  FacilityStore  │  RegionManager   │
└───────────────────────────────────────────────┬──────────────────────────────────────────────────┘
                                                │
         ┌──────────────────────────────────────┼──────────────────────────────────────┐
         │                                      │                                      │
┌────────▼────────┐                   ┌─────────▼─────────┐                  ┌─────────▼────────┐
│  HOS SIMULATOR  │                   │   RULES ENGINE    │                  │    PARAMETERS    │
│ (DOT 11/14/70h) │                   │    (CEL-based)    │                  │ (4.2k LOC Store) │
│                 │                   │                   │                  │                  │
│ DriverState     │                   │ RuleRegistry      │                  │ Parameters.java  │
│ EventSequencing │                   │ ContextVariables  │                  │ JsonParameters   │
│ Rest/SplitBreak │                   │ ModificationTarget│                  │ EarlySentinel99k │
└─────────────────┘                   └───────────────────┘                  └──────────────────┘
```

---

## 3. Subsystem Detailed Mapping

### Subsystem 1: Foundation & Core Primitives
* **Java Source:** `fleetmanager/foundation/*` (`Location`, `Time`, `TimeSpan`, `TimeZones`, `Comparables`).
* **Target Go Package:** `pkg/math` and `internal/domain/model`.
* **Key Invariants:**
  - Microsecond-accurate epoch time arithmetic with explicit timezone handling.
  - Coordinate distance geometry (Haversine & great-circle mileage calculations matching Java `Location.distanceTo`).

### Subsystem 2: High-Fidelity HOS Simulation Engine
* **Java Source:** `coreai/engine/smart_tl/library/hos-simulator` (74 classes).
* **Target Go Package:** `internal/domain/model/hos` (or dedicated subpackage).
* **Key Features to Replicate:**
  - Complete DOT Hours of Service regulations:
    - 11-hour driving limit within a 14-hour on-duty window.
    - 14-hour consecutive on-duty window following 10 consecutive hours off-duty.
    - 70-hour / 8-day rolling cycle rule.
    - 34-hour restart provisions.
    - Sleeper berth split rules (8/2 and 7/3 split sleeper breaks).
    - 30-minute mandatory rest break after 8 hours of driving.
  - Event sequence simulator: `Drive`, `Loading`, `Unloading`, `Rest`, `Hold`, `BorderCrossing`, `UpdateLocation`.
  - Carry-forward state projection (`releaseCarryForwardDriverState`) allowing forward simulation of driver duty cycles.

### Subsystem 3: Domain Attribute Value Objects
* **Java Source:** `fleetmanager/attributes/*` (`DriverAV.java` [1.9k LOC], `LoadAV.java` [5.2k LOC], `EquipmentAV.java`, `FacilityAV.java`, `RegionManager.java`, `TradeAreaManager.java`, `DomicileStore.java`).
* **Target Go Package:** `internal/domain/model`.
* **Key Features to Replicate:**
  - **`DriverAV`:**
    - Static attributes: Driver ID, team vs solo, equipment type, home domicile, driver pay rates, preferred operating regions.
    - Dynamic attributes: Current location, time-location reference (`CURRENT` vs `NEXT`), available time / PTA, remaining drive/duty/cycle hours, active load assignment, home-time requests, actionable flags.
  - **`LoadAV`:**
    - Pickup/delivery windows $[t_{early}, t_{late}]$ with early arrival sentinels (`ALLOWED_EARLY_NEVER_WAIT_SENTINEL_MINUTES = 99999`).
    - Revenue, weight, piece count, required equipment (van, reefer, flatbed, hazmat), driver whitelists, emergency load flags.
    - Exclusion rules: `revenueExclusion`, `timeSpanExclusion`, `bogusTimeExclusion`, `cancellationExclusion`, `delayedExclusion`.
    - Dwell time distributions at origin and destination facilities.
  - **Facilities & Regions:**
    - `FacilityStore`: operating hours, appointment types (live load vs drop/hook), average dwell time.
    - `SquareRegion` & `RegionManager`: geographic hierarchical aggregation for network balance.

### Subsystem 4: Explicit Configuration & Parameters Engine
* **Java Source:** `fleetmanager/Parameters.java` (4.2k LOC) and `fleetmanager/jsonparameters/*` (30+ parameter structs).
* **Target Go Package:** `internal/domain/config` (or strongly typed parameter structs injected per Inviolate 0).
* **Key Parameter Categories:**
  - `SimulationParameters`: start epoch, end epoch, time step duration, planning horizon length.
  - `OptimizationParameters`: solver time limits, MIP gap tolerances, branch-and-bound bounds.
  - `CostParameters`: loaded mileage rate, empty mileage rate, fixed cost per dispatch, empty-to-home penalty, driver bonus multipliers.
  - `OnTimeParameters`: early arrival penalties, late delivery penalties, grace periods.
  - `HOSParameters`: DOT rule selection (US 70/8 vs Texas intrastate vs Canadian regulations).
  - `RelayParameters` & `StickyTourParameters`.

### Subsystem 5: Feasibility Engine & Arc Generation
* **Java Source:** `fleetmanager/optimization/feasibility/*` (`FeasibilityChecker`, `ArcInfeasibility`, `ArcInfeasibilityStore`, `AutoCalibrationDiagnosticStore`), `ForkArcGenerator.java`, `RelayEligibilityChecker.java`.
* **Target Go Package:** `internal/domain/model/feasibility` and `internal/domain/policy`.
* **Key Feasibility Invariants:**
  - Time feasibility: Driver arrival time $\le$ Load latest pickup time; Driver delivery time $\le$ Load latest delivery time.
  - HOS feasibility: Required transit hours $\le$ Driver available drive hours; required duty cycle $\le$ remaining 70-hour window.
  - Equipment compatibility: Tractor equipment capability matches load equipment requirement.
  - Geographical / driver preference constraints: Driver domicile return feasibility within home-time window.
  - Multi-leg fork tours: Enumeration of multi-stop dispatches, relay handoffs, and bobtail repositioning movements.

### Subsystem 6: Objective Functions & Cost Modeling
* **Java Source:** `fleetmanager/decision/CostFunctions.java`, `RevenuePerTime.java`, `LinearVFAManager.java`, `SystemVFAValues.java`, `SmoothParams.java`.
* **Target Go Package:** `internal/domain/policy/cfa` and `internal/domain/policy/vfa`.
* **Exact Cost Formula:**
  $$\text{TotalCost}(d, \ell) = \text{FixedCost} + \text{LoadedMilesCost} + \text{EmptyMilesCost} + \text{EmptyToHomeCost} - \text{Bonus} - \bar{V}_t(S^x)$$
  Where:
  - $\text{FixedCost} = \text{unitCostFixedPerLoad}$
  - $\text{LoadedMilesCost} = \text{loadedMiles} \times \text{unitCostLoadedRate}$
  - $\text{EmptyMilesCost} = \text{emptyMiles} \times \text{unitCostEmptyRate}$
  - $\text{EmptyToHomeCost} = \text{emptyToHomeMiles} \times \text{unitCostEmptyToHome}$
  - $\text{Bonus} = \text{RuleBonuses} + \text{DriverRetentionBonus}$
  - $\bar{V}_t(S^x) = \text{Post-Decision State Value (Linear/Piecewise VFA)}$

### Subsystem 7: Optimization Network & Subproblem Formulation
* **Java Source:** `fleetmanager/optimization/DriverDecisionSubproblem.java` (8.7k LOC), `LocationTimeSubproblem.java`, `OrToolsSubproblemSolver.java`, `LPModelRequestBuilder.java`.
* **Target Go Package:** `internal/domain/policy` and `internal/service/solver`.
* **MIP Formulation:**
  $$\min \sum_{d \in \mathcal{D}} \sum_{\ell \in \mathcal{L}} \sum_{m \in \mathcal{M}} c(d, \ell, m) \cdot x_{d, \ell, m}$$
  Subject to:
  - Driver capacity: $\sum_{\ell, m} x_{d, \ell, m} \le 1, \quad \forall d \in \mathcal{D}$
  - Load fulfillment: $\sum_{d, m} x_{d, \ell, m} \le 1, \quad \forall \ell \in \mathcal{L}$
  - Feasibility: $x_{d, \ell, m} = 0 \text{ if } \text{Infeasible}(d, \ell, m)$
  - Integrality: $x_{d, \ell, m} \in \{0, 1\}$

### Subsystem 8: Dispatching & Tour Building
* **Java Source:** `fleetmanager/dispatching/*` (`DispatchRunner`, `DriverTour`, `TourComponent`, `RelaxationService`).
* **Target Go Package:** `internal/service/dispatch`.
* **Key Features:**
  - Multi-day driver tour synthesis.
  - Driver relay coordination at intermediate exchange facilities.
  - Solution extraction mapping MIP binary variables back into executable driver work orders.

### Subsystem 9: Business Rules Engine (CEL-based)
* **Java Source:** `fleetmanager/rules/*` (`SmartTlRuleRegistry`, `DriverContextVariables`, `LoadContextVariables`, `SmartTlRuleAdjustmentHelper`).
* **Target Go Package:** `internal/domain/rules` (using Go CEL interpreter `google/cel-go`).
* **Key Features:**
  - Evaluates CEL expressions against `(Driver, Load)` context.
  - Applies atomic modification actions (`add`, `multiply`, `override`, `ban`) to costs, bonuses, and feasibility flags.

### Subsystem 10: Discrete Event Simulation Loop
* **Java Source:** `fleetmanager/FleetManager.java`, `fleetmanager/Simulator.java`, `fleetmanager/statcalculator/*`.
* **Target Go Package:** `internal/service/simulator`.
* **Key Features:**
  - Time-stepping simulation across rolling horizons (e.g. 7-day to 28-day simulations).
  - Driver HOS updates, load arrival injections, decision executions, and KPI analytics (profit, revenue, loaded/empty ratio, service percentage).

### Subsystem 11: Parity Test Suites & Dual-Run Golden Fixtures
* **Java Source:** `engine/smart_tl/worker/src/test/java/fleetmanager/test/scenarios/*`.
* **Target Go Package:** `internal/adapter/legacy`.
* **Key Strategy:**
  - Export real-world serialized scenario inputs from Java runs (`data/Carriers/TEMPLATE/`).
  - Run side-by-side Go execution on $N=0$ configuration.
  - Assert exact match on matched $(d, \ell)$ pairs, dispatch timestamps, and route payouts within $1e-9$ numerical tolerance (Inviolate 1).

---

## 4. Multi-Phase Migration Roadmap

```
┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
│ PHASE 1: Mathematical Primitives & Simplex Foundation (pkg/math) [COMPLETED]                      │
└───────────────────────────────────┬──────────────────────────────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼──────────────────────────────────────────────────────────────┐
│ PHASE 2: Core Domain Primitives & Geography (Time, Location, Mileage, Basic Entities)            │
└───────────────────────────────────┬──────────────────────────────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼──────────────────────────────────────────────────────────────┐
│ PHASE 3: High-Fidelity HOS Simulation Engine (DOT 11/14/70, Split Breaks, Event Sequencing)      │
└───────────────────────────────────┬──────────────────────────────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼──────────────────────────────────────────────────────────────┐
│ PHASE 4: Complete Attribute Value Architecture (DriverAV, LoadAV, Facilities, Equipment)          │
└───────────────────────────────────┬──────────────────────────────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼──────────────────────────────────────────────────────────────┐
│ PHASE 5: Strongly Typed Parameter & Configuration Engine (Parameters.java Reimplementation)      │
└───────────────────────────────────┬──────────────────────────────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼──────────────────────────────────────────────────────────────┐
│ PHASE 6: Physical Feasibility Engine & Arc Generation (Time Windows, Dwell, Equipment, Relays)   │
└───────────────────────────────────┬──────────────────────────────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼──────────────────────────────────────────────────────────────┐
│ PHASE 7: Cost Functions, Objective Formulations & Linear/Piecewise VFA                           │
└───────────────────────────────────┬──────────────────────────────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼──────────────────────────────────────────────────────────────┐
│ PHASE 8: CEL Business Rules Engine Integration (google/cel-go evaluation on context)             │
└───────────────────────────────────┬──────────────────────────────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼──────────────────────────────────────────────────────────────┐
│ PHASE 9: Optimization Network Formulation & Subproblem Solvers (MIP / Flow Solvers)              │
└───────────────────────────────────┬──────────────────────────────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼──────────────────────────────────────────────────────────────┐
│ PHASE 10: Dispatch Runner, Tour Construction & Relaxation Engine                                 │
└───────────────────────────────────┬──────────────────────────────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼──────────────────────────────────────────────────────────────┐
│ PHASE 11: Full Simulation Orchestrator & Dual-Run Legacy Parity Benchmark Verification (N=0)     │
└──────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 5. Parity Verification Standard (Inviolate 1)

Every phase must include golden comparison test suites executing real scenario data from `coreai/engine/smart_tl/data/Carriers/` asserting:
1. **Physical Feasibility Parity:** Given identical driver and load states, the Go feasibility checker must produce bit-wise identical feasible candidate arc sets as Java's `FeasibilityChecker`.
2. **Cost Valuation Parity:** Evaluated arc costs $\text{TotalCost}(d, \ell)$ must match Java's `CostFunctions` output within $1e-9$ precision.
3. **MIP Solution Parity:** Optimization match solutions $x^*_{d, \ell}$ in $N=0$ monopolistic mode must select the exact same driver-load pairings and yield equivalent network contribution.
4. **Zero Ambient Configuration:** All parameters must be passed as explicit Go structs (Inviolate 0).
