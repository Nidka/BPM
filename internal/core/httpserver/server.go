package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/Nidka/BPM/internal/core/config"
	"github.com/Nidka/BPM/internal/core/contracts"
)

type Server struct {
	http            *http.Server
	logger          *slog.Logger
	shutdownTimeout time.Duration
	port            int
	localURL        string
	apiURL          string
	startupReporter contracts.StartupReporter
}

func New(
	cfg config.Config,
	logger *slog.Logger,
	handler http.Handler,
	startupReporter contracts.StartupReporter,
) *Server {
	localURL := ""
	if !cfg.App.IsProductionLike() {
		localURL = fmt.Sprintf("http://localhost:%d", cfg.HTTP.Port)
	}

	return &Server{
		http: &http.Server{
			Addr:              cfg.HTTP.Address(),
			Handler:           handler,
			ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
			ReadTimeout:       cfg.HTTP.ReadTimeout,
			WriteTimeout:      cfg.HTTP.WriteTimeout,
			IdleTimeout:       cfg.HTTP.IdleTimeout,
			MaxHeaderBytes:    1 << 20,
		},
		logger:          logger,
		shutdownTimeout: cfg.HTTP.ShutdownTimeout,
		port:            cfg.HTTP.Port,
		localURL:        localURL,
		apiURL:          localURL + cfg.HTTP.APIBasePath,
		startupReporter: startupReporter,
	}
}

func (server *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", server.http.Addr)
	if err != nil {
		return fmt.Errorf("listen HTTP on port %d: %w", server.port, err)
	}

	server.logger.Info("HTTP server is ready",
		"component", "http",
		"status", "listening",
		"address", server.http.Addr,
		"port", server.port,
	)
	if server.localURL != "" {
		server.logger.Info("local API available",
			"api", server.apiURL,
			"health", server.localURL+"/health",
			"readiness", server.localURL+"/ready",
		)
	}
	if server.startupReporter != nil {
		server.startupReporter.ReportStartup(contracts.StartupInfo{
			Address:       server.http.Addr,
			Port:          server.port,
			APIURL:        server.apiURL,
			HealthURL:     server.localURL + "/health",
			ReadinessURL:  server.localURL + "/ready",
			DatabaseReady: true,
		})
	}

	errorChannel := make(chan error, 1)
	go func() {
		if err := server.http.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorChannel <- err
			return
		}
		errorChannel <- nil
	}()

	select {
	case err := <-errorChannel:
		if err != nil {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
		server.logger.Info("shutting down HTTP server", "component", "http")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), server.shutdownTimeout)
		defer cancel()
		if err := server.http.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		return nil
	}
}
