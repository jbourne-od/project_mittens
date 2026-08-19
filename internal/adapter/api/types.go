package api

// LocationDTO represents geographic coordinates for API payloads.
type LocationDTO struct {
	NodeID string  `json:"node_id"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
}

// EquipmentDTO represents trailer/equipment specifications.
type EquipmentDTO struct {
	Type string `json:"type"`
}

// DriverDTO represents driver resource inputs for optimization requests.
type DriverDTO struct {
	ID                  string       `json:"id"`
	CurrentLocation     LocationDTO  `json:"current_location"`
	HomeLocation        LocationDTO  `json:"home_location"`
	AvailableEpoch      int64        `json:"available_epoch"`
	DriveHoursRemaining float64      `json:"drive_hours_remaining,omitempty"`
	DutyHoursRemaining  float64      `json:"duty_hours_remaining,omitempty"`
	Equipment           EquipmentDTO `json:"equipment,omitempty"`
}

// LoadDTO represents freight shipments in optimization requests.
type LoadDTO struct {
	ID                    string      `json:"id"`
	Origin                LocationDTO `json:"origin"`
	Destination           LocationDTO `json:"destination"`
	PickupEarliestEpoch   int64       `json:"pickup_earliest_epoch"`
	PickupLatestEpoch     int64       `json:"pickup_latest_epoch"`
	DeliveryEarliestEpoch int64       `json:"delivery_earliest_epoch"`
	DeliveryLatestEpoch   int64       `json:"delivery_latest_epoch"`
	Revenue               float64     `json:"revenue"`
	RequiredEquipment     string      `json:"required_equipment,omitempty"`
}

// FacilityDTO represents physical intermediate relay hubs and customer DCs.
type FacilityDTO struct {
	ID                  string      `json:"id"`
	Name                string      `json:"name,omitempty"`
	Location            LocationDTO `json:"location"`
	Type                string      `json:"type"`
	AverageDwellMinutes int         `json:"average_dwell_minutes,omitempty"`
}

// OptimizeRequest defines the payload for single-epoch matching optimization.
type OptimizeRequest struct {
	Epoch           int64       `json:"epoch"`
	Drivers         []DriverDTO `json:"drivers"`
	Loads           []LoadDTO   `json:"loads"`
	PolicyClass     string      `json:"policy_class,omitempty"`
	CompetitorScale int         `json:"competitor_scale,omitempty"`
}

// MatchDTO represents an individual driver-load assignment in the response.
type MatchDTO struct {
	DriverID              string  `json:"driver_id"`
	LoadID                string  `json:"load_id"`
	DispatchEpoch         int64   `json:"dispatch_epoch"`
	EstimatedContribution float64 `json:"estimated_contribution"`
}

// OptimizeResponse defines the response structure for dispatch optimization.
type OptimizeResponse struct {
	RunID                string     `json:"run_id"`
	Epoch                int64      `json:"epoch"`
	MatchCount           int        `json:"match_count"`
	Matches              []MatchDTO `json:"matches"`
	TotalNetContribution float64    `json:"total_net_contribution"`
	ExecutionDurationMs  float64    `json:"execution_duration_ms"`
}

// SimulateRequest defines the payload for multi-day continuous rolling simulation.
type SimulateRequest struct {
	RunID             string        `json:"run_id"`
	StartEpoch        int64         `json:"start_epoch"`
	HorizonDays       int           `json:"horizon_days"`
	DecisionStepHours int           `json:"decision_step_hours,omitempty"`
	EnableRelays      bool          `json:"enable_relays,omitempty"`
	MinRelayHaulMiles float64       `json:"min_relay_haul_miles,omitempty"`
	Drivers           []DriverDTO   `json:"drivers"`
	Facilities        []FacilityDTO `json:"facilities,omitempty"`
	LoadSchedule      []LoadDTO     `json:"load_schedule"`
}

// DailyKPISnapshotDTO represents daily operational metrics in simulation responses.
type DailyKPISnapshotDTO struct {
	DayIndex           int     `json:"day_index"`
	Epoch              int64   `json:"epoch"`
	ActiveDrivers      int     `json:"active_drivers"`
	TotalLoadedMiles   float64 `json:"total_loaded_miles"`
	TotalEmptyMiles    float64 `json:"total_empty_miles"`
	EmptyMileRatio     float64 `json:"empty_mile_ratio"`
	GrossRevenue       float64 `json:"gross_revenue"`
	TotalCost          float64 `json:"total_cost"`
	NetContribution    float64 `json:"net_contribution"`
	DirectTourCount    int     `json:"direct_tour_count"`
	RelayExchangeCount int     `json:"relay_exchange_count"`
}

// SimulateResponse defines the response structure for rolling horizon simulation.
type SimulateResponse struct {
	RunID                     string                `json:"run_id"`
	TotalDays                 int                   `json:"total_days"`
	TotalEpochs               int                   `json:"total_epochs"`
	CumulativeLoadedMiles     float64               `json:"cumulative_loaded_miles"`
	CumulativeEmptyMiles      float64               `json:"cumulative_empty_miles"`
	OverallEmptyRatio         float64               `json:"overall_empty_ratio"`
	CumulativeGrossRevenue    float64               `json:"cumulative_gross_revenue"`
	CumulativeCost            float64               `json:"cumulative_cost"`
	CumulativeNetContribution float64               `json:"cumulative_net_contribution"`
	DailyKPIs                 []DailyKPISnapshotDTO `json:"daily_kpis"`
}

// HealthResponse represents service health status.
type HealthResponse struct {
	Status        string  `json:"status"`
	Version       string  `json:"version"`
	UptimeSeconds float64 `json:"uptime_seconds"`
}

// ErrorResponse represents standard error payload format.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
