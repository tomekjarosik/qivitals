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
	tmpBinDir  = "./tmp"
	serverBin  = "./tmp/test-qivitals-server-bin"
	cliBin     = "./tmp/test-qivitals-cli-bin"
	tempDir    string
	configPath string
)

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}
	// Use absolute paths to ensure os.Remove works correctly
	tmpBinDir = filepath.Join(cwd, tmpBinDir)
	serverBin = filepath.Join(cwd, "tmp", "test-qivitals-bin")
	cliBin = filepath.Join(cwd, "tmp", "test-qivitals-cli-bin")
}

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
	buildCLI := exec.Command("go", "build", "-o", cliBin, "../cmd/qivitals-cli/")
	if err := runCmd(buildCLI); err != nil {
		log.Fatalf("Failed to build CLI: %v", err)
	}
	defer os.Remove(cliBin)

	log.Println("Building server binary...")
	buildServer := exec.Command("go", "build", "-o", serverBin, "../cmd/qivitals-server/")
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
			"address": serverAddress,
		},
		"database": map[string]interface{}{
			"url":       "", // Use in-memory
			"max_conns": 10,
		},
		"log": map[string]interface{}{
			"file":  filepath.Join(tempDir, "qivitals.log"),
			"level": "debug",
		},
		"tls": map[string]interface{}{
			"enabled":   true,
			"cert_file": certPath,
			"key_file":  keyPathCert,
		},
		"auth": map[string]interface{}{
			"users": map[string]interface{}{
				"testuser": map[string]interface{}{
					"public_keys": []string{pubKeyStr},
					"namespaces":  namespaces,
					"emails":      []string{"testuser@qivitals.local"}, // Added for Magic Link routing
				},
			},
		},
		"magic_link": map[string]interface{}{
			"app_base_url": "http://localhost:3000", // Required by Validate()
			"app_name":     "Qivitals E2E",
			"from_email":   "noreply@qivitals.local", // Required by Validate()
		},
		"email": map[string]interface{}{
			"sender_type": "file",
			"file_path":   filepath.Join(tempDir, "emails.jsonl"),
			"from_email":  "noreply@qivitals.local",
		},
		"cli": map[string]interface{}{
			"url":          serverAddress,
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

	log.Println("Cleaning up test artifacts...")

	if err := os.Remove(cliBin); err != nil {
		log.Printf("Warning: Failed to remove CLI binary %s: %v", cliBin, err)
	} else {
		log.Println("Removed CLI binary")
	}

	if err := os.Remove(serverBin); err != nil {
		log.Printf("Warning: Failed to remove server binary %s: %v", serverBin, err)
	} else {
		log.Println("Removed server binary")
	}
	os.Remove(tmpBinDir)

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

	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("\n--- SERVER STDOUT ---\n%s", outBuf.String())
			t.Logf("\n--- SERVER STDERR ---\n%s", errBuf.String())
		}
	})
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

// stopTestServer gracefully shuts down the server and waits for the port to be released.
func stopTestServer(t *testing.T, cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	cmd.Process.Signal(os.Interrupt)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// Success: process exited cleanly and port is released
	case <-time.After(3 * time.Second):
		// Timeout: force kill if it hangs
		t.Log("Server did not exit gracefully, forcing kill")
		cmd.Process.Kill()
		<-done // Reap the zombie process
	}
}

// runCLI executes the compiled CLI binary. It automatically injects QIVITALS_CONFIG.
func runCLIwithErr(t *testing.T, command string) (string, string) {
	// Let shlex handle the heavy lifting of bash parsing
	args, err := shlex.Split(command)
	if err != nil {
		t.Fatalf("Failed to split command string: %v", err)
	}

	args = append(args, "--config", configPath, "--output", "json")
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
