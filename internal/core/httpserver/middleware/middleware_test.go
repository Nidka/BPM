package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/Nidka/BPM-service/internal/core/httpserver/response"
)

type rejectingTokenVerifier struct{}

func (rejectingTokenVerifier) VerifyIDToken(context.Context, string) (contracts.Identity, error) {
	return contracts.Identity{}, context.Canceled
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(true, 31536000)(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	for _, header := range []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"Content-Security-Policy",
		"Strict-Transport-Security",
	} {
		if recorder.Header().Get(header) == "" {
			t.Fatalf("missing security header %s", header)
		}
	}
	if policy := recorder.Header().Get("Content-Security-Policy"); strings.Contains(policy, "unsafe-inline") {
		t.Fatalf("regular API routes must keep the restrictive CSP: %s", policy)
	}
}

func TestSecurityHeadersAllowEmbeddedSwaggerAssets(t *testing.T) {
	handler := SecurityHeaders(true, 31536000)(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/docs/", nil))

	policy := recorder.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"script-src 'self' 'unsafe-inline'", "connect-src 'self'", "frame-ancestors 'none'"} {
		if !strings.Contains(policy, directive) {
			t.Fatalf("Swagger CSP is missing %q: %s", directive, policy)
		}
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	handler := RequestID(CORS([]string{"https://app.example.com"})(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://attacker.example")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
}

func TestAuthenticateUsesSanitizedArrayError(t *testing.T) {
	handler := RequestID(Authenticate(rejectingTokenVerifier{})(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})))
	request := httptest.NewRequest(http.MethodGet, "/private", nil)
	request.Header.Set("Authorization", "Bearer invalid-token")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
	var body response.ErrorBody
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Errors) != 1 || body.Errors[0].Code != "UNAUTHENTICATED" || body.RequestID == "" {
		t.Fatalf("unexpected authentication response: %+v", body)
	}
}

func TestAuthenticateReportsUnavailableVerifier(t *testing.T) {
	handler := RequestID(Authenticate(nil)(http.NotFoundHandler()))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/private", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body response.ErrorBody
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Errors) != 1 || body.Errors[0].Code != "AUTHENTICATION_UNAVAILABLE" {
		t.Fatalf("unexpected unavailable response: %+v", body)
	}
}
