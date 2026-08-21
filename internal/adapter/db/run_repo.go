package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	// ErrRunNotFound is returned when a requested optimization run does not exist.
	ErrRunNotFound = errors.New("db: optimization run not found")
)

// OptimizationRun represents a top-level optimization batch record in PostgreSQL.
type OptimizationRun struct {
	RunID                string         `json:"run_id"`
	PolicyClass          string         `json:"policy_class"`
	FleetSize            int            `json:"fleet_size"`
	LoadsCount           int            `json:"loads_count"`
	MatchedCount         int            `json:"matched_count"`
	ObjectiveValue       float64        `json:"objective_value"`
	NetContribution      float64        `json:"net_contribution"`
	ExecutionDurationSec float64        `json:"execution_duration_sec"`
	Status               string         `json:"status"`
	Metadata             map[string]any `json:"metadata,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
}

// PostgresRunRepository provides database operations for OptimizationRun entities.
type PostgresRunRepository struct {
	pool *Pool
}

// NewPostgresRunRepository initializes a new PostgresRunRepository.
func NewPostgresRunRepository(pool *Pool) *PostgresRunRepository {
	return &PostgresRunRepository{pool: pool}
}

// Save inserts or updates an optimization run record.
func (r *PostgresRunRepository) Save(ctx context.Context, run OptimizationRun) error {
	metaJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		return fmt.Errorf("db: failed marshaling run metadata: %w", err)
	}

	query := `
		INSERT INTO optimization_runs (
			run_id, policy_class, fleet_size, loads_count, matched_count,
			objective_value, net_contribution, execution_duration_sec,
			status, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, COALESCE($11, NOW()))
		ON CONFLICT (run_id) DO UPDATE SET
			matched_count = EXCLUDED.matched_count,
			objective_value = EXCLUDED.objective_value,
			net_contribution = EXCLUDED.net_contribution,
			execution_duration_sec = EXCLUDED.execution_duration_sec,
			status = EXCLUDED.status,
			metadata = EXCLUDED.metadata;
	`

	var createdAt any = nil
	if !run.CreatedAt.IsZero() {
		createdAt = run.CreatedAt
	}

	_, err = r.pool.Exec(ctx, query,
		run.RunID, run.PolicyClass, run.FleetSize, run.LoadsCount, run.MatchedCount,
		run.ObjectiveValue, run.NetContribution, run.ExecutionDurationSec,
		run.Status, metaJSON, createdAt,
	)
	if err != nil {
		return fmt.Errorf("db: failed saving optimization run %s: %w", run.RunID, err)
	}

	return nil
}

// Get retrieves an optimization run by its RunID.
func (r *PostgresRunRepository) Get(ctx context.Context, runID string) (OptimizationRun, error) {
	query := `
		SELECT run_id, policy_class, fleet_size, loads_count, matched_count,
		       objective_value, net_contribution, execution_duration_sec,
		       status, metadata, created_at
		FROM optimization_runs
		WHERE run_id = $1;
	`

	row := r.pool.QueryRow(ctx, query, runID)

	var run OptimizationRun
	var metaBytes []byte

	err := row.Scan(
		&run.RunID, &run.PolicyClass, &run.FleetSize, &run.LoadsCount, &run.MatchedCount,
		&run.ObjectiveValue, &run.NetContribution, &run.ExecutionDurationSec,
		&run.Status, &metaBytes, &run.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OptimizationRun{}, fmt.Errorf("%w: %s", ErrRunNotFound, runID)
		}
		return OptimizationRun{}, fmt.Errorf("db: failed querying run %s: %w", runID, err)
	}

	if len(metaBytes) > 0 {
		var meta map[string]any
		if err := json.Unmarshal(metaBytes, &meta); err != nil {
			return OptimizationRun{}, fmt.Errorf("db: failed unmarshaling metadata for run %s: %w", runID, err)
		}
		run.Metadata = meta
	}

	return run, nil
}

// List returns recent optimization runs ordered by creation timestamp descending.
func (r *PostgresRunRepository) List(ctx context.Context, limit, offset int) ([]OptimizationRun, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT run_id, policy_class, fleet_size, loads_count, matched_count,
		       objective_value, net_contribution, execution_duration_sec,
		       status, metadata, created_at
		FROM optimization_runs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2;
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("db: failed listing optimization runs: %w", err)
	}
	defer rows.Close()

	var runs []OptimizationRun
	for rows.Next() {
		var run OptimizationRun
		var metaBytes []byte

		err := rows.Scan(
			&run.RunID, &run.PolicyClass, &run.FleetSize, &run.LoadsCount, &run.MatchedCount,
			&run.ObjectiveValue, &run.NetContribution, &run.ExecutionDurationSec,
			&run.Status, &metaBytes, &run.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("db: failed scanning run row: %w", err)
		}

		if len(metaBytes) > 0 {
			var meta map[string]any
			if err := json.Unmarshal(metaBytes, &meta); err != nil {
				return nil, fmt.Errorf("db: failed unmarshaling metadata for run %s: %w", run.RunID, err)
			}
			run.Metadata = meta
		}

		runs = append(runs, run)
	}

	return runs, rows.Err()
}
