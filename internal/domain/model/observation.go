package model

import (
	"sort"
)

// BidFeedback represents an endogenous market observation signal Y_{t+1} indicating
// whether a submitted spot pricing bid won or lost the contracted shipment.
type BidFeedback struct {
	LoadID       string
	Won          bool    // True if carrier was awarded the load at the bid price
	WinningPrice float64 // The clearing market price of the shipment
}

// Observation represents the generalized observation process W_{t+1} = (D_{t+1}, Y_{t+1}) at epoch t+1.
//
// Formulated under HLD Section 6.3:
//   - D_{t+1} represents realized customer load offers arriving from the market.
//   - Y_{t+1} represents endogenous feedback on pricing bids submitted at epoch t.
//
// In accordance with Inviolate 5, Observation is immutable once created.
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
