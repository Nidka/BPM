package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/Nidka/BPM/internal/core/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, cfg config.DatabaseConfig, logger *slog.Logger) (*pgxpool.Pool, error) {
	startedAt := time.Now()
	logger.Debug("connecting to PostgreSQL",
		"component", "database",
		"timeout", cfg.ConnectTimeout,
	)

	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		// Do not wrap the driver error because it can contain the connection URL.
		return nil, errors.New("DATABASE_URL is invalid")
	}

	poolConfig.MinConns = cfg.MinConnections
	poolConfig.MaxConns = cfg.MaxConnections
	poolConfig.MaxConnIdleTime = cfg.MaxConnectionIdle
	poolConfig.MaxConnLifetime = cfg.MaxConnectionLife
	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	poolConfig.ConnConfig.RuntimeParams["application_name"] = "bpm-api"
	poolConfig.ConnConfig.RuntimeParams["timezone"] = "UTC"
	poolConfig.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(cfg.StatementTimeout.Milliseconds(), 10)
	poolConfig.ConnConfig.RuntimeParams["lock_timeout"] = strconv.FormatInt(cfg.LockTimeout.Milliseconds(), 10)
	poolConfig.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "30000"

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	logger.Info("PostgreSQL connected successfully",
		"component", "database",
		"status", "ready",
		"connection_time_ms", time.Since(startedAt).Milliseconds(),
		"pool_min", cfg.MinConnections,
		"pool_max", cfg.MaxConnections,
	)
	return pool, nil
}
