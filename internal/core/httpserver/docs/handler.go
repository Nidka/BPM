// Package docs serves the versioned OpenAPI contract and its embedded UI.
package docs

import (
	_ "embed"
	"net/http"

	swaggerui "github.com/swaggest/swgui/v5emb"
)

const (
	UIPath   = "/docs/"
	SpecPath = "/openapi.yaml"
)

//go:embed openapi.yaml
var openAPISpec []byte

type Handler struct {
	ui http.Handler
}

func NewHandler() *Handler {
	return &Handler{
		ui: swaggerui.New("BPM API", SpecPath, UIPath),
	}
}

func (handler *Handler) Spec(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(openAPISpec)
}

func (handler *Handler) UI() http.Handler {
	return handler.ui
}
