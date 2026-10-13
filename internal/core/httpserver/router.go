package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/Nidka/BPM-service/internal/core/config"
	"github.com/Nidka/BPM-service/internal/core/httpserver/docs"
	"github.com/Nidka/BPM-service/internal/core/httpserver/health"
	httpmiddleware "github.com/Nidka/BPM-service/internal/core/httpserver/middleware"
	"github.com/Nidka/BPM-service/internal/core/httpserver/response"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type APIRouteRegistrar func(chi.Router)

type Dependencies struct {
	Database  health.Pinger
	APIRoutes []APIRouteRegistrar
}

func NewRouter(cfg config.Config, logger *slog.Logger, dependencies Dependencies) http.Handler {
	router := chi.NewRouter()
	rateLimiter := httpmiddleware.NewIPRateLimiter(cfg.HTTP.RateLimitRPS, cfg.HTTP.RateLimitBurst, cfg.HTTP.TrustProxyHeaders)

	router.Use(httpmiddleware.RequestID)
	router.Use(httpmiddleware.Recover(logger))
	router.Use(httpmiddleware.SecurityHeaders(cfg.App.IsProductionLike(), cfg.HTTP.HSTSMaxAgeSeconds))
	router.Use(httpmiddleware.CORS(cfg.HTTP.AllowedOrigins))
	router.Use(httpmiddleware.BodyLimit(cfg.HTTP.MaxJSONBodyBytes))
	router.Use(rateLimiter.Middleware)
	router.Use(httpmiddleware.AccessLog(logger, cfg.HTTP.TrustProxyHeaders))
	router.Use(chimiddleware.Timeout(cfg.HTTP.RequestTimeout))

	healthHandler := health.NewHandler(dependencies.Database, cfg.App.Version)
	documentationHandler := docs.NewHandler()
	router.Get("/health", healthHandler.Live)
	router.Get("/ready", healthHandler.Ready)
	router.Get(docs.SpecPath, documentationHandler.Spec)
	router.Get("/docs", func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, docs.UIPath, http.StatusPermanentRedirect)
	})
	router.Mount(docs.UIPath, documentationHandler.UI())

	router.Route(cfg.HTTP.APIBasePath, func(api chi.Router) {
		api.Get("/", func(writer http.ResponseWriter, _ *http.Request) {
			response.JSON(writer, http.StatusOK, map[string]string{
				"name":    cfg.App.Name,
				"version": cfg.App.Version,
			})
		})

		for _, registerRoutes := range dependencies.APIRoutes {
			registerRoutes(api)
		}
	})

	router.NotFound(func(writer http.ResponseWriter, request *http.Request) {
		response.Error(writer, http.StatusNotFound, httpmiddleware.RequestIDFromContext(request.Context()), response.APIError{
			Code:    "NOT_FOUND",
			Message: "El recurso solicitado no existe.",
		})
	})
	router.MethodNotAllowed(func(writer http.ResponseWriter, request *http.Request) {
		response.Error(writer, http.StatusMethodNotAllowed, httpmiddleware.RequestIDFromContext(request.Context()), response.APIError{
			Code:    "METHOD_NOT_ALLOWED",
			Message: "El método HTTP no está permitido.",
		})
	})

	return router
}
