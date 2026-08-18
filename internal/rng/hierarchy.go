package rng

const (
	// Domain separation tags to ensure distinct coordinate namespaces never collide
	tagExogenous = 0x45584f47454e5553 // "EXOGENUS"
	tagPlanner   = 0x504c414e4e455253 // "PLANNERS"
	tagBelief    = 0x42454c4945465354 // "BELIEFST"
	tagRollout   = 0x524f4c4c4f555453 // "ROLLOUTS"
)

// Factory provides hierarchical, coordinate-based derivation of independent PRNG streams.
// By deriving streams deterministically from structured coordinate keys:
//
//	Stream(episode, epoch, dimension)
//
// different runs and policies can evaluate the exact same exogenous scenarios (Common Random Numbers)
// regardless of differences in policy execution length or internal search tree branching.
type Factory struct {
	rootSeed uint64
}

// NewFactory creates a new stream Factory initialized with a 64-bit root seed.
func NewFactory(rootSeed uint64) *Factory {
	return &Factory{rootSeed: rootSeed}
}

// RootSeed returns the 64-bit root seed of the factory.
func (f *Factory) RootSeed() uint64 {
	return f.rootSeed
}

// Derive generates an independent, reproducible Stream derived from the factory's root seed
// and an arbitrary sequence of 64-bit coordinate keys.
func (f *Factory) Derive(keys ...uint64) *Stream {
	seed1, seed2 := HashCoordinates(f.rootSeed, keys...)
	return NewStream(seed1, seed2)
}

// ExogenousStream returns a dedicated stream for generating exogenous market events W_t.
//
// Coordinates:
//   - episode: Monte Carlo episode / replication index (>= 0)
//   - epoch: Decision epoch / time step (>= 0)
//   - dimension: Shock dimension (e.g., 0=Arrival, 1=Cancellation, 2=PriceUpdate)
//
// Guarantees: The sequence of random numbers generated from this stream is 100% invariant
// to the number of simulations or branching decisions made by the planner.
func (f *Factory) ExogenousStream(episode int, epoch int, dimension uint32) *Stream {
	return f.Derive(
		tagExogenous,
		uint64(episode),
		uint64(epoch),
		uint64(dimension),
	)
}

// PlannerStream returns a dedicated stream for a specific planner worker during an online planning epoch.
//
// Coordinates:
//   - epoch: Decision epoch (>= 0)
//   - simIndex: Simulation batch or root simulation index
//   - workerID: Worker / thread identifier (ensures zero cross-goroutine lock contention)
func (f *Factory) PlannerStream(epoch int, simIndex int, workerID int) *Stream {
	return f.Derive(
		tagPlanner,
		uint64(epoch),
		uint64(simIndex),
		uint64(workerID),
	)
}

// BeliefStream returns a dedicated stream for initializing or updating a specific belief particle.
//
// Coordinates:
//   - episode: Monte Carlo episode index
//   - particleID: Index of the particle in the belief filter
func (f *Factory) BeliefStream(episode int, particleID int) *Stream {
	return f.Derive(
		tagBelief,
		uint64(episode),
		uint64(particleID),
	)
}

// RolloutStream returns a dedicated stream for an individual rollout step.
func (f *Factory) RolloutStream(epoch int, simIndex int, step int) *Stream {
	return f.Derive(
		tagRollout,
		uint64(epoch),
		uint64(simIndex),
		uint64(step),
	)
}
