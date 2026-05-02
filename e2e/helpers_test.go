package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const e2eGrpcPort = "localhost:50099" // Use a specific port to avoid dev conflicts

// startTestServer runs the compiled server binary in the background
func startTestServer(t *testing.T) *exec.Cmd {
	cmd := exec.Command(serverBin, "serve", "--grpc-port", e2eGrpcPort)

	err := cmd.Start()
	require.NoError(t, err, "Failed to start test server")

	// Wait for port to open
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", e2eGrpcPort, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		return false
	}, 3*time.Second, 100*time.Millisecond, "Server failed to start in time")

	return cmd
}

// runCLI executes the compiled CLI binary with the given arguments
func runCLI(t *testing.T, args ...string) (stdout string, stderr string, err error) {
	cmd := exec.Command(cliBin, args...)

	// Start with the host's environment, but inject our specific SENSORCLI_URL
	// so the CLI knows to talk to the test server instead of the default port.
	cmd.Env = append(cmd.Environ(), "SENSORCLI_URL="+e2eGrpcPort)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// --- Human-Readable CLI Helpers ---

func Register(t *testing.T, namespace, name, desc string, graceful, failure int64, labels ...string) string {
	args := []string{
		"register",
		"--namespace", namespace,
		"--name", name,
		"--desc", desc,
		"--graceful", fmt.Sprint(graceful),
		"--failure", fmt.Sprint(failure),
	}
	for _, l := range labels {
		args = append(args, "--label", l)
	}

	out, stderr, err := runCLI(t, args...)
	require.NoError(t, err, "Failed to register %s/%s. Err: %s", namespace, name, stderr)
	require.Contains(t, out, "Sensor registered successfully")

	// Query it immediately to get the generated ID
	resp := Query(t, "--namespace", namespace, "--name", "name")
	return resp.Sensors[0].Spec.Id
}

func Report(t *testing.T, id string, data ...string) {
	args := []string{"report", "--id", id}
	for _, d := range data {
		args = append(args, "--data", d)
	}
	_, stderr, err := runCLI(t, args...)
	require.NoError(t, err, "Failed to report data for %s. Err: %s", id, stderr)
}

func Query(t *testing.T, args ...string) E2EQueryResponse {
	queryArgs := append([]string{"query", "-m"}, args...)
	out, stderr, err := runCLI(t, queryArgs...)
	require.NoError(t, err, "Query failed. Err: %s", stderr)

	var response E2EQueryResponse
	err = json.Unmarshal([]byte(out), &response)
	require.NoError(t, err, "Failed to parse JSON output")
	return response
}

func RequireState(t *testing.T, id, expectedState string) {
	resp := Query(t) // Query all
	for _, s := range resp.Sensors {
		if s.Spec.Id == id {
			assert.Equal(t, expectedState, s.Status.State, "Sensor state mismatch")
			return
		}
	}
	t.Fatalf("Sensor %s not found during state check", id)
}

func Delete(t *testing.T, id string) {
	out, stderr, err := runCLI(t, "delete", "--id", id)
	require.NoError(t, err, "Failed to delete sensor %s. Err: %s", id, stderr)
	require.Contains(t, out, "deleted successfully")
}

// --- Domain Models for JSON Parsing ---
type E2ESensor struct {
	Spec struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"spec"`
	Status struct {
		State        string            `json:"state"`
		ReportedData map[string]string `json:"reportedData"`
	} `json:"status"`
}

type E2EQueryResponse struct {
	Sensors []E2ESensor `json:"sensors"`
}
