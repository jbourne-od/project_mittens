package math

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrLAPEmptyMatrix is returned when an empty cost matrix is provided to the assignment solver.
	ErrLAPEmptyMatrix = errors.New("pkg/math: cost matrix is empty")
	// ErrLAPNonFinite is returned when a cost matrix contains NaN or Inf values.
	ErrLAPNonFinite = errors.New("pkg/math: cost matrix contains non-finite values")
)

// LAPAssignment represents the result of solving a Linear Assignment Problem.
type LAPAssignment struct {
	// RowToCol maps each row index i to its assigned column index j.
	// If row i is unassigned (e.g. Due to negative thresholding), RowToCol[i] = -1.
	RowToCol []int
	// ColToRow maps each column index j to its assigned row index i.
	// If column j is unassigned, ColToRow[j] = -1.
	ColToRow []int
	// TotalWeight is the sum of weights/costs of all active assignments.
	TotalWeight float64
	// MatchCount is the number of successfully matched pairs.
	MatchCount int
	// RowDuals provides optimal Dijkstra dual potentials for each row (driver shadow prices).
	RowDuals []float64
	// ColDuals provides optimal Dijkstra dual potentials for each column (load shadow prices).
	ColDuals []float64
}

// SolveLAP solves the rectangular Linear Assignment Problem (LAP) on an M x N matrix using
// the exact Successive Shortest Path (SSP) algorithm with Dijkstra dual potentials (Jonker-Volgenant foundation).
//
// Parameters:
//   - matrix: M x N weight matrix (M rows, N columns).
//   - maximize: If true, maximizes total weight (payout/profit). If false, minimizes total cost.
//   - allowNegative: If false and maximize is true, matched pairs with negative payout (< 0.0) are
//     pruned, allowing workers/resources to remain idle.
//
// Time Complexity: O(min(M, N)^2 * max(M, N)) polynomial time.
// Space Complexity: O(M + N) auxiliary memory.
//
// In accordance with Inviolate 5 (State Immutability), the input matrix is never modified.
func SolveLAP(matrix [][]float64, maximize bool, allowNegative bool) (LAPAssignment, error) {
	rows := len(matrix)
	if rows == 0 {
		return LAPAssignment{}, ErrLAPEmptyMatrix
	}
	cols := len(matrix[0])
	if cols == 0 {
		return LAPAssignment{}, ErrLAPEmptyMatrix
	}

	for i := 0; i < rows; i++ {
		if len(matrix[i]) != cols {
			return LAPAssignment{}, fmt.Errorf("%w: row %d length %d != cols %d",
				ErrDimensionMismatch, i, len(matrix[i]), cols)
		}
		for j := 0; j < cols; j++ {
			val := matrix[i][j]
			if math.IsNaN(val) || math.IsInf(val, 0) {
				return LAPAssignment{}, fmt.Errorf("%w: at (%d, %d)", ErrLAPNonFinite, i, j)
			}
		}
	}

	// If rows > cols, transpose the problem so n <= m (rows <= cols)
	transposed := rows > cols
	n := rows
	m := cols
	if transposed {
		n = cols
		m = rows
	}

	// Build 1-indexed cost matrix for minimization
	// cost[i][j] (1 <= i <= n, 1 <= j <= m)
	cost := make([][]float64, n+1)
	for i := 0; i <= n; i++ {
		cost[i] = make([]float64, m+1)
	}

	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			raw := matrix[i][j]
			val := raw
			if maximize {
				val = -raw // Negate to turn max-weight into min-cost
			}

			if transposed {
				cost[j+1][i+1] = val
			} else {
				cost[i+1][j+1] = val
			}
		}
	}

	// Dual potentials and assignment vectors (1-indexed)
	u := make([]float64, n+1)
	v := make([]float64, m+1)
	p := make([]int, m+1)   // p[j] = row matched to col j
	way := make([]int, m+1) // predecessor column in augmenting path

	minv := make([]float64, m+1)
	used := make([]bool, m+1)

	// Augment matching row by row
	for i := 1; i <= n; i++ {
		p[0] = i
		j0 := 0
		for j := 0; j <= m; j++ {
			minv[j] = math.Inf(1)
			used[j] = false
		}

		for {
			used[j0] = true
			i0 := p[j0]
			delta := math.Inf(1)
			j1 := 0

			for j := 1; j <= m; j++ {
				if !used[j] {
					cur := cost[i0][j] - u[i0] - v[j]
					if cur < minv[j] {
						minv[j] = cur
						way[j] = j0
					}
					if minv[j] < delta {
						delta = minv[j]
						j1 = j
					}
				}
			}

			for j := 0; j <= m; j++ {
				if used[j] {
					u[p[j]] += delta
					v[j] -= delta
				} else {
					minv[j] -= delta
				}
			}

			j0 = j1
			if p[j0] == 0 {
				break
			}
		}

		// Update augmenting path
		for {
			j1 := way[j0]
			p[j0] = p[j1]
			j0 = j1
			if j0 == 0 {
				break
			}
		}
	}

	// Extract solution back to 0-indexed structures
	rowToCol := make([]int, rows)
	for i := range rowToCol {
		rowToCol[i] = -1
	}
	colToRow := make([]int, cols)
	for j := range colToRow {
		colToRow[j] = -1
	}

	if !transposed {
		// p[j] is the 1-indexed row assigned to column j
		for j := 1; j <= m; j++ {
			r1 := p[j]
			if r1 > 0 && r1 <= rows {
				r := r1 - 1
				c := j - 1
				rowToCol[r] = c
				colToRow[c] = r
			}
		}
	} else {
		// Transposed: p[j] is the 1-indexed column assigned to row j
		for j := 1; j <= m; j++ {
			c1 := p[j]
			if c1 > 0 && c1 <= cols {
				c := c1 - 1
				r := j - 1
				rowToCol[r] = c
				colToRow[c] = r
			}
		}
	}

	// Filter out non-profitable / negative matches if !allowNegative
	totalWeight := 0.0
	matchCount := 0

	for r := 0; r < rows; r++ {
		c := rowToCol[r]
		if c == -1 {
			continue
		}

		weight := matrix[r][c]
		if !allowNegative && maximize && weight < 0.0 {
			// Prune negative payout assignment (driver stays idle)
			rowToCol[r] = -1
			colToRow[c] = -1
			continue
		}

		totalWeight += weight
		matchCount++
	}

	rowDuals := make([]float64, rows)
	colDuals := make([]float64, cols)

	if !transposed {
		for i := 0; i < rows; i++ {
			if maximize {
				rowDuals[i] = -u[i+1]
			} else {
				rowDuals[i] = u[i+1]
			}
		}
		for j := 0; j < cols; j++ {
			if maximize {
				colDuals[j] = -v[j+1]
			} else {
				colDuals[j] = v[j+1]
			}
		}
	} else {
		for i := 0; i < rows; i++ {
			if maximize {
				rowDuals[i] = -v[i+1]
			} else {
				rowDuals[i] = v[i+1]
			}
		}
		for j := 0; j < cols; j++ {
			if maximize {
				colDuals[j] = -u[j+1]
			} else {
				colDuals[j] = u[j+1]
			}
		}
	}

	return LAPAssignment{
		RowToCol:    rowToCol,
		ColToRow:    colToRow,
		TotalWeight: totalWeight,
		MatchCount:  matchCount,
		RowDuals:    rowDuals,
		ColDuals:    colDuals,
	}, nil
}
