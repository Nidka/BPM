package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Nidka/BPM/internal/core/config"
	"github.com/Nidka/BPM/internal/core/observability"
)

func Run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger := observability.DefaultLogger(cfg.App)
	slog.SetDefault(logger)
	logger.Info("BPM API starting",
		"component", "application",
		"port", cfg.HTTP.Port,
		"api_base_path", cfg.HTTP.APIBasePath,
	)

	flushSentry, err := observability.InitSentry(cfg)
	if err != nil {
		return err
	}
	defer flushSentry()

	startupBanner := observability.DefaultStartupBanner(cfg.App)
	container, err := NewContainer(ctx, cfg, logger, startupBanner)
	if err != nil {
		return err
	}
	defer container.Close()
	return container.Server.Run(ctx)
}
