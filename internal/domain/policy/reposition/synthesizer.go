package reposition

import (
	"context"
	"fmt"
	"sort"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

// RepositioningSynthesizer synthesizes economically optimal, HOS-compliant empty tractor repositioning moves.
type RepositioningSynthesizer struct {
	calc *RegionalBalanceCalculator
}

// NewRepositioningSynthesizer constructs an initialized RepositioningSynthesizer.
func NewRepositioningSynthesizer() *RepositioningSynthesizer {
	return &RepositioningSynthesizer{
		calc: NewRegionalBalanceCalculator(),
	}
}

// SynthesizeRepositioningMoves evaluates unassigned drivers in deficit markets and generates relocations to high-demand clusters.
func (s *RepositioningSynthesizer) SynthesizeRepositioningMoves(
	ctx context.Context,
	resource *model.ResourceState,
	regionMgr *model.RegionManager,
	unassignedDrivers []model.Driver,
	cfg RepositioningConfig,
) ([]RepositioningMove, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if resource == nil || regionMgr == nil || len(unassignedDrivers) == 0 {
		return nil, nil
	}

	balances := s.calc.ComputeBalance(resource, regionMgr, cfg.DefaultRegionalYield)
	surplusRegions := GetSurplusRegions(balances)

	if len(surplusRegions) == 0 {
		// No regional shortages to arbitrage into
		return nil, nil
	}

	// Map each surplus region to its target locations (cluster load origins or centroid)
	regionTargetLocs := make(map[string]model.Location, len(surplusRegions))
	for _, l := range resource.Loads() {
		regID := regionMgr.GetRegionID(l.Origin)
		if balances[regID].Deficit < 0 && regionTargetLocs[regID].NodeID == "" {
			regionTargetLocs[regID] = l.Origin
		}
	}

	var moves []RepositioningMove

	for _, driver := range unassignedDrivers {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		currentReg := regionMgr.GetRegionID(driver.CurrentLocation)
		currentSnap, exists := balances[currentReg]
		if !exists || currentSnap.Deficit < cfg.DeficitHurdle {
			// Driver is not in a surplus/deficit market requiring relocation
			continue
		}

		var bestMove *RepositioningMove
		var bestNetValue float64 = -1e9

		for _, targetReg := range surplusRegions {
			if targetReg == currentReg {
				continue
			}

			targetLoc, ok := regionTargetLocs[targetReg]
			if !ok {
				// Query region manager for registered geometric centroid
				if reg, exists := regionMgr.GetRegion(targetReg); exists && (reg.Centroid.Lat != 0 || reg.Centroid.Lon != 0) {
					targetLoc = reg.Centroid
				} else {
					targetLoc = model.Location{NodeID: targetReg}
				}
			}

			dist := driver.CurrentLocation.DistanceMiles(targetLoc)
			if dist <= 0 || dist > cfg.MaxRepositionDistanceMiles {
				continue
			}

			transitHours := dist / cfg.AverageTransitSpeedMPH
			if transitHours > driver.DriveHoursRemaining || transitHours > driver.DutyHoursRemaining {
				// Infeasible: exceeds driver's remaining HOS clocks
				continue
			}

			transitSeconds := int64(transitHours * 3600.0)
			arrivalEpoch := driver.AvailableEpoch + transitSeconds

			deadheadCost := dist * cfg.EmptyMileCostRate
			targetYield := balances[targetReg].ShadowPrice
			localYield := currentSnap.ShadowPrice

			netValue := targetYield - deadheadCost - localYield

			if netValue >= cfg.MinArbitrageThreshold && netValue > bestNetValue {
				bestNetValue = netValue
				move := RepositioningMove{
					DriverID:               driver.ID,
					OriginLocation:         driver.CurrentLocation,
					TargetRegionID:         targetReg,
					TargetLocation:         targetLoc,
					StartEpoch:             driver.AvailableEpoch,
					ArrivalEpoch:           arrivalEpoch,
					DeadheadMiles:          dist,
					EstimatedCost:          deadheadCost,
					ExpectedArbitrageYield: targetYield,
					NetRepositioningValue:  netValue,
				}
				bestMove = &move
			}
		}

		if bestMove != nil {
			moves = append(moves, *bestMove)
		}
	}

	// Deterministic canonical sorting by DriverID (Principle 2)
	sort.Slice(moves, func(i, j int) bool {
		if moves[i].DriverID == moves[j].DriverID {
			return moves[i].TargetRegionID < moves[j].TargetRegionID
		}
		return moves[i].DriverID < moves[j].DriverID
	})

	return moves, nil
}

// SummaryString formats a human-readable summary of the repositioning plan.
func SummaryString(moves []RepositioningMove) string {
	if len(moves) == 0 {
		return "No repositioning moves recommended."
	}
	totalCost := 0.0
	totalNet := 0.0
	for _, m := range moves {
		totalCost += m.EstimatedCost
		totalNet += m.NetRepositioningValue
	}
	return fmt.Sprintf("Recommended %d repositioning moves (Total Deadhead Cost: $%.2f, Expected Net Lift: $%.2f)",
		len(moves), totalCost, totalNet)
}
