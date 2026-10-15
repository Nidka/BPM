package request

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"

	"github.com/Nidka/BPM-service/internal/core/httpserver/middleware"
	"github.com/Nidka/BPM-service/internal/core/httpserver/response"
)

// VerifiedUID extracts an authenticated Firebase UID and applies the common
// email-verification policy used by business routes.
func VerifiedUID(w http.ResponseWriter, r *http.Request) (string, bool) {
	identity, ok := middleware.IdentityFromContext(r.Context())
	if !ok {
		response.Unauthenticated(w, middleware.RequestIDFromContext(r.Context()))
		return "", false
	}
	if !identity.EmailVerified {
		response.Error(w, http.StatusForbidden, middleware.RequestIDFromContext(r.Context()),
			response.APIError{Code: "EMAIL_NOT_VERIFIED", Message: "Verifica tu correo antes de continuar."})
		return "", false
	}
	return identity.UID, true
}

// DecodeJSON accepts one bounded JSON document and rejects unknown fields.
func DecodeJSON(w http.ResponseWriter, r *http.Request, target any, maxBytes int64) bool {
	id := middleware.RequestIDFromContext(r.Context())
	kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || kind != "application/json" {
		response.Error(w, http.StatusUnsupportedMediaType, id,
			response.APIError{Code: "UNSUPPORTED_MEDIA_TYPE", Message: "Envía una solicitud JSON."})
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	d.DisallowUnknownFields()
	if err = d.Decode(target); err == nil {
		var extra any
		if d.Decode(&extra) == io.EOF {
			return true
		}
	}
	response.Error(w, http.StatusBadRequest, id,
		response.APIError{Code: "INVALID_REQUEST", Message: "Revisa la estructura y el tamaño de la solicitud."})
	return false
}
