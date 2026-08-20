-- ==============================================================================
-- PROJECT MITTENS: PostgreSQL Schema DDL
-- ==============================================================================
-- Tables:
--   1. optimization_runs: High-level optimization batch metadata & metrics
--   2. decision_journal: Cryptographic append-only Semantic Journal & Merkle chain
--   3. fleet_drivers: Live driver resource state and HOS timelines
--   4. fleet_loads: Active stochastic/spot load tenders and statuses
-- ==============================================================================

CREATE TABLE IF NOT EXISTS optimization_runs (
    run_id TEXT PRIMARY KEY,
    policy_class TEXT NOT NULL,
    fleet_size INTEGER NOT NULL,
    loads_count INTEGER NOT NULL,
    matched_count INTEGER NOT NULL DEFAULT 0,
    objective_value DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    net_contribution DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    execution_duration_sec DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    status TEXT NOT NULL DEFAULT 'COMPLETED',
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS decision_journal (
    decision_id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES optimization_runs(run_id) ON DELETE CASCADE,
    epoch BIGINT NOT NULL,
    batch_seq INTEGER NOT NULL,
    policy_name TEXT NOT NULL,
    runtime_version TEXT NOT NULL,
    policy_param_hash TEXT,
    prev_record_hash TEXT NOT NULL,
    record_hash TEXT NOT NULL UNIQUE,
    initial_state_hash TEXT NOT NULL,
    resource_state_bytes BYTEA,
    information_state_bytes BYTEA,
    belief_state_bytes BYTEA,
    action_hash TEXT NOT NULL,
    action_bytes BYTEA,
    matched_count INTEGER NOT NULL DEFAULT 0,
    evaluated_arcs_count INTEGER NOT NULL DEFAULT 0,
    total_net_contribution DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    realized_observations_bytes BYTEA,
    next_state_hash TEXT,
    next_resource_state_bytes BYTEA,
    next_information_state_bytes BYTEA,
    next_belief_state_bytes BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fleet_drivers (
    driver_id TEXT PRIMARY KEY,
    current_node TEXT NOT NULL,
    home_node TEXT NOT NULL,
    current_lat DOUBLE PRECISION NOT NULL,
    current_lon DOUBLE PRECISION NOT NULL,
    available_epoch BIGINT NOT NULL,
    drive_hours_remaining DOUBLE PRECISION NOT NULL,
    duty_hours_remaining DOUBLE PRECISION NOT NULL,
    assigned_load_id TEXT,
    equipment_type TEXT NOT NULL DEFAULT 'DRY_VAN',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fleet_loads (
    load_id TEXT PRIMARY KEY,
    origin_node TEXT NOT NULL,
    dest_node TEXT NOT NULL,
    pickup_earliest_epoch BIGINT NOT NULL,
    pickup_latest_epoch BIGINT NOT NULL,
    delivery_earliest_epoch BIGINT NOT NULL,
    delivery_latest_epoch BIGINT NOT NULL,
    revenue DOUBLE PRECISION NOT NULL,
    required_equipment TEXT NOT NULL DEFAULT 'DRY_VAN',
    status TEXT NOT NULL DEFAULT 'AVAILABLE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indices for rapid lookup and Merkle chain traversal
CREATE INDEX IF NOT EXISTS idx_decision_journal_run_seq ON decision_journal(run_id, batch_seq ASC);
CREATE INDEX IF NOT EXISTS idx_decision_journal_record_hash ON decision_journal(record_hash);
CREATE INDEX IF NOT EXISTS idx_decision_journal_created_at ON decision_journal(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_optimization_runs_created_at ON optimization_runs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_fleet_drivers_node ON fleet_drivers(current_node);
CREATE INDEX IF NOT EXISTS idx_fleet_loads_status ON fleet_loads(status);
