package identityhttp

import (
	"context"
	"encoding/json"
	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/Nidka/BPM-service/internal/core/httpserver/middleware"
	"github.com/Nidka/BPM-service/internal/core/httpserver/response"
	"github.com/Nidka/BPM-service/internal/features/identity/domain"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type Registrar interface {
	Execute(context.Context, contracts.Identity, string) (domain.Profile, error)
}
type RegisterHandler struct {
	registrar Registrar
	logger    *slog.Logger
}

func NewRegisterHandler(registrar Registrar, logger *slog.Logger) *RegisterHandler {
	return &RegisterHandler{registrar: registrar, logger: logger}
}

func (h *RegisterHandler) Register(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.RequestIDFromContext(r.Context())
	identity, ok := middleware.IdentityFromContext(r.Context())
	if !ok {
		response.Unauthenticated(w, requestID)
		return
	}
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		response.Error(w, http.StatusUnsupportedMediaType, requestID, response.APIError{Code: "UNSUPPORTED_MEDIA_TYPE", Message: "Envía una solicitud JSON."})
		return
	}
	var input struct {
		DisplayName string `json:"displayName"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		response.Error(w, http.StatusBadRequest, requestID, response.APIError{Code: "INVALID_REQUEST", Message: "La solicitud no es válida."})
		return
	}
	profile, err := h.registrar.Execute(r.Context(), identity, input.DisplayName)
	if err != nil {
		writeIdentityError(w, r, err, h.logger)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": newProfileResponse(profile)})
}
