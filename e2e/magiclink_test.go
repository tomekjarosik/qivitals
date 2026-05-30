package e2e

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkflow_MagicLink(t *testing.T) {
	serverCmd := startTestServer(t)
	defer stopTestServer(t, serverCmd)

	emailFilePath := filepath.Join(tempDir, "emails.jsonl")
	targetEmail := "testuser@qivitals.local"
	baseURL := fmt.Sprintf("https://%s", serverAddress)

	// HTTP Client that skips TLS verification for self-signed E2E certs
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Request Magic Link via REST API
	sendPayload := map[string]string{"email": targetEmail}
	sendBody, _ := json.Marshal(sendPayload)

	resp, err := httpClient.Post(baseURL+"/api/v1/magiclink/send", "application/json", bytes.NewReader(sendBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Extract Token from the mocked email file
	token := extractTokenFromFile(t, emailFilePath, targetEmail)
	require.NotEmpty(t, token, "Failed to extract token from email file")

	// Validate Magic Link via REST API
	verifyPayload := map[string]string{"token": token}
	verifyBody, _ := json.Marshal(verifyPayload)

	resp, err = httpClient.Post(baseURL+"/api/v1/magiclink/verify", "application/json", bytes.NewReader(verifyBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var verifyResp struct {
		SessionToken string `json:"sessionToken"` // Note: grpc-gateway defaults to camelCase for JSON
	}
	err = json.NewDecoder(resp.Body).Decode(&verifyResp)
	resp.Body.Close()
	require.NoError(t, err)
	require.NotEmpty(t, verifyResp.SessionToken, "Session token should not be empty")

	// 4. Prove the session token works by calling a protected endpoint
	req, _ := http.NewRequest("GET", baseURL+"/api/v1/sensors?namespace=home", nil)
	req.Header.Set("Authorization", "Bearer "+verifyResp.SessionToken)

	resp, err = httpClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "Session token should grant access to protected endpoints")
	resp.Body.Close()
}

// extractTokenFromFile parses the JSONL file and uses Regex to pull the JWT from the URL.
func extractTokenFromFile(t *testing.T, filePath, email string) string {
	data, err := os.ReadFile(filePath)
	require.NoError(t, err, "Failed to read email file")

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	// Regex matches standard JWT characters (Base64URL + dots)
	re := regexp.MustCompile(`token=([a-zA-Z0-9_\-\.]+)`)

	// Iterate backwards to find the most recent email for this user
	for i := len(lines) - 1; i >= 0; i-- {
		var record struct {
			To       string `json:"to"`
			TextBody string `json:"text_body"`
		}
		if err := json.Unmarshal([]byte(lines[i]), &record); err != nil {
			continue
		}

		if record.To == email {
			matches := re.FindStringSubmatch(record.TextBody)
			if len(matches) > 1 {
				return matches[1]
			}
		}
	}

	t.Fatalf("Token not found for email %s in file %s", email, filePath)
	return ""
}
