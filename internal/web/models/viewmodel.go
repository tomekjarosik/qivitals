package models

import (
	"encoding/json"
	"html/template"
)

// StatusBadgeView holds the data needed to display a status indicator.
type StatusBadgeView struct {
	State   string // "OK", "DEGRADED", "DEAD" or anything
	ShowDot bool
}

// LabelPillsView is a list of key:value labels.
type LabelPillsView struct {
	Labels map[string]string
}

// ReportedDataView displays key‑value telemetry.
type ReportedDataView struct {
	Data map[string]string
}

// ConditionRuleView represents a condition rule for editing in the Web UI.
type ConditionRuleView struct {
	Name            string
	Expression      string
	TargetState     string
	MessageTemplate string
}

type ConditionView struct {
	Type    string
	Status  string // "True", "False", "Unknown"
	Reason  string
	Message string
}

// ConditionsByName returns conditions indexed by their Type (which matches a rule's Name).
// Used by the sensor detail page to link each rule to its latest evaluation.
type ConditionsByName map[string]ConditionView

// SensorCardView is a single sensor card on the dashboard.
type SensorCardView struct {
	ID                    string
	Name                  string
	Description           string
	Status                StatusBadgeView
	BackgroundClass       string
	Labels                LabelPillsView
	GracefulPeriodSeconds int64
	FailurePeriodSeconds  int64
	ReportedData          ReportedDataView
	LastReported          int64
	LastSpecUpdated       int64
	ShowLabels            bool
	Conditions            []ConditionView
	ConditionsByRule      ConditionsByName
	ConditionRules        []ConditionRuleView
}

// NamespaceGroupView groups sensors under a namespace.
type NamespaceGroupView struct {
	Namespace string
	Sensors   []SensorCardView
}

// FilterView holds current filter selections.
type FilterView struct {
	Namespace              string
	Search                 string
	Name                   string
	Statuses               []string
	Labels                 []LabelEntry // key+value pairs for form rendering
	HasLabelKeys           string
	OrderBy                string
	OrderDesc              bool
	ShowLabelsOnSensorGrid bool
}

// LabelEntry is a single key‑value pair for the label filter UI.
type LabelEntry struct {
	Key, Value string
}

func (fv FilterView) LabelsJSON() string {
	type entry struct{ Key, Value string }
	entries := make([]entry, len(fv.Labels))
	for i, l := range fv.Labels {
		entries[i] = entry{l.Key, l.Value}
	}
	b, _ := json.Marshal(entries)
	return string(b)
}

// EmptyStateView is the data for the empty‑state template.
type EmptyStateView struct {
	Title       string
	Description string
	IconSVG     template.HTML
}

// SensorGridData bundles the groups and the fallback empty state.
type SensorGridData struct {
	Groups []NamespaceGroupView
	Empty  EmptyStateView
}

// DashboardPageView is the top‑level data for the dashboard page.
type DashboardPageView struct {
	Now           string
	FullURL       string
	SensorGrid    SensorGridData
	Filter        FilterView
	Authenticated bool
	Username      string
}

// SensorDetailPageView is the data for the /sensors/{id} page.
type SensorDetailPageView struct {
	Sensor SensorCardView
}

func NewDefaultEmptyState() EmptyStateView {
	return EmptyStateView{
		Title:       "No sensors found",
		Description: "Try adjusting your filters or register a new sensor.",
		IconSVG: template.HTML(`<svg class="mx-auto h-16 w-16 text-slate-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
			      d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
		</svg>`),
	}
}
