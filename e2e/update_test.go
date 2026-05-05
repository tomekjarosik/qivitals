package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
)

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

	// Register the baseline
	sensorID := Register(t, initialNamespace, initialName, initialDesc, "3600s", "7200s", "env=prod", "tier=backend")

	tests := []updateTestCase{
		{
			name: "Patch Description",
			args: []string{"update", "--id", sensorID, "--description", "New Description"},
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
			args: []string{"update", "--id", sensorID, "--graceful", "1800s"},
			verify: func(t *testing.T, s *v1.Sensor) {
				assert.Equal(t, int64(1800), s.Spec.GracefulPeriodSeconds)
			},
		},
		{
			name: "Patch Failure Period",
			args: []string{"update", "--id", sensorID, "--failure", "5000s"},
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
