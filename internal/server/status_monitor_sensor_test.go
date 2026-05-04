package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
	"github.com/tomekjarosik/one-status/internal/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRegisterSensor_New(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

	sensor := &v1.Sensor{
		Metadata: &v1.ObjectMeta{
			Id:          "sensor-1",
			Name:        "Sensor One",
			Description: "First sensor",
			Labels: map[string]string{
				"env":    "production",
				"region": "us-east",
			},
		},
		Spec: &v1.SensorSpec{
			GracefulPeriodSeconds: 60,
			FailurePeriodSeconds:  120,
		},
	}

	req := &v1.RegisterSensorRequest{Sensor: sensor}
	resp, err := impl.RegisterSensor(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp.Sensor)
	assert.Equal(t, "sensor-1", resp.Sensor.Metadata.Id)
}

func TestRegisterSensor_EmptySensorID(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

	req := &v1.RegisterSensorRequest{
		Sensor: &v1.Sensor{
			Metadata: &v1.ObjectMeta{
				Id: "",
			},
			Spec: &v1.SensorSpec{},
		},
	}

	resp, err := impl.RegisterSensor(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp.Sensor)
	assert.NotEqual(t, "", resp.Sensor.Metadata.Id)
}

func TestSendSensorData(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

	// Register first
	sensor := &v1.Sensor{
		Metadata: &v1.ObjectMeta{
			Id:   "sensor-2",
			Name: "Sensor Two",
		},
		Spec: &v1.SensorSpec{
			GracefulPeriodSeconds: 60,
			FailurePeriodSeconds:  120,
		},
	}

	req := &v1.RegisterSensorRequest{Sensor: sensor}
	_, err := impl.RegisterSensor(context.Background(), req)
	assert.NoError(t, err)

	// Send OK data
	sendReq := &v1.ReportSensorRequest{
		Id:   "sensor-2",
		Data: map[string]string{},
	}
	sendResp, err := impl.ReportSensor(context.Background(), sendReq)
	assert.NoError(t, err)
	assert.NotNil(t, sendResp.Sensor)
	assert.NotZero(t, sendResp.Sensor.Status.LastUpdatedTimestamp)

	// Send failure data
	sendReq2 := &v1.ReportSensorRequest{
		Id:   "sensor-2",
		Data: map[string]string{},
	}
	sendResp2, err := impl.ReportSensor(context.Background(), sendReq2)
	assert.NoError(t, err)
	assert.NotNil(t, sendResp2.Sensor)
	assert.Equal(t, "sensor-2", sendResp2.Sensor.Metadata.Id)
}

func TestSendSensorData_NonExistent(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

	req := &v1.ReportSensorRequest{
		Id:   "non-existent",
		Data: map[string]string{},
	}
	resp, err := impl.ReportSensor(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestQuerySensors(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

	// Register multiple sensors
	for i := 1; i <= 5; i++ {
		sensorId := "sensor-" + string(rune('0'+i))
		sensor := &v1.Sensor{
			Metadata: &v1.ObjectMeta{
				Id:   sensorId,
				Name: "Sensor " + string(rune('0'+i)),
				Labels: map[string]string{
					"type":     "sensor",
					"instance": "instance-" + string(rune('0'+i)),
				},
			},
			Spec: &v1.SensorSpec{
				GracefulPeriodSeconds: 60,
				FailurePeriodSeconds:  120,
			},
		}
		req := &v1.RegisterSensorRequest{Sensor: sensor}
		_, err := impl.RegisterSensor(context.Background(), req)
		assert.NoError(t, err)

		// Send OK data
		sendReq := &v1.ReportSensorRequest{
			Id:   sensorId,
			Data: map[string]string{},
		}
		_, err = impl.ReportSensor(context.Background(), sendReq)
		assert.NoError(t, err)
	}

	// Query all sensors
	queryReq := &v1.QuerySensorsRequest{}
	queryResp, err := impl.QuerySensors(context.Background(), queryReq)
	assert.NoError(t, err)

	assert.Greater(t, len(queryResp.Sensors), 0)
	for _, sensor := range queryResp.Sensors {
		assert.Equal(t, "OK", sensor.Status.State)
		assert.NotZero(t, sensor.Status.LastUpdatedTimestamp)
	}

	// Query all ACTIVE sensors (status filter)
	queryReqActive := &v1.QuerySensorsRequest{
		Statuses: []string{"OK"},
	}
	queryRespActive, err := impl.QuerySensors(context.Background(), queryReqActive)
	assert.NoError(t, err)

	if len(queryResp.Sensors) > 0 {
		assert.GreaterOrEqual(t, len(queryRespActive.Sensors), 1)
		for _, sensor := range queryRespActive.Sensors {
			assert.Equal(t, "OK", sensor.Status.State)
		}
	}
}

func TestQuerySensors_ById(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

	// Register sensors
	sensors := []*v1.Sensor{
		{
			Metadata: &v1.ObjectMeta{
				Id:   "prefix-a-sensor-1",
				Name: "prefix-a-sensor",
			},
			Spec: &v1.SensorSpec{GracefulPeriodSeconds: 60},
		},
		{
			Metadata: &v1.ObjectMeta{
				Id:   "prefix-b-sensor-2",
				Name: "prefix-b-sensor",
			},
			Spec: &v1.SensorSpec{GracefulPeriodSeconds: 60},
		},
		{
			Metadata: &v1.ObjectMeta{
				Id:   "other-sensor-3",
				Name: "other-sensor",
			},
			Spec: &v1.SensorSpec{GracefulPeriodSeconds: 60},
		},
	}

	for _, sensor := range sensors {
		req := &v1.RegisterSensorRequest{Sensor: sensor}
		_, err := impl.RegisterSensor(context.Background(), req)
		assert.NoError(t, err)

		sendReq := &v1.ReportSensorRequest{
			Id:   sensor.Metadata.Id,
			Data: map[string]string{},
		}
		_, err = impl.ReportSensor(context.Background(), sendReq)
		assert.NoError(t, err)
	}

	// Query by exact sensor ID
	queryReqExact := &v1.QuerySensorsRequest{
		Id: "prefix-a-sensor-1",
	}
	queryRespExact, err := impl.QuerySensors(context.Background(), queryReqExact)
	assert.NoError(t, err)

	assert.Greater(t, len(queryRespExact.Sensors), 0)
	filteredCount := 0
	for _, sensor := range queryRespExact.Sensors {
		if sensor.Metadata.Id == "prefix-a-sensor-1" {
			filteredCount++
		}
	}
	assert.Equal(t, 1, filteredCount)
}

func TestQuerySensors_ByLabels(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

	// Register sensors with different labels
	sensors := []*v1.Sensor{
		{
			Metadata: &v1.ObjectMeta{
				Id:   "sensor-1",
				Name: "Sensor One",
				Labels: map[string]string{
					"app":    "web",
					"region": "us-east",
				},
			},
			Spec: &v1.SensorSpec{GracefulPeriodSeconds: 60},
		},
		{
			Metadata: &v1.ObjectMeta{
				Id:   "sensor-2",
				Name: "Sensor Two",
				Labels: map[string]string{
					"app":    "api",
					"region": "us-west",
				},
			},
			Spec: &v1.SensorSpec{GracefulPeriodSeconds: 60},
		},
	}

	for _, sensor := range sensors {
		req := &v1.RegisterSensorRequest{Sensor: sensor}
		_, err := impl.RegisterSensor(context.Background(), req)
		assert.NoError(t, err)

		sendReq := &v1.ReportSensorRequest{
			Id:   sensor.Metadata.Id,
			Data: map[string]string{},
		}
		_, err = impl.ReportSensor(context.Background(), sendReq)
		assert.NoError(t, err)
	}

	// Query by single label
	queryReq := &v1.QuerySensorsRequest{
		Labels: map[string]string{"app": "web"},
	}
	queryResp, err := impl.QuerySensors(context.Background(), queryReq)
	assert.NoError(t, err)

	filteredCount := 0
	for _, sensor := range queryResp.Sensors {
		if sensor.Metadata.Id == "sensor-1" {
			filteredCount++
			assert.Equal(t, "OK", sensor.Status.State)
		}
	}
	assert.Greater(t, filteredCount, 0)

	// Query by multiple labels (AND logic)
	queryReqMultiple := &v1.QuerySensorsRequest{
		Labels: map[string]string{
			"region": "us-east",
			"app":    "web",
		},
	}
	queryRespMultiple, err := impl.QuerySensors(context.Background(), queryReqMultiple)
	assert.NoError(t, err)

	filteredMultiple := 0
	for _, sensor := range queryRespMultiple.Sensors {
		if sensor.Metadata.Id == "sensor-1" { // Only sensor-1 matches both labels
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
			expectedStatus: "OK",
		},
		{
			name:           "Active - within graceful period",
			age:            30,
			gracefulPeriod: 60,
			failurePeriod:  120,
			expectedStatus: "OK",
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
				LastUpdated: lastOk,
			}

			status := calculateSensorStatus(state)
			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

func TestRegisterSensor_InvalidPeriods(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)

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
			sensor := &v1.Sensor{
				Metadata: &v1.ObjectMeta{
					Id:   "",
					Name: "Test Sensor - " + tt.name, // <-- Make name unique per test case
				},
				Spec: &v1.SensorSpec{
					GracefulPeriodSeconds: tt.graceful,
					FailurePeriodSeconds:  tt.failure,
				},
			}

			req := &v1.RegisterSensorRequest{Sensor: sensor}
			resp, err := impl.RegisterSensor(context.Background(), req)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestPatchSensor(t *testing.T) {
	strg := storage.NewMemorySensorStorage()
	impl := NewStatusMonitorService(strg)
	ctx := context.Background()

	// Initial Seed Data
	initialSensor := &v1.Sensor{
		Metadata: &v1.ObjectMeta{
			Id:          "test-sensor",
			Name:        "Original Name",
			Namespace:   "default",
			Description: "Original Description",
			Labels:      map[string]string{"env": "prod", "version": "1"},
		},
		Spec: &v1.SensorSpec{
			GracefulPeriodSeconds: 60,
			FailurePeriodSeconds:  120,
		},
	}
	_, err := impl.RegisterSensor(ctx, &v1.RegisterSensorRequest{Sensor: initialSensor})
	require.NoError(t, err)

	tests := []struct {
		name           string
		request        *v1.PatchSensorRequest
		expectedError  codes.Code
		verifyRelation func(t *testing.T, updated *v1.Sensor) // Callback for deep verification
	}{
		{
			name: "Success - Replace existing label",
			request: &v1.PatchSensorRequest{
				Id: "test-sensor",
				Operations: []*v1.PatchOperation{
					{Op: "replace", Path: "/metadata/labels/env", Value: `"staging"`},
				},
			},
			expectedError: codes.OK,
			verifyRelation: func(t *testing.T, updated *v1.Sensor) {
				assert.Equal(t, "staging", updated.Metadata.Labels["env"])
				assert.Equal(t, "1", updated.Metadata.Labels["version"], "Unrelated labels should persist")
			},
		},
		{
			name: "Success - Add new label",
			request: &v1.PatchSensorRequest{
				Id: "test-sensor",
				Operations: []*v1.PatchOperation{
					{Op: "add", Path: "/metadata/labels/region", Value: `"us-east-1"`},
				},
			},
			expectedError: codes.OK,
			verifyRelation: func(t *testing.T, updated *v1.Sensor) {
				assert.Equal(t, "us-east-1", updated.Metadata.Labels["region"])
				assert.Equal(t, "staging", updated.Metadata.Labels["env"])
			},
		},
		{
			name: "Success - Patch spec integer (via JSON string)",
			request: &v1.PatchSensorRequest{
				Id: "test-sensor",
				Operations: []*v1.PatchOperation{
					{Op: "replace", Path: "/spec/graceful_period_seconds", Value: `300`},
				},
			},
			expectedError: codes.OK,
			verifyRelation: func(t *testing.T, updated *v1.Sensor) {
				assert.Equal(t, int64(300), updated.Spec.GracefulPeriodSeconds)
			},
		},
		{
			name: "Success - Patch spec integer (via JSON string)",
			request: &v1.PatchSensorRequest{
				Id: "test-sensor",
				Operations: []*v1.PatchOperation{
					{Op: "replace", Path: "/spec/failure_period_seconds", Value: `300`},
				},
			},
			expectedError: codes.OK,
			verifyRelation: func(t *testing.T, updated *v1.Sensor) {
				assert.Equal(t, int64(300), updated.Spec.FailurePeriodSeconds)
			},
		},
		{
			name: "Success - Remove a label",
			request: &v1.PatchSensorRequest{
				Id: "test-sensor",
				Operations: []*v1.PatchOperation{
					{Op: "remove", Path: "/metadata/labels/version"},
				},
			},
			expectedError: codes.OK,
			verifyRelation: func(t *testing.T, updated *v1.Sensor) {
				_, exists := updated.Metadata.Labels["version"]
				assert.False(t, exists)
			},
		},
		{
			name: "Failure - Missing ID",
			request: &v1.PatchSensorRequest{
				Id: "",
				Operations: []*v1.PatchOperation{
					{Op: "replace", Path: "/metadata/labels/env", Value: `"dev"`},
				},
			},
			expectedError: codes.InvalidArgument,
		},
		{
			name: "Failure - Unauthorized path (Immutable field)",
			request: &v1.PatchSensorRequest{
				Id: "test-sensor",
				Operations: []*v1.PatchOperation{
					{Op: "replace", Path: "/metadata/id", Value: `"New ID"`},
				},
			},
			expectedError: codes.InvalidArgument,
		},
		{
			name: "Failure - Non-existent sensor",
			request: &v1.PatchSensorRequest{
				Id: "ghost-sensor",
				Operations: []*v1.PatchOperation{
					{Op: "replace", Path: "/metadata/labels/env", Value: `"dev"`},
				},
			},
			expectedError: codes.NotFound,
		},
		{
			name: "Failure - Unsupported Operation",
			request: &v1.PatchSensorRequest{
				Id: "test-sensor",
				Operations: []*v1.PatchOperation{
					{Op: "move", Path: "/metadata/labels/env", Value: `"dev"`},
				},
			},
			expectedError: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := impl.QuerySensors(ctx, &v1.QuerySensorsRequest{Id: initialSensor.Metadata.Id})
			require.NoError(t, err)
			tt.request.Version = state.Sensors[0].Metadata.ResourceVersion

			resp, err := impl.PatchSensor(ctx, tt.request)

			if tt.expectedError != codes.OK {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedError, st.Code())
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.NotNil(t, resp.Sensor)

			// Verify the result matches the logic (Deep Verification)
			if tt.verifyRelation != nil {
				tt.verifyRelation(t, resp.Sensor)
			}

			// Critical Step: Verify Persistence (The "Map vs Territory" Check)
			// We fetch directly from storage to ensure the 'map' (the response)
			// matches the 'territory' (the actual database/storage).
			persistedState, err := strg.GetStatus(ctx, tt.request.Id)
			require.NoError(t, err)

			// We use the response's value as the ground truth for what was saved
			assert.Equal(t, resp.Sensor.Metadata.Name, persistedState.Info.Name)
			assert.Equal(t, resp.Sensor.Metadata.Namespace, persistedState.Info.Namespace)
			assert.Equal(t, resp.Sensor.Metadata.Labels, persistedState.Info.Labels)
			assert.Equal(t, resp.Sensor.Spec.GracefulPeriodSeconds, persistedState.Info.GracefulPeriod)
			assert.Equal(t, resp.Sensor.Spec.FailurePeriodSeconds, persistedState.Info.FailurePeriod)
		})
	}
}
