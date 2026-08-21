// Package dispatch coordinates multi-leg tour synthesis, relay handoffs,
// and execution work orders across active driver fleets.
//
// In accordance with Project Mittens AGENTS.md:
//   - Dependency Boundaries: Imports /internal/domain/model, /internal/domain/policy, /pkg/logging
//   - Inviolate 5: State immutability via value-based allocation.
//   - Inviolate 6: Zero mutexes on hot paths.
package dispatch

import (
	"context"
	"fmt"
	"sort"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

// RelayDispatchBatch represents a combined dispatch execution plan containing both
// single-driver multi-leg tours and dual-driver coordinated relay handoffs.
type RelayDispatchBatch struct {
	BatchEpoch           int64
	DirectTours          []*policy.DriverTour
	RelayExchanges       []policy.RelayExchange
	TotalTours           int
	TotalRelays          int
	TotalLoadedMiles     float64
	TotalEmptyMiles      float64
	TotalDistanceMiles   float64
	EmptyRatio           float64
	TotalGrossRevenue    float64
	TotalOperatingCost   float64
	TotalNetContribution float64
	AssignedDriverIDs    []string
	AssignedLoadIDs      []string
}

// RelayDispatchRunner coordinates combined direct tour building and relay handoffs.
type RelayDispatchRunner struct {
	directRunner     *DispatchRunner
	relaySynthesizer *policy.RelaySynthesizer
}

// NewRelayDispatchRunner constructs a new RelayDispatchRunner.
func NewRelayDispatchRunner(
	directRunner *DispatchRunner,
	relaySynthesizer *policy.RelaySynthesizer,
) *RelayDispatchRunner {
	if directRunner == nil {
		directRunner = NewDispatchRunner(nil)
	}
	return &RelayDispatchRunner{
		directRunner:     directRunner,
		relaySynthesizer: relaySynthesizer,
	}
}

// SynthesizeRelayBatch processes the active fleet and available loads:
//  1. Discovers and coordinates profitable multi-driver relay handoffs for long-haul freight.
//  2. Synthesizes multi-leg chained tours for remaining drivers on localized regional freight.
//  3. Aggregates complete whole-fleet operational and financial KPIs into an immutable RelayDispatchBatch.
func (r *RelayDispatchRunner) SynthesizeRelayBatch(
	ctx context.Context,
	epoch int64,
	drivers []model.Driver,
	matches []model.DriverLoadMatch,
	allLoads []model.Load,
	minRelayHaulMiles float64,
) (*RelayDispatchBatch, error) {
	// 1. Identify and synthesize relay handoffs if synthesizer is configured
	var relayExchanges []policy.RelayExchange
	assignedRelayDrivers := make(map[string]bool)
	assignedRelayLoads := make(map[string]bool)

	if r.relaySynthesizer != nil {
		exchanges, err := r.relaySynthesizer.SynthesizeRelays(ctx, drivers, allLoads, minRelayHaulMiles)
		if err != nil {
			return nil, fmt.Errorf("dispatch: relay synthesis failed: %w", err)
		}
		if len(exchanges) > 0 {
			relayExchanges = exchanges
			for _, ex := range exchanges {
				assignedRelayDrivers[ex.InboundSegment.DriverID] = true
				assignedRelayDrivers[ex.OutboundSegment.DriverID] = true
				assignedRelayLoads[ex.LoadID] = true
			}
		}
	}

	// 2. Filter remaining drivers and matches for direct tour synthesis
	remainingDrivers := make([]model.Driver, 0, len(drivers))
	for _, d := range drivers {
		if !assignedRelayDrivers[d.ID] {
			remainingDrivers = append(remainingDrivers, d)
		}
	}

	remainingMatches := make([]model.DriverLoadMatch, 0, len(matches))
	for _, m := range matches {
		if !assignedRelayDrivers[m.DriverID] && !assignedRelayLoads[m.LoadID] {
			remainingMatches = append(remainingMatches, m)
		}
	}

	remainingLoads := make([]model.Load, 0, len(allLoads))
	for _, l := range allLoads {
		if !assignedRelayLoads[l.ID] {
			remainingLoads = append(remainingLoads, l)
		}
	}

	// 3. Synthesize direct tours for remaining fleet
	directBatch, err := r.directRunner.SynthesizeBatch(ctx, epoch, remainingDrivers, remainingMatches, remainingLoads)
	if err != nil {
		return nil, err
	}

	// 4. Aggregate combined KPIs
	totalLoaded := directBatch.TotalLoadedMiles
	totalEmpty := directBatch.TotalEmptyMiles
	totalRev := directBatch.TotalGrossRevenue
	totalCost := directBatch.TotalOperatingCost
	totalNet := directBatch.TotalNetContribution

	assignedDriversMap := make(map[string]bool)
	assignedLoadsMap := make(map[string]bool)

	for _, lID := range directBatch.AssignedLoadIDs {
		assignedLoadsMap[lID] = true
	}

	for _, ex := range relayExchanges {
		totalLoaded += ex.InboundSegment.LoadedMiles + ex.OutboundSegment.LoadedMiles
		totalEmpty += ex.InboundSegment.DeadheadMiles + ex.OutboundSegment.DeadheadMiles
		totalRev += ex.TotalRevenue
		totalCost += ex.TotalCost
		totalNet += ex.NetContribution

		assignedDriversMap[ex.InboundSegment.DriverID] = true
		assignedDriversMap[ex.OutboundSegment.DriverID] = true
		assignedLoadsMap[ex.LoadID] = true
	}

	for _, tour := range directBatch.Tours {
		assignedDriversMap[tour.DriverID()] = true
	}

	assignedDriverList := make([]string, 0, len(assignedDriversMap))
	for dID := range assignedDriversMap {
		assignedDriverList = append(assignedDriverList, dID)
	}
	sort.Strings(assignedDriverList)

	assignedLoadList := make([]string, 0, len(assignedLoadsMap))
	for lID := range assignedLoadsMap {
		assignedLoadList = append(assignedLoadList, lID)
	}
	sort.Strings(assignedLoadList)

	totalDistance := totalLoaded + totalEmpty
	emptyRatio := 0.0
	if totalDistance > 0 {
		emptyRatio = totalEmpty / totalDistance
	}

	return &RelayDispatchBatch{
		BatchEpoch:           epoch,
		DirectTours:          directBatch.Tours,
		RelayExchanges:       relayExchanges,
		TotalTours:           len(directBatch.Tours),
		TotalRelays:          len(relayExchanges),
		TotalLoadedMiles:     totalLoaded,
		TotalEmptyMiles:      totalEmpty,
		TotalDistanceMiles:   totalDistance,
		EmptyRatio:           emptyRatio,
		TotalGrossRevenue:    totalRev,
		TotalOperatingCost:   totalCost,
		TotalNetContribution: totalNet,
		AssignedDriverIDs:    assignedDriverList,
		AssignedLoadIDs:      assignedLoadList,
	}, nil
}
