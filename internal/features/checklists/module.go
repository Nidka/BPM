package checklists

import (
	"log/slog"

	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/Nidka/BPM-service/internal/core/httpserver/middleware"
	httpadapter "github.com/Nidka/BPM-service/internal/features/checklists/adapters/http"
	"github.com/Nidka/BPM-service/internal/features/checklists/adapters/postgres"
	"github.com/Nidka/BPM-service/internal/features/checklists/application"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Routes(pool *pgxpool.Pool, verifier contracts.TokenVerifier, logger *slog.Logger, sessions ...contracts.SessionValidator) func(chi.Router) {
	repository := postgres.New(pool)
	h := httpadapter.New(application.New(repository), application.NewAssignments(repository), logger)
	return func(r chi.Router) { h.Register(r, middleware.Authenticate(verifier, sessions...)) }
}
