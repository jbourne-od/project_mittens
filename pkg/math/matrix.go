package math

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrDimensionMismatch is returned when matrix or vector dimensions are incompatible.
	ErrDimensionMismatch = errors.New("pkg/math: matrix/vector dimension mismatch")
	// ErrNotSquare is returned when a square matrix is required but non-square dimensions were provided.
	ErrNotSquare = errors.New("pkg/math: matrix must be square")
	// ErrNotPositiveDefinite is returned when Cholesky decomposition fails on a non-positive-definite matrix.
	ErrNotPositiveDefinite = errors.New("pkg/math: matrix is not positive-definite")
	// ErrSingularMatrix is returned when a triangular matrix has a zero on its diagonal during solve.
	ErrSingularMatrix = errors.New("pkg/math: triangular matrix is singular (zero diagonal)")
)

// DenseMatrix represents a 2D matrix stored as a contiguous 1D flat slice in row-major layout.
//
// In accordance with high-performance Go memory layout principles, using a contiguous 1D slice
// ensures CPU cache locality, minimizes GC pointer tracking overhead, and eliminates pointer-of-pointers indirection.
type DenseMatrix struct {
	rows int
	cols int
	data []float64
}

// NewDenseMatrix allocates and returns a new zero-initialized DenseMatrix of dimension rows x cols.
// Panics if rows <= 0 or cols <= 0 (Inviolate 8 fail-closed).
func NewDenseMatrix(rows, cols int) *DenseMatrix {
	if rows <= 0 || cols <= 0 {
		panic(fmt.Sprintf("pkg/math: NewDenseMatrix called with invalid dimensions (%d, %d)", rows, cols))
	}
	return &DenseMatrix{
		rows: rows,
		cols: cols,
		data: make([]float64, rows*cols),
	}
}

// NewDenseMatrixWithData creates a new DenseMatrix wrapping a copy of the provided flat data slice.
// Returns an error if len(data) != rows * cols.
func NewDenseMatrixWithData(rows, cols int, data []float64) (*DenseMatrix, error) {
	if rows <= 0 || cols <= 0 {
		return nil, fmt.Errorf("%w: invalid dimensions (%d, %d)", ErrDimensionMismatch, rows, cols)
	}
	if len(data) != rows*cols {
		return nil, fmt.Errorf("%w: data length %d != rows*cols %d", ErrDimensionMismatch, len(data), rows*cols)
	}

	copied := make([]float64, len(data))
	copy(copied, data)

	return &DenseMatrix{
		rows: rows,
		cols: cols,
		data: copied,
	}, nil
}

// Eye returns an n x n identity matrix.
func Eye(n int) *DenseMatrix {
	m := NewDenseMatrix(n, n)
	for i := 0; i < n; i++ {
		m.data[i*n+i] = 1.0
	}
	return m
}

// Rows returns the number of rows in the matrix.
func (m *DenseMatrix) Rows() int {
	return m.rows
}

// Cols returns the number of columns in the matrix.
func (m *DenseMatrix) Cols() int {
	return m.cols
}

// At returns the element at row r and column c (0-indexed).
// Panics if r or c are out of bounds.
func (m *DenseMatrix) At(r, c int) float64 {
	if r < 0 || r >= m.rows || c < 0 || c >= m.cols {
		panic(fmt.Sprintf("pkg/math: matrix index out of bounds: (%d, %d) for matrix (%d, %d)", r, c, m.rows, m.cols))
	}
	return m.data[r*m.cols+c]
}

// Set sets the element at row r and column c (0-indexed) to val.
// Panics if r or c are out of bounds.
func (m *DenseMatrix) Set(r, c int, val float64) {
	if r < 0 || r >= m.rows || c < 0 || c >= m.cols {
		panic(fmt.Sprintf("pkg/math: matrix index out of bounds: (%d, %d) for matrix (%d, %d)", r, c, m.rows, m.cols))
	}
	m.data[r*m.cols+c] = val
}

// Clone returns a deep copy of the matrix with an independent backing slice (Inviolate 5).
func (m *DenseMatrix) Clone() *DenseMatrix {
	copied := make([]float64, len(m.data))
	copy(copied, m.data)
	return &DenseMatrix{
		rows: m.rows,
		cols: m.cols,
		data: copied,
	}
}

// Data returns a deep copy of the underlying flat 1D data slice.
func (m *DenseMatrix) Data() []float64 {
	copied := make([]float64, len(m.data))
	copy(copied, m.data)
	return copied
}

// MulVec computes the matrix-vector product y = A * x.
// Returns ErrDimensionMismatch if len(x) != m.Cols().
func (m *DenseMatrix) MulVec(x []float64) ([]float64, error) {
	if len(x) != m.cols {
		return nil, fmt.Errorf("%w: vector length %d != matrix cols %d", ErrDimensionMismatch, len(x), m.cols)
	}

	y := make([]float64, m.rows)
	for r := 0; r < m.rows; r++ {
		rowOffset := r * m.cols
		sum := 0.0
		for c := 0; c < m.cols; c++ {
			sum += m.data[rowOffset+c] * x[c]
		}
		y[r] = sum
	}
	return y, nil
}

// Mul computes the matrix-matrix product C = A * B.
// Returns ErrDimensionMismatch if m.Cols() != other.Rows().
func (m *DenseMatrix) Mul(other *DenseMatrix) (*DenseMatrix, error) {
	if m.cols != other.rows {
		return nil, fmt.Errorf("%w: A cols %d != B rows %d", ErrDimensionMismatch, m.cols, other.rows)
	}

	result := NewDenseMatrix(m.rows, other.cols)
	for r := 0; r < m.rows; r++ {
		for c := 0; c < other.cols; c++ {
			sum := 0.0
			for k := 0; k < m.cols; k++ {
				sum += m.data[r*m.cols+k] * other.data[k*other.cols+c]
			}
			result.data[r*other.cols+c] = sum
		}
	}
	return result, nil
}

// Dot computes the inner product between two vectors of equal length.
func Dot(a, b []float64) (float64, error) {
	if len(a) != len(b) {
		return 0.0, fmt.Errorf("%w: vector lengths %d != %d", ErrDimensionMismatch, len(a), len(b))
	}
	sum := 0.0
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum, nil
}

// Norm2 computes the Euclidean L2 norm of a vector: ||a||_2 = sqrt(\sum a_i^2).
func Norm2(a []float64) float64 {
	sumSq := 0.0
	for _, v := range a {
		sumSq += v * v
	}
	return math.Sqrt(sumSq)
}

// Cholesky computes the Cholesky factor L of a symmetric, positive-definite matrix A such that A = L * L^T.
//
// Returns a lower-triangular DenseMatrix L.
// If the matrix is non-square, non-symmetric, or non-positive-definite, Cholesky returns an error.
func Cholesky(a *DenseMatrix) (*DenseMatrix, error) {
	if a.rows != a.cols {
		return nil, fmt.Errorf("%w: (%d, %d)", ErrNotSquare, a.rows, a.cols)
	}
	n := a.rows
	L := NewDenseMatrix(n, n)

	for j := 0; j < n; j++ {
		sum := 0.0
		for k := 0; k < j; k++ {
			val := L.data[j*n+k]
			sum += val * val
		}

		diag := a.data[j*n+j] - sum
		if diag <= 1e-15 {
			return nil, fmt.Errorf("%w: non-positive diagonal pivot at index (%d, %d): %e", ErrNotPositiveDefinite, j, j, diag)
		}

		Ljj := math.Sqrt(diag)
		L.data[j*n+j] = Ljj

		for i := j + 1; i < n; i++ {
			sumK := 0.0
			for k := 0; k < j; k++ {
				sumK += L.data[i*n+k] * L.data[j*n+k]
			}
			L.data[i*n+j] = (a.data[i*n+j] - sumK) / Ljj
		}
	}

	return L, nil
}

// Transpose returns a newly allocated matrix that is the transpose of m (Inviolate 5).
func (m *DenseMatrix) Transpose() *DenseMatrix {
	trans := NewDenseMatrix(m.cols, m.rows)
	for r := 0; r < m.rows; r++ {
		for c := 0; c < m.cols; c++ {
			trans.data[c*m.rows+r] = m.data[r*m.cols+c]
		}
	}
	return trans
}

// SolveLower solves the lower-triangular linear system L * y = b via forward substitution.
//
// Assumes L is an n x n lower-triangular matrix with non-zero diagonal entries.
// Returns an error if dimensions mismatch or if a diagonal entry is zero.
func SolveLower(L *DenseMatrix, b []float64) ([]float64, error) {
	if L.rows != L.cols {
		return nil, ErrNotSquare
	}
	n := L.rows
	if len(b) != n {
		return nil, fmt.Errorf("%w: matrix size %d != vector size %d", ErrDimensionMismatch, n, len(b))
	}

	y := make([]float64, n)
	for i := 0; i < n; i++ {
		sum := 0.0
		for j := 0; j < i; j++ {
			sum += L.data[i*n+j] * y[j]
		}
		diag := L.data[i*n+i]
		if math.Abs(diag) < 1e-15 {
			return nil, fmt.Errorf("%w: diagonal at index %d is zero (%e)", ErrSingularMatrix, i, diag)
		}
		y[i] = (b[i] - sum) / diag
	}

	return y, nil
}

// SolveUpper solves the upper-triangular linear system U * x = y via backward substitution.
//
// Assumes U is an n x n upper-triangular matrix with non-zero diagonal entries.
// Returns an error if dimensions mismatch or if a diagonal entry is zero.
func SolveUpper(U *DenseMatrix, y []float64) ([]float64, error) {
	if U.rows != U.cols {
		return nil, ErrNotSquare
	}
	n := U.rows
	if len(y) != n {
		return nil, fmt.Errorf("%w: matrix size %d != vector size %d", ErrDimensionMismatch, n, len(y))
	}

	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		sum := 0.0
		for j := i + 1; j < n; j++ {
			sum += U.data[i*n+j] * x[j]
		}
		diag := U.data[i*n+i]
		if math.Abs(diag) < 1e-15 {
			return nil, fmt.Errorf("%w: diagonal at index %d is zero (%e)", ErrSingularMatrix, i, diag)
		}
		x[i] = (y[i] - sum) / diag
	}

	return x, nil
}

// SolveCholesky solves the symmetric positive-definite linear system A * x = (L * L^T) * x = b.
//
// Parameters:
//   - L: Lower-triangular Cholesky factor of matrix A (i.e. A = L * L^T).
//   - b: Right-hand side vector.
//
// Computes solution x in O(n^2) time via forward substitution L * y = b followed by
// backward substitution L^T * x = y without inverting A directly.
func SolveCholesky(L *DenseMatrix, b []float64) ([]float64, error) {
	if L.rows != L.cols {
		return nil, ErrNotSquare
	}

	// 1. Forward substitution: L * y = b
	y, err := SolveLower(L, b)
	if err != nil {
		return nil, fmt.Errorf("pkg/math: SolveCholesky forward solve failed: %w", err)
	}

	// 2. Backward substitution: L^T * x = y
	LT := L.Transpose()
	x, err := SolveUpper(LT, y)
	if err != nil {
		return nil, fmt.Errorf("pkg/math: SolveCholesky backward solve failed: %w", err)
	}

	return x, nil
}
