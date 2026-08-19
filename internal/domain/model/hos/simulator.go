package hos

import (
	"errors"
	"fmt"
	"math"
	"time"
)

var (
	// ErrSimulatorNilClocks is returned when Simulate is invoked with nil DriverClocks.
	ErrSimulatorNilClocks = errors.New("domain/model/hos: initial DriverClocks cannot be nil")
)

// Simulator executes operational event timelines against regulatory HOS policies.
//
// In accordance with Inviolate 6 (Lock-Free Hot Paths), Simulator contains zero mutexes
// and is completely stateless and safe for parallel evaluation across lookahead branches.
type Simulator struct{}

// NewSimulator returns a new Simulator instance.
func NewSimulator() *Simulator {
	return &Simulator{}
}

// Simulate processes an ordered slice of dispatch events against active driver clocks.
//
// When autoInsertRests is true, the simulation automatically inserts mandatory statutory rest periods
// (30-minute hygiene breaks, 10-hour daily resets, 34-hour cycle restarts) whenever a duty limit is reached,
// accurately modeling real-world driver travel timelines.
func (s *Simulator) Simulate(initialClocks *DriverClocks, events []Event, specs PolicySpecs, autoInsertRests bool) (*SimulationResult, error) {
	if initialClocks == nil {
		return nil, ErrSimulatorNilClocks
	}
	if err := specs.Validate(); err != nil {
		return nil, err
	}

	clocks := initialClocks.Clone()
	timeline := make([]TimelineEntry, 0, len(events)*2)

	totalDuration := 0
	totalDrive := 0
	totalRest := 0
	totalDuty := 0
	totalDistance := 0.0

	for _, ev := range events {
		switch ev.Type {
		case EventDrive:
			remDuration := ev.DurationMin
			totalDistance += ev.DistanceMiles
			distPerMin := 0.0
			if ev.DurationMin > 0 {
				distPerMin = ev.DistanceMiles / float64(ev.DurationMin)
			}

			for remDuration > 0 {
				availDrive := clocks.MaxImmediateDriveMin()
				if availDrive <= 0 {
					if !autoInsertRests {
						return nil, fmt.Errorf("%w: driver %s hit HOS limit during driving at %v", ErrHOSViolation, ev.Description, clocks.Now())
					}
					// Determine required reset
					clocks, timeline = s.insertRequiredReset(clocks, specs, ev.LocationID, &timeline, &totalDuration, &totalRest)
					continue
				}

				driveChunk := min(remDuration, availDrive)
				start := clocks.Now()
				nextClocks, err := clocks.ApplyDrive(driveChunk, specs)
				if err != nil {
					return nil, err
				}

				chunkDist := distPerMin * float64(driveChunk)
				chunkEvent := DriveEvent(driveChunk, chunkDist, ev.LocationID)
				timeline = append(timeline, TimelineEntry{
					StartTime:      start,
					EndTime:        nextClocks.Now(),
					Event:          chunkEvent,
					ClocksSnapshot: *nextClocks,
				})

				clocks = nextClocks
				remDuration -= driveChunk
				totalDuration += driveChunk
				totalDrive += driveChunk
				totalDuty += driveChunk
			}

		case EventLoading, EventUnloading:
			remDuration := ev.DurationMin
			for remDuration > 0 {
				availDuty := min(clocks.RemainingShiftMin(), clocks.RemainingCycleMin())
				if availDuty <= 0 {
					if !autoInsertRests {
						return nil, fmt.Errorf("%w: driver hit HOS limit during on-duty %s at %v", ErrHOSViolation, ev.Type, clocks.Now())
					}
					clocks, timeline = s.insertRequiredReset(clocks, specs, ev.LocationID, &timeline, &totalDuration, &totalRest)
					continue
				}

				dutyChunk := min(remDuration, availDuty)
				start := clocks.Now()
				nextClocks, err := clocks.ApplyOnDutyNotDriving(dutyChunk, specs)
				if err != nil {
					return nil, err
				}

				timeline = append(timeline, TimelineEntry{
					StartTime:      start,
					EndTime:        nextClocks.Now(),
					Event:          Event{Type: ev.Type, DurationMin: dutyChunk, LocationID: ev.LocationID, Description: ev.Description},
					ClocksSnapshot: *nextClocks,
				})

				clocks = nextClocks
				remDuration -= dutyChunk
				totalDuration += dutyChunk
				totalDuty += dutyChunk
			}

		case EventRest:
			start := clocks.Now()
			nextClocks, err := clocks.ApplyOffDuty(ev.DurationMin, ev.IsSleeper, specs)
			if err != nil {
				return nil, err
			}

			timeline = append(timeline, TimelineEntry{
				StartTime:      start,
				EndTime:        nextClocks.Now(),
				Event:          ev,
				ClocksSnapshot: *nextClocks,
			})

			clocks = nextClocks
			totalDuration += ev.DurationMin
			totalRest += ev.DurationMin

		case EventHold:
			// Hold default is on-duty waiting unless specified
			start := clocks.Now()
			nextClocks, err := clocks.ApplyOnDutyNotDriving(ev.DurationMin, specs)
			if err != nil {
				return nil, err
			}

			timeline = append(timeline, TimelineEntry{
				StartTime:      start,
				EndTime:        nextClocks.Now(),
				Event:          ev,
				ClocksSnapshot: *nextClocks,
			})

			clocks = nextClocks
			totalDuration += ev.DurationMin
			totalDuty += ev.DurationMin

		case EventBorderCrossing:
			start := clocks.Now()
			nextClocks, err := clocks.ApplyOnDutyNotDriving(ev.DurationMin, specs)
			if err != nil {
				return nil, err
			}

			timeline = append(timeline, TimelineEntry{
				StartTime:      start,
				EndTime:        nextClocks.Now(),
				Event:          ev,
				ClocksSnapshot: *nextClocks,
			})

			clocks = nextClocks
			totalDuration += ev.DurationMin
			totalDuty += ev.DurationMin
		}
	}

	return &SimulationResult{
		InitialClocks:      initialClocks,
		FinalClocks:        clocks,
		Timeline:           timeline,
		TotalDurationMin:   totalDuration,
		TotalDriveMin:      totalDrive,
		TotalRestMin:       totalRest,
		TotalDutyMin:       totalDuty,
		TotalDistanceMiles: totalDistance,
	}, nil
}

func (s *Simulator) insertRequiredReset(
	clocks *DriverClocks,
	specs PolicySpecs,
	locID string,
	timeline *[]TimelineEntry,
	totalDuration *int,
	totalRest *int,
) (*DriverClocks, []TimelineEntry) {
	var restDuration int
	var isSleeper bool
	var desc string

	if clocks.RemainingCycleMin() <= 0 {
		restDuration = specs.WeeklyResetMin
		isSleeper = false
		desc = "Mandatory 34-Hour Weekly Cycle Restart"
	} else if clocks.RemainingDrivingMin() <= 0 || clocks.RemainingShiftMin() <= 0 {
		restDuration = specs.DailyResetMin
		isSleeper = true
		desc = "Mandatory 10-Hour Daily Off-Duty Reset"
	} else if clocks.RemainingHygieneMin() <= 0 {
		restDuration = specs.RestBreakMin
		isSleeper = false
		desc = "Mandatory 30-Minute Rest Break (Hygiene)"
	} else {
		// General fallback daily reset
		restDuration = specs.DailyResetMin
		isSleeper = true
		desc = "Mandatory Daily Off-Duty Reset"
	}

	start := clocks.Now()
	nextClocks, _ := clocks.ApplyOffDuty(restDuration, isSleeper, specs)
	restEvent := RestEvent(restDuration, isSleeper, locID)
	restEvent.Description = desc

	*timeline = append(*timeline, TimelineEntry{
		StartTime:      start,
		EndTime:        nextClocks.Now(),
		Event:          restEvent,
		ClocksSnapshot: *nextClocks,
	})

	*totalDuration += restDuration
	*totalRest += restDuration

	return nextClocks, *timeline
}

// TripFeasibilityResult models the complete evaluation of a driver-load assignment.
type TripFeasibilityResult struct {
	IsFeasible           bool
	PickupArrivalTime    time.Time
	LoadingEndTime       time.Time
	DeliveryArrivalTime  time.Time
	UnloadingEndTime     time.Time
	DeadheadDriveMin     int
	LoadedDriveMin       int
	InsertedRestMin      int
	InsertedDwellMin     int
	TotalTripDurationMin int
	InfeasibilityReason  string
}

// EvaluateTripFeasibility performs end-to-end trip projection, evaluating:
//  1. Deadhead transit to origin facility.
//  2. Early arrival dwell if driver arrives prior to pickupEarliest.
//  3. Freight loading at origin.
//  4. Linehaul transit to destination facility.
//  5. Early arrival dwell if driver arrives prior to deliveryEarliest.
//  6. Freight unloading at destination.
//
// In accordance with legacy FleetManager parity, checks pickup window [pickupEarliest, pickupLatest]
// and delivery window [deliveryEarliest, deliveryLatest].
func (s *Simulator) EvaluateTripFeasibility(
	initialClocks *DriverClocks,
	deadheadMiles, loadedMiles float64,
	loadingMin, unloadingMin int,
	avgSpeedMPH float64,
	pickupEarliest, pickupLatest time.Time,
	deliveryEarliest, deliveryLatest time.Time,
	specs PolicySpecs,
) (*TripFeasibilityResult, error) {
	if avgSpeedMPH <= 0 {
		avgSpeedMPH = 50.0 // Canonical default 50 MPH
	}

	deadheadDriveMin := int(math.Ceil((deadheadMiles / avgSpeedMPH) * 60.0))
	loadedDriveMin := int(math.Ceil((loadedMiles / avgSpeedMPH) * 60.0))

	// Step 1: Simulate Deadhead transit to origin
	deadheadEvents := []Event{
		DriveEvent(deadheadDriveMin, deadheadMiles, "ORIGIN_DEADHEAD"),
	}
	res1, err := s.Simulate(initialClocks, deadheadEvents, specs, true)
	if err != nil {
		return nil, err
	}

	pickupArrival := res1.FinalClocks.Now()
	clocksAfterPickup := res1.FinalClocks

	// Step 2: Model early arrival dwell at origin if arriving prior to pickupEarliest
	insertedDwellMin := 0
	dwellRestMin := 0
	if !pickupEarliest.IsZero() && pickupArrival.Before(pickupEarliest) {
		earlyPickupDwell := int(math.Ceil(pickupEarliest.Sub(pickupArrival).Minutes()))
		if earlyPickupDwell > 0 {
			insertedDwellMin += earlyPickupDwell
			resDwell, err := s.Simulate(clocksAfterPickup, []Event{HoldEvent(earlyPickupDwell, "ORIGIN_EARLY_ARRIVAL_DWELL")}, specs, true)
			if err != nil {
				return nil, err
			}
			dwellRestMin += resDwell.TotalRestMin
			clocksAfterPickup = resDwell.FinalClocks
		}
	}

	// Step 3: Freight loading at origin
	resLoading, err := s.Simulate(clocksAfterPickup, []Event{LoadingEvent(loadingMin, "ORIGIN_FACILITY")}, specs, true)
	if err != nil {
		return nil, err
	}
	loadingEnd := resLoading.FinalClocks.Now()
	clocksAfterLoading := resLoading.FinalClocks

	// Step 4: Linehaul transit to destination facility
	linehaulEvents := []Event{
		DriveEvent(loadedDriveMin, loadedMiles, "LINEHAUL_TRANSIT"),
	}
	resLinehaul, err := s.Simulate(clocksAfterLoading, linehaulEvents, specs, true)
	if err != nil {
		return nil, err
	}
	deliveryArrival := resLinehaul.FinalClocks.Now()
	clocksAfterDelivery := resLinehaul.FinalClocks

	// Step 5: Model early arrival dwell at destination if arriving prior to deliveryEarliest
	if !deliveryEarliest.IsZero() && deliveryArrival.Before(deliveryEarliest) {
		earlyDeliveryDwell := int(math.Ceil(deliveryEarliest.Sub(deliveryArrival).Minutes()))
		if earlyDeliveryDwell > 0 {
			insertedDwellMin += earlyDeliveryDwell
			resDwell, err := s.Simulate(clocksAfterDelivery, []Event{HoldEvent(earlyDeliveryDwell, "DEST_EARLY_ARRIVAL_DWELL")}, specs, true)
			if err != nil {
				return nil, err
			}
			dwellRestMin += resDwell.TotalRestMin
			clocksAfterDelivery = resDwell.FinalClocks
		}
	}

	// Step 6: Freight unloading at destination
	resUnloading, err := s.Simulate(clocksAfterDelivery, []Event{UnloadingEvent(unloadingMin, "DEST_FACILITY")}, specs, true)
	if err != nil {
		return nil, err
	}
	unloadingEnd := resUnloading.FinalClocks.Now()

	totalDuration := int(math.Ceil(unloadingEnd.Sub(initialClocks.Now()).Minutes()))
	totalInsertedRest := (res1.TotalRestMin + resLoading.TotalRestMin + resLinehaul.TotalRestMin + resUnloading.TotalRestMin + dwellRestMin)

	result := &TripFeasibilityResult{
		IsFeasible:           true,
		PickupArrivalTime:    pickupArrival,
		LoadingEndTime:       loadingEnd,
		DeliveryArrivalTime:  deliveryArrival,
		UnloadingEndTime:     unloadingEnd,
		DeadheadDriveMin:     deadheadDriveMin,
		LoadedDriveMin:       loadedDriveMin,
		InsertedRestMin:      totalInsertedRest,
		InsertedDwellMin:     insertedDwellMin,
		TotalTripDurationMin: totalDuration,
	}

	// Validate appointment and time window feasibility
	if !pickupLatest.IsZero() && pickupArrival.After(pickupLatest) {
		result.IsFeasible = false
		result.InfeasibilityReason = fmt.Sprintf("pickup arrival %v is after latest pickup window %v", pickupArrival, pickupLatest)
	} else if !deliveryLatest.IsZero() && deliveryArrival.After(deliveryLatest) {
		result.IsFeasible = false
		result.InfeasibilityReason = fmt.Sprintf("delivery arrival %v is after latest delivery window %v", deliveryArrival, deliveryLatest)
	}

	return result, nil
}
