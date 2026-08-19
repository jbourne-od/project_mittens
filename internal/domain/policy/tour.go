package policy

import (
	"errors"
	"fmt"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
)

var (
	// ErrEmptyTour is returned when constructing a tour with zero legs.
	ErrEmptyTour = errors.New("domain/policy: tour must contain at least one leg")
	// ErrTourDiscontinuousSpatial is returned when consecutive tour legs have mismatched locations.
	ErrTourDiscontinuousSpatial = errors.New("domain/policy: tour legs are spatially discontinuous")
	// ErrTourDiscontinuousTemporal is returned when consecutive tour legs have backward time jumps.
	ErrTourDiscontinuousTemporal = errors.New("domain/policy: tour legs are temporally discontinuous")
)

// TourLegType defines the physical or regulatory activity represented by a tour segment.
type TourLegType string

const (
	// LegDeadhead represents empty transit from driver current location to load pickup facility.
	LegDeadhead TourLegType = "DEADHEAD"
	// LegLoaded represents revenue-generating haul from origin to destination.
	LegLoaded TourLegType = "LOADED"
	// LegRest represents a mandatory 10-hour reset or 30-minute hygiene rest interval.
	LegRest TourLegType = "REST"
	// LegDwell represents waiting at a facility during an appointment time window.
	LegDwell TourLegType = "DWELL"
	// LegRepositionHome represents an empty reposition movement returning the driver to their home domicile.
	LegRepositionHome TourLegType = "REPOSITION_HOME"
)

// TourLeg represents an individual continuous movement or regulatory rest period in a driver tour.
//
// In accordance with Inviolate 5 (State Immutability), TourLeg values are immutable once constructed.
type TourLeg struct {
	Type            TourLegType
	LoadID          string
	Origin          model.Location
	Destination     model.Location
	StartEpoch      int64
	EndEpoch        int64
	DistanceMiles   float64
	DurationMinutes int
	CostBreakdown   TripCostBreakdown
}

// DriverTour represents a multi-leg chained dispatch work order for an individual driver.
//
// Formulated under HLD Section 5 and Legacy Java Parity (`fleetmanager/dispatching/DriverTour.java`),
// a DriverTour sequences multiple loads, mandatory rest periods, and optional domicile returns
// while preserving continuous Hours-of-Service and spatial flow conservation.
type DriverTour struct {
	driverID           string
	legs               []TourLeg
	totalLoadedMiles   float64
	totalEmptyMiles    float64
	totalDistanceMiles float64
	emptyRatio         float64
	grossRevenue       float64
	totalCost          float64
	netContribution    float64
	startsAtEpoch      int64
	endsAtEpoch        int64
	endsAtDomicile     bool
	finalLocation      model.Location
	finalClocks        *hos.DriverClocks
}

// NewDriverTour creates and validates an immutable DriverTour from a sequence of tour legs.
func NewDriverTour(
	driverID string,
	legs []TourLeg,
	finalClocks *hos.DriverClocks,
	domicileLocation *model.Location,
) (*DriverTour, error) {
	if len(legs) == 0 {
		return nil, ErrEmptyTour
	}
	if driverID == "" {
		return nil, errors.New("domain/policy: driverID cannot be empty")
	}

	copiedLegs := make([]TourLeg, len(legs))
	copy(copiedLegs, legs)

	var loadedMiles, emptyMiles, grossRev, totalCost float64

	// Verify spatial and temporal contiguity across consecutive legs
	for i := 0; i < len(copiedLegs); i++ {
		leg := copiedLegs[i]

		if leg.EndEpoch < leg.StartEpoch {
			return nil, fmt.Errorf("%w: leg %d ends (%d) before it starts (%d)",
				ErrTourDiscontinuousTemporal, i, leg.EndEpoch, leg.StartEpoch)
		}

		if i > 0 {
			prev := copiedLegs[i-1]
			if leg.StartEpoch < prev.EndEpoch {
				return nil, fmt.Errorf("%w: leg %d starts (%d) before prev leg ends (%d)",
					ErrTourDiscontinuousTemporal, i, leg.StartEpoch, prev.EndEpoch)
			}
			// Allow micro-distance floating point tolerance for spatial continuity (0.5 miles)
			dist := prev.Destination.DistanceMiles(leg.Origin)
			if dist > 0.5 && prev.Destination.NodeID != leg.Origin.NodeID {
				return nil, fmt.Errorf("%w: leg %d origin (%s) does not match prev leg dest (%s), dist = %.2f mi",
					ErrTourDiscontinuousSpatial, i, leg.Origin.NodeID, prev.Destination.NodeID, dist)
			}
		}

		if leg.Type == LegLoaded {
			loadedMiles += leg.DistanceMiles
			grossRev += leg.CostBreakdown.Revenue
		} else if leg.Type == LegDeadhead || leg.Type == LegRepositionHome {
			emptyMiles += leg.DistanceMiles
		}

		totalCost += leg.CostBreakdown.TotalCost
	}

	totalDist := loadedMiles + emptyMiles
	emptyRatio := 0.0
	if totalDist > 0 {
		emptyRatio = emptyMiles / totalDist
	}

	netContrib := grossRev - totalCost

	firstLeg := copiedLegs[0]
	lastLeg := copiedLegs[len(copiedLegs)-1]

	endsAtDom := false
	if domicileLocation != nil {
		if lastLeg.Destination.DistanceMiles(*domicileLocation) < 5.0 || lastLeg.Destination.NodeID == domicileLocation.NodeID {
			endsAtDom = true
		}
	}

	var clonedClocks *hos.DriverClocks
	if finalClocks != nil {
		clonedClocks = finalClocks.Clone()
	}

	return &DriverTour{
		driverID:           driverID,
		legs:               copiedLegs,
		totalLoadedMiles:   loadedMiles,
		totalEmptyMiles:    emptyMiles,
		totalDistanceMiles: totalDist,
		emptyRatio:         emptyRatio,
		grossRevenue:       grossRev,
		totalCost:          totalCost,
		netContribution:    netContrib,
		startsAtEpoch:      firstLeg.StartEpoch,
		endsAtEpoch:        lastLeg.EndEpoch,
		endsAtDomicile:     endsAtDom,
		finalLocation:      lastLeg.Destination,
		finalClocks:        clonedClocks,
	}, nil
}

// DriverID returns the driver ID assigned to this tour.
func (t *DriverTour) DriverID() string {
	return t.driverID
}

// Legs returns a deep copy of all tour legs.
func (t *DriverTour) Legs() []TourLeg {
	out := make([]TourLeg, len(t.legs))
	copy(out, t.legs)
	return out
}

// LegCount returns the total number of legs in the tour.
func (t *DriverTour) LegCount() int {
	return len(t.legs)
}

// LoadedLegCount returns the number of revenue-producing loaded legs in the tour.
func (t *DriverTour) LoadedLegCount() int {
	count := 0
	for _, l := range t.legs {
		if l.Type == LegLoaded {
			count++
		}
	}
	return count
}

// TotalLoadedMiles returns total revenue-generating loaded miles.
func (t *DriverTour) TotalLoadedMiles() float64 {
	return t.totalLoadedMiles
}

// TotalEmptyMiles returns total non-revenue empty miles (deadhead + reposition).
func (t *DriverTour) TotalEmptyMiles() float64 {
	return t.totalEmptyMiles
}

// TotalDistanceMiles returns the sum of loaded and empty miles.
func (t *DriverTour) TotalDistanceMiles() float64 {
	return t.totalDistanceMiles
}

// EmptyRatio returns the proportion of empty miles over total miles.
func (t *DriverTour) EmptyRatio() float64 {
	return t.emptyRatio
}

// GrossRevenue returns total gross revenue earned across all loaded legs.
func (t *DriverTour) GrossRevenue() float64 {
	return t.grossRevenue
}

// TotalCost returns the total operating cost incurred across all legs.
func (t *DriverTour) TotalCost() float64 {
	return t.totalCost
}

// NetContribution returns net operating profit (GrossRevenue - TotalCost).
func (t *DriverTour) NetContribution() float64 {
	return t.netContribution
}

// ProfitPerTotalMile returns the net contribution earned per total mile driven.
func (t *DriverTour) ProfitPerTotalMile() float64 {
	if t.totalDistanceMiles <= 0 {
		return 0.0
	}
	return t.netContribution / t.totalDistanceMiles
}

// StartsAtEpoch returns the start timestamp of the first tour leg.
func (t *DriverTour) StartsAtEpoch() int64 {
	return t.startsAtEpoch
}

// EndsAtEpoch returns the completion timestamp of the final tour leg.
func (t *DriverTour) EndsAtEpoch() int64 {
	return t.endsAtEpoch
}

// TotalDurationHours returns the total elapsed time of the tour in decimal hours.
func (t *DriverTour) TotalDurationHours() float64 {
	return float64(t.endsAtEpoch-t.startsAtEpoch) / 3600.0
}

// EndsAtDomicile returns true if the final leg delivers or repositions the driver at their home domicile.
func (t *DriverTour) EndsAtDomicile() bool {
	return t.endsAtDomicile
}

// FinalLocation returns the physical destination where the driver finishes the tour.
func (t *DriverTour) FinalLocation() model.Location {
	return t.finalLocation
}

// FinalClocks returns a deep copy of the driver's HOS clocks upon tour completion.
func (t *DriverTour) FinalClocks() *hos.DriverClocks {
	if t.finalClocks == nil {
		return nil
	}
	return t.finalClocks.Clone()
}

// Clone creates an exact deep copy of the DriverTour (Inviolate 5).
func (t *DriverTour) Clone() *DriverTour {
	copiedLegs := make([]TourLeg, len(t.legs))
	copy(copiedLegs, t.legs)

	var clonedClocks *hos.DriverClocks
	if t.finalClocks != nil {
		clonedClocks = t.finalClocks.Clone()
	}

	return &DriverTour{
		driverID:           t.driverID,
		legs:               copiedLegs,
		totalLoadedMiles:   t.totalLoadedMiles,
		totalEmptyMiles:    t.totalEmptyMiles,
		totalDistanceMiles: t.totalDistanceMiles,
		emptyRatio:         t.emptyRatio,
		grossRevenue:       t.grossRevenue,
		totalCost:          t.totalCost,
		netContribution:    t.netContribution,
		startsAtEpoch:      t.startsAtEpoch,
		endsAtEpoch:        t.endsAtEpoch,
		endsAtDomicile:     t.endsAtDomicile,
		finalLocation:      t.finalLocation,
		finalClocks:        clonedClocks,
	}
}
