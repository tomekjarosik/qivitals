package e2e

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_RegisterAndQuery(t *testing.T) {
	serverCmd := startTestServer(t)
	defer func() {
		serverCmd.Process.Kill() // Ensure we clean up the process!
	}()

	out, stderr, err := runCLI(t, "register", "--namespace", "e2e", "--name", "db-check")
	require.NoError(t, err, "CLI register failed. Stderr: %s", stderr)
	require.Contains(t, out, "Sensor registered successfully")

	out, stderr, err = runCLI(t, "query", "--namespace", "e2e", "-m")
	require.NoError(t, err, "CLI query failed. Stderr: %s", stderr)

	var response struct {
		Sensors []struct {
			Spec struct {
				Name string `json:"name"`
			} `json:"spec"`
			Status struct {
				State string `json:"state"`
			} `json:"status"`
		} `json:"sensors"`
	}

	err = json.Unmarshal([]byte(out), &response)
	require.NoError(t, err, "Failed to parse CLI JSON output")

	require.Len(t, response.Sensors, 1)
	assert.Equal(t, "db-check", response.Sensors[0].Spec.Name)
	assert.Equal(t, "ACTIVE", response.Sensors[0].Status.State)
}
