package legacy

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
)

// LocationStore maps postal codes and node IDs to physical geographic coordinates.
type LocationStore struct {
	locations map[string]model.Location
}

// NewLocationStore initializes a LocationStore from a map with defensive copying (Inviolate 5).
func NewLocationStore(locs map[string]model.Location) *LocationStore {
	copied := make(map[string]model.Location, len(locs))
	for k, v := range locs {
		copied[k] = v
	}
	return &LocationStore{locations: copied}
}

// GetLocation retrieves the Location for a given node ID or postal zip code,
// supporting 5-digit zero-padding and 3-digit/2-digit regional prefix fallbacks.
func (ls *LocationStore) GetLocation(id string) (model.Location, bool) {
	if ls == nil || ls.locations == nil {
		return model.Location{}, false
	}
	id = strings.TrimSpace(id)
	if id == "" || id == "." || id == "NONE" {
		return model.Location{}, false
	}

	// 1. Exact match
	if loc, ok := ls.locations[id]; ok {
		return loc, true
	}

	// 2. 5-digit zero-padding (e.g. "1020" -> "01020")
	if len(id) < 5 {
		padded := strings.Repeat("0", 5-len(id)) + id
		if loc, ok := ls.locations[padded]; ok {
			return loc, true
		}
	}

	// 3. 3-digit prefix match (e.g. "01020" -> "010")
	if len(id) >= 3 {
		prefix3 := id[:3]
		if loc, ok := ls.locations[prefix3]; ok {
			return loc, true
		}
	}

	// 4. 2-digit prefix match (e.g. "01020" -> "01")
	if len(id) >= 2 {
		prefix2 := id[:2]
		if loc, ok := ls.locations[prefix2]; ok {
			return loc, true
		}
	}

	return model.Location{}, false
}

// ParseLocations parses a tab- or whitespace-delimited locations.txt file into a LocationStore.
func ParseLocations(r io.Reader) (*LocationStore, error) {
	scanner := bufio.NewScanner(r)
	locMap := make(map[string]model.Location)
	var headerMap map[string]int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var fields []string
		if strings.Contains(line, "\t") {
			fields = strings.Split(line, "\t")
			for i := range fields {
				fields[i] = strings.TrimSpace(fields[i])
			}
		} else {
			fields = strings.Fields(line)
		}

		if headerMap == nil {
			headerMap = make(map[string]int)
			for idx, col := range fields {
				headerMap[strings.ToUpper(col)] = idx
			}
			continue
		}

		if len(fields) < 3 {
			continue
		}

		codeIdx, okCode := headerMap["CODE"]
		latIdx, okLat := headerMap["LAT"]
		lonIdx, okLon := headerMap["LONG"]
		if !okLon {
			lonIdx, okLon = headerMap["LON"]
		}
		zipIdx, okZip := headerMap["ZIPCODE"]
		if !okZip {
			zipIdx, okZip = headerMap["ZIP"]
		}

		code := fields[0]
		if okCode && codeIdx < len(fields) {
			code = fields[codeIdx]
		}

		latStr := fields[1]
		if okLat && latIdx < len(fields) {
			latStr = fields[latIdx]
		}

		lonStr := fields[2]
		if okLon && lonIdx < len(fields) {
			lonStr = fields[lonIdx]
		}

		lat, err := strconv.ParseFloat(latStr, 64)
		if err != nil {
			continue
		}
		lon, err := strconv.ParseFloat(lonStr, 64)
		if err != nil {
			continue
		}

		loc := model.Location{
			NodeID: code,
			Lat:    lat,
			Lon:    lon,
		}
		locMap[code] = loc
		trimmedCode := strings.TrimLeft(code, "0")
		if trimmedCode != "" && trimmedCode != code {
			locMap[trimmedCode] = loc
		}

		if okZip && zipIdx < len(fields) {
			zip := fields[zipIdx]
			if zip != "" && zip != "." && zip != "NONE" {
				locZip := model.Location{
					NodeID: zip,
					Lat:    lat,
					Lon:    lon,
				}
				locMap[zip] = locZip
				trimmedZip := strings.TrimLeft(zip, "0")
				if trimmedZip != "" && trimmedZip != zip {
					locMap[trimmedZip] = locZip
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("legacy: failed reading locations: %w", err)
	}

	return NewLocationStore(locMap), nil
}

// ParseDrivers parses a legacy drivers.txt file into a slice of domain model Drivers.
func ParseDrivers(r io.Reader, locStore *LocationStore) ([]model.Driver, error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var drivers []model.Driver
	var headerMap map[string]int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Detect delimiter
		var fields []string
		if strings.Contains(line, "\t") {
			fields = strings.Split(line, "\t")
			for i := range fields {
				fields[i] = strings.TrimSpace(fields[i])
			}
		} else {
			fields = strings.Fields(line)
		}

		if headerMap == nil {
			headerMap = make(map[string]int)
			for idx, col := range fields {
				headerMap[strings.ToUpper(col)] = idx
			}
			continue
		}

		getField := func(names ...string) string {
			for _, n := range names {
				if idx, ok := headerMap[strings.ToUpper(n)]; ok && idx < len(fields) {
					return fields[idx]
				}
			}
			return ""
		}

		driverID := getField("DRIVER_ID", "DRVR_ID", "ID", "Driver_ID")
		if driverID == "" && len(fields) > 0 {
			driverID = fields[0]
		}

		availDT := getField("DRIVER_DT", "AVAIL_DT", "AVAILABLE_DT", "NEXT_ON_DUTY_DT", "Available_DT")
		availTM := getField("DRIVER_TM", "AVAIL_TM", "AVAILABLE_TM", "NEXT_ON_DUTY_TM", "Available_TM")
		availLocID := getField("DRIVER_LOCATION", "AVAILABLE_LOCATION", "CURR_LOC", "NEXT_ON_DUTY_LOCATION", "LOCATION", "Available_Location")
		homeLocID := getField("HOME", "HOME_LOCATION", "HOME_LOC", "Home")
		equipStr := getField("EQUIPMENT", "EQUIP", "REQ_EQUIP", "Equipment")

		// Positional fallback if headers not found
		if availLocID == "" && len(fields) >= 6 {
			availDT = fields[3]
			availTM = fields[4]
			availLocID = fields[5]
			homeLocID = fields[2]
			if len(fields) >= 7 {
				equipStr = fields[6]
			}
		}

		if availLocID == "" || availLocID == "NONE" || availLocID == "." {
			if homeLocID != "" && homeLocID != "NONE" && homeLocID != "." {
				availLocID = homeLocID
			}
		}

		availEpoch, _ := parseLegacyDateTime(availDT, availTM)

		loc, ok := locStore.GetLocation(availLocID)
		if !ok {
			loc = model.Location{NodeID: availLocID}
		}

		homeLoc, ok := locStore.GetLocation(homeLocID)
		if !ok {
			homeLoc = loc
		}

		equipType := model.EquipDryVan
		switch strings.ToUpper(equipStr) {
		case "R", "REEFER", "REEFER_53":
			equipType = model.EquipReefer
		case "FB", "FLATBED", "FLATBED_53":
			equipType = model.EquipFlatbed
		case "TANKER":
			equipType = model.EquipTanker
		}

		// Parse initial HOS clocks if available
		var clocks *hos.DriverClocks
		if availEpoch > 0 {
			startTime := time.Unix(availEpoch, 0).UTC()
			clocks = hos.NewDriverClocks(hos.USPolicySpecs(), startTime)

			drivingHoursStr := getField("DRIVING_HOURS")
			if drivingHoursStr != "" {
				driveH, _ := strconv.ParseFloat(drivingHoursStr, 64)
				driveMin := int(math.Round(driveH * 60.0))
				if driveMin > 0 {
					if nextClocks, err := clocks.ApplyDrive(driveMin, hos.USPolicySpecs()); err == nil {
						clocks = nextClocks
					}
				}
			}
		}

		d := model.Driver{
			ID:                  driverID,
			CurrentLocation:     loc,
			HomeLocation:        homeLoc,
			AvailableEpoch:      availEpoch,
			DriveHoursRemaining: 11.0,
			DutyHoursRemaining:  14.0,
			Equipment:           model.Equipment{Type: equipType},
			Clocks:              clocks,
		}

		drivers = append(drivers, d)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("legacy: failed reading drivers: %w", err)
	}

	return drivers, nil
}

// ParseLoads parses a legacy loads.txt file into a slice of domain model Loads.
func ParseLoads(r io.Reader, locStore *LocationStore, maxLoads int) ([]model.Load, error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var loads []model.Load
	var headerMap map[string]int

	for scanner.Scan() {
		if maxLoads > 0 && len(loads) >= maxLoads {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var fields []string
		if strings.Contains(line, "\t") {
			fields = strings.Split(line, "\t")
			for i := range fields {
				fields[i] = strings.TrimSpace(fields[i])
			}
		} else {
			fields = strings.Fields(line)
		}

		if headerMap == nil {
			headerMap = make(map[string]int)
			for idx, col := range fields {
				headerMap[strings.ToUpper(col)] = idx
			}
			continue
		}

		getField := func(names ...string) string {
			for _, n := range names {
				if idx, ok := headerMap[strings.ToUpper(n)]; ok && idx < len(fields) {
					return fields[idx]
				}
			}
			return ""
		}

		loadID := getField("LOAD_ID", "ID")
		if loadID == "" && len(fields) > 0 {
			loadID = fields[0]
		}

		origID := getField("ORIG", "ORIGIN", "PICKUP_LOC")
		destID := getField("DEST", "DESTINATION", "DROPOFF_LOC")

		pkupStDT := getField("PKUP_ST_DT", "PICKUP_ST_DT")
		pkupStTM := getField("PKUP_ST_TM", "PICKUP_ST_TM")
		pkupEndDT := getField("PKUP_END_DT", "PICKUP_END_DT")
		pkupEndTM := getField("PKUP_END_TM", "PICKUP_END_TM")

		dlvStDT := getField("DLVERY_ST_DT", "DELIVERY_ST_DT")
		dlvStTM := getField("DLVERY_ST_TM", "DELIVERY_ST_TM")
		dlvEndDT := getField("DLVERY_END_DT", "DELIVERY_END_DT")
		dlvEndTM := getField("DLVERY_END_TM", "DELIVERY_END_TM")

		linehaulRevStr := getField("LINE_HAUL_REV", "LINEHAUL_REV", "REVENUE")
		otherRevStr := getField("OTHER_REV")
		equipStr := getField("EQUIP", "EQUIPMENT", "REQ_EQUIP")

		// Positional fallback
		if origID == "" && len(fields) >= 25 {
			origID = fields[10]
			destID = fields[17]
			pkupStDT = fields[12]
			pkupStTM = fields[13]
			pkupEndDT = fields[14]
			pkupEndTM = fields[15]
			dlvStDT = fields[19]
			dlvStTM = fields[20]
			dlvEndDT = fields[21]
			dlvEndTM = fields[22]
			linehaulRevStr = fields[24]
			if len(fields) >= 26 {
				otherRevStr = fields[25]
			}
			if len(fields) >= 4 {
				equipStr = fields[3]
			}
		}

		linehaulRev, _ := strconv.ParseFloat(linehaulRevStr, 64)
		otherRev, _ := strconv.ParseFloat(otherRevStr, 64)
		totalRev := linehaulRev + otherRev

		pkupEarliest, _ := parseLegacyDateTime(pkupStDT, pkupStTM)
		pkupLatest, _ := parseLegacyDateTime(pkupEndDT, pkupEndTM)
		dlvEarliest, _ := parseLegacyDateTime(dlvStDT, dlvStTM)
		dlvLatest, _ := parseLegacyDateTime(dlvEndDT, dlvEndTM)

		origLoc, ok := locStore.GetLocation(origID)
		if !ok {
			origLoc = model.Location{NodeID: origID}
		}
		destLoc, ok := locStore.GetLocation(destID)
		if !ok {
			destLoc = model.Location{NodeID: destID}
		}

		equipType := model.EquipDryVan
		switch strings.ToUpper(equipStr) {
		case "R", "REEFER", "REEFER_53":
			equipType = model.EquipReefer
		case "FB", "FLATBED", "FLATBED_53":
			equipType = model.EquipFlatbed
		case "TANKER":
			equipType = model.EquipTanker
		}

		l := model.Load{
			ID:                    loadID,
			Origin:                origLoc,
			Destination:           destLoc,
			PickupEarliestEpoch:   pkupEarliest,
			PickupLatestEpoch:     pkupLatest,
			DeliveryEarliestEpoch: dlvEarliest,
			DeliveryLatestEpoch:   dlvLatest,
			Revenue:               totalRev,
			RequiredEquipment:     equipType,
		}

		loads = append(loads, l)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("legacy: failed reading loads: %w", err)
	}

	return loads, nil
}

// LoadCarrierScenario loads locations, drivers, and loads from their respective file paths.
func LoadCarrierScenario(locationsFile, driversFile, loadsFile string, maxLoads int) ([]model.Driver, []model.Load, *LocationStore, error) {
	locF, err := os.Open(locationsFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("legacy: cannot open locations file: %w", err)
	}
	defer locF.Close()

	locStore, err := ParseLocations(locF)
	if err != nil {
		return nil, nil, nil, err
	}

	driverF, err := os.Open(driversFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("legacy: cannot open drivers file: %w", err)
	}
	defer driverF.Close()

	drivers, err := ParseDrivers(driverF, locStore)
	if err != nil {
		return nil, nil, nil, err
	}

	loadF, err := os.Open(loadsFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("legacy: cannot open loads file: %w", err)
	}
	defer loadF.Close()

	loads, err := ParseLoads(loadF, locStore, maxLoads)
	if err != nil {
		return nil, nil, nil, err
	}

	return drivers, loads, locStore, nil
}

// parseLegacyDateTime converts YYYYMMDD and HHMM strings into a UTC Unix epoch timestamp.
func parseLegacyDateTime(dtStr, tmStr string) (int64, error) {
	dtStr = strings.TrimSpace(dtStr)
	tmStr = strings.TrimSpace(tmStr)
	if dtStr == "NONE" || dtStr == "." || dtStr == "" || len(dtStr) != 8 {
		return 0, nil
	}

	for len(tmStr) < 4 {
		tmStr = "0" + tmStr
	}

	combined := dtStr + tmStr
	t, err := time.Parse("200601021504", combined)
	if err != nil {
		return 0, err
	}
	return t.UTC().Unix(), nil
}

// ParsePreAssignments parses a legacy preassignments.txt file into a map of LoadID -> DriverID.
func ParsePreAssignments(r io.Reader) (map[string]string, error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	preassignments := make(map[string]string)
	var headerMap map[string]int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var fields []string
		if strings.Contains(line, "\t") {
			fields = strings.Split(line, "\t")
			for i := range fields {
				fields[i] = strings.TrimSpace(fields[i])
			}
		} else {
			fields = strings.Fields(line)
		}

		if headerMap == nil {
			headerMap = make(map[string]int)
			for idx, col := range fields {
				headerMap[strings.ToUpper(col)] = idx
			}
			continue
		}

		loadIdx, okL := headerMap["LOAD_ID"]
		drvrIdx, okD := headerMap["DRVR_ID"]
		if !okD {
			drvrIdx, okD = headerMap["DRIVER_ID"]
		}

		if okL && okD && loadIdx < len(fields) && drvrIdx < len(fields) {
			loadID := fields[loadIdx]
			driverID := fields[drvrIdx]
			if loadID != "" && driverID != "" && driverID != "NONE" && driverID != "." {
				preassignments[loadID] = driverID
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("legacy: failed reading preassignments: %w", err)
	}

	return preassignments, nil
}

// ParseRelays parses a legacy RC_all_relays.txt or allPossibleRelays.txt file into a slice of candidate relay node IDs.
func ParseRelays(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	relaySet := make(map[string]bool)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) > 0 {
			nodeID := fields[0]
			if nodeID != "" && nodeID != "NONE" && nodeID != "." {
				relaySet[nodeID] = true
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("legacy: failed reading relays: %w", err)
	}

	relays := make([]string, 0, len(relaySet))
	for r := range relaySet {
		relays = append(relays, r)
	}
	return relays, nil
}

// TimeAtHomeSchedule represents a scheduled time-off window for a driver.
type TimeAtHomeSchedule struct {
	DriverID       string
	StartEpoch     int64
	EndEpoch       int64
	ReturnLocation string
}

// ParseTimeAtHomeSchedules parses legacy driverTAHSchedules.txt or tah_schedule.txt into a map of DriverID -> schedules.
func ParseTimeAtHomeSchedules(r io.Reader) (map[string][]TimeAtHomeSchedule, error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	schedules := make(map[string][]TimeAtHomeSchedule)
	var headerMap map[string]int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var fields []string
		if strings.Contains(line, "\t") {
			fields = strings.Split(line, "\t")
			for i := range fields {
				fields[i] = strings.TrimSpace(fields[i])
			}
		} else {
			fields = strings.Fields(line)
		}

		if headerMap == nil {
			headerMap = make(map[string]int)
			for idx, col := range fields {
				headerMap[strings.ToUpper(col)] = idx
			}
			continue
		}

		getField := func(names ...string) string {
			for _, n := range names {
				if idx, ok := headerMap[strings.ToUpper(n)]; ok && idx < len(fields) {
					return fields[idx]
				}
			}
			return ""
		}

		driverID := getField("DRIVER_ID", "DRVR_ID")
		stDT := getField("SCHEDULED_TIME_OFF_ST_DT", "TIME_OFF_ST_DT", "ST_DT")
		stTM := getField("SCHEDULED_TIME_OFF_ST_TM", "TIME_OFF_ST_TM", "ST_TM")
		endDT := getField("SCHEDULED_TIME_OFF_END_DT", "TIME_OFF_END_DT", "END_DT")
		endTM := getField("SCHEDULED_TIME_OFF_END_TM", "TIME_OFF_END_TM", "END_TM")
		retLoc := getField("SCHEDULED_TIME_OFF_RETURN_LOC", "RETURN_LOC")

		stEpoch, _ := parseLegacyDateTime(stDT, stTM)
		endEpoch, _ := parseLegacyDateTime(endDT, endTM)

		if driverID != "" && stEpoch > 0 {
			sched := TimeAtHomeSchedule{
				DriverID:       driverID,
				StartEpoch:     stEpoch,
				EndEpoch:       endEpoch,
				ReturnLocation: retLoc,
			}
			schedules[driverID] = append(schedules[driverID], sched)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("legacy: failed reading TAH schedules: %w", err)
	}

	return schedules, nil
}
