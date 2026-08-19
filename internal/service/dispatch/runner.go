package dispatch

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

// WorkOrderBatch represents an immutable set of multi-day driver tour work orders
// generated for a dispatch execution epoch.
type WorkOrderBatch struct {
	BatchEpoch           int64
	Tours                []*policy.DriverTour
	TotalTours           int
	TotalLoadedMiles     float64
	TotalEmptyMiles      float64
	TotalDistanceMiles   float64
	EmptyRatio           float64
	TotalGrossRevenue    float64
	TotalOperatingCost   float64
	TotalNetContribution float64
	AssignedLoadIDs      []string
	UnassignedLoadIDs    []string
}

// DispatchRunner orchestrates multi-leg tour synthesis across a fleet of active drivers.
//
// In accordance with Clean Architecture:
//   - Package `internal/service/dispatch` coordinates policy evaluations and model transitions.
//   - Safe for concurrent multi-goroutine execution.
type DispatchRunner struct {
	synthesizer *policy.TourSynthesizer
}

// NewDispatchRunner constructs a new DispatchRunner.
func NewDispatchRunner(synthesizer *policy.TourSynthesizer) *DispatchRunner {
	if synthesizer == nil {
		synthesizer = policy.NewTourSynthesizer(policy.DefaultTourSynthesizerConfig())
	}
	return &DispatchRunner{
		synthesizer: synthesizer,
	}
}

// SynthesizeBatch executes parallel multi-leg tour synthesis for all matched driver-load pairs.
//
// Parameters:
//   - ctx: Context for timeout / cancellation.
//   - epoch: The active decision epoch timestamp.
//   - drivers: Slice of active drivers in the resource state.
//   - matches: Matched driver-load pairings from the single-epoch optimizer.
//   - allLoads: All available candidate loads in the network (for initial load resolution and continuation chaining).
//
// Returns:
//   - WorkOrderBatch containing synthesized multi-leg tours and summary KPIs.
func (r *DispatchRunner) SynthesizeBatch(
	ctx context.Context,
	epoch int64,
	drivers []model.Driver,
	matches []model.DriverLoadMatch,
	allLoads []model.Load,
) (*WorkOrderBatch, error) {
	driverMap := make(map[string]model.Driver, len(drivers))
	for _, d := range drivers {
		driverMap[d.ID] = d
	}

	loadMap := make(map[string]model.Load, len(allLoads))
	for _, l := range allLoads {
		loadMap[l.ID] = l
	}

	// Sort matches canonically by DriverID ascending (Principle 2)
	sortedMatches := make([]model.DriverLoadMatch, len(matches))
	copy(sortedMatches, matches)
	sort.Slice(sortedMatches, func(i, j int) bool {
		return sortedMatches[i].DriverID < sortedMatches[j].DriverID
	})

	type tourResult struct {
		index int
		tour  *policy.DriverTour
		err   error
	}

	resultsChan := make(chan tourResult, len(sortedMatches))
	var wg sync.WaitGroup

	// Parallel synthesis across matched drivers
	for i, match := range sortedMatches {
		wg.Add(1)
		go func(idx int, m model.DriverLoadMatch) {
			defer wg.Done()

			driver, okD := driverMap[m.DriverID]
			if !okD {
				resultsChan <- tourResult{index: idx, err: fmt.Errorf("dispatch: driver %s not found", m.DriverID)}
				return
			}

			initLoad, okL := loadMap[m.LoadID]
			if !okL {
				resultsChan <- tourResult{index: idx, err: fmt.Errorf("dispatch: load %s not found", m.LoadID)}
				return
			}

			tour, err := r.synthesizer.SynthesizeTour(ctx, driver, initLoad, allLoads)
			resultsChan <- tourResult{index: idx, tour: tour, err: err}
		}(i, match)
	}

	wg.Wait()
	close(resultsChan)

	// Collect results in deterministic canonical order
	results := make([]tourResult, len(sortedMatches))
	for res := range resultsChan {
		results[res.index] = res
	}

	var tours []*policy.DriverTour
	assignedLoadsMap := make(map[string]bool)

	var totalLoadedMiles, totalEmptyMiles, grossRev, totalCost float64

	for _, res := range results {
		if res.err != nil {
			return nil, fmt.Errorf("dispatch: tour synthesis failed for index %d: %w", res.index, res.err)
		}

		tours = append(tours, res.tour)
		totalLoadedMiles += res.tour.TotalLoadedMiles()
		totalEmptyMiles += res.tour.TotalEmptyMiles()
		grossRev += res.tour.GrossRevenue()
		totalCost += res.tour.TotalCost()

		for _, leg := range res.tour.Legs() {
			if leg.Type == policy.LegLoaded && leg.LoadID != "" {
				assignedLoadsMap[leg.LoadID] = true
			}
		}
	}

	totalDist := totalLoadedMiles + totalEmptyMiles
	emptyRatio := 0.0
	if totalDist > 0 {
		emptyRatio = totalEmptyMiles / totalDist
	}

	// Canonical sorting of assigned and unassigned load IDs
	var assignedIDs, unassignedIDs []string
	for id := range assignedLoadsMap {
		assignedIDs = append(assignedIDs, id)
	}
	sort.Strings(assignedIDs)

	for _, l := range allLoads {
		if !assignedLoadsMap[l.ID] {
			unassignedIDs = append(unassignedIDs, l.ID)
		}
	}
	sort.Strings(unassignedIDs)

	return &WorkOrderBatch{
		BatchEpoch:           epoch,
		Tours:                tours,
		TotalTours:           len(tours),
		TotalLoadedMiles:     totalLoadedMiles,
		TotalEmptyMiles:      totalEmptyMiles,
		TotalDistanceMiles:   totalDist,
		EmptyRatio:           emptyRatio,
		TotalGrossRevenue:    grossRev,
		TotalOperatingCost:   totalCost,
		TotalNetContribution: grossRev - totalCost,
		AssignedLoadIDs:      assignedIDs,
		UnassignedLoadIDs:    unassignedIDs,
	}, nil
}
