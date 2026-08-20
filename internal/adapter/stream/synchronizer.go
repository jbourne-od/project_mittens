package stream

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
)

// SyncReport summarizes the changes applied during a streaming state synchronization cycle.
type SyncReport struct {
	DriversUpdated int   `json:"drivers_updated"`
	DriversAdded   int   `json:"drivers_added"`
	LoadsAdded     int   `json:"loads_added"`
	LoadsCanceled  int   `json:"loads_canceled"`
	TotalDrivers   int   `json:"total_drivers"`
	TotalLoads     int   `json:"total_loads"`
	SyncTimestamp  int64 `json:"sync_timestamp"`
}

// StateSynchronizer merges accumulated streaming buffer events into freshly allocated domain ResourceStates.
type StateSynchronizer struct {
	buffer *StreamBuffer
}

// NewStateSynchronizer constructs a StateSynchronizer bound to the given StreamBuffer.
func NewStateSynchronizer(buffer *StreamBuffer) *StateSynchronizer {
	if buffer == nil {
		buffer = NewStreamBuffer()
	}
	return &StateSynchronizer{
		buffer: buffer,
	}
}

// Synchronize drains the buffer and constructs an updated, immutable ResourceState from baseResource (Inviolate 5).
func (s *StateSynchronizer) Synchronize(baseResource *model.ResourceState, currentEpoch int64) (*model.ResourceState, SyncReport, error) {
	pings, tenders, cancels := s.buffer.Drain()

	report := SyncReport{
		SyncTimestamp: time.Now().Unix(),
	}

	driverMap := make(map[string]model.Driver)
	if baseResource != nil {
		for _, d := range baseResource.Drivers() {
			driverMap[d.ID] = d
		}
	}

	// 1. Apply Driver Telemetry Updates
	for driverID, ping := range pings {
		if existing, exists := driverMap[driverID]; exists {
			// Update existing driver
			loc := existing.CurrentLocation
			loc.Lat = ping.Lat
			loc.Lon = ping.Lon
			if ping.NodeID != "" {
				loc.NodeID = ping.NodeID
			}

			availEpoch := existing.AvailableEpoch
			if ping.PTAEpoch > 0 {
				availEpoch = ping.PTAEpoch
			} else if currentEpoch > 0 && availEpoch < currentEpoch {
				availEpoch = currentEpoch
			}

			updatedDriver := model.Driver{
				ID:                  existing.ID,
				CurrentLocation:     loc,
				HomeLocation:        existing.HomeLocation,
				AvailableEpoch:      availEpoch,
				DriveHoursRemaining: ping.DriveHoursRemaining,
				DutyHoursRemaining:  ping.DutyHoursRemaining,
				AssignedLoadID:      existing.AssignedLoadID,
				Equipment:           existing.Equipment,
				Endorsements:        existing.Endorsements,
				Clocks:              existing.Clocks,
			}

			if ping.AssignedLoadID != "" {
				updatedDriver.AssignedLoadID = ping.AssignedLoadID
			}
			if ping.EquipmentType != "" {
				updatedDriver.Equipment = model.Equipment{Type: parseEquipment(ping.EquipmentType)}
			}
			if len(ping.Endorsements) > 0 {
				ends := make([]model.Endorsement, len(ping.Endorsements))
				for i, e := range ping.Endorsements {
					ends[i] = model.Endorsement(strings.ToUpper(strings.TrimSpace(e)))
				}
				updatedDriver.Endorsements = ends
			}

			driverMap[driverID] = updatedDriver
			report.DriversUpdated++
		} else {
			// Ingest new driver discovered in live telemetry
			nodeID := ping.NodeID
			if nodeID == "" {
				nodeID = fmt.Sprintf("GPS_%.4f_%.4f", ping.Lat, ping.Lon)
			}
			loc := model.Location{
				NodeID: nodeID,
				Lat:    ping.Lat,
				Lon:    ping.Lon,
			}
			availEpoch := ping.PTAEpoch
			if availEpoch <= 0 {
				availEpoch = currentEpoch
			}
			if availEpoch <= 0 {
				availEpoch = time.Now().Unix()
			}

			ends := make([]model.Endorsement, len(ping.Endorsements))
			for i, e := range ping.Endorsements {
				ends[i] = model.Endorsement(strings.ToUpper(strings.TrimSpace(e)))
			}

			newDriver := model.Driver{
				ID:                  driverID,
				CurrentLocation:     loc,
				HomeLocation:        loc,
				AvailableEpoch:      availEpoch,
				DriveHoursRemaining: ping.DriveHoursRemaining,
				DutyHoursRemaining:  ping.DutyHoursRemaining,
				AssignedLoadID:      ping.AssignedLoadID,
				Equipment:           model.Equipment{Type: parseEquipment(ping.EquipmentType)},
				Endorsements:        ends,
				Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(availEpoch, 0)),
			}
			driverMap[driverID] = newDriver
			report.DriversAdded++
		}
	}

	// 2. Apply Load Tender Updates and Cancellations
	loadMap := make(map[string]model.Load)
	if baseResource != nil {
		for _, l := range baseResource.Loads() {
			loadMap[l.ID] = l
		}
	}

	// Process cancellations
	for loadID := range cancels {
		if _, exists := loadMap[loadID]; exists {
			delete(loadMap, loadID)
			report.LoadsCanceled++
		}
	}

	// Process new tenders
	for loadID, tender := range tenders {
		origNode := tender.OriginNodeID
		if origNode == "" {
			origNode = fmt.Sprintf("ORIG_%.4f_%.4f", tender.OriginLat, tender.OriginLon)
		}
		destNode := tender.DestinationNodeID
		if destNode == "" {
			destNode = fmt.Sprintf("DEST_%.4f_%.4f", tender.DestLat, tender.DestLon)
		}

		ends := make([]model.Endorsement, len(tender.RequiredEndorsements))
		for i, e := range tender.RequiredEndorsements {
			ends[i] = model.Endorsement(strings.ToUpper(strings.TrimSpace(e)))
		}

		newLoad := model.Load{
			ID: loadID,
			Origin: model.Location{
				NodeID: origNode,
				Lat:    tender.OriginLat,
				Lon:    tender.OriginLon,
			},
			Destination: model.Location{
				NodeID: destNode,
				Lat:    tender.DestLat,
				Lon:    tender.DestLon,
			},
			PickupEarliestEpoch:   tender.PickupEarliestEpoch,
			PickupLatestEpoch:     tender.PickupLatestEpoch,
			DeliveryEarliestEpoch: tender.DeliveryEarliestEpoch,
			DeliveryLatestEpoch:   tender.DeliveryLatestEpoch,
			Revenue:               tender.Revenue,
			RequiredEquipment:     parseEquipment(tender.RequiredEquipment),
			RequiredEndorsements:  ends,
		}

		loadMap[loadID] = newLoad
		report.LoadsAdded++
	}

	// 3. Assemble Canonical Driver and Load Slices
	drivers := make([]model.Driver, 0, len(driverMap))
	for _, d := range driverMap {
		drivers = append(drivers, d)
	}
	sort.Slice(drivers, func(i, j int) bool {
		return drivers[i].ID < drivers[j].ID
	})

	loads := make([]model.Load, 0, len(loadMap))
	for _, l := range loadMap {
		loads = append(loads, l)
	}
	sort.Slice(loads, func(i, j int) bool {
		return loads[i].ID < loads[j].ID
	})

	report.TotalDrivers = len(drivers)
	report.TotalLoads = len(loads)

	// Return newly allocated immutable ResourceState
	return model.NewResourceState(drivers, loads), report, nil
}

func parseEquipment(eqStr string) model.EquipmentType {
	switch strings.ToUpper(strings.TrimSpace(eqStr)) {
	case "REEFER", "REEFER_53", "R":
		return model.EquipReefer
	case "FLATBED", "FLATBED_53", "FB":
		return model.EquipFlatbed
	case "TANKER":
		return model.EquipTanker
	default:
		return model.EquipDryVan
	}
}
