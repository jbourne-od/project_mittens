package explain_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/pkg/explain"
)

func TestExplainer_CompleteBreakdownAndCounterfactuals(t *testing.T) {
	explainer := explain.NewExplainer()
	formatter := explain.NewFormatter()

	prov := policy.DecisionProvenance{
		OptimizationRunID:    "OPT-RUN-1001",
		BatchEpoch:           1700000000,
		PolicyName:           "CFA",
		ThetaParameters:      []float64{1.0, 1.2, 1.0, 0.5},
		MatchedCount:         1,
		TotalNetContribution: 1800.0,
		TotalObjectiveValue:  1950.0,
		EvaluatedArcs: []policy.CandidateEvaluation{
			// Driver 1: Winning Match to Load A
			{
				DriverID: "D1",
				LoadID:   "L-A",
				CostBreakdown: policy.TripCostBreakdown{
					Revenue:         2500.0,
					LoadedCost:      500.0,
					EmptyCost:       100.0,
					EmptyToHomeCost: 50.0,
					DwellCost:       50.0,
					TotalCost:       700.0,
					NetContribution: 1800.0, // 2500 - 700
				},
				VFAValue:           150.0,
				TotalScore:         1950.0, // 1800 + 150
				PostDecisionRegion: "MIDWEST",
				DeadheadMiles:      35.0,
				LoadedMiles:        450.0,
				InsertedDwellMin:   30,
				IsAssigned:         true,
			},
			// Driver 1: Rejected Candidate Load B (Lower score)
			{
				DriverID: "D1",
				LoadID:   "L-B",
				CostBreakdown: policy.TripCostBreakdown{
					Revenue:         2000.0,
					LoadedCost:      600.0,
					EmptyCost:       300.0, // High deadhead
					TotalCost:       900.0,
					NetContribution: 1100.0,
				},
				VFAValue:           50.0,
				TotalScore:         1150.0,
				PostDecisionRegion: "SOUTHEAST",
				DeadheadMiles:      120.0,
				LoadedMiles:        400.0,
				IsAssigned:         false,
			},
			// Driver 1: Rejected Candidate Load C (Negative net contribution)
			{
				DriverID: "D1",
				LoadID:   "L-C",
				CostBreakdown: policy.TripCostBreakdown{
					Revenue:         800.0,
					LoadedCost:      500.0,
					EmptyCost:       450.0,
					TotalCost:       950.0,
					NetContribution: -150.0,
				},
				TotalScore:         -150.0,
				PostDecisionRegion: "NORTHEAST",
				DeadheadMiles:      180.0,
				LoadedMiles:        200.0,
				IsAssigned:         false,
			},
			// Driver 2: Idle (Negative candidate only)
			{
				DriverID: "D2",
				LoadID:   "L-D",
				CostBreakdown: policy.TripCostBreakdown{
					Revenue:         600.0,
					LoadedCost:      400.0,
					EmptyCost:       350.0,
					TotalCost:       750.0,
					NetContribution: -150.0,
				},
				TotalScore:         -150.0,
				PostDecisionRegion: "SOUTH",
				DeadheadMiles:      150.0,
				LoadedMiles:        150.0,
				IsAssigned:         false,
			},
		},
	}

	prior := map[string]float64{"AGGRESSIVE": 0.20, "MODERATE": 0.60, "PASSIVE": 0.20}
	posterior := map[string]float64{"AGGRESSIVE": 0.55, "MODERATE": 0.35, "PASSIVE": 0.10}

	exp, err := explainer.ExplainDecision(prov, prior, posterior)
	if err != nil {
		t.Fatalf("ExplainDecision failed: %v", err)
	}

	if exp.MatchedDriversCount != 1 {
		t.Errorf("expected 1 matched driver, got %d", exp.MatchedDriversCount)
	}
	if exp.IdleDriversCount != 1 {
		t.Errorf("expected 1 idle driver, got %d", exp.IdleDriversCount)
	}
	if exp.TotalDrivers != 2 {
		t.Errorf("expected 2 total drivers, got %d", exp.TotalDrivers)
	}

	// Verify Driver 1 Match Explanation
	m1 := exp.MatchedExplanations[0]
	if m1.DriverID != "D1" || m1.AssignedLoadID != "L-A" {
		t.Errorf("unexpected match: %s -> %s", m1.DriverID, m1.AssignedLoadID)
	}
	if m1.ImmediateNetMargin != 1800.0 {
		t.Errorf("expected immediate net margin 1800.0, got %f", m1.ImmediateNetMargin)
	}
	if len(m1.RejectedAlternatives) != 2 {
		t.Fatalf("expected 2 rejected alternatives for D1, got %d", len(m1.RejectedAlternatives))
	}

	// Verify Counterfactual ordering: L-B (Delta: 800) should come before L-C (Delta: 2100)
	alt0 := m1.RejectedAlternatives[0]
	if alt0.LoadID != "L-B" {
		t.Errorf("expected closest alternative L-B first, got %s", alt0.LoadID)
	}
	if alt0.ScoreDelta != 800.0 { // 1950 - 1150
		t.Errorf("expected score delta 800.0, got %f", alt0.ScoreDelta)
	}

	alt1 := m1.RejectedAlternatives[1]
	if alt1.LoadID != "L-C" {
		t.Errorf("expected alternative L-C second, got %s", alt1.LoadID)
	}
	if alt1.ScoreDelta != 2100.0 { // 1950 - (-150)
		t.Errorf("expected score delta 2100.0, got %f", alt1.ScoreDelta)
	}

	// Verify Driver 2 Idle Explanation
	idle2 := exp.IdleExplanations[0]
	if idle2.DriverID != "D2" {
		t.Errorf("expected idle driver D2, got %s", idle2.DriverID)
	}
	if idle2.ReasonCode != "NEGATIVE_EXPECTED_MARGIN" {
		t.Errorf("expected reason NEGATIVE_EXPECTED_MARGIN, got %s", idle2.ReasonCode)
	}

	// Verify Belief Shift
	if exp.BeliefShift == nil {
		t.Fatal("expected non-nil belief shift explanation")
	}
	if exp.BeliefShift.DominantPosture != "AGGRESSIVE" {
		t.Errorf("expected dominant posture AGGRESSIVE, got %s", exp.BeliefShift.DominantPosture)
	}

	// Verify Markdown rendering
	md := formatter.FormatMarkdown(exp)
	if !strings.Contains(md, "OPT-RUN-1001") {
		t.Errorf("expected markdown to contain run ID")
	}
	if !strings.Contains(md, "L-A") || !strings.Contains(md, "L-B") {
		t.Errorf("expected markdown to contain load IDs")
	}
	if !strings.Contains(md, "NEGATIVE_EXPECTED_MARGIN") {
		t.Errorf("expected markdown to contain idle reason")
	}
}

func TestExplainer_ConcurrentSafety(t *testing.T) {
	explainer := explain.NewExplainer()
	formatter := explain.NewFormatter()

	prov := policy.DecisionProvenance{
		OptimizationRunID: "OPT-CONC-01",
		BatchEpoch:        1700000000,
		PolicyName:        "CFA",
		EvaluatedArcs: []policy.CandidateEvaluation{
			{
				DriverID: "D1",
				LoadID:   "L1",
				CostBreakdown: policy.TripCostBreakdown{
					Revenue:         1500.0,
					LoadedCost:      500.0,
					NetContribution: 1000.0,
				},
				TotalScore: 1000.0,
				IsAssigned: true,
			},
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			exp, err := explainer.ExplainDecision(prov, nil, nil)
			if err != nil {
				t.Errorf("concurrent ExplainDecision error: %v", err)
				return
			}
			md := formatter.FormatMarkdown(exp)
			if len(md) == 0 {
				t.Errorf("empty markdown generated")
			}
		}()
	}
	wg.Wait()
}
