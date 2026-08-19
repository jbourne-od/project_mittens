package benchmarks_test

import (
	"fmt"
	"math/rand"
	"testing"

	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

// BenchmarkLAP_ExactSolver evaluates SolveLinearAssignment on dense cost matrices
// across standard operational carrier fleet dimensions.
func BenchmarkLAP_ExactSolver(b *testing.B) {
	sizes := []int{10, 50, 100, 250, 500}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("Matrix_%dx%d", n, n), func(b *testing.B) {
			rng := rand.New(rand.NewSource(42))
			costMatrix := make([][]float64, n)
			for i := 0; i < n; i++ {
				costMatrix[i] = make([]float64, n)
				for j := 0; j < n; j++ {
					// Random contribution in [50, 2500]
					costMatrix[i][j] = 50.0 + rng.Float64()*2450.0
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				result, err := pkgmath.SolveLAP(costMatrix, true, true)
				if err != nil {
					b.Fatalf("SolveLAP failed: %v", err)
				}
				if len(result.RowToCol) != n {
					b.Fatalf("unexpected matching size: %d", len(result.RowToCol))
				}
			}
		})
	}
}

// BenchmarkLAP_Rectangular evaluates rectangular maximization matching (e.g. 50 drivers x 500 candidate loads).
func BenchmarkLAP_Rectangular(b *testing.B) {
	drivers := 50
	loads := 500

	rng := rand.New(rand.NewSource(101))
	costMatrix := make([][]float64, drivers)
	for i := 0; i < drivers; i++ {
		costMatrix[i] = make([]float64, loads)
		for j := 0; j < loads; j++ {
			if rng.Float64() < 0.85 { // 85% feasible candidates
				costMatrix[i][j] = 100.0 + rng.Float64()*1800.0
			} else {
				costMatrix[i][j] = -1e9 // Infeasible
			}
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result, err := pkgmath.SolveLAP(costMatrix, true, false)
		if err != nil {
			b.Fatalf("SolveLAP failed: %v", err)
		}
		if len(result.RowToCol) != drivers {
			b.Fatalf("unexpected row assignment count: %d", len(result.RowToCol))
		}
	}
}
