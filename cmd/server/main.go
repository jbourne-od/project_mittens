// Package main provides the production HTTP and gRPC server daemon for Project Mittens.
//
// It hosts the OpenAPI 3.1 REST API endpoints for optimization and simulation,
// the high-speed binary gRPC OptimizerService protocol, scrapes Prometheus metrics on /metrics,
// exposes /healthz probes, and coordinates distributed tracing with OpenTelemetry collectors.
//
// In accordance with Inviolate 0 (Explicit Configuration) and Inviolate 4 (Closed Business Logic),
// all parameters, dependencies, and execution contexts are injected explicitly at runtime.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/adapter/api"
	mittensgrpc "github.com/optimaldynamics/project-mittens/internal/adapter/grpc"
	"github.com/optimaldynamics/project-mittens/pkg/logging"
	"github.com/optimaldynamics/project-mittens/pkg/telemetry"
	"google.golang.org/grpc"
)

func main() {
	host := flag.String("host", "0.0.0.0", "HTTP/gRPC server bind address")
	port := flag.Int("port", 8080, "HTTP server bind port")
	grpcPort := flag.Int("grpc-port", 9090, "gRPC server bind port (0 to disable)")
	readTimeoutSec := flag.Int("read-timeout", 15, "HTTP request read timeout in seconds")
	writeTimeoutSec := flag.Int("write-timeout", 30, "HTTP response write timeout in seconds")
	idleTimeoutSec := flag.Int("idle-timeout", 60, "HTTP keep-alive idle timeout in seconds")
	otlpEndpoint := flag.String("otlp-endpoint", "", "OpenTelemetry collector or Tempo gRPC endpoint (e.g. tempo:4317)")
	enableTracing := flag.Bool("enable-tracing", true, "Enable OpenTelemetry distributed tracing")
	logLevel := flag.String("log-level", "info", "Structured log level: debug, info, warn, error")
	dbURL := flag.String("database-url", "", "PostgreSQL database connection URL (e.g. postgres://user:pw@host:5432/mittens)")
	flag.Parse()

	// Environment variable overrides for cloud-native deployment (AWS Fargate / App Runner / Cloud Run)
	serverHost := *host
	if envHost := os.Getenv("HOST"); envHost != "" && *host == "0.0.0.0" {
		serverHost = envHost
	}

	serverPort := *port
	if envPort := os.Getenv("PORT"); envPort != "" && *port == 8080 {
		if p, err := strconv.Atoi(envPort); err == nil {
			serverPort = p
		}
	}

	serverGRPCPort := *grpcPort
	if envGRPCPort := os.Getenv("GRPC_PORT"); envGRPCPort != "" && *grpcPort == 9090 {
		if p, err := strconv.Atoi(envGRPCPort); err == nil {
			serverGRPCPort = p
		}
	}

	logLvlStr := *logLevel
	if envLvl := os.Getenv("LOG_LEVEL"); envLvl != "" && *logLevel == "info" {
		logLvlStr = envLvl
	}

	endpoint := *otlpEndpoint
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if endpoint == "" {
		endpoint = "tempo:4317"
	}

	databaseURL := *dbURL
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}

	// 1. Initialize structured logging explicitly (Inviolate 0)
	var lvl logging.Level
	switch logLvlStr {
	case "debug":
		lvl = logging.LevelDebug
	case "warn":
		lvl = logging.LevelWarn
	case "error":
		lvl = logging.LevelError
	default:
		lvl = logging.LevelInfo
	}

	logger := logging.New(logging.Config{
		Level:  lvl,
		Format: logging.FormatJSON,
	})
	slog.SetDefault(logger)

	slog.Info("Starting Project Mittens Optimization Server",
		"version", "1.0.0",
		"host", serverHost,
		"http_port", serverPort,
		"grpc_port", serverGRPCPort,
		"otlp_endpoint", endpoint,
		"tracing_enabled", *enableTracing,
	)

	// 2. Initialize OpenTelemetry tracing & metrics (Inviolate 0)
	telemetryCfg := telemetry.TelemetryConfig{
		ServiceName:    "project-mittens",
		ServiceVersion: "1.0.0",
		Environment:    "production",
		EnableTracing:  *enableTracing,
		EnableMetrics:  true,
		SampleRate:     1.0,
		OTLPEndpoint:   endpoint,
	}

	telemetryProvider, err := telemetry.InitGlobalProvider(telemetryCfg)
	if err != nil {
		slog.Warn("Failed initializing full OpenTelemetry provider, falling back to Prometheus defaults", "error", err)
	} else {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := telemetryProvider.Shutdown(shutdownCtx); err != nil {
				slog.Error("Error shutting down telemetry provider", "error", err)
			}
		}()
	}

	// 3. Instantiate HTTP REST API Server with explicit configuration
	srvCfg := api.ServerConfig{
		Host:            serverHost,
		Port:            serverPort,
		ReadTimeoutSec:  *readTimeoutSec,
		WriteTimeoutSec: *writeTimeoutSec,
		IdleTimeoutSec:  *idleTimeoutSec,
		DatabaseURL:     databaseURL,
	}
	server, err := api.NewServer(srvCfg)
	if err != nil {
		slog.Error("Failed to initialize API server", "error", err)
		os.Exit(1)
	}

	// 4. Instantiate and Register gRPC Adapter
	var grpcAdapter *mittensgrpc.Server
	var rawGRPCSrv *grpc.Server
	if serverGRPCPort > 0 {
		apiDeps := server.Dependencies()
		grpcDeps := mittensgrpc.Dependencies{
			Journal:         apiDeps.Journal,
			CryptoStore:     apiDeps.CryptoStore,
			DBPool:          apiDeps.DBPool,
			RunRepository:   apiDeps.RunRepository,
			StreamBuffer:    apiDeps.StreamBuffer,
			StreamSync:      apiDeps.StreamSync,
			RepositionSynth: apiDeps.RepositionSynth,
		}
		grpcAdapter = mittensgrpc.NewServer(mittensgrpc.ServerConfig{
			Host: serverHost,
			Port: serverGRPCPort,
		}, grpcDeps)

		rawGRPCSrv = grpc.NewServer()
		grpcAdapter.Register(rawGRPCSrv)
	}

	// 5. Start Servers in background goroutines
	serverErrChan := make(chan error, 2)
	go func() {
		slog.Info("HTTP server listening", "address", fmt.Sprintf("%s:%d", serverHost, serverPort))
		if err := server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrChan <- fmt.Errorf("http server: %w", err)
		}
	}()

	if grpcAdapter != nil {
		go func() {
			slog.Info("gRPC server listening", "address", fmt.Sprintf("%s:%d", serverHost, serverGRPCPort))
			if err := grpcAdapter.Start(); err != nil {
				serverErrChan <- fmt.Errorf("grpc server: %w", err)
			}
		}()
	}

	// 6. Setup graceful shutdown listener for OS termination signals
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-serverErrChan:
		slog.Error("Server failed unexpectedly", "error", err)
		os.Exit(1)
	case sig := <-stopChan:
		slog.Info("Received OS termination signal, initiating graceful shutdown", "signal", sig.String())
	}

	// 7. Gracefully shut down servers with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if grpcAdapter != nil {
		grpcAdapter.Stop()
		slog.Info("gRPC server stopped")
	}

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Graceful HTTP shutdown failed, forcing server exit", "error", err)
		os.Exit(1)
	}

	slog.Info("Project Mittens Optimization Server exited cleanly")
}
