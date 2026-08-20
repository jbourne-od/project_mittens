package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps a high-performance pgxpool.Pool instance with lifecycle management and healthchecks.
//
// In accordance with Inviolate 6 (Lock-Free Concurrency), the underlying pgx connection pool
// uses non-blocking goroutine allocation and handles concurrent queries safely.
type Pool struct {
	cfg  DBConfig
	pool *pgxpool.Pool
}

// NewPool establishes and verifies a connection pool to the PostgreSQL database.
func NewPool(ctx context.Context, cfg DBConfig) (*Pool, error) {
	poolCfg, err := cfg.ToPgxPoolConfig()
	if err != nil {
		return nil, err
	}

	p, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: failed initializing connection pool: %w", err)
	}

	// Verify connection with health check timeout
	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnTimeout)
	defer cancel()

	if err := p.Ping(pingCtx); err != nil {
		p.Close()
		return nil, fmt.Errorf("db: connection ping failed to %s:%d/%s: %w", cfg.Host, cfg.Port, cfg.Database, err)
	}

	return &Pool{
		cfg:  cfg,
		pool: p,
	}, nil
}

// Ping verifies that the database is reachable and accepting queries.
func (p *Pool) Ping(ctx context.Context) error {
	if p == nil || p.pool == nil {
		return fmt.Errorf("db: pool is uninitialized")
	}
	return p.pool.Ping(ctx)
}

// Close gracefully terminates all pooled database connections.
func (p *Pool) Close() {
	if p != nil && p.pool != nil {
		p.pool.Close()
	}
}

// Exec executes a SQL command without returning rows.
func (p *Pool) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return p.pool.Exec(ctx, sql, arguments...)
}

// Query executes a SQL query returning multiple rows.
func (p *Pool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return p.pool.Query(ctx, sql, args...)
}

// QueryRow executes a SQL query expected to return at most one row.
func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.pool.QueryRow(ctx, sql, args...)
}

// Begin initiates a new transaction on a pooled connection.
func (p *Pool) Begin(ctx context.Context) (pgx.Tx, error) {
	return p.pool.Begin(ctx)
}

// Acquire reserves a single connection from the pool.
func (p *Pool) Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	return p.pool.Acquire(ctx)
}

// Config returns the configuration used to create this pool.
func (p *Pool) Config() DBConfig {
	return p.cfg
}

// Underlying returns the raw pgxpool.Pool handle.
func (p *Pool) Underlying() *pgxpool.Pool {
	return p.pool
}

// Stat returns pool utilization and connection counters.
func (p *Pool) Stat() *pgxpool.Stat {
	if p == nil || p.pool == nil {
		return nil
	}
	return p.pool.Stat()
}
