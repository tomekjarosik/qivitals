package e2e

import (
	"bytes"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/shlex"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const serverAddress = "localhost:50099"

var (
	serverBin  = "./tmp/test-qivitals-bin"
	cliBin     = "./tmp/test-qivitals-cli-bin"
	tempDir    string
	configPath string
)

func runCmd(cmd *exec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Command failed: %s %v\nError: %v\nFull Output:\n%s", cmd.Path, cmd.Args, err, string(out))
		return err
	}
	return nil
}

func TestMain(m *testing.M) {
	var err error
	tempDir, err = os.MkdirTemp("", "qivitals-e2e-*")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	log.Println("Building CLI binary...")
	buildCLI := exec.Command("go", "build", "-o", cliBin, "../cmd/qivitals-cli")
	if err := runCmd(buildCLI); err != nil {
		log.Fatalf("Failed to build CLI: %v", err)
	}
	defer os.Remove(cliBin)

	log.Println("Building server binary...")
	buildServer := exec.Command("go", "build", "-o", serverBin, "../cmd/qivitals/main.go")
	if err := runCmd(buildServer); err != nil {
		log.Fatalf("Failed to build server: %v", err)
	}
	defer os.Remove(serverBin)

	keyPath := filepath.Join(tempDir, "key")
	log.Printf("Generating keys at %s...", keyPath)
	genCmd := exec.Command(cliBin, "generate-keys", "-f", keyPath)
	if err := runCmd(genCmd); err != nil {
		log.Fatalf("Failed to generate keys: %v", err)
	}

	pubKeyPath := keyPath + ".pub"
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		log.Fatalf("Failed to read public key: %v", err)
	}
	pubKeyStr := strings.TrimSpace(string(pubKeyBytes))

	// Generate Certificates for TLS
	certDir := filepath.Join(tempDir, "certs")
	log.Printf("Generating TLS certificates in %s...", certDir)
	genCertCmd := exec.Command(serverBin, "generate-certs", "--output", certDir)
	if err := runCmd(genCertCmd); err != nil {
		log.Fatalf("Failed to generate certs: %v", err)
	}
	certPath := filepath.Join(certDir, "server.crt")
	keyPathCert := filepath.Join(certDir, "server.key")

	// Ensure all namespaces used in tests are accessible
	namespaces := []string{"default", "home", "infra", "app", "chores", "temp", "network", "staging"}

	config := map[string]interface{}{
		"server": map[string]interface{}{
			"address":            "localhost:50099",
			"log_file":           filepath.Join(tempDir, "qivitals.log"),
			"tls_cert_file":      certPath,
			"tls_key_file":       keyPathCert,
			"database_url":       "", // Use in-memory
			"database_max_conns": 10,
			"auth": map[string]interface{}{
				"users": map[string]interface{}{
					"testuser": map[string]interface{}{
						"publicKeys": []string{pubKeyStr},
						"namespaces": namespaces,
					},
				},
			},
		},
		"cli": map[string]interface{}{
			"url":          "localhost:50099",
			"tls_insecure": true, // Enable TLS for tests
			"verbose":      false,
			"machine":      false,
			"identity": map[string]interface{}{
				"username": "testuser",
				"keyPath":  keyPath,
			},
		},
	}

	configData, err := yaml.Marshal(config)
	if err != nil {
		log.Fatalf("Failed to marshal config: %v", err)
	}

	configPath = filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		log.Fatalf("Failed to write config: %v", err)
	}

	code := m.Run()
	os.Exit(code)
}

func startTestServer(t *testing.T) *exec.Cmd {
	// Start server with the generated config
	cmd := exec.Command(serverBin, "serve", "--config", configPath)

	// Capture output to debug startup issues
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Start()
	require.NoError(t, err, "Failed to start test server")

	// Wait for port to open
	// We capture the result to print logs if it fails
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", serverAddress, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		return false
	}, 3*time.Second, 100*time.Millisecond, "Server failed to start in time")

	return cmd
}

// runCLI executes the compiled CLI binary. It automatically injects QIVITALS_CONFIG.
func runCLIwithErr(t *testing.T, command string) (string, string) {
	// Let shlex handle the heavy lifting of bash parsing
	args, err := shlex.Split(command)
	if err != nil {
		t.Fatalf("Failed to split command string: %v", err)
	}

	args = append(args, "--config", configPath, "--machine")
	cmd := exec.Command(cliBin, args...)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	if err != nil {
		t.Logf("CLI Error: %s", errBuf.String())
	}

	return outBuf.String(), errBuf.String()
}

func runCLI(t *testing.T, command string) string {
	outStr, errStr := runCLIwithErr(t, command)
	require.Empty(t, errStr, "CLI command failed")
	return outStr
}
