package grpc_test

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	mittensgrpc "github.com/optimaldynamics/project-mittens/internal/adapter/grpc"
	"github.com/optimaldynamics/project-mittens/internal/service"
	pkgjournal "github.com/optimaldynamics/project-mittens/pkg/journal"
	mittensv1 "github.com/optimaldynamics/project-mittens/proto/mittens/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

func setupTestGRPCServer(t *testing.T) (mittensv1.OptimizerServiceClient, *pkgjournal.MemoryStore, service.Journal, func()) {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer()

	cryptoStore := pkgjournal.NewMemoryStore()
	memJournal := service.NewMemoryJournal()

	adapter := mittensgrpc.NewServer(mittensgrpc.DefaultServerConfig(), mittensgrpc.Dependencies{
		Journal:     memJournal,
		CryptoStore: cryptoStore,
	})
	adapter.Register(grpcServer)

	go func() {
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Logf("test grpc server stopped: %v", err)
		}
	}()

	bufDialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		"passthrough://bufnet",
		grpc.WithContextDialer(bufDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed dialing bufnet: %v", err)
	}

	client := mittensv1.NewOptimizerServiceClient(conn)

	cleanup := func() {
		_ = conn.Close()
		grpcServer.GracefulStop()
		_ = lis.Close()
	}

	return client, cryptoStore, memJournal, cleanup
}

func TestGRPC_Optimize_MonopolisticCFA(t *testing.T) {
	client, _, _, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()
	req := &mittensv1.OptimizeRequest{
		PolicyClass:     "CFA",
		CompetitorScale: "N0",
		Epoch:           now,
		Drivers: []*mittensv1.Driver{
			{
				Id: "D-01",
				CurrentLocation: &mittensv1.Location{
					NodeId: "CHI",
					Lat:    41.8781,
					Lon:    -87.6298,
				},
				HomeLocation: &mittensv1.Location{
					NodeId: "CHI",
					Lat:    41.8781,
					Lon:    -87.6298,
				},
				AvailableEpoch:      now,
				RemainingDriveHours: 11.0,
				RemainingDutyHours:  14.0,
				Equipment: &mittensv1.Equipment{
					Type: "DRY_VAN",
				},
			},
		},
		Loads: []*mittensv1.Load{
			{
				Id: "LOAD_CHI_IND",
				Origin: &mittensv1.Location{
					NodeId: "CHI",
					Lat:    41.8781,
					Lon:    -87.6298,
				},
				Destination: &mittensv1.Location{
					NodeId: "IND",
					Lat:    39.7684,
					Lon:    -86.1581,
				},
				Revenue:               1800.0,
				PickupEarliestEpoch:   now,
				PickupLatestEpoch:     now + 36000,
				DeliveryEarliestEpoch: now + 18000,
				DeliveryLatestEpoch:   now + 120000,
				RequiredEquipment:     "DRY_VAN",
			},
		},
		CostConfig: &mittensv1.CostConfig{
			LoadedCostPerMile: 2.00,
			EmptyCostPerMile:  1.50,
		},
		FeasibilityConfig: &mittensv1.FeasibilityConfig{
			MaxEmptyMiles: 300.0,
		},
	}

	resp, err := client.Optimize(ctx, req)
	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}

	if resp.MatchCount != 1 {
		t.Fatalf("expected 1 match, got %d", resp.MatchCount)
	}

	if len(resp.Matches) != 1 {
		t.Fatalf("expected 1 match item, got %d", len(resp.Matches))
	}

	match := resp.Matches[0]
	if match.DriverId != "D-01" || match.LoadId != "LOAD_CHI_IND" {
		t.Errorf("unexpected match: %s -> %s", match.DriverId, match.LoadId)
	}

	if match.EstimatedContribution <= 0 {
		t.Errorf("expected positive contribution, got $%.2f", match.EstimatedContribution)
	}

	if resp.Provenance == nil {
		t.Fatalf("expected non-nil decision provenance")
	}

	if resp.Provenance.MatchedCount != 1 {
		t.Errorf("expected provenance matched count 1, got %d", resp.Provenance.MatchedCount)
	}

	if len(resp.Provenance.EvaluatedArcs) > 0 && !resp.Provenance.EvaluatedArcs[0].Feasible {
		t.Errorf("expected evaluated arc in provenance to be feasible")
	}
}

func TestGRPC_Optimize_CompetitivePOMDP(t *testing.T) {
	client, _, _, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()
	req := &mittensv1.OptimizeRequest{
		PolicyClass:     "CFA",
		CompetitorScale: "N1",
		Epoch:           now,
		Drivers: []*mittensv1.Driver{
			{
				Id: "D-01",
				CurrentLocation: &mittensv1.Location{
					NodeId: "CHI",
					Lat:    41.8781,
					Lon:    -87.6298,
				},
				HomeLocation: &mittensv1.Location{
					NodeId: "CHI",
					Lat:    41.8781,
					Lon:    -87.6298,
				},
				AvailableEpoch:      now,
				RemainingDriveHours: 11.0,
				RemainingDutyHours:  14.0,
				Equipment: &mittensv1.Equipment{
					Type: "DRY_VAN",
				},
			},
		},
		Loads: []*mittensv1.Load{
			{
				Id: "LOAD_CHI_IND",
				Origin: &mittensv1.Location{
					NodeId: "CHI",
					Lat:    41.8781,
					Lon:    -87.6298,
				},
				Destination: &mittensv1.Location{
					NodeId: "IND",
					Lat:    39.7684,
					Lon:    -86.1581,
				},
				Revenue:               2500.0,
				PickupEarliestEpoch:   now,
				PickupLatestEpoch:     now + 36000,
				DeliveryEarliestEpoch: now + 18000,
				DeliveryLatestEpoch:   now + 120000,
				RequiredEquipment:     "DRY_VAN",
			},
		},
	}

	resp, err := client.Optimize(ctx, req)
	if err != nil {
		t.Fatalf("Competitive Optimize failed: %v", err)
	}

	if resp.MatchCount != 1 {
		t.Fatalf("expected 1 match, got %d", resp.MatchCount)
	}

	if resp.TotalNetContribution <= 0 {
		t.Errorf("expected positive contribution, got $%.2f", resp.TotalNetContribution)
	}
}

func TestGRPC_Optimize_Policies_VFA_and_DLA(t *testing.T) {
	client, _, _, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	for _, polClass := range []string{"VFA", "DLA"} {
		req := &mittensv1.OptimizeRequest{
			PolicyClass:     polClass,
			CompetitorScale: "N0",
			Epoch:           now,
			Drivers: []*mittensv1.Driver{
				{
					Id: "D-01",
					CurrentLocation: &mittensv1.Location{
						NodeId: "CHI",
						Lat:    41.8781,
						Lon:    -87.6298,
					},
					AvailableEpoch:      now,
					RemainingDriveHours: 11.0,
					RemainingDutyHours:  14.0,
				},
			},
			Loads: []*mittensv1.Load{
				{
					Id: "LOAD-01",
					Origin: &mittensv1.Location{
						NodeId: "CHI",
						Lat:    41.8781,
						Lon:    -87.6298,
					},
					Destination: &mittensv1.Location{
						NodeId: "IND",
						Lat:    39.7684,
						Lon:    -86.1581,
					},
					Revenue:               2200.0,
					PickupEarliestEpoch:   now,
					PickupLatestEpoch:     now + 36000,
					DeliveryEarliestEpoch: now + 18000,
					DeliveryLatestEpoch:   now + 120000,
				},
			},
		}

		resp, err := client.Optimize(ctx, req)
		if err != nil {
			t.Fatalf("Optimize with %s failed: %v", polClass, err)
		}
		if resp.MatchCount != 1 {
			t.Errorf("expected 1 match for %s, got %d", polClass, resp.MatchCount)
		}
	}
}

func TestGRPC_Optimize_HazmatEndorsementFiltering(t *testing.T) {
	client, _, _, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	// 1. Non-certified driver cannot take Hazmat load
	nonCertifiedReq := &mittensv1.OptimizeRequest{
		PolicyClass: "CFA",
		Epoch:       now,
		Drivers: []*mittensv1.Driver{
			{
				Id:                  "D-CLEAN",
				CurrentLocation:     &mittensv1.Location{NodeId: "CHI", Lat: 41.8781, Lon: -87.6298},
				AvailableEpoch:      now,
				RemainingDriveHours: 11.0,
				RemainingDutyHours:  14.0,
				HazmatCertified:     false,
			},
		},
		Loads: []*mittensv1.Load{
			{
				Id:                    "L-HAZMAT",
				Origin:                &mittensv1.Location{NodeId: "CHI", Lat: 41.8781, Lon: -87.6298},
				Destination:           &mittensv1.Location{NodeId: "IND", Lat: 39.7684, Lon: -86.1581},
				Revenue:               3000.0,
				PickupEarliestEpoch:   now,
				PickupLatestEpoch:     now + 36000,
				DeliveryEarliestEpoch: now + 18000,
				DeliveryLatestEpoch:   now + 120000,
				Hazmat:                true,
			},
		},
	}

	resp, err := client.Optimize(ctx, nonCertifiedReq)
	if err != nil {
		t.Fatalf("Optimize non-certified failed: %v", err)
	}
	if resp.MatchCount != 0 {
		t.Errorf("expected 0 matches for non-hazmat driver on hazmat load, got %d", resp.MatchCount)
	}

	// 2. Hazmat certified driver successfully matches
	certifiedReq := &mittensv1.OptimizeRequest{
		PolicyClass: "CFA",
		Epoch:       now,
		Drivers: []*mittensv1.Driver{
			{
				Id:                  "D-CERTIFIED",
				CurrentLocation:     &mittensv1.Location{NodeId: "CHI", Lat: 41.8781, Lon: -87.6298},
				AvailableEpoch:      now,
				RemainingDriveHours: 11.0,
				RemainingDutyHours:  14.0,
				HazmatCertified:     true,
			},
		},
		Loads: []*mittensv1.Load{
			{
				Id:                    "L-HAZMAT",
				Origin:                &mittensv1.Location{NodeId: "CHI", Lat: 41.8781, Lon: -87.6298},
				Destination:           &mittensv1.Location{NodeId: "IND", Lat: 39.7684, Lon: -86.1581},
				Revenue:               3000.0,
				PickupEarliestEpoch:   now,
				PickupLatestEpoch:     now + 36000,
				DeliveryEarliestEpoch: now + 18000,
				DeliveryLatestEpoch:   now + 120000,
				Hazmat:                true,
			},
		},
	}

	respCert, errCert := client.Optimize(ctx, certifiedReq)
	if errCert != nil {
		t.Fatalf("Optimize certified failed: %v", errCert)
	}
	if respCert.MatchCount != 1 {
		t.Errorf("expected 1 match for hazmat certified driver, got %d", respCert.MatchCount)
	}
}

func TestGRPC_Optimize_FailClosedInvalidRequests(t *testing.T) {
	client, _, _, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Empty request
	_, err := client.Optimize(ctx, &mittensv1.OptimizeRequest{})
	if err == nil {
		t.Fatalf("expected error on empty optimize request, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Errorf("expected codes.InvalidArgument, got %v", st.Code())
	}
}

func TestGRPC_StreamTelemetryAndTenders(t *testing.T) {
	client, _, _, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().Unix()

	// 1. Ingest Telemetry
	telResp, err := client.StreamTelemetry(ctx, &mittensv1.StreamTelemetryRequest{
		DriverId: "D-STREAM-1",
		Epoch:    now,
		CurrentLocation: &mittensv1.Location{
			NodeId: "CHI",
			Lat:    41.8781,
			Lon:    -87.6298,
		},
		RemainingDriveHours: 9.5,
		RemainingDutyHours:  12.0,
	})
	if err != nil {
		t.Fatalf("StreamTelemetry failed: %v", err)
	}
	if !telResp.Accepted {
		t.Errorf("expected telemetry accepted, got false")
	}

	// 2. Ingest Tenders
	tenResp, err := client.StreamTenders(ctx, &mittensv1.StreamTendersRequest{
		Loads: []*mittensv1.Load{
			{
				Id: "L-STREAM-1",
				Origin: &mittensv1.Location{
					NodeId: "CHI",
					Lat:    41.8781,
					Lon:    -87.6298,
				},
				Destination: &mittensv1.Location{
					NodeId: "DAL",
					Lat:    32.7767,
					Lon:    -96.7970,
				},
				Revenue:             2800.0,
				PickupEarliestEpoch: now,
			},
		},
	})
	if err != nil {
		t.Fatalf("StreamTenders failed: %v", err)
	}
	if tenResp.AcceptedCount != 1 {
		t.Errorf("expected 1 tender accepted, got %d", tenResp.AcceptedCount)
	}
}

func TestGRPC_PlanRepositioning(t *testing.T) {
	client, _, _, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().Unix()
	resp, err := client.PlanRepositioning(ctx, &mittensv1.RepositionPlanRequest{
		Epoch: now,
		IdleDrivers: []*mittensv1.Driver{
			{
				Id:                  "D-IDLE-1",
				CurrentLocation:     &mittensv1.Location{NodeId: "CHI", Lat: 41.8781, Lon: -87.6298},
				AvailableEpoch:      now,
				RemainingDriveHours: 11.0,
				RemainingDutyHours:  14.0,
			},
		},
		MaxRepositionDistanceMiles: 400.0,
	})
	if err != nil {
		t.Fatalf("PlanRepositioning failed: %v", err)
	}

	if resp == nil {
		t.Fatalf("expected non-nil reposition response")
	}
}

func TestGRPC_ExplainAndReplayDecision(t *testing.T) {
	client, _, _, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	// 1. Run optimization to create a journaled decision
	optResp, err := client.Optimize(ctx, &mittensv1.OptimizeRequest{
		PolicyClass: "CFA",
		Epoch:       now,
		Drivers: []*mittensv1.Driver{
			{
				Id:                  "D-01",
				CurrentLocation:     &mittensv1.Location{NodeId: "CHI", Lat: 41.8781, Lon: -87.6298},
				AvailableEpoch:      now,
				RemainingDriveHours: 11.0,
				RemainingDutyHours:  14.0,
			},
		},
		Loads: []*mittensv1.Load{
			{
				Id:                    "LOAD_CHI_IND",
				Origin:                &mittensv1.Location{NodeId: "CHI", Lat: 41.8781, Lon: -87.6298},
				Destination:           &mittensv1.Location{NodeId: "IND", Lat: 39.7684, Lon: -86.1581},
				Revenue:               1800.0,
				PickupEarliestEpoch:   now,
				PickupLatestEpoch:     now + 36000,
				DeliveryEarliestEpoch: now + 18000,
				DeliveryLatestEpoch:   now + 120000,
			},
		},
	})
	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}

	decisionID := optResp.DecisionId

	// 2. Test ExplainDecision
	explainResp, err := client.ExplainDecision(ctx, &mittensv1.ExplainDecisionRequest{
		DecisionId: decisionID,
	})
	if err != nil {
		t.Fatalf("ExplainDecision failed: %v", err)
	}
	if explainResp.DecisionId != decisionID {
		t.Errorf("expected explain decision ID %s, got %s", decisionID, explainResp.DecisionId)
	}
	if !strings.Contains(explainResp.MarkdownSummary, "Optimization Decision Explanation") {
		t.Errorf("expected markdown report header, got: %s", explainResp.MarkdownSummary)
	}

	// 3. Test ReplayDecision
	replayResp, err := client.ReplayDecision(ctx, &mittensv1.ReplayDecisionRequest{
		DecisionId: decisionID,
	})
	if err != nil {
		t.Fatalf("ReplayDecision failed: %v", err)
	}
	if replayResp.Status != "BIT_EXACT_MATCH" {
		t.Errorf("expected BIT_EXACT_MATCH, got %s", replayResp.Status)
	}
	if !replayResp.InitialStateHashMatch || !replayResp.ActionHashMatch {
		t.Errorf("expected state and action hash matches to be true, got initial=%v action=%v",
			replayResp.InitialStateHashMatch, replayResp.ActionHashMatch)
	}
}

func TestGRPC_VerifyMerkleChain(t *testing.T) {
	client, _, _, cleanup := setupTestGRPCServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()

	// 1. Run optimization to generate sealed Merkle records
	optResp, err := client.Optimize(ctx, &mittensv1.OptimizeRequest{
		PolicyClass: "CFA",
		Epoch:       now,
		Drivers: []*mittensv1.Driver{
			{
				Id:                  "D-01",
				CurrentLocation:     &mittensv1.Location{NodeId: "CHI", Lat: 41.8781, Lon: -87.6298},
				AvailableEpoch:      now,
				RemainingDriveHours: 11.0,
				RemainingDutyHours:  14.0,
			},
		},
		Loads: []*mittensv1.Load{
			{
				Id:                    "LOAD_CHI_IND",
				Origin:                &mittensv1.Location{NodeId: "CHI", Lat: 41.8781, Lon: -87.6298},
				Destination:           &mittensv1.Location{NodeId: "IND", Lat: 39.7684, Lon: -86.1581},
				Revenue:               1800.0,
				PickupEarliestEpoch:   now,
				PickupLatestEpoch:     now + 36000,
				DeliveryEarliestEpoch: now + 18000,
				DeliveryLatestEpoch:   now + 120000,
			},
		},
	})
	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}

	runID := optResp.RunId

	// 2. Verify Merkle Chain
	merkleResp, err := client.VerifyMerkleChain(ctx, &mittensv1.VerifyMerkleChainRequest{
		RunId: runID,
	})
	if err != nil {
		t.Fatalf("VerifyMerkleChain failed: %v", err)
	}
	if !merkleResp.IsValid {
		t.Errorf("expected Merkle chain to be valid, got invalid: %s", merkleResp.VerificationMessage)
	}
	if merkleResp.TotalRecordsVerified < 1 {
		t.Errorf("expected at least 1 verified record, got %d", merkleResp.TotalRecordsVerified)
	}
}
