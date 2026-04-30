package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
	"github.com/tomekjarosik/one-status/internal/storage"
)

func TestServiceImplImplementsInterface(t *testing.T) {
	var _ v1.StatusServiceServer = &StatusMonitorService{}
	assert.True(t, true, "StatusMonitorService implements StatusServiceServer")
}

func TestRegisterSensor_Duplicate(t *testing.T) {
	storage := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(storage)

	sensor1 := &v1.SensorInfo{
		SensorId:              "sensor-1",
		SensorName:            "Sensor One",
		Description:           "First sensor",
		GracefulPeriodSeconds: 60,
		FailurePeriodSeconds:  120,
	}

	req1 := &v1.RegisterSensorRequest{Sensor: sensor1}
	resp1, err := impl.RegisterSensor(context.Background(), req1)

	assert.NoError(t, err)
	assert.True(t, resp1.Success)
	assert.Equal(t, "sensor-1", resp1.SensorId)

	req2 := &v1.RegisterSensorRequest{Sensor: sensor1}
	resp2, err := impl.RegisterSensor(context.Background(), req2)

	assert.Error(t, err)
	assert.False(t, resp2.Success)
	assert.Equal(t, "sensor-1", resp2.SensorId)
}
