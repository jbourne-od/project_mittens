package rng

import (
	"math"
	"sync"
	"testing"
)

func TestDeterminism(t *testing.T) {
	rootSeed := uint64(123456789)
	factory1 := NewFactory(rootSeed)
	factory2 := NewFactory(rootSeed)

	s1 := factory1.ExogenousStream(1, 10, 2)
	s2 := factory2.ExogenousStream(1, 10, 2)

	for i := 0; i < 1000; i++ {
		v1 := s1.Float64()
		v2 := s2.Float64()
		if v1 != v2 {
			t.Fatalf("mismatch at step %d: %f != %f", i, v1, v2)
		}
	}
}

func TestCRN_ShockInvariance(t *testing.T) {
	// Key requirement: An exogenous market shock at episode e, epoch t, dimension d
	// must be identical regardless of how many internal random draws a policy makes
	// or how long the policy runs.
	rootSeed := uint64(987654321)
	factory := NewFactory(rootSeed)

	targetEpisode := 3
	targetEpoch := 5
	targetDimension := uint32(1) // e.g., TenderArrival rate

	// 1. Policy A: Short myopic trajectory (3 steps, draws 10 internal numbers per step)
	policyA_ShockStream := factory.ExogenousStream(targetEpisode, targetEpoch, targetDimension)
	// Simulate Policy A running and consuming internal planner random numbers
	plannerStreamA := factory.PlannerStream(0, 0, 1)
	for i := 0; i < 50; i++ {
		_ = plannerStreamA.Float64()
	}
	expectedShockValue := policyA_ShockStream.Float64()

	// 2. Policy B: Deep search trajectory (50 steps, draws 10,000 internal numbers)
	policyB_ShockStream := factory.ExogenousStream(targetEpisode, targetEpoch, targetDimension)
	plannerStreamB := factory.PlannerStream(0, 0, 1)
	for i := 0; i < 50000; i++ {
		_ = plannerStreamB.NormFloat64()
	}
	actualShockValue := policyB_ShockStream.Float64()

	if expectedShockValue != actualShockValue {
		t.Fatalf("CRN failure: Policy B received different shock (%f) than Policy A (%f) for same (e=%d, t=%d, d=%d)",
			actualShockValue, expectedShockValue, targetEpisode, targetEpoch, targetDimension)
	}
}

func TestStreamDecorrelation(t *testing.T) {
	// Verify that streams derived from adjacent keys (e.g. dimension 0 vs dimension 1)
	// produce distinct, uncorrelated outputs.
	factory := NewFactory(42)
	s1 := factory.ExogenousStream(1, 1, 0)
	s2 := factory.ExogenousStream(1, 1, 1)

	n := 10000
	var sum1, sum2, sumProd float64
	for i := 0; i < n; i++ {
		x := s1.Float64() - 0.5
		y := s2.Float64() - 0.5
		sum1 += x * x
		sum2 += y * y
		sumProd += x * y
	}

	correlation := sumProd / math.Sqrt(sum1*sum2)
	if math.Abs(correlation) > 0.05 {
		t.Errorf("unexpected high correlation between adjacent streams: %f", correlation)
	}
}

func TestSampleDiscrete(t *testing.T) {
	stream := NewStream(101, 202)
	weights := []float64{1.0, 3.0, 6.0} // Probabilities: 0.1, 0.3, 0.6
	counts := make([]int, 3)

	trials := 100000
	for i := 0; i < trials; i++ {
		idx := stream.SampleDiscrete(weights)
		if idx < 0 || idx >= 3 {
			t.Fatalf("invalid index %d", idx)
		}
		counts[idx]++
	}

	p0 := float64(counts[0]) / float64(trials)
	p1 := float64(counts[1]) / float64(trials)
	p2 := float64(counts[2]) / float64(trials)

	if math.Abs(p0-0.1) > 0.01 || math.Abs(p1-0.3) > 0.01 || math.Abs(p2-0.6) > 0.01 {
		t.Errorf("discrete sampling out of expected statistical bounds: [%f, %f, %f]", p0, p1, p2)
	}

	// Edge cases
	if stream.SampleDiscrete([]float64{}) != -1 {
		t.Errorf("expected -1 for empty weights")
	}
	if stream.SampleDiscrete([]float64{0.0, 0.0}) != -1 {
		t.Errorf("expected -1 for all zero weights")
	}
}

func TestPermutationAndShuffle(t *testing.T) {
	stream := NewStream(555, 777)
	p := stream.Permutation(10)
	if len(p) != 10 {
		t.Fatalf("expected length 10, got %d", len(p))
	}

	seen := make(map[int]bool)
	for _, v := range p {
		if v < 0 || v >= 10 || seen[v] {
			t.Fatalf("invalid permutation element %d (duplicate or out of range)", v)
		}
		seen[v] = true
	}
}

func TestStatelessCoordinateSampling(t *testing.T) {
	rootSeed := uint64(777888999)

	// Test uniform repeatability
	u1 := SampleUniform(rootSeed, 2, 4, 1)
	u2 := SampleUniform(rootSeed, 2, 4, 1)
	if u1 != u2 {
		t.Errorf("SampleUniform not deterministic: %f != %f", u1, u2)
	}

	// Test normal repeatability and mean
	n1 := SampleNormal(rootSeed, 5, 2, 0, 10.0, 2.0)
	n2 := SampleNormal(rootSeed, 5, 2, 0, 10.0, 2.0)
	if n1 != n2 {
		t.Errorf("SampleNormal not deterministic: %f != %f", n1, n2)
	}

	// Test Bernoulli
	bAlways := SampleBernoulli(rootSeed, 1, 1, 1, 1.0)
	if !bAlways {
		t.Errorf("expected true for p=1.0")
	}
	bNever := SampleBernoulli(rootSeed, 1, 1, 1, 0.0)
	if bNever {
		t.Errorf("expected false for p=0.0")
	}
}

func TestConcurrentStreamDerivation(t *testing.T) {
	factory := NewFactory(12345)
	var wg sync.WaitGroup

	numWorkers := 32
	numStreamsPerWorker := 500

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < numStreamsPerWorker; i++ {
				st := factory.PlannerStream(i, workerID, workerID)
				_ = st.Float64()
				_ = st.NormFloat64()
			}
		}(w)
	}

	wg.Wait()
}

func BenchmarkDeriveStream(b *testing.B) {
	factory := NewFactory(42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = factory.ExogenousStream(1, i, 0)
	}
}

func BenchmarkStreamFloat64(b *testing.B) {
	stream := NewStream(123, 456)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = stream.Float64()
	}
}

func BenchmarkSampleUniform(b *testing.B) {
	rootSeed := uint64(42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SampleUniform(rootSeed, 1, i, 0)
	}
}
