package model

import (
	"errors"
	"fmt"
	"sort"
)

var (
	// ErrDriverNotFound is returned when a requested driver ID does not exist in the resource state.
	ErrDriverNotFound = errors.New("domain/model: driver ID not found in resource state")
	// ErrLoadNotFound is returned when a requested load ID does not exist in the resource state.
	ErrLoadNotFound = errors.New("domain/model: load ID not found in resource state")
	// ErrDriverAlreadyAssigned is returned when an assignment targets a driver who is already actively en route.
	ErrDriverAlreadyAssigned = errors.New("domain/model: driver is already assigned to an active load")
	// ErrDuplicateMatch is returned when multiple matches target the same driver or load within a single action.
	ErrDuplicateMatch = errors.New("domain/model: duplicate driver or load match in action")
)

// Location represents a physical spatial node in the transportation network.
type Location struct {
	NodeID string
	Lat    float64
	Lon    float64
}

// Driver represents a power unit / driver resource asset in the fleet.
type Driver struct {
	ID                  string
	CurrentLocation     Location
	AvailableEpoch      int64
	DutyHoursRemaining  float64
	DriveHoursRemaining float64
	AssignedLoadID      string // Empty string indicates the driver is unassigned / idle
}

// IsIdle returns true if the driver is currently unassigned.
func (d Driver) IsIdle() bool {
	return d.AssignedLoadID == ""
}

// Load represents a customer freight load shipment card available in the network.
type Load struct {
	ID                     string
	Origin                 Location
	Destination            Location
	PickupEarliestEpoch    int64
	PickupLatestEpoch      int64
	DeliveryEarliestEpoch  int64
	DeliveryLatestEpoch    int64
	RequiredEquipment      string
	Revenue                float64
	EstimatedTransitEpochs int64
}

// ResourceState (R_t) models the fully observable physical state of the carrier network at discrete epoch t.
//
// In accordance with Inviolate 2 (MOMDP Separation), Inviolate 5 (State Immutability), and
// Principle 2 (Deterministic Reproducibility):
//   - Drivers and Loads are stored as canonical sorted slices indexed by ID to guarantee bit-wise
//     deterministic iteration across all policy evaluations.
//   - All transitions return newly allocated *ResourceState pointers.
//   - Slice queries return deep-copied slice headers to prevent external mutations.
type ResourceState struct {
	drivers     []Driver
	loads       []Load
	driverIndex map[string]int // ID -> index in drivers slice
	loadIndex   map[string]int // ID -> index in loads slice
}

// NewResourceState initializes and returns an immutable ResourceState with canonical sorting by entity ID.
//
// In Go, passing slices passes reference headers. This constructor deep-copies both slices
// and sorts them canonically to guarantee determinism (AGENTS.md Section 3.4).
func NewResourceState(drivers []Driver, loads []Load) *ResourceState {
	copiedDrivers := make([]Driver, len(drivers))
	copy(copiedDrivers, drivers)
	sort.Slice(copiedDrivers, func(i, j int) bool {
		return copiedDrivers[i].ID < copiedDrivers[j].ID
	})

	copiedLoads := make([]Load, len(loads))
	copy(copiedLoads, loads)
	sort.Slice(copiedLoads, func(i, j int) bool {
		return copiedLoads[i].ID < copiedLoads[j].ID
	})

	dIndex := make(map[string]int, len(copiedDrivers))
	for i, d := range copiedDrivers {
		dIndex[d.ID] = i
	}

	lIndex := make(map[string]int, len(copiedLoads))
	for i, l := range copiedLoads {
		lIndex[l.ID] = i
	}

	return &ResourceState{
		drivers:     copiedDrivers,
		loads:       copiedLoads,
		driverIndex: dIndex,
		loadIndex:   lIndex,
	}
}

// Drivers returns a deep copy of the active driver fleet slice.
func (rs *ResourceState) Drivers() []Driver {
	out := make([]Driver, len(rs.drivers))
	copy(out, rs.drivers)
	return out
}

// Loads returns a deep copy of the outstanding load inventory slice.
func (rs *ResourceState) Loads() []Load {
	out := make([]Load, len(rs.loads))
	copy(out, rs.loads)
	return out
}

// DriverCount returns the number of drivers in the resource state.
func (rs *ResourceState) DriverCount() int {
	return len(rs.drivers)
}

// LoadCount returns the number of available loads in the resource state.
func (rs *ResourceState) LoadCount() int {
	return len(rs.loads)
}

// GetDriver returns a driver by ID in O(1) time.
func (rs *ResourceState) GetDriver(id string) (Driver, bool) {
	idx, ok := rs.driverIndex[id]
	if !ok {
		return Driver{}, false
	}
	return rs.drivers[idx], true
}

// GetLoad returns a load by ID in O(1) time.
func (rs *ResourceState) GetLoad(id string) (Load, bool) {
	idx, ok := rs.loadIndex[id]
	if !ok {
		return Load{}, false
	}
	return rs.loads[idx], true
}

// DriverLoadMatch represents the physical assignment of a driver resource to a customer load.
type DriverLoadMatch struct {
	DriverID              string
	LoadID                string
	DispatchEpoch         int64
	EstimatedContribution float64
}

// Transition executes the deterministic physical resource transition:
//
//	R_{t+1} = f_R(R_t, x_t, D_{t+1})
//
// Transitions driver positions and hours upon assignment, removes matched loads,
// and integrates newly realized customer load offers D_{t+1}.
//
// In accordance with Inviolate 5, this method leaves the parent ResourceState unaltered
// and returns a newly allocated *ResourceState pointer.
func (rs *ResourceState) Transition(matches []DriverLoadMatch, newLoads []Load) (*ResourceState, error) {
	// Validate matches
	seenDrivers := make(map[string]bool, len(matches))
	seenLoads := make(map[string]bool, len(matches))

	for _, m := range matches {
		if seenDrivers[m.DriverID] {
			return nil, fmt.Errorf("%w: driver %s matched multiple times", ErrDuplicateMatch, m.DriverID)
		}
		seenDrivers[m.DriverID] = true

		if seenLoads[m.LoadID] {
			return nil, fmt.Errorf("%w: load %s matched multiple times", ErrDuplicateMatch, m.LoadID)
		}
		seenLoads[m.LoadID] = true

		d, exists := rs.GetDriver(m.DriverID)
		if !exists {
			return nil, fmt.Errorf("%w: driver %s", ErrDriverNotFound, m.DriverID)
		}
		if !d.IsIdle() {
			return nil, fmt.Errorf("%w: driver %s has active load %s", ErrDriverAlreadyAssigned, d.ID, d.AssignedLoadID)
		}

		_, loadExists := rs.GetLoad(m.LoadID)
		if !loadExists {
			return nil, fmt.Errorf("%w: load %s", ErrLoadNotFound, m.LoadID)
		}
	}

	// Prepare updated drivers
	nextDrivers := make([]Driver, len(rs.drivers))
	copy(nextDrivers, rs.drivers)

	for _, m := range matches {
		idx := rs.driverIndex[m.DriverID]
		load := rs.loads[rs.loadIndex[m.LoadID]]

		d := nextDrivers[idx]
		d.CurrentLocation = load.Destination
		d.AvailableEpoch = m.DispatchEpoch + load.EstimatedTransitEpochs
		d.AssignedLoadID = "" // Load completed upon transition to next epoch
		// Update hours of service
		transitHours := float64(load.EstimatedTransitEpochs)
		d.DriveHoursRemaining = max(0.0, d.DriveHoursRemaining-transitHours)
		d.DutyHoursRemaining = max(0.0, d.DutyHoursRemaining-transitHours)

		nextDrivers[idx] = d
	}

	// Prepare remaining loads (unmatched loads + newLoads)
	nextLoads := make([]Load, 0, len(rs.loads)-len(matches)+len(newLoads))
	for _, l := range rs.loads {
		if !seenLoads[l.ID] {
			nextLoads = append(nextLoads, l)
		}
	}
	nextLoads = append(nextLoads, newLoads...)

	return NewResourceState(nextDrivers, nextLoads), nil
}
