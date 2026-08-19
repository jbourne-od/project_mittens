package policy

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
)

var (
	// ErrNoFeasibleTour is returned when no multi-leg tour can be feasibly synthesized.
	ErrNoFeasibleTour = errors.New("domain/policy: no feasible tour found for driver")
)

// TourSynthesizerConfig holds configuration parameters governing multi-leg tour synthesis.
type TourSynthesizerConfig struct {
	// MaxTourLegs is the maximum number of loaded legs in a single tour (default: 3).
	MaxTourLegs int
	// MaxPlanningHorizonHours is the time ceiling for the synthesized tour in hours (default: 72.0).
	MaxPlanningHorizonHours float64
	// MaxDeadheadMiles is the maximum permitted empty miles between chained legs (default: 150.0).
	MaxDeadheadMiles float64
	// AutoRepositionHome appends an empty domicile reposition leg if driver has a domicile.
	AutoRepositionHome bool
	// CostConfig specifies operational mileage and hourly cost rates.
	CostConfig model.CostConfig
	// FeasibilityConfig specifies speed limits, dwell times, and rest parameters.
	FeasibilityConfig model.FeasibilityConfig
}

// DefaultTourSynthesizerConfig returns production defaults for tour synthesis.
func DefaultTourSynthesizerConfig() TourSynthesizerConfig {
	return TourSynthesizerConfig{
		MaxTourLegs:             3,
		MaxPlanningHorizonHours: 72.0,
		MaxDeadheadMiles:        150.0,
		AutoRepositionHome:      false,
		CostConfig:              model.DefaultCostConfig(),
		FeasibilityConfig:       model.DefaultFeasibilityConfig(),
	}
}

// TourSynthesizer builds multi-day chained driver tours across sequential freight loads.
//
// In accordance with Inviolate 5 (State Immutability) and Inviolate 6 (Lock-Free Concurrency),
// TourSynthesizer is stateless and executes concurrently across worker goroutines.
type TourSynthesizer struct {
	config TourSynthesizerConfig
}

// NewTourSynthesizer creates a new TourSynthesizer instance.
func NewTourSynthesizer(config TourSynthesizerConfig) *TourSynthesizer {
	if config.MaxTourLegs <= 0 {
		config.MaxTourLegs = 3
	}
	if config.MaxPlanningHorizonHours <= 0 {
		config.MaxPlanningHorizonHours = 72.0
	}
	if config.MaxDeadheadMiles <= 0 {
		config.MaxDeadheadMiles = 150.0
	}
	return &TourSynthesizer{config: config}
}

// SynthesizeTour generates an optimal multi-leg DriverTour starting from initialLoad.
//
// Evaluates downstream continuations from candidatePool up to MaxTourLegs, checking
// continuous HOS feasibility, facility dwell windows, and domicile return options.
func (ts *TourSynthesizer) SynthesizeTour(
	ctx context.Context,
	driver model.Driver,
	initialLoad model.Load,
	candidatePool []model.Load,
) (*DriverTour, error) {
	if initialLoad.ID == "" {
		return nil, errors.New("domain/policy: initialLoad cannot be empty")
	}

	// 1. Build Leg 1 from initialLoad
	leg1Legs, endClocks, currentLoc, currentEpoch, err := ts.simulateSingleLoad(
		driver.CurrentLocation,
		driver.AvailableEpoch,
		driver.Clocks,
		initialLoad,
		driver,
	)
	if err != nil {
		return nil, fmt.Errorf("domain/policy: initial load infeasible for tour: %w", err)
	}

	allLegs := append([]TourLeg{}, leg1Legs...)
	usedLoads := map[string]bool{initialLoad.ID: true}

	// 2. Greedy recursive chaining for subsequent legs up to MaxTourLegs
	for legNum := 2; legNum <= ts.config.MaxTourLegs; legNum++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		bestCandidate, bestCandidateLegs, bestEndClocks, bestLoc, bestEpoch, found := ts.findBestContinuation(
			currentLoc,
			currentEpoch,
			endClocks,
			candidatePool,
			usedLoads,
			driver,
		)

		if !found {
			break // No more feasible continuation loads
		}

		// Append continuation legs
		allLegs = append(allLegs, bestCandidateLegs...)
		usedLoads[bestCandidate.ID] = true
		endClocks = bestEndClocks
		currentLoc = bestLoc
		currentEpoch = bestEpoch
	}

	// 3. Optional Domicile Return Leg
	var domicileLoc *model.Location
	if driver.HomeLocation.NodeID != "" || driver.HomeLocation.Lat != 0 || driver.HomeLocation.Lon != 0 {
		domicileLoc = &driver.HomeLocation
	}

	if ts.config.AutoRepositionHome && domicileLoc != nil {
		homeDist := currentLoc.DistanceMiles(*domicileLoc)
		if homeDist > 1.0 {
			homeLeg, err := ts.simulateHomeReposition(currentLoc, currentEpoch, endClocks, *domicileLoc, driver)
			if err == nil {
				allLegs = append(allLegs, homeLeg)
			}
		}
	}

	// 4. Construct validated DriverTour
	return NewDriverTour(driver.ID, allLegs, endClocks, domicileLoc)
}

// simulateSingleLoad forward-simulates deadhead + dwell + loaded movement + rest for a single load.
func (ts *TourSynthesizer) simulateSingleLoad(
	startLoc model.Location,
	startEpoch int64,
	startClocks *hos.DriverClocks,
	load model.Load,
	driver model.Driver,
) ([]TourLeg, *hos.DriverClocks, model.Location, int64, error) {
	// Equipment check
	if !driver.Equipment.CanHandle(load.RequiredEquipment, load.RequiredEndorsements) {
		return nil, nil, startLoc, startEpoch, errors.New("domain/policy: driver equipment incompatible with load")
	}

	deadheadMiles := startLoc.DistanceMiles(load.Origin)
	loadedMiles := load.Origin.DistanceMiles(load.Destination)

	avgSpeed := ts.config.FeasibilityConfig.AverageSpeedMPH
	if avgSpeed <= 0 {
		avgSpeed = 50.0
	}

	deadheadMinutes := int((deadheadMiles / avgSpeed) * 60.0)
	loadedMinutes := int((loadedMiles / avgSpeed) * 60.0)

	var pickupEarliest, pickupLatest, deliveryEarliest, deliveryLatest time.Time
	if load.PickupEarliestEpoch > 0 {
		pickupEarliest = time.Unix(load.PickupEarliestEpoch, 0)
	}
	if load.PickupLatestEpoch > 0 {
		pickupLatest = time.Unix(load.PickupLatestEpoch, 0)
	}
	if load.DeliveryEarliestEpoch > 0 {
		deliveryEarliest = time.Unix(load.DeliveryEarliestEpoch, 0)
	}
	if load.DeliveryLatestEpoch > 0 {
		deliveryLatest = time.Unix(load.DeliveryLatestEpoch, 0)
	}

	sim := hos.NewSimulator()
	specs := ts.config.FeasibilityConfig.HOSPolicySpecs
	if specs.MaxDrivingMin == 0 {
		specs = hos.USPolicySpecs()
	}

	feasResult, err := sim.EvaluateTripFeasibility(
		startClocks,
		deadheadMiles,
		loadedMiles,
		60, // 60 min loading
		60, // 60 min unloading
		avgSpeed,
		pickupEarliest,
		pickupLatest,
		deliveryEarliest,
		deliveryLatest,
		specs,
	)
	if err != nil {
		return nil, nil, startLoc, startEpoch, err
	}
	if !feasResult.IsFeasible {
		return nil, nil, startLoc, startEpoch, fmt.Errorf("domain/policy: trip infeasible: %s", feasResult.InfeasibilityReason)
	}

	deliveryArrivalEpoch := feasResult.UnloadingEndTime.Unix()

	// Check planning horizon ceiling
	if float64(deliveryArrivalEpoch-startEpoch)/3600.0 > ts.config.MaxPlanningHorizonHours {
		return nil, nil, startLoc, startEpoch, errors.New("domain/policy: exceeds tour planning horizon")
	}

	// Build Leg Components
	var legs []TourLeg
	curEpoch := startEpoch

	// 1. Deadhead Leg
	if deadheadMiles > 0.1 {
		deadheadEnd := curEpoch + int64(deadheadMinutes*60)
		deadheadCost := deadheadMiles * ts.config.CostConfig.EmptyMileRate
		legs = append(legs, TourLeg{
			Type:            LegDeadhead,
			LoadID:          load.ID,
			Origin:          startLoc,
			Destination:     load.Origin,
			StartEpoch:      curEpoch,
			EndEpoch:        deadheadEnd,
			DistanceMiles:   deadheadMiles,
			DurationMinutes: deadheadMinutes,
			CostBreakdown: TripCostBreakdown{
				EmptyCost: deadheadCost,
				TotalCost: deadheadCost,
			},
		})
		curEpoch = deadheadEnd
	}

	// 2. Dwell / Pickup wait (if early arrival)
	if feasResult.InsertedDwellMin > 0 {
		dwellEnd := curEpoch + int64(feasResult.InsertedDwellMin*60)
		dwellCost := (float64(feasResult.InsertedDwellMin) / 60.0) * ts.config.CostConfig.EarlyArrivalPerHour
		legs = append(legs, TourLeg{
			Type:            LegDwell,
			LoadID:          load.ID,
			Origin:          load.Origin,
			Destination:     load.Origin,
			StartEpoch:      curEpoch,
			EndEpoch:        dwellEnd,
			DistanceMiles:   0.0,
			DurationMinutes: feasResult.InsertedDwellMin,
			CostBreakdown: TripCostBreakdown{
				DwellCost: dwellCost,
				TotalCost: dwellCost,
			},
		})
		curEpoch = dwellEnd
	}

	// 3. Mandatory Rest Leg (if HOS hygiene break inserted)
	if feasResult.InsertedRestMin > 0 {
		restEnd := curEpoch + int64(feasResult.InsertedRestMin*60)
		legs = append(legs, TourLeg{
			Type:            LegRest,
			LoadID:          load.ID,
			Origin:          load.Origin,
			Destination:     load.Origin,
			StartEpoch:      curEpoch,
			EndEpoch:        restEnd,
			DistanceMiles:   0.0,
			DurationMinutes: feasResult.InsertedRestMin,
		})
		curEpoch = restEnd
	}

	// 4. Loaded Haul Leg
	loadedEnd := deliveryArrivalEpoch
	loadedCost := loadedMiles * ts.config.CostConfig.LoadedMileRate
	fixedCost := ts.config.CostConfig.FixedCostPerLoad
	totalLegCost := loadedCost + fixedCost
	netContrib := load.Revenue - totalLegCost

	legs = append(legs, TourLeg{
		Type:            LegLoaded,
		LoadID:          load.ID,
		Origin:          load.Origin,
		Destination:     load.Destination,
		StartEpoch:      curEpoch,
		EndEpoch:        loadedEnd,
		DistanceMiles:   loadedMiles,
		DurationMinutes: loadedMinutes,
		CostBreakdown: TripCostBreakdown{
			Revenue:         load.Revenue,
			LoadedCost:      loadedCost,
			FixedCost:       fixedCost,
			TotalCost:       totalLegCost,
			NetContribution: netContrib,
		},
	})

	return legs, feasResult.FinalClocks, load.Destination, deliveryArrivalEpoch, nil
}

// findBestContinuation searches candidatePool for the highest-yield feasible next load.
func (ts *TourSynthesizer) findBestContinuation(
	currentLoc model.Location,
	currentEpoch int64,
	currentClocks *hos.DriverClocks,
	candidatePool []model.Load,
	usedLoads map[string]bool,
	driver model.Driver,
) (model.Load, []TourLeg, *hos.DriverClocks, model.Location, int64, bool) {
	type candidateOption struct {
		load       model.Load
		legs       []TourLeg
		endClocks  *hos.DriverClocks
		endLoc     model.Location
		endEpoch   int64
		netContrib float64
	}

	var options []candidateOption

	for _, cand := range candidatePool {
		if usedLoads[cand.ID] {
			continue
		}

		deadhead := currentLoc.DistanceMiles(cand.Origin)
		if deadhead > ts.config.MaxDeadheadMiles {
			continue
		}

		legs, endClocks, endLoc, endEpoch, err := ts.simulateSingleLoad(
			currentLoc,
			currentEpoch,
			currentClocks,
			cand,
			driver,
		)
		if err != nil {
			continue // Infeasible continuation
		}

		var legNetContrib float64
		for _, l := range legs {
			legNetContrib += l.CostBreakdown.NetContribution
		}

		options = append(options, candidateOption{
			load:       cand,
			legs:       legs,
			endClocks:  endClocks,
			endLoc:     endLoc,
			endEpoch:   endEpoch,
			netContrib: legNetContrib,
		})
	}

	if len(options) == 0 {
		return model.Load{}, nil, nil, currentLoc, currentEpoch, false
	}

	// Sort by NetContribution descending; break ties by LoadID ascending (Principle 2)
	sort.Slice(options, func(i, j int) bool {
		if options[i].netContrib != options[j].netContrib {
			return options[i].netContrib > options[j].netContrib
		}
		return options[i].load.ID < options[j].load.ID
	})

	best := options[0]
	return best.load, best.legs, best.endClocks, best.endLoc, best.endEpoch, true
}

// simulateHomeReposition creates a deadhead reposition leg from current location back to driver domicile.
func (ts *TourSynthesizer) simulateHomeReposition(
	currentLoc model.Location,
	currentEpoch int64,
	currentClocks *hos.DriverClocks,
	domicileLoc model.Location,
	driver model.Driver,
) (TourLeg, error) {
	dist := currentLoc.DistanceMiles(domicileLoc)
	if dist <= 0.5 {
		return TourLeg{}, errors.New("domain/policy: already at domicile")
	}

	avgSpeed := ts.config.FeasibilityConfig.AverageSpeedMPH
	if avgSpeed <= 0 {
		avgSpeed = 50.0
	}

	driveMinutes := int((dist / avgSpeed) * 60.0)
	endEpoch := currentEpoch + int64(driveMinutes*60)
	cost := dist * ts.config.CostConfig.EmptyToHomeRate

	return TourLeg{
		Type:            LegRepositionHome,
		Origin:          currentLoc,
		Destination:     domicileLoc,
		StartEpoch:      currentEpoch,
		EndEpoch:        endEpoch,
		DistanceMiles:   dist,
		DurationMinutes: driveMinutes,
		CostBreakdown: TripCostBreakdown{
			EmptyToHomeCost: cost,
			TotalCost:       cost,
			NetContribution: -cost,
		},
	}, nil
}
