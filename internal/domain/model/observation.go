package model

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

var (
	// ErrInvalidObservation is returned when an observation contains non-physical or negative quantities.
	ErrInvalidObservation = errors.New("domain/model: invalid observation data")
)

// BidFeedback represents an endogenous market observation signal Y_{t+1} indicating
// whether a submitted spot pricing bid won or lost the contracted shipment.
type BidFeedback struct {
	LoadID       string
	Won          bool    // True if carrier was awarded the load at the bid price
	WinningPrice float64 // The clearing market price of the shipment
}

// Observation models the generalized exogenous transaction feedback W_{t+1} = (D_{t+1}, Y_{t+1})
// received by the carrier at decision epoch t+1 (HLD Section 6.3).
//
// In accordance with MOMDP factoring:
//   - D_{t+1} represents realized customer load offers arriving from the market.
//   - Y_{t+1} represents endogenous feedback on pricing bids submitted at epoch t.
//
// In accordance with Inviolate 5 (State Immutability), Observation is immutable once constructed.
type Observation struct {
	epoch    int64
	loads    []Load
	feedback []BidFeedback
}

// NewObservation creates an immutable Observation with canonically sorted loads and bid feedback.
func NewObservation(epoch int64, loads []Load, feedback []BidFeedback) *Observation {
	copiedLoads := make([]Load, len(loads))
	copy(copiedLoads, loads)
	sort.Slice(copiedLoads, func(i, j int) bool {
		return copiedLoads[i].ID < copiedLoads[j].ID
	})

	copiedFeedback := make([]BidFeedback, len(feedback))
	copy(copiedFeedback, feedback)
	sort.Slice(copiedFeedback, func(i, j int) bool {
		return copiedFeedback[i].LoadID < copiedFeedback[j].LoadID
	})

	return &Observation{
		epoch:    epoch,
		loads:    copiedLoads,
		feedback: copiedFeedback,
	}
}

// Epoch returns the observation arrival epoch index.
func (o *Observation) Epoch() int64 {
	return o.epoch
}

// Loads returns a deep copy of newly arrived customer load requests (D_{t+1}).
func (o *Observation) Loads() []Load {
	out := make([]Load, len(o.loads))
	copy(out, o.loads)
	return out
}

// Feedback returns a deep copy of bid win/loss feedback signals (Y_{t+1}).
func (o *Observation) Feedback() []BidFeedback {
	out := make([]BidFeedback, len(o.feedback))
	copy(out, o.feedback)
	return out
}

// LoadCount returns the number of newly realized customer loads in this observation.
func (o *Observation) LoadCount() int {
	return len(o.loads)
}

// FeedbackCount returns the number of bid win/loss outcomes reported in this observation.
func (o *Observation) FeedbackCount() int {
	return len(o.feedback)
}

// WonCount returns the number of winning bids reported in this observation.
func (o *Observation) WonCount() int {
	won := 0
	for _, fb := range o.feedback {
		if fb.Won {
			won++
		}
	}
	return won
}

// PostureObservationProfile specifies the expected likelihood distribution parameters
// for a specific latent competitive posture \Theta_k.
type PostureObservationProfile struct {
	// ExpectedWinProbability is the binomial win probability p_k on submitted spot bids (0.0 < p_k < 1.0).
	ExpectedWinProbability float64
	// ExpectedSpotRateMean is the Gaussian mean market rate index under posture \Theta_k.
	ExpectedSpotRateMean float64
	// ExpectedSpotRateStdDev is the standard deviation of market rates under posture \Theta_k.
	ExpectedSpotRateStdDev float64
	// ExpectedOffersMean is the Poisson mean arrival count of load offers under posture \Theta_k.
	ExpectedOffersMean float64
}

// MarketObservationModel evaluates log-likelihoods \ln P(o_{t+1} \mid \Theta = stateKey)
// across configured competitor postures.
//
// In accordance with Inviolate 5 (State Immutability), the observation model is immutable once constructed.
type MarketObservationModel struct {
	profiles map[string]PostureObservationProfile
}

// NewMarketObservationModel constructs an observation model mapping latent state keys to likelihood profiles.
func NewMarketObservationModel(profiles map[string]PostureObservationProfile) (*MarketObservationModel, error) {
	if len(profiles) == 0 {
		return nil, errors.New("domain/model: observation model requires at least one posture profile")
	}

	copied := make(map[string]PostureObservationProfile, len(profiles))
	for k, prof := range profiles {
		if prof.ExpectedWinProbability <= 0.0 || prof.ExpectedWinProbability >= 1.0 {
			return nil, fmt.Errorf("domain/model: profile %s ExpectedWinProbability must be in (0, 1), got %f",
				k, prof.ExpectedWinProbability)
		}
		if prof.ExpectedSpotRateStdDev <= 0.0 {
			return nil, fmt.Errorf("domain/model: profile %s ExpectedSpotRateStdDev must be positive, got %f",
				k, prof.ExpectedSpotRateStdDev)
		}
		if prof.ExpectedOffersMean <= 0.0 {
			return nil, fmt.Errorf("domain/model: profile %s ExpectedOffersMean must be positive, got %f",
				k, prof.ExpectedOffersMean)
		}
		copied[k] = prof
	}

	return &MarketObservationModel{profiles: copied}, nil
}

// LogLikelihood computes the exact log-likelihood \ln P(o_{t+1} \mid \Theta_k) for the given observation.
//
// Log-space computation prevents floating-point underflow during multi-epoch product accumulation.
func (m *MarketObservationModel) LogLikelihood(obs *Observation, stateKey string) (float64, error) {
	if obs == nil {
		return 0.0, errors.New("domain/model: observation cannot be nil")
	}

	prof, ok := m.profiles[stateKey]
	if !ok {
		return 0.0, fmt.Errorf("domain/model: unknown latent state key %s in observation model", stateKey)
	}

	// 1. Binomial Log-Likelihood for spot bid win/loss outcomes Y_{t+1}:
	// \ln P(k wins out of n bids) = \ln \binom{n}{k} + k \ln(p) + (n - k) \ln(1 - p)
	logBinomial := 0.0
	n := obs.FeedbackCount()
	k := obs.WonCount()
	if n > 0 {
		p := prof.ExpectedWinProbability
		logComb := logFactorial(n) - logFactorial(k) - logFactorial(n-k)
		logBinomial = logComb + float64(k)*math.Log(p) + float64(n-k)*math.Log(1.0-p)
	}

	// 2. Gaussian Log-Likelihood for realized market clearing spot rate:
	// Computed from winning prices in feedback or load revenues
	spotRate := prof.ExpectedSpotRateMean
	if n > 0 {
		totalPrice := 0.0
		for _, fb := range obs.feedback {
			totalPrice += fb.WinningPrice
		}
		spotRate = totalPrice / float64(n)
	} else if obs.LoadCount() > 0 {
		totalRev := 0.0
		totalMiles := 0.0
		for _, l := range obs.loads {
			totalRev += l.Revenue
			miles := l.Origin.DistanceMiles(l.Destination)
			totalMiles += miles
		}
		if totalMiles > 0 {
			spotRate = totalRev / totalMiles
		}
	}

	diff := spotRate - prof.ExpectedSpotRateMean
	variance := prof.ExpectedSpotRateStdDev * prof.ExpectedSpotRateStdDev
	logGaussianRate := -0.5*math.Log(2*math.Pi) - math.Log(prof.ExpectedSpotRateStdDev) - (diff*diff)/(2.0*variance)

	// 3. Poisson Log-Likelihood for realized customer load offer volume D_{t+1}:
	// \ln \text{Pois}(x; \lambda) = x \ln(\lambda) - \lambda - \ln(x!)
	x := obs.LoadCount()
	lambda := prof.ExpectedOffersMean
	logPoissonOffers := float64(x)*math.Log(lambda) - lambda - logFactorial(x)

	totalLogLikelihood := logBinomial + logGaussianRate + logPoissonOffers
	return totalLogLikelihood, nil
}

// logFactorial returns \ln(n!) using exact computation for n <= 20
// and Ramanujan's high-order asymptotic approximation for n > 20.
func logFactorial(n int) float64 {
	if n <= 1 {
		return 0.0
	}
	if n <= 20 {
		fact := 1.0
		for i := 2; i <= n; i++ {
			fact *= float64(i)
		}
		return math.Log(fact)
	}
	// Ramanujan's Stirling expansion: \ln(n!) \approx n \ln(n) - n + 0.5 \ln(2\pi n) + 1/(12n) - 1/(360n^3)
	fn := float64(n)
	return fn*math.Log(fn) - fn + 0.5*math.Log(2*math.Pi*fn) + 1.0/(12.0*fn) - 1.0/(360.0*fn*fn*fn)
}
