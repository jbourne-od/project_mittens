package simulation

import (
	"fmt"
	"math"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

// CompetitorPosture represents the latent competitive state of the market.
type CompetitorPosture int

const (
	PostureAggressive CompetitorPosture = iota // Aggressive pricing (85% of spot rate, high capacity)
	PostureModerate                            // Normal market pricing (100% of spot rate)
	PosturePassive                             // Passive pricing (118% of spot rate, scarce capacity)
)

func (p CompetitorPosture) String() string {
	switch p {
	case PostureAggressive:
		return "AGGRESSIVE"
	case PostureModerate:
		return "MODERATE"
	case PosturePassive:
		return "PASSIVE"
	default:
		return "UNKNOWN"
	}
}

// MarketConfig configures the ground-truth competitive market dynamics.
type MarketConfig struct {
	InitialPosture   CompetitorPosture
	TransitionProb   [][]float64 // 3x3 transition matrix P(\Theta_{t+1}^* | \Theta_t^*)
	PriceMultipliers []float64   // Mean pricing multiplier by posture [0.85, 1.00, 1.18]
	PriceNoiseStdDev float64     // Gaussian price noise std dev (e.g. 0.04)
	SpotRatePerMile  float64     // National average spot rate ($2.50/mi)
	FuelPricePerGal  float64     // National average diesel price ($3.85/gal)
}

// DefaultMarketConfig returns realistic freight market competitive dynamics.
func DefaultMarketConfig() MarketConfig {
	return MarketConfig{
		InitialPosture: PostureModerate,
		// Sticky Markov chain: 70% persistence, 15% transition to neighbors
		TransitionProb: [][]float64{
			{0.70, 0.20, 0.10}, // Aggressive -> [Aggressive, Moderate, Passive]
			{0.15, 0.70, 0.15}, // Moderate   -> [Aggressive, Moderate, Passive]
			{0.10, 0.20, 0.70}, // Passive    -> [Aggressive, Moderate, Passive]
		},
		PriceMultipliers: []float64{0.86, 1.00, 1.18},
		PriceNoiseStdDev: 0.03,
		SpotRatePerMile:  2.50,
		FuelPricePerGal:  3.85,
	}
}

// MarketOutcome captures the physical resolution of a competitive auction round.
type MarketOutcome struct {
	Epoch               int64
	TrueCompetitorState CompetitorPosture
	CarrierBidLoads     []string           // Load IDs bid by carrier
	WonLoads            []model.Load       // Loads awarded to carrier
	LostLoads           []model.Load       // Loads won by competitor
	CarrierRevenues     map[string]float64 // Actual revenues earned on won loads
	CompetitorBids      map[string]float64 // Ground-truth competitor bids (all loads)
	ObservedFeedback    map[string]float64 // Censored feedback (carrier observed only on bid lanes)
}

// MarketEnvironment simulates the partially observable competitive freight environment.
type MarketEnvironment struct {
	cfg          MarketConfig
	rng          *pkgmath.RNG
	currentEpoch int64
	latentState  CompetitorPosture
}

// NewMarketEnvironment initializes a fresh market simulator instance.
func NewMarketEnvironment(cfg MarketConfig, seed uint64) (*MarketEnvironment, error) {
	if len(cfg.TransitionProb) != 3 || len(cfg.PriceMultipliers) != 3 {
		return nil, fmt.Errorf("market_env: invalid config dimensions (must be 3x3 for postures)")
	}
	return &MarketEnvironment{
		cfg:         cfg,
		rng:         pkgmath.NewRNG(seed),
		latentState: cfg.InitialPosture,
	}, nil
}

// LatentState returns the current true hidden competitor posture.
func (m *MarketEnvironment) LatentState() CompetitorPosture {
	return m.latentState
}

// Step advances the market by one discrete epoch:
//  1. Transitions latent competitor state \Theta_{t+1}^* \sim P(\cdot | \Theta_t^*).
//  2. Resolves carrier bids against competitor pricing.
//  3. Emits censored observation feedback (only on bid loads).
func (m *MarketEnvironment) Step(
	epoch int64,
	carrierAction *model.Action,
	availableLoads []model.Load,
) (MarketOutcome, *model.Observation, error) {
	m.currentEpoch = epoch

	// 1. Stochastic Latent Transition \Theta_{t+1}^*
	row := m.cfg.TransitionProb[int(m.latentState)]
	u := m.rng.Float64()
	cum := 0.0
	for nextState, prob := range row {
		cum += prob
		if u <= cum || nextState == len(row)-1 {
			m.latentState = CompetitorPosture(nextState)
			break
		}
	}

	// 2. Generate Competitor Bids for all available loads
	multiplierMean := m.cfg.PriceMultipliers[int(m.latentState)]
	competitorBids := make(map[string]float64, len(availableLoads))

	for _, load := range availableLoads {
		// Base benchmark load price from haul mileage
		mileage := load.Origin.DistanceMiles(load.Destination)
		basePrice := mileage * m.cfg.SpotRatePerMile
		if basePrice <= 0 {
			basePrice = load.Revenue
		}

		// Competitor bid: basePrice * (multiplierMean + noise)
		noise := (m.rng.Float64() - 0.5) * 2.0 * m.cfg.PriceNoiseStdDev
		compPrice := basePrice * math.Max(0.5, multiplierMean+noise)
		competitorBids[load.ID] = compPrice
	}

	// 3. Resolve Auction: Match carrier bids vs competitor bids
	matchedLoadsMap := make(map[string]bool)
	carrierBidsMap := make(map[string]float64)
	if carrierAction != nil {
		for _, match := range carrierAction.Matches() {
			matchedLoadsMap[match.LoadID] = true
		}
		for _, b := range carrierAction.Bids() {
			carrierBidsMap[b.LoadID] = b.BidPrice
		}
	}

	wonLoads := make([]model.Load, 0)
	lostLoads := make([]model.Load, 0)
	carrierRevenues := make(map[string]float64)
	observedFeedback := make(map[string]float64)
	bidLoadIDs := make([]string, 0)

	for _, load := range availableLoads {
		isBid := matchedLoadsMap[load.ID]
		if !isBid {
			continue
		}
		bidLoadIDs = append(bidLoadIDs, load.ID)

		// Carrier pricing: if explicit price submitted, use it; otherwise use listed load revenue
		carrierBid, hasCustomPrice := carrierBidsMap[load.ID]
		if !hasCustomPrice || carrierBid <= 0 {
			carrierBid = load.Revenue
		}

		compBid := competitorBids[load.ID]

		// Censored observation feedback: Carrier observes competitor bid on lane it bid on
		observedFeedback[load.ID] = compBid

		// First-price auction win condition: Carrier wins if its bid <= competitor bid
		// (or if carrier matches load under contract rate and competitor is underbid)
		if carrierBid <= compBid*1.02 { // 2% tie-breaking grace for incumbent carrier
			wonLoads = append(wonLoads, load)
			carrierRevenues[load.ID] = carrierBid
		} else {
			lostLoads = append(lostLoads, load)
		}
	}

	loadMap := make(map[string]model.Load, len(availableLoads))
	for _, l := range availableLoads {
		loadMap[l.ID] = l
	}

	feedbacks := make([]model.BidFeedback, 0, len(bidLoadIDs))
	for _, loadID := range bidLoadIDs {
		carrierBid := carrierBidsMap[loadID]
		compBid := competitorBids[loadID]
		load := loadMap[loadID]
		miles := load.Origin.DistanceMiles(load.Destination)
		ratePerMile := compBid
		if miles > 0 {
			ratePerMile = compBid / miles
		}
		feedbacks = append(feedbacks, model.BidFeedback{
			LoadID:       loadID,
			Won:          carrierBid <= compBid*1.02,
			WinningPrice: ratePerMile,
		})
	}

	outcome := MarketOutcome{
		Epoch:               epoch,
		TrueCompetitorState: m.latentState,
		CarrierBidLoads:     bidLoadIDs,
		WonLoads:            wonLoads,
		LostLoads:           lostLoads,
		CarrierRevenues:     carrierRevenues,
		CompetitorBids:      competitorBids,
		ObservedFeedback:    observedFeedback,
	}

	// 4. Construct Censored Observation for Belief Filter
	obs := model.NewObservation(epoch, availableLoads, feedbacks)

	return outcome, obs, nil
}
