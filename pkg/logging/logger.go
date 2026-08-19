package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Level defines the logging severity level.
type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

// ToSlogLevel converts Level string to slog.Level.
func (l Level) ToSlogLevel() slog.Level {
	switch strings.ToUpper(string(l)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Format defines the serialization output format.
type Format string

const (
	FormatJSON Format = "JSON"
	FormatText Format = "TEXT"
)

// Config configures structured slog logger initialization.
type Config struct {
	Level     Level     `json:"level"`
	Format    Format    `json:"format"`
	Output    io.Writer `json:"-"`
	AddSource bool      `json:"add_source"`
}

// DefaultConfig returns standard human-readable info-level text logging.
func DefaultConfig() Config {
	return Config{
		Level:     LevelInfo,
		Format:    FormatText,
		Output:    os.Stderr,
		AddSource: false,
	}
}

// DebugConfig returns high-verbosity debug-level logging.
func DebugConfig() Config {
	return Config{
		Level:     LevelDebug,
		Format:    FormatText,
		Output:    os.Stderr,
		AddSource: true,
	}
}

// New creates a structured slog.Logger based on explicit Config (Inviolate 0).
func New(cfg Config) *slog.Logger {
	out := cfg.Output
	if out == nil {
		out = os.Stderr
	}

	opts := &slog.HandlerOptions{
		Level:     cfg.Level.ToSlogLevel(),
		AddSource: cfg.AddSource,
	}

	var handler slog.Handler
	if cfg.Format == FormatJSON {
		handler = slog.NewJSONHandler(out, opts)
	} else {
		handler = slog.NewTextHandler(out, opts)
	}

	return slog.New(handler)
}

// NewNop returns a no-op logger that discards all log output.
// Useful in tight simulation loops and Monte Carlo tree search (Inviolate 6).
func NewNop() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError + 100, // Never triggers
	}))
}

type ctxKey struct{}

// ContextData carries execution correlation IDs through context.Context.
type ContextData struct {
	OptimizationRunID string
	BatchEpoch        int64
	PolicyClass       string
}

// WithContextData injects optimization context metadata into context.Context.
func WithContextData(ctx context.Context, data ContextData) context.Context {
	return context.WithValue(ctx, ctxKey{}, data)
}

// GetContextData retrieves optimization context metadata from context.Context.
func GetContextData(ctx context.Context) (ContextData, bool) {
	data, ok := ctx.Value(ctxKey{}).(ContextData)
	return data, ok
}

// FromContext extracts context data and returns a logger with bound correlation attributes.
func FromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	if base == nil {
		base = slog.Default()
	}
	if data, ok := GetContextData(ctx); ok {
		var attrs []any
		if data.OptimizationRunID != "" {
			attrs = append(attrs, slog.String("run_id", data.OptimizationRunID))
		}
		if data.BatchEpoch != 0 {
			attrs = append(attrs, slog.Int64("epoch", data.BatchEpoch))
		}
		if data.PolicyClass != "" {
			attrs = append(attrs, slog.String("policy", data.PolicyClass))
		}
		if len(attrs) > 0 {
			return base.With(attrs...)
		}
	}
	return base
}
