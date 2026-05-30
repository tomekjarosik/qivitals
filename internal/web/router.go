package web

import (
	"html/template"
	"net/http"

	"github.com/tomekjarosik/qivitals/internal/auth"
)

// Router handles the assembly of the HTTP gateway and the Web UI.
type Router struct {
	Templates *template.Template
	Gateway   http.Handler
	Dashboard http.Handler
	Details   http.Handler
}

// NewRouter assembles the multiplexer.
func NewRouter(gateway, dashboard, details http.Handler) http.Handler {
	mux := http.NewServeMux()

	// API Gateway Route
	mux.Handle("/api/", gateway)

	// UI Routes
	mux.Handle("/sensors/{id}", details)
	mux.Handle("/", dashboard)

	return auth.InjectAuthContext(mux)
}
