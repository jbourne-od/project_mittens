// Package policy implements the four universal classes of Powell policies
// (PFAs, CFAs, VFAs, and DLAs) over the MOMDP state space.
//
// In accordance with Project Mittens AGENTS.md:
//   - Dependency Boundaries: Imports /internal/domain/model, /internal/domain/model/hos, /pkg/logging
//   - Zero I/O, offline, zero wall-clock time or global mutable state.
//   - Inviolate 5: State immutability via value-based allocation.
//   - Inviolate 6: Zero mutexes on hot paths.
package policy

import (
	"context"
	"log/slog"
	"sort"
	"sync"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/pkg/logging"
)

// RelaySynthesizer discovers and optimizes two-driver relay exchanges across long-haul freight corridors.
type RelaySynthesizer struct {
	costCfg       model.CostConfig
	relayCfg      RelayConfig
	policySpecs   hos.PolicySpecs
	facilityStore *model.FacilityStore
	logger        *slog.Logger
}

// NewRelaySynthesizer constructs a new immutable RelaySynthesizer instance.
func NewRelaySynthesizer(
	costCfg model.CostConfig,
	relayCfg RelayConfig,
	policySpecs hos.PolicySpecs,
	facilityStore *model.FacilityStore,
	logger *slog.Logger,
) *RelaySynthesizer {
	if logger == nil {
		logger = logging.NewNop()
	}
	return &RelaySynthesizer{
		costCfg:       costCfg,
		relayCfg:      relayCfg,
		policySpecs:   policySpecs,
		facilityStore: facilityStore,
		logger:        logger,
	}
}

// SynthesizeRelays scans the candidate pool of idle drivers, unassigned loads, and network relay hubs,
// synthesizing mutually profitable, non-overlapping dual-driver relay exchanges.
func (rs *RelaySynthesizer) SynthesizeRelays(
	ctx context.Context,
	drivers []model.Driver,
	loads []model.Load,
	minHaulMiles float64,
) ([]RelayExchange, error) {
	if len(drivers) < 2 || len(loads) == 0 || rs.facilityStore == nil || rs.facilityStore.Count() == 0 {
		return nil, nil
	}

	if minHaulMiles <= 0 {
		minHaulMiles = 450.0 // Minimum haul distance to warrant relay split
	}

	// Filter candidate long-haul loads
	longHaulLoads := make([]model.Load, 0, len(loads))
	for _, l := range loads {
		if l.Origin.DistanceMiles(l.Destination) >= minHaulMiles {
			longHaulLoads = append(longHaulLoads, l)
		}
	}

	if len(longHaulLoads) == 0 {
		return nil, nil
	}

	// Find all relay hub facilities
	relayHubs := make([]model.Facility, 0)
	for _, fac := range rs.facilityStore.Facilities() {
		if fac.Type == model.FacilityRelayHub || fac.Type == model.FacilityTerminal {
			relayHubs = append(relayHubs, fac)
		}
	}

	// If no designated relay hubs found, find nearest facilities
	if len(relayHubs) == 0 {
		return nil, nil
	}

	type relayCandidate struct {
		exchange  *RelayExchange
		driverIn  string
		driverOut string
		loadID    string
		net       float64
	}

	// Concurrently evaluate candidate combinations
	candidatesChan := make(chan relayCandidate, len(longHaulLoads)*len(relayHubs))
	var wg sync.WaitGroup

	for _, load := range longHaulLoads {
		for _, hub := range relayHubs {
			// Hub must lie roughly between origin and destination
			dIn := load.Origin.DistanceMiles(hub.Location)
			dOut := hub.Location.DistanceMiles(load.Destination)
			dDirect := load.Origin.DistanceMiles(load.Destination)

			if dIn+dOut > dDirect*rs.relayCfg.MaxRelayDetourRatio {
				continue
			}

			wg.Add(1)
			go func(l model.Load, h model.Facility) {
				defer wg.Done()

				for _, d1 := range drivers {
					// Driver 1 must be within reasonable deadhead to origin (<= 250 miles)
					if d1.CurrentLocation.DistanceMiles(l.Origin) > 250.0 {
						continue
					}

					for _, d2 := range drivers {
						if d1.ID == d2.ID {
							continue
						}
						// Driver 2 must be within reasonable deadhead to relay hub (<= 250 miles)
						if d2.CurrentLocation.DistanceMiles(h.Location) > 250.0 {
							continue
						}

						exchange, err := EvaluateRelayFeasibility(
							d1, d2, l, h,
							rs.costCfg, rs.relayCfg, rs.policySpecs,
						)
						if err == nil && exchange.NetContribution > 0 {
							select {
							case candidatesChan <- relayCandidate{
								exchange:  exchange,
								driverIn:  d1.ID,
								driverOut: d2.ID,
								loadID:    l.ID,
								net:       exchange.NetContribution,
							}:
							case <-ctx.Done():
								return
							}
						}
					}
				}
			}(load, hub)
		}
	}

	go func() {
		wg.Wait()
		close(candidatesChan)
	}()

	var allCandidates []relayCandidate
	for cand := range candidatesChan {
		allCandidates = append(allCandidates, cand)
	}

	if len(allCandidates) == 0 {
		return nil, nil
	}

	// Sort candidates deterministically by Net Contribution descending
	sort.Slice(allCandidates, func(i, j int) bool {
		if allCandidates[i].net != allCandidates[j].net {
			return allCandidates[i].net > allCandidates[j].net
		}
		if allCandidates[i].loadID != allCandidates[j].loadID {
			return allCandidates[i].loadID < allCandidates[j].loadID
		}
		if allCandidates[i].driverIn != allCandidates[j].driverIn {
			return allCandidates[i].driverIn < allCandidates[j].driverIn
		}
		return allCandidates[i].driverOut < allCandidates[j].driverOut
	})

	// Greedy non-conflicting selection (each driver and load assigned at most once)
	usedDrivers := make(map[string]bool)
	usedLoads := make(map[string]bool)
	selectedExchanges := make([]RelayExchange, 0)

	for _, cand := range allCandidates {
		if usedDrivers[cand.driverIn] || usedDrivers[cand.driverOut] || usedLoads[cand.loadID] {
			continue
		}

		usedDrivers[cand.driverIn] = true
		usedDrivers[cand.driverOut] = true
		usedLoads[cand.loadID] = true
		selectedExchanges = append(selectedExchanges, *cand.exchange)
	}

	return selectedExchanges, nil
}
