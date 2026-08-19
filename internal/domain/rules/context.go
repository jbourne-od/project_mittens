package rules

import (
	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

// BuildEvaluationContext constructs a structured CEL variable environment from domain entities.
func BuildEvaluationContext(
	driver model.Driver,
	load model.Load,
	deadheadMiles, loadedMiles float64,
) map[string]any {
	driverEndorsements := make([]string, len(driver.Equipment.Endorsements))
	for i, e := range driver.Equipment.Endorsements {
		driverEndorsements[i] = string(e)
	}

	loadEndorsements := make([]string, len(load.RequiredEndorsements))
	for i, e := range load.RequiredEndorsements {
		loadEndorsements[i] = string(e)
	}

	driverMap := map[string]any{
		"id":                    driver.ID,
		"equipment_type":        string(driver.Equipment.Type),
		"current_location_id":   driver.CurrentLocation.NodeID,
		"current_lat":           driver.CurrentLocation.Lat,
		"current_lon":           driver.CurrentLocation.Lon,
		"available_epoch":       driver.AvailableEpoch,
		"drive_hours_remaining": driver.DriveHoursRemaining,
		"duty_hours_remaining":  driver.DutyHoursRemaining,
		"endorsements":          driverEndorsements,
	}

	loadMap := map[string]any{
		"id":                      load.ID,
		"origin_id":               load.Origin.NodeID,
		"origin_lat":              load.Origin.Lat,
		"origin_lon":              load.Origin.Lon,
		"destination_id":          load.Destination.NodeID,
		"destination_lat":         load.Destination.Lat,
		"destination_lon":         load.Destination.Lon,
		"required_equipment":      string(load.RequiredEquipment),
		"required_endorsements":   loadEndorsements,
		"revenue":                 load.Revenue,
		"pickup_earliest_epoch":   load.PickupEarliestEpoch,
		"pickup_latest_epoch":     load.PickupLatestEpoch,
		"delivery_earliest_epoch": load.DeliveryEarliestEpoch,
		"delivery_latest_epoch":   load.DeliveryLatestEpoch,
	}

	arcMap := map[string]any{
		"deadhead_miles": deadheadMiles,
		"loaded_miles":   loadedMiles,
		"total_miles":    deadheadMiles + loadedMiles,
	}

	return map[string]any{
		"driver": driverMap,
		"load":   loadMap,
		"arc":    arcMap,
	}
}
