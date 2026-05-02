package e2e

import (
	"bytes"
	"net"
	"os/exec"
	"testing"
	"time"

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
