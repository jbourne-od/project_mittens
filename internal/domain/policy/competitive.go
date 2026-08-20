// Package policy implements the universal classes of Powell policies
// (PFAs, CFAs, VFAs, and DLAs) over the factored MOMDP state space.
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /pkg/telemetry, /pkg/logging
//   - Imported By: /internal/service, /internal/adapter
//   - Strict Rule: Pure execution, zero I/O, zero global mutable state.
package policy

import (
	"context"
	"fmt"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// CompetitivePricingConfig specifies explicit parameters for endogenous spot price formulation
// under competitor capacity belief states (Inviolate 0).
type CompetitivePricingConfig struct {
	// BaselineRatePerMile is the nominal spot bid rate ($/loaded mile) in balanced or tight markets.
	BaselineRatePerMile float64
	// SurplusRatePerMile is the elevated spot bid rate ($/loaded mile) in loose/passive competitor markets.
	SurplusRatePerMile float64
	// BaseMarginHurdle is the minimum baseline dollar margin above total trip cost ($).
	BaseMarginHurdle float64
	// AggressiveRiskHurdle is the additional margin hurdle under aggressive competitor concentration ($).
	AggressiveRiskHurdle float64
	// PassiveSurplusThreshold is the probability threshold b_t(PASSIVE) required to trigger surplus pricing.
	PassiveSurplusThreshold float64
}

// DefaultCompetitivePricingConfig returns calibrated market defaults for dynamic spot rate bidding.
func DefaultCompetitivePricingConfig() CompetitivePricingConfig {
	return CompetitivePricingConfig{
		BaselineRatePerMile:     2.52,
		SurplusRatePerMile:      2.92,
		BaseMarginHurdle:        75.0,
		AggressiveRiskHurdle:    125.0,
		PassiveSurplusThreshold: 0.65,
	}
}

// Validate verifies that pricing parameters are positive and mathematically consistent (Inviolate 8).
func (c CompetitivePricingConfig) Validate() error {
	if c.BaselineRatePerMile <= 0 {
		return fmt.Errorf("policy: baseline rate per mile must be positive, got %.2f", c.BaselineRatePerMile)
	}
	if c.SurplusRatePerMile < c.BaselineRatePerMile {
		return fmt.Errorf("policy: surplus rate (%.2f) must be >= baseline rate (%.2f)", c.SurplusRatePerMile, c.BaselineRatePerMile)
	}
	if c.BaseMarginHurdle < 0 {
		return fmt.Errorf("policy: base margin hurdle must be non-negative, got %.2f", c.BaseMarginHurdle)
	}
	if c.AggressiveRiskHurdle < 0 {
		return fmt.Errorf("policy: aggressive risk hurdle must be non-negative, got %.2f", c.AggressiveRiskHurdle)
	}
	if c.PassiveSurplusThreshold <= 0 || c.PassiveSurplusThreshold > 1.0 {
		return fmt.Errorf("policy: passive surplus threshold must be in (0, 1], got %.2f", c.PassiveSurplusThreshold)
	}
	return nil
}

// CompetitivePOMDPPolicy combines a primary matching policy (such as CFA or VFA) with
// an endogenous, belief-dependent spot rate pricing policy over the MOMDP state S_t = (R_t, I_t, b_t).
//
// In accordance with Inviolate 4 (Single Closed Source of Business Logic) and Inviolate 3 (Competitive Genericity),
// dynamic rate formulation and competitor opportunity cost hurdles are strictly encapsulated within this typed policy.
type CompetitivePOMDPPolicy[C model.CompetitorScale] struct {
	underlying Policy[C]
	pricingCfg CompetitivePricingConfig
}

// NewCompetitivePOMDPPolicy initializes a new CompetitivePOMDPPolicy instance.
func NewCompetitivePOMDPPolicy[C model.CompetitorScale](
	underlying Policy[C],
	pricingCfg CompetitivePricingConfig,
) (*CompetitivePOMDPPolicy[C], error) {
	if underlying == nil {
		return nil, fmt.Errorf("policy: underlying matching policy cannot be nil")
	}
	if err := pricingCfg.Validate(); err != nil {
		return nil, fmt.Errorf("policy: invalid competitive pricing config: %w", err)
	}
	return &CompetitivePOMDPPolicy[C]{
		underlying: underlying,
		pricingCfg: pricingCfg,
	}, nil
}

// Name returns the descriptive name of the combined policy.
func (p *CompetitivePOMDPPolicy[C]) Name() string {
	return fmt.Sprintf("CompetitivePOMDP_%s", p.underlying.Name())
}

// PricingConfig returns the active pricing configuration.
func (p *CompetitivePOMDPPolicy[C]) PricingConfig() CompetitivePricingConfig {
	return p.pricingCfg
}

// Evaluate solves the matching decision under the underlying policy and formulates dynamic spot pricing bids
// using the active competitive belief vector b_t(Theta_t).
func (p *CompetitivePOMDPPolicy[C]) Evaluate(
	ctx context.Context,
	state *model.State[C],
) (*model.Action, DecisionProvenance, error) {
	if state == nil {
		return nil, DecisionProvenance{}, fmt.Errorf("policy: cannot evaluate nil state")
	}

	ctx, span := telemetry.StartSpan(ctx, "Policy.CompetitivePOMDP.Evaluate")
	defer span.End()

	// 1. Solve physical fleet-to-load matching via underlying policy
	baseAction, prov, err := p.underlying.Evaluate(ctx, state)
	if err != nil {
		return nil, DecisionProvenance{}, fmt.Errorf("policy: underlying evaluate failed: %w", err)
	}

	// 2. Evaluate dynamic spot bids under competitive belief state b_t
	bState := state.Belief()
	var aggressiveProb float64
	var passiveProb float64
	if bState != nil {
		aggressiveProb = bState.Probability("AGGRESSIVE")
		passiveProb = bState.Probability("PASSIVE")
	}

	// Risk-aware opportunity cost hurdle:
	// In aggressive markets: require higher hurdle margin to prevent committing fleet to low-margin freight.
	minHurdle := p.pricingCfg.BaseMarginHurdle + p.pricingCfg.AggressiveRiskHurdle*aggressiveProb

	// Rate per loaded mile selection:
	targetRatePerMile := p.pricingCfg.BaselineRatePerMile
	if passiveProb >= p.pricingCfg.PassiveSurplusThreshold {
		targetRatePerMile = p.pricingCfg.SurplusRatePerMile
	}

	// Map total cost per assigned load from evaluated candidate arcs
	costByLoadID := make(map[string]float64, len(prov.EvaluatedArcs))
	for _, arc := range prov.EvaluatedArcs {
		if arc.IsAssigned {
			costByLoadID[arc.LoadID] = arc.CostBreakdown.TotalCost
		}
	}

	// Formulate spot pricing bids for all matched loads
	matches := baseAction.Matches()
	spotBids := make([]model.SpotPriceBid, 0, len(matches))
	for _, m := range matches {
		load, found := state.Resource().GetLoad(m.LoadID)
		if !found {
			continue
		}
		miles := load.Origin.DistanceMiles(load.Destination)
		bidPrice := miles * targetRatePerMile

		// Cost floor protection: never bid below trip cost + minimum hurdle margin
		costFloor := costByLoadID[m.LoadID] + minHurdle
		if bidPrice < costFloor {
			bidPrice = costFloor
		}

		spotBids = append(spotBids, model.SpotPriceBid{
			LoadID:   m.LoadID,
			BidPrice: bidPrice,
		})
	}

	span.SetAttributes(
		attribute.Int("policy.matches_count", len(matches)),
		attribute.Int("policy.spot_bids_count", len(spotBids)),
		attribute.Float64("policy.min_hurdle", minHurdle),
		attribute.Float64("policy.target_rate_per_mile", targetRatePerMile),
	)

	// Combine matches and spot bids into final Action (Inviolate 5: fresh allocation)
	action := model.NewAction(matches, spotBids)
	return action, prov, nil
}
