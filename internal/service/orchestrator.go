package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	pkgjournal "github.com/optimaldynamics/project-mittens/pkg/journal"
	"github.com/optimaldynamics/project-mittens/pkg/logging"
	"github.com/optimaldynamics/project-mittens/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// OptimizationService coordinates single-epoch carrier optimization batches,
// validating physical feasibility, transitioning immutable MOMDP states, and persisting
// full decision provenance to the Semantic Journal.
type OptimizationService[C model.CompetitorScale] struct {
	journal      Journal
	cryptoStore  pkgjournal.JournalStore
	logger       *slog.Logger
	beliefFilter *model.BeliefFilter[C]
}

// NewOptimizationService initializes a new OptimizationService instance.
func NewOptimizationService[C model.CompetitorScale](journal Journal, logger *slog.Logger) *OptimizationService[C] {
	if journal == nil {
		journal = NewMemoryJournal()
	}
	if logger == nil {
		logger = logging.NewNop()
	}
	return &OptimizationService[C]{
		journal:     journal,
		cryptoStore: pkgjournal.NewMemoryStore(),
		logger:      logger,
	}
}

// WithCryptoStore sets a custom JournalStore (e.g. FileStore or persistent store).
func (s *OptimizationService[C]) WithCryptoStore(store pkgjournal.JournalStore) *OptimizationService[C] {
	if store != nil {
		s.cryptoStore = store
	}
	return s
}

// CryptoStore returns the active cryptographic journal store instance.
func (s *OptimizationService[C]) CryptoStore() pkgjournal.JournalStore {
	return s.cryptoStore
}

// WithBeliefFilter configures an active Bayesian belief filter for competitive market updates.
func (s *OptimizationService[C]) WithBeliefFilter(filter *model.BeliefFilter[C]) *OptimizationService[C] {
	s.beliefFilter = filter
	return s
}

// Journal returns the active decision journal instance.
func (s *OptimizationService[C]) Journal() Journal {
	return s.journal
}

// OptimizeEpoch executes a single discrete optimization step:
//  1. Evaluates the active Powell policy over MOMDP state S_t = (R_t, I_t, b_t).
//  2. Validates joint action a_t = (x_t, p_t) physical feasibility against R_t (Inviolate 5 & 8).
//  3. Emits an immutable JournalEntry to the Semantic Journal (Inviolate 7 & Section 5.4).
//  4. Calculates deterministic physical resource transition R_{t+1} = f_R(R_t, x_t, D_{t+1}).
//  5. Advances information and belief states to produce next state S_{t+1}.
func (s *OptimizationService[C]) OptimizeEpoch(
	ctx context.Context,
	state *model.State[C],
	pol policy.Policy[C],
	nextEpoch int64,
	newLoads []model.Load,
) (*model.Action, policy.DecisionProvenance, *model.State[C], error) {
	if state == nil {
		return nil, policy.DecisionProvenance{}, nil, fmt.Errorf("service: cannot optimize nil state")
	}
	if pol == nil {
		return nil, policy.DecisionProvenance{}, nil, fmt.Errorf("service: cannot optimize with nil policy")
	}

	startTime := time.Now()
	ctx, span := telemetry.StartSpan(ctx, "Optimizer.OptimizeEpoch")
	defer span.End()

	driverCount := len(state.Resource().Drivers())
	loadCount := len(state.Resource().Loads())
	competitorScale := 0
	if state.Belief() != nil {
		competitorScale = state.Belief().Scale().CompetitorDimension()
	}
	span.SetAttributes(telemetry.OptimizationSpanAttributes(pol.Name(), driverCount, loadCount, competitorScale)...)

	logger := logging.FromContext(ctx, s.logger)
	currentEpoch := state.Information().Epoch()

	logger.DebugContext(ctx, "starting epoch optimization",
		slog.Int64("epoch", currentEpoch),
		slog.String("policy", pol.Name()),
		slog.Int("drivers", driverCount),
		slog.Int("loads", loadCount),
	)

	// 1. Evaluate Policy
	action, prov, err := pol.Evaluate(ctx, state)
	if err != nil {
		telemetry.RecordOptimizationDuration(ctx, time.Since(startTime).Seconds(), pol.Name(), "error")
		logger.ErrorContext(ctx, "policy evaluation failed", slog.String("error", err.Error()))
		return nil, policy.DecisionProvenance{}, nil, fmt.Errorf("service: policy evaluation failed: %w", err)
	}

	// 2. Validate Action Physical Feasibility
	if err := action.ValidateFeasibility(state.Resource()); err != nil {
		telemetry.RecordOptimizationDuration(ctx, time.Since(startTime).Seconds(), pol.Name(), "error")
		telemetry.RecordInvariantFailure(ctx, "ActionFeasibilityViolation")
		logger.ErrorContext(ctx, "action feasibility validation failed", slog.String("error", err.Error()))
		return nil, policy.DecisionProvenance{}, nil, fmt.Errorf("service: action validation failed: %w", err)
	}

	// 3. Encode Initial State & Action Hashes for Cryptographic Provenance
	initialStateHash, _ := pkgjournal.HashState(state)
	rBytes, _, _ := pkgjournal.EncodeCanonicalResource(state.Resource())
	iBytes, _, _ := pkgjournal.EncodeCanonicalInformation(state.Information())
	bBytes, _, _ := pkgjournal.EncodeCanonicalBelief(state.Belief())
	aBytes, aHash, _ := pkgjournal.EncodeCanonicalAction(action)

	runID := fmt.Sprintf("RUN-%s", pol.Name())
	decisionID := GenerateDecisionID(pol.Name(), currentEpoch, s.journal.Count()+1)
	prov.OptimizationRunID = decisionID

	// 4. Physical Resource Transition R_{t+1}
	_, transSpan := telemetry.StartSpan(ctx, "State.ResourceTransition")
	nextResource, err := state.Resource().Transition(action.Matches(), newLoads)
	if err != nil {
		transSpan.End()
		telemetry.RecordOptimizationDuration(ctx, time.Since(startTime).Seconds(), pol.Name(), "error")
		logger.ErrorContext(ctx, "resource state transition failed", slog.String("error", err.Error()))
		return nil, policy.DecisionProvenance{}, nil, fmt.Errorf("service: resource transition failed: %w", err)
	}
	transSpan.SetAttributes(
		attribute.Int("transition.active_drivers", len(nextResource.Drivers())),
		attribute.Int("transition.remaining_loads", len(nextResource.Loads())),
	)
	transSpan.End()

	// 5. Belief State Transition b_{t+1}
	nextBelief := state.Belief()
	if s.beliefFilter != nil {
		_, beliefSpan := telemetry.StartSpan(ctx, "MOMDP.BeliefTransition")
		obs := model.NewObservation(nextEpoch, newLoads, nil)
		filtered, err := s.beliefFilter.Filter(state.Belief(), obs, action)
		if err != nil {
			beliefSpan.End()
			telemetry.RecordOptimizationDuration(ctx, time.Since(startTime).Seconds(), pol.Name(), "error")
			telemetry.RecordInvariantFailure(ctx, "BeliefFilterTransitionError")
			logger.ErrorContext(ctx, "belief filter update failed", slog.String("error", err.Error()))
			return nil, policy.DecisionProvenance{}, nil, fmt.Errorf("service: belief filter update failed: %w", err)
		}
		nextBelief = filtered
		beliefSpan.End()
	}

	// 6. Information State Transition I_{t+1}
	if nextEpoch <= currentEpoch {
		nextEpoch = currentEpoch + 3600 // Default 1 hour advance if not specified
	}
	fuelPrice := state.Information().FuelPriceIndex()
	spotRate := state.Information().NationalSpotRateIndex()
	weatherAlerts := state.Information().WeatherAlertCount()

	nextInfo, err := state.Information().Transition(nextEpoch, fuelPrice, spotRate, weatherAlerts)
	if err != nil {
		telemetry.RecordOptimizationDuration(ctx, time.Since(startTime).Seconds(), pol.Name(), "error")
		return nil, policy.DecisionProvenance{}, nil, fmt.Errorf("service: info transition failed: %w", err)
	}

	// 7. Construct Next State S_{t+1}
	nextState, err := model.NewState(nextResource, nextInfo, nextBelief)
	if err != nil {
		telemetry.RecordOptimizationDuration(ctx, time.Since(startTime).Seconds(), pol.Name(), "error")
		return nil, policy.DecisionProvenance{}, nil, fmt.Errorf("service: next state construction failed: %w", err)
	}

	// 8. Seal Cryptographic Journal Record & Merkle Chain Link
	nextStateHash, _ := pkgjournal.HashState(nextState)
	prevHash := pkgjournal.GenesisPrevHash
	if lastRec, ok := s.cryptoStore.LastRecord(runID); ok {
		prevHash = lastRec.RecordHash
	}

	paramHash := pkgjournal.ComputeSHA256([]byte(pol.Name()))
	if len(prov.ThetaParameters) > 0 {
		if pHash, err := pkgjournal.HashParameters(prov.ThetaParameters); err == nil {
			paramHash = pHash
		}
	}

	cryptoRec := pkgjournal.JournalRecord{
		RunID:                 runID,
		Epoch:                 currentEpoch,
		BatchSeq:              s.journal.Count() + 1,
		DecisionID:            decisionID,
		PrevRecordHash:        prevHash,
		RuntimeVersion:        pkgjournal.CurrentRuntimeVersion,
		PolicyName:            pol.Name(),
		PolicyParamHash:       paramHash,
		InitialStateHash:      initialStateHash,
		ResourceStateBytes:    rBytes,
		InformationStateBytes: iBytes,
		BeliefStateBytes:      bBytes,
		ActionHash:            aHash,
		ActionBytes:           aBytes,
		MatchedCount:          action.MatchCount(),
		EvaluatedArcsCount:    len(prov.EvaluatedArcs),
		TotalNetContribution:  prov.TotalNetContribution,
		NextStateHash:         nextStateHash,
	}
	cryptoRec.Seal()

	// 9. Record in Semantic Journal
	entry := JournalEntry{
		DecisionID:           decisionID,
		BatchEpoch:           currentEpoch,
		PolicyName:           pol.Name(),
		MatchedCount:         action.MatchCount(),
		TotalObjective:       prov.TotalObjectiveValue,
		TotalNetContribution: prov.TotalNetContribution,
		Matches:              action.Matches(),
		Provenance:           prov,
		CryptographicRecord:  cryptoRec,
	}
	_, journalSpan := telemetry.StartSpan(ctx, "SemanticJournal.RecordDecision")
	journalSpan.SetAttributes(
		attribute.String("journal.decision_id", decisionID),
		attribute.Int("journal.matched_count", action.MatchCount()),
		attribute.Float64("journal.net_contribution", prov.TotalNetContribution),
		attribute.String("journal.record_hash", cryptoRec.RecordHash),
	)
	if err := s.journal.Record(ctx, entry); err != nil {
		journalSpan.End()
		telemetry.RecordOptimizationDuration(ctx, time.Since(startTime).Seconds(), pol.Name(), "error")
		telemetry.RecordInvariantFailure(ctx, "SemanticJournalRecordFailure")
		logger.ErrorContext(ctx, "failed to record journal entry", slog.String("error", err.Error()))
		return nil, policy.DecisionProvenance{}, nil, fmt.Errorf("service: failed to commit semantic journal entry: %w", err)
	}
	journalSpan.End()

	// Also update standalone cryptoStore if distinct from journal (e.g. MemoryStore)
	if s.cryptoStore != nil {
		if _, isSame := s.journal.(pkgjournal.JournalStore); !isSame {
			_ = s.cryptoStore.Append(cryptoRec)
		}
	}

	durationSec := time.Since(startTime).Seconds()
	telemetry.RecordOptimizationDuration(ctx, durationSec, pol.Name(), "success")
	telemetry.RecordMatchesProduced(ctx, int64(action.MatchCount()), pol.Name())

	span.SetAttributes(
		attribute.Int("optimization.match_count", action.MatchCount()),
		attribute.Float64("optimization.net_contribution", prov.TotalNetContribution),
		attribute.String("optimization.decision_id", decisionID),
		attribute.String("optimization.record_hash", cryptoRec.RecordHash),
	)

	logger.InfoContext(ctx, "epoch optimization completed",
		slog.Int64("epoch", currentEpoch),
		slog.Int("matches", action.MatchCount()),
		slog.Float64("net_contribution", prov.TotalNetContribution),
		slog.Int("remaining_loads", len(nextResource.Loads())),
		slog.Int("active_drivers", len(nextResource.Drivers())),
		slog.String("record_hash", cryptoRec.RecordHash),
	)

	return action, prov, nextState, nil
}
