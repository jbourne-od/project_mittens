package policy

import (
	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/feasibility"
	"github.com/optimaldynamics/project-mittens/internal/domain/rules"
)

// TripCostBreakdown provides a granular itemization of all economic revenue, cost,
// and penalty components for a driver-load assignment, matching legacy CostFunctions.java.
type TripCostBreakdown struct {
	Revenue         float64
	FixedCost       float64
	LoadedCost      float64
	EmptyCost       float64
	EmptyToHomeCost float64
	DwellCost       float64
	LatePenalty     float64
	DriverBonus     float64
	TotalCost       float64
	NetContribution float64
}

// CalculateTripCost computes the exact financial contribution and cost breakdown for a candidate arc.
//
// In accordance with legacy CostFunctions.java:
//
//	TotalCost = FixedCost + LoadedCost + EmptyCost + EmptyToHomeCost + DwellCost + LatePenalty - DriverBonus
//	NetContribution = Revenue - TotalCost
func CalculateTripCost(
	driver model.Driver,
	load model.Load,
	arc feasibility.CandidateArc,
	cfg model.CostConfig,
) TripCostBreakdown {
	revenue := load.Revenue

	// 1. Fixed dispatch cost
	fixedCost := cfg.FixedCostPerLoad

	// 2. Linehaul loaded transit cost
	loadedCost := arc.LoadedMiles * cfg.LoadedMileRate

	// 3. Empty deadhead repositioning cost
	emptyCost := arc.DeadheadMiles * cfg.EmptyMileRate

	// 4. Empty-to-home repositioning distance and cost
	var emptyToHomeMiles float64
	var emptyToHomeCost float64
	if (driver.CurrentLocation.NodeID != "" || driver.CurrentLocation.Lat != 0 || driver.CurrentLocation.Lon != 0) &&
		(load.Destination.NodeID != "" || load.Destination.Lat != 0 || load.Destination.Lon != 0) {
		// Compute distance from load delivery location back towards driver domicile / current location
		emptyToHomeMiles = load.Destination.DistanceMiles(driver.CurrentLocation)
		emptyToHomeCost = emptyToHomeMiles * cfg.EmptyToHomeRate
	}

	// 5. Facility dwell waiting cost
	dwellHours := float64(arc.InsertedDwellMin) / 60.0
	dwellCost := dwellHours * cfg.EarlyArrivalPerHour

	// 6. Late delivery appointment penalty
	var latePenalty float64
	if load.DeliveryLatestEpoch > 0 && arc.DeliveryArrivalTime.Unix() > load.DeliveryLatestEpoch {
		lateHours := float64(arc.DeliveryArrivalTime.Unix()-load.DeliveryLatestEpoch) / 3600.0
		latePenalty = lateHours * cfg.LateDeliveryPerHour
	}

	// 7. Driver retention / preferred lane bonus
	driverBonus := 0.0
	if cfg.DriverBonusWeight > 0 {
		// Bonus for keeping deadhead low relative to loaded revenue miles
		if arc.LoadedMiles > 0 && arc.DeadheadMiles < 50.0 {
			driverBonus = 25.0 * cfg.DriverBonusWeight
		}
	}

	totalCost := fixedCost + loadedCost + emptyCost + emptyToHomeCost + dwellCost + latePenalty - driverBonus
	netContribution := revenue - totalCost

	return TripCostBreakdown{
		Revenue:         revenue,
		FixedCost:       fixedCost,
		LoadedCost:      loadedCost,
		EmptyCost:       emptyCost,
		EmptyToHomeCost: emptyToHomeCost,
		DwellCost:       dwellCost,
		LatePenalty:     latePenalty,
		DriverBonus:     driverBonus,
		TotalCost:       totalCost,
		NetContribution: netContribution,
	}
}

// CalculateTripCostWithRules computes the exact financial contribution incorporating business rule mutations.
func CalculateTripCostWithRules(
	driver model.Driver,
	load model.Load,
	arc feasibility.CandidateArc,
	cfg model.CostConfig,
	ruleRes rules.RuleEvaluationResult,
) TripCostBreakdown {
	revenue := load.Revenue

	// 1. Fixed dispatch cost with rule multiplier
	fixedCost := cfg.FixedCostPerLoad * ruleRes.FixedCostMultiplier

	// 2. Linehaul loaded transit cost with rule multiplier
	loadedCost := arc.LoadedMiles * cfg.LoadedMileRate * ruleRes.LoadedRateMultiplier

	// 3. Empty deadhead repositioning cost with rule multiplier
	emptyCost := arc.DeadheadMiles * cfg.EmptyMileRate * ruleRes.EmptyRateMultiplier

	// 4. Empty-to-home repositioning distance and cost with rule multiplier
	var emptyToHomeMiles float64
	var emptyToHomeCost float64
	if (driver.CurrentLocation.NodeID != "" || driver.CurrentLocation.Lat != 0 || driver.CurrentLocation.Lon != 0) &&
		(load.Destination.NodeID != "" || load.Destination.Lat != 0 || load.Destination.Lon != 0) {
		emptyToHomeMiles = load.Destination.DistanceMiles(driver.CurrentLocation)
		emptyToHomeCost = emptyToHomeMiles * cfg.EmptyToHomeRate * ruleRes.EmptyToHomeMultiplier
	}

	// 5. Facility dwell waiting cost
	dwellHours := float64(arc.InsertedDwellMin) / 60.0
	dwellCost := dwellHours * cfg.EarlyArrivalPerHour

	// 6. Late delivery appointment penalty
	var latePenalty float64
	if load.DeliveryLatestEpoch > 0 && arc.DeliveryArrivalTime.Unix() > load.DeliveryLatestEpoch {
		lateHours := float64(arc.DeliveryArrivalTime.Unix()-load.DeliveryLatestEpoch) / 3600.0
		latePenalty = lateHours * cfg.LateDeliveryPerHour
	}

	// 7. Driver retention bonus + Rule Bonus
	driverBonus := ruleRes.Bonus
	if cfg.DriverBonusWeight > 0 {
		if arc.LoadedMiles > 0 && arc.DeadheadMiles < 50.0 {
			driverBonus += 25.0 * cfg.DriverBonusWeight
		}
	}

	totalCost := fixedCost + loadedCost + emptyCost + emptyToHomeCost + dwellCost + latePenalty - driverBonus
	netContribution := revenue - totalCost

	return TripCostBreakdown{
		Revenue:         revenue,
		FixedCost:       fixedCost,
		LoadedCost:      loadedCost,
		EmptyCost:       emptyCost,
		EmptyToHomeCost: emptyToHomeCost,
		DwellCost:       dwellCost,
		LatePenalty:     latePenalty,
		DriverBonus:     driverBonus,
		TotalCost:       totalCost,
		NetContribution: netContribution,
	}
}
