package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/internal/service"
	"github.com/optimaldynamics/project-mittens/pkg/journal"
)

// PostgresJournalStore provides a production-grade PostgreSQL implementation of both
// pkg/journal.JournalStore (Merkle chain audit trail) and internal/service.Journal (decision store).
//
// In accordance with Section 5.4 (Semantic Journaling) and Inviolate 7 (Complete Provenance),
// every dispatch decision is committed within a transaction verifying hash chain continuity.
type PostgresJournalStore struct {
	pool       *Pool
	memJournal *service.MemoryJournal
}

// NewPostgresJournalStore initializes a new PostgresJournalStore.
func NewPostgresJournalStore(pool *Pool) *PostgresJournalStore {
	return &PostgresJournalStore{
		pool:       pool,
		memJournal: service.NewMemoryJournal(),
	}
}

// Append commits a sealed JournalRecord to PostgreSQL within a transaction, verifying Merkle continuity.
func (s *PostgresJournalStore) Append(rec journal.JournalRecord) error {
	if !rec.VerifyIntegrity() {
		return fmt.Errorf("%w: record %s failed self-integrity hash check", journal.ErrCorruptedChain, rec.DecisionID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: failed starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Ensure parent optimization_run exists to satisfy foreign key
	ensureRunSQL := `
		INSERT INTO optimization_runs (run_id, policy_class, fleet_size, loads_count, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (run_id) DO NOTHING;
	`
	policyName := "UNKNOWN"
	if rec.PolicyName != "" {
		policyName = rec.PolicyName
	}
	if _, err := tx.Exec(ctx, ensureRunSQL, rec.RunID, policyName, 0, 0); err != nil {
		return fmt.Errorf("db: failed ensuring parent run %s: %w", rec.RunID, err)
	}

	// 2. Query last record for this run to verify Merkle chain link
	lastRecSQL := `
		SELECT decision_id, record_hash, batch_seq
		FROM decision_journal
		WHERE run_id = $1
		ORDER BY batch_seq DESC
		LIMIT 1
		FOR UPDATE;
	`
	var lastDecisionID, lastRecordHash string
	var lastBatchSeq int

	row := tx.QueryRow(ctx, lastRecSQL, rec.RunID)
	err = row.Scan(&lastDecisionID, &lastRecordHash, &lastBatchSeq)

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("db: failed querying previous record in run %s: %w", rec.RunID, err)
	}

	if errors.Is(err, pgx.ErrNoRows) {
		// Genesis record check
		if rec.PrevRecordHash != journal.GenesisPrevHash {
			return fmt.Errorf("%w: genesis record %s must have genesis prev hash, got %s",
				journal.ErrCorruptedChain, rec.DecisionID, rec.PrevRecordHash)
		}
	} else {
		// Merkle link continuity check
		if rec.PrevRecordHash != lastRecordHash {
			return fmt.Errorf("%w: record %s prev hash %s does not match previous record %s hash %s",
				journal.ErrCorruptedChain, rec.DecisionID, rec.PrevRecordHash, lastDecisionID, lastRecordHash)
		}
	}

	// 3. Insert Journal Record
	insertSQL := `
		INSERT INTO decision_journal (
			decision_id, run_id, epoch, batch_seq, policy_name,
			runtime_version, policy_param_hash, prev_record_hash, record_hash,
			initial_state_hash, resource_state_bytes, information_state_bytes, belief_state_bytes,
			action_hash, action_bytes, matched_count, evaluated_arcs_count, total_net_contribution,
			realized_observations_bytes, next_state_hash, next_resource_state_bytes,
			next_information_state_bytes, next_belief_state_bytes, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $17, $18,
			$19, $20, $21,
			$22, $23, NOW()
		);
	`

	runtimeVer := rec.RuntimeVersion
	if runtimeVer == "" {
		runtimeVer = journal.CurrentRuntimeVersion
	}

	_, err = tx.Exec(ctx, insertSQL,
		rec.DecisionID, rec.RunID, rec.Epoch, rec.BatchSeq, policyName,
		runtimeVer, rec.PolicyParamHash, rec.PrevRecordHash, rec.RecordHash,
		rec.InitialStateHash, rec.ResourceStateBytes, rec.InformationStateBytes, rec.BeliefStateBytes,
		rec.ActionHash, rec.ActionBytes, rec.MatchedCount, rec.EvaluatedArcsCount, rec.TotalNetContribution,
		rec.RealizedObservationsBytes, rec.NextStateHash, rec.NextResourceStateBytes,
		rec.NextInformationStateBytes, rec.NextBeliefStateBytes,
	)
	if err != nil {
		return fmt.Errorf("db: failed inserting journal record %s: %w", rec.DecisionID, err)
	}

	return tx.Commit(ctx)
}

// Get retrieves a record by its unique DecisionID.
func (s *PostgresJournalStore) Get(decisionID string) (journal.JournalRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT decision_id, run_id, epoch, batch_seq, policy_name,
		       runtime_version, policy_param_hash, prev_record_hash, record_hash,
		       initial_state_hash, resource_state_bytes, information_state_bytes, belief_state_bytes,
		       action_hash, action_bytes, matched_count, evaluated_arcs_count, total_net_contribution,
		       realized_observations_bytes, next_state_hash, next_resource_state_bytes,
		       next_information_state_bytes, next_belief_state_bytes
		FROM decision_journal
		WHERE decision_id = $1;
	`

	row := s.pool.QueryRow(ctx, query, decisionID)
	return scanJournalRecord(row)
}

// ListByRun returns all records for a given RunID in logical execution order.
func (s *PostgresJournalStore) ListByRun(runID string) ([]journal.JournalRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	query := `
		SELECT decision_id, run_id, epoch, batch_seq, policy_name,
		       runtime_version, policy_param_hash, prev_record_hash, record_hash,
		       initial_state_hash, resource_state_bytes, information_state_bytes, belief_state_bytes,
		       action_hash, action_bytes, matched_count, evaluated_arcs_count, total_net_contribution,
		       realized_observations_bytes, next_state_hash, next_resource_state_bytes,
		       next_information_state_bytes, next_belief_state_bytes
		FROM decision_journal
		WHERE run_id = $1
		ORDER BY batch_seq ASC;
	`

	rows, err := s.pool.Query(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("db: failed querying records for run %s: %w", runID, err)
	}
	defer rows.Close()

	var records []journal.JournalRecord
	for rows.Next() {
		rec, err := scanJournalRecordFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("db: failed scanning journal record: %w", err)
		}
		records = append(records, rec)
	}

	return records, rows.Err()
}

// LastRecord returns the latest record committed for a given RunID.
func (s *PostgresJournalStore) LastRecord(runID string) (journal.JournalRecord, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT decision_id, run_id, epoch, batch_seq, policy_name,
		       runtime_version, policy_param_hash, prev_record_hash, record_hash,
		       initial_state_hash, resource_state_bytes, information_state_bytes, belief_state_bytes,
		       action_hash, action_bytes, matched_count, evaluated_arcs_count, total_net_contribution,
		       realized_observations_bytes, next_state_hash, next_resource_state_bytes,
		       next_information_state_bytes, next_belief_state_bytes
		FROM decision_journal
		WHERE run_id = $1
		ORDER BY batch_seq DESC
		LIMIT 1;
	`

	row := s.pool.QueryRow(ctx, query, runID)
	rec, err := scanJournalRecord(row)
	if err != nil {
		return journal.JournalRecord{}, false
	}
	return rec, true
}

// VerifyRunChain validates the full cryptographic hash chain for a run.
func (s *PostgresJournalStore) VerifyRunChain(runID string) (bool, string, error) {
	records, err := s.ListByRun(runID)
	if err != nil {
		return false, "", err
	}
	if len(records) == 0 {
		// Fallback: If runID is actually a decisionID, lookup the record to find its parent RunID
		if rec, getErr := s.Get(runID); getErr == nil && rec.RunID != "" {
			records, _ = s.ListByRun(rec.RunID)
		}
	}
	if len(records) == 0 {
		return true, "empty_run", nil
	}

	for i, r := range records {
		if !r.VerifyIntegrity() {
			return false, r.DecisionID, fmt.Errorf("%w: record %s self-hash mismatch", journal.ErrCorruptedChain, r.DecisionID)
		}
		if i == 0 {
			if r.PrevRecordHash != journal.GenesisPrevHash {
				return false, r.DecisionID, fmt.Errorf("%w: genesis record %s has invalid prev hash", journal.ErrCorruptedChain, r.DecisionID)
			}
		} else {
			prev := records[i-1]
			if r.PrevRecordHash != prev.RecordHash {
				return false, r.DecisionID, fmt.Errorf("%w: link broken between %s and %s", journal.ErrCorruptedChain, prev.DecisionID, r.DecisionID)
			}
		}
	}
	return true, records[len(records)-1].RecordHash, nil
}

// -----------------------------------------------------------------------------
// service.Journal Interface Implementation
// -----------------------------------------------------------------------------

// Record persists a service.JournalEntry to PostgreSQL and retains rich in-memory attribution.
func (s *PostgresJournalStore) Record(ctx context.Context, entry service.JournalEntry) error {
	if s.memJournal != nil {
		_ = s.memJournal.Record(ctx, entry)
	}

	rec := entry.CryptographicRecord
	if rec.DecisionID == "" {
		rec.DecisionID = entry.DecisionID
	}
	if rec.RunID == "" {
		rec.RunID = entry.Provenance.OptimizationRunID
		if rec.RunID == "" {
			rec.RunID = "DEFAULT_RUN"
		}
	}
	if rec.PolicyName == "" {
		rec.PolicyName = entry.PolicyName
	}
	if rec.Epoch == 0 {
		rec.Epoch = entry.BatchEpoch
	}
	if rec.MatchedCount == 0 {
		rec.MatchedCount = entry.MatchedCount
	}
	if rec.TotalNetContribution == 0 {
		rec.TotalNetContribution = entry.TotalNetContribution
	}
	if rec.RuntimeVersion == "" {
		rec.RuntimeVersion = journal.CurrentRuntimeVersion
	}
	if rec.PrevRecordHash == "" {
		if last, found := s.LastRecord(rec.RunID); found {
			rec.PrevRecordHash = last.RecordHash
			rec.BatchSeq = last.BatchSeq + 1
		} else {
			rec.PrevRecordHash = journal.GenesisPrevHash
			rec.BatchSeq = 0
		}
	}
	if rec.RecordHash == "" {
		rec.RecordHash = rec.ComputeRecordHash()
	}

	return s.Append(rec)
}

// GetEntries returns recent recorded entries across optimization runs.
func (s *PostgresJournalStore) GetEntries() []service.JournalEntry {
	if s.memJournal != nil {
		memEntries := s.memJournal.GetEntries()
		if len(memEntries) > 0 {
			return memEntries
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
		SELECT decision_id, run_id, epoch, batch_seq, policy_name,
		       runtime_version, policy_param_hash, prev_record_hash, record_hash,
		       initial_state_hash, resource_state_bytes, information_state_bytes, belief_state_bytes,
		       action_hash, action_bytes, matched_count, evaluated_arcs_count, total_net_contribution,
		       realized_observations_bytes, next_state_hash, next_resource_state_bytes,
		       next_information_state_bytes, next_belief_state_bytes
		FROM decision_journal
		ORDER BY created_at DESC
		LIMIT 100;
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var entries []service.JournalEntry
	for rows.Next() {
		rec, err := scanJournalRecordFromRows(rows)
		if err != nil {
			continue
		}
		entries = append(entries, toServiceEntry(rec))
	}
	return entries
}

// GetEntry retrieves a specific JournalEntry by its decisionID.
func (s *PostgresJournalStore) GetEntry(decisionID string) (service.JournalEntry, bool) {
	if s.memJournal != nil {
		if entry, found := s.memJournal.GetEntry(decisionID); found {
			return entry, true
		}
	}

	rec, err := s.Get(decisionID)
	if err != nil {
		return service.JournalEntry{}, false
	}
	return toServiceEntry(rec), true
}

// Count returns the total number of recorded journal entries.
func (s *PostgresJournalStore) Count() int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM decision_journal;").Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

func toServiceEntry(rec journal.JournalRecord) service.JournalEntry {
	return service.JournalEntry{
		DecisionID:           rec.DecisionID,
		BatchEpoch:           rec.Epoch,
		PolicyName:           rec.PolicyName,
		MatchedCount:         rec.MatchedCount,
		TotalObjective:       rec.TotalNetContribution,
		TotalNetContribution: rec.TotalNetContribution,
		Matches:              nil,
		Provenance: policy.DecisionProvenance{
			OptimizationRunID:    rec.RunID,
			BatchEpoch:           rec.Epoch,
			PolicyName:           rec.PolicyName,
			MatchedCount:         rec.MatchedCount,
			TotalNetContribution: rec.TotalNetContribution,
			TotalObjectiveValue:  rec.TotalNetContribution,
		},
		CryptographicRecord: rec,
	}
}

func scanJournalRecord(row pgx.Row) (journal.JournalRecord, error) {
	var rec journal.JournalRecord

	err := row.Scan(
		&rec.DecisionID, &rec.RunID, &rec.Epoch, &rec.BatchSeq, &rec.PolicyName,
		&rec.RuntimeVersion, &rec.PolicyParamHash, &rec.PrevRecordHash, &rec.RecordHash,
		&rec.InitialStateHash, &rec.ResourceStateBytes, &rec.InformationStateBytes, &rec.BeliefStateBytes,
		&rec.ActionHash, &rec.ActionBytes, &rec.MatchedCount, &rec.EvaluatedArcsCount, &rec.TotalNetContribution,
		&rec.RealizedObservationsBytes, &rec.NextStateHash, &rec.NextResourceStateBytes,
		&rec.NextInformationStateBytes, &rec.NextBeliefStateBytes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return journal.JournalRecord{}, fmt.Errorf("%w", journal.ErrRecordNotFound)
		}
		return journal.JournalRecord{}, err
	}

	return rec, nil
}

func scanJournalRecordFromRows(rows pgx.Rows) (journal.JournalRecord, error) {
	var rec journal.JournalRecord

	err := rows.Scan(
		&rec.DecisionID, &rec.RunID, &rec.Epoch, &rec.BatchSeq, &rec.PolicyName,
		&rec.RuntimeVersion, &rec.PolicyParamHash, &rec.PrevRecordHash, &rec.RecordHash,
		&rec.InitialStateHash, &rec.ResourceStateBytes, &rec.InformationStateBytes, &rec.BeliefStateBytes,
		&rec.ActionHash, &rec.ActionBytes, &rec.MatchedCount, &rec.EvaluatedArcsCount, &rec.TotalNetContribution,
		&rec.RealizedObservationsBytes, &rec.NextStateHash, &rec.NextResourceStateBytes,
		&rec.NextInformationStateBytes, &rec.NextBeliefStateBytes,
	)
	if err != nil {
		return journal.JournalRecord{}, err
	}

	return rec, nil
}
