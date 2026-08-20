package replay_test

import (
	"context"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/pkg/journal"
	"github.com/optimaldynamics/project-mittens/pkg/replay"
)

func createTestScenario() (*model.State[model.AggregatedMarket], policy.Policy[model.AggregatedMarket]) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{
			ID:                  "DRV-01",
			CurrentLocation:     locChi,
			HomeLocation:        locChi,
			AvailableEpoch:      startEpoch,
			DriveHoursRemaining: 11.0,
			DutyHoursRemaining:  14.0,
			Equipment:           model.Equipment{Type: model.EquipDryVan},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
	}

	loads := []model.Load{
		{
			ID:                    "LOAD-01",
			Origin:                locChi,
			Destination:           locAtl,
			RequiredEquipment:     model.EquipDryVan,
			PickupEarliestEpoch:   startEpoch,
			PickupLatestEpoch:     startEpoch + 36000,
			DeliveryEarliestEpoch: startEpoch + 18000,
			DeliveryLatestEpoch:   startEpoch + 120000,
			Revenue:               3000.0,
		},
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(startEpoch, 2.50, 3.50, 1)
	scale := model.AggregatedMarket{}
	states := []string{"AGGRESSIVE", "MODERATE", "PASSIVE"}
	belief, _ := model.NewBelief(scale, states, []float64{0.10, 0.20, 0.70})

	state, _ := model.NewState(res, info, belief)

	cfa := policy.NewCFAPolicy[model.AggregatedMarket](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	return state, cfa
}

func TestReplayEngine_BitExactVerification(t *testing.T) {
	state, pol := createTestScenario()
	ctx := context.Background()

	// 1. Initial Execution & Record Creation
	action, prov, err := pol.Evaluate(ctx, state)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	sHash, _ := journal.HashState(state)
	_, aHash, _ := journal.EncodeCanonicalAction(action)

	rec := journal.JournalRecord{
		RunID:                "RUN-REPLAY-01",
		Epoch:                state.Information().Epoch(),
		BatchSeq:             1,
		DecisionID:           "DEC-REPLAY-001",
		RuntimeVersion:       journal.CurrentRuntimeVersion,
		PolicyName:           pol.Name(),
		InitialStateHash:     sHash,
		ActionHash:           aHash,
		MatchedCount:         action.MatchCount(),
		TotalNetContribution: prov.TotalNetContribution,
	}
	rec.Seal()

	// 2. Offline Replay Execution
	engine, err := replay.NewReplayEngine[model.AggregatedMarket](pol)
	if err != nil {
		t.Fatalf("NewReplayEngine failed: %v", err)
	}

	report, err := engine.ReplayDecision(ctx, rec, state)
	if err != nil {
		t.Fatalf("ReplayDecision failed: %v", err)
	}

	if !report.IsBitExact {
		t.Fatalf("expected bit-exact replay, got drifts: %v", report.DriftDetails)
	}
	if !report.InitialStateHashMatch {
		t.Errorf("expected initial state hash match")
	}
	if !report.ActionHashMatch {
		t.Errorf("expected action hash match")
	}
	if report.ContributionDelta != 0.0 {
		t.Errorf("expected 0 contribution delta, got %f", report.ContributionDelta)
	}
}

func TestReplayEngine_DriftDetection(t *testing.T) {
	state, pol := createTestScenario()
	ctx := context.Background()

	// Record with deliberately altered net contribution and action hash
	sHash, _ := journal.HashState(state)
	rec := journal.JournalRecord{
		RunID:                "RUN-DRIFT-01",
		Epoch:                state.Information().Epoch(),
		BatchSeq:             1,
		DecisionID:           "DEC-DRIFT-001",
		RuntimeVersion:       journal.CurrentRuntimeVersion,
		PolicyName:           pol.Name(),
		InitialStateHash:     sHash,
		ActionHash:           "tampered-action-hash",
		MatchedCount:         999,
		TotalNetContribution: 99999.0,
	}
	rec.Seal()

	engine, _ := replay.NewReplayEngine[model.AggregatedMarket](pol)
	report, err := engine.ReplayDecision(ctx, rec, state)
	if err != nil {
		t.Fatalf("ReplayDecision failed: %v", err)
	}

	if report.IsBitExact {
		t.Errorf("expected drift detection, but report marked bit-exact")
	}
	if report.ActionHashMatch {
		t.Errorf("expected action hash mismatch")
	}
	if len(report.DriftDetails) == 0 {
		t.Errorf("expected drift details explaining discrepancies")
	}
}
