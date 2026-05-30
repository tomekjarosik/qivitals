package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// TestWorkflow_MonthlyBills tests a manual sensor registration and data reporting.
func TestWorkflow_MonthlyBills(t *testing.T) {
	serverCmd := startTestServer(t)
	defer stopTestServer(t, serverCmd)

	// Setup: Register sensor
	stdout := runCLI(t, "register --namespace home --name water-bill --description 'Monthly water utility bill' --graceful 30d --failure 35d --label category=bills")
	sensorID := strings.TrimSpace(stdout)
	require.NotEmpty(t, sensorID, "Register command should return sensor ID")

	// Action: Report data
	runCLI(t, fmt.Sprintf("report --id %s --data paid_amount=45.50 --data method=bank_transfer", sensorID))

	// Verify: Query and check state
	queryOut := runCLI(t, "query --namespace home --name water-bill")

	var resp v1.QuerySensorsResponse
	err := protojson.Unmarshal([]byte(queryOut), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Sensors, 1)

	sn := resp.Sensors[0]

	assert.Equal(t, "OK", sn.Status.State)
	assert.Equal(t, "45.50", sn.Status.ReportedData["paid_amount"])
	assert.Equal(t, "bank_transfer", sn.Status.ReportedData["method"])
	assert.Equal(t, "Monthly water utility bill", sn.Metadata.Description)
	assert.Equal(t, "water-bill", sn.Metadata.Name)
	assert.Equal(t, "home", sn.Metadata.Namespace)
	assert.Equal(t, "bills", sn.Metadata.Labels["category"])

	// Verify durations (30 days = 2592000s, 35 days = 3024000s)
	assert.Equal(t, int64(2592000), sn.Spec.GracefulPeriodSeconds)
	assert.Equal(t, int64(3024000), sn.Spec.FailurePeriodSeconds)
}

func TestWorkflow_ITInfrastructure(t *testing.T) {
	serverCmd := startTestServer(t)
	defer stopTestServer(t, serverCmd)

	// 1. Register Sensors
	tlsID := strings.TrimSpace(runCLI(t, "register --namespace infra --name tls-jarosik-online --description 'TLS cert for main domain' --graceful 60d --failure 90d"))
	require.NotEmpty(t, tlsID)

	backupID := strings.TrimSpace(runCLI(t, "register --namespace infra --name backup-proxmox-nextcloud --description 'Daily Nextcloud VM backup' --graceful 25h --failure 48h"))
	require.NotEmpty(t, backupID)

	vmPingID := strings.TrimSpace(runCLI(t, "register --namespace infra --name ping-nextcloud-vm --description 'Nextcloud internal health endpoint' --graceful 300s --failure 900s"))
	require.NotEmpty(t, vmPingID)

	// 2. Simulate Cron Jobs
	runCLI(t, fmt.Sprintf("report --id %s --data days_remaining=65", tlsID))
	runCLI(t, fmt.Sprintf("report --id %s --data size_gb=14.2 --data duration_sec=450", backupID))
	runCLI(t, fmt.Sprintf("report --id %s --data latency_ms=4", vmPingID))

	// 3. Verifications
	queryOut := runCLI(t, "query --namespace infra")
	var resp v1.QuerySensorsResponse
	err := protojson.Unmarshal([]byte(queryOut), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Sensors, 3)

	// Verify TLS Sensor
	tlsSensor := findSensorByID(resp.Sensors, tlsID)
	require.NotNil(t, tlsSensor)
	assert.Equal(t, "OK", tlsSensor.Status.State)
	assert.Equal(t, "65", tlsSensor.Status.ReportedData["days_remaining"])
	assert.Equal(t, "infra", tlsSensor.Metadata.Namespace)
	assert.Equal(t, "tls-jarosik-online", tlsSensor.Metadata.Name)

	// Verify Backup Sensor
	backupSensor := findSensorByID(resp.Sensors, backupID)
	require.NotNil(t, backupSensor)
	assert.Equal(t, "OK", backupSensor.Status.State)
	assert.Equal(t, "14.2", backupSensor.Status.ReportedData["size_gb"])
	assert.Equal(t, "450", backupSensor.Status.ReportedData["duration_sec"])

	// Verify Ping Sensor
	vmPingSensor := findSensorByID(resp.Sensors, vmPingID)
	require.NotNil(t, vmPingSensor)
	assert.Equal(t, "OK", vmPingSensor.Status.State)
	assert.Equal(t, "4", vmPingSensor.Status.ReportedData["latency_ms"])
}

func TestWorkflow_HomeNetwork(t *testing.T) {
	serverCmd := startTestServer(t)
	defer stopTestServer(t, serverCmd)

	// Register sensor
	id := strings.TrimSpace(runCLI(t, "register --namespace network --name isp-connection --description 'Internet connectivity check' --graceful 120s --failure 300s --label location=home"))
	require.NotEmpty(t, id)

	// Report data
	runCLI(t, fmt.Sprintf("report --id %s --data packet_loss=0%%", id))

	// Verify
	queryOut := runCLI(t, "query --namespace network --name isp-connection")
	var resp v1.QuerySensorsResponse
	err := protojson.Unmarshal([]byte(queryOut), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Sensors, 1)

	sn := resp.Sensors[0]
	assert.Equal(t, "OK", sn.Status.State)
	assert.Equal(t, "0%", sn.Status.ReportedData["packet_loss"])
	assert.Equal(t, "network", sn.Metadata.Namespace)
	assert.Equal(t, "isp-connection", sn.Metadata.Name)
	assert.Equal(t, "home", sn.Metadata.Labels["location"])
	assert.Equal(t, int64(120), sn.Spec.GracefulPeriodSeconds)
	assert.Equal(t, int64(300), sn.Spec.FailurePeriodSeconds)
}

func TestWorkflow_FamilyChores(t *testing.T) {
	serverCmd := startTestServer(t)
	defer stopTestServer(t, serverCmd)

	// Register sensors
	dogID := strings.TrimSpace(runCLI(t, "register --namespace chores --name feed-dog --description 'Feed the dog' --graceful 14h --failure 24h"))
	require.NotEmpty(t, dogID)

	plantsID := strings.TrimSpace(runCLI(t, "register --namespace chores --name water-plants --description 'Water living room plants' --graceful 3d --failure 5d"))
	require.NotEmpty(t, plantsID)

	// Report data
	runCLI(t, fmt.Sprintf("report --id %s --data feeder=timmy", dogID))
	runCLI(t, fmt.Sprintf("report --id %s", plantsID))

	// Verify
	queryOut := runCLI(t, "query --namespace chores")
	var resp v1.QuerySensorsResponse
	err := protojson.Unmarshal([]byte(queryOut), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Sensors, 2)

	dogSensor := findSensorByID(resp.Sensors, dogID)
	require.NotNil(t, dogSensor)
	assert.Equal(t, "OK", dogSensor.Status.State)
	assert.Equal(t, "timmy", dogSensor.Status.ReportedData["feeder"])

	plantsSensor := findSensorByID(resp.Sensors, plantsID)
	require.NotNil(t, plantsSensor)
	assert.Equal(t, "OK", plantsSensor.Status.State)
}

func TestWorkflow_TemporaryProject(t *testing.T) {
	serverCmd := startTestServer(t)
	defer stopTestServer(t, serverCmd)

	// Register sensor
	id := strings.TrimSpace(runCLI(t, "register --namespace temp --name build-job-123 --description 'Short-lived build job' --graceful 1h --failure 2h"))
	require.NotEmpty(t, id)

	// Report data
	runCLI(t, fmt.Sprintf("report --id %s --data progress=50%%", id))

	// Delete sensor
	out := runCLI(t, fmt.Sprintf("delete --id %s", id))
	require.Contains(t, out, "deleted successfully", "Delete command should succeed")

	// Verify it's gone
	queryOut := runCLI(t, "query --namespace temp")
	var resp v1.QuerySensorsResponse
	err := protojson.Unmarshal([]byte(queryOut), &resp)
	require.NoError(t, err)
	assert.Empty(t, resp.Sensors, "Sensor should be deleted")
}

func TestWorkflow_AdvancedQueryFiltering(t *testing.T) {
	serverCmd := startTestServer(t)
	defer stopTestServer(t, serverCmd)

	// Register helpers
	reg := func(ns, name string) {
		out := runCLI(t, fmt.Sprintf("register --namespace %s --name %s --graceful 1h --failure 2h", ns, name))
		require.NotEmpty(t, strings.TrimSpace(out))
	}
	reg("infra", "db-backup-us-east")
	reg("infra", "db-backup-eu-west")
	reg("app", "api-health")
	reg("app", "api-staging-health")
	reg("chores", "clean-backup-drives")

	// Helper to get names from output
	getNames := func(out string) []string {
		var resp v1.QuerySensorsResponse
		protojson.Unmarshal([]byte(out), &resp)
		var names []string
		for _, s := range resp.Sensors {
			names = append(names, s.Metadata.Name)
		}
		return names
	}

	// Case A: Free-text search
	names := getNames(runCLI(t, "query --search backup"))
	assert.Contains(t, names, "db-backup-us-east")
	assert.Contains(t, names, "db-backup-eu-west")
	assert.Contains(t, names, "clean-backup-drives")

	// Case B: Search + Namespace
	names = getNames(runCLI(t, "query --search backup --namespace infra"))
	assert.Len(t, names, 2)
	assert.Contains(t, names, "db-backup-us-east")
	assert.NotContains(t, names, "clean-backup-drives")
}

func TestWorkflow_Update_Success(t *testing.T) {
	serverCmd := startTestServer(t)
	defer stopTestServer(t, serverCmd)

	// 1. Register baseline
	sensorID := strings.TrimSpace(runCLI(t, "register --namespace staging --name web-api --description 'Primary API server' --graceful 1h --failure 2h --label env=prod --label tier=backend"))
	require.NotEmpty(t, sensorID)

	// 2. Update
	out := runCLI(t, fmt.Sprintf("update --id %s --description 'Updated API Description' --graceful 1800s", sensorID))
	require.Contains(t, out, "updated successfully")

	// 3. Verify
	queryOut := runCLI(t, "query --namespace staging --name web-api")
	var resp v1.QuerySensorsResponse
	err := protojson.Unmarshal([]byte(queryOut), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Sensors, 1)

	updatedSensor := resp.Sensors[0]
	assert.Equal(t, "Updated API Description", updatedSensor.Metadata.Description)
	assert.Equal(t, int64(1800), updatedSensor.Spec.GracefulPeriodSeconds)
	assert.Equal(t, "prod", updatedSensor.Metadata.Labels["env"])

	// Verify unchanged fields
	assert.Equal(t, "staging", updatedSensor.Metadata.Namespace)
	assert.Equal(t, "web-api", updatedSensor.Metadata.Name)
	assert.Equal(t, "backend", updatedSensor.Metadata.Labels["tier"])
}

// findSensorByID is a helper to find a sensor in a list by ID
func findSensorByID(sensors []*v1.Sensor, id string) *v1.Sensor {
	for _, s := range sensors {
		if s.Metadata.Id == id {
			return s
		}
	}
	return nil
}

// getNamesFromResponse is a helper to extract sensor names from a query response.
func getNamesFromResponse(resp *v1.QuerySensorsResponse) []string {
	var names []string
	for _, s := range resp.Sensors {
		names = append(names, s.Metadata.Name)
	}
	return names
}
