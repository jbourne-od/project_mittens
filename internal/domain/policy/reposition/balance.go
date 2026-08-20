package reposition

import (
	"sort"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

// RegionalBalanceCalculator computes supply, demand, and shadow pricing across network regions.
type RegionalBalanceCalculator struct{}

// NewRegionalBalanceCalculator initializes a new RegionalBalanceCalculator.
func NewRegionalBalanceCalculator() *RegionalBalanceCalculator {
	return &RegionalBalanceCalculator{}
}

// ComputeBalance evaluates driver supply and load demand distributions across geographical regions.
func (c *RegionalBalanceCalculator) ComputeBalance(
	resource *model.ResourceState,
	regionMgr *model.RegionManager,
	defaultYields map[string]float64,
) map[string]RegionalBalanceSnapshot {
	if resource == nil || regionMgr == nil {
		return make(map[string]RegionalBalanceSnapshot)
	}

	driverCounts := make(map[string]int)
	for _, d := range resource.Drivers() {
		regID := regionMgr.GetRegionID(d.CurrentLocation)
		if regID != "" {
			driverCounts[regID]++
		}
	}

	tenderCounts := make(map[string]int)
	inboundCounts := make(map[string]int)
	for _, l := range resource.Loads() {
		origReg := regionMgr.GetRegionID(l.Origin)
		if origReg != "" {
			tenderCounts[origReg]++
		}
		destReg := regionMgr.GetRegionID(l.Destination)
		if destReg != "" {
			inboundCounts[destReg]++
		}
	}

	// Gather all referenced regions
	allRegionIDs := make(map[string]bool)
	for r := range driverCounts {
		allRegionIDs[r] = true
	}
	for r := range tenderCounts {
		allRegionIDs[r] = true
	}
	for r := range inboundCounts {
		allRegionIDs[r] = true
	}
	for r := range defaultYields {
		allRegionIDs[r] = true
	}

	result := make(map[string]RegionalBalanceSnapshot, len(allRegionIDs))

	for regID := range allRegionIDs {
		avail := driverCounts[regID]
		outbound := tenderCounts[regID]
		inbound := inboundCounts[regID]
		deficit := avail - outbound // > 0: oversupplied backhaul; < 0: undersupplied headhaul

		baseYield := 1500.0
		if y, ok := defaultYields[regID]; ok && y > 0 {
			baseYield = y
		}

		// Compute regional shadow price \lambda_r
		var shadowPrice float64
		if outbound > avail {
			// Severe shortage of capacity: incremental truck has elevated marginal value
			shortageRatio := float64(outbound-avail) / float64(max(1, outbound))
			shadowPrice = baseYield * (1.0 + 0.35*shortageRatio)
		} else if avail > outbound {
			// Oversupply of empty capacity: incremental truck has depressed marginal value
			surplusRatio := float64(avail-outbound) / float64(max(1, avail))
			decay := 1.0 - 0.40*surplusRatio
			if decay < 0.20 {
				decay = 0.20
			}
			shadowPrice = baseYield * decay
		} else {
			shadowPrice = baseYield
		}

		result[regID] = RegionalBalanceSnapshot{
			RegionID:             regID,
			AvailableDrivers:     avail,
			OutboundTenders:      outbound,
			Deficit:              deficit,
			ShadowPrice:          shadowPrice,
			InboundFlow:          inbound,
			AverageYieldPerTruck: baseYield,
		}
	}

	return result
}

// GetDeficitRegions returns region IDs with excess unassigned drivers (Deficit > hurdle), sorted by deficit descending.
func GetDeficitRegions(balances map[string]RegionalBalanceSnapshot, hurdle int) []string {
	var deficits []string
	for regID, snap := range balances {
		if snap.Deficit >= hurdle {
			deficits = append(deficits, regID)
		}
	}
	sort.Slice(deficits, func(i, j int) bool {
		return balances[deficits[i]].Deficit > balances[deficits[j]].Deficit
	})
	return deficits
}

// GetSurplusRegions returns region IDs with freight capacity shortages (Deficit < 0), sorted by shadow price descending.
func GetSurplusRegions(balances map[string]RegionalBalanceSnapshot) []string {
	var surplus []string
	for regID, snap := range balances {
		if snap.Deficit < 0 {
			surplus = append(surplus, regID)
		}
	}
	sort.Slice(surplus, func(i, j int) bool {
		return balances[surplus[i]].ShadowPrice > balances[surplus[j]].ShadowPrice
	})
	return surplus
}
