package logging_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/optimaldynamics/project-mittens/pkg/logging"
)

func TestLogger_LevelsAndFormat(t *testing.T) {
	var buf bytes.Buffer

	cfg := logging.Config{
		Level:  logging.LevelInfo,
		Format: logging.FormatJSON,
		Output: &buf,
	}

	logger := logging.New(cfg)

	// Debug should be suppressed at LevelInfo
	logger.Debug("this is debug")
	if buf.Len() > 0 {
		t.Fatalf("expected debug log to be suppressed, got: %s", buf.String())
	}

	// Info should be emitted as JSON
	logger.Info("optimization started", "drivers", 10, "loads", 15)
	output := buf.String()

	if !strings.Contains(output, `"level":"INFO"`) {
		t.Fatalf("expected JSON level INFO in output, got: %s", output)
	}
	if !strings.Contains(output, `"msg":"optimization started"`) {
		t.Fatalf("expected JSON msg in output, got: %s", output)
	}
	if !strings.Contains(output, `"drivers":10`) {
		t.Fatalf("expected JSON attribute drivers in output, got: %s", output)
	}
}

func TestLogger_ContextPropagation(t *testing.T) {
	var buf bytes.Buffer

	cfg := logging.Config{
		Level:  logging.LevelDebug,
		Format: logging.FormatText,
		Output: &buf,
	}

	baseLogger := logging.New(cfg)

	ctx := logging.WithContextData(context.Background(), logging.ContextData{
		OptimizationRunID: "RUN-1234",
		BatchEpoch:        1724054400,
		PolicyClass:       "CFA",
	})

	ctxLogger := logging.FromContext(ctx, baseLogger)
	ctxLogger.Debug("evaluating batch")

	out := buf.String()
	if !strings.Contains(out, "run_id=RUN-1234") {
		t.Fatalf("expected run_id in log, got: %s", out)
	}
	if !strings.Contains(out, "epoch=1724054400") {
		t.Fatalf("expected epoch in log, got: %s", out)
	}
	if !strings.Contains(out, "policy=CFA") {
		t.Fatalf("expected policy in log, got: %s", out)
	}
}

func TestLogger_Nop(t *testing.T) {
	nop := logging.NewNop()
	nop.Info("should do nothing", "k", "v")
	nop.Error("should do nothing", "k", "v")
}
