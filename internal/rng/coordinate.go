package rng

import (
	"math"
)

// SampleUniform generates a single uniform float64 in [0.0, 1.0) directly at coordinate (episode, epoch, dimension).
// This is a stateless operation that does not maintain or mutate any PRNG state.
func SampleUniform(rootSeed uint64, episode int, epoch int, dimension uint32) float64 {
	s1, s2 := HashCoordinates(
		rootSeed,
		tagExogenous,
		uint64(episode),
		uint64(epoch),
		uint64(dimension),
	)
	stream := NewStream(s1, s2)
	return stream.Float64()
}

// SampleNormal generates a single Gaussian random variable ~ N(mean, stdDev^2) at the given coordinates.
func SampleNormal(rootSeed uint64, episode int, epoch int, dimension uint32, mean, stdDev float64) float64 {
	s1, s2 := HashCoordinates(
		rootSeed,
		tagExogenous,
		uint64(episode),
		uint64(epoch),
		uint64(dimension),
	)
	stream := NewStream(s1, s2)
	return mean + stdDev*stream.NormFloat64()
}

// SampleExponential generates an exponential random variable with parameter lambda (mean = 1/lambda).
func SampleExponential(rootSeed uint64, episode int, epoch int, dimension uint32, lambda float64) float64 {
	if lambda <= 0 {
		return math.NaN()
	}
	u := SampleUniform(rootSeed, episode, epoch, dimension)
	// -log(1 - u) / lambda is distributed as Exp(lambda)
	return -math.Log1p(-u) / lambda
}

// SampleBernoulli returns true with probability p and false with probability (1 - p).
func SampleBernoulli(rootSeed uint64, episode int, epoch int, dimension uint32, p float64) bool {
	if p <= 0 {
		return false
	}
	if p >= 1.0 {
		return true
	}
	return SampleUniform(rootSeed, episode, epoch, dimension) < p
}
