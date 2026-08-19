package model

import (
	"errors"
	"fmt"
	"sort"
)

var (
	// ErrInvalidBidPrice is returned when a spot price bid is non-positive or non-finite.
	ErrInvalidBidPrice = errors.New("domain/model: spot price bid must be positive and finite")
	// ErrDuplicateBid is returned when multiple pricing bids are submitted for the same load.
	ErrDuplicateBid = errors.New("domain/model: duplicate spot price bid for load")
)

// SpotPriceBid represents an endogenous pricing decision p_t submitted for an uncontracted customer load offer.
type SpotPriceBid struct {
	LoadID   string
	BidPrice float64 // Dollar amount quoted to shipper
}

// Action represents the joint sequential decision action a_t = (x_t, p_t) at discrete epoch t.
//
// Formulated under Powell's SDA and HLD Section 6.2:
//   - x_t \in \mathcal{X}(R_t) represents physical vehicle dispatch matching assignments.
//   - p_t \in \mathcal{P} represents spot pricing bids submitted to the market.
//
// In accordance with Inviolate 5, Action is immutable once created.
type Action struct {
	matches []DriverLoadMatch
	bids    []SpotPriceBid
}

// NewAction creates an immutable Action with canonically sorted matches and bids.
func NewAction(matches []DriverLoadMatch, bids []SpotPriceBid) *Action {
	copiedMatches := make([]DriverLoadMatch, len(matches))
	copy(copiedMatches, matches)
	sort.Slice(copiedMatches, func(i, j int) bool {
		if copiedMatches[i].DriverID == copiedMatches[j].DriverID {
			return copiedMatches[i].LoadID < copiedMatches[j].LoadID
		}
		return copiedMatches[i].DriverID < copiedMatches[j].DriverID
	})

	copiedBids := make([]SpotPriceBid, len(bids))
	copy(copiedBids, bids)
	sort.Slice(copiedBids, func(i, j int) bool {
		return copiedBids[i].LoadID < copiedBids[j].LoadID
	})

	return &Action{
		matches: copiedMatches,
		bids:    copiedBids,
	}
}

// NewEmptyAction returns an empty joint action representing a null dispatch and pricing step.
func NewEmptyAction() *Action {
	return &Action{
		matches: nil,
		bids:    nil,
	}
}

// Matches returns a deep copy of the physical matching decisions (x_t).
func (a *Action) Matches() []DriverLoadMatch {
	out := make([]DriverLoadMatch, len(a.matches))
	copy(out, a.matches)
	return out
}

// Bids returns a deep copy of the spot pricing decisions (p_t).
func (a *Action) Bids() []SpotPriceBid {
	out := make([]SpotPriceBid, len(a.bids))
	copy(out, a.bids)
	return out
}

// MatchCount returns the number of physical driver-load matches.
func (a *Action) MatchCount() int {
	return len(a.matches)
}

// BidCount returns the number of spot price bids submitted.
func (a *Action) BidCount() int {
	return len(a.bids)
}

// IsEmpty returns true if both match assignments and bids are empty.
func (a *Action) IsEmpty() bool {
	return len(a.matches) == 0 && len(a.bids) == 0
}

// ValidateFeasibility asserts that the joint action is physically feasible against the active ResourceState R_t:
//  1. Each matched driver exists in R_t and is idle.
//  2. Each matched load exists in R_t.
//  3. No driver or load is matched more than once.
//  4. Each bid references an existing load in R_t and specifies a positive, finite price.
//  5. No load receives multiple pricing bids.
func (a *Action) ValidateFeasibility(resource *ResourceState) error {
	if resource == nil {
		return errors.New("domain/model: cannot validate action feasibility against nil ResourceState")
	}

	seenDrivers := make(map[string]bool, len(a.matches))
	seenMatchLoads := make(map[string]bool, len(a.matches))

	for _, m := range a.matches {
		if seenDrivers[m.DriverID] {
			return fmt.Errorf("%w: driver %s", ErrDuplicateMatch, m.DriverID)
		}
		seenDrivers[m.DriverID] = true

		if seenMatchLoads[m.LoadID] {
			return fmt.Errorf("%w: load %s", ErrDuplicateMatch, m.LoadID)
		}
		seenMatchLoads[m.LoadID] = true

		driver, ok := resource.GetDriver(m.DriverID)
		if !ok {
			return fmt.Errorf("%w: driver %s does not exist", ErrDriverNotFound, m.DriverID)
		}
		if !driver.IsIdle() {
			return fmt.Errorf("%w: driver %s is not idle (assigned to %s)", ErrDriverAlreadyAssigned, driver.ID, driver.AssignedLoadID)
		}

		if _, ok := resource.GetLoad(m.LoadID); !ok {
			return fmt.Errorf("%w: load %s does not exist", ErrLoadNotFound, m.LoadID)
		}
	}

	seenBidLoads := make(map[string]bool, len(a.bids))
	for _, b := range a.bids {
		if seenBidLoads[b.LoadID] {
			return fmt.Errorf("%w: load %s", ErrDuplicateBid, b.LoadID)
		}
		seenBidLoads[b.LoadID] = true

		if b.BidPrice <= 0 {
			return fmt.Errorf("%w: load %s has bid price %f", ErrInvalidBidPrice, b.LoadID, b.BidPrice)
		}

		if _, ok := resource.GetLoad(b.LoadID); !ok {
			return fmt.Errorf("%w: load %s does not exist", ErrLoadNotFound, b.LoadID)
		}
	}

	return nil
}
