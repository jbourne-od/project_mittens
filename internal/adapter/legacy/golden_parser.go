// Package legacy provides adapters and verification parsers for legacy Java
// carrier matching datasets and golden operational test files.
//
// In accordance with Project Mittens AGENTS.md:
//   - Dependency Boundaries: Imports /internal/domain/model, /pkg/logging
//   - Inviolate 5: State immutability via value-based allocation.
package legacy

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// GoldenDriverMove represents an individual evaluated move or tour segment in rankedListsForDrivers.txt.
type GoldenDriverMove struct {
	DriverID          string
	DataReliable      bool
	Rank1             int
	Rank2             int
	MovementType      string // "LOADED", "DRIVER_HOLD", "EMPTY_TO_HOME", "RELAY"
	MovementID        string
	LoadID            string
	DriverCurrentTime string
	DriverCurrentLoc  string
	LoadCurrentTime   string
	LoadCurrentLoc    string
	ShipperID         string
	ReqEquipment      string
	EmptyMiles        float64
	LoadedMiles       float64
	Revenue           float64
	Cost              float64
	PickupLoc         string
	PickupStartTime   string
	PickupEndTime     string
	PickupArrivalTime string
	DropoffLoc        string
	DropoffStartTime  string
	DropoffEndTime    string
	UnloadStartTime   string
	UnloadEndTime     string
	DriverRecords     string // Raw HOS timeline string
	ReducedCost       float64
	IsEmptyToHome     bool
	IsHold            bool
	IsLoaded          bool
}

// GoldenLoadMove represents an individual evaluated assignment candidate in rankedListsForLoads.txt.
type GoldenLoadMove struct {
	LoadID       string
	SystemID     string
	Rank1        int
	Rank2        int
	MovementType string
	DriverID     string
	EmptyMiles   float64
	LoadedMiles  float64
	PickupLoc    string
	DropoffLoc   string
}

// ParseGoldenRankedDrivers parses a tab-delimited rankedListsForDrivers.txt file.
func ParseGoldenRankedDrivers(r io.Reader) ([]GoldenDriverMove, error) {
	scanner := bufio.NewScanner(r)
	// Allow large token buffer for long timeline strings
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var headerMap map[string]int
	var records []GoldenDriverMove

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, "\t")
		if headerMap == nil {
			headerMap = make(map[string]int)
			for idx, col := range fields {
				headerMap[strings.TrimSpace(col)] = idx
			}
			continue
		}

		getField := func(colName string) string {
			idx, ok := headerMap[colName]
			if !ok || idx >= len(fields) {
				return ""
			}
			return strings.TrimSpace(fields[idx])
		}

		getFloat := func(colName string) float64 {
			valStr := getField(colName)
			if valStr == "" {
				return 0.0
			}
			val, _ := strconv.ParseFloat(valStr, 64)
			return val
		}

		getInt := func(colName string) int {
			valStr := getField(colName)
			if valStr == "" {
				return 0
			}
			val, _ := strconv.Atoi(valStr)
			return val
		}

		driverID := getField("DRIVER_ID")
		if driverID == "" {
			continue
		}

		movType := strings.ToUpper(getField("MOVEMENT_TYPE"))

		rec := GoldenDriverMove{
			DriverID:          driverID,
			DataReliable:      strings.ToLower(getField("DRIVER_DATA_RELIABLE")) == "true",
			Rank1:             getInt("1ST_ASSIGN_RANK"),
			Rank2:             getInt("2ND_ASSIGN_RANK"),
			MovementType:      movType,
			MovementID:        getField("MOVEMENT_ID"),
			LoadID:            getField("LOAD_ID"),
			DriverCurrentTime: getField("DRIVER_CURRENT_TIME"),
			DriverCurrentLoc:  getField("DRIVER_CURRENT_LOCATION"),
			LoadCurrentTime:   getField("LOAD_CURRENT_TIME"),
			LoadCurrentLoc:    getField("LOAD_CURRENT_LOCATION"),
			ShipperID:         getField("SHIPPER_ID"),
			ReqEquipment:      getField("REQ_EQUIP"),
			EmptyMiles:        getFloat("EMPTY_MILES"),
			LoadedMiles:       getFloat("LOADED_MILES"),
			Revenue:           getFloat("HARD_DOLLAR_REVENUE"),
			Cost:              getFloat("HARD_DOLLAR_COST"),
			PickupLoc:         getField("PICK_UP_LOC"),
			PickupStartTime:   getField("PICK_UP_ST_TIME"),
			PickupEndTime:     getField("PICK_UP_END_TIME"),
			PickupArrivalTime: getField("PICK_UP_ARRIVAL_TIME"),
			DropoffLoc:        getField("DROP_OFF_LOC"),
			DropoffStartTime:  getField("DROP_OFF_ST_TIME"),
			DropoffEndTime:    getField("DROP_OFF_END_TIME"),
			UnloadStartTime:   getField("UNLOADING_START_TIME"),
			UnloadEndTime:     getField("UNLOADING_END_TIME"),
			DriverRecords:     getField("DRIVER_RECORDS"),
			ReducedCost:       getFloat("REDUCED_COST"),
			IsEmptyToHome:     movType == "EMPTY_TO_HOME",
			IsHold:            movType == "DRIVER_HOLD",
			IsLoaded:          movType == "LOADED",
		}

		records = append(records, rec)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("legacy: failed reading golden ranked drivers: %w", err)
	}

	return records, nil
}

// ParseGoldenRankedDriversFile reads and parses a rankedListsForDrivers.txt file by filepath.
func ParseGoldenRankedDriversFile(path string) ([]GoldenDriverMove, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("legacy: failed opening %s: %w", path, err)
	}
	defer file.Close()
	return ParseGoldenRankedDrivers(file)
}

// ParseGoldenRankedLoads parses a tab-delimited rankedListsForLoads.txt file.
func ParseGoldenRankedLoads(r io.Reader) ([]GoldenLoadMove, error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var headerMap map[string]int
	var records []GoldenLoadMove

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, "\t")
		if headerMap == nil {
			headerMap = make(map[string]int)
			for idx, col := range fields {
				headerMap[strings.TrimSpace(col)] = idx
			}
			continue
		}

		getField := func(colName string) string {
			idx, ok := headerMap[colName]
			if !ok || idx >= len(fields) {
				return ""
			}
			return strings.TrimSpace(fields[idx])
		}

		getFloat := func(colName string) float64 {
			valStr := getField(colName)
			if valStr == "" {
				return 0.0
			}
			val, _ := strconv.ParseFloat(valStr, 64)
			return val
		}

		getInt := func(colName string) int {
			valStr := getField(colName)
			if valStr == "" {
				return 0
			}
			val, _ := strconv.Atoi(valStr)
			return val
		}

		loadID := getField("LOAD_ID")
		if loadID == "" {
			continue
		}

		rec := GoldenLoadMove{
			LoadID:       loadID,
			SystemID:     getField("SYSTEM_ID"),
			Rank1:        getInt("1ST_ASSIGN_RANK"),
			Rank2:        getInt("2ND_ASSIGN_RANK"),
			MovementType: strings.ToUpper(getField("MOVEMENT_TYPE")),
			DriverID:     getField("DRIVER_ID"),
			EmptyMiles:   getFloat("EMPTY_MILES"),
			LoadedMiles:  getFloat("LOADED_MILES"),
			PickupLoc:    getField("PICK_UP_LOC"),
			DropoffLoc:   getField("DROP_OFF_LOC"),
		}

		records = append(records, rec)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("legacy: failed reading golden ranked loads: %w", err)
	}

	return records, nil
}

// ParseGoldenRankedLoadsFile reads and parses a rankedListsForLoads.txt file by filepath.
func ParseGoldenRankedLoadsFile(path string) ([]GoldenLoadMove, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("legacy: failed opening %s: %w", path, err)
	}
	defer file.Close()
	return ParseGoldenRankedLoads(file)
}
