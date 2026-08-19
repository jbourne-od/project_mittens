package hos

import "time"

// EventType represents the operational category of a dispatch event.
type EventType int

const (
	// EventDrive represents commercial motor vehicle movement.
	EventDrive EventType = iota
	// EventLoading represents on-duty freight loading at an origin facility.
	EventLoading
	// EventUnloading represents on-duty freight unloading at a destination facility.
	EventUnloading
	// EventRest represents an off-duty or sleeper berth rest period.
	EventRest
	// EventHold represents facility dwell or appointment waiting time.
	EventHold
	// EventBorderCrossing represents customs border inspection and transit dwell.
	EventBorderCrossing
)

func (e EventType) String() string {
	switch e {
	case EventDrive:
		return "DRIVE"
	case EventLoading:
		return "LOADING"
	case EventUnloading:
		return "UNLOADING"
	case EventRest:
		return "REST"
	case EventHold:
		return "HOLD"
	case EventBorderCrossing:
		return "BORDER_CROSSING"
	default:
		return "UNKNOWN"
	}
}

// Event represents an atomic operational task executed by a driver.
type Event struct {
	Type          EventType
	DurationMin   int
	DistanceMiles float64
	IsSleeper     bool
	LocationID    string
	Description   string
}

// DriveEvent creates a CMV movement driving event.
func DriveEvent(durationMin int, distanceMiles float64, locID string) Event {
	return Event{
		Type:          EventDrive,
		DurationMin:   durationMin,
		DistanceMiles: distanceMiles,
		LocationID:    locID,
		Description:   "Commercial Driving",
	}
}

// LoadingEvent creates an on-duty loading event at an origin facility.
func LoadingEvent(durationMin int, locID string) Event {
	return Event{
		Type:        EventLoading,
		DurationMin: durationMin,
		LocationID:  locID,
		Description: "Freight Loading",
	}
}

// UnloadingEvent creates an on-duty unloading event at a destination facility.
func UnloadingEvent(durationMin int, locID string) Event {
	return Event{
		Type:        EventUnloading,
		DurationMin: durationMin,
		LocationID:  locID,
		Description: "Freight Unloading",
	}
}

// RestEvent creates an off-duty or sleeper berth rest period.
func RestEvent(durationMin int, isSleeper bool, locID string) Event {
	desc := "Off-Duty Rest"
	if isSleeper {
		desc = "Sleeper Berth Rest"
	}
	return Event{
		Type:        EventRest,
		DurationMin: durationMin,
		IsSleeper:   isSleeper,
		LocationID:  locID,
		Description: desc,
	}
}

// HoldEvent creates a facility dwell / wait event.
func HoldEvent(durationMin int, locID string) Event {
	return Event{
		Type:        EventHold,
		DurationMin: durationMin,
		LocationID:  locID,
		Description: "Facility Hold",
	}
}

// BorderCrossingEvent creates a border crossing customs dwell event.
func BorderCrossingEvent(durationMin int, locID string) Event {
	return Event{
		Type:        EventBorderCrossing,
		DurationMin: durationMin,
		LocationID:  locID,
		Description: "Border Crossing",
	}
}

// TimelineEntry records the execution of an individual event within the simulation timeline.
type TimelineEntry struct {
	StartTime      time.Time
	EndTime        time.Time
	Event          Event
	ClocksSnapshot DriverClocks
}

// SimulationResult captures the complete outcome of an HOS timeline simulation.
type SimulationResult struct {
	InitialClocks      *DriverClocks
	FinalClocks        *DriverClocks
	Timeline           []TimelineEntry
	TotalDurationMin   int
	TotalDriveMin      int
	TotalRestMin       int
	TotalDutyMin       int
	TotalDistanceMiles float64
}
