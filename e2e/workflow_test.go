package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
)

// --- Real-World Workflows ---

func TestWorkflow_MonthlyBills(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	// 1. Setup: A sensor for a monthly water bill (graceful: 30 days, failure: 35 days)
	days30 := int64(30 * 24 * 60 * 60)
	days35 := int64(35 * 24 * 60 * 60)
	waterBillID := Register(t, "home", "water-bill", "Monthly water utility bill", days30, days35, "category=bills", "type=manual")

	// Action: Human clicks "I paid this" (sends a report)
	Report(t, waterBillID, "paid_amount=45.50", "method=bank_transfer")

	// Verify: Bill is marked as ACTIVE (paid)
	RequireState(t, waterBillID, "OK")

	// Verify details
	res := Query(t, "--namespace", "home", "--name", "water-bill")
	assert.Equal(t, "45.50", res.Sensors[0].Status.ReportedData["paid_amount"])
}

func TestWorkflow_ITInfrastructure(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	// 1. Register TLS Certificate monitor (graceful: 60 days, failure: 90 days)
	tlsID := Register(t, "infra", "tls-jarosik-online", "TLS cert for main domain", 60*24*3600, 90*24*3600)

	// 2. Register Backup Monitor (graceful: 25 hours, failure: 48 hours)
	backupID := Register(t, "infra", "backup-proxmox-nextcloud", "Daily Nextcloud VM backup", 25*3600, 48*3600)

	// 3. Register VM Alive Ping (graceful: 5 mins, failure: 15 mins)
	vmPingID := Register(t, "infra", "ping-nextcloud-vm", "Nextcloud internal health endpoint", 300, 900)

	// --- Simulate automated Cron Jobs running ---

	// Cert bot runs and reports 65 days remaining
	Report(t, tlsID, "days_remaining=65")

	// Backup script finishes successfully
	Report(t, backupID, "size_gb=14.2", "duration_sec=450")

	// Uptime kuma / ping script runs
	Report(t, vmPingID, "latency_ms=4")

	// --- Verifications ---
	RequireState(t, tlsID, "OK")
	RequireState(t, backupID, "OK")
	RequireState(t, vmPingID, "OK")
}

func TestWorkflow_HomeNetwork(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	// Router pings 8.8.8.8 every minute. Graceful=2 mins, Failure=5 mins
	internetID := Register(t, "network", "isp-connection", "Internet connectivity check", 120, 300, "location=home")

	// Router sends OK
	Report(t, internetID, "packet_loss=0%")
	RequireState(t, internetID, "OK")
}

func TestWorkflow_FamilyChores(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	// Dog needs feeding twice a day (grace: 14 hours)
	dogID := Register(t, "chores", "feed-dog", "Feed the dog", 14*3600, 24*3600)

	// Plants need water every 3 days
	plantsID := Register(t, "chores", "water-plants", "Water living room plants", 3*24*3600, 5*24*3600)

	// Kid presses NFC button next to dog bowl
	Report(t, dogID, "feeder=timmy")
	RequireState(t, dogID, "OK")

	// No one watered plants yet, but we just registered it, so it might technically be DEAD depending on initialization logic.
	// We simulate a report to make it active.
	Report(t, plantsID)
	RequireState(t, plantsID, "OK")
}

func TestWorkflow_TemporaryProject(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	// 1. Create a sensor for a temporary build job
	jobID := Register(t, "temp", "build-job-123", "Short-lived build job", 3600, 7200)

	// 2. Job is active
	Report(t, jobID, "progress=50%")
	RequireState(t, jobID, "OK")

	// 3. Job finishes, we delete the sensor
	Delete(t, jobID)

	// 4. Verify it's gone
	res := Query(t, "--namespace", "temp")
	assert.Len(t, res.Sensors, 0, "Sensor should be deleted")
}

func TestWorkflow_AdvancedQueryFiltering(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	// 1. Setup a diverse set of sensors across different namespaces and names
	Register(t, "infra", "db-backup-us-east", "Postgres backup", 3600, 7200, "env=prod", "team=data", "critical=true")
	Register(t, "infra", "db-backup-eu-west", "Postgres backup EU", 3600, 7200, "env=prod", "team=data", "critical=true")
	Register(t, "app", "api-health", "API healthcheck", 60, 120, "env=prod", "team=backend")
	Register(t, "app", "api-staging-health", "Staging API ping", 60, 120, "env=staging", "team=backend")
	Register(t, "chores", "clean-backup-drives", "Manual cleanup of old tapes", 86400, 172800, "manual=true")

	// Helper to extract names from a query response
	getNames := func(resp *v1.QuerySensorsResponse) []string {
		var names []string
		for _, s := range resp.Sensors {
			names = append(names, s.Metadata.Name)
		}
		return names
	}

	// Case A: Free-text search across namespaces
	// Should find both DB backups and the manual chore because they all contain "backup" in name or desc
	resSearch := Query(t, "--search", "backup")
	assert.Len(t, resSearch.Sensors, 3)
	names := getNames(resSearch)
	assert.Contains(t, names, "db-backup-us-east")
	assert.Contains(t, names, "db-backup-eu-west")
	assert.Contains(t, names, "clean-backup-drives")

	// Case B: Search + Namespace filter
	// Should only find the infra backups, ignoring the chore
	resSearchNS := Query(t, "--search", "backup", "--namespace", "infra")
	assert.Len(t, resSearchNS.Sensors, 2)
	assert.NotContains(t, getNames(resSearchNS), "clean-backup-drives")

	// Case C: Exact Label matching
	// Find all prod sensors
	resProd := Query(t, "--label", "env=prod")
	assert.Len(t, resProd.Sensors, 3) // both DBs + api-health

	// Case D: Multiple Exact Labels (AND logic)
	// Find prod sensors owned by backend team
	resProdBackend := Query(t, "--label", "env=prod", "--label", "team=backend")
	assert.Len(t, resProdBackend.Sensors, 1)
	assert.Equal(t, "api-health", resProdBackend.Sensors[0].Metadata.Name)

	// Case E: Has-Label (Key existence only)
	// Find any sensor that has the "critical" label, regardless of its value
	resCritical := Query(t, "--has-label", "critical")
	assert.Len(t, resCritical.Sensors, 2)
	names = getNames(resCritical)
	assert.Contains(t, names, "db-backup-us-east")
	assert.Contains(t, names, "db-backup-eu-west")
}

// TestWorkflow_Update_Success verifies that passing a sequence of valid flags
// to the 'update' command correctly modifies the sensor in the database.
func TestWorkflow_Update_Success(t *testing.T) {
	// 1. Start the server (This helper is part of your e2e package)
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	// 2. Setup: Register a baseline sensor to act as our target
	// We use the Register helper to create a sensor with predictable initial values.
	// Using a natural key (namespace/name) to make the test more human-readable.
	initialNamespace := "production"
	initialName := "web-api"
	initialDesc := "Primary API server"
	initialGraceful := int64(3600) // 1 hour
	initialFailure := int64(7200)  // 2 hours

	sensorID := Register(t, initialNamespace, initialName, initialDesc, initialGraceful, initialFailure, "env=prod", "tier=backend")

	updateArgs := []string{
		"update",
		"--id", sensorID,
		"--desc", "Updated API Description",
		"--graceful", "1800",
	}

	// We execute the CLI tool as a separate process
	out, stderr, err := runCLI(t, updateArgs...)
	require.NoError(t, err, "CLI Patch command failed. Stderr: %s", stderr)
	require.Contains(t, out, "updated successfully", "CLI did not report success")

	queryArgs := []string{"query", "--namespace", initialNamespace, "--name", initialName}
	resp := Query(t, queryArgs...)

	require.Len(t, resp.Sensors, 1, "Sensor should still exist in the namespace")
	updatedSensor := resp.Sensors[0]

	// Assert all updated fields
	assert.Equal(t, "Updated API Description", updatedSensor.Metadata.Description)
	assert.Equal(t, int64(1800), updatedSensor.Spec.GracefulPeriodSeconds)

	// Assert that unmentioned fields remained unchanged (The "Regression" check)
	assert.Equal(t, initialName, updatedSensor.Metadata.Name)
	assert.Equal(t, initialNamespace, updatedSensor.Metadata.Namespace)
	assert.Equal(t, "prod", updatedSensor.Metadata.Labels["env"])
}

func TestWorkflow_Update_AllFlagsSeparately(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	// Define a struct to represent a single flag-based test case
	type updateTestCase struct {
		name   string
		args   []string                         // The flags to pass to the CLI
		verify func(t *testing.T, s *v1.Sensor) // How to verify the change
	}

	// Define the baseline sensor
	initialNamespace := "production"
	initialName := "web-api"
	initialDesc := "Original Description"
	initialGraceful := int64(3600)
	initialFailure := int64(7200)

	// Register the baseline
	sensorID := Register(t, initialNamespace, initialName, initialDesc, initialGraceful, initialFailure, "env=prod", "tier=backend")

	tests := []updateTestCase{
		{
			name: "Patch Description",
			args: []string{"update", "--id", sensorID, "--desc", "New Description"},
			verify: func(t *testing.T, s *v1.Sensor) {
				assert.Equal(t, "New Description", s.Metadata.Description)
			},
		},
		{
			name: "Rename Sensor",
			args: []string{"update", "--id", sensorID, "--new-name", "new-web-api"},
			verify: func(t *testing.T, s *v1.Sensor) {
				assert.Equal(t, "new-web-api", s.Metadata.Name)
			},
		},
		{
			name: "Rename Namespace",
			args: []string{"update", "--id", sensorID, "--new-namespace", "ns_x"},
			verify: func(t *testing.T, s *v1.Sensor) {
				assert.Equal(t, "ns_x", s.Metadata.Namespace)
			},
		},
		{
			name: "Patch Graceful Period",
			args: []string{"update", "--id", sensorID, "--graceful", "1800"},
			verify: func(t *testing.T, s *v1.Sensor) {
				assert.Equal(t, int64(1800), s.Spec.GracefulPeriodSeconds)
			},
		},
		{
			name: "Patch Failure Period",
			args: []string{"update", "--id", sensorID, "--failure", "5000"},
			verify: func(t *testing.T, s *v1.Sensor) {
				assert.Equal(t, int64(5000), s.Spec.FailurePeriodSeconds)
			},
		},
		{
			name: "Rename Sensor",
			args: []string{"update", "--id", sensorID, "--new-name", "new-web-api"},
			verify: func(t *testing.T, s *v1.Sensor) {
				assert.Equal(t, "new-web-api", s.Metadata.Name)
			},
		},
		{
			name: "Move Namespace",
			args: []string{"update", "--id", sensorID, "--new-namespace", "staging"},
			verify: func(t *testing.T, s *v1.Sensor) {
				assert.Equal(t, "staging", s.Metadata.Namespace)
			},
		},
		{
			name: "Add Label",
			args: []string{"update", "--id", sensorID, "--label", "owner=alice"},
			verify: func(t *testing.T, s *v1.Sensor) {
				assert.Equal(t, "alice", s.Metadata.Labels["owner"])
				assert.Equal(t, "prod", s.Metadata.Labels["env"]) // Ensure old labels persist
			},
		},
		{
			name: "Remove Label",
			args: []string{"update", "--id", sensorID, "--remove-label", "tier"},
			verify: func(t *testing.T, s *v1.Sensor) {
				_, exists := s.Metadata.Labels["tier"]
				assert.False(t, exists)
				assert.Equal(t, "prod", s.Metadata.Labels["env"]) // Ensure others remain
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, stderr, err := runCLI(t, tt.args...)
			require.NoError(t, err, "CLI error for %s: %s", tt.name, stderr)
			require.Contains(t, out, "updated successfully")

			queryArgs := []string{"query", "--id", sensorID}
			resp := Query(t, queryArgs...)
			require.Len(t, resp.Sensors, 1)
			updatedSensor := resp.Sensors[0]

			tt.verify(t, updatedSensor)
		})
	}
}
