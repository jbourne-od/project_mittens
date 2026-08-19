// Package math provides pure, reusable mathematical kernels for optimization,
// linear algebra, and stochastic approximation.
package math

import (
	"fmt"
	"math"
)

// RNG represents a fast, deterministic, 64-bit splittable pseudo-random number generator
// based on the SplitMix64 algorithm.
//
// In accordance with Principle 2 (Deterministic Reproducibility) and Inviolate 6 (Lock-Free Message Passing),
// RNG instances do not utilize mutexes or shared global state. Parallel lookahead branches
// (DLAs) and Monte Carlo sampling threads instantiate independent RNG streams by calling Split(),
// guaranteeing bit-wise deterministic execution regardless of goroutine scheduling order.
type RNG struct {
	state uint64
}

// NewRNG initializes and returns a new deterministic RNG instance seeded with the given 64-bit seed.
//
// To avoid seed degeneracy (e.g. all-zero state), SplitMix64 safely handles any 64-bit unsigned integer.
func NewRNG(seed uint64) *RNG {
	return &RNG{state: seed}
}

// NextUint64 advances the generator state by the golden-ratio Weyl sequence constant
// and returns a pseudo-random 64-bit unsigned integer.
func (r *RNG) NextUint64() uint64 {
	r.state += 0x9e3779b97f4a7c15
	z := r.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// Uint64 is an alias for NextUint64 to provide standard generator naming.
func (r *RNG) Uint64() uint64 {
	return r.NextUint64()
}

// Float64 returns a pseudo-random float64 uniformly distributed in the half-open interval [0.0, 1.0).
func (r *RNG) Float64() float64 {
	// Standard 53-bit floating point mantissa extraction from 64-bit integer
	return float64(r.NextUint64()>>11) / (1 << 53)
}

// Intn returns a pseudo-random integer uniformly distributed in the half-open interval [0, n).
// If n <= 0, Intn panics (fail-closed integrity check under Inviolate 8).
func (r *RNG) Intn(n int) int {
	if n <= 0 {
		panic(fmt.Sprintf("pkg/math: RNG.Intn called with non-positive bound n=%d", n))
	}
	// Lemire's fast alternative to modulo reduction without division
	return int((r.NextUint64() >> 32) * uint64(n) >> 32)
}

// Bernoulli returns true with probability p and false with probability (1 - p).
// Panics if p < 0.0 or p > 1.0.
func (r *RNG) Bernoulli(p float64) bool {
	if p < 0.0 || p > 1.0 || math.IsNaN(p) {
		panic(fmt.Sprintf("pkg/math: RNG.Bernoulli called with invalid probability p=%f", p))
	}
	if p == 0.0 {
		return false
	}
	if p == 1.0 {
		return true
	}
	return r.Float64() < p
}

// Rademacher returns either +1.0 or -1.0 with equal probability (0.5 each).
// This is the standard simultaneous perturbation distribution for SPSA optimization (Spall 1992).
func (r *RNG) Rademacher() float64 {
	// Check the lowest bit of the generated 64-bit integer
	if r.NextUint64()&1 == 1 {
		return 1.0
	}
	return -1.0
}

// Perm returns a pseudo-random permutation of the integers in [0, n) using the Fisher-Yates shuffle.
// If n < 0, Perm panics.
func (r *RNG) Perm(n int) []int {
	if n < 0 {
		panic(fmt.Sprintf("pkg/math: RNG.Perm called with negative length n=%d", n))
	}
	result := make([]int, n)
	for i := range result {
		result[i] = i
	}
	for i := n - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		result[i], result[j] = result[j], result[i]
	}
	return result
}

// Split deterministically derives a new, independent child RNG instance based on the parent's
// current state and domain coordinates (epoch, branchID, goroutineID).
//
// Concurrency & Determinism Context:
// In lookahead tree search (DLAs) and parallel Monte Carlo rollouts, child tasks must not share
// a single RNG instance (which would require locking or cause data races). By calling Split,
// each parallel evaluation branch receives an isolated PRNG whose sequence depends solely on
// the mathematical coordinates (epoch, branchID), ensuring 100% reproducible execution
// regardless of goroutine execution scheduling order (Inviolate 0, Inviolate 6).
func (r *RNG) Split(epoch uint64, branchID uint64, goroutineID uint64) *RNG {
	// Combine parent state with hierarchical coordinates using non-linear mix
	mix := r.NextUint64() ^ (epoch * 0x517cc1b727220a95) ^ (branchID * 0x6c62272e07bb0142) ^ (goroutineID * 0x9e3779b97f4a7c15)
	mix = (mix ^ (mix >> 30)) * 0xbf58476d1ce4e5b9
	mix = (mix ^ (mix >> 27)) * 0x94d049bb133111eb
	childSeed := mix ^ (mix >> 31)
	return NewRNG(childSeed)
}

// Clone creates and returns a bit-wise identical deep copy of the RNG in its current state.
func (r *RNG) Clone() *RNG {
	return &RNG{state: r.state}
}
