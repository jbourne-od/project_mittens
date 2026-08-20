// Package journal implements lossless cryptographic state journaling,
// deterministic Merkle hash chaining, and replay audit logs for Project Mittens.
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /pkg/math
//   - Imported By: /internal/service, /pkg/replay
//   - Strict Rule: Pure execution, deterministic serialization, zero ambient I/O.
package journal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
)

// ComputeSHA256 returns the lowercase hexadecimal SHA-256 hash of a byte slice.
func ComputeSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// CanonicalDriverDTO represents a driver state in canonical serializable order.
type CanonicalDriverDTO struct {
	ID                  string   `json:"id"`
	CurrentLocationNode string   `json:"current_location_node"`
	CurrentLat          float64  `json:"current_lat"`
	CurrentLon          float64  `json:"current_lon"`
	HomeLocationNode    string   `json:"home_location_node"`
	HomeLat             float64  `json:"home_lat"`
	HomeLon             float64  `json:"home_lon"`
	AvailableEpoch      int64    `json:"available_epoch"`
	DriveHoursRemaining float64  `json:"drive_hours_remaining"`
	DutyHoursRemaining  float64  `json:"duty_hours_remaining"`
	AssignedLoadID      string   `json:"assigned_load_id,omitempty"`
	EquipmentType       string   `json:"equipment_type"`
	Endorsements        []string `json:"endorsements,omitempty"`
}

// CanonicalLoadDTO represents a load state in canonical serializable order.
type CanonicalLoadDTO struct {
	ID                    string   `json:"id"`
	OriginNode            string   `json:"origin_node"`
	OriginLat             float64  `json:"origin_lat"`
	OriginLon             float64  `json:"origin_lon"`
	DestinationNode       string   `json:"destination_node"`
	DestLat               float64  `json:"dest_lat"`
	DestLon               float64  `json:"dest_lon"`
	PickupEarliestEpoch   int64    `json:"pickup_earliest_epoch"`
	PickupLatestEpoch     int64    `json:"pickup_latest_epoch"`
	DeliveryEarliestEpoch int64    `json:"delivery_earliest_epoch"`
	DeliveryLatestEpoch   int64    `json:"delivery_latest_epoch"`
	Revenue               float64  `json:"revenue"`
	RequiredEquipment     string   `json:"required_equipment"`
	RequiredEndorsements  []string `json:"required_endorsements,omitempty"`
}

// CanonicalResourceDTO encapsulates the sorted canonical resource state.
type CanonicalResourceDTO struct {
	Drivers []CanonicalDriverDTO `json:"drivers"`
	Loads   []CanonicalLoadDTO   `json:"loads"`
}

// CanonicalInformationDTO captures the information state in canonical serializable order.
type CanonicalInformationDTO struct {
	Epoch             int64   `json:"epoch"`
	SpotRateIndex     float64 `json:"spot_rate_index"`
	FuelPriceIndex    float64 `json:"fuel_price_index"`
	WeatherAlertCount int     `json:"weather_alert_count"`
}

// CanonicalBeliefDTO captures the posterior belief simplex in canonical key order.
type CanonicalBeliefDTO struct {
	CompetitorDimension int                `json:"competitor_dimension"`
	Probabilities       map[string]float64 `json:"probabilities"`
}

// CanonicalMatchDTO captures a matched dispatch pairing.
type CanonicalMatchDTO struct {
	DriverID string `json:"driver_id"`
	LoadID   string `json:"load_id"`
}

// CanonicalSpotBidDTO captures an endogenous spot rate bid.
type CanonicalSpotBidDTO struct {
	LoadID   string  `json:"load_id"`
	BidPrice float64 `json:"bid_price"`
}

// CanonicalActionDTO encapsulates the sorted canonical decision action.
type CanonicalActionDTO struct {
	Matches []CanonicalMatchDTO   `json:"matches"`
	Bids    []CanonicalSpotBidDTO `json:"bids"`
}

// EncodeCanonicalResource serializes a ResourceState into deterministic canonical JSON bytes.
func EncodeCanonicalResource(res *model.ResourceState) ([]byte, string, error) {
	if res == nil {
		return nil, "", fmt.Errorf("journal: cannot encode nil resource state")
	}

	drivers := res.Drivers()
	driverDTOs := make([]CanonicalDriverDTO, len(drivers))
	for i, d := range drivers {
		var ends []string
		if len(d.Endorsements) > 0 {
			ends = make([]string, len(d.Endorsements))
			for idx, e := range d.Endorsements {
				ends[idx] = string(e)
			}
			sort.Strings(ends)
		}

		driverDTOs[i] = CanonicalDriverDTO{
			ID:                  d.ID,
			CurrentLocationNode: d.CurrentLocation.NodeID,
			CurrentLat:          d.CurrentLocation.Lat,
			CurrentLon:          d.CurrentLocation.Lon,
			HomeLocationNode:    d.HomeLocation.NodeID,
			HomeLat:             d.HomeLocation.Lat,
			HomeLon:             d.HomeLocation.Lon,
			AvailableEpoch:      d.AvailableEpoch,
			DriveHoursRemaining: d.DriveHoursRemaining,
			DutyHoursRemaining:  d.DutyHoursRemaining,
			AssignedLoadID:      d.AssignedLoadID,
			EquipmentType:       string(d.Equipment.Type),
			Endorsements:        ends,
		}
	}
	sort.Slice(driverDTOs, func(i, j int) bool {
		return driverDTOs[i].ID < driverDTOs[j].ID
	})

	loads := res.Loads()
	loadDTOs := make([]CanonicalLoadDTO, len(loads))
	for i, l := range loads {
		var reqEnds []string
		if len(l.RequiredEndorsements) > 0 {
			reqEnds = make([]string, len(l.RequiredEndorsements))
			for idx, e := range l.RequiredEndorsements {
				reqEnds[idx] = string(e)
			}
			sort.Strings(reqEnds)
		}

		loadDTOs[i] = CanonicalLoadDTO{
			ID:                    l.ID,
			OriginNode:            l.Origin.NodeID,
			OriginLat:             l.Origin.Lat,
			OriginLon:             l.Origin.Lon,
			DestinationNode:       l.Destination.NodeID,
			DestLat:               l.Destination.Lat,
			DestLon:               l.Destination.Lon,
			PickupEarliestEpoch:   l.PickupEarliestEpoch,
			PickupLatestEpoch:     l.PickupLatestEpoch,
			DeliveryEarliestEpoch: l.DeliveryEarliestEpoch,
			DeliveryLatestEpoch:   l.DeliveryLatestEpoch,
			Revenue:               l.Revenue,
			RequiredEquipment:     string(l.RequiredEquipment),
			RequiredEndorsements:  reqEnds,
		}
	}
	sort.Slice(loadDTOs, func(i, j int) bool {
		return loadDTOs[i].ID < loadDTOs[j].ID
	})

	dto := CanonicalResourceDTO{
		Drivers: driverDTOs,
		Loads:   loadDTOs,
	}

	data, err := json.Marshal(dto)
	if err != nil {
		return nil, "", fmt.Errorf("journal: failed to marshal resource state: %w", err)
	}

	return data, ComputeSHA256(data), nil
}

// EncodeCanonicalInformation serializes an InformationState into deterministic canonical JSON bytes.
func EncodeCanonicalInformation(info *model.InformationState) ([]byte, string, error) {
	if info == nil {
		return nil, "", fmt.Errorf("journal: cannot encode nil information state")
	}

	dto := CanonicalInformationDTO{
		Epoch:             info.Epoch(),
		SpotRateIndex:     info.NationalSpotRateIndex(),
		FuelPriceIndex:    info.FuelPriceIndex(),
		WeatherAlertCount: info.WeatherAlertCount(),
	}

	data, err := json.Marshal(dto)
	if err != nil {
		return nil, "", fmt.Errorf("journal: failed to marshal information state: %w", err)
	}

	return data, ComputeSHA256(data), nil
}

// EncodeCanonicalBelief serializes a Belief into deterministic canonical JSON bytes.
func EncodeCanonicalBelief[C model.CompetitorScale](belief *model.Belief[C]) ([]byte, string, error) {
	if belief == nil {
		return nil, "", fmt.Errorf("journal: cannot encode nil belief state")
	}

	keys := belief.StateKeys()
	sort.Strings(keys)

	probs := make(map[string]float64, len(keys))
	for _, k := range keys {
		probs[k] = belief.Probability(k)
	}

	dto := CanonicalBeliefDTO{
		CompetitorDimension: belief.Scale().CompetitorDimension(),
		Probabilities:       probs,
	}

	data, err := json.Marshal(dto)
	if err != nil {
		return nil, "", fmt.Errorf("journal: failed to marshal belief state: %w", err)
	}

	return data, ComputeSHA256(data), nil
}

// EncodeCanonicalAction serializes an Action into deterministic canonical JSON bytes.
func EncodeCanonicalAction(action *model.Action) ([]byte, string, error) {
	if action == nil {
		return nil, "", fmt.Errorf("journal: cannot encode nil action")
	}

	matches := action.Matches()
	matchDTOs := make([]CanonicalMatchDTO, len(matches))
	for i, m := range matches {
		matchDTOs[i] = CanonicalMatchDTO{
			DriverID: m.DriverID,
			LoadID:   m.LoadID,
		}
	}
	sort.Slice(matchDTOs, func(i, j int) bool {
		if matchDTOs[i].DriverID == matchDTOs[j].DriverID {
			return matchDTOs[i].LoadID < matchDTOs[j].LoadID
		}
		return matchDTOs[i].DriverID < matchDTOs[j].DriverID
	})

	bids := action.Bids()
	bidDTOs := make([]CanonicalSpotBidDTO, len(bids))
	for i, b := range bids {
		bidDTOs[i] = CanonicalSpotBidDTO{
			LoadID:   b.LoadID,
			BidPrice: b.BidPrice,
		}
	}
	sort.Slice(bidDTOs, func(i, j int) bool {
		return bidDTOs[i].LoadID < bidDTOs[j].LoadID
	})

	dto := CanonicalActionDTO{
		Matches: matchDTOs,
		Bids:    bidDTOs,
	}

	data, err := json.Marshal(dto)
	if err != nil {
		return nil, "", fmt.Errorf("journal: failed to marshal action: %w", err)
	}

	return data, ComputeSHA256(data), nil
}

// HashState returns the combined SHA-256 hash over S_t = (R_t, I_t, b_t).
func HashState[C model.CompetitorScale](state *model.State[C]) (string, error) {
	if state == nil {
		return "", fmt.Errorf("journal: cannot hash nil state")
	}

	_, rHash, err := EncodeCanonicalResource(state.Resource())
	if err != nil {
		return "", err
	}
	_, iHash, err := EncodeCanonicalInformation(state.Information())
	if err != nil {
		return "", err
	}
	_, bHash, err := EncodeCanonicalBelief(state.Belief())
	if err != nil {
		return "", err
	}

	combined := fmt.Sprintf("%s:%s:%s", rHash, iHash, bHash)
	return ComputeSHA256([]byte(combined)), nil
}

// HashParameters computes a canonical SHA-256 digest over arbitrary configuration parameters.
func HashParameters(params any) (string, error) {
	if params == nil {
		return ComputeSHA256([]byte("nil")), nil
	}
	data, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("journal: failed to marshal parameters for hashing: %w", err)
	}
	return ComputeSHA256(data), nil
}

// DecodeCanonicalResource decodes a CanonicalResourceDTO JSON byte slice into a domain ResourceState.
func DecodeCanonicalResource(data []byte) (*model.ResourceState, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("journal: empty resource state data")
	}
	var dto CanonicalResourceDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return nil, fmt.Errorf("journal: failed unmarshaling canonical resource: %w", err)
	}

	drivers := make([]model.Driver, len(dto.Drivers))
	for i, d := range dto.Drivers {
		var ends []model.Endorsement
		if len(d.Endorsements) > 0 {
			ends = make([]model.Endorsement, len(d.Endorsements))
			for idx, e := range d.Endorsements {
				ends[idx] = model.Endorsement(e)
			}
		}
		drivers[i] = model.Driver{
			ID: d.ID,
			CurrentLocation: model.Location{
				NodeID: d.CurrentLocationNode,
				Lat:    d.CurrentLat,
				Lon:    d.CurrentLon,
			},
			HomeLocation: model.Location{
				NodeID: d.HomeLocationNode,
				Lat:    d.HomeLat,
				Lon:    d.HomeLon,
			},
			AvailableEpoch:      d.AvailableEpoch,
			DriveHoursRemaining: d.DriveHoursRemaining,
			DutyHoursRemaining:  d.DutyHoursRemaining,
			AssignedLoadID:      d.AssignedLoadID,
			Equipment:           model.Equipment{Type: model.EquipmentType(d.EquipmentType)},
			Endorsements:        ends,
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(d.AvailableEpoch, 0).UTC()),
		}
	}

	loads := make([]model.Load, len(dto.Loads))
	for i, l := range dto.Loads {
		var reqEnds []model.Endorsement
		if len(l.RequiredEndorsements) > 0 {
			reqEnds = make([]model.Endorsement, len(l.RequiredEndorsements))
			for idx, e := range l.RequiredEndorsements {
				reqEnds[idx] = model.Endorsement(e)
			}
		}
		loads[i] = model.Load{
			ID: l.ID,
			Origin: model.Location{
				NodeID: l.OriginNode,
				Lat:    l.OriginLat,
				Lon:    l.OriginLon,
			},
			Destination: model.Location{
				NodeID: l.DestinationNode,
				Lat:    l.DestLat,
				Lon:    l.DestLon,
			},
			PickupEarliestEpoch:   l.PickupEarliestEpoch,
			PickupLatestEpoch:     l.PickupLatestEpoch,
			DeliveryEarliestEpoch: l.DeliveryEarliestEpoch,
			DeliveryLatestEpoch:   l.DeliveryLatestEpoch,
			Revenue:               l.Revenue,
			RequiredEquipment:     model.EquipmentType(l.RequiredEquipment),
			RequiredEndorsements:  reqEnds,
		}
	}

	return model.NewResourceState(drivers, loads), nil
}

// DecodeCanonicalInformation decodes a CanonicalInformationDTO JSON byte slice into a domain InformationState.
func DecodeCanonicalInformation(data []byte) (*model.InformationState, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("journal: empty information state data")
	}
	var dto CanonicalInformationDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return nil, fmt.Errorf("journal: failed unmarshaling canonical information: %w", err)
	}

	return model.NewInformationState(dto.Epoch, dto.FuelPriceIndex, dto.SpotRateIndex, dto.WeatherAlertCount)
}

// DecodeCanonicalBelief decodes a CanonicalBeliefDTO JSON byte slice into a typed domain Belief.
func DecodeCanonicalBelief[C model.CompetitorScale](scale C, data []byte) (*model.Belief[C], error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("journal: empty belief state data")
	}
	var dto CanonicalBeliefDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return nil, fmt.Errorf("journal: failed unmarshaling canonical belief: %w", err)
	}

	keys := make([]string, 0, len(dto.Probabilities))
	for k := range dto.Probabilities {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	probs := make([]float64, len(keys))
	for i, k := range keys {
		probs[i] = dto.Probabilities[k]
	}

	return model.NewBelief(scale, keys, probs)
}
