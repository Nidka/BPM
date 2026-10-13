package identityhttp

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Routes struct {
	handler           *Handler
	authenticate      func(http.Handler) http.Handler
	registration      *RegisterHandler
	registrationLimit func(http.Handler) http.Handler
}

func NewRoutes(handler *Handler, authenticate func(http.Handler) http.Handler) *Routes {
	return &Routes{handler: handler, authenticate: authenticate}
}

func (routes *Routes) Register(router chi.Router) {
	router.With(routes.authenticate).Get("/me", routes.handler.Me)
	if routes.registration != nil {
		router.With(routes.registrationLimit, routes.authenticate).Post("/auth/register", routes.registration.Register)
	}
}

func (routes *Routes) WithRegistration(handler *RegisterHandler, limiter func(http.Handler) http.Handler) *Routes {
	routes.registration = handler
	routes.registrationLimit = limiter
	return routes
}
