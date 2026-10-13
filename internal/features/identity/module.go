package identity

import (
	"errors"
	"log/slog"

	"github.com/Nidka/BPM-service/internal/core/contracts"
	httpmiddleware "github.com/Nidka/BPM-service/internal/core/httpserver/middleware"
	identityhttp "github.com/Nidka/BPM-service/internal/features/identity/adapters/http"
	identitypostgres "github.com/Nidka/BPM-service/internal/features/identity/adapters/postgres"
	"github.com/Nidka/BPM-service/internal/features/identity/application"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dependencies struct {
	Database      *pgxpool.Pool
	TokenVerifier contracts.TokenVerifier
	Logger        *slog.Logger
}

type Module struct {
	routes *identityhttp.Routes
}

func New(dependencies Dependencies) (*Module, error) {
	if dependencies.Database == nil {
		return nil, errors.New("identity database is required")
	}
	if dependencies.Logger == nil {
		dependencies.Logger = slog.Default()
	}

	repository := identitypostgres.NewRepository(dependencies.Database)
	getCurrentProfile := application.NewGetCurrentProfile(repository)
	handler := identityhttp.NewHandler(getCurrentProfile, dependencies.Logger)
	registerHandler := identityhttp.NewRegisterHandler(application.NewRegister(repository), dependencies.Logger)
	registrationLimit := httpmiddleware.NewIPRateLimiter(0.2, 5, false)
	return &Module{
		routes: identityhttp.NewRoutes(handler, httpmiddleware.Authenticate(dependencies.TokenVerifier)).WithRegistration(registerHandler, registrationLimit.Middleware),
	}, nil
}

func (module *Module) RegisterRoutes(router chi.Router) {
	module.routes.Register(router)
}
