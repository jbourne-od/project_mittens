package policy_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

func TestExactLAP_AssignmentParadoxResolution(t *testing.T) {
	// Setup the classic Assignment Paradox:
	// - Driver 1 can service Load A (Score $1,000) or Load B (Score $900).
	// - Driver 2 can ONLY service Load A (Score $950), Load B is infeasible.
	//
	// Greedy Assignment:
	//   1. Driver 1 -> Load A (Score 1000)
	//   2. Driver 2 has no feasible load left.
	//   Total = $1,000 (1 match)
	//
	// Exact LAP Assignment:
	//   1. Driver 1 -> Load B (Score 900)
	//   2. Driver 2 -> Load A (Score 950)
	//   Total = $1,850 (2 matches, +$850 global net improvement!)

	epoch := time.Now().Unix()

	evals := []policy.CandidateEvaluation{
		{
			DriverID:   "D-01",
			LoadID:     "L-A",
			TotalScore: 1000.0,
			CostBreakdown: policy.TripCostBreakdown{
				NetContribution: 1000.0,
			},
		},
		{
			DriverID:   "D-02",
			LoadID:     "L-A",
			TotalScore: 950.0,
			CostBreakdown: policy.TripCostBreakdown{
				NetContribution: 950.0,
			},
		},
		{
			DriverID:   "D-01",
			LoadID:     "L-B",
			TotalScore: 900.0,
			CostBreakdown: policy.TripCostBreakdown{
				NetContribution: 900.0,
			},
		},
	}

	// 1. Verify Greedy Matcher yields suboptimal $1,000
	greedyMatcher := policy.NewBipartiteMatcherWithAlgorithm(policy.AlgorithmGreedy)
	greedyMatches, _, greedyScore, _ := greedyMatcher.SolveMatching(evals, epoch, false)

	if len(greedyMatches) != 1 || greedyScore != 1000.0 {
		t.Fatalf("expected greedy matcher to pick 1 match with score 1000.0, got %d matches (score %.2f)",
			len(greedyMatches), greedyScore)
	}
	if greedyMatches[0].DriverID != "D-01" || greedyMatches[0].LoadID != "L-A" {
		t.Errorf("expected greedy to match D-01 -> L-A, got %v", greedyMatches[0])
	}

	// 2. Verify Exact LAP Matcher resolves paradox and yields optimal $1,850
	lapMatcher := policy.NewBipartiteMatcherWithAlgorithm(policy.AlgorithmExactLAP)
	lapMatches, _, lapScore, lapNetContrib := lapMatcher.SolveMatching(evals, epoch, false)

	if len(lapMatches) != 2 || lapScore != 1850.0 {
		t.Fatalf("expected exact LAP matcher to pick 2 matches with score 1850.0, got %d matches (score %.2f)",
			len(lapMatches), lapScore)
	}
	if lapNetContrib != 1850.0 {
		t.Errorf("expected net contribution 1850.0, got %.2f", lapNetContrib)
	}

	// Verify exact pairings: D-01 -> L-B, D-02 -> L-A
	assignmentMap := make(map[string]string)
	for _, m := range lapMatches {
		assignmentMap[m.DriverID] = m.LoadID
	}

	if assignmentMap["D-01"] != "L-B" {
		t.Errorf("expected D-01 -> L-B, got %s", assignmentMap["D-01"])
	}
	if assignmentMap["D-02"] != "L-A" {
		t.Errorf("expected D-02 -> L-A, got %s", assignmentMap["D-02"])
	}

	t.Logf("Assignment Paradox successfully resolved: Greedy = $%.2f (1 match) -> Exact LAP = $%.2f (2 matches, +$%.2f global surplus)",
		greedyScore, lapScore, lapScore-greedyScore)
}

func TestExactLAP_DenseAsymmetricScaling(t *testing.T) {
	epoch := time.Now().Unix()

	// 10 Drivers vs 50 Loads with random dense scores
	numDrivers := 10
	numLoads := 50

	var evals []policy.CandidateEvaluation
	for d := 0; d < numDrivers; d++ {
		dID := fmt.Sprintf("D-%02d", d)
		for l := 0; l < numLoads; l++ {
			lID := fmt.Sprintf("L-%02d", l)
			// Score formula creating competing overlaps
			score := float64(500 + ((d+1)*17+(l+1)*23)%600)
			evals = append(evals, policy.CandidateEvaluation{
				DriverID:   dID,
				LoadID:     lID,
				TotalScore: score,
				CostBreakdown: policy.TripCostBreakdown{
					NetContribution: score,
				},
			})
		}
	}

	greedyMatcher := policy.NewBipartiteMatcherWithAlgorithm(policy.AlgorithmGreedy)
	_, _, greedyScore, _ := greedyMatcher.SolveMatching(evals, epoch, false)

	lapMatcher := policy.NewBipartiteMatcherWithAlgorithm(policy.AlgorithmExactLAP)
	lapMatches, _, lapScore, _ := lapMatcher.SolveMatching(evals, epoch, false)

	if len(lapMatches) != numDrivers {
		t.Fatalf("expected %d matches, got %d", numDrivers, len(lapMatches))
	}

	if lapScore < greedyScore {
		t.Errorf("exact LAP score %.2f cannot be less than greedy score %.2f", lapScore, greedyScore)
	}

	t.Logf("Dense Asymmetric Matching: Greedy = $%.2f, Exact LAP = $%.2f (delta: +$%.2f)",
		greedyScore, lapScore, lapScore-greedyScore)
}

func TestExactLAP_DeterministicTieBreaking(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	// 5 identical drivers vs 5 identical loads
	drivers := make([]model.Driver, 5)
	for i := 0; i < 5; i++ {
		drivers[i] = model.Driver{
			ID:              fmt.Sprintf("D-%02d", i+1),
			CurrentLocation: locChi,
			HomeLocation:    locChi,
			AvailableEpoch:  startEpoch,
			Equipment:       model.Equipment{Type: model.EquipDryVan},
		}
	}

	loads := make([]model.Load, 5)
	for i := 0; i < 5; i++ {
		loads[i] = model.Load{
			ID:                  fmt.Sprintf("L-%02d", i+1),
			Origin:              locChi,
			Destination:         locAtl,
			Revenue:             3000.0,
			RequiredEquipment:   model.EquipDryVan,
			PickupEarliestEpoch: startEpoch,
			PickupLatestEpoch:   startEpoch + 3600,
			DeliveryLatestEpoch: startEpoch + 48*3600,
		}
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(startEpoch, 1.0, 2.50, 0)
	state, _ := model.NewState(res, info, model.NewMonopolisticBelief())

	cfa := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	// Run 30 times and verify bit-wise identical match tuples
	var firstRunMatches []model.DriverLoadMatch
	for run := 0; run < 30; run++ {
		action, _, err := cfa.Evaluate(context.Background(), state)
		if err != nil {
			t.Fatalf("Run %d failed: %v", run, err)
		}

		matches := action.Matches()
		if run == 0 {
			firstRunMatches = matches
			if len(matches) != 5 {
				t.Fatalf("expected 5 matches, got %d", len(matches))
			}
		} else {
			if len(matches) != len(firstRunMatches) {
				t.Fatalf("Run %d match count %d != %d", run, len(matches), len(firstRunMatches))
			}
			for idx := range matches {
				if matches[idx] != firstRunMatches[idx] {
					t.Fatalf("Run %d determinism broken at index %d: %v vs %v",
						run, idx, matches[idx], firstRunMatches[idx])
				}
			}
		}
	}
}
