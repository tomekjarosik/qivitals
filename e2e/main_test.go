package e2e

import (
	"log"
	"os"
	"os/exec"
	"testing"
)

var (
	serverBin = "./test-statussvc-bin"
	cliBin    = "./test-sensorcli-bin"
)

func TestMain(m *testing.M) {
	// Run the tests and exit with the resulting code
	os.Exit(runTests(m))
}

// runTests handles setup, execution, and ensures deferred cleanup runs
func runTests(m *testing.M) int {
	log.Println("Building server binary...")
	buildServer := exec.Command("go", "build", "-o", serverBin, "../cmd/statussvc/main.go")
	if err := buildServer.Run(); err != nil {
		log.Fatalf("Failed to build server: %v", err)
	}
	// This defer will now safely execute when runTests returns!
	defer os.Remove(serverBin)

	log.Println("Building CLI binary...")
	buildCLI := exec.Command("go", "build", "-o", cliBin, "../cmd/sensorcli/main.go")
	if err := buildCLI.Run(); err != nil {
		log.Fatalf("Failed to build CLI: %v", err)
	}
	defer os.Remove(cliBin)

	return m.Run()
}
