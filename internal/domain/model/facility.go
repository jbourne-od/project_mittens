package model

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

var (
	// ErrFacilityNotFound is returned when a requested facility ID is not found in the store.
	ErrFacilityNotFound = errors.New("domain/model: facility ID not found in facility store")
)

// FacilityType defines the operational role of a physical transportation node.
type FacilityType string

const (
	// FacilityShipper represents an origin freight shipping facility.
	FacilityShipper FacilityType = "SHIPPER"
	// FacilityReceiver represents a destination freight receiving facility.
	FacilityReceiver FacilityType = "RECEIVER"
	// FacilityTerminal represents an internal carrier domicile or maintenance terminal.
	FacilityTerminal FacilityType = "TERMINAL"
	// FacilityRelayHub represents an intermediate trailer exchange or driver relay hub.
	FacilityRelayHub FacilityType = "RELAY_HUB"
)

// Facility represents a physical shipping, receiving, or interchange node.
type Facility struct {
	ID                  string
	Name                string
	Location            Location
	Type                FacilityType
	OpenMinutesOfDay    int // Minutes from midnight (e.g. 480 = 8:00 AM)
	CloseMinutesOfDay   int // Minutes from midnight (e.g. 1020 = 5:00 PM, 1440 = 24/7)
	AverageDwellMinutes int // Expected loading/unloading or interchange dwell
	RequiresAppointment bool
}

// IsOpenAt checks if the facility is open for service at the specified timestamp.
func (f Facility) IsOpenAt(t time.Time) bool {
	if f.OpenMinutesOfDay == 0 && (f.CloseMinutesOfDay == 0 || f.CloseMinutesOfDay >= 1440) {
		return true // 24/7 operating facility
	}

	minuteOfDay := t.Hour()*60 + t.Minute()
	if f.OpenMinutesOfDay <= f.CloseMinutesOfDay {
		return minuteOfDay >= f.OpenMinutesOfDay && minuteOfDay <= f.CloseMinutesOfDay
	}
	// Overnight operating window (e.g. 20:00 to 06:00)
	return minuteOfDay >= f.OpenMinutesOfDay || minuteOfDay <= f.CloseMinutesOfDay
}

// FacilityStore provides an immutable, high-speed spatial lookup index for network facilities.
//
// In accordance with Inviolate 5 (Immutability) and Inviolate 6 (Lock-Free Hot Paths),
// FacilityStore is pure and thread-safe for concurrent lookups.
type FacilityStore struct {
	facilities []Facility
	byID       map[string]int
}

// NewFacilityStore constructs an immutable FacilityStore with canonical ID sorting.
func NewFacilityStore(facilities []Facility) *FacilityStore {
	copied := make([]Facility, len(facilities))
	copy(copied, facilities)
	sort.Slice(copied, func(i, j int) bool {
		return copied[i].ID < copied[j].ID
	})

	index := make(map[string]int, len(copied))
	for i, f := range copied {
		index[f.ID] = i
	}

	return &FacilityStore{
		facilities: copied,
		byID:       index,
	}
}

// GetFacility returns a facility by its unique ID.
func (fs *FacilityStore) GetFacility(id string) (Facility, error) {
	idx, ok := fs.byID[id]
	if !ok {
		return Facility{}, fmt.Errorf("%w: %s", ErrFacilityNotFound, id)
	}
	return fs.facilities[idx], nil
}

// Count returns the total number of facilities in the store.
func (fs *FacilityStore) Count() int {
	return len(fs.facilities)
}

// FindNearest returns the closest facility of the specified type to a given location.
func (fs *FacilityStore) FindNearest(loc Location, facType FacilityType) (Facility, float64, error) {
	if len(fs.facilities) == 0 {
		return Facility{}, 0, ErrFacilityNotFound
	}

	var best Facility
	bestDist := mathMaxFloat
	found := false

	for _, f := range fs.facilities {
		if facType != "" && f.Type != facType {
			continue
		}
		dist := loc.DistanceMiles(f.Location)
		if dist < bestDist {
			bestDist = dist
			best = f
			found = true
		}
	}

	if !found {
		return Facility{}, 0, ErrFacilityNotFound
	}
	return best, bestDist, nil
}

const mathMaxFloat = 1.79769313486231570814527423731704356798070e+308
