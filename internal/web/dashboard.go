package web

import (
	"bytes"
	"context"
	"embed"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
	"github.com/tomekjarosik/one-status/internal/server"
)

//go:embed templates/**
var templateFS embed.FS

type DashboardHandler struct {
	svc  *server.StatusMonitorService
	tmpl *template.Template
}

func NewDashboardHandler(svc *server.StatusMonitorService) (*DashboardHandler, error) {
	tmpl := template.New("").Funcs(templateFuncs())
	tmpl, err := tmpl.ParseFS(templateFS,
		"templates/index.html",
		"templates/components/*.html",
	)
	if err != nil {
		return nil, err
	}

	return &DashboardHandler{
		svc:  svc,
		tmpl: tmpl,
	}, nil
}

type NamespaceGroup struct {
	Namespace string
	Sensors   []*v1.Sensor
}

type TemplateData struct {
	Now                 string
	FullURL             string
	NamespaceGroups     []NamespaceGroup
	CurrentNamespace    string
	CurrentSearch       string
	CurrentName         string
	CurrentStatuses     []string
	CurrentLabels       map[string]string
	CurrentHasLabelKeys string
	CurrentOrderBy      string
	CurrentOrderDesc    bool
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
	// Handle GET requests (Filtering via URL Query Params)
	// Multi‑statuses
	statuses := q["statuses"] // []string

	// Labels: parse labels[key]=val into a map
	labels := make(map[string]string)
	for key, vals := range q {
		if strings.HasPrefix(key, "labels[") && strings.HasSuffix(key, "]") {
			k := key[7 : len(key)-1]
			if len(vals) > 0 {
				labels[k] = vals[0]
			}
		}
	}

	// has_label_keys comma‑separated string
	hasLabelKeysStr := q.Get("has_label_keys")
	var hasLabelKeys []string
	if hasLabelKeysStr != "" {
		hasLabelKeys = strings.Split(hasLabelKeysStr, ",")
	}

	orderDesc, _ := strconv.ParseBool(q.Get("order_desc"))

	req := &v1.QuerySensorsRequest{
		Namespace:    q.Get("namespace"),
		Name:         q.Get("name"),
		Search:       q.Get("search"),
		Statuses:     statuses,
		Labels:       labels,
		HasLabelKeys: hasLabelKeys,
		OrderBy:      q.Get("order_by"),
		OrderDesc:    orderDesc,
		// Pagination can be added later if needed
	}
	resp, err := h.svc.QuerySensors(context.Background(), req)
	if err != nil {
		http.Error(w, "Failed to load sensors", http.StatusInternalServerError)
		return
	}

	groupsMap := make(map[string][]*v1.Sensor)
	for _, s := range resp.Sensors {
		ns := s.Metadata.Namespace
		groupsMap[ns] = append(groupsMap[ns], s)
	}

	var groups []NamespaceGroup
	for ns, sensors := range groupsMap {
		groups = append(groups, NamespaceGroup{
			Namespace: ns,
			Sensors:   sensors,
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Namespace < groups[j].Namespace
	})
	for i := range groups {
		sort.Slice(groups[i].Sensors, func(j, k int) bool {
			return groups[i].Sensors[j].Metadata.Name < groups[i].Sensors[k].Metadata.Name
		})
	}

	// Pass current filter values to template for pre‑filling
	data := TemplateData{
		Now:                 time.Now().Format("2006-01-02 15:04:05"),
		FullURL:             r.URL.RequestURI(),
		NamespaceGroups:     groups,
		CurrentNamespace:    q.Get("namespace"),
		CurrentSearch:       q.Get("search"),
		CurrentName:         q.Get("name"),
		CurrentStatuses:     statuses, // []string
		CurrentLabels:       labels,   // map[string]string (you may expose a slice instead)
		CurrentHasLabelKeys: hasLabelKeysStr,
		CurrentOrderBy:      q.Get("order_by"),
		CurrentOrderDesc:    orderDesc,
	}

	// Detect if this is an HTMX request
	isHTMX := r.Header.Get("HX-Request") == "true"

	var templateName string
	if isHTMX {
		// Only render the inner content
		templateName = "sensor-grid"
	} else {
		// Render the full page shell
		templateName = "main"
	}

	var buf bytes.Buffer
	log.Printf("Rendering template: %s", templateName)
	if err := h.tmpl.ExecuteTemplate(&buf, templateName, data); err != nil {
		log.Printf("Template error: %v", err)
		// Now it is safe to call http.Error because no headers have been sent yet
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}
