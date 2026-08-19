package model

import (
	"fmt"
	"math"
)

// SquareRegion represents a geographic bounding box region for spatial aggregation
// and lane-balance modeling, matching legacy FleetManager SquareRegion geometry.
type SquareRegion struct {
	ID       string
	MinLat   float64
	MaxLat   float64
	MinLon   float64
	MaxLon   float64
	Centroid Location
}

// Contains returns true if the specified location falls inside this region bounding box.
func (r SquareRegion) Contains(loc Location) bool {
	return loc.Lat >= r.MinLat && loc.Lat <= r.MaxLat && loc.Lon >= r.MinLon && loc.Lon <= r.MaxLon
}

// RegionManager manages hierarchical spatial grids and assigns geographic coordinates to regional clusters.
type RegionManager struct {
	gridStepDeg float64 // Degree resolution of grid squares (e.g. 1.0 degree ~ 69 miles)
	regions     []SquareRegion
}

// NewRegionManager constructs a RegionManager covering North America with degree-resolution grid squares.
func NewRegionManager(gridStepDeg float64) *RegionManager {
	if gridStepDeg <= 0 {
		gridStepDeg = 1.0
	}
	return &RegionManager{
		gridStepDeg: gridStepDeg,
		regions:     nil,
	}
}

// AddRegion registers a custom regional bounding box.
func (rm *RegionManager) AddRegion(region SquareRegion) {
	rm.regions = append(rm.regions, region)
}

// Regions returns all registered square regions.
func (rm *RegionManager) Regions() []SquareRegion {
	out := make([]SquareRegion, len(rm.regions))
	copy(out, rm.regions)
	return out
}

// GetRegionID returns the canonical region identifier for a given geographic coordinate.
func (rm *RegionManager) GetRegionID(loc Location) string {
	// First check registered named regions
	for _, r := range rm.regions {
		if r.Contains(loc) {
			return r.ID
		}
	}

	// Fall back to spatial degree grid
	latIdx := int(math.Floor(loc.Lat / rm.gridStepDeg))
	lonIdx := int(math.Floor(loc.Lon / rm.gridStepDeg))
	return fmt.Sprintf("REG_%d_%d", latIdx, lonIdx)
}

// DistanceBetweenRegions returns the approximate great-circle distance between two region centroids.
func (rm *RegionManager) DistanceBetweenRegions(r1ID, r2ID string) float64 {
	if r1ID == r2ID {
		return 0.0
	}
	var lat1, lon1, lat2, lon2 float64
	_, err1 := fmt.Sscanf(r1ID, "REG_%f_%f", &lat1, &lon1)
	_, err2 := fmt.Sscanf(r2ID, "REG_%f_%f", &lat2, &lon2)
	if err1 != nil || err2 != nil {
		return 0.0
	}

	loc1 := Location{Lat: lat1 * rm.gridStepDeg, Lon: lon1 * rm.gridStepDeg}
	loc2 := Location{Lat: lat2 * rm.gridStepDeg, Lon: lon2 * rm.gridStepDeg}
	return loc1.DistanceMiles(loc2)
}
