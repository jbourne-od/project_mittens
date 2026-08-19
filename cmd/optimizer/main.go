// Package main provides the static binary entrypoint for the Project Mittens optimizer.
//
// In accordance with Inviolate 0 (Explicit Configuration) and Inviolate 4 (Closed Business Logic),
// all parameters, dependencies, and execution contexts are injected explicitly at runtime.
// Package-level init() functions and ambient environment variable discovery are strictly prohibited.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/optimaldynamics/project-mittens/pkg/logging"
)

func main() {
	// Initialize structured slog logger
	cfg := logging.DefaultConfig()
	logger := logging.New(cfg)
	slog.SetDefault(logger)

	ctx := logging.WithContextData(context.Background(), logging.ContextData{
		OptimizationRunID: "BOOTSTRAP",
	})

	logger.InfoContext(ctx, "Project Mittens Optimization Engine Initialized",
		slog.String("version", "0.1.0-alpha"),
		slog.String("log_level", string(cfg.Level)),
		slog.String("format", string(cfg.Format)),
	)

	os.Exit(0)
}
