package response

import (
	"encoding/json"
	"net/http"
)

const (
	CodeInternalError             = "INTERNAL_ERROR"
	CodeValidationError           = "VALIDATION_ERROR"
	CodeUnauthenticated           = "UNAUTHENTICATED"
	CodeAuthenticationUnavailable = "AUTHENTICATION_UNAVAILABLE"
)

// APIError is a safe, stable error exposed to clients. It intentionally does
// not accept a Go error, which prevents SQL, provider or infrastructure details
// from being serialized accidentally.
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type ErrorBody struct {
	Errors    []APIError `json:"errors"`
	RequestID string     `json:"requestId,omitempty"`
}

func JSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}

func Error(writer http.ResponseWriter, status int, requestID string, apiErrors ...APIError) {
	if len(apiErrors) == 0 {
		apiErrors = []APIError{internalError()}
	}
	JSON(writer, status, ErrorBody{
		Errors:    apiErrors,
		RequestID: requestID,
	})
}

func InternalError(writer http.ResponseWriter, requestID string) {
	Error(writer, http.StatusInternalServerError, requestID, internalError())
}

func Unauthenticated(writer http.ResponseWriter, requestID string) {
	Error(writer, http.StatusUnauthorized, requestID, APIError{
		Code:    CodeUnauthenticated,
		Message: "Se requiere autenticación.",
	})
}

func AuthenticationUnavailable(writer http.ResponseWriter, requestID string) {
	Error(writer, http.StatusServiceUnavailable, requestID, APIError{
		Code:    CodeAuthenticationUnavailable,
		Message: "El servicio de autenticación no está disponible.",
	})
}

func ValidationError(writer http.ResponseWriter, requestID string, fields map[string]string) {
	Error(writer, http.StatusUnprocessableEntity, requestID, APIError{
		Code:    CodeValidationError,
		Message: "Uno o más campos son inválidos.",
		Fields:  fields,
	})
}

func internalError() APIError {
	return APIError{
		Code:    CodeInternalError,
		Message: "Ocurrió un error interno.",
	}
}
