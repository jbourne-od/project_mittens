package stream

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// StreamBuffer provides a high-concurrency, thread-safe in-memory buffer
// for accumulating real-time ELD telemetry pings and TMS freight tenders.
type StreamBuffer struct {
	mu            sync.Mutex
	driverPings   map[string]ELDDriverPingDTO
	loadTenders   map[string]TMSLoadTenderDTO
	cancellations map[string]TenderCancelDTO

	totalPings   atomic.Uint64
	totalTenders atomic.Uint64
	lastIngest   atomic.Int64
	lastSync     atomic.Int64
}

// NewStreamBuffer constructs an initialized, empty StreamBuffer instance.
func NewStreamBuffer() *StreamBuffer {
	return &StreamBuffer{
		driverPings:   make(map[string]ELDDriverPingDTO),
		loadTenders:   make(map[string]TMSLoadTenderDTO),
		cancellations: make(map[string]TenderCancelDTO),
	}
}

// IngestDriverPing validates and buffers a single ELD driver telemetry update.
func (b *StreamBuffer) IngestDriverPing(ping ELDDriverPingDTO) error {
	if err := ValidateDriverPing(ping); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.totalPings.Add(1)
	b.lastIngest.Store(time.Now().Unix())

	// If a newer ping already exists for this driver, ignore older out-of-order ping
	if existing, ok := b.driverPings[ping.DriverID]; ok && existing.Timestamp > ping.Timestamp {
		return nil
	}

	b.driverPings[ping.DriverID] = ping
	return nil
}

// IngestDriverBatch validates and buffers multiple ELD driver telemetry updates atomically.
func (b *StreamBuffer) IngestDriverBatch(pings []ELDDriverPingDTO) error {
	for _, p := range pings {
		if err := b.IngestDriverPing(p); err != nil {
			return err
		}
	}
	return nil
}

// IngestLoadTender validates and buffers a new customer load tender.
func (b *StreamBuffer) IngestLoadTender(tender TMSLoadTenderDTO) error {
	if err := ValidateLoadTender(tender); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// If load was previously canceled, re-tendering removes it from cancellation map
	delete(b.cancellations, tender.LoadID)
	b.loadTenders[tender.LoadID] = tender
	b.totalTenders.Add(1)
	b.lastIngest.Store(time.Now().Unix())
	return nil
}

// IngestTenderBatch validates and buffers multiple load tenders atomically.
func (b *StreamBuffer) IngestTenderBatch(tenders []TMSLoadTenderDTO) error {
	for _, t := range tenders {
		if err := b.IngestLoadTender(t); err != nil {
			return err
		}
	}
	return nil
}

// CancelTender buffers a cancellation signal for a specified load ID.
func (b *StreamBuffer) CancelTender(cancel TenderCancelDTO) error {
	if strings.TrimSpace(cancel.LoadID) == "" {
		return ErrEmptyLoadID
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.loadTenders, cancel.LoadID)
	b.cancellations[cancel.LoadID] = cancel
	b.lastIngest.Store(time.Now().Unix())
	return nil
}

// Status returns a point-in-time snapshot of the buffer queue depths and ingestion counters.
func (b *StreamBuffer) Status() StreamStatusDTO {
	b.mu.Lock()
	defer b.mu.Unlock()

	return StreamStatusDTO{
		BufferedDriverPings:   len(b.driverPings),
		BufferedTendersCount:  len(b.loadTenders),
		BufferedCancellations: len(b.cancellations),
		LastIngestTimestamp:   b.lastIngest.Load(),
		LastSyncTimestamp:     b.lastSync.Load(),
		TotalPingsIngested:    b.totalPings.Load(),
		TotalTendersIngested:  b.totalTenders.Load(),
	}
}

// Drain extracts all currently buffered updates and clears the buffer for the next cycle.
func (b *StreamBuffer) Drain() (map[string]ELDDriverPingDTO, map[string]TMSLoadTenderDTO, map[string]TenderCancelDTO) {
	b.mu.Lock()
	defer b.mu.Unlock()

	pings := b.driverPings
	tenders := b.loadTenders
	cancels := b.cancellations

	b.driverPings = make(map[string]ELDDriverPingDTO)
	b.loadTenders = make(map[string]TMSLoadTenderDTO)
	b.cancellations = make(map[string]TenderCancelDTO)
	b.lastSync.Store(time.Now().Unix())

	return pings, tenders, cancels
}

// Snapshot returns a defensive copy of all currently buffered items without draining.
func (b *StreamBuffer) Snapshot() (map[string]ELDDriverPingDTO, map[string]TMSLoadTenderDTO, map[string]TenderCancelDTO) {
	b.mu.Lock()
	defer b.mu.Unlock()

	pings := make(map[string]ELDDriverPingDTO, len(b.driverPings))
	for k, v := range b.driverPings {
		pings[k] = v
	}

	tenders := make(map[string]TMSLoadTenderDTO, len(b.loadTenders))
	for k, v := range b.loadTenders {
		tenders[k] = v
	}

	cancels := make(map[string]TenderCancelDTO, len(b.cancellations))
	for k, v := range b.cancellations {
		cancels[k] = v
	}

	return pings, tenders, cancels
}

// ValidateDriverPing validates field constraints for an ELD telemetry ping.
func ValidateDriverPing(ping ELDDriverPingDTO) error {
	if strings.TrimSpace(ping.DriverID) == "" {
		return ErrEmptyDriverID
	}
	if ping.Lat < -90.0 || ping.Lat > 90.0 || math.IsNaN(ping.Lat) || math.IsInf(ping.Lat, 0) ||
		ping.Lon < -180.0 || ping.Lon > 180.0 || math.IsNaN(ping.Lon) || math.IsInf(ping.Lon, 0) {
		return fmt.Errorf("%w: lat=%.4f, lon=%.4f", ErrInvalidCoordinates, ping.Lat, ping.Lon)
	}
	if ping.DriveHoursRemaining < 0 || math.IsNaN(ping.DriveHoursRemaining) || math.IsInf(ping.DriveHoursRemaining, 0) ||
		ping.DutyHoursRemaining < 0 || math.IsNaN(ping.DutyHoursRemaining) || math.IsInf(ping.DutyHoursRemaining, 0) {
		return fmt.Errorf("%w: drive=%.2f, duty=%.2f", ErrInvalidHOS, ping.DriveHoursRemaining, ping.DutyHoursRemaining)
	}
	return nil
}

// ValidateLoadTender validates field constraints for a TMS load tender.
func ValidateLoadTender(tender TMSLoadTenderDTO) error {
	if strings.TrimSpace(tender.LoadID) == "" {
		return ErrEmptyLoadID
	}
	if tender.OriginLat < -90.0 || tender.OriginLat > 90.0 || math.IsNaN(tender.OriginLat) || math.IsInf(tender.OriginLat, 0) ||
		tender.OriginLon < -180.0 || tender.OriginLon > 180.0 || math.IsNaN(tender.OriginLon) || math.IsInf(tender.OriginLon, 0) ||
		tender.DestLat < -90.0 || tender.DestLat > 90.0 || math.IsNaN(tender.DestLat) || math.IsInf(tender.DestLat, 0) ||
		tender.DestLon < -180.0 || tender.DestLon > 180.0 || math.IsNaN(tender.DestLon) || math.IsInf(tender.DestLon, 0) {
		return ErrInvalidCoordinates
	}
	if tender.Revenue < 0 || math.IsNaN(tender.Revenue) || math.IsInf(tender.Revenue, 0) {
		return fmt.Errorf("%w: revenue=%.2f", ErrInvalidRevenue, tender.Revenue)
	}
	if tender.DeliveryLatestEpoch < tender.PickupEarliestEpoch && tender.DeliveryLatestEpoch > 0 {
		return fmt.Errorf("stream: delivery latest epoch %d is before pickup earliest epoch %d", tender.DeliveryLatestEpoch, tender.PickupEarliestEpoch)
	}
	return nil
}
