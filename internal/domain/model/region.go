package model

import (
	"fmt"
	"math"
	"sort"
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
//
// In accordance with Inviolate 5 (Immutability) and Inviolate 6 (Lock-Free Hot Paths),
// RegionManager is constructed immutably and is safe for concurrent access.
type RegionManager struct {
	gridStepDeg float64 // Degree resolution of grid squares (e.g. 1.0 degree ~ 69 miles)
	regions     []SquareRegion
	byID        map[string]SquareRegion
}

// NewRegionManager constructs an immutable RegionManager with canonical region indexing.
func NewRegionManager(gridStepDeg float64, regions []SquareRegion) *RegionManager {
	if gridStepDeg <= 0 {
		gridStepDeg = 1.0
	}

	copied := make([]SquareRegion, len(regions))
	copy(copied, regions)
	sort.Slice(copied, func(i, j int) bool {
		return copied[i].ID < copied[j].ID
	})

	index := make(map[string]SquareRegion, len(copied))
	for i := range copied {
		// Calculate centroid if not explicitly set
		if copied[i].Centroid.Lat == 0 && copied[i].Centroid.Lon == 0 && (copied[i].MinLat != 0 || copied[i].MaxLat != 0) {
			copied[i].Centroid = Location{
				NodeID: copied[i].ID + "_CTR",
				Lat:    (copied[i].MinLat + copied[i].MaxLat) / 2.0,
				Lon:    (copied[i].MinLon + copied[i].MaxLon) / 2.0,
			}
		}
		index[copied[i].ID] = copied[i]
	}

	return &RegionManager{
		gridStepDeg: gridStepDeg,
		regions:     copied,
		byID:        index,
	}
}

// Regions returns a deep copy of all registered square regions.
func (rm *RegionManager) Regions() []SquareRegion {
	out := make([]SquareRegion, len(rm.regions))
	copy(out, rm.regions)
	return out
}

// GetRegion returns a registered region by its ID.
func (rm *RegionManager) GetRegion(id string) (SquareRegion, bool) {
	r, ok := rm.byID[id]
	return r, ok
}

// GetRegionID returns the canonical region identifier for a given geographic coordinate.
func (rm *RegionManager) GetRegionID(loc Location) string {
	// 1. Check registered named regions
	for _, r := range rm.regions {
		if r.Contains(loc) {
			return r.ID
		}
	}

	// 2. Fall back to degree grid square
	latIdx := int(math.Floor(loc.Lat / rm.gridStepDeg))
	lonIdx := int(math.Floor(loc.Lon / rm.gridStepDeg))
	return fmt.Sprintf("REG_%d_%d", latIdx, lonIdx)
}

// DistanceBetweenRegions returns the great-circle distance between two region centroids.
func (rm *RegionManager) DistanceBetweenRegions(r1ID, r2ID string) float64 {
	if r1ID == r2ID {
		return 0.0
	}

	loc1, ok1 := rm.getCentroid(r1ID)
	loc2, ok2 := rm.getCentroid(r2ID)
	if !ok1 || !ok2 {
		return 0.0
	}

	return loc1.DistanceMiles(loc2)
}

func (rm *RegionManager) getCentroid(regionID string) (Location, bool) {
	// 1. Check registered region map
	if r, ok := rm.byID[regionID]; ok {
		return r.Centroid, true
	}

	// 2. Parse grid cell ID (e.g. REG_41_-88)
	var latIdx, lonIdx int
	n, err := fmt.Sscanf(regionID, "REG_%d_%d", &latIdx, &lonIdx)
	if err == nil && n == 2 {
		// Use center of the grid square
		centerLat := (float64(latIdx) + 0.5) * rm.gridStepDeg
		centerLon := (float64(lonIdx) + 0.5) * rm.gridStepDeg
		return Location{
			NodeID: regionID + "_CTR",
			Lat:    centerLat,
			Lon:    centerLon,
		}, true
	}

	return Location{}, false
}
