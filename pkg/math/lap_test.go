package math_test

import (
	"math"
	"testing"

	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

func TestLAP_KnownSquareMatrix(t *testing.T) {
	// Standard 3x3 minimization problem:
	// Cost = [10 19  8]
	//        [10  1 12]
	//        [13 16  9]
	//
	// Optimal matching:
	// Row 0 -> Col 0 (Cost 10)
	// Row 1 -> Col 1 (Cost 1)
	// Row 2 -> Col 2 (Cost 9)
	// Total Cost = 10 + 1 + 9 = 20
	costMatrix := [][]float64{
		{10, 19, 8},
		{10, 1, 12},
		{13, 16, 9},
	}

	res, err := pkgmath.SolveLAP(costMatrix, false, true)
	if err != nil {
		t.Fatalf("SolveLAP failed: %v", err)
	}

	if res.TotalWeight != 20.0 {
		t.Errorf("expected total cost 20.0, got %f", res.TotalWeight)
	}
	if res.RowToCol[0] != 0 || res.RowToCol[1] != 1 || res.RowToCol[2] != 2 {
		t.Errorf("unexpected rowToCol assignment: %v", res.RowToCol)
	}
}

func TestLAP_MaximizationRectangular(t *testing.T) {
	// 2 Drivers vs 4 Loads (Payout Matrix):
	// Payout = [100  50 200  80]
	//          [150 120 180  90]
	//
	// Optimal pairing for max payout:
	// If Driver 0 -> Load 2 (200), Driver 1 -> Load 0 (150) => Total = 350
	// (Greedy would pick Driver 0 -> Load 2 (200), Driver 1 -> Load 1 (120) => Total = 320!)
	payoutMatrix := [][]float64{
		{100, 50, 200, 80},
		{150, 120, 180, 90},
	}

	res, err := pkgmath.SolveLAP(payoutMatrix, true, true)
	if err != nil {
		t.Fatalf("SolveLAP failed: %v", err)
	}

	if res.TotalWeight != 350.0 {
		t.Errorf("expected total weight 350.0, got %f", res.TotalWeight)
	}
	if res.RowToCol[0] != 2 || res.RowToCol[1] != 0 {
		t.Errorf("unexpected assignment: %v", res.RowToCol)
	}
}

func TestLAP_BruteForceParity(t *testing.T) {
	rng := pkgmath.NewRNG(42)

	// Test on 50 random 6x6 matrices against exact brute force enumeration
	n := 6
	for testIdx := 0; testIdx < 50; testIdx++ {
		mat := make([][]float64, n)
		for i := 0; i < n; i++ {
			mat[i] = make([]float64, n)
			for j := 0; j < n; j++ {
				mat[i][j] = float64(rng.Intn(1000))
			}
		}

		res, err := pkgmath.SolveLAP(mat, true, true)
		if err != nil {
			t.Fatalf("Test %d: SolveLAP failed: %v", testIdx, err)
		}

		// Brute force all n! permutations
		bestWeight := math.Inf(-1)
		var permute func([]int, int)
		p := make([]int, n)
		for i := 0; i < n; i++ {
			p[i] = i
		}

		permute = func(arr []int, k int) {
			if k == len(arr) {
				w := 0.0
				for r, c := range arr {
					w += mat[r][c]
				}
				if w > bestWeight {
					bestWeight = w
				}
				return
			}
			for i := k; i < len(arr); i++ {
				arr[k], arr[i] = arr[i], arr[k]
				permute(arr, k+1)
				arr[k], arr[i] = arr[i], arr[k]
			}
		}
		permute(p, 0)

		if math.Abs(res.TotalWeight-bestWeight) > 1e-9 {
			t.Fatalf("Test %d: LAP total weight %.2f != brute force optimum %.2f",
				testIdx, res.TotalWeight, bestWeight)
		}
	}
}

func TestLAP_NegativeThresholdPruning(t *testing.T) {
	// 3 Drivers vs 3 Loads:
	// Driver 0 has profitable loads [100, 50, -10]
	// Driver 1 has profitable loads [-20, 80, -5]
	// Driver 2 has ONLY negative loads [-50, -100, -30]
	payoutMatrix := [][]float64{
		{100, 50, -10},
		{-20, 80, -5},
		{-50, -100, -30},
	}

	res, err := pkgmath.SolveLAP(payoutMatrix, true, false)
	if err != nil {
		t.Fatalf("SolveLAP failed: %v", err)
	}

	// Driver 0 -> Load 0 (100)
	// Driver 1 -> Load 1 (80)
	// Driver 2 -> Unassigned (-1)
	if res.MatchCount != 2 {
		t.Errorf("expected 2 matches, got %d", res.MatchCount)
	}
	if res.RowToCol[0] != 0 || res.RowToCol[1] != 1 || res.RowToCol[2] != -1 {
		t.Errorf("unexpected assignment: %v", res.RowToCol)
	}
	if res.TotalWeight != 180.0 {
		t.Errorf("expected total weight 180.0, got %f", res.TotalWeight)
	}
}

func TestLAP_FailClosedValidations(t *testing.T) {
	// Empty matrix
	if _, err := pkgmath.SolveLAP(nil, true, true); err == nil {
		t.Errorf("expected error for nil matrix")
	}
	if _, err := pkgmath.SolveLAP([][]float64{}, true, true); err == nil {
		t.Errorf("expected error for empty matrix")
	}

	// Matrix with NaN
	nanMatrix := [][]float64{
		{10.0, math.NaN()},
		{5.0, 20.0},
	}
	if _, err := pkgmath.SolveLAP(nanMatrix, true, true); err == nil {
		t.Errorf("expected error for NaN in matrix")
	}

	// Non-rectangular jagged matrix
	jaggedMatrix := [][]float64{
		{10.0, 20.0, 30.0},
		{5.0, 20.0},
	}
	if _, err := pkgmath.SolveLAP(jaggedMatrix, true, true); err == nil {
		t.Errorf("expected error for jagged matrix")
	}
}
