package rng

// Mix64 computes a 64-bit SplitMix64 mixing transformation on x.
// SplitMix64 provides excellent avalanche properties and collision resistance,
// making it ideal for hashing multi-dimensional coordinates into uncorrelated PRNG seeds.
func Mix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// HashCoordinates mixes a 64-bit root seed with an arbitrary sequence of coordinate keys.
// It returns a pair of high-entropy 64-bit seed values (seed1, seed2) suitable for initializing
// a 128-bit PCG pseudo-random generator state.
func HashCoordinates(rootSeed uint64, keys ...uint64) (uint64, uint64) {
	state := rootSeed
	for _, k := range keys {
		state = Mix64(state ^ Mix64(k))
	}
	seed1 := Mix64(state)
	seed2 := Mix64(state ^ 0x9e3779b97f4a7c15)
	return seed1, seed2
}
