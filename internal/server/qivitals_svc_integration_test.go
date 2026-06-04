package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"github.com/tomekjarosik/qivitals/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestIntegration_EndToEndFlow(t *testing.T) {
	// 1. Setup real network listener on a random available port (localhost:0)
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err, "Failed to create listener")
	defer listener.Close()

	// 2. Start the gRPC server with our service implementation
	grpcServer := grpc.NewServer()
	impl := NewStatusMonitorService(storage.NewMemorySensorStorage())
	v1.RegisterQiVitalsServiceServer(grpcServer, impl)

	go func() {
		if err := grpcServer.Serve(listener); err != nil && err != grpc.ErrServerStopped {
			t.Logf("gRPC server error: %v", err)
		}
	}()
	defer grpcServer.GracefulStop()

	// Give the server a tiny moment to start
	time.Sleep(10 * time.Millisecond)

	// 3. Create a real gRPC client connected to the test server
	conn, err := grpc.NewClient(
		listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err, "Failed to connect to gRPC server")
	defer conn.Close()

	client := v1.NewQiVitalsServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// --- Step A: Register a new sensor ---
	regResp, err := client.RegisterSensor(ctx, &v1.RegisterSensorRequest{
		Sensor: &v1.Sensor{
			Metadata: &v1.ObjectMeta{
				Name:        "integration-test-job",
				Description: "Testing end-to-end flow",
				Labels: map[string]string{
					"env": "test",
				},
			},
			Spec: &v1.SensorSpec{
				GracefulPeriodSeconds: 300,
				FailurePeriodSeconds:  600,
			},
		},
	})
	require.NoError(t, err, "Failed to register sensor")
	require.NotNil(t, regResp.Sensor)

	sensorID := regResp.Sensor.Metadata.Id
	require.NotEmpty(t, sensorID, "Expected a generated sensor ID")

	// --- Step B: Report data for the sensor ---
	reportResp, err := client.ReportSensor(ctx, &v1.ReportSensorRequest{
		Id: sensorID,
		Data: map[string]string{
			"cpu_usage": "45%",
		},
	})
	require.NoError(t, err, "Failed to report sensor data")
	require.NotNil(t, reportResp.Sensor)

	// --- Step C: Query the sensor and verify status ---
	queryResp, err := client.QuerySensors(ctx, &v1.QuerySensorsRequest{
		Id: sensorID,
	})
	require.NoError(t, err, "Failed to query sensors")
	require.Len(t, queryResp.Sensors, 1, "Expected exactly 1 sensor in response")

	sensor := queryResp.Sensors[0]
	assert.Equal(t, sensorID, sensor.Metadata.Id)

	// Ensure the nested Status object exists
	require.NotNil(t, sensor.Status, "Expected Sensor to have a Status object")

	// Check the fields inside the nested Status object
	assert.Equal(t, v1.SensorState_OK, sensor.Status.State, "Sensor should be active after receiving a report")
	assert.Greater(t, sensor.Status.LastReportedTimestamp, int64(0), "Timestamp should be recorded")

	// Verify that the reported data was actually saved!
	assert.Equal(t, "45%", sensor.Status.ReportedData["cpu_usage"], "Reported data should be stored in status")
}
