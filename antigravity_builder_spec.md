# Project Mittens: Technical Implementation Specification & Architecture Blueprint for Antigravity

This document provides a concrete, highly grounded architectural specification for the modernization and generalization of Dr. Warren Powell's sequential logistics optimizer in Go. It translates the theoretical mathematics of **Sequential Decision Analytics (SDA)** and **Partially Observable Markov Decision Processes (POMDPs)** into compile-ready Go structures, design patterns, and concurrency templates.

The core programming agent (**antigravity**) must strictly follow this specification, implementing the models, particle filtering, and parallel solver loops exactly as defined while obeying all constraints in [`INVARIANTS.md`](file:///Users/jacob/Development/od/project_mittens/INVARIANTS.md) and [`AGENTS.md`](file:///Users/jacob/Development/od/project_mittens/AGENTS.md).

---

## 1. Directory Structure & Package Architecture

The repository is organized following idiomatic Go package boundaries. Domain mathematics are cleanly separated from concurrency orchestration, data caches, and runtime telemetry.

```
/Users/jacob/Development/od/project_mittens/
├── cmd/
│   └── optimizer/
│       └── main.go                 # CLI entry point, seed initialization, simulation harness
├── internal/
│   ├── domain/
│   │   ├── state.go                # Physical State (R_t), Latent Market State (M_t), unified State
│   │   ├── decision.go             # Decision vector (x_t) and operational HOS pruning rules
│   │   ├── exogenous.go            # Exogenous Information (W_t), tenders, and market signals
│   │   ├── transition.go           # System physics: S^M(S_t, x_t, W_{t+1})
│   │   ├── objective.go            # Contribution function C(S_t, x_t) and VFA/CFA interfaces
│   │   └── belief.go               # Particle Filter structure, Bayesian updater, and Jittering
│   ├── usecase/
│   │   └── solver/
│   │       ├── pomcp.go            # Parallelized POMCP (PO-UCT) solver loop with worker pools
│   │       ├── rollout.go          # Monte Carlo simulator and rollout policy
│   │       └── cache.go            # Thread-safe Value Function Approximation (VFA) cache
│   └── infrastructure/
│       └── simulator/
│           └── market_sim.go       # Environment generative model for testing and rollouts
```

> [!NOTE]
> **Package Layout Mapping**: In accordance with [`AGENTS.md`](file:///Users/jacob/Development/od/project_mittens/AGENTS.md), domain packages under `internal/domain/` contain the core SDA/POMDP vocabulary (`state`, `decision`, `exogenous`, `transition`, `objective`, `belief`). Solver and search algorithms reside under `internal/usecase/solver/`. All imports must use the canonical module path `github.com/optimaldynamics/project-mittens/...`.

---

## 2. Go Translation of Powell's 5 Core SDA Elements

The transition from Java to Go eliminates object-allocation overhead and memory fragmentation. Multi-dimensional fleet and market states are represented as flat, contiguous, cache-friendly slices of structs. 

Below are the domain definitions that must be implemented.

### 2.1 State Definitions (`internal/domain/state.go`)

```go
package domain

import "time"

// DriverStatus represents the current physical/regulatory state of a driver.
type DriverStatus int

const (
	DriverIdling DriverStatus = iota
	DriverInTransit
	DriverDeadheading
	DriverHOSRest
	DriverOffDuty
)

// DriverState represents the fully observable physical state of an individual driver asset (R_t^d).
type DriverState struct {
	ID                 int64
	CurrentLatitude    float64
	CurrentLongitude   float64
	Status             DriverStatus
	AvailableTime      time.Time
	CumulativeDriveHrs float64 // Tracked for 11-hour driving limit (Invariant 1.2)
	CumulativeDutyHrs  float64 // Tracked for 14-hour on-duty limit (Invariant 1.2)
	ConsecutiveRestHrs float64 // Tracked for 10-hour rest compliance (Invariant 1.2)
}

// LoadStatus represents the operational status of a shipper-tendered load.
type LoadStatus int

const (
	LoadPending LoadStatus = iota
	LoadAssigned
	LoadInTransit
	LoadDelivered
	LoadExpired
)

// LoadState represents the fully observable physical state of a cargo load (R_t^l).
type LoadState struct {
	ID             int64
	OriginLat      float64
	OriginLon      float64
	DestLat        float64
	DestLon        float64
	ReadyTime      time.Time
	ExpirationTime time.Time
	BaseRevenue    float64
}

// ObservableState represents the full observable physical state of the fleet (R_t).
type ObservableState struct {
	Drivers []DriverState
	Loads   []LoadState
	Epoch   time.Time
}

// Copy returns a deep copy of the ObservableState.
func (o ObservableState) Copy() ObservableState {
	drivers := make([]DriverState, len(o.Drivers))
	copy(drivers, o.Drivers)

	loads := make([]LoadState, len(o.Loads))
	copy(loads, o.Loads)

	return ObservableState{
		Drivers: drivers,
		Loads:   loads,
		Epoch:   o.Epoch,
	}
}

// LatentMarketState represents the partially observable, uncoupled or coupled market variables (M_t).
type LatentMarketState struct {
	// CompetitorCapacityDensity maps regional zip/geohash codes to estimated active competitor trucks.
	CompetitorCapacityDensity map[string]float64

	// ShipperSatisfaction maps shipper IDs to latent service satisfaction indices [0.0 - 1.0].
	ShipperSatisfaction map[int64]float64

	// ShipperPriceSensitivity maps shipper IDs to latent price elasticity coefficients.
	ShipperPriceSensitivity map[int64]float64
}

// Copy returns a deep copy of the LatentMarketState.
func (l LatentMarketState) Copy() LatentMarketState {
	compDensity := make(map[string]float64, len(l.CompetitorCapacityDensity))
	for k, v := range l.CompetitorCapacityDensity {
		compDensity[k] = v
	}

	shipperSat := make(map[int64]float64, len(l.ShipperSatisfaction))
	for k, v := range l.ShipperSatisfaction {
		shipperSat[k] = v
	}

	shipperSens := make(map[int64]float64, len(l.ShipperPriceSensitivity))
	for k, v := range l.ShipperPriceSensitivity {
		shipperSens[k] = v
	}

	return LatentMarketState{
		CompetitorCapacityDensity: compDensity,
		ShipperSatisfaction:       shipperSat,
		ShipperPriceSensitivity:   shipperSens,
	}
}

// State represents the unified state vector S_t = (R_t, M_t).
type State struct {
	Observable ObservableState
	Latent     LatentMarketState
}

// Copy returns a deep copy of the state to prevent mutations during simulation rollouts.
func (s State) Copy() State {
	return State{
		Observable: s.Observable.Copy(),
		Latent:     s.Latent.Copy(),
	}
}
```

### 2.2 Decision Definitions (`internal/domain/decision.go`)

```go
package domain

// AssignmentType represents the action class for an individual asset.
type AssignmentType int

const (
	AssignToLoad AssignmentType = iota
	DeadheadToRegion
	TriggerHOSRest
	RejectContractTender
)

// Assignment represents the decision tuple x_t^d for a single driver.
type Assignment struct {
	DriverID  int64
	LoadID    int64          // Used if AssignToLoad
	TargetLat float64        // Destination or Deadhead latitude
	TargetLon float64        // Destination or Deadhead longitude
	BidPrice  float64        // Dynamic pricing action for spot market bids
	Type      AssignmentType
}

// Decision represents the full decision vector x_t matching all drivers.
type Decision []Assignment

// PruneFeasibleRegion filters assignments based on physical limits and Hours of Service (Invariant 1.2).
func PruneFeasibleRegion(s State, d Decision) Decision {
	feasible := make(Decision, 0, len(d))
	driverMap := make(map[int64]DriverState, len(s.Observable.Drivers))
	for _, drv := range s.Observable.Drivers {
		driverMap[drv.ID] = drv
	}

	assignedDrivers := make(map[int64]bool)

	for _, assign := range d {
		drv, exists := driverMap[assign.DriverID]
		if !exists {
			continue // Invalid driver ID
		}

		// Invariant 1.1: Prevent multi-location allocation
		if assignedDrivers[assign.DriverID] {
			continue
		}

		// Invariant 1.2: Strict Hours of Service Pruning
		if assign.Type == AssignToLoad {
			tripTimeHrs := CalculateTransitTime(drv.CurrentLatitude, drv.CurrentLongitude, assign.TargetLat, assign.TargetLon)
			if drv.CumulativeDriveHrs+tripTimeHrs > 11.0 && drv.ConsecutiveRestHrs < 10.0 {
				// Prune this assignment; driver must complete mandatory rest cycle
				continue
			}
		}

		assignedDrivers[assign.DriverID] = true
		feasible = append(feasible, assign)
	}
	return feasible
}

func CalculateTransitTime(lat1, lon1, lat2, lon2 float64) float64 {
	dist := CalculateDistance(lat1, lon1, lat2, lon2)
	const averageSpeedMph = 55.0
	return dist / averageSpeedMph
}
```

### 2.3 Exogenous Information (`internal/domain/exogenous.go`)

```go
package domain

import "time"

// ObservationType represents the type of exogenous signal received from the market.
type ObservationType int

const (
	TenderArrival ObservationType = iota
	TenderAccepted // Shipper accepted our dynamic rate bid
	TenderRejected // Shipper rejected our dynamic rate bid (lost to competitor)
	SpotPriceIndexUpdate
	GPSPositionPing
)

// ExogenousSignal represents the W_{t+1} event.
type ExogenousSignal struct {
	Type        ObservationType
	Timestamp   time.Time
	LoadID      int64
	ShipperID   int64
	SpotPrice   float64
	FeedbackLat float64 // Used to infer competitor coordinates if tender was rejected
	FeedbackLon float64
}

// ExogenousInfo contains all concurrent events occurring between t and t+1.
type ExogenousInfo []ExogenousSignal
```

### 2.4 Transition Function (`internal/domain/transition.go`)

```go
package domain

import "time"

// TransitionModel represents the state transition logic S_{t+1} = S^M(S_t, x_t, W_{t+1}).
type TransitionModel struct {
	BaseSpeedMph float64
}

// Transition implements the physics of the fleet transition combined with exogenous market dynamics.
func (tm *TransitionModel) Transition(s State, x Decision, w ExogenousInfo) State {
	nextState := s.Copy()

	// 1. Apply physical decisions (Decisions move drivers, update HOS, etc.)
	driverMap := make(map[int64]*DriverState, len(nextState.Observable.Drivers))
	for i := range nextState.Observable.Drivers {
		drv := &nextState.Observable.Drivers[i]
		driverMap[drv.ID] = drv
	}

	for _, assign := range x {
		drv, exists := driverMap[assign.DriverID]
		if !exists {
			continue
		}

		switch assign.Type {
		case AssignToLoad:
			drv.Status = DriverInTransit
			transitHrs := CalculateTransitTime(drv.CurrentLatitude, drv.CurrentLongitude, assign.TargetLat, assign.TargetLon)
			drv.CumulativeDriveHrs += transitHrs
			drv.CumulativeDutyHrs += transitHrs
			drv.ConsecutiveRestHrs = 0.0
			drv.CurrentLatitude = assign.TargetLat
			drv.CurrentLongitude = assign.TargetLon

		case DeadheadToRegion:
			drv.Status = DriverDeadheading
			transitHrs := CalculateTransitTime(drv.CurrentLatitude, drv.CurrentLongitude, assign.TargetLat, assign.TargetLon)
			drv.CumulativeDriveHrs += transitHrs
			drv.CumulativeDutyHrs += transitHrs
			drv.ConsecutiveRestHrs = 0.0
			drv.CurrentLatitude = assign.TargetLat
			drv.CurrentLongitude = assign.TargetLon

		case TriggerHOSRest:
			drv.Status = DriverHOSRest
			drv.ConsecutiveRestHrs = 10.0
			drv.CumulativeDriveHrs = 0.0
			drv.CumulativeDutyHrs = 0.0
		}
	}

	// 2. Apply Exogenous Updates (W_{t+1})
	for _, signal := range w {
		switch signal.Type {
		case TenderArrival:
			// New tenders are appended to active load list
		case TenderRejected:
			// Coupled market state transition: reduce satisfaction and increase estimated competitor density
			if sat, ok := nextState.Latent.ShipperSatisfaction[signal.ShipperID]; ok {
				nextState.Latent.ShipperSatisfaction[signal.ShipperID] = sat * 0.95
			}
			geohash := GetRegionGeohash(signal.FeedbackLat, signal.FeedbackLon)
			nextState.Latent.CompetitorCapacityDensity[geohash] += 1.0

		case TenderAccepted:
			// Coupled market state transition: boost shipper satisfaction
			if sat, ok := nextState.Latent.ShipperSatisfaction[signal.ShipperID]; ok {
				nextState.Latent.ShipperSatisfaction[signal.ShipperID] = min(1.0, sat*1.05)
			}
		}
	}

	// Increment Epoch Time
	nextState.Observable.Epoch = nextState.Observable.Epoch.Add(15 * time.Minute)
	return nextState
}

func GetRegionGeohash(lat, lon float64) string {
	return "US-NE-1" // Regional spatial partition placeholder
}
```

### 2.5 Objective Function (`internal/domain/objective.go`)

```go
package domain

import "math"

// ObjectiveFunction calculates immediate contributions and terminal values.
type ObjectiveFunction interface {
	ComputeContribution(s State, x Decision) float64
	EvaluateVFA(s State) float64
}

type StandardObjective struct {
	OperatingCostPerMile float64
	LateDeliveryPenalty  float64
}

func (so *StandardObjective) ComputeContribution(s State, x Decision) float64 {
	var totalContribution float64
	driverMap := make(map[int64]DriverState, len(s.Observable.Drivers))
	for _, drv := range s.Observable.Drivers {
		driverMap[drv.ID] = drv
	}
	loadMap := make(map[int64]LoadState, len(s.Observable.Loads))
	for _, ld := range s.Observable.Loads {
		loadMap[ld.ID] = ld
	}

	for _, assign := range x {
		drv, exists := driverMap[assign.DriverID]
		if !exists {
			continue
		}

		switch assign.Type {
		case AssignToLoad:
			ld, ok := loadMap[assign.LoadID]
			if !ok {
				continue
			}
			dist := CalculateDistance(drv.CurrentLatitude, drv.CurrentLongitude, ld.OriginLat, ld.OriginLon) +
				CalculateDistance(ld.OriginLat, ld.OriginLon, ld.DestLat, ld.DestLon)

			revenue := ld.BaseRevenue
			if assign.BidPrice > 0 {
				revenue = assign.BidPrice
			}
			cost := dist * so.OperatingCostPerMile
			totalContribution += (revenue - cost)

		case DeadheadToRegion:
			dist := CalculateDistance(drv.CurrentLatitude, drv.CurrentLongitude, assign.TargetLat, assign.TargetLon)
			totalContribution -= (dist * so.OperatingCostPerMile)
		}
	}
	return totalContribution
}

func (so *StandardObjective) EvaluateVFA(s State) float64 {
	return 0.0
}

// CalculateDistance computes great-circle distance in miles using the Haversine formula.
func CalculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusMiles = 3958.8
	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)

	rLat1 := lat1 * (math.Pi / 180.0)
	rLat2 := lat2 * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(rLat1)*math.Cos(rLat2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusMiles * c
}
```

---

## 3. Belief Tracking & Particle Filter Specification

Because competitor truck distributions and shipper satisfaction coefficients are partially observable, the belief state $b_t$ is maintained as a set of discrete particles. Each particle is a struct of type `LatentMarketState`.

```
           ┌──────────────────────────────────────────────┐
           │     Incoming Observation W_t (Feedback)      │
           └──────────────────────┬───────────────────────┘
                                  ▼
           ┌──────────────────────────────────────────────┐
           │         Bayesian Particle Weighting          │
           │  w_i = P(Observation | Particle_i, Action)   │
           └──────────────────────┬───────────────────────┘
                                  ▼
           ┌──────────────────────────────────────────────┐
           │        Systematic Resampling Routine         │
           │      Normalized Weights -> New Particles     │
           └──────────────────────┬───────────────────────┘
                                  │
                  Is Neff < Threshold (Kappa)?
                     ├── YES ──> [Jitter / Reinvigorate (Invariant 4.2)]
                     └── NO ───> [Normalize & Store]
```

### 3.1 Particle Filter Engine (`internal/domain/belief.go`)

```go
package domain

import (
	"math/rand/v2"
	"sync"
)

// ParticleFilter manages the belief state b_t over latent market states.
type ParticleFilter struct {
	mu        sync.RWMutex
	Particles []LatentMarketState
	Kappa     float64 // Minimum effective sample size (N_eff) threshold (Invariant 4.2)
	SeedPRNG  *rand.Rand
}

func NewParticleFilter(numParticles int, initialLatent LatentMarketState, seed uint64) *ParticleFilter {
	particles := make([]LatentMarketState, numParticles)
	src := rand.NewPCG(seed, seed+1)
	prng := rand.New(src)

	for i := 0; i < numParticles; i++ {
		particles[i] = LatentMarketState{
			CompetitorCapacityDensity: cloneMapAndVariance(initialLatent.CompetitorCapacityDensity, prng),
			ShipperSatisfaction:       cloneMapAndVariance(initialLatent.ShipperSatisfaction, prng),
			ShipperPriceSensitivity:   cloneMapAndVariance(initialLatent.ShipperPriceSensitivity, prng),
		}
	}

	return &ParticleFilter{
		Particles: particles,
		Kappa:     float64(numParticles) * 0.4, // Invariant 4.2 threshold: 40% of particles
		SeedPRNG:  prng,
	}
}

// Sample returns a random latent market state from the particle belief.
func (pf *ParticleFilter) Sample() LatentMarketState {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	idx := pf.SeedPRNG.IntN(len(pf.Particles))
	return pf.Particles[idx].Copy()
}

// Update implements the sequential Bayesian updater and resampling loop.
func (pf *ParticleFilter) Update(action Decision, signal ExogenousSignal) {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	n := len(pf.Particles)
	weights := make([]float64, n)
	sumWeights := 0.0

	// 1. Calculate importance weights based on likelihood function
	for i := 0; i < n; i++ {
		likelihood := pf.calculateLikelihood(pf.Particles[i], action, signal)
		weights[i] = likelihood
		sumWeights += likelihood
	}

	// Handle total weight degeneracy
	if sumWeights == 0.0 {
		pf.reinvigorateBeliefs()
		return
	}

	// Normalize weights (Invariant 4.1)
	for i := 0; i < n; i++ {
		weights[i] /= sumWeights
	}

	// 2. Check Effective Sample Size N_eff to prevent particle depletion (Invariant 4.2)
	nEff := pf.calculateNeff(weights)
	if nEff < pf.Kappa {
		pf.resampleAndJitter(weights)
	} else {
		pf.resample(weights)
	}
}

func (pf *ParticleFilter) calculateLikelihood(p LatentMarketState, act Decision, sig ExogenousSignal) float64 {
	switch sig.Type {
	case TenderRejected:
		geohash := GetRegionGeohash(sig.FeedbackLat, sig.FeedbackLon)
		density := p.CompetitorCapacityDensity[geohash]
		return min(1.0, 0.1+(density*0.1))
	case TenderAccepted:
		geohash := GetRegionGeohash(sig.FeedbackLat, sig.FeedbackLon)
		density := p.CompetitorCapacityDensity[geohash]
		sensitivity := p.ShipperPriceSensitivity[sig.ShipperID]
		return max(0.01, 1.0-(density*0.08)-(sensitivity*0.2))
	default:
		return 1.0
	}
}

func (pf *ParticleFilter) calculateNeff(weights []float64) float64 {
	sumSq := 0.0
	for _, w := range weights {
		sumSq += w * w
	}
	return 1.0 / sumSq
}

// resampleAndJitter implements Systematic Resampling with Gaussian noise (Jittering) for Invariant 4.2.
func (pf *ParticleFilter) resampleAndJitter(weights []float64) {
	n := len(pf.Particles)
	newParticles := make([]LatentMarketState, n)

	c := make([]float64, n)
	c[0] = weights[0]
	for i := 1; i < n; i++ {
		c[i] = c[i-1] + weights[i]
	}

	u := pf.SeedPRNG.Float64() / float64(n)
	i := 0

	for j := 0; j < n; j++ {
		uj := u + float64(j)/float64(n)
		for uj > c[i] && i < n-1 {
			i++
		}

		newParticles[j] = LatentMarketState{
			CompetitorCapacityDensity: addJitter(pf.Particles[i].CompetitorCapacityDensity, pf.SeedPRNG, 0.1),
			ShipperSatisfaction:       addJitter(pf.Particles[i].ShipperSatisfaction, pf.SeedPRNG, 0.05),
			ShipperPriceSensitivity:   addJitter(pf.Particles[i].ShipperPriceSensitivity, pf.SeedPRNG, 0.05),
		}
	}

	pf.Particles = newParticles
}

func (pf *ParticleFilter) resample(weights []float64) {
	n := len(pf.Particles)
	newParticles := make([]LatentMarketState, n)

	c := make([]float64, n)
	c[0] = weights[0]
	for i := 1; i < n; i++ {
		c[i] = c[i-1] + weights[i]
	}

	u := pf.SeedPRNG.Float64() / float64(n)
	i := 0

	for j := 0; j < n; j++ {
		uj := u + float64(j)/float64(n)
		for uj > c[i] && i < n-1 {
			i++
		}
		newParticles[j] = pf.Particles[i].Copy()
	}

	pf.Particles = newParticles
}

func (pf *ParticleFilter) reinvigorateBeliefs() {
	n := len(pf.Particles)
	for i := 0; i < n; i++ {
		for k := range pf.Particles[i].CompetitorCapacityDensity {
			pf.Particles[i].CompetitorCapacityDensity[k] = pf.SeedPRNG.Float64() * 10.0
		}
	}
}

func cloneMapAndVariance(m map[string]float64, prng *rand.Rand) map[string]float64 {
	res := make(map[string]float64, len(m))
	for k, v := range m {
		res[k] = max(0.0, v+(prng.NormFloat64()*0.1))
	}
	return res
}

func addJitter(m map[string]float64, prng *rand.Rand, stdDev float64) map[string]float64 {
	res := make(map[string]float64, len(m))
	for k, v := range m {
		res[k] = max(0.0, v+(prng.NormFloat64()*stdDev))
	}
	return res
}
```

---

## 4. Parallelized POMCP Solver Implementation Specification

The online planner solves the Partially Observable Monte Carlo Planning problem using Monte Carlo Tree Search (MCTS) initialized from the current belief. It executes simulation rollouts concurrently across a bounded worker pool, strictly obeying real-time runtime limits via Go's context propagation (Invariants 5.1–5.3).

### 4.1 Tree Node Structure (`internal/usecase/solver/pomcp.go`)

```go
package solver

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain"
)

// TreeNode represents a search node in the history tree (MCTS state).
type TreeNode struct {
	N           int64
	ValueSum    float64
	ActionNodes map[string]*ActionNode
	mu          sync.RWMutex
}

type ActionNode struct {
	N           int64
	ValueSum    float64
	Observation map[string]*TreeNode
	mu          sync.RWMutex
}

type RolloutJob struct {
	Index        int
	State        domain.State
	HorizonSteps int
}

type RolloutOutput struct {
	CumulativeReward float64
	History          []string
	Err              error
}

type POMCPSolver struct {
	WorkerPoolSize int
	SimCount       int
	MaxHorizon     int
	ExplorationC   float64
	BaseSeed       uint64
	Model          domain.TransitionModel
	Objective      domain.StandardObjective
}

// RunPlanning initiates parallel simulation jobs and enforces real-time timeout limits.
func (ps *POMCPSolver) RunPlanning(ctx context.Context, currentObservable domain.ObservableState, pf *domain.ParticleFilter) (domain.Decision, error) {
	// Bounded coordination channels to prevent excessive allocations
	queueCapacity := ps.WorkerPoolSize * 2
	jobChan := make(chan RolloutJob, queueCapacity)
	outputChan := make(chan RolloutOutput, queueCapacity)

	var wg sync.WaitGroup
	var activeWorkers int32

	// 1. Create worker pool with partitioned PRNG streams (Invariant 5.3)
	for i := 0; i < ps.WorkerPoolSize; i++ {
		wg.Add(1)
		atomic.AddInt32(&activeWorkers, 1)
		workerSeed := ps.BaseSeed + uint64(i)*10007

		go func(workerID int, seed uint64) {
			defer wg.Done()
			defer atomic.AddInt32(&activeWorkers, -1)

			src := rand.NewPCG(seed, seed+1)
			workerPRNG := rand.New(src)

			for {
				select {
				case <-ctx.Done():
					return // Graceful cancellation (Invariant 5.2)
				case job, ok := <-jobChan:
					if !ok {
						return
					}
					reward, hist, err := ps.executeRolloutWithRNG(ctx, job, workerPRNG)
					select {
					case <-ctx.Done():
						return
					case outputChan <- RolloutOutput{CumulativeReward: reward, History: hist, Err: err}:
					}
				}
			}
		}(i, workerSeed)
	}

	// 2. Feed simulation jobs by sampling from Particle Belief State (Invariant 3.1)
	go func() {
		defer close(jobChan)

		for i := 0; i < ps.SimCount; i++ {
			latentSample := pf.Sample()

			simState := domain.State{
				Observable: currentObservable.Copy(),
				Latent:     latentSample,
			}

			select {
			case <-ctx.Done():
				return
			case jobChan <- RolloutJob{Index: i, State: simState, HorizonSteps: ps.MaxHorizon}:
			}
		}
	}()

	// 3. Close output channel once workers finish
	go func() {
		wg.Wait()
		close(outputChan)
	}()

	// 4. Aggregate results with timeout protection (Invariant 5.2)
	bestDecision, err := ps.aggregateSearchTree(ctx, outputChan)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return ps.safeMyopicBackupPolicy(currentObservable)
		}
		return nil, err
	}

	return bestDecision, nil
}

func (ps *POMCPSolver) executeRolloutWithRNG(ctx context.Context, job RolloutJob, rng *rand.Rand) (float64, []string, error) {
	// Executes randomized rollouts using TransitionModel (S^M) and StandardObjective
	return 1250.50, []string{"A1-O1", "A2-O2"}, nil
}

func (ps *POMCPSolver) aggregateSearchTree(ctx context.Context, results chan RolloutOutput) (domain.Decision, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case res, ok := <-results:
			if !ok {
				return domain.Decision{}, nil
			}
			if res.Err != nil {
				return nil, res.Err
			}
		}
	}
}

func (ps *POMCPSolver) safeMyopicBackupPolicy(obs domain.ObservableState) (domain.Decision, error) {
	// Deterministic myopic fallback dispatch rule (Invariant 5.2)
	return domain.Decision{}, nil
}
```

---

## 5. Thread-Safe Value Function Approximation Cache

To prevent read lock contention during concurrent lookahead traversals, a localized `sync.RWMutex` wraps a multi-dimensional state lookup index. This protects against data races (Invariant 5.1) while allowing millions of reading simulations to proceed with zero allocation overhead.

```go
package solver

import (
	"sync"
)

// VFAPostDecisionCache represents a thread-safe lookup table for V(S_t^x).
type VFAPostDecisionCache struct {
	mu    sync.RWMutex
	table map[string]float64 // Keys are spatial-temporal coordinate indices
}

func NewVFACache() *VFAPostDecisionCache {
	return &VFAPostDecisionCache{
		table: make(map[string]float64),
	}
}

// Get performs zero-allocation concurrent reads using the read lock.
func (c *VFAPostDecisionCache) Get(key string) (float64, bool) {
	c.mu.RLock()
	val, ok := c.table[key]
	c.mu.RUnlock()
	return val, ok
}

// Update performs high-efficiency write locking during offline update epochs.
func (c *VFAPostDecisionCache) Update(key string, val float64) {
	c.mu.Lock()
	c.table[key] = val
	c.mu.Unlock()
}
```

---

## 6. Phased Development & Verification Backlog for Antigravity

Antigravity must execute the development of Project Mittens in six structured phases. Every phase requires writing specific Go unit tests (`_test.go`) that must pass before proceeding.

### Phase 1: Physical Domain & HOS Rules (Invariants 1.1–1.3)
- [ ] Implement `state.go` with flat slices of Driver and Load structs.
- [ ] Implement Hours of Service checking in `decision.go`.
- [ ] **Verification**: Write `TestFeasibleRegionPruning` where a driver with 10.5 hours of cumulative driving is assigned to a 2-hour transit load. The test must verify that the assignment is successfully pruned from the decision space.

### Phase 2: System Transitions (Powell's 5 Core Elements)
- [ ] Implement transition physics in `transition.go`.
- [ ] Implement cost calculations in `objective.go`.
- [ ] **Verification**: Write `TestSystemTransitionDeterministic` to verify that executing action $x_t$ on state $S_t$ moves the driver location to the exact destination coordinate and deducts operating costs correctly from the objective sum.

### Phase 3: Particle Filter & Jittering Updates (Invariants 4.1 & 4.2)
- [ ] Implement Bayesian likelihood weighting and Systematic Resampling in `belief.go`.
- [ ] Implement Particle Jittering when $N_{\text{eff}} < \kappa$.
- [ ] **Verification**: Write `TestEffectiveSampleSizeDepletion`. Seed a particle filter with 100 particles. Feed 10 consecutive outlier `TenderRejected` observations. Verify that $N_{\text{eff}}$ drops below `Kappa`, and that the Jittering routine fires, shifting the variance of the competitor density parameters and preventing particle collapse.

### Phase 4: Thread-Safe Value Function Cache (Invariant 5.1)
- [ ] Implement the read-mostly lookup table in `cache.go`.
- [ ] **Verification**: Write `TestVFACacheRaceConditions` spawning 100 concurrent reading goroutines and 5 concurrent writing goroutines. Run using the race detector:
  ```bash
  go test -race -v -run=TestVFACacheRaceConditions ./...
  ```
  The test must pass with zero runtime panics or race alerts.

### Phase 5: Concurrent POMCP Solver & Context Deadlines (Invariants 5.2 & 5.3)
- [ ] Implement tree search, bounded worker pools, and channel result collection in `pomcp.go`.
- [ ] Implement context cancellation triggers for worker routines.
- [ ] **Verification**: Write `TestRealTimePlanningTimeout`. Configure the solver to execute 1,000,000 simulations but pass a context with a hard timeout of `15 * time.Millisecond`. Verify that the execution returns within 16ms, terminates all worker goroutines without leaking, and returns the fallback safe myopic decision.

### Phase 6: Degeneracy Boundary Verification (Classic Powell Model Integration)
- [ ] Implement uncoupled transition state parameters.
- [ ] **Verification**: Write `TestClassicSDA_Degeneracy_Condition`. Execute the solver with Competitor Count set to zero ($N = 0$). Verify that:
  1. The latent state maps remain completely static.
  2. The particle filter belief collapses to a single static probability point mass.
  3. The POMCP solver trajectories and value tables match the exact deterministic myopic policy returns.
  This confirms that our generalized model degenerates mathematically to Dr. Powell's classic exogenous dynamic model.
