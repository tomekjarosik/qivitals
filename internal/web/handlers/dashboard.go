package handlers

import (
	"bytes"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"github.com/tomekjarosik/qivitals/internal/web"
	"github.com/tomekjarosik/qivitals/internal/web/models"
	"github.com/tomekjarosik/qivitals/internal/web/models/components"
	"github.com/tomekjarosik/qivitals/internal/web/models/pages"
)

type DashboardHandler struct {
	renderer web.Renderer
	client   v1.QiVitalsServiceClient
}

func NewDashboardHandler(renderer web.Renderer, client v1.QiVitalsServiceClient) *DashboardHandler {
	return &DashboardHandler{renderer: renderer, client: client}
}

func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	statuses := q["statuses"]
	showLabels := q.Get("filters") == "1"

	// Build label entries
	var labels []models.LabelEntry
	for key, vals := range q {
		if strings.HasPrefix(key, "labels[") && strings.HasSuffix(key, "]") {
			k := key[7 : len(key)-1]
			if k != "" && len(vals) > 0 {
				labels = append(labels, models.LabelEntry{Key: k, Value: vals[0]})
			}
		}
	}

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
		Labels:       toMap(labels),
		HasLabelKeys: hasLabelKeys,
		OrderBy:      q.Get("order_by"),
		OrderDesc:    orderDesc,
	}

	resp, err := h.client.QuerySensors(r.Context(), req)
	if err != nil {
		http.Error(w, "Failed to load sensors", http.StatusInternalServerError)
		return
	}

	// Convert proto → view models
	groupsMap := map[string][]models.SensorCardView{}
	for _, s := range resp.Sensors {
		ns := s.Metadata.Namespace
		card := sensorToCardView(s)
		card.ShowLabels = showLabels
		groupsMap[ns] = append(groupsMap[ns], card)
	}

	var groups []models.NamespaceGroupView
	for ns, cards := range groupsMap {
		sort.Slice(cards, func(i, j int) bool { return cards[i].Name < cards[j].Name })
		groups = append(groups, models.NamespaceGroupView{
			Namespace: ns,
			Sensors:   cards,
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Namespace < groups[j].Namespace })

	filter := models.FilterView{
		Namespace:              q.Get("namespace"),
		Search:                 q.Get("search"),
		Name:                   q.Get("name"),
		Statuses:               statuses,
		Labels:                 labels,
		HasLabelKeys:           hasLabelKeysStr,
		OrderBy:                q.Get("order_by"),
		OrderDesc:              orderDesc,
		ShowLabelsOnSensorGrid: showLabels,
	}

	empty := models.NewDefaultEmptyState()
	sensorGridData := models.SensorGridData{
		Groups: groups,
		Empty:  empty,
	}

	pageData := models.DashboardPageView{
		Now:        time.Now().Format("2006-01-02 15:04:05"),
		FullURL:    r.URL.RequestURI(),
		SensorGrid: sensorGridData,
		Filter:     filter,
	}

	isHTMX := r.Header.Get("HX-Request") == "true"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var buf bytes.Buffer
	if isHTMX {
		gridComp := components.NewSensorGrid(pageData.SensorGrid, h.renderer)
		if err := gridComp.Render(r.Context(), &buf); err != nil {
			log.Printf("Render error: %v", err)
			http.Error(w, "Render error", http.StatusInternalServerError)
			return
		}
	} else {
		comp := pages.NewDashboardPage(pageData, h.renderer)
		if err := comp.Render(r.Context(), &buf); err != nil {
			log.Printf("Render error: %v", err)
			http.Error(w, "Render error", http.StatusInternalServerError)
			return
		}
	}

	buf.WriteTo(w)
}

func sensorToCardView(s *v1.Sensor) models.SensorCardView {
	bgClass := "bg-slate-50/90" // fallback
	switch s.Status.State {
	case "OK":
		bgClass = "bg-emerald-50/90"
	case "DEGRADED":
		bgClass = "bg-amber-50/90"
	case "DEAD":
		bgClass = "bg-rose-50/90"
	}
	var rules []models.ConditionRuleView

	for _, r := range s.Spec.Rules {
		rules = append(rules, models.ConditionRuleView{
			Name:            r.Name,
			Expression:      r.Expression,
			TargetState:     r.TargetState,
			MessageTemplate: r.MessageTemplate,
		})
	}
	var conditions []models.ConditionView
	conditionsByRule := make(models.ConditionsByName)
	for _, c := range s.Status.Conditions {
		cv := models.ConditionView{
			Type:    c.Type,
			Status:  c.Status,
			Reason:  c.Reason,
			Message: c.Message,
		}
		conditions = append(conditions, cv)
		conditionsByRule[c.Type] = cv
	}

	return models.SensorCardView{
		ID:                    s.Metadata.Id,
		Name:                  s.Metadata.Name,
		Description:           s.Metadata.Description,
		Status:                models.StatusBadgeView{State: s.Status.State, ShowDot: true},
		BackgroundClass:       bgClass,
		Labels:                models.LabelPillsView{Labels: s.Metadata.Labels},
		GracefulPeriodSeconds: s.Spec.GracefulPeriodSeconds,
		FailurePeriodSeconds:  s.Spec.FailurePeriodSeconds,
		ReportedData:          models.ReportedDataView{Data: s.Status.ReportedData},
		LastReported:          s.Status.LastReportedTimestamp,
		LastSpecUpdated:       s.Status.LastSpecUpdatedTimestamp,
		ConditionRules:        rules,
		Conditions:            conditions,
		ConditionsByRule:      conditionsByRule,
	}
}

func toMap(entries []models.LabelEntry) map[string]string {
	m := make(map[string]string, len(entries))
	for _, e := range entries {
		m[e.Key] = e.Value
	}
	return m
}
