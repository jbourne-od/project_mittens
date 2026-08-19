// Package policy implements the four universal classes of Powell policies
// (PFAs, CFAs, VFAs, and DLAs) over the MOMDP state space.
//
// In accordance with Project Mittens AGENTS.md:
//   - Dependency Boundaries: Imports /internal/domain/model, /internal/domain/model/feasibility, /internal/domain/model/hos, /pkg/math, /pkg/logging
//   - Zero I/O, offline, zero wall-clock time or global mutable state.
//   - Inviolate 5: State immutability via value-based allocation and fresh pointers.
//   - Inviolate 6: Zero mutexes on hot paths.
package policy

import (
	"errors"
	"fmt"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
)

var (
	// ErrRelayInfeasible is returned when a candidate relay exchange violates temporal, spatial, or HOS constraints.
	ErrRelayInfeasible = errors.New("domain/policy: relay exchange is physically or temporally infeasible")
	// ErrRelaySameDriver is returned when the inbound and outbound driver are identical.
	ErrRelaySameDriver = errors.New("domain/policy: relay exchange requires two distinct drivers")
)

// RelayLegRole designates the segment role of a driver in a relay exchange.
type RelayLegRole string

const (
	// RelayLegInbound represents the first driver taking the load from Origin to the Relay Facility.
	RelayLegInbound RelayLegRole = "INBOUND"
	// RelayLegOutbound represents the second driver taking the load from the Relay Facility to Destination.
	RelayLegOutbound RelayLegRole = "OUTBOUND"
)

// RelayDriverSegment represents the complete itinerary and HOS timeline for one driver in a relay exchange.
type RelayDriverSegment struct {
	DriverID         string
	Role             RelayLegRole
	Origin           model.Location
	Destination      model.Location
	StartEpoch       int64
	EndEpoch         int64
	DeadheadMiles    float64
	LoadedMiles      float64
	InsertedDwellMin int
	InsertedRestMin  int
	TotalTripMin     int
	CostBreakdown    TripCostBreakdown
	ResultingClocks  *hos.DriverClocks
}

// RelayExchange represents a coordinated two-driver load handoff at an intermediate relay facility.
//
// In accordance with Inviolate 5 (Immutability), RelayExchange is an immutable value-based struct.
type RelayExchange struct {
	LoadID               string
	RelayFacility        model.Facility
	InboundSegment       RelayDriverSegment
	OutboundSegment      RelayDriverSegment
	HandoffStartEpoch    int64
	HandoffCompleteEpoch int64
	InterchangeDwellMin  int
	TotalRevenue         float64
	TotalCost            float64
	NetContribution      float64
	DeliveryArrivalTime  time.Time
}

// RelayConfig specifies operational parameters for synthesizing relay handoffs.
type RelayConfig struct {
	InterchangeDwellMinutes int     // Minimum dwell required for trailer swap (e.g. 60 min)
	MaxRelayDetourRatio     float64 // Maximum allowable detour distance ratio (e.g. 1.25 = 25% detour)
	RelayInterchangeCost    float64 // Fixed operational cost of facility handoff (e.g. $50.0)
	AverageSpeedMPH         float64 // Cruising speed (default: 50.0 mph)
}

// DefaultRelayConfig returns standard industrial relay configuration parameters.
func DefaultRelayConfig() RelayConfig {
	return RelayConfig{
		InterchangeDwellMinutes: 60,
		MaxRelayDetourRatio:     1.25,
		RelayInterchangeCost:    50.0,
		AverageSpeedMPH:         50.0,
	}
}

// EvaluateRelayFeasibility evaluates whether two drivers can legally and profitably coordinate a relay handoff
// for a given load at the specified intermediate relay facility.
func EvaluateRelayFeasibility(
	driverIn model.Driver,
	driverOut model.Driver,
	load model.Load,
	relayFac model.Facility,
	costCfg model.CostConfig,
	relayCfg RelayConfig,
	policySpecs hos.PolicySpecs,
) (*RelayExchange, error) {
	if driverIn.ID == driverOut.ID {
		return nil, ErrRelaySameDriver
	}

	speed := relayCfg.AverageSpeedMPH
	if speed <= 0 {
		speed = 50.0
	}

	interchangeDwell := relayCfg.InterchangeDwellMinutes
	if interchangeDwell <= 0 {
		interchangeDwell = 60
	}

	// 1. Validate equipment compatibility
	if load.RequiredEquipment != "" {
		if driverIn.Equipment.Type != "" && driverIn.Equipment.Type != load.RequiredEquipment {
			return nil, fmt.Errorf("%w: inbound driver equipment mismatch", ErrRelayInfeasible)
		}
		if driverOut.Equipment.Type != "" && driverOut.Equipment.Type != load.RequiredEquipment {
			return nil, fmt.Errorf("%w: outbound driver equipment mismatch", ErrRelayInfeasible)
		}
	}

	// 2. Validate detour ratio
	directDist := load.Origin.DistanceMiles(load.Destination)
	inboundLoadedDist := load.Origin.DistanceMiles(relayFac.Location)
	outboundLoadedDist := relayFac.Location.DistanceMiles(load.Destination)
	relayTotalLoadedDist := inboundLoadedDist + outboundLoadedDist

	if directDist > 0 && relayCfg.MaxRelayDetourRatio > 1.0 {
		if relayTotalLoadedDist > directDist*relayCfg.MaxRelayDetourRatio {
			return nil, fmt.Errorf("%w: relay detour distance exceeds max ratio (%.1f > %.1f)",
				ErrRelayInfeasible, relayTotalLoadedDist, directDist*relayCfg.MaxRelayDetourRatio)
		}
	}

	// -------------------------------------------------------------
	// 3. Forward Project Inbound Driver: Loc(d_in) -> Origin -> RelayFac
	// -------------------------------------------------------------
	inboundDH := driverIn.CurrentLocation.DistanceMiles(load.Origin)
	dhMinutesIn := int(inboundDH / speed * 60.0)

	// Inbound departure
	inStartEpoch := driverIn.AvailableEpoch
	dhArrivalEpoch := inStartEpoch + int64(dhMinutesIn*60)

	// Pickup window alignment
	pickupStartEpoch := dhArrivalEpoch
	if pickupStartEpoch < load.PickupEarliestEpoch {
		pickupStartEpoch = load.PickupEarliestEpoch
	}
	if load.PickupLatestEpoch > 0 && pickupStartEpoch > load.PickupLatestEpoch {
		return nil, fmt.Errorf("%w: inbound driver arrives after pickup latest epoch", ErrRelayInfeasible)
	}

	// Pickup dwell (standard 120 min)
	pickupDwellMin := 120
	transitStartEpochIn := pickupStartEpoch + int64(pickupDwellMin*60)

	// Transit to RelayFac
	transitMinutesIn := int(inboundLoadedDist / speed * 60.0)
	relayArrivalEpochIn := transitStartEpochIn + int64(transitMinutesIn*60)

	// Check facility operating hours at inbound arrival
	inboundArrivalTime := time.Unix(relayArrivalEpochIn, 0).UTC()
	if !relayFac.IsOpenAt(inboundArrivalTime) {
		return nil, fmt.Errorf("%w: relay facility is closed at inbound arrival time %s",
			ErrRelayInfeasible, inboundArrivalTime.Format(time.RFC3339))
	}

	// Evaluate Inbound HOS compliance
	sim := hos.NewSimulator()
	clocksIn := driverIn.Clocks
	if clocksIn == nil {
		clocksIn = hos.NewDriverClocks(policySpecs, time.Unix(inStartEpoch, 0).UTC())
	}

	eventsIn := []hos.Event{
		hos.DriveEvent(dhMinutesIn, inboundDH, load.Origin.NodeID),
		{Type: hos.EventLoading, DurationMin: pickupDwellMin, LocationID: load.Origin.NodeID, Description: "Load Pickup"},
		hos.DriveEvent(transitMinutesIn, inboundLoadedDist, relayFac.ID),
		{Type: hos.EventUnloading, DurationMin: interchangeDwell, LocationID: relayFac.ID, Description: "Relay Interchange Dropoff"},
	}

	resIn, err := sim.Simulate(clocksIn, eventsIn, policySpecs, false)
	if err != nil {
		return nil, fmt.Errorf("%w: inbound driver HOS violation: %v", ErrRelayInfeasible, err)
	}
	clocksIn = resIn.FinalClocks

	inboundEndEpoch := relayArrivalEpochIn + int64(interchangeDwell*60)

	// -------------------------------------------------------------
	// 4. Forward Project Outbound Driver: Loc(d_out) -> RelayFac -> Destination
	// -------------------------------------------------------------
	outboundDH := driverOut.CurrentLocation.DistanceMiles(relayFac.Location)
	dhMinutesOut := int(outboundDH / speed * 60.0)

	outStartEpoch := driverOut.AvailableEpoch
	dhArrivalEpochOut := outStartEpoch + int64(dhMinutesOut*60)

	// Outbound driver cannot depart RelayFac until:
	// 1. Inbound driver arrives and completes interchange drop-off (inboundEndEpoch)
	// 2. Outbound driver arrives at RelayFac (dhArrivalEpochOut)
	handoffEpoch := inboundEndEpoch
	if dhArrivalEpochOut > handoffEpoch {
		handoffEpoch = dhArrivalEpochOut
	}

	// Transit from RelayFac to Destination
	transitMinutesOut := int(outboundLoadedDist / speed * 60.0)
	destArrivalEpoch := handoffEpoch + int64(transitMinutesOut*60)
	destArrivalTime := time.Unix(destArrivalEpoch, 0).UTC()

	// Check latest delivery window
	if load.DeliveryLatestEpoch > 0 && destArrivalEpoch > load.DeliveryLatestEpoch {
		return nil, fmt.Errorf("%w: relay delivery arrives after delivery latest epoch (%s > %s)",
			ErrRelayInfeasible, destArrivalTime.Format(time.RFC3339),
			time.Unix(load.DeliveryLatestEpoch, 0).UTC().Format(time.RFC3339))
	}

	// Delivery unloading dwell
	deliveryDwellMin := 120
	outboundEndEpoch := destArrivalEpoch + int64(deliveryDwellMin*60)

	// Evaluate Outbound HOS compliance
	clocksOut := driverOut.Clocks
	if clocksOut == nil {
		clocksOut = hos.NewDriverClocks(policySpecs, time.Unix(outStartEpoch, 0).UTC())
	}

	eventsOut := []hos.Event{
		hos.DriveEvent(dhMinutesOut, outboundDH, relayFac.ID),
	}
	if handoffEpoch > dhArrivalEpochOut {
		waitMin := int((handoffEpoch - dhArrivalEpochOut) / 60)
		if waitMin > 0 {
			eventsOut = append(eventsOut, hos.Event{
				Type:        hos.EventHold,
				DurationMin: waitMin,
				LocationID:  relayFac.ID,
				Description: "Relay Handoff Wait",
			})
		}
	}
	eventsOut = append(eventsOut,
		hos.DriveEvent(transitMinutesOut, outboundLoadedDist, load.Destination.NodeID),
		hos.Event{Type: hos.EventUnloading, DurationMin: deliveryDwellMin, LocationID: load.Destination.NodeID, Description: "Delivery Unload"},
	)

	resOut, err := sim.Simulate(clocksOut, eventsOut, policySpecs, false)
	if err != nil {
		return nil, fmt.Errorf("%w: outbound driver HOS violation: %v", ErrRelayInfeasible, err)
	}
	clocksOut = resOut.FinalClocks

	// -------------------------------------------------------------
	// 5. Cost Breakdown & Economic Accounting
	// -------------------------------------------------------------
	// Inbound cost
	inTotalTripMin := dhMinutesIn + pickupDwellMin + transitMinutesIn + interchangeDwell
	inLoadedCost := inboundLoadedDist * costCfg.LoadedMileRate
	inEmptyCost := inboundDH * costCfg.EmptyMileRate
	inFixedCost := costCfg.FixedCostPerLoad / 2.0
	inTotalCost := inLoadedCost + inEmptyCost + inFixedCost

	costBreakdownIn := TripCostBreakdown{
		Revenue:         load.Revenue / 2.0,
		FixedCost:       inFixedCost,
		LoadedCost:      inLoadedCost,
		EmptyCost:       inEmptyCost,
		EmptyToHomeCost: 0.0,
		DwellCost:       0.0,
		LatePenalty:     0.0,
		DriverBonus:     0.0,
		TotalCost:       inTotalCost,
		NetContribution: (load.Revenue / 2.0) - inTotalCost,
	}

	inboundSegment := RelayDriverSegment{
		DriverID:         driverIn.ID,
		Role:             RelayLegInbound,
		Origin:           driverIn.CurrentLocation,
		Destination:      relayFac.Location,
		StartEpoch:       inStartEpoch,
		EndEpoch:         inboundEndEpoch,
		DeadheadMiles:    inboundDH,
		LoadedMiles:      inboundLoadedDist,
		InsertedDwellMin: pickupDwellMin + interchangeDwell,
		InsertedRestMin:  0,
		TotalTripMin:     inTotalTripMin,
		CostBreakdown:    costBreakdownIn,
		ResultingClocks:  clocksIn,
	}

	// Outbound cost
	outTotalTripMin := dhMinutesOut + transitMinutesOut + deliveryDwellMin
	outLoadedCost := outboundLoadedDist * costCfg.LoadedMileRate
	outEmptyCost := outboundDH * costCfg.EmptyMileRate
	outFixedCost := costCfg.FixedCostPerLoad / 2.0
	outTotalCost := outLoadedCost + outEmptyCost + outFixedCost

	costBreakdownOut := TripCostBreakdown{
		Revenue:         load.Revenue / 2.0,
		FixedCost:       outFixedCost,
		LoadedCost:      outLoadedCost,
		EmptyCost:       outEmptyCost,
		EmptyToHomeCost: 0.0,
		DwellCost:       0.0,
		LatePenalty:     0.0,
		DriverBonus:     0.0,
		TotalCost:       outTotalCost,
		NetContribution: (load.Revenue / 2.0) - outTotalCost,
	}

	outboundSegment := RelayDriverSegment{
		DriverID:         driverOut.ID,
		Role:             RelayLegOutbound,
		Origin:           driverOut.CurrentLocation,
		Destination:      load.Destination,
		StartEpoch:       outStartEpoch,
		EndEpoch:         outboundEndEpoch,
		DeadheadMiles:    outboundDH,
		LoadedMiles:      outboundLoadedDist,
		InsertedDwellMin: deliveryDwellMin,
		InsertedRestMin:  0,
		TotalTripMin:     outTotalTripMin,
		CostBreakdown:    costBreakdownOut,
		ResultingClocks:  clocksOut,
	}

	totalRelayCost := inTotalCost + outTotalCost + relayCfg.RelayInterchangeCost
	netRelayContribution := load.Revenue - totalRelayCost

	return &RelayExchange{
		LoadID:               load.ID,
		RelayFacility:        relayFac,
		InboundSegment:       inboundSegment,
		OutboundSegment:      outboundSegment,
		HandoffStartEpoch:    relayArrivalEpochIn,
		HandoffCompleteEpoch: handoffEpoch,
		InterchangeDwellMin:  interchangeDwell,
		TotalRevenue:         load.Revenue,
		TotalCost:            totalRelayCost,
		NetContribution:      netRelayContribution,
		DeliveryArrivalTime:  destArrivalTime,
	}, nil
}
