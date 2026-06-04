package models

import v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"

func AvailableSensorStates() []string {
	return []string{
		v1.SensorState_UNKNOWN.String(),
		v1.SensorState_OK.String(),
		v1.SensorState_DEGRADED.String(),
		v1.SensorState_FAILED.String(),
		v1.SensorState_PAUSED.String(),
	}
}
