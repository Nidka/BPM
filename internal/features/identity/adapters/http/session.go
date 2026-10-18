package identityhttp

import (
	"context"
	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/Nidka/BPM-service/internal/core/httpserver/middleware"
	"github.com/Nidka/BPM-service/internal/core/httpserver/response"
	"net/http"
)

type SessionEnder interface {
	End(context.Context, contracts.Identity) error
}

func SessionActivity(w http.ResponseWriter, r *http.Request) {
	// Authenticate ya comprobó la sesión y registró su actividad en PostgreSQL.
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]string{"status": "ACTIVE"}})
}

func SessionLogout(sessions SessionEnder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, ok := middleware.IdentityFromContext(r.Context())
		if !ok {
			response.Unauthenticated(w, middleware.RequestIDFromContext(r.Context()))
			return
		}
		if err := sessions.End(r.Context(), identity); err != nil {
			response.AuthenticationUnavailable(w, middleware.RequestIDFromContext(r.Context()))
			return
		}
		response.JSON(w, http.StatusOK, map[string]any{"data": map[string]string{"status": "SIGNED_OUT"}})
	}
}
