package grpc_test

import (
	"context"
	"net"
	"testing"
	"time"

	mittensgrpc "github.com/optimaldynamics/project-mittens/internal/adapter/grpc"
	mittensv1 "github.com/optimaldynamics/project-mittens/proto/mittens/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

func setupTestGRPCServer(t *testing.T) (mittensv1.OptimizerServiceClient, func()) {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer()

	adapter := mittensgrpc.NewServer(mittensgrpc.DefaultServerConfig(), mittensgrpc.Dependencies{})
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

	return client, cleanup
}

func TestGRPC_Optimize_MonopolisticCFA(t *testing.T) {
	client, cleanup := setupTestGRPCServer(t)
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
}

func TestGRPC_Optimize_CompetitivePOMDP(t *testing.T) {
	client, cleanup := setupTestGRPCServer(t)
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

func TestGRPC_StreamTelemetryAndTenders(t *testing.T) {
	client, cleanup := setupTestGRPCServer(t)
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
