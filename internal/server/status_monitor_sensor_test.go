package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
	"github.com/tomekjarosik/one-status/internal/storage"
)

func TestRegisterSensor_New(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

	sensor := &v1.SensorInfo{
		SensorId:              "sensor-1",
		SensorName:            "Sensor One",
		Description:           "First sensor",
		GracefulPeriodSeconds: 60,
		FailurePeriodSeconds:  120,
		Labels: []*v1.Label{
			{Key: "env", Value: "production"},
			{Key: "region", Value: "us-east"},
		},
	}

	req := &v1.RegisterSensorRequest{Sensor: sensor}
	resp, err := impl.RegisterSensor(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "sensor-1", resp.SensorId)
	assert.NotZero(t, resp.Timestamp)
}

func TestRegisterSensor_EmptySensorID(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

	req := &v1.RegisterSensorRequest{
		Sensor: &v1.SensorInfo{
			SensorId: "",
		},
	}

	resp, err := impl.RegisterSensor(context.Background(), req)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotEqual(t, "", resp.SensorId)
}

func TestSendSensorData(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

	// Register first
	sensor := &v1.SensorInfo{
		SensorId:              "sensor-2",
		SensorName:            "Sensor Two",
		GracefulPeriodSeconds: 60,
		FailurePeriodSeconds:  120,
	}

	req := &v1.RegisterSensorRequest{Sensor: sensor}
	resp, err := impl.RegisterSensor(context.Background(), req)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	// Send OK data
	sendReq := &v1.ReportSensorRequest{
		SensorId: "sensor-2",
		Data:     map[string]string{},
	}
	sendResp, err := impl.ReportSensor(context.Background(), sendReq)
	assert.NoError(t, err)
	assert.True(t, sendResp.Success)
	assert.NotZero(t, sendResp.Timestamp)

	// Send failure data
	sendReq2 := &v1.ReportSensorRequest{
		SensorId: "sensor-2",
		Data:     map[string]string{},
	}
	sendResp2, err := impl.ReportSensor(context.Background(), sendReq2)
	assert.NoError(t, err)
	assert.True(t, sendResp2.Success)
	assert.Equal(t, "sensor-2", sendResp2.SensorId)

}

func TestSendSensorData_NonExistent(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

	req := &v1.ReportSensorRequest{
		SensorId: "non-existent",
		Data:     map[string]string{},
	}
	resp, err := impl.ReportSensor(context.Background(), req)

	assert.Error(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "non-existent", resp.SensorId)
}

func TestQuerySensors(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

	// Register multiple sensors
	for i := 1; i <= 5; i++ {
		sensorId := "sensor-" + string(rune('0'+i))
		sensor := &v1.SensorInfo{
			SensorId:              sensorId,
			SensorName:            "Sensor " + string(rune('0'+i)),
			GracefulPeriodSeconds: 60,
			FailurePeriodSeconds:  120,
			Labels: []*v1.Label{
				{Key: "type", Value: "sensor"},
				{Key: "instance", Value: "instance-" + string(rune('0'+i))},
			},
		}
		req := &v1.RegisterSensorRequest{Sensor: sensor}
		_, err := impl.RegisterSensor(context.Background(), req)
		assert.NoError(t, err)

		// Send OK data
		sendReq := &v1.ReportSensorRequest{
			SensorId: sensorId,
			Data:     map[string]string{},
		}
		_, err = impl.ReportSensor(context.Background(), sendReq)
		assert.NoError(t, err)
	}

	// Query all sensors
	queryReq := &v1.QuerySensorsRequest{}
	queryResp, err := impl.QuerySensors(context.Background(), queryReq)
	assert.NoError(t, err)

	assert.Greater(t, len(queryResp.Sensors), 0)
	for _, status := range queryResp.Sensors {
		assert.Equal(t, "ACTIVE", status.Status)
		assert.NotZero(t, status.LastOkTimestamp)
	}

	// Query all ACTIVE sensors (status filter)
	queryReqActive := &v1.QuerySensorsRequest{
		Status: "ACTIVE",
	}
	queryRespActive, err := impl.QuerySensors(context.Background(), queryReqActive)
	assert.NoError(t, err)

	if len(queryResp.Sensors) > 0 {
		assert.GreaterOrEqual(t, len(queryRespActive.Sensors), 1)
		for _, status := range queryRespActive.Sensors {
			assert.Equal(t, "ACTIVE", status.Status)
		}
	}
}

func TestQuerySensors_ByPath(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

	// Register sensors with different patterns
	sensors := []*v1.SensorInfo{
		{
			SensorId:              "prefix-a-sensor-1",
			SensorName:            "prefix-a-sensor",
			GracefulPeriodSeconds: 60,
		},
		{
			SensorId:              "prefix-b-sensor-2",
			SensorName:            "prefix-b-sensor",
			GracefulPeriodSeconds: 60,
		},
		{
			SensorId:              "other-sensor-3",
			SensorName:            "other-sensor",
			GracefulPeriodSeconds: 60,
		},
	}

	for _, sensor := range sensors {
		req := &v1.RegisterSensorRequest{Sensor: sensor}
		_, err := impl.RegisterSensor(context.Background(), req)
		assert.NoError(t, err)

		sendReq := &v1.ReportSensorRequest{
			SensorId: sensor.SensorId,
			Data:     map[string]string{},
		}
		_, err = impl.ReportSensor(context.Background(), sendReq)
		assert.NoError(t, err)
	}
	
	// Query by exact sensor ID
	queryReqExact := &v1.QuerySensorsRequest{
		Path: "prefix-a-sensor-1",
	}
	queryRespExact, err := impl.QuerySensors(context.Background(), queryReqExact)
	assert.NoError(t, err)

	assert.Greater(t, len(queryRespExact.Sensors), 0)
	filteredCount := 0
	for _, status := range queryRespExact.Sensors {
		if status.SensorId == "prefix-a-sensor-1" {
			filteredCount++
		}
	}
	assert.Equal(t, 1, filteredCount)

}

func TestQuerySensors_ByLabels(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

	// Register sensors with different labels
	sensors := []*v1.SensorInfo{
		{
			SensorId:              "sensor-1",
			SensorName:            "Sensor One",
			GracefulPeriodSeconds: 60,
			Labels: []*v1.Label{
				{Key: "app", Value: "web"},
				{Key: "region", Value: "us-east"},
			},
		},
		{
			SensorId:              "sensor-2",
			SensorName:            "Sensor Two",
			GracefulPeriodSeconds: 60,
			Labels: []*v1.Label{
				{Key: "app", Value: "api"},
				{Key: "region", Value: "us-west"},
			},
		},
	}

	for _, sensor := range sensors {
		req := &v1.RegisterSensorRequest{Sensor: sensor}
		_, err := impl.RegisterSensor(context.Background(), req)
		assert.NoError(t, err)

		sendReq := &v1.ReportSensorRequest{
			SensorId: sensor.SensorId,
			Data:     map[string]string{},
		}
		_, err = impl.ReportSensor(context.Background(), sendReq)
		assert.NoError(t, err)
	}

	// Query by single label
	queryReq := &v1.QuerySensorsRequest{
		Labels: []*v1.Label{
			{Key: "app", Value: "web"},
		},
	}
	queryResp, err := impl.QuerySensors(context.Background(), queryReq)
	assert.NoError(t, err)

	filteredCount := 0
	for _, status := range queryResp.Sensors {
		if status.SensorId == "sensor-1" {
			filteredCount++
			assert.Equal(t, "ACTIVE", status.Status)
		}
	}
	assert.Greater(t, filteredCount, 0)

	// Query by multiple labels (AND logic)
	queryReqMultiple := &v1.QuerySensorsRequest{
		Labels: []*v1.Label{
			{Key: "region", Value: "us-east"},
			{Key: "app", Value: "web"},
		},
	}
	queryRespMultiple, err := impl.QuerySensors(context.Background(), queryReqMultiple)
	assert.NoError(t, err)

	filteredMultiple := 0
	for _, status := range queryRespMultiple.Sensors {
		if status.SensorId == "sensor-1" { // Only sensor-1 matches both labels
			filteredMultiple++
		}
	}
	assert.Equal(t, 1, filteredMultiple)
}

func TestStatusCalculation(t *testing.T) {
	tests := []struct {
		name           string
		age            int64
		gracefulPeriod int64
		failurePeriod  int64
		expectedStatus string
	}{
		{
			name:           "Active - recent OK",
			age:            0,
			gracefulPeriod: 60,
			failurePeriod:  120,
			expectedStatus: "ACTIVE",
		},
		{
			name:           "Active - within graceful period",
			age:            30,
			gracefulPeriod: 60,
			failurePeriod:  120,
			expectedStatus: "ACTIVE",
		},
		{
			name:           "Degraded - within graceful period",
			age:            60,
			gracefulPeriod: 60,
			failurePeriod:  120,
			expectedStatus: "DEGRADED",
		},
		{
			name:           "Degraded - within failure period",
			age:            90,
			gracefulPeriod: 60,
			failurePeriod:  120,
			expectedStatus: "DEGRADED",
		},
		{
			name:           "Dead - expired graceful period",
			age:            150,
			gracefulPeriod: 60,
			failurePeriod:  120,
			expectedStatus: "DEAD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now().Unix()
			lastOk := now - tt.age

			state := &storage.SensorState{
				Info: &storage.SensorInfo{
					GracefulPeriod: tt.gracefulPeriod,
					FailurePeriod:  tt.failurePeriod,
				},
				LastOkTimestamp: lastOk,
				LastUpdated:     lastOk,
			}

			status := calculateSensorStatus(state)
			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

func TestRegisterSensor_InvalidPeriods(t *testing.T) {
	storage := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(storage)

	tests := []struct {
		name        string
		graceful    int64
		failure     int64
		shouldError bool
	}{
		{
			name:        "Valid periods",
			graceful:    60,
			failure:     120,
			shouldError: false,
		},
		{
			name:        "Failure before grace",
			graceful:    120,
			failure:     60,
			shouldError: false,
		},
		{
			name:        "Zero periods",
			graceful:    0,
			failure:     0,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sensor := &v1.SensorInfo{
				SensorId:              "", // each time new ID, like in Kubernetes objectId
				SensorName:            "Test Sensor",
				GracefulPeriodSeconds: tt.graceful,
				FailurePeriodSeconds:  tt.failure,
			}

			req := &v1.RegisterSensorRequest{Sensor: sensor}
			resp, err := impl.RegisterSensor(context.Background(), req)

			if tt.shouldError {
				assert.Error(t, err)
				assert.False(t, resp.Success)
			} else {
				assert.NoError(t, err)
				assert.True(t, resp.Success)
			}
		})
	}
}
