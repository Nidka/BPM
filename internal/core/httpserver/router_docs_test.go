package httpserver

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Nidka/BPM-service/internal/core/config"
)

type documentationPinger struct{}

func (documentationPinger) Ping(context.Context) error { return nil }

func TestRouterServesSwaggerUIAndOpenAPI(t *testing.T) {
	cfg := config.Config{
		App: config.AppConfig{Environment: config.Production, Version: "test"},
		HTTP: config.HTTPConfig{
			APIBasePath:      "/api/v1",
			MaxJSONBodyBytes: 1 << 20,
			RateLimitRPS:     10,
			RateLimitBurst:   10,
			RequestTimeout:   time.Second,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := NewRouter(cfg, logger, Dependencies{Database: documentationPinger{}})

	t.Run("swagger UI", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/docs/", nil))

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Header().Get("Content-Security-Policy"), "script-src 'self'") {
			t.Fatalf("Swagger route does not have its scoped CSP")
		}
	})

	t.Run("OpenAPI contract", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), "openapi: 3.0.3") {
			t.Fatalf("OpenAPI contract was not served")
		}
	})
}
