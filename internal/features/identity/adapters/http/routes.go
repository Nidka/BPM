package identityhttp

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Routes struct {
	handler      *Handler
	authenticate func(http.Handler) http.Handler
}

func NewRoutes(handler *Handler, authenticate func(http.Handler) http.Handler) *Routes {
	return &Routes{handler: handler, authenticate: authenticate}
}

func (routes *Routes) Register(router chi.Router) {
	router.With(routes.authenticate).Get("/me", routes.handler.Me)
}
