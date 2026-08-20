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
	"github.com/optimaldynamics/project-mittens/internal/adapter/stream"
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
	if health.Database != "in-memory" {
		t.Errorf("expected default database status 'in-memory', got %s", health.Database)
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

	// 4. Invalid competitor scale (<0)
	compReq := api.OptimizeRequest{
		Epoch:           1000,
		CompetitorScale: -1,
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
		t.Errorf("expected 400 Bad Request on competitor_scale < 0, got %d", rr4.Code)
	}
}

func TestAPI_OptimizeEndpoint_CompetitorScaleAndPolicies(t *testing.T) {
	srv := api.NewServer(api.DefaultServerConfig())
	now := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	policies := []string{"CFA", "PiecewiseVFA", "DLA"}
	for _, pol := range policies {
		t.Run(pol+"_N1", func(t *testing.T) {
			reqPayload := api.OptimizeRequest{
				Epoch:           now,
				PolicyClass:     pol,
				CompetitorScale: 1,
				Drivers: []api.DriverDTO{
					{
						ID:                  "DRV_01",
						CurrentLocation:     api.LocationDTO{NodeID: "A", Lat: 40.0, Lon: -75.0},
						HomeLocation:        api.LocationDTO{NodeID: "A", Lat: 40.0, Lon: -75.0},
						AvailableEpoch:      now,
						DriveHoursRemaining: 11.0,
						DutyHoursRemaining:  14.0,
						Equipment:           api.EquipmentDTO{Type: "DRY_VAN"},
					},
				},
				Loads: []api.LoadDTO{
					{
						ID:                    "LOAD_01",
						Origin:                api.LocationDTO{NodeID: "A", Lat: 40.0, Lon: -75.0},
						Destination:           api.LocationDTO{NodeID: "B", Lat: 41.0, Lon: -74.0},
						PickupEarliestEpoch:   now,
						PickupLatestEpoch:     now + 3600,
						DeliveryEarliestEpoch: now + 7200,
						DeliveryLatestEpoch:   now + 14400,
						Revenue:               1500.0,
						RequiredEquipment:     "DRY_VAN",
					},
				},
			}

			body, _ := json.Marshal(reqPayload)
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/optimize", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			srv.Router().ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected 200 OK for policy %s N=1, got %d: %s", pol, rr.Code, rr.Body.String())
			}
		})
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

func TestAPI_StreamingIngestionEndpoints(t *testing.T) {
	srv := api.NewServer(api.DefaultServerConfig())
	nowEpoch := time.Now().UTC().Unix()

	// 1. Ingest Telemetry Pings
	telemetryReq := api.StreamTelemetryRequestDTO{
		Pings: []stream.ELDDriverPingDTO{
			{
				DriverID:            "STREAM-DRV-01",
				Timestamp:           nowEpoch,
				Lat:                 41.8781,
				Lon:                 -87.6298,
				DriveHoursRemaining: 9.5,
				DutyHoursRemaining:  12.0,
			},
		},
	}
	bodyTele, _ := json.Marshal(telemetryReq)
	reqTele, _ := http.NewRequest(http.MethodPost, "/api/v1/stream/telemetry", bytes.NewReader(bodyTele))
	rrTele := httptest.NewRecorder()
	srv.Router().ServeHTTP(rrTele, reqTele)

	if rrTele.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/stream/telemetry, got %d: %s", rrTele.Code, rrTele.Body.String())
	}

	// 2. Ingest Load Tenders
	tendersReq := api.StreamTendersRequestDTO{
		Tenders: []stream.TMSLoadTenderDTO{
			{
				LoadID:                "STREAM-TENDER-01",
				Timestamp:             nowEpoch,
				OriginNodeID:          "CHI",
				OriginLat:             41.8781,
				OriginLon:             -87.6298,
				DestinationNodeID:     "ATL",
				DestLat:               33.7490,
				DestLon:               -84.3880,
				PickupEarliestEpoch:   nowEpoch + 3600,
				PickupLatestEpoch:     nowEpoch + 7200,
				DeliveryEarliestEpoch: nowEpoch + 14400,
				DeliveryLatestEpoch:   nowEpoch + 86400,
				Revenue:               2100.0,
				RequiredEquipment:     "DRY_VAN",
			},
		},
	}
	bodyTenders, _ := json.Marshal(tendersReq)
	reqTenders, _ := http.NewRequest(http.MethodPost, "/api/v1/stream/tenders", bytes.NewReader(bodyTenders))
	rrTenders := httptest.NewRecorder()
	srv.Router().ServeHTTP(rrTenders, reqTenders)

	if rrTenders.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/stream/tenders, got %d: %s", rrTenders.Code, rrTenders.Body.String())
	}

	// 3. Check Stream Status
	reqStatus, _ := http.NewRequest(http.MethodGet, "/api/v1/stream/status", nil)
	rrStatus := httptest.NewRecorder()
	srv.Router().ServeHTTP(rrStatus, reqStatus)

	if rrStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/stream/status, got %d: %s", rrStatus.Code, rrStatus.Body.String())
	}

	var statusDTO api.StreamStatusResponseDTO
	if err := json.NewDecoder(rrStatus.Body).Decode(&statusDTO); err != nil {
		t.Fatalf("failed decoding stream status: %v", err)
	}
	if statusDTO.Status.BufferedDriverPings < 1 || statusDTO.Status.BufferedTendersCount < 1 {
		t.Errorf("expected buffered pings and tenders, got pings=%d tenders=%d",
			statusDTO.Status.BufferedDriverPings, statusDTO.Status.BufferedTendersCount)
	}

	// 4. Ingest Cancel
	cancelsReq := api.StreamCancelsRequestDTO{
		Cancellations: []stream.TenderCancelDTO{
			{
				LoadID:    "STREAM-TENDER-01",
				Timestamp: nowEpoch + 10,
				Reason:    "Shipper canceled order",
			},
		},
	}
	bodyCancels, _ := json.Marshal(cancelsReq)
	reqCancels, _ := http.NewRequest(http.MethodPost, "/api/v1/stream/cancels", bytes.NewReader(bodyCancels))
	rrCancels := httptest.NewRecorder()
	srv.Router().ServeHTTP(rrCancels, reqCancels)

	if rrCancels.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/stream/cancels, got %d: %s", rrCancels.Code, rrCancels.Body.String())
	}
}

func TestAPI_RepositionPlanEndpoint(t *testing.T) {
	srv := api.NewServer(api.DefaultServerConfig())
	nowEpoch := time.Now().UTC().Unix()

	repoReq := api.RepositionPlanRequestDTO{
		Drivers: []api.DriverDTO{
			{
				ID: "DRV-01",
				CurrentLocation: api.LocationDTO{
					NodeID: "CLT",
					Lat:    35.2271,
					Lon:    -80.8431,
				},
				AvailableEpoch:      nowEpoch,
				DriveHoursRemaining: 11.0,
				DutyHoursRemaining:  14.0,
				Equipment:           api.EquipmentDTO{Type: "DRY_VAN"},
			},
		},
		Loads: []api.LoadDTO{
			{
				ID: "LOAD-ATL-CHI",
				Origin: api.LocationDTO{
					NodeID: "ATL",
					Lat:    33.7490,
					Lon:    -84.3880,
				},
				Destination: api.LocationDTO{
					NodeID: "CHI",
					Lat:    41.8781,
					Lon:    -87.6298,
				},
				PickupEarliestEpoch:   nowEpoch + 36000,
				PickupLatestEpoch:     nowEpoch + 72000,
				DeliveryEarliestEpoch: nowEpoch + 72000,
				DeliveryLatestEpoch:   nowEpoch + 108000,
				Revenue:               2800.0,
				RequiredEquipment:     "DRY_VAN",
			},
		},
	}

	body, _ := json.Marshal(repoReq)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/reposition/plan", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/reposition/plan, got %d: %s", rr.Code, rr.Body.String())
	}

	var planDTO api.RepositionPlanResponseDTO
	if err := json.NewDecoder(rr.Body).Decode(&planDTO); err != nil {
		t.Fatalf("failed decoding reposition plan response: %v", err)
	}
}

func TestAPI_ScenarioCatalogEndpoints(t *testing.T) {
	srv := api.NewServer(api.DefaultServerConfig())

	// 1. Test GET /api/v1/scenarios
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/scenarios", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/scenarios, got %d: %s", rr.Code, rr.Body.String())
	}

	var listResp struct {
		Scenarios []api.ScenarioSummaryDTO `json:"scenarios"`
		Count     int                      `json:"count"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed decoding scenarios response: %v", err)
	}
	if listResp.Count < 5 || len(listResp.Scenarios) < 5 {
		t.Fatalf("expected at least 5 scenarios, got %d", listResp.Count)
	}

	// 2. Test GET /api/v1/scenarios/07_test_dispatch
	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/scenarios/07_test_dispatch", nil)
	rr2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/scenarios/07_test_dispatch, got %d: %s", rr2.Code, rr2.Body.String())
	}

	var detail api.ScenarioDetailDTO
	if err := json.NewDecoder(rr2.Body).Decode(&detail); err != nil {
		t.Fatalf("failed decoding scenario detail response: %v", err)
	}
	if detail.Summary.ID != "07_test_dispatch" {
		t.Errorf("expected scenario ID 07_test_dispatch, got %s", detail.Summary.ID)
	}
	if len(detail.Drivers) != 3 || len(detail.Loads) != 2 {
		t.Errorf("expected 3 drivers and 2 loads, got %d drivers and %d loads", len(detail.Drivers), len(detail.Loads))
	}

	// 3. Test GET non-existent scenario -> 404
	req404, _ := http.NewRequest(http.MethodGet, "/api/v1/scenarios/unknown_scenario_xyz", nil)
	rr404 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr404, req404)

	if rr404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown scenario, got %d", rr404.Code)
	}
}

func TestAPI_StaticWebServing(t *testing.T) {
	srv := api.NewServer(api.DefaultServerConfig())

	// 1. Test GET / (serves index.html from embedded web.Assets)
	reqRoot, _ := http.NewRequest(http.MethodGet, "/", nil)
	rrRoot := httptest.NewRecorder()
	srv.Router().ServeHTTP(rrRoot, reqRoot)

	if rrRoot.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /, got %d (body: %s)", rrRoot.Code, rrRoot.Body.String())
	}
	body := rrRoot.Body.String()
	if !strings.Contains(body, "html") {
		t.Errorf("expected HTML response, got: %s", body)
	}

	// 2. Test GET /simulation/route (SPA Client-side Route Fallback -> index.html)
	reqSPA, _ := http.NewRequest(http.MethodGet, "/simulation/route", nil)
	rrSPA := httptest.NewRecorder()
	srv.Router().ServeHTTP(rrSPA, reqSPA)

	if rrSPA.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from SPA route, got %d", rrSPA.Code)
	}
	if !strings.Contains(rrSPA.Body.String(), "html") {
		t.Errorf("expected HTML response for SPA route, got: %s", rrSPA.Body.String())
	}

	// 3. Test GET /api/v1/unknown (API route -> 404)
	reqAPI, _ := http.NewRequest(http.MethodGet, "/api/v1/nonexistent", nil)
	rrAPI := httptest.NewRecorder()
	srv.Router().ServeHTTP(rrAPI, reqAPI)

	if rrAPI.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for missing API route, got %d", rrAPI.Code)
	}
}
