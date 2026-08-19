package math_test

import (
	"math"
	"testing"

	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

func TestDenseMatrix_BasicOps(t *testing.T) {
	m := pkgmath.NewDenseMatrix(2, 3)
	m.Set(0, 0, 1.0)
	m.Set(0, 1, 2.0)
	m.Set(0, 2, 3.0)
	m.Set(1, 0, 4.0)
	m.Set(1, 1, 5.0)
	m.Set(1, 2, 6.0)

	if m.Rows() != 2 || m.Cols() != 3 {
		t.Fatalf("expected dimensions (2, 3), got (%d, %d)", m.Rows(), m.Cols())
	}
	if m.At(1, 2) != 6.0 {
		t.Fatalf("m.At(1, 2) = %v; expected 6.0", m.At(1, 2))
	}

	// Clone test
	clone := m.Clone()
	clone.Set(0, 0, 99.0)
	if m.At(0, 0) == 99.0 {
		t.Fatalf("modifying cloned matrix mutated parent matrix (Inviolate 5 violation)")
	}
}

func TestDenseMatrix_MulVec(t *testing.T) {
	// A = [[1, 2], [3, 4]], x = [5, 6] -> A*x = [17, 39]
	A, err := pkgmath.NewDenseMatrixWithData(2, 2, []float64{1, 2, 3, 4})
	if err != nil {
		t.Fatalf("failed to create matrix: %v", err)
	}

	x := []float64{5, 6}
	y, err := A.MulVec(x)
	if err != nil {
		t.Fatalf("MulVec failed: %v", err)
	}

	if len(y) != 2 || y[0] != 17 || y[1] != 39 {
		t.Fatalf("MulVec result %v; expected [17, 39]", y)
	}

	// Dimension mismatch
	_, err = A.MulVec([]float64{1, 2, 3})
	if err == nil {
		t.Fatalf("expected error on dimension mismatch")
	}
}

func TestDenseMatrix_Mul(t *testing.T) {
	// A = [[1, 2], [3, 4]], B = [[5, 6], [7, 8]] -> C = [[19, 22], [43, 50]]
	A, _ := pkgmath.NewDenseMatrixWithData(2, 2, []float64{1, 2, 3, 4})
	B, _ := pkgmath.NewDenseMatrixWithData(2, 2, []float64{5, 6, 7, 8})

	C, err := A.Mul(B)
	if err != nil {
		t.Fatalf("Mul failed: %v", err)
	}

	expected := []float64{19, 22, 43, 50}
	for i, exp := range expected {
		if C.Data()[i] != exp {
			t.Fatalf("C.Data()[%d] = %v; expected %v", i, C.Data()[i], exp)
		}
	}
}

func TestDotAndNorm2(t *testing.T) {
	a := []float64{1, 2, 3}
	b := []float64{4, 5, 6}

	dot, err := pkgmath.Dot(a, b)
	if err != nil || dot != 32 {
		t.Fatalf("Dot(a, b) = %v, err=%v; expected 32", dot, err)
	}

	norm := pkgmath.Norm2([]float64{3, 4})
	if norm != 5.0 {
		t.Fatalf("Norm2([3, 4]) = %v; expected 5.0", norm)
	}
}

func TestCholesky_Decomposition(t *testing.T) {
	// Known positive-definite matrix A = [[4, 12, -16], [12, 37, -43], [-16, -43, 98]]
	// Lower factor L = [[2, 0, 0], [6, 1, 0], [-8, 5, 3]]
	A, err := pkgmath.NewDenseMatrixWithData(3, 3, []float64{
		4, 12, -16,
		12, 37, -43,
		-16, -43, 98,
	})
	if err != nil {
		t.Fatalf("matrix creation failed: %v", err)
	}

	L, err := pkgmath.Cholesky(A)
	if err != nil {
		t.Fatalf("Cholesky failed on positive definite matrix: %v", err)
	}

	expectedL := []float64{
		2, 0, 0,
		6, 1, 0,
		-8, 5, 3,
	}

	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			exp := expectedL[r*3+c]
			got := L.At(r, c)
			if math.Abs(got-exp) > 1e-12 {
				t.Fatalf("L(%d, %d) = %v; expected %v", r, c, got, exp)
			}
		}
	}

	// Verify L * L^T = A
	// Build L^T
	LT := pkgmath.NewDenseMatrix(3, 3)
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			LT.Set(r, c, L.At(c, r))
		}
	}
	recon, err := L.Mul(LT)
	if err != nil {
		t.Fatalf("L * L^T failed: %v", err)
	}

	for i := 0; i < 9; i++ {
		if math.Abs(recon.Data()[i]-A.Data()[i]) > 1e-12 {
			t.Fatalf("reconstructed A[%d] = %v; expected %v", i, recon.Data()[i], A.Data()[i])
		}
	}
}

func TestCholesky_NonPositiveDefinite(t *testing.T) {
	// Matrix with non-positive determinant/pivot
	A, _ := pkgmath.NewDenseMatrixWithData(2, 2, []float64{
		1, 2,
		2, 1, // determinant = 1 - 4 = -3
	})

	_, err := pkgmath.Cholesky(A)
	if err == nil {
		t.Fatalf("expected error on non-positive definite matrix, got nil")
	}
}

func TestSolveLower(t *testing.T) {
	// L = [[2, 0], [3, 4]], b = [4, 14]
	// y[0] = 4/2 = 2
	// y[1] = (14 - 3*2) / 4 = 8/4 = 2
	L, _ := pkgmath.NewDenseMatrixWithData(2, 2, []float64{
		2, 0,
		3, 4,
	})
	b := []float64{4, 14}

	y, err := pkgmath.SolveLower(L, b)
	if err != nil {
		t.Fatalf("SolveLower failed: %v", err)
	}

	if math.Abs(y[0]-2.0) > 1e-12 || math.Abs(y[1]-2.0) > 1e-12 {
		t.Fatalf("SolveLower result %v; expected [2, 2]", y)
	}
}
