package web

import (
	"context"
	"embed"
	"html/template"
	"net/http"
	"time"

	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
	"github.com/tomekjarosik/one-status/internal/server"
)

//go:embed templates/*
var templateFS embed.FS

type DashboardHandler struct {
	svc  *server.StatusMonitorService
	tmpl *template.Template
}

func NewDashboardHandler(svc *server.StatusMonitorService) (*DashboardHandler, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/index.html")
	if err != nil {
		return nil, err
	}

	return &DashboardHandler{
		svc:  svc,
		tmpl: tmpl,
	}, nil
}

type TemplateData struct {
	Now     string
	Sensors []*v1.Sensor
	// Provide the current query params back to the template so the search box stays populated
	CurrentNamespace string
	CurrentStatus    string
}

func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle POST requests for "Poking" a sensor
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		sensorID := r.FormValue("poke_id")
		if sensorID != "" {
			// Call ReportSensor to reset the timestamp!
			_, _ = h.svc.ReportSensor(context.Background(), &v1.ReportSensorRequest{
				Id:      sensorID,
				Message: "Manual poke via Web UI",
			})
		}

		// Redirect back to the same page (preserving any query filters)
		http.Redirect(w, r, r.URL.String(), http.StatusFound)
		return
	}

	// Handle GET requests (Filtering via URL Query Params)
	q := r.URL.Query()
	namespaceFilter := q.Get("namespace")
	statusFilter := q.Get("status")

	req := &v1.QuerySensorsRequest{
		Namespace: namespaceFilter,
	}

	// If the user selected a specific status in the UI
	if statusFilter != "" && statusFilter != "ALL" {
		req.Statuses = []string{statusFilter}
	}

	resp, err := h.svc.QuerySensors(context.Background(), req)
	if err != nil {
		http.Error(w, "Failed to load sensors", http.StatusInternalServerError)
		return
	}

	data := TemplateData{
		Now:              time.Now().Format("2006-01-02 15:04:05"),
		Sensors:          resp.Sensors,
		CurrentNamespace: namespaceFilter,
		CurrentStatus:    statusFilter,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.Execute(w, data); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}
