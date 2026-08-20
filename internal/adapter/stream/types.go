package stream

import "errors"

var (
	// ErrEmptyDriverID is returned when an ingested telemetry ping lacks a driver ID.
	ErrEmptyDriverID = errors.New("stream: driver ID cannot be empty")
	// ErrEmptyLoadID is returned when an ingested load tender lacks a load ID.
	ErrEmptyLoadID = errors.New("stream: load ID cannot be empty")
	// ErrInvalidCoordinates is returned when latitude or longitude is invalid.
	ErrInvalidCoordinates = errors.New("stream: invalid spatial coordinates")
	// ErrInvalidHOS is returned when drive or duty hours are negative or non-finite.
	ErrInvalidHOS = errors.New("stream: invalid HOS remaining values")
	// ErrInvalidRevenue is returned when load revenue is negative or non-finite.
	ErrInvalidRevenue = errors.New("stream: load revenue must be non-negative")
)

// ELDDriverPingDTO represents an incoming real-time GPS telemetry and HOS clock update from an ELD device.
type ELDDriverPingDTO struct {
	DriverID            string   `json:"driver_id"`
	Timestamp           int64    `json:"timestamp"`
	Lat                 float64  `json:"lat"`
	Lon                 float64  `json:"lon"`
	NodeID              string   `json:"node_id,omitempty"`
	DriveHoursRemaining float64  `json:"drive_hours_remaining"`
	DutyHoursRemaining  float64  `json:"duty_hours_remaining"`
	PTAEpoch            int64    `json:"pta_epoch,omitempty"`
	AssignedLoadID      string   `json:"assigned_load_id,omitempty"`
	EquipmentType       string   `json:"equipment_type,omitempty"`
	Endorsements        []string `json:"endorsements,omitempty"`
}

// TMSLoadTenderDTO represents an incoming customer freight load tender from a TMS or EDI 204 feed.
type TMSLoadTenderDTO struct {
	LoadID                string   `json:"load_id"`
	Timestamp             int64    `json:"timestamp"`
	OriginNodeID          string   `json:"origin_node_id"`
	OriginLat             float64  `json:"origin_lat"`
	OriginLon             float64  `json:"origin_lon"`
	DestinationNodeID     string   `json:"destination_node_id"`
	DestLat               float64  `json:"dest_lat"`
	DestLon               float64  `json:"dest_lon"`
	PickupEarliestEpoch   int64    `json:"pickup_earliest_epoch"`
	PickupLatestEpoch     int64    `json:"pickup_latest_epoch"`
	DeliveryEarliestEpoch int64    `json:"delivery_earliest_epoch"`
	DeliveryLatestEpoch   int64    `json:"delivery_latest_epoch"`
	Revenue               float64  `json:"revenue"`
	RequiredEquipment     string   `json:"required_equipment,omitempty"`
	RequiredEndorsements  []string `json:"required_endorsements,omitempty"`
}

// TenderCancelDTO represents an incoming cancellation signal for a previously tendered load.
type TenderCancelDTO struct {
	LoadID    string `json:"load_id"`
	Reason    string `json:"reason,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// StreamStatusDTO captures real-time operational indicators for the streaming ingestion buffer.
type StreamStatusDTO struct {
	BufferedDriverPings   int    `json:"buffered_driver_pings"`
	BufferedTendersCount  int    `json:"buffered_tenders_count"`
	BufferedCancellations int    `json:"buffered_cancellations"`
	LastIngestTimestamp   int64  `json:"last_ingest_timestamp"`
	LastSyncTimestamp     int64  `json:"last_sync_timestamp"`
	TotalPingsIngested    uint64 `json:"total_pings_ingested"`
	TotalTendersIngested  uint64 `json:"total_tenders_ingested"`
}
