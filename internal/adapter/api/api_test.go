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
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// 1. Test /healthz
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /healthz, got %d", resp.StatusCode)
	}

	var health api.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("failed decoding health response: %v", err)
	}
	if health.Status != "OK" || health.Version != "1.0.0" {
		t.Errorf("unexpected health body: %+v", health)
	}

	// 2. Test /metrics
	mResp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	defer mResp.Body.Close()

	if mResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /metrics, got %d", mResp.StatusCode)
	}

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(mResp.Body)
	mText := buf.String()

	if !strings.Contains(mText, "project-mittens") {
		t.Errorf("expected project-mittens service_name in Prometheus metrics, got: %s", mText)
	}
}

func TestAPI_OptimizeEndpoint(t *testing.T) {
	srv := api.NewServer(api.DefaultServerConfig())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

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
	resp, err := http.Post(ts.URL+"/api/v1/optimize", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("POST /api/v1/optimize failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/optimize, got %d", resp.StatusCode)
	}

	var optResp api.OptimizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&optResp); err != nil {
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
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// 1. Empty fleet error
	emptyReq := api.OptimizeRequest{Epoch: 1000, Drivers: []api.DriverDTO{}}
	b1, _ := json.Marshal(emptyReq)
	r1, err := http.Post(ts.URL+"/api/v1/optimize", "application/json", bytes.NewReader(b1))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer r1.Body.Close()

	if r1.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on empty drivers, got %d", r1.StatusCode)
	}

	// 2. Malformed JSON
	r2, err := http.Post(ts.URL+"/api/v1/optimize", "application/json", bytes.NewReader([]byte("{invalid-json")))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer r2.Body.Close()

	if r2.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on malformed JSON, got %d", r2.StatusCode)
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
	r3, err := http.Post(ts.URL+"/api/v1/optimize", "application/json", bytes.NewReader(b3))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer r3.Body.Close()

	if r3.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on unknown policy, got %d", r3.StatusCode)
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
	r4, err := http.Post(ts.URL+"/api/v1/optimize", "application/json", bytes.NewReader(b4))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer r4.Body.Close()

	if r4.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on competitor_scale > 0, got %d", r4.StatusCode)
	}
}

func TestAPI_SimulateEndpoint(t *testing.T) {
	srv := api.NewServer(api.DefaultServerConfig())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

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
	resp, err := http.Post(ts.URL+"/api/v1/simulate", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("POST /api/v1/simulate failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/simulate, got %d", resp.StatusCode)
	}

	var simResp api.SimulateResponse
	if err := json.NewDecoder(resp.Body).Decode(&simResp); err != nil {
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

func TestAPI_SemanticJournalRecording(t *testing.T) {
	srv := api.NewServer(api.DefaultServerConfig())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

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
				PickupLatestEpoch:     1700003600,
				DeliveryEarliestEpoch: 1700010000,
				DeliveryLatestEpoch:   1700080000,
				Revenue:               2500.0,
			},
		},
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		t.Fatalf("failed marshaling request: %v", err)
	}

	resp, err := http.Post(ts.URL+"/api/v1/optimize", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/v1/optimize failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
}
