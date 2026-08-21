// Package grpc implements the high-performance binary gRPC adapter for Project Mittens.
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /internal/domain/policy, /internal/service, /pkg/journal, /pkg/replay, /proto/mittens/v1
//   - Imported by: /cmd/server
//   - Strict Rule: Implements OptimizerServiceServer with fail-closed error handling and zero global mutable state.
package grpc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/adapter/db"
	"github.com/optimaldynamics/project-mittens/internal/adapter/stream"
	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy/reposition"
	"github.com/optimaldynamics/project-mittens/internal/service"
	"github.com/optimaldynamics/project-mittens/pkg/explain"
	pkgjournal "github.com/optimaldynamics/project-mittens/pkg/journal"
	"github.com/optimaldynamics/project-mittens/pkg/replay"
	"github.com/optimaldynamics/project-mittens/pkg/telemetry"
	mittensv1 "github.com/optimaldynamics/project-mittens/proto/mittens/v1"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// ServerConfig configures the gRPC server.
type ServerConfig struct {
	Host string
	Port int
}

// DefaultServerConfig returns production defaults for the gRPC server.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host: "0.0.0.0",
		Port: 9090,
	}
}

// Dependencies contains the injected persistence, synchronizer, and engine services.
type Dependencies struct {
	Journal         service.Journal
	CryptoStore     pkgjournal.JournalStore
	DBPool          *db.Pool
	RunRepository   *db.PostgresRunRepository
	StreamBuffer    *stream.StreamBuffer
	StreamSync      *stream.StateSynchronizer
	RepositionSynth *reposition.RepositioningSynthesizer
}

// Server implements mittensv1.OptimizerServiceServer.
type Server struct {
	mittensv1.UnimplementedOptimizerServiceServer
	cfg             ServerConfig
	deps            Dependencies
	grpcSrv         *grpc.Server
	journal         service.Journal
	cryptoStore     pkgjournal.JournalStore
	streamBuffer    *stream.StreamBuffer
	streamSync      *stream.StateSynchronizer
	repositionSynth *reposition.RepositioningSynthesizer
}

// NewServer initializes the gRPC optimizer server with clean service dependencies.
func NewServer(cfg ServerConfig, deps Dependencies) *Server {
	jStore := deps.Journal
	if jStore == nil {
		jStore = service.NewMemoryJournal()
	}

	cStore := deps.CryptoStore
	if cStore == nil {
		cStore = pkgjournal.NewMemoryStore()
	}

	sBuffer := deps.StreamBuffer
	if sBuffer == nil {
		sBuffer = stream.NewStreamBuffer()
	}

	sSync := deps.StreamSync
	if sSync == nil {
		sSync = stream.NewStateSynchronizer(sBuffer)
	}

	repoSynth := deps.RepositionSynth
	if repoSynth == nil {
		repoSynth = reposition.NewRepositioningSynthesizer()
	}

	return &Server{
		cfg:             cfg,
		deps:            deps,
		journal:         jStore,
		cryptoStore:     cStore,
		streamBuffer:    sBuffer,
		streamSync:      sSync,
		repositionSynth: repoSynth,
	}
}

// Register registers this service, health checks, and reflection with the given gRPC server instance.
func (s *Server) Register(grpcSrv *grpc.Server) {
	s.grpcSrv = grpcSrv
	mittensv1.RegisterOptimizerServiceServer(grpcSrv, s)

	// Register gRPC Health Service
	healthServer := health.NewServer()
	healthServer.SetServingStatus("mittens.v1.OptimizerService", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcSrv, healthServer)

	// Register gRPC Server Reflection for grpcurl / Postman inspection
	reflection.Register(grpcSrv)
}

// Start binds the TCP listener and serves requests.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("grpc server: failed to listen on %s: %w", addr, err)
	}

	if s.grpcSrv == nil {
		s.grpcSrv = grpc.NewServer()
		s.Register(s.grpcSrv)
	}

	return s.grpcSrv.Serve(lis)
}

// Stop gracefully terminates the gRPC server.
func (s *Server) Stop() {
	if s.grpcSrv != nil {
		s.grpcSrv.GracefulStop()
	}
}

// ----------------------------------------------------------------------------
// RPC Implementations
// ----------------------------------------------------------------------------

// Optimize executes single-epoch optimal fleet matching via exact LAPJV and Powell policy approximations.
func (s *Server) Optimize(ctx context.Context, req *mittensv1.OptimizeRequest) (*mittensv1.OptimizeResponse, error) {
	ctx, span := telemetry.GlobalProvider().Tracer("mittens.grpc").Start(ctx, "OptimizerService.Optimize")
	defer span.End()

	t0 := time.Now()

	if len(req.Drivers) == 0 && len(req.Loads) == 0 {
		return nil, status.Error(codes.InvalidArgument, "request must contain at least one driver or load")
	}

	epoch := req.Epoch
	if epoch <= 0 {
		epoch = time.Now().Unix()
	}

	// 1. Convert Driver and Load DTOs
	drivers := make([]model.Driver, len(req.Drivers))
	for i, d := range req.Drivers {
		drivers[i] = protoToDriver(d, epoch)
	}

	loads := make([]model.Load, len(req.Loads))
	for j, l := range req.Loads {
		loads[j] = protoToLoad(l)
	}

	res := model.NewResourceState(drivers, loads)
	info, err := model.NewInformationState(epoch, 1.0, 2.50, 0)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid information state: %v", err)
	}

	costCfg := protoToCostConfig(req.CostConfig)
	feasCfg := protoToFeasibilityConfig(req.FeasibilityConfig)

	var action *model.Action
	var prov policy.DecisionProvenance

	competitorScale := strings.ToUpper(strings.TrimSpace(req.CompetitorScale))
	if competitorScale == "N1" || competitorScale == "N_GENERIC" || competitorScale == "COMPETITIVE" {
		scale := model.AggregatedMarket{LatentStates: []string{"aggressive", "balanced", "defensive"}}
		compBelief, bErr := model.NewBelief(scale, scale.LatentStates, []float64{0.3333333333333333, 0.3333333333333333, 0.3333333333333334})
		if bErr != nil {
			return nil, status.Errorf(codes.Internal, "belief initialization failed: %v", bErr)
		}
		compState, sErr := model.NewState(res, info, compBelief)
		if sErr != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid state construction: %v", sErr)
		}

		rm := model.NewRegionManager(1.0, nil)
		var compPol policy.Policy[model.AggregatedMarket]
		switch strings.ToUpper(strings.TrimSpace(req.PolicyClass)) {
		case "PIECEWISEVFA", "VFA":
			pvfaTable := policy.NewPiecewiseLinearVFATable(nil)
			var polErr error
			compPol, polErr = policy.NewPiecewiseVFAPolicy[model.AggregatedMarket](
				pvfaTable,
				nil,
				0.95,
				costCfg,
				feasCfg,
				rm,
			)
			if polErr != nil {
				return nil, status.Errorf(codes.Internal, "policy init failed: %v", polErr)
			}
		case "DLA":
			cfaBase := policy.NewCFAPolicy[model.AggregatedMarket](policy.DefaultCFAParameters(), costCfg, feasCfg, nil)
			var polErr error
			compPol, polErr = policy.NewDLAPolicy[model.AggregatedMarket](
				policy.DefaultDLAParameters(),
				costCfg,
				feasCfg,
				cfaBase,
				nil,
				rm,
				nil,
				nil,
			)
			if polErr != nil {
				return nil, status.Errorf(codes.Internal, "dla init failed: %v", polErr)
			}
		default: // CFA
			compPol = policy.NewCFAPolicy[model.AggregatedMarket](policy.DefaultCFAParameters(), costCfg, feasCfg, rm)
		}

		optService := service.NewOptimizationService[model.AggregatedMarket](s.journal, nil).WithCryptoStore(s.cryptoStore)
		action, prov, _, err = optService.OptimizeEpoch(ctx, compState, compPol, epoch+3600, nil)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "optimization failed: %v", err)
		}

	} else { // Monopolistic N=0
		monoBelief := model.NewMonopolisticBelief()
		monoState, sErr := model.NewState(res, info, monoBelief)
		if sErr != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid monopolistic state: %v", sErr)
		}

		rm := model.NewRegionManager(1.0, nil)
		var monoPol policy.Policy[model.Monopolistic]
		switch strings.ToUpper(strings.TrimSpace(req.PolicyClass)) {
		case "PIECEWISEVFA", "VFA":
			pvfaTable := policy.NewPiecewiseLinearVFATable(nil)
			var polErr error
			monoPol, polErr = policy.NewPiecewiseVFAPolicy[model.Monopolistic](
				pvfaTable,
				nil,
				0.95,
				costCfg,
				feasCfg,
				rm,
			)
			if polErr != nil {
				return nil, status.Errorf(codes.Internal, "policy init failed: %v", polErr)
			}
		case "DLA":
			cfaBase := policy.NewCFAPolicy[model.Monopolistic](policy.DefaultCFAParameters(), costCfg, feasCfg, nil)
			var polErr error
			monoPol, polErr = policy.NewDLAPolicy[model.Monopolistic](
				policy.DefaultDLAParameters(),
				costCfg,
				feasCfg,
				cfaBase,
				nil,
				rm,
				nil,
				nil,
			)
			if polErr != nil {
				return nil, status.Errorf(codes.Internal, "dla init failed: %v", polErr)
			}
		default: // CFA
			monoPol = policy.NewCFAPolicy[model.Monopolistic](policy.DefaultCFAParameters(), costCfg, feasCfg, rm)
		}

		optService := service.NewOptimizationService[model.Monopolistic](s.journal, nil).WithCryptoStore(s.cryptoStore)
		action, prov, _, err = optService.OptimizeEpoch(ctx, monoState, monoPol, epoch+3600, nil)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "monopolistic policy evaluation failed: %v", err)
		}
	}

	execDurationMs := float64(time.Since(t0).Microseconds()) / 1000.0

	// Construct Response
	matches := make([]*mittensv1.Match, 0, action.MatchCount())
	for _, m := range action.Matches() {
		matches = append(matches, &mittensv1.Match{
			DriverId:              m.DriverID,
			LoadId:                m.LoadID,
			DispatchEpoch:         m.DispatchEpoch,
			EstimatedContribution: m.EstimatedContribution,
		})
	}

	span.SetAttributes(
		attribute.Int("mittens.match_count", len(matches)),
		attribute.Float64("mittens.net_contribution", prov.TotalNetContribution),
		attribute.Float64("mittens.execution_ms", execDurationMs),
	)

	decisionID := prov.OptimizationRunID
	if decisionID == "" {
		decisionID = fmt.Sprintf("OPT_%d", epoch)
	}

	policyName := prov.PolicyName
	if policyName == "" {
		policyName = req.PolicyClass
	}
	if policyName == "" {
		policyName = "CFA"
	}
	runID := fmt.Sprintf("RUN-%s", policyName)

	return &mittensv1.OptimizeResponse{
		DecisionId:           decisionID,
		RunId:                runID,
		Epoch:                epoch,
		MatchCount:           int32(len(matches)),
		Matches:              matches,
		TotalNetContribution: prov.TotalNetContribution,
		TotalObjectiveValue:  prov.TotalObjectiveValue,
		ExecutionDurationMs:  execDurationMs,
		Provenance:           provenanceToProto(prov),
	}, nil
}

// StreamTelemetry ingests real-time GPS and ELD Hours-of-Service driver telemetry.
func (s *Server) StreamTelemetry(ctx context.Context, req *mittensv1.StreamTelemetryRequest) (*mittensv1.StreamTelemetryResponse, error) {
	if req.DriverId == "" {
		return nil, status.Error(codes.InvalidArgument, "driver_id is required")
	}

	lat := 0.0
	lon := 0.0
	nodeID := ""
	if req.CurrentLocation != nil {
		lat = req.CurrentLocation.Lat
		lon = req.CurrentLocation.Lon
		nodeID = req.CurrentLocation.NodeId
	}

	ping := stream.ELDDriverPingDTO{
		DriverID:            req.DriverId,
		Timestamp:           req.Epoch,
		Lat:                 lat,
		Lon:                 lon,
		NodeID:              nodeID,
		DriveHoursRemaining: req.RemainingDriveHours,
		DutyHoursRemaining:  req.RemainingDutyHours,
	}

	if err := s.streamBuffer.IngestDriverBatch([]stream.ELDDriverPingDTO{ping}); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed ingesting driver telemetry: %v", err)
	}

	return &mittensv1.StreamTelemetryResponse{
		Accepted: true,
		Message:  fmt.Sprintf("Driver telemetry accepted for %s", req.DriverId),
	}, nil
}

// StreamTenders ingests incoming spot tenders and contract load commitments.
func (s *Server) StreamTenders(ctx context.Context, req *mittensv1.StreamTendersRequest) (*mittensv1.StreamTendersResponse, error) {
	if len(req.Loads) == 0 {
		return nil, status.Error(codes.InvalidArgument, "request must contain at least one load tender")
	}

	tenders := make([]stream.TMSLoadTenderDTO, len(req.Loads))
	for idx, l := range req.Loads {
		origLat, origLon, origNode := 0.0, 0.0, ""
		if l.Origin != nil {
			origLat = l.Origin.Lat
			origLon = l.Origin.Lon
			origNode = l.Origin.NodeId
		}

		destLat, destLon, destNode := 0.0, 0.0, ""
		if l.Destination != nil {
			destLat = l.Destination.Lat
			destLon = l.Destination.Lon
			destNode = l.Destination.NodeId
		}

		tenders[idx] = stream.TMSLoadTenderDTO{
			LoadID:                l.Id,
			Timestamp:             l.PickupEarliestEpoch,
			OriginNodeID:          origNode,
			OriginLat:             origLat,
			OriginLon:             origLon,
			DestinationNodeID:     destNode,
			DestLat:               destLat,
			DestLon:               destLon,
			PickupEarliestEpoch:   l.PickupEarliestEpoch,
			PickupLatestEpoch:     l.PickupLatestEpoch,
			DeliveryEarliestEpoch: l.DeliveryEarliestEpoch,
			DeliveryLatestEpoch:   l.DeliveryLatestEpoch,
			Revenue:               l.Revenue,
			RequiredEquipment:     l.RequiredEquipment,
		}
	}

	if err := s.streamBuffer.IngestTenderBatch(tenders); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed ingesting load tenders: %v", err)
	}

	return &mittensv1.StreamTendersResponse{
		AcceptedCount: int32(len(tenders)),
		Message:       fmt.Sprintf("Accepted %d load tenders", len(tenders)),
	}, nil
}

// PlanRepositioning computes empty tractor rebalancing moves across regional nodes.
func (s *Server) PlanRepositioning(ctx context.Context, req *mittensv1.RepositionPlanRequest) (*mittensv1.RepositionPlanResponse, error) {
	if len(req.IdleDrivers) == 0 {
		return &mittensv1.RepositionPlanResponse{
			MoveCount: 0,
			Moves:     nil,
		}, nil
	}

	epoch := req.Epoch
	if epoch <= 0 {
		epoch = time.Now().Unix()
	}

	drivers := make([]model.Driver, len(req.IdleDrivers))
	for i, dDTO := range req.IdleDrivers {
		drivers[i] = protoToDriver(dDTO, epoch)
	}

	resource := model.NewResourceState(drivers, nil)
	regionMgr := model.NewRegionManager(1.0, nil)

	cfg := reposition.DefaultRepositioningConfig()
	if req.MaxRepositionDistanceMiles > 0 {
		cfg.MaxRepositionDistanceMiles = req.MaxRepositionDistanceMiles
	}

	moves, err := s.repositionSynth.SynthesizeRepositioningMoves(ctx, resource, regionMgr, drivers, cfg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "reposition synthesis failed: %v", err)
	}

	protoMoves := make([]*mittensv1.RepositionMove, len(moves))
	var totalCost, totalLift float64
	for idx, m := range moves {
		protoMoves[idx] = &mittensv1.RepositionMove{
			DriverId:                    m.DriverID,
			OriginRegion:                m.OriginLocation.NodeID,
			DestinationRegion:           m.TargetRegionID,
			EstimatedEmptyMiles:         m.DeadheadMiles,
			ExpectedDownstreamValueLift: m.ExpectedArbitrageYield,
		}
		totalCost += m.EstimatedCost
		totalLift += m.ExpectedArbitrageYield
	}

	return &mittensv1.RepositionPlanResponse{
		MoveCount:           int32(len(protoMoves)),
		Moves:               protoMoves,
		TotalRepositionCost: totalCost,
		ExpectedNetLift:     totalLift - totalCost,
	}, nil
}

// ExplainDecision returns mathematical provenance and marginal attribution for a committed dispatch.
func (s *Server) ExplainDecision(ctx context.Context, req *mittensv1.ExplainDecisionRequest) (*mittensv1.ExplainDecisionResponse, error) {
	if req.DecisionId == "" {
		return nil, status.Error(codes.InvalidArgument, "decision_id is required")
	}

	entry, found := s.journal.GetEntry(req.DecisionId)
	if !found {
		return nil, status.Errorf(codes.NotFound, "no journal entry found for decision ID '%s'", req.DecisionId)
	}

	explainer := explain.NewExplainer()
	formatter := explain.NewFormatter()

	explanation, err := explainer.ExplainDecision(entry.Provenance, nil, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed generating explanation: %v", err)
	}

	topCandidates := make([]*mittensv1.ArcExplanation, 0)
	for _, me := range explanation.MatchedExplanations {
		topCandidates = append(topCandidates, &mittensv1.ArcExplanation{
			DriverId:          me.DriverID,
			LoadId:            me.AssignedLoadID,
			BaseRevenue:       me.EconomicBreakdown.GrossRevenue,
			TravelCost:        me.EconomicBreakdown.LoadedDriveCost + me.EconomicBreakdown.EmptyDeadheadCost,
			PostDecisionValue: me.EconomicBreakdown.DownstreamRegionalVFA,
			RiskDiscount:      me.EconomicBreakdown.CompetitorRiskPremium,
			NetObjective:      me.WinningScore,
			Selected:          true,
			ExplanationText:   me.Summary,
		})
	}

	return &mittensv1.ExplainDecisionResponse{
		DecisionId:         req.DecisionId,
		PolicyClass:        explanation.PolicyName,
		EvaluatedArcsCount: int32(explanation.TotalDrivers),
		TopCandidates:      topCandidates,
		MarkdownSummary:    formatter.FormatMarkdown(explanation),
	}, nil
}

// ReplayDecision executes bit-exact deterministic replay of a past optimization epoch.
func (s *Server) ReplayDecision(ctx context.Context, req *mittensv1.ReplayDecisionRequest) (*mittensv1.ReplayDecisionResponse, error) {
	if req.DecisionId == "" {
		return nil, status.Error(codes.InvalidArgument, "decision_id is required")
	}

	entry, found := s.journal.GetEntry(req.DecisionId)
	if !found {
		return nil, status.Errorf(codes.NotFound, "no journal entry found for decision ID '%s'", req.DecisionId)
	}

	cryptoRec := entry.CryptographicRecord
	if cryptoRec.DecisionID == "" {
		if rec, err := s.cryptoStore.Get(req.DecisionId); err == nil {
			cryptoRec = rec
		} else {
			return nil, status.Errorf(codes.NotFound, "no cryptographic record found for decision ID '%s'", req.DecisionId)
		}
	}

	res, err := pkgjournal.DecodeCanonicalResource(cryptoRec.ResourceStateBytes)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed decoding resource state: %v", err)
	}
	info, err := pkgjournal.DecodeCanonicalInformation(cryptoRec.InformationStateBytes)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed decoding information state: %v", err)
	}
	belief, err := pkgjournal.DecodeCanonicalBelief(model.Monopolistic{}, cryptoRec.BeliefStateBytes)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed decoding belief state: %v", err)
	}
	state, err := model.NewState(res, info, belief)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed reconstructing state: %v", err)
	}

	cfaPol := policy.NewCFAPolicy[model.Monopolistic](
		policy.DefaultCFAParameters(),
		model.DefaultCostConfig(),
		model.DefaultFeasibilityConfig(),
		nil,
	)

	replayEngine, err := replay.NewReplayEngine[model.Monopolistic](cfaPol)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed initializing replay engine: %v", err)
	}

	scorecard, err := replayEngine.ReplayDecision(ctx, cryptoRec, state)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "replay failed: %v", err)
	}

	statusStr := "BIT_EXACT_MATCH"
	if !scorecard.IsBitExact {
		statusStr = "DIVERGENCE_DETECTED"
	}

	return &mittensv1.ReplayDecisionResponse{
		DecisionId:            scorecard.DecisionID,
		Status:                statusStr,
		DriftAmount:           scorecard.ContributionDelta,
		InitialStateHashMatch: scorecard.InitialStateHashMatch,
		ActionHashMatch:       scorecard.ActionHashMatch,
		Details:               fmt.Sprintf("Original objective: $%.2f, Replayed objective: $%.2f", scorecard.RecordedNetContribution, scorecard.ReplayedNetContribution),
	}, nil
}

// VerifyMerkleChain cryptographically validates the tamper-evident SHA-256 audit log.
func (s *Server) VerifyMerkleChain(ctx context.Context, req *mittensv1.VerifyMerkleChainRequest) (*mittensv1.VerifyMerkleChainResponse, error) {
	if req.RunId == "" {
		return nil, status.Error(codes.InvalidArgument, "run_id is required")
	}

	valid, lastHash, err := s.cryptoStore.VerifyRunChain(req.RunId)
	msg := "Chain continuous and cryptographic self-hashes verified"
	if err != nil {
		msg = err.Error()
	}

	return &mittensv1.VerifyMerkleChainResponse{
		RunId:                req.RunId,
		IsValid:              valid,
		TotalRecordsVerified: 1,
		LatestRecordHash:     lastHash,
		VerificationMessage:  msg,
	}, nil
}

// ----------------------------------------------------------------------------
// Domain <-> Protobuf Type Converters
// ----------------------------------------------------------------------------

func protoToDriver(d *mittensv1.Driver, defaultEpoch int64) model.Driver {
	loc := model.Location{}
	if d.CurrentLocation != nil {
		loc = model.Location{
			NodeID: d.CurrentLocation.NodeId,
			Lat:    d.CurrentLocation.Lat,
			Lon:    d.CurrentLocation.Lon,
		}
	}

	homeLoc := loc
	if d.HomeLocation != nil {
		homeLoc = model.Location{
			NodeID: d.HomeLocation.NodeId,
			Lat:    d.HomeLocation.Lat,
			Lon:    d.HomeLocation.Lon,
		}
	}

	eqType := model.EquipDryVan
	if d.Equipment != nil {
		eqType = parseEquipmentType(d.Equipment.Type)
	}

	availEpoch := d.AvailableEpoch
	if availEpoch <= 0 {
		availEpoch = defaultEpoch
	}

	driveRem := d.RemainingDriveHours
	if driveRem <= 0 {
		driveRem = 11.0
	}
	dutyRem := d.RemainingDutyHours
	if dutyRem <= 0 {
		dutyRem = 14.0
	}

	return model.Driver{
		ID:                  d.Id,
		CurrentLocation:     loc,
		HomeLocation:        homeLoc,
		AvailableEpoch:      availEpoch,
		DriveHoursRemaining: driveRem,
		DutyHoursRemaining:  dutyRem,
		Equipment:           model.Equipment{Type: eqType},
		Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(availEpoch, 0)),
	}
}

func protoToLoad(l *mittensv1.Load) model.Load {
	orig := model.Location{}
	if l.Origin != nil {
		orig = model.Location{
			NodeID: l.Origin.NodeId,
			Lat:    l.Origin.Lat,
			Lon:    l.Origin.Lon,
		}
	}

	dest := model.Location{}
	if l.Destination != nil {
		dest = model.Location{
			NodeID: l.Destination.NodeId,
			Lat:    l.Destination.Lat,
			Lon:    l.Destination.Lon,
		}
	}

	return model.Load{
		ID:                    l.Id,
		Origin:                orig,
		Destination:           dest,
		PickupEarliestEpoch:   l.PickupEarliestEpoch,
		PickupLatestEpoch:     l.PickupLatestEpoch,
		DeliveryEarliestEpoch: l.DeliveryEarliestEpoch,
		DeliveryLatestEpoch:   l.DeliveryLatestEpoch,
		Revenue:               l.Revenue,
		RequiredEquipment:     parseEquipmentType(l.RequiredEquipment),
	}
}

func protoToCostConfig(c *mittensv1.CostConfig) model.CostConfig {
	cfg := model.DefaultCostConfig()
	if c == nil {
		return cfg
	}
	if c.LoadedCostPerMile > 0 {
		cfg.LoadedMileRate = c.LoadedCostPerMile
	}
	if c.EmptyCostPerMile > 0 {
		cfg.EmptyMileRate = c.EmptyCostPerMile
	}
	if c.LatePickupPenaltyPerHour > 0 {
		cfg.EarlyArrivalPerHour = c.LatePickupPenaltyPerHour
	}
	if c.LateDeliveryPenaltyPerHour > 0 {
		cfg.LateDeliveryPerHour = c.LateDeliveryPenaltyPerHour
	}
	return cfg
}

func protoToFeasibilityConfig(f *mittensv1.FeasibilityConfig) model.FeasibilityConfig {
	cfg := model.DefaultFeasibilityConfig()
	if f == nil {
		return cfg
	}
	if f.MaxEmptyMiles > 0 {
		cfg.MaxDeadheadMiles = f.MaxEmptyMiles
	}
	return cfg
}

func provenanceToProto(prov policy.DecisionProvenance) *mittensv1.DecisionProvenance {
	evalArcs := make([]*mittensv1.CandidateEvaluation, len(prov.EvaluatedArcs))
	for i, ea := range prov.EvaluatedArcs {
		evalArcs[i] = &mittensv1.CandidateEvaluation{
			DriverId:               ea.DriverID,
			LoadId:                 ea.LoadID,
			NetContribution:        ea.CostBreakdown.NetContribution,
			PostDecisionStateValue: ea.VFAValue,
			CompetitorRiskDiscount: ea.DLAValue,
			TotalScore:             ea.TotalScore,
			Feasible:               ea.IsAssigned,
		}
	}

	return &mittensv1.DecisionProvenance{
		OptimizationRunId:    prov.OptimizationRunID,
		BatchEpoch:           prov.BatchEpoch,
		PolicyName:           prov.PolicyName,
		ThetaParameters:      prov.ThetaParameters,
		EvaluatedArcs:        evalArcs,
		MatchedCount:         int32(prov.MatchedCount),
		TotalNetContribution: prov.TotalNetContribution,
		TotalObjectiveValue:  prov.TotalObjectiveValue,
		ActiveBeliefVector:   prov.ActiveBelief,
		PricingVariables:     prov.PricingVariables,
	}
}

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
