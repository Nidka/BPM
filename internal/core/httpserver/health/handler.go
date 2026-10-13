package health

import (
	"context"
	"net/http"
	"time"

	"github.com/Nidka/BPM-service/internal/core/httpserver/middleware"
	"github.com/Nidka/BPM-service/internal/core/httpserver/response"
)

type Pinger interface {
	Ping(context.Context) error
}

type Handler struct {
	database Pinger
	version  string
}

func NewHandler(database Pinger, version string) *Handler {
	return &Handler{database: database, version: version}
}

func (handler *Handler) Live(writer http.ResponseWriter, _ *http.Request) {
	response.JSON(writer, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": handler.version,
	})
}

func (handler *Handler) Ready(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()

	if err := handler.database.Ping(ctx); err != nil {
		response.Error(writer, http.StatusServiceUnavailable, middleware.RequestIDFromContext(request.Context()), response.APIError{
			Code:    "NOT_READY",
			Message: "Las dependencias del servicio no están disponibles.",
		})
		return
	}
	response.JSON(writer, http.StatusOK, map[string]string{"status": "ready"})
}
