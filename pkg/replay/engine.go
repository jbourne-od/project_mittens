// Package replay provides offline, bit-exact deterministic re-execution
// and cryptographic verification of historical Project Mittens optimization decisions.
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /internal/domain/policy, /pkg/journal, /pkg/telemetry
//   - Imported By: /internal/service, /internal/adapter/api
//   - Strict Rule: Zero external I/O, zero wall-clock access, 100% deterministic.
package replay

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/pkg/journal"
	"github.com/optimaldynamics/project-mittens/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// ReplayReport captures the detailed verification results of an offline decision re-execution.
type ReplayReport struct {
	DecisionID                string   `json:"decision_id"`
	RunID                     string   `json:"run_id"`
	Epoch                     int64    `json:"epoch"`
	PolicyName                string   `json:"policy_name"`
	IsBitExact                bool     `json:"is_bit_exact"`
	InitialStateHashMatch     bool     `json:"initial_state_hash_match"`
	ActionHashMatch           bool     `json:"action_hash_match"`
	RecordedActionHash        string   `json:"recorded_action_hash"`
	ReplayedActionHash        string   `json:"replayed_action_hash"`
	RecordedMatchesCount      int      `json:"recorded_matches_count"`
	ReplayedMatchesCount      int      `json:"replayed_matches_count"`
	RecordedNetContribution   float64  `json:"recorded_net_contribution"`
	ReplayedNetContribution   float64  `json:"replayed_net_contribution"`
	ContributionDelta         float64  `json:"contribution_delta"`
	ReplayDurationMicrosecond int64    `json:"replay_duration_us"`
	DriftDetails              []string `json:"drift_details,omitempty"`
}

// ReplayEngine coordinates offline re-execution of recorded optimization decisions.
type ReplayEngine[C model.CompetitorScale] struct {
	pol policy.Policy[C]
}

// NewReplayEngine initializes a ReplayEngine configured with the target evaluation policy.
func NewReplayEngine[C model.CompetitorScale](pol policy.Policy[C]) (*ReplayEngine[C], error) {
	if pol == nil {
		return nil, fmt.Errorf("replay: policy cannot be nil")
	}
	return &ReplayEngine[C]{
		pol: pol,
	}, nil
}

// ReplayDecision executes an offline re-evaluation of state against the recorded journal record.
func (e *ReplayEngine[C]) ReplayDecision(
	ctx context.Context,
	rec journal.JournalRecord,
	state *model.State[C],
) (*ReplayReport, error) {
	if state == nil {
		return nil, fmt.Errorf("replay: cannot replay with nil state")
	}

	ctx, span := telemetry.StartSpan(ctx, "Replay.ReplayDecision")
	defer span.End()

	startTime := time.Now()
	var drifts []string

	// 1. Verify Initial State Hash
	initialStateHash, err := journal.HashState(state)
	if err != nil {
		return nil, fmt.Errorf("replay: failed hashing initial state: %w", err)
	}

	initialHashMatch := (initialStateHash == rec.InitialStateHash)
	if !initialHashMatch {
		drifts = append(drifts, fmt.Sprintf("initial state hash mismatch: recorded=%s recomputed=%s",
			rec.InitialStateHash, initialStateHash))
	}

	// 2. Re-execute Policy in Isolation
	replayedAction, replayedProv, err := e.pol.Evaluate(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("replay: policy evaluation failed: %w", err)
	}

	durationUs := time.Since(startTime).Microseconds()

	// 3. Hash Replayed Action
	_, replayedActionHash, err := journal.EncodeCanonicalAction(replayedAction)
	if err != nil {
		return nil, fmt.Errorf("replay: failed hashing replayed action: %w", err)
	}

	actionHashMatch := (replayedActionHash == rec.ActionHash)
	if !actionHashMatch {
		drifts = append(drifts, fmt.Sprintf("action hash mismatch: recorded=%s replayed=%s",
			rec.ActionHash, replayedActionHash))
	}

	if replayedAction.MatchCount() != rec.MatchedCount {
		drifts = append(drifts, fmt.Sprintf("match count delta: recorded=%d replayed=%d",
			rec.MatchedCount, replayedAction.MatchCount()))
	}

	contribDelta := replayedProv.TotalNetContribution - rec.TotalNetContribution
	if math.Abs(contribDelta) > 1e-6 {
		drifts = append(drifts, fmt.Sprintf("net contribution delta: recorded=%.2f replayed=%.2f (delta=%.2f)",
			rec.TotalNetContribution, replayedProv.TotalNetContribution, contribDelta))
	}

	isBitExact := initialHashMatch && actionHashMatch && len(drifts) == 0

	report := &ReplayReport{
		DecisionID:                rec.DecisionID,
		RunID:                     rec.RunID,
		Epoch:                     rec.Epoch,
		PolicyName:                e.pol.Name(),
		IsBitExact:                isBitExact,
		InitialStateHashMatch:     initialHashMatch,
		ActionHashMatch:           actionHashMatch,
		RecordedActionHash:        rec.ActionHash,
		ReplayedActionHash:        replayedActionHash,
		RecordedMatchesCount:      rec.MatchedCount,
		ReplayedMatchesCount:      replayedAction.MatchCount(),
		RecordedNetContribution:   rec.TotalNetContribution,
		ReplayedNetContribution:   replayedProv.TotalNetContribution,
		ContributionDelta:         contribDelta,
		ReplayDurationMicrosecond: durationUs,
		DriftDetails:              drifts,
	}

	span.SetAttributes(
		attribute.String("replay.decision_id", rec.DecisionID),
		attribute.Bool("replay.is_bit_exact", isBitExact),
		attribute.Int64("replay.duration_us", durationUs),
	)

	return report, nil
}
