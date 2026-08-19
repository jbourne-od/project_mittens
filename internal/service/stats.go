package service

import (
	"math"

	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

// KPIReport represents an immutable snapshot of aggregate performance metrics.
// Replicates the reporting metrics produced by legacy StatisticCalculator.java.
type KPIReport struct {
	// Mileage Metrics
	TotalLoadedMiles float64 `json:"total_loaded_miles"`
	TotalEmptyMiles  float64 `json:"total_empty_miles"`
	TotalMiles       float64 `json:"total_miles"`
	EmptyRatio       float64 `json:"empty_ratio"` // EmptyMiles / TotalMiles

	// Time & Operations Metrics
	TotalDwellMinutes      int     `json:"total_dwell_minutes"`
	TotalInsertedRestMin   int     `json:"total_inserted_rest_minutes"`
	TotalLateMinutes       int     `json:"total_late_minutes"`
	LoadsOffered           int     `json:"loads_offered"`
	LoadsServiced          int     `json:"loads_serviced"`
	LoadsUnserviced        int     `json:"loads_unserviced"`
	ServicePercentage      float64 `json:"service_percentage"`
	OnTimePickupPercentage float64 `json:"on_time_pickup_percentage"`
	OnTimeDeliveryPercent  float64 `json:"on_time_delivery_percentage"`
	DriverWorkHours        float64 `json:"driver_work_hours"`
	DriverAvailableHours   float64 `json:"driver_available_hours"`
	DriverUtilization      float64 `json:"driver_utilization"`

	// Financial Metrics
	GrossRevenue         float64 `json:"gross_revenue"`
	TotalFixedCost       float64 `json:"total_fixed_cost"`
	TotalLoadedCost      float64 `json:"total_loaded_cost"`
	TotalEmptyCost       float64 `json:"total_empty_cost"`
	TotalEmptyToHomeCost float64 `json:"total_empty_to_home_cost"`
	TotalDwellCost       float64 `json:"total_dwell_cost"`
	TotalLatePenalty     float64 `json:"total_late_penalty"`
	TotalDriverBonus     float64 `json:"total_driver_bonus"`
	TotalCost            float64 `json:"total_cost"`
	NetContribution      float64 `json:"net_contribution"`

	// Unit Financial Averages
	RevenuePerLoadedMile float64 `json:"revenue_per_loaded_mile"`
	RevenuePerTotalMile  float64 `json:"revenue_per_total_mile"`
	CostPerTotalMile     float64 `json:"cost_per_total_mile"`
	ProfitPerTotalMile   float64 `json:"profit_per_total_mile"`
	ProfitPerLoadedMile  float64 `json:"profit_per_loaded_mile"`
}

// StatisticCalculator accumulates operational and financial metrics across simulation epochs.
//
// In accordance with Inviolate 6 (Lock-Free Hot Paths), instances are designed for sequential
// accumulation per simulation thread, and emit immutable KPIReport snapshots on demand.
type StatisticCalculator struct {
	totalLoadedMiles     float64
	totalEmptyMiles      float64
	totalDwellMinutes    int
	totalInsertedRestMin int
	totalLateMinutes     int

	loadsOffered     int
	loadsServiced    int
	loadsUnserviced  int
	onTimePickups    int
	onTimeDeliveries int

	driverWorkHours      float64
	driverAvailableHours float64

	grossRevenue         float64
	totalFixedCost       float64
	totalLoadedCost      float64
	totalEmptyCost       float64
	totalEmptyToHomeCost float64
	totalDwellCost       float64
	totalLatePenalty     float64
	totalDriverBonus     float64
	totalCost            float64
	netContribution      float64
}

// NewStatisticCalculator initializes an empty StatisticCalculator.
func NewStatisticCalculator() *StatisticCalculator {
	return &StatisticCalculator{}
}

// RecordLoadOffers records incoming customer load offers for service percentage tracking.
func (sc *StatisticCalculator) RecordLoadOffers(count int) {
	if count > 0 {
		sc.loadsOffered += count
	}
}

// RecordUnservicedLoads increments unserviced load count.
func (sc *StatisticCalculator) RecordUnservicedLoads(count int) {
	if count > 0 {
		sc.loadsUnserviced += count
	}
}

// RecordDriverHours records active working hours and total available fleet hours.
func (sc *StatisticCalculator) RecordDriverHours(workHours, availableHours float64) {
	sc.driverWorkHours += workHours
	sc.driverAvailableHours += availableHours
}

// RecordDispatch accumulates metrics from an executed driver-load assignment.
func (sc *StatisticCalculator) RecordDispatch(
	cost policy.TripCostBreakdown,
	loadedMiles, deadheadMiles float64,
	dwellMin, insertedRestMin int,
	onTimePickup, onTimeDelivery bool,
) {
	sc.loadsServiced++
	sc.totalLoadedMiles += loadedMiles
	sc.totalEmptyMiles += deadheadMiles
	sc.totalDwellMinutes += dwellMin
	sc.totalInsertedRestMin += insertedRestMin

	if onTimePickup {
		sc.onTimePickups++
	}
	if onTimeDelivery {
		sc.onTimeDeliveries++
	}

	sc.grossRevenue += cost.Revenue
	sc.totalFixedCost += cost.FixedCost
	sc.totalLoadedCost += cost.LoadedCost
	sc.totalEmptyCost += cost.EmptyCost
	sc.totalEmptyToHomeCost += cost.EmptyToHomeCost
	sc.totalDwellCost += cost.DwellCost
	sc.totalLatePenalty += cost.LatePenalty
	sc.totalDriverBonus += cost.DriverBonus
	sc.totalCost += cost.TotalCost
	sc.netContribution += cost.NetContribution
}

// Snapshot returns an immutable point-in-time KPIReport summarizing all accumulated metrics.
func (sc *StatisticCalculator) Snapshot() KPIReport {
	totMiles := sc.totalLoadedMiles + sc.totalEmptyMiles

	var emptyRatio float64
	if totMiles > 0 {
		emptyRatio = sc.totalEmptyMiles / totMiles
	}

	var servicePct float64
	if sc.loadsOffered > 0 {
		servicePct = (float64(sc.loadsServiced) / float64(sc.loadsOffered)) * 100.0
	}

	var onTimePickupPct float64
	var onTimeDeliveryPct float64
	if sc.loadsServiced > 0 {
		onTimePickupPct = (float64(sc.onTimePickups) / float64(sc.loadsServiced)) * 100.0
		onTimeDeliveryPct = (float64(sc.onTimeDeliveries) / float64(sc.loadsServiced)) * 100.0
	}

	var utilization float64
	if sc.driverAvailableHours > 0 {
		utilization = (sc.driverWorkHours / sc.driverAvailableHours) * 100.0
	}

	var revPerLoaded float64
	var revPerTotal float64
	var costPerTotal float64
	var profitPerTotal float64
	var profitPerLoaded float64

	if sc.totalLoadedMiles > 0 {
		revPerLoaded = sc.grossRevenue / sc.totalLoadedMiles
		profitPerLoaded = sc.netContribution / sc.totalLoadedMiles
	}
	if totMiles > 0 {
		revPerTotal = sc.grossRevenue / totMiles
		costPerTotal = sc.totalCost / totMiles
		profitPerTotal = sc.netContribution / totMiles
	}

	return KPIReport{
		TotalLoadedMiles:       roundFloat(sc.totalLoadedMiles, 2),
		TotalEmptyMiles:        roundFloat(sc.totalEmptyMiles, 2),
		TotalMiles:             roundFloat(totMiles, 2),
		EmptyRatio:             roundFloat(emptyRatio, 4),
		TotalDwellMinutes:      sc.totalDwellMinutes,
		TotalInsertedRestMin:   sc.totalInsertedRestMin,
		TotalLateMinutes:       sc.totalLateMinutes,
		LoadsOffered:           sc.loadsOffered,
		LoadsServiced:          sc.loadsServiced,
		LoadsUnserviced:        sc.loadsUnserviced,
		ServicePercentage:      roundFloat(servicePct, 2),
		OnTimePickupPercentage: roundFloat(onTimePickupPct, 2),
		OnTimeDeliveryPercent:  roundFloat(onTimeDeliveryPct, 2),
		DriverWorkHours:        roundFloat(sc.driverWorkHours, 2),
		DriverAvailableHours:   roundFloat(sc.driverAvailableHours, 2),
		DriverUtilization:      roundFloat(utilization, 2),
		GrossRevenue:           roundFloat(sc.grossRevenue, 2),
		TotalFixedCost:         roundFloat(sc.totalFixedCost, 2),
		TotalLoadedCost:        roundFloat(sc.totalLoadedCost, 2),
		TotalEmptyCost:         roundFloat(sc.totalEmptyCost, 2),
		TotalEmptyToHomeCost:   roundFloat(sc.totalEmptyToHomeCost, 2),
		TotalDwellCost:         roundFloat(sc.totalDwellCost, 2),
		TotalLatePenalty:       roundFloat(sc.totalLatePenalty, 2),
		TotalDriverBonus:       roundFloat(sc.totalDriverBonus, 2),
		TotalCost:              roundFloat(sc.totalCost, 2),
		NetContribution:        roundFloat(sc.netContribution, 2),
		RevenuePerLoadedMile:   roundFloat(revPerLoaded, 4),
		RevenuePerTotalMile:    roundFloat(revPerTotal, 4),
		CostPerTotalMile:       roundFloat(costPerTotal, 4),
		ProfitPerTotalMile:     roundFloat(profitPerTotal, 4),
		ProfitPerLoadedMile:    roundFloat(profitPerLoaded, 4),
	}
}

func roundFloat(val float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(val*pow) / pow
}
