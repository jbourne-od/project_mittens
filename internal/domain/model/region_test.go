package model_test

import (
	"testing"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

func TestRegionManager_NamedAndGridRegions(t *testing.T) {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}

	midwest := model.SquareRegion{
		ID:       "MIDWEST",
		MinLat:   40.0,
		MaxLat:   45.0,
		MinLon:   -90.0,
		MaxLon:   -85.0,
		Centroid: model.Location{NodeID: "MIDWEST_CTR", Lat: 42.5, Lon: -87.5},
	}
	southeast := model.SquareRegion{
		ID:       "SOUTHEAST",
		MinLat:   30.0,
		MaxLat:   35.0,
		MinLon:   -88.0,
		MaxLon:   -80.0,
		Centroid: model.Location{NodeID: "SOUTHEAST_CTR", Lat: 32.5, Lon: -84.0},
	}

	rm := model.NewRegionManager(1.0, []model.SquareRegion{southeast, midwest})

	// 1. Check Contains and GetRegionID for registered regions
	if regionID := rm.GetRegionID(locChi); regionID != "MIDWEST" {
		t.Fatalf("expected Chicago in MIDWEST, got %s", regionID)
	}
	if regionID := rm.GetRegionID(locAtl); regionID != "SOUTHEAST" {
		t.Fatalf("expected Atlanta in SOUTHEAST, got %s", regionID)
	}

	// 2. Check fallback to grid cell for an unregistered coordinate (e.g. Seattle, WA)
	locSea := model.Location{NodeID: "SEA", Lat: 47.6062, Lon: -122.3321}
	if regionID := rm.GetRegionID(locSea); regionID != "REG_47_-123" {
		t.Fatalf("expected Seattle in REG_47_-123, got %s", regionID)
	}

	// 3. Check DistanceBetweenRegions between named regions
	distNamed := rm.DistanceBetweenRegions("MIDWEST", "SOUTHEAST")
	if distNamed < 600.0 || distNamed > 800.0 {
		t.Fatalf("expected distance between MIDWEST and SOUTHEAST centroids ~710 miles, got %f", distNamed)
	}

	// 4. Check DistanceBetweenRegions between degree grid regions
	distGrid := rm.DistanceBetweenRegions("REG_41_-88", "REG_33_-85")
	if distGrid < 500.0 || distGrid > 700.0 {
		t.Fatalf("expected distance between grid regions ~580 miles, got %f", distGrid)
	}

	// 5. Distance to self
	if selfDist := rm.DistanceBetweenRegions("MIDWEST", "MIDWEST"); selfDist != 0.0 {
		t.Fatalf("expected self distance = 0, got %f", selfDist)
	}
}
