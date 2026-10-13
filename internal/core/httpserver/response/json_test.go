package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestErrorUsesArrayContract(t *testing.T) {
	recorder := httptest.NewRecorder()
	Error(recorder, http.StatusForbidden, "request-123", APIError{
		Code:    "ACCESS_DENIED",
		Message: "No tienes permisos para realizar esta acción.",
	})

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
	var body ErrorBody
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.RequestID != "request-123" || len(body.Errors) != 1 {
		t.Fatalf("unexpected error response: %+v", body)
	}
	if body.Errors[0].Code != "ACCESS_DENIED" || body.Errors[0].Fields != nil {
		t.Fatalf("unexpected error detail: %+v", body.Errors[0])
	}
}

func TestValidationErrorIncludesInvalidFields(t *testing.T) {
	recorder := httptest.NewRecorder()
	ValidationError(recorder, "request-456", map[string]string{
		"email": "El correo no tiene un formato válido.",
	})

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", recorder.Code)
	}
	var body ErrorBody
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Errors) != 1 || body.Errors[0].Code != CodeValidationError {
		t.Fatalf("unexpected validation response: %+v", body)
	}
	if body.Errors[0].Fields["email"] == "" {
		t.Fatalf("expected email field error: %+v", body.Errors[0])
	}
}

func TestInternalErrorNeverIncludesInfrastructureDetails(t *testing.T) {
	recorder := httptest.NewRecorder()
	InternalError(recorder, "request-789")

	body := recorder.Body.String()
	if body == "" || strings.Contains(body, "postgres") || strings.Contains(body, "password") {
		t.Fatalf("unsafe internal error response: %s", body)
	}
}
