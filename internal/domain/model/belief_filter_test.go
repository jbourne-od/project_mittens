package model_test

import (
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

// makeObservation creates a synthetic test observation matching specific volume, bid win rate, and spot rate metrics.
func makeObservation(epoch int64, loadCount int, totalBids int, wonBids int, spotPrice float64) *model.Observation {
	loads := make([]model.Load, loadCount)
	for i := 0; i < loadCount; i++ {
		loads[i] = model.Load{
			ID:      fmt.Sprintf("L-%04d", i),
			Revenue: spotPrice * 500.0, // Assuming 500-mile average haul
		}
	}

	feedback := make([]model.BidFeedback, totalBids)
	for i := 0; i < totalBids; i++ {
		feedback[i] = model.BidFeedback{
			LoadID:       fmt.Sprintf("BID-%04d", i),
			Won:          i < wonBids,
			WinningPrice: spotPrice,
		}
	}

	return model.NewObservation(epoch, loads, feedback)
}

func TestBeliefFilter_MonopolisticDegeneracy(t *testing.T) {
	filter := model.NewMonopolisticFilter()
	prior := model.NewMonopolisticBelief()

	obs := makeObservation(1, 100, 20, 10, 2.50)

	post, err := filter.Filter(prior, obs, nil)
	if err != nil {
		t.Fatalf("Monopolistic Filter failed: %v", err)
	}

	// Must collapse to Dirac delta
	if post.Dimension() != 1 {
		t.Errorf("expected dimension 1 for monopolistic belief, got %d", post.Dimension())
	}
	if post.Probability(model.MonopolisticSingletonKey) != 1.0 {
		t.Errorf("expected probability 1.0 on singleton key, got %f",
			post.Probability(model.MonopolisticSingletonKey))
	}
}

func TestBeliefFilter_BayesianPosteriorConvergence(t *testing.T) {
	// Define 3 latent competitor postures
	states := []string{"Aggressive", "Moderate", "CapacityConstrained"}

	// Persistent transition dynamics (Markov state persists with 90% probability)
	transitionData := [][]float64{
		{0.90, 0.05, 0.05}, // Aggressive -> [0.90, 0.05, 0.05]
		{0.05, 0.90, 0.05}, // Moderate -> [0.05, 0.90, 0.05]
		{0.05, 0.05, 0.90}, // CapacityConstrained -> [0.05, 0.05, 0.90]
	}

	tm, err := model.NewTransitionMatrix(states, transitionData)
	if err != nil {
		t.Fatalf("NewTransitionMatrix failed: %v", err)
	}

	// Distinct observation profiles
	profiles := map[string]model.PostureObservationProfile{
		"Aggressive": {
			ExpectedWinProbability: 0.15,
			ExpectedSpotRateMean:   1.80,
			ExpectedSpotRateStdDev: 0.20,
			ExpectedOffersMean:     50.0,
		},
		"Moderate": {
			ExpectedWinProbability: 0.50,
			ExpectedSpotRateMean:   2.50,
			ExpectedSpotRateStdDev: 0.20,
			ExpectedOffersMean:     100.0,
		},
		"CapacityConstrained": {
			ExpectedWinProbability: 0.85,
			ExpectedSpotRateMean:   3.20,
			ExpectedSpotRateStdDev: 0.20,
			ExpectedOffersMean:     150.0,
		},
	}

	om, err := model.NewMarketObservationModel(profiles)
	if err != nil {
		t.Fatalf("NewMarketObservationModel failed: %v", err)
	}

	scale := model.AggregatedMarket{LatentStates: states}
	filter, err := model.NewCompetitiveBeliefFilter(scale, tm, om)
	if err != nil {
		t.Fatalf("NewCompetitiveBeliefFilter failed: %v", err)
	}

	// Initialize with uniform prior: [1/3, 1/3, 1/3]
	prior, err := model.NewBelief(scale, states, []float64{1.0 / 3.0, 1.0 / 3.0, 1.0 / 3.0})
	if err != nil {
		t.Fatalf("NewBelief failed: %v", err)
	}

	// Simulate ground truth: True market state is "Aggressive"
	// Feed 5 consecutive observations matching "Aggressive" profile (low win rate 3/20 = 15%, rate $1.82, 52 offers)
	currentBelief := prior
	for epoch := 1; epoch <= 5; epoch++ {
		obs := makeObservation(int64(epoch), 52, 20, 3, 1.82)

		nextBelief, err := filter.Filter(currentBelief, obs, nil)
		if err != nil {
			t.Fatalf("Epoch %d Filter failed: %v", epoch, err)
		}

		t.Logf("Epoch %d: Belief = [Aggressive: %.4f, Moderate: %.4f, CapacityConstrained: %.4f]",
			epoch,
			nextBelief.Probability("Aggressive"),
			nextBelief.Probability("Moderate"),
			nextBelief.Probability("CapacityConstrained"),
		)

		currentBelief = nextBelief
	}

	// Verify belief posterior converged strongly to "Aggressive" (> 99.5%)
	pAggressive := currentBelief.Probability("Aggressive")
	if pAggressive < 0.995 {
		t.Errorf("expected belief on Aggressive to converge > 0.995, got %f", pAggressive)
	}
}

func TestBeliefFilter_10000StepSimplexStability(t *testing.T) {
	states := []string{"Low", "Mid", "High"}
	tm := model.NewIdentityTransitionMatrix(states)

	profiles := map[string]model.PostureObservationProfile{
		"Low":  {ExpectedWinProbability: 0.2, ExpectedSpotRateMean: 1.5, ExpectedSpotRateStdDev: 0.3, ExpectedOffersMean: 30},
		"Mid":  {ExpectedWinProbability: 0.5, ExpectedSpotRateMean: 2.5, ExpectedSpotRateStdDev: 0.3, ExpectedOffersMean: 60},
		"High": {ExpectedWinProbability: 0.8, ExpectedSpotRateMean: 3.5, ExpectedSpotRateStdDev: 0.3, ExpectedOffersMean: 90},
	}
	om, _ := model.NewMarketObservationModel(profiles)
	scale := model.AggregatedMarket{LatentStates: states}
	filter, _ := model.NewCompetitiveBeliefFilter(scale, tm, om)

	current, _ := model.NewBelief(scale, states, []float64{1.0 / 3.0, 1.0 / 3.0, 1.0 / 3.0})
	rng := pkgmath.NewRNG(777)

	for step := 0; step < 10000; step++ {
		loadCount := 40 + rng.Intn(40)
		totalBids := 10
		wonBids := rng.Intn(11)
		spotRate := 1.5 + 2.0*rng.Float64()

		obs := makeObservation(int64(step), loadCount, totalBids, wonBids, spotRate)

		next, err := filter.Filter(current, obs, nil)
		if err != nil {
			t.Fatalf("Step %d Filter failed: %v", step, err)
		}

		// Check simplex tolerance (Inviolate 8)
		sum := pkgmath.CompensatedSum(next.Probabilities())
		if math.Abs(sum-1.0) > 1e-11 {
			t.Fatalf("Step %d: Belief simplex drift exceeded 1e-11: sum = %.16f", step, sum)
		}

		current = next
	}
}

func TestBeliefFilter_ParentStateImmutability(t *testing.T) {
	states := []string{"StateA", "StateB"}
	tm := model.NewIdentityTransitionMatrix(states)
	profiles := map[string]model.PostureObservationProfile{
		"StateA": {ExpectedWinProbability: 0.3, ExpectedSpotRateMean: 2.0, ExpectedSpotRateStdDev: 0.5, ExpectedOffersMean: 50},
		"StateB": {ExpectedWinProbability: 0.7, ExpectedSpotRateMean: 3.0, ExpectedSpotRateStdDev: 0.5, ExpectedOffersMean: 100},
	}
	om, _ := model.NewMarketObservationModel(profiles)
	scale := model.AggregatedMarket{LatentStates: states}
	filter, _ := model.NewCompetitiveBeliefFilter(scale, tm, om)

	prior, _ := model.NewBelief(scale, states, []float64{0.5, 0.5})

	obs := makeObservation(1, 95, 20, 15, 3.10)

	post, err := filter.Filter(prior, obs, nil)
	if err != nil {
		t.Fatalf("Filter failed: %v", err)
	}

	// Prior must remain exactly [0.5, 0.5]
	if prior.Probability("StateA") != 0.5 || prior.Probability("StateB") != 0.5 {
		t.Errorf("Parent prior belief was mutated: StateA=%f, StateB=%f",
			prior.Probability("StateA"), prior.Probability("StateB"))
	}

	// Posterior must reflect measurement update
	if post.Probability("StateB") <= 0.5 {
		t.Errorf("Posterior on StateB expected to increase > 0.5, got %f", post.Probability("StateB"))
	}
}

func TestBeliefFilter_ConcurrentRaceDetector(t *testing.T) {
	states := []string{"PostA", "PostB", "PostC"}
	tm := model.NewIdentityTransitionMatrix(states)
	profiles := map[string]model.PostureObservationProfile{
		"PostA": {ExpectedWinProbability: 0.3, ExpectedSpotRateMean: 2.0, ExpectedSpotRateStdDev: 0.5, ExpectedOffersMean: 50},
		"PostB": {ExpectedWinProbability: 0.5, ExpectedSpotRateMean: 2.5, ExpectedSpotRateStdDev: 0.5, ExpectedOffersMean: 75},
		"PostC": {ExpectedWinProbability: 0.7, ExpectedSpotRateMean: 3.0, ExpectedSpotRateStdDev: 0.5, ExpectedOffersMean: 100},
	}
	om, _ := model.NewMarketObservationModel(profiles)
	scale := model.AggregatedMarket{LatentStates: states}
	filter, _ := model.NewCompetitiveBeliefFilter(scale, tm, om)

	prior, _ := model.NewBelief(scale, states, []float64{1.0 / 3.0, 1.0 / 3.0, 1.0 / 3.0})

	var wg sync.WaitGroup
	numWorkers := 32

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for iter := 0; iter < 100; iter++ {
				loadCount := 50 + (workerID+iter)%50
				totalBids := 10
				wonBids := (workerID + iter) % 11
				spotRate := 2.0 + float64(iter%10)*0.1

				obs := makeObservation(int64(iter), loadCount, totalBids, wonBids, spotRate)

				next, err := filter.Filter(prior, obs, nil)
				if err != nil {
					t.Errorf("Worker %d Filter failed: %v", workerID, err)
					return
				}
				if next.Dimension() != 3 {
					t.Errorf("Worker %d unexpected dimension %d", workerID, next.Dimension())
				}
			}
		}(i)
	}

	wg.Wait()
}
