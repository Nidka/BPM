package docs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSpec(t *testing.T) {
	handler := NewHandler()
	recorder := httptest.NewRecorder()
	handler.Spec(recorder, httptest.NewRequest(http.MethodGet, SpecPath, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/yaml; charset=utf-8" {
		t.Fatalf("unexpected content type %q", contentType)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "openapi: 3.0.3") || !strings.Contains(body, "/api/v1/me:") {
		t.Fatalf("unexpected OpenAPI document: %s", body)
	}
}

func TestUI(t *testing.T) {
	handler := NewHandler().UI()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, UIPath, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "BPM API") || !strings.Contains(body, SpecPath) {
		t.Fatalf("Swagger UI is not configured with the BPM API contract")
	}
}
