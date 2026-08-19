package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/internal/service"
	"github.com/optimaldynamics/project-mittens/internal/service/dispatch"
)

// parseEquipmentType maps incoming equipment strings into domain EquipmentType.
func parseEquipmentType(eqStr string) model.EquipmentType {
	switch strings.ToUpper(strings.TrimSpace(eqStr)) {
	case "REEFER", "REEFER_53", "R":
		return model.EquipReefer
	case "FLATBED", "FLATBED_53", "FB":
		return model.EquipFlatbed
	case "TANKER":
		return model.EquipTanker
	default:
		return model.EquipDryVan
	}
}

// ServerState holds in-memory operational metrics and uptime.
type ServerState struct {
	StartTime          time.Time
	RequestsTotal      atomic.Uint64
	OptimizeCallsTotal atomic.Uint64
	SimulateCallsTotal atomic.Uint64
}

// Handler provides HTTP request handling methods.
type Handler struct {
	state *ServerState
}

// NewHandler initializes a new Handler.
func NewHandler() *Handler {
	return &Handler{
		state: &ServerState{
			StartTime: time.Now().UTC(),
		},
	}
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, msg string) {
	h.writeJSON(w, status, ErrorResponse{Code: code, Message: msg})
}

// HandleHealth serves the /healthz endpoint.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.state.StartTime).Seconds()
	h.writeJSON(w, http.StatusOK, HealthResponse{
		Status:        "OK",
		Version:       "1.0.0",
		UptimeSeconds: uptime,
	})
}

// HandleMetrics serves the /metrics Prometheus text endpoint.
func (h *Handler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.state.StartTime).Seconds()
	reqs := h.state.RequestsTotal.Load()
	opts := h.state.OptimizeCallsTotal.Load()
	sims := h.state.SimulateCallsTotal.Load()

	metricsText := fmt.Sprintf(`# HELP mittens_requests_total Total number of HTTP requests processed
# TYPE mittens_requests_total counter
mittens_requests_total %d
# HELP mittens_optimize_calls_total Total number of single-epoch optimizations
# TYPE mittens_optimize_calls_total counter
mittens_optimize_calls_total %d
# HELP mittens_simulate_calls_total Total number of rolling simulations
# TYPE mittens_simulate_calls_total counter
mittens_simulate_calls_total %d
# HELP mittens_uptime_seconds Server uptime in seconds
# TYPE mittens_uptime_seconds gauge
mittens_uptime_seconds %.2f
`, reqs, opts, sims, uptime)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(metricsText))
}

// HandleOptimize executes single-epoch optimal fleet matching.
func (h *Handler) HandleOptimize(w http.ResponseWriter, r *http.Request) {
	h.state.RequestsTotal.Add(1)
	h.state.OptimizeCallsTotal.Add(1)

	var req OptimizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if req.Epoch <= 0 {
		req.Epoch = time.Now().UTC().Unix()
	}
	if len(req.Drivers) == 0 {
		h.writeError(w, http.StatusBadRequest, "EMPTY_FLEET", "drivers array cannot be empty")
		return
	}

	if req.PolicyClass != "" && strings.ToUpper(req.PolicyClass) != "CFA" {
		h.writeError(w, http.StatusBadRequest, "UNSUPPORTED_POLICY", fmt.Sprintf("policy class '%s' is not supported via REST; only CFA is currently supported", req.PolicyClass))
		return
	}
	if req.CompetitorScale != 0 {
		h.writeError(w, http.StatusBadRequest, "UNSUPPORTED_COMPETITOR_SCALE", "competitor_scale > 0 is not yet supported via REST optimize endpoint; use 0 (monopolistic)")
		return
	}

	startTime := time.Now()

	// Convert DTOs to Domain Models
	drivers := make([]model.Driver, len(req.Drivers))
	for i, dDTO := range req.Drivers {
		equipType := parseEquipmentType(dDTO.Equipment.Type)

		availEpoch := dDTO.AvailableEpoch
		if availEpoch <= 0 {
			availEpoch = req.Epoch
		}

		driveRem := dDTO.DriveHoursRemaining
		if driveRem <= 0 {
			driveRem = 11.0
		}
		dutyRem := dDTO.DutyHoursRemaining
		if dutyRem <= 0 {
			dutyRem = 14.0
		}

		drivers[i] = model.Driver{
			ID: dDTO.ID,
			CurrentLocation: model.Location{
				NodeID: dDTO.CurrentLocation.NodeID,
				Lat:    dDTO.CurrentLocation.Lat,
				Lon:    dDTO.CurrentLocation.Lon,
			},
			HomeLocation: model.Location{
				NodeID: dDTO.HomeLocation.NodeID,
				Lat:    dDTO.HomeLocation.Lat,
				Lon:    dDTO.HomeLocation.Lon,
			},
			AvailableEpoch:      availEpoch,
			DriveHoursRemaining: driveRem,
			DutyHoursRemaining:  dutyRem,
			Equipment:           model.Equipment{Type: equipType},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(availEpoch, 0)),
		}
	}

	loads := make([]model.Load, len(req.Loads))
	for j, lDTO := range req.Loads {
		equipType := parseEquipmentType(lDTO.RequiredEquipment)

		loads[j] = model.Load{
			ID: lDTO.ID,
			Origin: model.Location{
				NodeID: lDTO.Origin.NodeID,
				Lat:    lDTO.Origin.Lat,
				Lon:    lDTO.Origin.Lon,
			},
			Destination: model.Location{
				NodeID: lDTO.Destination.NodeID,
				Lat:    lDTO.Destination.Lat,
				Lon:    lDTO.Destination.Lon,
			},
			PickupEarliestEpoch:   lDTO.PickupEarliestEpoch,
			PickupLatestEpoch:     lDTO.PickupLatestEpoch,
			DeliveryEarliestEpoch: lDTO.DeliveryEarliestEpoch,
			DeliveryLatestEpoch:   lDTO.DeliveryLatestEpoch,
			Revenue:               lDTO.Revenue,
			RequiredEquipment:     equipType,
		}
	}

	res := model.NewResourceState(drivers, loads)
	info, err := model.NewInformationState(req.Epoch, 1.0, 2.50, 0)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_INFO_STATE", err.Error())
		return
	}
	belief := model.NewMonopolisticBelief()
	state, err := model.NewState(res, info, belief)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_STATE", err.Error())
		return
	}

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	action, prov, err := cfaPol.Evaluate(r.Context(), state)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "OPTIMIZATION_FAILED", err.Error())
		return
	}

	matches := make([]MatchDTO, 0, action.MatchCount())
	for _, m := range action.Matches() {
		matches = append(matches, MatchDTO{
			DriverID:              m.DriverID,
			LoadID:                m.LoadID,
			DispatchEpoch:         m.DispatchEpoch,
			EstimatedContribution: m.EstimatedContribution,
		})
	}

	durationMs := float64(time.Since(startTime).Microseconds()) / 1000.0

	h.writeJSON(w, http.StatusOK, OptimizeResponse{
		RunID:                fmt.Sprintf("OPT_%d", req.Epoch),
		Epoch:                req.Epoch,
		MatchCount:           len(matches),
		Matches:              matches,
		TotalNetContribution: prov.TotalNetContribution,
		ExecutionDurationMs:  durationMs,
	})
}

// HandleSimulate executes multi-epoch rolling horizon continuous simulation.
func (h *Handler) HandleSimulate(w http.ResponseWriter, r *http.Request) {
	h.state.RequestsTotal.Add(1)
	h.state.SimulateCallsTotal.Add(1)

	var req SimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if req.RunID == "" {
		req.RunID = fmt.Sprintf("SIM_%d", time.Now().Unix())
	}
	if req.StartEpoch <= 0 {
		req.StartEpoch = time.Now().UTC().Unix()
	}
	if req.HorizonDays <= 0 {
		req.HorizonDays = 7
	}
	if req.DecisionStepHours <= 0 {
		req.DecisionStepHours = 24
	}
	if req.MinRelayHaulMiles <= 0 {
		req.MinRelayHaulMiles = 450.0
	}

	// Drivers
	drivers := make([]model.Driver, len(req.Drivers))
	for i, dDTO := range req.Drivers {
		availEpoch := dDTO.AvailableEpoch
		if availEpoch <= 0 {
			availEpoch = req.StartEpoch
		}
		driveRem := dDTO.DriveHoursRemaining
		if driveRem <= 0 {
			driveRem = 11.0
		}
		dutyRem := dDTO.DutyHoursRemaining
		if dutyRem <= 0 {
			dutyRem = 14.0
		}
		equipType := parseEquipmentType(dDTO.Equipment.Type)

		drivers[i] = model.Driver{
			ID:                  dDTO.ID,
			CurrentLocation:     model.Location{NodeID: dDTO.CurrentLocation.NodeID, Lat: dDTO.CurrentLocation.Lat, Lon: dDTO.CurrentLocation.Lon},
			HomeLocation:        model.Location{NodeID: dDTO.HomeLocation.NodeID, Lat: dDTO.HomeLocation.Lat, Lon: dDTO.HomeLocation.Lon},
			AvailableEpoch:      availEpoch,
			DriveHoursRemaining: driveRem,
			DutyHoursRemaining:  dutyRem,
			Equipment:           model.Equipment{Type: equipType},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(availEpoch, 0)),
		}
	}

	// Facilities
	var facs []model.Facility
	for _, fDTO := range req.Facilities {
		facs = append(facs, model.Facility{
			ID:                  fDTO.ID,
			Name:                fDTO.Name,
			Location:            model.Location{NodeID: fDTO.Location.NodeID, Lat: fDTO.Location.Lat, Lon: fDTO.Location.Lon},
			Type:                model.FacilityRelayHub,
			OpenMinutesOfDay:    0,
			CloseMinutesOfDay:   1440,
			AverageDwellMinutes: 60,
		})
	}
	facStore := model.NewFacilityStore(facs)

	// Stream mapping
	loadMap := make(map[int64][]model.Load)
	for _, lDTO := range req.LoadSchedule {
		equipType := parseEquipmentType(lDTO.RequiredEquipment)
		l := model.Load{
			ID:                    lDTO.ID,
			Origin:                model.Location{NodeID: lDTO.Origin.NodeID, Lat: lDTO.Origin.Lat, Lon: lDTO.Origin.Lon},
			Destination:           model.Location{NodeID: lDTO.Destination.NodeID, Lat: lDTO.Destination.Lat, Lon: lDTO.Destination.Lon},
			PickupEarliestEpoch:   lDTO.PickupEarliestEpoch,
			PickupLatestEpoch:     lDTO.PickupLatestEpoch,
			DeliveryEarliestEpoch: lDTO.DeliveryEarliestEpoch,
			DeliveryLatestEpoch:   lDTO.DeliveryLatestEpoch,
			Revenue:               lDTO.Revenue,
			RequiredEquipment:     equipType,
		}
		bucket := (l.PickupEarliestEpoch / 86400) * 86400
		loadMap[bucket] = append(loadMap[bucket], l)
	}
	stream := service.NewStaticLoadStream(loadMap)

	res := model.NewResourceState(drivers, nil)
	info, _ := model.NewInformationState(req.StartEpoch, 1.0, 2.50, 0)
	belief := model.NewMonopolisticBelief()
	state, err := model.NewState(res, info, belief)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_STATE", err.Error())
		return
	}

	relaySynth := policy.NewRelaySynthesizer(model.DefaultCostConfig(), policy.DefaultRelayConfig(), hos.USPolicySpecs(), facStore, nil)
	relayRunner := dispatch.NewRelayDispatchRunner(nil, relaySynth)
	cfaPol := policy.NewCFAPolicy[model.Monopolistic](policy.DefaultCFAParameters(), model.DefaultCostConfig(), model.DefaultFeasibilityConfig(), nil)
	simRunner := service.NewRollingHorizonRunner[model.Monopolistic](nil, relayRunner, nil, nil)

	cfg := service.RollingHorizonConfig{
		RunID:             req.RunID,
		StartEpoch:        req.StartEpoch,
		HorizonDays:       req.HorizonDays,
		DecisionStepHours: req.DecisionStepHours,
		EnableRelays:      req.EnableRelays,
		MinRelayHaulMiles: req.MinRelayHaulMiles,
		EnableVFALearning: false,
	}

	report, _, err := simRunner.Run(r.Context(), cfg, state, cfaPol, stream)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "SIMULATION_FAILED", err.Error())
		return
	}

	dailyKPIs := make([]DailyKPISnapshotDTO, len(report.DailySnapshots))
	for k, snap := range report.DailySnapshots {
		dailyKPIs[k] = DailyKPISnapshotDTO{
			DayIndex:           snap.DayIndex,
			Epoch:              snap.Epoch,
			ActiveDrivers:      snap.ActiveDrivers,
			TotalLoadedMiles:   snap.TotalLoadedMiles,
			TotalEmptyMiles:    snap.TotalEmptyMiles,
			EmptyMileRatio:     snap.EmptyRatio,
			GrossRevenue:       snap.TotalGrossRevenue,
			TotalCost:          snap.TotalOperatingCost,
			NetContribution:    snap.TotalNetContribution,
			DirectTourCount:    snap.DirectToursCount,
			RelayExchangeCount: snap.RelayExchangesCount,
		}
	}

	h.writeJSON(w, http.StatusOK, SimulateResponse{
		RunID:                     report.RunID,
		TotalDays:                 report.TotalDays,
		TotalEpochs:               report.TotalEpochs,
		CumulativeLoadedMiles:     report.TotalLoadedMiles,
		CumulativeEmptyMiles:      report.TotalEmptyMiles,
		OverallEmptyRatio:         report.GlobalEmptyRatio,
		CumulativeGrossRevenue:    report.TotalGrossRevenue,
		CumulativeCost:            report.TotalOperatingCost,
		CumulativeNetContribution: report.TotalNetContribution,
		DailyKPIs:                 dailyKPIs,
	})
}
