package reposition

import (
	"errors"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

var (
	// ErrInvalidConfig is returned when repositioning configuration parameters are out of physical bounds.
	ErrInvalidConfig = errors.New("reposition: invalid repositioning configuration")
)

// RegionalBalanceSnapshot captures the supply, demand, and shadow pricing status of a geographical freight region.
type RegionalBalanceSnapshot struct {
	RegionID             string  `json:"region_id"`
	AvailableDrivers     int     `json:"available_drivers"`
	OutboundTenders      int     `json:"outbound_tenders"`
	Deficit              int     `json:"deficit"`      // Deficit = AvailableDrivers - OutboundTenders (>0 indicates truck surplus/backhaul)
	ShadowPrice          float64 `json:"shadow_price"` // Marginal value ($) of an incremental driver stationed in this region
	InboundFlow          int     `json:"inbound_flow"` // Count of loaded trucks currently en-route to this region
	AverageYieldPerTruck float64 `json:"average_yield_per_truck"`
}

// RepositioningMove represents an autonomous empty tractor repositioning action from a deficit market to a high-yield cluster.
type RepositioningMove struct {
	DriverID               string         `json:"driver_id"`
	OriginLocation         model.Location `json:"origin_location"`
	TargetRegionID         string         `json:"target_region_id"`
	TargetLocation         model.Location `json:"target_location"`
	StartEpoch             int64          `json:"start_epoch"`
	ArrivalEpoch           int64          `json:"arrival_epoch"`
	DeadheadMiles          float64        `json:"deadhead_miles"`
	EstimatedCost          float64        `json:"estimated_cost"`
	ExpectedArbitrageYield float64        `json:"expected_arbitrage_yield"`
	NetRepositioningValue  float64        `json:"net_repositioning_value"`
}

// RepositioningConfig encapsulates hyperparameters and physical limits for fleet repositioning.
type RepositioningConfig struct {
	// MaxRepositionDistanceMiles is the maximum allowable empty relocation distance (e.g. 450 miles).
	MaxRepositionDistanceMiles float64 `json:"max_reposition_distance_miles"`

	// EmptyMileCostRate is the operating cost per empty mile ($/mile, e.g. 1.85).
	EmptyMileCostRate float64 `json:"empty_mile_cost_rate"`

	// MinArbitrageThreshold is the minimum required expected net profit lift ($) to authorize repositioning.
	MinArbitrageThreshold float64 `json:"min_arbitrage_threshold"`

	// DeficitHurdle is the minimum excess driver count in a region before triggering outbound relocation.
	DeficitHurdle int `json:"deficit_hurdle"`

	// AverageTransitSpeedMPH is the assumed cruising speed for transit duration estimation (e.g. 50.0 mph).
	AverageTransitSpeedMPH float64 `json:"average_transit_speed_mph"`

	// DefaultRegionalYield maps region IDs to baseline expected outbound load yields ($/load).
	DefaultRegionalYield map[string]float64 `json:"default_regional_yield,omitempty"`
}

// DefaultRepositioningConfig returns production defaults for fleet repositioning optimization.
func DefaultRepositioningConfig() RepositioningConfig {
	return RepositioningConfig{
		MaxRepositionDistanceMiles: 450.0,
		EmptyMileCostRate:          1.85,
		MinArbitrageThreshold:      150.0,
		DeficitHurdle:              1,
		AverageTransitSpeedMPH:     50.0,
		DefaultRegionalYield: map[string]float64{
			"CHI": 1800.0,
			"ATL": 1650.0,
			"DAL": 1900.0,
			"CLT": 1550.0,
			"IND": 1700.0,
			"KC":  1750.0,
			"MIA": 900.0, // Classic Florida backhaul dead-zone
		},
	}
}

// Validate validates physical bounds on RepositioningConfig fields.
func (cfg RepositioningConfig) Validate() error {
	if cfg.MaxRepositionDistanceMiles <= 0 {
		return ErrInvalidConfig
	}
	if cfg.EmptyMileCostRate <= 0 {
		return ErrInvalidConfig
	}
	if cfg.AverageTransitSpeedMPH <= 0 {
		return ErrInvalidConfig
	}
	return nil
}
