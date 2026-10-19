package organizationhttp

import (
	"github.com/Nidka/BPM-service/internal/core/httpserver/middleware"
	"github.com/Nidka/BPM-service/internal/core/httpserver/response"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (h *Handler) checkMember(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.actor(w, r)
	if !ok {
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &input) {
		return
	}
	if r.URL.RawQuery != "" {
		response.Error(w, 400, middleware.RequestIDFromContext(r.Context()), response.APIError{Code: "INVALID_REQUEST", Message: "Envía únicamente el correo en el formulario."})
		return
	}
	status, err := h.service.CheckMember(r.Context(), uid, chi.URLParam(r, "organizationID"), input.Email)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": map[string]any{"status": status}})
}
