package math_test

import (
	"sync"
	"testing"

	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

func TestRNG_DeterministicReproducibility(t *testing.T) {
	const seed = 0xdeadbeefcafebabe
	rng1 := pkgmath.NewRNG(seed)
	rng2 := pkgmath.NewRNG(seed)

	for i := 0; i < 10000; i++ {
		v1 := rng1.NextUint64()
		v2 := rng2.NextUint64()
		if v1 != v2 {
			t.Fatalf("step %d: expected identical values %d == %d", i, v1, v2)
		}
	}
}

func TestRNG_DistinctSeeds(t *testing.T) {
	rng1 := pkgmath.NewRNG(42)
	rng2 := pkgmath.NewRNG(43)

	matchCount := 0
	for i := 0; i < 1000; i++ {
		if rng1.NextUint64() == rng2.NextUint64() {
			matchCount++
		}
	}
	if matchCount > 5 {
		t.Fatalf("distinct seeds produced unexpectedly high number of collisions: %d", matchCount)
	}
}

func TestRNG_Float64_RangeAndMean(t *testing.T) {
	rng := pkgmath.NewRNG(123456789)
	const n = 100000
	sum := 0.0

	for i := 0; i < n; i++ {
		f := rng.Float64()
		if f < 0.0 || f >= 1.0 {
			t.Fatalf("step %d: Float64() out of bounds: %f", i, f)
		}
		sum += f
	}

	mean := sum / float64(n)
	if mean < 0.49 || mean > 0.51 {
		t.Fatalf("Float64() empirical mean %f deviates significantly from expected 0.5", mean)
	}
}

func TestRNG_Intn_Distribution(t *testing.T) {
	rng := pkgmath.NewRNG(987654321)
	const (
		n       = 100000
		buckets = 10
	)
	counts := make([]int, buckets)

	for i := 0; i < n; i++ {
		val := rng.Intn(buckets)
		if val < 0 || val >= buckets {
			t.Fatalf("Intn(%d) produced out of range value %d", buckets, val)
		}
		counts[val]++
	}

	expectedPerBucket := float64(n) / float64(buckets)
	for i, c := range counts {
		ratio := float64(c) / expectedPerBucket
		if ratio < 0.95 || ratio > 1.05 {
			t.Errorf("bucket %d count %d (ratio %f) outside expected uniform tolerance", i, c, ratio)
		}
	}
}

func TestRNG_BernoulliAndRademacher(t *testing.T) {
	rng := pkgmath.NewRNG(555555)
	const n = 100000

	// Test Bernoulli p=0.3
	trueCount := 0
	for i := 0; i < n; i++ {
		if rng.Bernoulli(0.3) {
			trueCount++
		}
	}
	pEst := float64(trueCount) / float64(n)
	if pEst < 0.29 || pEst > 0.31 {
		t.Errorf("Bernoulli(0.3) empirical probability %f outside expected tolerance", pEst)
	}

	// Test Rademacher (+1 / -1)
	posCount := 0
	for i := 0; i < n; i++ {
		r := rng.Rademacher()
		if r != 1.0 && r != -1.0 {
			t.Fatalf("Rademacher returned invalid value %f", r)
		}
		if r == 1.0 {
			posCount++
		}
	}
	rEst := float64(posCount) / float64(n)
	if rEst < 0.49 || rEst > 0.51 {
		t.Errorf("Rademacher ratio %f outside expected 0.5", rEst)
	}
}

func TestRNG_Perm(t *testing.T) {
	rng := pkgmath.NewRNG(777)
	const size = 20
	perm := rng.Perm(size)

	if len(perm) != size {
		t.Fatalf("expected len %d, got %d", size, len(perm))
	}
	seen := make(map[int]bool)
	for _, v := range perm {
		if v < 0 || v >= size {
			t.Fatalf("Perm element %d out of range [0, %d)", v, size)
		}
		if seen[v] {
			t.Fatalf("Perm contains duplicate element %d", v)
		}
		seen[v] = true
	}
}

func TestRNG_Split_HierarchicalDeterminism(t *testing.T) {
	parentRNG1 := pkgmath.NewRNG(1337)
	parentRNG2 := pkgmath.NewRNG(1337)

	// Derive children at same coordinates
	child1 := parentRNG1.Split(10, 3, 1)
	child2 := parentRNG2.Split(10, 3, 1)

	for i := 0; i < 1000; i++ {
		if child1.NextUint64() != child2.NextUint64() {
			t.Fatalf("step %d: Split at identical coordinates yielded non-identical sequence", i)
		}
	}

	// Derive child at different branch ID
	childDiffBranch := parentRNG1.Split(10, 4, 1)
	if child1.NextUint64() == childDiffBranch.NextUint64() {
		t.Fatalf("Split at different branchIDs produced identical starting integer")
	}
}

func TestRNG_ConcurrentParallelTreeSplitting(t *testing.T) {
	// Verify zero race conditions when deriving child streams concurrently in parallel goroutines
	rootRNG := pkgmath.NewRNG(20260819)
	const numGoroutines = 16
	const iterations = 500

	results := make([][]uint64, numGoroutines)
	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		gID := uint64(g)
		// Deterministically split stream for goroutine
		gRNG := rootRNG.Clone().Split(1, gID, gID)

		go func(slot int, r *pkgmath.RNG) {
			defer wg.Done()
			seq := make([]uint64, iterations)
			for i := 0; i < iterations; i++ {
				seq[i] = r.NextUint64()
			}
			results[slot] = seq
		}(g, gRNG)
	}

	wg.Wait()

	// Verify all goroutines produced non-empty, distinct output sequences
	for g := 0; g < numGoroutines; g++ {
		if len(results[g]) != iterations {
			t.Fatalf("goroutine %d failed to produce %d iterations", g, iterations)
		}
		if g > 0 && results[g][0] == results[g-1][0] {
			t.Fatalf("adjacent goroutines %d and %d produced identical first element %d", g, g-1, results[g][0])
		}
	}
}

func TestRNG_FailClosedPanics(t *testing.T) {
	rng := pkgmath.NewRNG(1)

	assertPanic := func(name string, f func()) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("%s: expected panic on invalid input, but did not panic", name)
			}
		}()
		f()
	}

	assertPanic("Intn(0)", func() { rng.Intn(0) })
	assertPanic("Intn(-5)", func() { rng.Intn(-5) })
	assertPanic("Bernoulli(-0.1)", func() { rng.Bernoulli(-0.1) })
	assertPanic("Bernoulli(1.1)", func() { rng.Bernoulli(1.1) })
	assertPanic("Perm(-1)", func() { rng.Perm(-1) })
}
