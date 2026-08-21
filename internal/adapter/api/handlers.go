package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/optimaldynamics/project-mittens/internal/adapter/db"
	"github.com/optimaldynamics/project-mittens/internal/adapter/stream"
	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy/reposition"
	"github.com/optimaldynamics/project-mittens/internal/service"
	"github.com/optimaldynamics/project-mittens/internal/service/dispatch"
	"github.com/optimaldynamics/project-mittens/pkg/explain"
	pkgjournal "github.com/optimaldynamics/project-mittens/pkg/journal"
	"github.com/optimaldynamics/project-mittens/pkg/replay"
	"github.com/optimaldynamics/project-mittens/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
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

// HandlerDependencies provides explicitly injected persistence and processing engines.
type HandlerDependencies struct {
	Journal         service.Journal
	CryptoStore     pkgjournal.JournalStore
	DBPool          *db.Pool
	RunRepository   *db.PostgresRunRepository
	StreamBuffer    *stream.StreamBuffer
	StreamSync      *stream.StateSynchronizer
	RepositionSynth *reposition.RepositioningSynthesizer
}

// Handler provides HTTP request handling methods.
type Handler struct {
	state           *ServerState
	journal         service.Journal
	cryptoStore     pkgjournal.JournalStore
	dbPool          *db.Pool
	runRepo         *db.PostgresRunRepository
	streamBuffer    *stream.StreamBuffer
	streamSync      *stream.StateSynchronizer
	repositionSynth *reposition.RepositioningSynthesizer
}

// NewHandler initializes a new Handler with default in-memory engines.
func NewHandler(journal ...service.Journal) *Handler {
	var j service.Journal
	if len(journal) > 0 && journal[0] != nil {
		j = journal[0]
	} else {
		j = service.NewMemoryJournal()
	}
	buf := stream.NewStreamBuffer()
	return &Handler{
		state: &ServerState{
			StartTime: time.Now().UTC(),
		},
		journal:         j,
		cryptoStore:     pkgjournal.NewMemoryStore(),
		streamBuffer:    buf,
		streamSync:      stream.NewStateSynchronizer(buf),
		repositionSynth: reposition.NewRepositioningSynthesizer(),
	}
}

// NewHandlerWithDeps initializes a Handler with explicitly injected dependencies.
func NewHandlerWithDeps(deps HandlerDependencies) *Handler {
	j := deps.Journal
	if j == nil {
		j = service.NewMemoryJournal()
	}
	cs := deps.CryptoStore
	if cs == nil {
		cs = pkgjournal.NewMemoryStore()
	}
	buf := deps.StreamBuffer
	if buf == nil {
		buf = stream.NewStreamBuffer()
	}
	sync := deps.StreamSync
	if sync == nil {
		sync = stream.NewStateSynchronizer(buf)
	}
	synth := deps.RepositionSynth
	if synth == nil {
		synth = reposition.NewRepositioningSynthesizer()
	}

	return &Handler{
		state: &ServerState{
			StartTime: time.Now().UTC(),
		},
		journal:         j,
		cryptoStore:     cs,
		dbPool:          deps.DBPool,
		runRepo:         deps.RunRepository,
		streamBuffer:    buf,
		streamSync:      sync,
		repositionSynth: synth,
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
	dbStatus := "in-memory"
	if h.dbPool != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := h.dbPool.Ping(ctx); err != nil {
			dbStatus = "unreachable"
		} else {
			dbStatus = "connected"
		}
	}

	h.writeJSON(w, http.StatusOK, HealthResponse{
		Status:        "OK",
		Version:       "1.0.0",
		UptimeSeconds: uptime,
		Database:      dbStatus,
	})
}

// HandleOptimize executes single-epoch optimal fleet matching.
func (h *Handler) HandleOptimize(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.StartSpan(r.Context(), "HTTP.POST.api.v1.optimize")
	defer span.End()

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

	span.SetAttributes(telemetry.OptimizationSpanAttributes(req.PolicyClass, len(req.Drivers), len(req.Loads), req.CompetitorScale)...)

	if req.PolicyClass != "" {
		pUpper := strings.ToUpper(strings.TrimSpace(req.PolicyClass))
		if pUpper != "CFA" && pUpper != "PIECEWISEVFA" && pUpper != "VFA" && pUpper != "DLA" {
			h.writeError(w, http.StatusBadRequest, "UNSUPPORTED_POLICY", fmt.Sprintf("policy class '%s' is not supported; valid classes are CFA, PiecewiseVFA, DLA", req.PolicyClass))
			return
		}
	}
	if req.CompetitorScale < 0 {
		h.writeError(w, http.StatusBadRequest, "INVALID_COMPETITOR_SCALE", "competitor_scale must be >= 0")
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

	costCfg := model.DefaultCostConfig()
	if req.CostConfig != nil {
		costCfg.FixedCostPerLoad = req.CostConfig.FixedCostPerLoad
		costCfg.LoadedMileRate = req.CostConfig.LoadedMileRate
		costCfg.EmptyMileRate = req.CostConfig.EmptyMileRate
		costCfg.EmptyToHomeRate = req.CostConfig.EmptyToHomeRate
		costCfg.EarlyArrivalPerHour = req.CostConfig.EarlyArrivalPerHour
		costCfg.LateDeliveryPerHour = req.CostConfig.LateDeliveryPerHour
		if req.CostConfig.DriverBonusWeight > 0 {
			costCfg.DriverBonusWeight = req.CostConfig.DriverBonusWeight
		}
	}

	feasCfg := model.DefaultFeasibilityConfig()
	if req.FeasibilityConfig != nil {
		if req.FeasibilityConfig.MaxDeadheadMiles > 0 {
			feasCfg.MaxDeadheadMiles = req.FeasibilityConfig.MaxDeadheadMiles
		}
		if req.FeasibilityConfig.MaxEarlyDwellHours > 0 {
			feasCfg.MaxEarlyDwellHours = req.FeasibilityConfig.MaxEarlyDwellHours
		}
		if req.FeasibilityConfig.MaxLateDeliveryHours > 0 {
			feasCfg.MaxLateDeliveryHours = req.FeasibilityConfig.MaxLateDeliveryHours
		}
		if req.FeasibilityConfig.AverageSpeedMPH > 0 {
			feasCfg.AverageSpeedMPH = req.FeasibilityConfig.AverageSpeedMPH
		}
	}

	var action *model.Action
	var prov policy.DecisionProvenance

	if req.CompetitorScale > 0 {
		scale := model.AggregatedMarket{LatentStates: []string{"aggressive", "balanced", "defensive"}}
		compBelief, err := model.NewBelief(scale, scale.LatentStates, []float64{0.3333333333333333, 0.3333333333333333, 0.3333333333333334})
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "BELIEF_CREATION_FAILED", err.Error())
			return
		}
		compState, err := model.NewState(res, info, compBelief)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "INVALID_STATE", err.Error())
			return
		}

		var compPol policy.Policy[model.AggregatedMarket]
		switch strings.ToUpper(strings.TrimSpace(req.PolicyClass)) {
		case "PIECEWISEVFA", "VFA":
			compPol = policy.NewPiecewiseVFAPolicy[model.AggregatedMarket](
				nil,
				nil,
				0.95,
				costCfg,
				feasCfg,
				nil,
			)
		case "DLA":
			cfaBase := policy.NewCFAPolicy[model.AggregatedMarket](
				policy.DefaultCFAParameters(),
				costCfg,
				feasCfg,
				nil,
			)
			compPol = policy.NewDLAPolicy[model.AggregatedMarket](
				policy.DefaultDLAParameters(),
				costCfg,
				feasCfg,
				cfaBase,
				nil,
				nil,
				nil,
				nil,
			)
		default:
			compPol = policy.NewCFAPolicy[model.AggregatedMarket](
				policy.DefaultCFAParameters(),
				costCfg,
				feasCfg,
				nil,
			)
		}

		optService := service.NewOptimizationService[model.AggregatedMarket](h.journal, nil).WithCryptoStore(h.cryptoStore)
		action, prov, _, err = optService.OptimizeEpoch(ctx, compState, compPol, req.Epoch+3600, nil)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "OPTIMIZATION_FAILED", err.Error())
			return
		}
	} else {
		belief := model.NewMonopolisticBelief()
		state, err := model.NewState(res, info, belief)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "INVALID_STATE", err.Error())
			return
		}

		var pol policy.Policy[model.Monopolistic]
		switch strings.ToUpper(strings.TrimSpace(req.PolicyClass)) {
		case "PIECEWISEVFA", "VFA":
			pol = policy.NewPiecewiseVFAPolicy[model.Monopolistic](
				nil,
				nil,
				0.95,
				costCfg,
				feasCfg,
				nil,
			)
		case "DLA":
			cfaBase := policy.NewCFAPolicy[model.Monopolistic](
				policy.DefaultCFAParameters(),
				costCfg,
				feasCfg,
				nil,
			)
			pol = policy.NewDLAPolicy[model.Monopolistic](
				policy.DefaultDLAParameters(),
				costCfg,
				feasCfg,
				cfaBase,
				nil,
				nil,
				nil,
				nil,
			)
		default:
			pol = policy.NewCFAPolicy[model.Monopolistic](
				policy.DefaultCFAParameters(),
				costCfg,
				feasCfg,
				nil,
			)
		}

		optService := service.NewOptimizationService[model.Monopolistic](h.journal, nil).WithCryptoStore(h.cryptoStore)
		action, prov, _, err = optService.OptimizeEpoch(ctx, state, pol, req.Epoch+3600, nil)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "OPTIMIZATION_FAILED", err.Error())
			return
		}
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

	decisionID := prov.OptimizationRunID
	if decisionID == "" {
		decisionID = fmt.Sprintf("OPT_%d", req.Epoch)
	}

	policyName := prov.PolicyName
	if policyName == "" {
		policyName = req.PolicyClass
	}
	if policyName == "" {
		policyName = "CFA"
	}
	runID := fmt.Sprintf("RUN-%s", policyName)

	h.writeJSON(w, http.StatusOK, OptimizeResponse{
		DecisionID:           decisionID,
		RunID:                runID,
		Epoch:                req.Epoch,
		MatchCount:           len(matches),
		Matches:              matches,
		TotalNetContribution: prov.TotalNetContribution,
		ExecutionDurationMs:  durationMs,
	})
}

// HandleSimulate executes multi-epoch rolling horizon continuous simulation.
func (h *Handler) HandleSimulate(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.StartSpan(r.Context(), "HTTP.POST.api.v1.simulate")
	defer span.End()

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

	span.SetAttributes(telemetry.SimulationSpanAttributes(req.RunID, req.HorizonDays, 0)...)
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

	// Stream mapping relative to simulation decision step intervals
	loadMap := make(map[int64][]model.Load)
	stepSec := int64(req.DecisionStepHours * 3600)
	if stepSec <= 0 {
		stepSec = 86400
	}
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
		relativeSec := l.PickupEarliestEpoch - req.StartEpoch
		if relativeSec < 0 {
			relativeSec = 0
		}
		epochIndex := relativeSec / stepSec
		targetEpoch := req.StartEpoch + epochIndex*stepSec
		loadMap[targetEpoch] = append(loadMap[targetEpoch], l)
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

	report, _, err := simRunner.Run(ctx, cfg, state, cfaPol, stream)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "SIMULATION_FAILED", err.Error())
		return
	}

	span.SetAttributes(attribute.Int("simulation.epoch_count", report.TotalEpochs))

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

// HandleListDecisions returns a list of recorded optimization decisions from the Semantic Journal.
func (h *Handler) HandleListDecisions(w http.ResponseWriter, r *http.Request) {
	entries := h.journal.GetEntries()
	summaries := make([]DecisionSummaryDTO, len(entries))
	for i, entry := range entries {
		summaries[i] = DecisionSummaryDTO{
			DecisionID:           entry.DecisionID,
			BatchEpoch:           entry.BatchEpoch,
			PolicyName:           entry.PolicyName,
			MatchedCount:         entry.MatchedCount,
			TotalObjective:       entry.TotalObjective,
			TotalNetContribution: entry.TotalNetContribution,
		}
	}
	h.writeJSON(w, http.StatusOK, summaries)
}

// HandleGetDecision retrieves the raw Semantic Journal record for a specific decision.
func (h *Handler) HandleGetDecision(w http.ResponseWriter, r *http.Request) {
	decisionID := chi.URLParam(r, "id")
	if decisionID == "" {
		h.writeError(w, http.StatusBadRequest, "MISSING_DECISION_ID", "decision ID path parameter is required")
		return
	}

	entry, found := h.journal.GetEntry(decisionID)
	if !found {
		h.writeError(w, http.StatusNotFound, "DECISION_NOT_FOUND", fmt.Sprintf("no journal entry found for decision ID '%s'", decisionID))
		return
	}

	h.writeJSON(w, http.StatusOK, entry)
}

// HandleExplainDecision generates a comprehensive causal explainability report and counterfactual comparison.
func (h *Handler) HandleExplainDecision(w http.ResponseWriter, r *http.Request) {
	_, span := telemetry.StartSpan(r.Context(), "HTTP.GET.api.v1.decisions.explain")
	defer span.End()

	decisionID := chi.URLParam(r, "id")
	if decisionID == "" {
		h.writeError(w, http.StatusBadRequest, "MISSING_DECISION_ID", "decision ID path parameter is required")
		return
	}

	entry, found := h.journal.GetEntry(decisionID)
	if !found {
		h.writeError(w, http.StatusNotFound, "DECISION_NOT_FOUND", fmt.Sprintf("no journal entry found for decision ID '%s'", decisionID))
		return
	}

	explainer := explain.NewExplainer()
	formatter := explain.NewFormatter()

	explanation, err := explainer.ExplainDecision(entry.Provenance, nil, nil)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "EXPLANATION_FAILED", fmt.Sprintf("failed to generate explanation: %v", err))
		return
	}

	md := formatter.FormatMarkdown(explanation)

	span.SetAttributes(
		attribute.String("decision.id", decisionID),
		attribute.Int("decision.matched_drivers", explanation.MatchedDriversCount),
		attribute.Int("decision.idle_drivers", explanation.IdleDriversCount),
	)

	h.writeJSON(w, http.StatusOK, ExplainResponseDTO{
		DecisionID:  decisionID,
		Explanation: explanation,
		Markdown:    md,
	})
}

// HandleReplayDecision executes an offline bit-exact re-evaluation of a recorded optimization decision.
func (h *Handler) HandleReplayDecision(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.StartSpan(r.Context(), "HTTP.POST.api.v1.decisions.replay")
	defer span.End()

	decisionID := chi.URLParam(r, "id")
	if decisionID == "" {
		h.writeError(w, http.StatusBadRequest, "MISSING_DECISION_ID", "decision ID path parameter is required")
		return
	}

	entry, found := h.journal.GetEntry(decisionID)
	if !found {
		h.writeError(w, http.StatusNotFound, "DECISION_NOT_FOUND", fmt.Sprintf("no journal entry found for decision ID '%s'", decisionID))
		return
	}

	cryptoRec := entry.CryptographicRecord
	if cryptoRec.DecisionID == "" {
		if rec, err := h.cryptoStore.Get(decisionID); err == nil {
			cryptoRec = rec
		} else {
			h.writeError(w, http.StatusNotFound, "CRYPTO_RECORD_NOT_FOUND", fmt.Sprintf("no cryptographic record found for decision ID '%s'", decisionID))
			return
		}
	}

	if cryptoRec.PolicyName != "" && !strings.HasPrefix(strings.ToUpper(cryptoRec.PolicyName), "CFA") {
		h.writeError(w, http.StatusBadRequest, "UNSUPPORTED_REPLAY_POLICY", fmt.Sprintf("policy class '%s' is not supported via REST replay; only CFA is currently supported", cryptoRec.PolicyName))
		return
	}

	// Reconstruct state and policy from cryptographic bytes
	res, err := pkgjournal.DecodeCanonicalResource(cryptoRec.ResourceStateBytes)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STATE_DECODING_FAILED", fmt.Sprintf("failed decoding recorded resource state: %v", err))
		return
	}
	info, err := pkgjournal.DecodeCanonicalInformation(cryptoRec.InformationStateBytes)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STATE_DECODING_FAILED", fmt.Sprintf("failed decoding recorded information state: %v", err))
		return
	}
	belief, err := pkgjournal.DecodeCanonicalBelief(model.Monopolistic{}, cryptoRec.BeliefStateBytes)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STATE_DECODING_FAILED", fmt.Sprintf("failed decoding recorded belief state: %v", err))
		return
	}
	state, err := model.NewState(res, info, belief)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STATE_RECONSTRUCTION_FAILED", err.Error())
		return
	}

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	replayEngine, err := replay.NewReplayEngine[model.Monopolistic](cfaPol)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "REPLAY_ENGINE_INIT_FAILED", err.Error())
		return
	}

	report, err := replayEngine.ReplayDecision(ctx, cryptoRec, state)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "REPLAY_FAILED", err.Error())
		return
	}

	span.SetAttributes(
		attribute.String("decision.id", decisionID),
		attribute.Bool("replay.is_bit_exact", report.IsBitExact),
	)

	h.writeJSON(w, http.StatusOK, ReplayResponseDTO{
		DecisionID:                report.DecisionID,
		RunID:                     report.RunID,
		Epoch:                     report.Epoch,
		PolicyName:                report.PolicyName,
		IsBitExact:                report.IsBitExact,
		InitialStateHashMatch:     report.InitialStateHashMatch,
		ActionHashMatch:           report.ActionHashMatch,
		RecordedActionHash:        report.RecordedActionHash,
		ReplayedActionHash:        report.ReplayedActionHash,
		RecordedMatchesCount:      report.RecordedMatchesCount,
		ReplayedMatchesCount:      report.ReplayedMatchesCount,
		RecordedNetContribution:   report.RecordedNetContribution,
		ReplayedNetContribution:   report.ReplayedNetContribution,
		ContributionDelta:         report.ContributionDelta,
		ReplayDurationMicrosecond: report.ReplayDurationMicrosecond,
		DriftDetails:              report.DriftDetails,
	})
}

// HandleVerifyRunIntegrity validates the cryptographic hash chain continuity of an optimization run.
func (h *Handler) HandleVerifyRunIntegrity(w http.ResponseWriter, r *http.Request) {
	_, span := telemetry.StartSpan(r.Context(), "HTTP.GET.api.v1.runs.integrity")
	defer span.End()

	runID := chi.URLParam(r, "id")
	if runID == "" {
		h.writeError(w, http.StatusBadRequest, "MISSING_RUN_ID", "run ID path parameter is required")
		return
	}

	valid, lastHash, err := h.cryptoStore.VerifyRunChain(runID)
	status := "VALID"
	brokenID := ""
	if !valid || err != nil {
		status = "CORRUPTED"
		if err != nil {
			brokenID = err.Error()
		}
	}

	h.writeJSON(w, http.StatusOK, ChainIntegrityResponseDTO{
		RunID:            runID,
		IsValid:          valid,
		LatestRecordHash: lastHash,
		BrokenRecordID:   brokenID,
		Status:           status,
	})
}

// HandleStreamTelemetry ingests a batch of live ELD GPS and HOS clock updates into the stream buffer.
func (h *Handler) HandleStreamTelemetry(w http.ResponseWriter, r *http.Request) {
	_, span := telemetry.StartSpan(r.Context(), "HTTP.POST.api.v1.stream.telemetry")
	defer span.End()

	var req StreamTelemetryRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if len(req.Pings) == 0 {
		h.writeError(w, http.StatusBadRequest, "EMPTY_BATCH", "pings slice cannot be empty")
		return
	}

	if err := h.streamBuffer.IngestDriverBatch(req.Pings); err != nil {
		h.writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, StreamStatusResponseDTO{
		Status: h.streamBuffer.Status(),
	})
}

// HandleStreamTenders ingests a batch of new customer load tenders into the stream buffer.
func (h *Handler) HandleStreamTenders(w http.ResponseWriter, r *http.Request) {
	_, span := telemetry.StartSpan(r.Context(), "HTTP.POST.api.v1.stream.tenders")
	defer span.End()

	var req StreamTendersRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if len(req.Tenders) == 0 {
		h.writeError(w, http.StatusBadRequest, "EMPTY_BATCH", "tenders slice cannot be empty")
		return
	}

	if err := h.streamBuffer.IngestTenderBatch(req.Tenders); err != nil {
		h.writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, StreamStatusResponseDTO{
		Status: h.streamBuffer.Status(),
	})
}

// HandleStreamCancels processes incoming freight tender cancellation requests.
func (h *Handler) HandleStreamCancels(w http.ResponseWriter, r *http.Request) {
	_, span := telemetry.StartSpan(r.Context(), "HTTP.POST.api.v1.stream.cancels")
	defer span.End()

	var req StreamCancelsRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	for _, c := range req.Cancellations {
		if err := h.streamBuffer.CancelTender(c); err != nil {
			h.writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
			return
		}
	}

	h.writeJSON(w, http.StatusOK, StreamStatusResponseDTO{
		Status: h.streamBuffer.Status(),
	})
}

// HandleStreamStatus returns real-time metrics and queue depths of the streaming ingestion buffer.
func (h *Handler) HandleStreamStatus(w http.ResponseWriter, r *http.Request) {
	_, span := telemetry.StartSpan(r.Context(), "HTTP.GET.api.v1.stream.status")
	defer span.End()

	h.writeJSON(w, http.StatusOK, StreamStatusResponseDTO{
		Status: h.streamBuffer.Status(),
	})
}

// HandleRepositionPlan synthesizes empty tractor repositioning moves to balance regional freight capacity.
func (h *Handler) HandleRepositionPlan(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.StartSpan(r.Context(), "HTTP.POST.api.v1.reposition.plan")
	defer span.End()

	var req RepositionPlanRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if len(req.Drivers) == 0 {
		h.writeError(w, http.StatusBadRequest, "EMPTY_DRIVERS", "drivers slice cannot be empty")
		return
	}

	// Reconstruct drivers
	drivers := make([]model.Driver, len(req.Drivers))
	for i, dDTO := range req.Drivers {
		availEpoch := dDTO.AvailableEpoch
		if availEpoch <= 0 {
			availEpoch = time.Now().UTC().Unix()
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
			ID:                  dDTO.ID,
			CurrentLocation:     model.Location{NodeID: dDTO.CurrentLocation.NodeID, Lat: dDTO.CurrentLocation.Lat, Lon: dDTO.CurrentLocation.Lon},
			HomeLocation:        model.Location{NodeID: dDTO.HomeLocation.NodeID, Lat: dDTO.HomeLocation.Lat, Lon: dDTO.HomeLocation.Lon},
			AvailableEpoch:      availEpoch,
			DriveHoursRemaining: driveRem,
			DutyHoursRemaining:  dutyRem,
			Equipment:           model.Equipment{Type: parseEquipmentType(dDTO.Equipment.Type)},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(availEpoch, 0)),
		}
	}

	// Reconstruct loads
	loads := make([]model.Load, len(req.Loads))
	for i, lDTO := range req.Loads {
		loads[i] = model.Load{
			ID:                    lDTO.ID,
			Origin:                model.Location{NodeID: lDTO.Origin.NodeID, Lat: lDTO.Origin.Lat, Lon: lDTO.Origin.Lon},
			Destination:           model.Location{NodeID: lDTO.Destination.NodeID, Lat: lDTO.Destination.Lat, Lon: lDTO.Destination.Lon},
			PickupEarliestEpoch:   lDTO.PickupEarliestEpoch,
			PickupLatestEpoch:     lDTO.PickupLatestEpoch,
			DeliveryEarliestEpoch: lDTO.DeliveryEarliestEpoch,
			DeliveryLatestEpoch:   lDTO.DeliveryLatestEpoch,
			Revenue:               lDTO.Revenue,
			RequiredEquipment:     parseEquipmentType(lDTO.RequiredEquipment),
		}
	}

	resource := model.NewResourceState(drivers, loads)
	regionMgr := model.NewRegionManager(1.0, nil)

	cfg := reposition.DefaultRepositioningConfig()
	if req.Config != nil {
		cfg = *req.Config
	}

	moves, err := h.repositionSynth.SynthesizeRepositioningMoves(ctx, resource, regionMgr, drivers, cfg)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "REPOSITION_SYNTHESIS_FAILED", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, RepositionPlanResponseDTO{
		Moves:      moves,
		TotalMoves: len(moves),
		Summary:    reposition.SummaryString(moves),
	})
}

// HandleListScenarios returns the catalog of pre-packaged golden and operational scenarios.
func (h *Handler) HandleListScenarios(w http.ResponseWriter, r *http.Request) {
	scenarios := getPrepackagedScenarioSummaries()
	h.writeJSON(w, http.StatusOK, map[string]any{
		"scenarios": scenarios,
		"count":     len(scenarios),
	})
}

// HandleGetScenario returns the full initial state and configuration for a specified scenario ID.
func (h *Handler) HandleGetScenario(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "MISSING_SCENARIO_ID", "scenario ID is required")
		return
	}

	detail, found := getPrepackagedScenarioDetail(id)
	if !found {
		h.writeError(w, http.StatusNotFound, "SCENARIO_NOT_FOUND", fmt.Sprintf("scenario %s not found in catalog", id))
		return
	}

	h.writeJSON(w, http.StatusOK, detail)
}

func getPrepackagedScenarioSummaries() []ScenarioSummaryDTO {
	return []ScenarioSummaryDTO{
		{
			ID:            "07_test_dispatch",
			Name:          "07_test_dispatch (Monopolistic Parity Baseline)",
			Description:   "Authoritative Java-to-Go parity fixture evaluating single-epoch dispatch with 100% bitwise assignment equivalence.",
			Category:      "Golden Parity",
			DriverCount:   3,
			LoadCount:     2,
			DefaultPolicy: "CFA",
		},
		{
			ID:            "16_optimal_tours",
			Name:          "16_test_dispatch_optimal_tours (Multi-Leg Tours)",
			Description:   "Complex operational network evaluating spatial flow conservation and multi-leg chained tour synthesis.",
			Category:      "Multi-Leg Tours",
			DriverCount:   49,
			LoadCount:     362,
			DefaultPolicy: "CFA",
		},
		{
			ID:            "13_relays",
			Name:          "13_test_relays (Relay Interchange Coordination)",
			Description:   "Relay exchange network evaluating dual-driver handoffs at candidate terminal nodes.",
			Category:      "Relays",
			DriverCount:   100,
			LoadCount:     250,
			DefaultPolicy: "CFA",
		},
		{
			ID:            "05_home_time",
			Name:          "05_test_home_time (Scheduled Time-At-Home)",
			Description:   "Fleet scheduling subject to mandatory driver time-at-home windows and domicile return constraints.",
			Category:      "Driver Preferences",
			DriverCount:   29,
			LoadCount:     100,
			DefaultPolicy: "CFA",
		},
		{
			ID:            "14_preassignments",
			Name:          "14_test_pre_assignments (Locked Commitments)",
			Description:   "Operational matching with 258 preassigned driver-load pairs and unassigned spot market volume.",
			Category:      "Pre-Assignments",
			DriverCount:   187,
			LoadCount:     150,
			DefaultPolicy: "CFA",
		},
		{
			ID:            "17_geoconstraints",
			Name:          "17_test_driver_geo_constraints (Regional Boundaries)",
			Description:   "Carrier operational boundaries, regional domain filtering, and maximum deadhead distance limits.",
			Category:      "Geo Constraints",
			DriverCount:   207,
			LoadCount:     200,
			DefaultPolicy: "CFA",
		},
		{
			ID:            "15_ontime",
			Name:          "15_test_on_time_parameters (10-Day HOS Logs)",
			Description:   "Rolling multi-day historical HOS duty logs and tight delivery appointment time windows.",
			Category:      "HOS & Scheduling",
			DriverCount:   146,
			LoadCount:     150,
			DefaultPolicy: "CFA",
		},
		{
			ID:            "midwest_corridor",
			Name:          "Midwest Express Freight Corridor",
			Description:   "High-density freight corridor connecting Chicago, Detroit, Indianapolis, Columbus, and St. Louis.",
			Category:      "Live Demonstration",
			DriverCount:   12,
			LoadCount:     18,
			DefaultPolicy: "CFA",
		},
		{
			ID:            "transcon_reefer",
			Name:          "Transcontinental Temperature-Controlled Network",
			Description:   "Coast-to-coast refrigerated linehaul network with team transit times and strict temperature integrity.",
			Category:      "Live Demonstration",
			DriverCount:   16,
			LoadCount:     24,
			DefaultPolicy: "PiecewiseVFA",
		},
	}
}

func getPrepackagedScenarioDetail(id string) (ScenarioDetailDTO, bool) {
	summaries := getPrepackagedScenarioSummaries()
	var targetSummary *ScenarioSummaryDTO
	for _, s := range summaries {
		if s.ID == id {
			targetSummary = &s
			break
		}
	}
	if targetSummary == nil {
		return ScenarioDetailDTO{}, false
	}

	baseEpoch := time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC).Unix()

	// 1. Return tailored datasets based on scenario ID
	switch id {
	case "07_test_dispatch":
		locA := LocationDTO{NodeID: "QUIRC4_LOC", Lat: 41.50, Lon: -87.50}
		locB := LocationDTO{NodeID: "SMIVA2_LOC", Lat: 42.00, Lon: -88.00}
		locC := LocationDTO{NodeID: "SONRW2_LOC", Lat: 40.80, Lon: -87.20}
		locL1O := LocationDTO{NodeID: "LOAD1_ORIG", Lat: 41.55, Lon: -87.45}
		locL1D := LocationDTO{NodeID: "LOAD1_DEST", Lat: 42.10, Lon: -86.90}
		locL2O := LocationDTO{NodeID: "LOAD2_ORIG", Lat: 40.85, Lon: -87.15}
		locL2D := LocationDTO{NodeID: "LOAD2_DEST", Lat: 41.70, Lon: -86.50}

		drivers := []DriverDTO{
			{ID: "QUIRC4", CurrentLocation: locA, HomeLocation: locA, AvailableEpoch: baseEpoch, DriveHoursRemaining: 11.0, DutyHoursRemaining: 14.0, Equipment: EquipmentDTO{Type: "DRY_VAN"}},
			{ID: "SMIVA2", CurrentLocation: locB, HomeLocation: locB, AvailableEpoch: baseEpoch, DriveHoursRemaining: 11.0, DutyHoursRemaining: 14.0, Equipment: EquipmentDTO{Type: "DRY_VAN"}},
			{ID: "SONRW2", CurrentLocation: locC, HomeLocation: locC, AvailableEpoch: baseEpoch, DriveHoursRemaining: 11.0, DutyHoursRemaining: 14.0, Equipment: EquipmentDTO{Type: "DRY_VAN"}},
		}
		loads := []LoadDTO{
			{ID: "270391", Origin: locL2O, Destination: locL2D, PickupEarliestEpoch: baseEpoch, PickupLatestEpoch: baseEpoch + 14400, DeliveryEarliestEpoch: baseEpoch + 7200, DeliveryLatestEpoch: baseEpoch + 28800, Revenue: 925.00, RequiredEquipment: "DRY_VAN"},
			{ID: "270392", Origin: locL1O, Destination: locL1D, PickupEarliestEpoch: baseEpoch, PickupLatestEpoch: baseEpoch + 14400, DeliveryEarliestEpoch: baseEpoch + 7200, DeliveryLatestEpoch: baseEpoch + 28800, Revenue: 944.00, RequiredEquipment: "DRY_VAN"},
		}
		return ScenarioDetailDTO{
			Summary: *targetSummary,
			Drivers: drivers,
			Loads:   loads,
			CostConfig: &CostConfigDTO{
				FixedCostPerLoad:    50.0,
				LoadedMileRate:      1.50,
				EmptyMileRate:       1.20,
				EmptyToHomeRate:     0.30,
				EarlyArrivalPerHour: 25.0,
				LateDeliveryPerHour: 75.0,
			},
			FeasibilityConfig: &FeasibilityConfigDTO{
				MaxDeadheadMiles: 300.0,
				AverageSpeedMPH:  50.0,
			},
		}, true

	case "midwest_corridor":
		cities := map[string]LocationDTO{
			"CHI": {NodeID: "CHI", Lat: 41.8781, Lon: -87.6298},
			"DET": {NodeID: "DET", Lat: 42.3314, Lon: -83.0458},
			"IND": {NodeID: "IND", Lat: 39.7684, Lon: -86.1581},
			"CMH": {NodeID: "CMH", Lat: 39.9612, Lon: -82.9988},
			"STL": {NodeID: "STL", Lat: 38.6270, Lon: -90.1994},
			"MKE": {NodeID: "MKE", Lat: 43.0389, Lon: -87.9065},
			"CVG": {NodeID: "CVG", Lat: 39.1031, Lon: -84.5120},
			"GRR": {NodeID: "GRR", Lat: 42.9634, Lon: -85.6681},
		}
		driverCities := []string{"CHI", "CHI", "DET", "DET", "IND", "IND", "CMH", "CMH", "STL", "STL", "MKE", "CVG"}
		drivers := make([]DriverDTO, len(driverCities))
		for i, city := range driverCities {
			loc := cities[city]
			drivers[i] = DriverDTO{
				ID:                  fmt.Sprintf("DRV-MW-%02d", i+1),
				CurrentLocation:     loc,
				HomeLocation:        loc,
				AvailableEpoch:      baseEpoch + int64(i*900),
				DriveHoursRemaining: 10.5 + float64(i%2)*0.5,
				DutyHoursRemaining:  13.5 + float64(i%2)*0.5,
				Equipment:           EquipmentDTO{Type: "DRY_VAN"},
			}
		}

		loadPairs := [][2]string{
			{"CHI", "DET"}, {"CHI", "IND"}, {"CHI", "STL"}, {"CHI", "CMH"},
			{"DET", "CHI"}, {"DET", "CMH"}, {"DET", "IND"},
			{"IND", "CHI"}, {"IND", "STL"}, {"IND", "CVG"},
			{"CMH", "DET"}, {"CMH", "CHI"}, {"CMH", "CVG"},
			{"STL", "CHI"}, {"STL", "IND"}, {"MKE", "CHI"},
			{"CVG", "CMH"}, {"GRR", "DET"},
		}
		loads := make([]LoadDTO, len(loadPairs))
		for i, pair := range loadPairs {
			orig := cities[pair[0]]
			dest := cities[pair[1]]
			dist := (&model.Location{Lat: orig.Lat, Lon: orig.Lon}).DistanceMiles(model.Location{Lat: dest.Lat, Lon: dest.Lon})
			loads[i] = LoadDTO{
				ID:                    fmt.Sprintf("LOAD-MW-%03d", i+100),
				Origin:                orig,
				Destination:           dest,
				PickupEarliestEpoch:   baseEpoch + int64(i*1200),
				PickupLatestEpoch:     baseEpoch + int64(i*1200) + 18000,
				DeliveryEarliestEpoch: baseEpoch + int64(i*1200) + 7200,
				DeliveryLatestEpoch:   baseEpoch + int64(i*1200) + 43200,
				Revenue:               math.Round(dist*2.45 + 150.0),
				RequiredEquipment:     "DRY_VAN",
			}
		}
		return ScenarioDetailDTO{
			Summary:           *targetSummary,
			Drivers:           drivers,
			Loads:             loads,
			CostConfig:        &CostConfigDTO{FixedCostPerLoad: 40.0, LoadedMileRate: 1.45, EmptyMileRate: 1.15, EmptyToHomeRate: 0.40, EarlyArrivalPerHour: 20.0, LateDeliveryPerHour: 60.0},
			FeasibilityConfig: &FeasibilityConfigDTO{MaxDeadheadMiles: 250.0, AverageSpeedMPH: 52.0},
		}, true

	default:
		// Generate high-density structured network with authentic nodes
		networkCities := []struct {
			code string
			lat  float64
			lon  float64
		}{
			{"CHI", 41.8781, -87.6298}, {"ATL", 33.7490, -84.3880}, {"DAL", 32.7767, -96.7970},
			{"DEN", 39.7392, -104.9903}, {"LAX", 34.0522, -118.2437}, {"SEA", 47.6062, -122.3321},
			{"PHX", 33.4484, -112.0740}, {"MIA", 25.7617, -80.1918}, {"JAX", 30.3322, -81.6557},
			{"NYC", 40.7128, -74.0060}, {"BOS", 42.3601, -71.0589}, {"PHL", 39.9526, -75.1652},
			{"PIT", 40.4406, -79.9959}, {"IND", 39.7684, -86.1581}, {"STL", 38.6270, -90.1994},
			{"MCI", 39.0997, -94.5786}, {"MSP", 44.9778, -93.2650}, {"MEM", 35.1495, -90.0490},
			{"BNA", 36.1627, -86.7816}, {"CLT", 35.2271, -80.8431},
		}

		numDrivers := targetSummary.DriverCount
		if numDrivers > 50 {
			numDrivers = 50 // cap for interactive visual playground smoothness
		}
		numLoads := targetSummary.LoadCount
		if numLoads > 80 {
			numLoads = 80 // cap for visual smoothness
		}

		drivers := make([]DriverDTO, numDrivers)
		for i := 0; i < numDrivers; i++ {
			c := networkCities[i%len(networkCities)]
			loc := LocationDTO{NodeID: c.code, Lat: c.lat + float64(i%3-1)*0.05, Lon: c.lon + float64(i%3-1)*0.05}
			equip := "DRY_VAN"
			if id == "transcon_reefer" || i%4 == 0 {
				equip = "REEFER"
			}
			drivers[i] = DriverDTO{
				ID:                  fmt.Sprintf("DRV-%s-%02d", targetSummary.ID, i+1),
				CurrentLocation:     loc,
				HomeLocation:        loc,
				AvailableEpoch:      baseEpoch + int64(i%5)*1800,
				DriveHoursRemaining: 11.0,
				DutyHoursRemaining:  14.0,
				Equipment:           EquipmentDTO{Type: equip},
			}
		}

		loads := make([]LoadDTO, numLoads)
		for i := 0; i < numLoads; i++ {
			origC := networkCities[i%len(networkCities)]
			destC := networkCities[(i*3+7)%len(networkCities)]
			origLoc := LocationDTO{NodeID: origC.code, Lat: origC.lat + float64(i%3-1)*0.04, Lon: origC.lon + float64(i%3-1)*0.04}
			destLoc := LocationDTO{NodeID: destC.code, Lat: destC.lat, Lon: destC.lon}
			dist := (&model.Location{Lat: origLoc.Lat, Lon: origLoc.Lon}).DistanceMiles(model.Location{Lat: destLoc.Lat, Lon: destLoc.Lon})
			equip := "DRY_VAN"
			if id == "transcon_reefer" || i%3 == 0 {
				equip = "REEFER"
			}
			loads[i] = LoadDTO{
				ID:                    fmt.Sprintf("LOAD-%s-%03d", targetSummary.ID, i+1),
				Origin:                origLoc,
				Destination:           destLoc,
				PickupEarliestEpoch:   baseEpoch + int64(i%8)*3600,
				PickupLatestEpoch:     baseEpoch + int64(i%8)*3600 + 28800,
				DeliveryEarliestEpoch: baseEpoch + int64(i%8)*3600 + 18000,
				DeliveryLatestEpoch:   baseEpoch + int64(i%8)*3600 + 86400,
				Revenue:               math.Round(dist*2.20 + 200.0),
				RequiredEquipment:     equip,
			}
		}

		return ScenarioDetailDTO{
			Summary:           *targetSummary,
			Drivers:           drivers,
			Loads:             loads,
			CostConfig:        &CostConfigDTO{FixedCostPerLoad: 50.0, LoadedMileRate: 1.50, EmptyMileRate: 1.20, EmptyToHomeRate: 0.35, EarlyArrivalPerHour: 20.0, LateDeliveryPerHour: 60.0},
			FeasibilityConfig: &FeasibilityConfigDTO{MaxDeadheadMiles: 400.0, AverageSpeedMPH: 50.0},
		}, true
	}
}
