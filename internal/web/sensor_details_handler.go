package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
	"github.com/tomekjarosik/one-status/internal/view"
	"github.com/tomekjarosik/one-status/internal/view/components"
)

type SensorDetailsHandler struct {
	svc v1.StatusServiceServer
}

func NewSensorDetailsHandler(svc v1.StatusServiceServer) *SensorDetailsHandler {
	return &SensorDetailsHandler{svc: svc}
}

func (h *SensorDetailsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r, id)
	case http.MethodPost:
		h.handlePost(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *SensorDetailsHandler) handleGet(w http.ResponseWriter, r *http.Request, id string) {
	resp, err := h.svc.QuerySensors(r.Context(), &v1.QuerySensorsRequest{Id: id})
	if err != nil || len(resp.Sensors) == 0 {
		http.Error(w, "Sensor not found", http.StatusNotFound)
		return
	}
	card := sensorToCardView(resp.Sensors[0])
	page := components.NewSensorDetailPage(view.SensorDetailPageView{Sensor: card})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.Render(r.Context(), w); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *SensorDetailsHandler) handlePost(w http.ResponseWriter, r *http.Request, id string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var ops []*v1.PatchOperation

	// Metadata
	newName := r.FormValue("name")
	nameJSON, _ := json.Marshal(newName)
	ops = append(ops, &v1.PatchOperation{Op: "replace", Path: "/metadata/name", Value: string(nameJSON)})

	descJSON, _ := json.Marshal(r.FormValue("description"))
	ops = append(ops, &v1.PatchOperation{Op: "replace", Path: "/metadata/description", Value: string(descJSON)})

	// Labels
	keys := r.Form["label_key"]
	vals := r.Form["label_value"]
	newLabels := make(map[string]string)
	for i, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		v := ""
		if i < len(vals) {
			v = strings.TrimSpace(vals[i])
		}
		newLabels[k] = v
	}
	labelsJSON, _ := json.Marshal(newLabels)
	ops = append(ops, &v1.PatchOperation{Op: "replace", Path: "/metadata/labels", Value: string(labelsJSON)})

	// Spec
	gracefulStr := r.FormValue("graceful_period_seconds")
	gracefulVal := int64(0)
	if s, err := strconv.ParseInt(gracefulStr, 10, 64); err == nil {
		gracefulVal = s
	}
	failureStr := r.FormValue("failure_period_seconds")
	failureVal := int64(0)
	if s, err := strconv.ParseInt(failureStr, 10, 64); err == nil {
		failureVal = s
	}

	gracefulJSON, _ := json.Marshal(gracefulVal)
	ops = append(ops, &v1.PatchOperation{Op: "replace", Path: "/spec/graceful_period_seconds", Value: string(gracefulJSON)})
	failureJSON, _ := json.Marshal(failureVal)
	ops = append(ops, &v1.PatchOperation{Op: "replace", Path: "/spec/failure_period_seconds", Value: string(failureJSON)})

	// Resource version
	resp, _ := h.svc.QuerySensors(r.Context(), &v1.QuerySensorsRequest{Id: id})
	if len(resp.Sensors) == 0 {
		http.Error(w, "Sensor disappeared", http.StatusNotFound)
		return
	}
	version := resp.Sensors[0].Metadata.ResourceVersion

	_, err := h.svc.PatchSensor(r.Context(), &v1.PatchSensorRequest{
		Id:         id,
		Version:    version,
		Operations: ops,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Update failed: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
}
