package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/adapter/api"
)

func TestAPI_HealthAndMetrics(t *testing.T) {
	srv := api.NewServer(api.DefaultServerConfig())

	// 1. Test /healthz
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /healthz, got %d", rr.Code)
	}

	var health api.HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&health); err != nil {
		t.Fatalf("failed decoding health response: %v", err)
	}
	if health.Status != "OK" || health.Version != "1.0.0" {
		t.Errorf("unexpected health body: %+v", health)
	}

	// 2. Test /metrics
	mReq, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	mRR := httptest.NewRecorder()
	srv.Router().ServeHTTP(mRR, mReq)

	if mRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /metrics, got %d", mRR.Code)
	}

	mText := mRR.Body.String()
	if !strings.Contains(mText, "project-mittens") {
		t.Errorf("expected project-mittens service_name in Prometheus metrics, got: %s", mText)
	}
}

func TestAPI_OptimizeEndpoint(t *testing.T) {
	srv := api.NewServer(api.DefaultServerConfig())
	now := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	reqPayload := api.OptimizeRequest{
		Epoch: now,
		Drivers: []api.DriverDTO{
			{
				ID:                  "DRV_01",
				CurrentLocation:     api.LocationDTO{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298},
				HomeLocation:        api.LocationDTO{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298},
				AvailableEpoch:      now,
				DriveHoursRemaining: 11.0,
				DutyHoursRemaining:  14.0,
				Equipment:           api.EquipmentDTO{Type: "DRY_VAN"},
			},
			{
				ID:                  "DRV_02",
				CurrentLocation:     api.LocationDTO{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880},
				HomeLocation:        api.LocationDTO{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880},
				AvailableEpoch:      now,
				DriveHoursRemaining: 11.0,
				DutyHoursRemaining:  14.0,
				Equipment:           api.EquipmentDTO{Type: "DRY_VAN"},
			},
		},
		Loads: []api.LoadDTO{
			{
				ID:                    "LOAD_CHI_IND",
				Origin:                api.LocationDTO{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298},
				Destination:           api.LocationDTO{NodeID: "IND", Lat: 39.7684, Lon: -86.1581},
				PickupEarliestEpoch:   now,
				PickupLatestEpoch:     now + 36000,
				DeliveryEarliestEpoch: now + 18000,
				DeliveryLatestEpoch:   now + 120000,
				Revenue:               1800.0,
				RequiredEquipment:     "DRY_VAN",
			},
			{
				ID:                    "LOAD_ATL_BHM",
				Origin:                api.LocationDTO{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880},
				Destination:           api.LocationDTO{NodeID: "BHM", Lat: 33.5186, Lon: -86.8104},
				PickupEarliestEpoch:   now,
				PickupLatestEpoch:     now + 36000,
				DeliveryEarliestEpoch: now + 18000,
				DeliveryLatestEpoch:   now + 120000,
				Revenue:               1600.0,
				RequiredEquipment:     "DRY_VAN",
			},
		},
	}

	bodyBytes, _ := json.Marshal(reqPayload)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/optimize", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/optimize, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var optResp api.OptimizeResponse
	if err := json.NewDecoder(rr.Body).Decode(&optResp); err != nil {
		t.Fatalf("failed decoding optimize response: %v", err)
	}

	t.Logf("API Optimize Response: %d matches, Net Contribution: $%.2f, Duration: %.2fms",
		optResp.MatchCount, optResp.TotalNetContribution, optResp.ExecutionDurationMs)

	if optResp.MatchCount != 2 {
		t.Errorf("expected 2 matches, got %d", optResp.MatchCount)
	}
	if optResp.TotalNetContribution <= 0 {
		t.Errorf("expected positive contribution, got $%.2f", optResp.TotalNetContribution)
	}
	if optResp.ExecutionDurationMs <= 0 {
		t.Errorf("expected positive execution duration")
	}
}

func TestAPI_OptimizeEndpoint_ValidationErrors(t *testing.T) {
	srv := api.NewServer(api.DefaultServerConfig())

	// 1. Empty fleet error
	emptyReq := api.OptimizeRequest{Epoch: 1000, Drivers: []api.DriverDTO{}}
	b1, _ := json.Marshal(emptyReq)
	req1, _ := http.NewRequest(http.MethodPost, "/api/v1/optimize", bytes.NewReader(b1))
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on empty drivers, got %d", rr1.Code)
	}

	// 2. Malformed JSON
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/optimize", bytes.NewReader([]byte("{invalid-json")))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on malformed JSON, got %d", rr2.Code)
	}

	// 3. Unsupported policy class
	badPolicyReq := api.OptimizeRequest{
		Epoch:       1000,
		PolicyClass: "UNKNOWN_POLICY",
		Drivers: []api.DriverDTO{
			{ID: "D1", CurrentLocation: api.LocationDTO{NodeID: "A"}, HomeLocation: api.LocationDTO{NodeID: "A"}},
		},
	}
	b3, _ := json.Marshal(badPolicyReq)
	req3, _ := http.NewRequest(http.MethodPost, "/api/v1/optimize", bytes.NewReader(b3))
	req3.Header.Set("Content-Type", "application/json")
	rr3 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on unknown policy, got %d", rr3.Code)
	}

	// 4. Unsupported competitor scale (>0)
	compReq := api.OptimizeRequest{
		Epoch:           1000,
		CompetitorScale: 3,
		Drivers: []api.DriverDTO{
			{ID: "D1", CurrentLocation: api.LocationDTO{NodeID: "A"}, HomeLocation: api.LocationDTO{NodeID: "A"}},
		},
	}
	b4, _ := json.Marshal(compReq)
	req4, _ := http.NewRequest(http.MethodPost, "/api/v1/optimize", bytes.NewReader(b4))
	req4.Header.Set("Content-Type", "application/json")
	rr4 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr4, req4)

	if rr4.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on competitor_scale > 0, got %d", rr4.Code)
	}
}

func TestAPI_SimulateEndpoint(t *testing.T) {
	srv := api.NewServer(api.DefaultServerConfig())

	startEpoch := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()
	locChi := api.LocationDTO{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := api.LocationDTO{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	locSdf := api.LocationDTO{NodeID: "SDF", Lat: 38.2527, Lon: -85.7585}

	simReq := api.SimulateRequest{
		RunID:             "SIM_HTTP_TEST",
		StartEpoch:        startEpoch,
		HorizonDays:       3,
		DecisionStepHours: 24,
		EnableRelays:      true,
		MinRelayHaulMiles: 400.0,
		Drivers: []api.DriverDTO{
			{ID: "DRV_1", CurrentLocation: locChi, HomeLocation: locChi, AvailableEpoch: startEpoch},
			{ID: "DRV_2", CurrentLocation: locAtl, HomeLocation: locAtl, AvailableEpoch: startEpoch},
			{ID: "DRV_3", CurrentLocation: locSdf, HomeLocation: locSdf, AvailableEpoch: startEpoch},
		},
		Facilities: []api.FacilityDTO{
			{ID: "FAC_SDF_HUB", Location: locSdf, Type: "RELAY_HUB", AverageDwellMinutes: 60},
		},
		LoadSchedule: []api.LoadDTO{
			{ID: "L_D1", Origin: locChi, Destination: locAtl, PickupEarliestEpoch: startEpoch, PickupLatestEpoch: startEpoch + 36000, DeliveryLatestEpoch: startEpoch + 120000, Revenue: 3200.0},
			{ID: "L_D2", Origin: locAtl, Destination: locChi, PickupEarliestEpoch: startEpoch + 86400, PickupLatestEpoch: startEpoch + 122400, DeliveryLatestEpoch: startEpoch + 200000, Revenue: 3100.0},
			{ID: "L_D3", Origin: locChi, Destination: locAtl, PickupEarliestEpoch: startEpoch + 172800, PickupLatestEpoch: startEpoch + 208800, DeliveryLatestEpoch: startEpoch + 300000, Revenue: 3300.0},
		},
	}

	bodyBytes, _ := json.Marshal(simReq)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/simulate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/simulate, got %d", rr.Code)
	}

	var simResp api.SimulateResponse
	if err := json.NewDecoder(rr.Body).Decode(&simResp); err != nil {
		t.Fatalf("failed decoding simulate response: %v", err)
	}

	t.Logf("API Simulate Response: RunID=%s, TotalDays=%d, TotalEpochs=%d, Revenue=$%.2f, Net=$%.2f, DailyKPIs=%d",
		simResp.RunID, simResp.TotalDays, simResp.TotalEpochs, simResp.CumulativeGrossRevenue, simResp.CumulativeNetContribution, len(simResp.DailyKPIs))

	if simResp.TotalDays != 3 || simResp.TotalEpochs != 3 {
		t.Errorf("expected 3 days and 3 epochs, got days=%d, epochs=%d", simResp.TotalDays, simResp.TotalEpochs)
	}
	if len(simResp.DailyKPIs) != 3 {
		t.Errorf("expected 3 daily KPI records, got %d", len(simResp.DailyKPIs))
	}
}

func TestAPI_ServerLifecycle(t *testing.T) {
	cfg := api.DefaultServerConfig()
	cfg.Port = 0 // Auto-bind port
	srv := api.NewServer(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestAPI_SemanticJournalAndExplainability(t *testing.T) {
	srv := api.NewServer(api.DefaultServerConfig())

	reqPayload := api.OptimizeRequest{
		Epoch:       1700000000,
		PolicyClass: "CFA",
		Drivers: []api.DriverDTO{
			{
				ID: "D1",
				CurrentLocation: api.LocationDTO{
					NodeID: "NYC",
					Lat:    40.7128,
					Lon:    -74.0060,
				},
				HomeLocation: api.LocationDTO{
					NodeID: "NYC",
					Lat:    40.7128,
					Lon:    -74.0060,
				},
				Equipment: api.EquipmentDTO{
					Type: "DRY_VAN_53",
				},
				AvailableEpoch:      1700000000,
				DriveHoursRemaining: 11.0,
				DutyHoursRemaining:  14.0,
			},
		},
		Loads: []api.LoadDTO{
			{
				ID: "L1",
				Origin: api.LocationDTO{
					NodeID: "NYC",
					Lat:    40.7128,
					Lon:    -74.0060,
				},
				Destination: api.LocationDTO{
					NodeID: "CHI",
					Lat:    41.8781,
					Lon:    -87.6298,
				},
				RequiredEquipment:     "DRY_VAN_53",
				PickupEarliestEpoch:   1700000000,
				PickupLatestEpoch:     17000036000,
				DeliveryEarliestEpoch: 1700010000,
				DeliveryLatestEpoch:   1700000000 + 150000,
				Revenue:               2500.0,
			},
		},
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		t.Fatalf("failed marshaling request: %v", err)
	}

	// 1. POST /api/v1/optimize
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/optimize", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	var optResp api.OptimizeResponse
	if err := json.NewDecoder(rr.Body).Decode(&optResp); err != nil {
		t.Fatalf("failed decoding optimize response: %v", err)
	}
	if optResp.DecisionID == "" {
		t.Fatalf("expected non-empty DecisionID in optimize response")
	}

	// 2. Test GET /api/v1/decisions
	listReq, _ := http.NewRequest(http.MethodGet, "/api/v1/decisions", nil)
	listRR := httptest.NewRecorder()
	srv.Router().ServeHTTP(listRR, listReq)

	if listRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /api/v1/decisions, got %d", listRR.Code)
	}

	var summaries []api.DecisionSummaryDTO
	if err := json.NewDecoder(listRR.Body).Decode(&summaries); err != nil {
		t.Fatalf("failed decoding decisions list: %v", err)
	}
	if len(summaries) == 0 {
		t.Fatalf("expected at least 1 decision summary, got 0")
	}
	if summaries[0].DecisionID != optResp.DecisionID {
		t.Errorf("expected decision ID %s, got %s", optResp.DecisionID, summaries[0].DecisionID)
	}

	// 3. Test GET /api/v1/decisions/{id}
	getReq, _ := http.NewRequest(http.MethodGet, "/api/v1/decisions/"+optResp.DecisionID, nil)
	getRR := httptest.NewRecorder()
	srv.Router().ServeHTTP(getRR, getReq)

	if getRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /api/v1/decisions/{id}, got %d", getRR.Code)
	}

	// 4. Test GET /api/v1/decisions/{id}/explain
	expReq, _ := http.NewRequest(http.MethodGet, "/api/v1/decisions/"+optResp.DecisionID+"/explain", nil)
	expRR := httptest.NewRecorder()
	srv.Router().ServeHTTP(expRR, expReq)

	if expRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /api/v1/decisions/{id}/explain, got %d", expRR.Code)
	}

	var explainDTO api.ExplainResponseDTO
	if err := json.NewDecoder(expRR.Body).Decode(&explainDTO); err != nil {
		t.Fatalf("failed decoding explain response: %v", err)
	}
	if explainDTO.DecisionID != optResp.DecisionID {
		t.Errorf("expected decision ID %s in explanation, got %s", optResp.DecisionID, explainDTO.DecisionID)
	}
	if !strings.Contains(explainDTO.Markdown, "Optimization Decision Explanation") {
		t.Errorf("expected markdown header in explain response, got: %s", explainDTO.Markdown)
	}
	if !strings.Contains(explainDTO.Markdown, "D1") || !strings.Contains(explainDTO.Markdown, "L1") {
		t.Errorf("expected driver D1 and load L1 in explain markdown, got: %s", explainDTO.Markdown)
	}

	// 5. Test POST /api/v1/decisions/{id}/replay
	replayReq, _ := http.NewRequest(http.MethodPost, "/api/v1/decisions/"+optResp.DecisionID+"/replay", nil)
	replayRR := httptest.NewRecorder()
	srv.Router().ServeHTTP(replayRR, replayReq)

	if replayRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from POST /api/v1/decisions/{id}/replay, got %d: %s", replayRR.Code, replayRR.Body.String())
	}

	var replayDTO api.ReplayResponseDTO
	if err := json.NewDecoder(replayRR.Body).Decode(&replayDTO); err != nil {
		t.Fatalf("failed decoding replay response: %v", err)
	}
	if !replayDTO.IsBitExact {
		t.Fatalf("expected bit-exact replay verification, got drifts: %v", replayDTO.DriftDetails)
	}
	if !replayDTO.InitialStateHashMatch || !replayDTO.ActionHashMatch {
		t.Errorf("expected state and action hash matches in replay")
	}

	// 6. Test GET /api/v1/runs/{id}/integrity
	integrityReq, _ := http.NewRequest(http.MethodGet, "/api/v1/runs/RUN-CFA/integrity", nil)
	integrityRR := httptest.NewRecorder()
	srv.Router().ServeHTTP(integrityRR, integrityReq)

	if integrityRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /api/v1/runs/{id}/integrity, got %d", integrityRR.Code)
	}

	var integrityDTO api.ChainIntegrityResponseDTO
	if err := json.NewDecoder(integrityRR.Body).Decode(&integrityDTO); err != nil {
		t.Fatalf("failed decoding integrity response: %v", err)
	}
	if !integrityDTO.IsValid {
		t.Errorf("expected valid cryptographic chain for RUN-CFA, got invalid status: %s", integrityDTO.Status)
	}

	// 7. Test GET /api/v1/decisions/NON_EXISTENT/explain (404 NOT FOUND)
	notFoundReq, _ := http.NewRequest(http.MethodGet, "/api/v1/decisions/NON_EXISTENT_ID/explain", nil)
	notFoundRR := httptest.NewRecorder()
	srv.Router().ServeHTTP(notFoundRR, notFoundReq)

	if notFoundRR.Code != http.StatusNotFound {
		t.Errorf("expected 404 NOT FOUND, got %d", notFoundRR.Code)
	}
}
