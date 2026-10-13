package observability

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Nidka/BPM-service/internal/core/config"
	"github.com/getsentry/sentry-go"
	"github.com/lmittmann/tint"
)

func NewLogger(cfg config.AppConfig, output io.Writer) *slog.Logger {
	level := new(slog.LevelVar)
	switch strings.ToUpper(cfg.LogLevel) {
	case "DEBUG":
		level.Set(slog.LevelDebug)
	case "WARN":
		level.Set(slog.LevelWarn)
	case "ERROR":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}

	var handler slog.Handler
	if cfg.Environment == config.Development || cfg.Environment == config.Test {
		handler = tint.NewTextHandler(output, &tint.Options{
			Level:      level,
			TimeFormat: "15:04:05.000",
		})
	} else {
		handler = slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level})
	}
	return slog.New(handler).With(
		"service", cfg.Name,
		"environment", cfg.Environment,
		"version", cfg.Version,
	)
}

func InitSentry(cfg config.Config) (func(), error) {
	if cfg.Observability.SentryDSN == "" {
		return func() {}, nil
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.Observability.SentryDSN,
		Environment:      cfg.App.Environment,
		Release:          cfg.App.Version,
		AttachStacktrace: true,
		SendDefaultPII:   false,
	}); err != nil {
		return nil, fmt.Errorf("initialize Sentry: %w", err)
	}
	return func() {
		sentry.Flush(2 * time.Second)
	}, nil
}

func DefaultLogger(cfg config.AppConfig) *slog.Logger {
	return NewLogger(cfg, os.Stdout)
}
