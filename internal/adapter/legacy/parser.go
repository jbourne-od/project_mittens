package legacy

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

// LocationStore maps postal codes and node IDs to physical geographic coordinates.
type LocationStore struct {
	locations map[string]model.Location
}

// NewLocationStore initializes a LocationStore from a map.
func NewLocationStore(locs map[string]model.Location) *LocationStore {
	return &LocationStore{locations: locs}
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

	isHeader := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if isHeader {
			isHeader = false
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		code := fields[0]
		lat, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		lon, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			continue
		}

		loc := model.Location{
			NodeID: code,
			Lat:    lat,
			Lon:    lon,
		}
		locMap[code] = loc

		// Also index zero-padded code if shorter than 5 digits
		if len(code) < 5 {
			padded := fmt.Sprintf("%0*s", 5, code)
			padded = strings.ReplaceAll(padded, " ", "0")
			locMap[padded] = loc
		}

		// If zipcode column exists (e.g. last column), index by zipcode too
		zip := fields[len(fields)-1]
		if zip != "." && zip != "" && zip != code {
			locMap[zip] = model.Location{
				NodeID: zip,
				Lat:    lat,
				Lon:    lon,
			}
			if len(zip) < 5 {
				paddedZip := fmt.Sprintf("%0*s", 5, zip)
				paddedZip = strings.ReplaceAll(paddedZip, " ", "0")
				locMap[paddedZip] = locMap[zip]
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
	var drivers []model.Driver

	isHeader := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if isHeader {
			isHeader = false
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		driverID := fields[0]
		availDT := fields[3]
		availTM := fields[4]
		availLocID := fields[5]

		availEpoch, _ := parseLegacyDateTime(availDT, availTM)

		loc, ok := locStore.GetLocation(availLocID)
		if !ok {
			loc = model.Location{NodeID: availLocID}
		}

		equipType := model.EquipDryVan
		if len(fields) >= 7 {
			switch strings.ToUpper(fields[6]) {
			case "REEFER", "REEFER_53":
				equipType = model.EquipReefer
			case "FLATBED", "FLATBED_53":
				equipType = model.EquipFlatbed
			case "TANKER":
				equipType = model.EquipTanker
			}
		}

		homeLocID := fields[2]
		homeLoc, ok := locStore.GetLocation(homeLocID)
		if !ok {
			homeLoc = loc
		}

		d := model.Driver{
			ID:                  driverID,
			CurrentLocation:     loc,
			HomeLocation:        homeLoc,
			AvailableEpoch:      availEpoch,
			DriveHoursRemaining: 11.0,
			DutyHoursRemaining:  14.0,
			Equipment:           model.Equipment{Type: equipType},
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
	var loads []model.Load

	isHeader := true
	for scanner.Scan() {
		if maxLoads > 0 && len(loads) >= maxLoads {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if isHeader {
			isHeader = false
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 25 {
			continue
		}

		loadID := fields[0]
		origID := fields[10]
		destID := fields[17]

		pkupStDT := fields[12]
		pkupStTM := fields[13]
		pkupEndDT := fields[14]
		pkupEndTM := fields[15]

		dlvStDT := fields[19]
		dlvStTM := fields[20]
		dlvEndDT := fields[21]
		dlvEndTM := fields[22]

		linehaulRev, _ := strconv.ParseFloat(fields[24], 64)
		otherRev := 0.0
		if len(fields) >= 26 {
			otherRev, _ = strconv.ParseFloat(fields[25], 64)
		}
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
		if len(fields) >= 4 {
			switch strings.ToUpper(fields[3]) {
			case "REEFER", "REEFER_53":
				equipType = model.EquipReefer
			case "FLATBED", "FLATBED_53":
				equipType = model.EquipFlatbed
			case "TANKER":
				equipType = model.EquipTanker
			}
		}

		l := model.Load{
			ID:                    loadID,
			Origin:                origLoc,
			Destination:           destLoc,
			PickupEarliestEpoch:   pkupEarliest,
			PickupLatestEpoch:     pkupLatest,
			DeliveryEarliestEpoch: dlvEarliest,
			DeliveryLatestEpoch:   dlvLatest,
			RequiredEquipment:     equipType,
			Revenue:               totalRev,
		}

		loads = append(loads, l)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("legacy: failed reading loads: %w", err)
	}

	return loads, nil
}

// LoadCarrierScenario reads and parses a complete carrier test scenario directory.
func LoadCarrierScenario(
	locationsFile, driversFile, loadsFile string,
	maxLoads int,
) ([]model.Driver, []model.Load, *LocationStore, error) {
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
