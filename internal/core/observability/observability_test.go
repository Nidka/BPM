package observability

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/Nidka/BPM-service/internal/core/config"
)

func TestDevelopmentLoggerUsesColorizedReadableOutput(t *testing.T) {
	var output bytes.Buffer
	logger := NewLogger(config.AppConfig{
		Name:        "bpm-api",
		Environment: config.Development,
		Version:     "test",
		LogLevel:    "INFO",
	}, &output)

	logger.Info("PostgreSQL connected successfully", "status", "ready")
	line := output.String()
	plainLine := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(line, "")

	if !strings.Contains(line, "\x1b[") {
		t.Fatalf("expected development log to contain terminal colors, got %q", line)
	}
	for _, expected := range []string{"PostgreSQL connected successfully", "status=ready"} {
		if !strings.Contains(plainLine, expected) {
			t.Fatalf("expected development log to contain %q, got %q", expected, plainLine)
		}
	}
}

func TestProductionLoggerUsesJSONWithoutTerminalColors(t *testing.T) {
	var output bytes.Buffer
	logger := NewLogger(config.AppConfig{
		Name:        "bpm-api",
		Environment: config.Production,
		Version:     "test",
		LogLevel:    "INFO",
	}, &output)

	logger.Info("HTTP server is ready", slog.Int("port", 8080))
	line := output.String()
	if strings.Contains(line, "\x1b[") {
		t.Fatalf("production log must not contain terminal colors: %q", line)
	}

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("production log must be valid JSON: %v", err)
	}
	if record["msg"] != "HTTP server is ready" || record["port"] != float64(8080) {
		t.Fatalf("unexpected production log: %#v", record)
	}
}
