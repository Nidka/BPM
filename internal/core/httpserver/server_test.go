package httpserver

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Nidka/BPM-service/internal/core/config"
)

func TestRunReportsOccupiedPortBeforeLoggingReady(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve test port: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	server := New(config.Config{
		App: config.AppConfig{Environment: config.Test},
		HTTP: config.HTTPConfig{
			Host:            "127.0.0.1",
			Port:            port,
			APIBasePath:     "/api/v1",
			ShutdownTimeout: time.Second,
		},
	}, logger, http.NotFoundHandler(), nil)

	err = server.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "listen HTTP on port "+strconv.Itoa(port)) {
		t.Fatalf("expected occupied port error, got %v", err)
	}
	if strings.Contains(logs.String(), "HTTP server is ready") {
		t.Fatalf("server must not log readiness before binding the port: %s", logs.String())
	}
}
