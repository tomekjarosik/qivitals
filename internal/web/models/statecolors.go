package models

import v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"

// StateBackground returns the Tailwind background class for a sensor state.
func StateBackground(state v1.SensorState) string {
	switch state {
	case v1.SensorState_OK:
		return "bg-emerald-50/90"
	case v1.SensorState_DEGRADED:
		return "bg-amber-50/90"
	case v1.SensorState_FAILED:
		return "bg-rose-50/90"
	case v1.SensorState_PAUSED:
		return "bg-blue-50/9"
	default:
		return "bg-zinc-50/90"
	}
}

// StateDotClass returns the Tailwind classes for a state indicator dot.
func StateDotClass(state string) string {
	switch state {
	case "OK":
		return "bg-emerald-500 ring-emerald-100"
	case "DEGRADED":
		return "bg-amber-500 ring-amber-100"
	default:
		return "bg-rose-500 ring-rose-100"
	}
}

// ConditionDotClass returns classes for a condition evaluation dot.
func ConditionDotClass(status string) string {
	switch status {
	case "True":
		return "bg-rose-500 ring-rose-200"
	case "False":
		return "bg-emerald-500 ring-emerald-200"
	default:
		return "bg-slate-400 ring-slate-200"
	}
}

// ConditionPillClass returns classes for a condition status pill.
func ConditionPillClass(status string) string {
	switch status {
	case "True":
		return "bg-rose-100 text-rose-700"
	case "False":
		return "bg-emerald-100 text-emerald-700"
	default:
		return "bg-slate-200 text-slate-600"
	}
}
