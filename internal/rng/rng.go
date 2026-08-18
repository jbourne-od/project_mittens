package rng

import (
	"math/rand/v2"
)

// Stream represents an isolated, deterministic pseudo-random stream.
// It wraps a 128-bit PCG generator from math/rand/v2.
//
// Streams are NOT safe for concurrent use across multiple goroutines without external locking.
// To use RNG in parallel code, derive independent streams for each worker using Factory.Derive.
type Stream struct {
	prng *rand.Rand
}

// NewStream creates an independent PRNG Stream initialized with a 128-bit PCG state (seed1, seed2).
func NewStream(seed1, seed2 uint64) *Stream {
	src := rand.NewPCG(seed1, seed2)
	return &Stream{
		prng: rand.New(src),
	}
}

// Float64 returns a pseudo-random float64 in [0.0, 1.0) with 53 bits of precision.
func (s *Stream) Float64() float64 {
	return s.prng.Float64()
}

// NormFloat64 returns a standard normal random variable ~ N(0, 1).
func (s *Stream) NormFloat64() float64 {
	return s.prng.NormFloat64()
}

// ExpFloat64 returns an exponential random variable with rate parameter lambda = 1.0 (mean = 1.0).
func (s *Stream) ExpFloat64() float64 {
	return s.prng.ExpFloat64()
}

// Uint64 returns a pseudo-random 64-bit unsigned integer.
func (s *Stream) Uint64() uint64 {
	return s.prng.Uint64()
}

// IntN returns a pseudo-random int in [0, n). Panics if n <= 0.
func (s *Stream) IntN(n int) int {
	return s.prng.IntN(n)
}

// Shuffle pseudo-randomizes the order of elements using Fisher-Yates shuffle.
func (s *Stream) Shuffle(n int, swap func(i, j int)) {
	s.prng.Shuffle(n, swap)
}

// Permutation returns a pseudo-random permutation of the integers in [0, n).
func (s *Stream) Permutation(n int) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	s.Shuffle(n, func(i, j int) {
		p[i], p[j] = p[j], p[i]
	})
	return p
}

// SampleDiscrete samples an index in [0, len(weights)) according to categorical unnormalized weights.
// Returns -1 if weights is empty or all weights are zero.
func (s *Stream) SampleDiscrete(weights []float64) int {
	if len(weights) == 0 {
		return -1
	}
	if len(weights) == 1 {
		if weights[0] > 0 {
			return 0
		}
		return -1
	}

	var totalMass float64
	for _, w := range weights {
		if w > 0 {
			totalMass += w
		}
	}
	if totalMass <= 0 {
		return -1
	}

	u := s.Float64() * totalMass
	var cum float64
	for i, w := range weights {
		if w > 0 {
			cum += w
			if u <= cum {
				return i
			}
		}
	}
	return len(weights) - 1
}

// Fork creates a new child Stream derived from the current stream's state.
func (s *Stream) Fork() *Stream {
	s1 := s.Uint64()
	s2 := s.Uint64()
	return NewStream(s1, s2)
}
