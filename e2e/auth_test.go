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

func TestWorkflow_UnauthorizedNamespaceRegistration(t *testing.T) {
	serverCmd := startTestServer(t)
	defer stopTestServer(t, serverCmd)

	// Try to register a sensor in a namespace the test user doesn't have access to
	// The config only allows: default, home, infra, app, chores, temp, network
	_, stderr := runCLIwithErr(t, "register --namespace finance --name budget-tracker --description 'Monthly budget' --graceful 30d --failure 35d")

	// Should fail with an error
	require.NotEmpty(t, stderr, "Register command should produce output")
	assert.Contains(t, stderr, "code = PermissionDenied desc = user testuser cannot register sensor in namespace finance", "CLI should report an error for unauthorized namespace")
}

// TODO:
//func TestWorkflow_UnauthorizedNamespaceQuery(t *testing.T) {
//	serverCmd := startTestServer(t)
//	defer serverCmd.Process.Kill()
//
//	// Register a sensor in an authorized namespace first
//	id := strings.TrimSpace(runCLI(t, "register --namespace home --name authorized-sensor --description 'Should be accessible' --graceful 1h --failure 2h"))
//	require.NotEmpty(t, id)
//
//	// Try to query a sensor in an unauthorized namespace
//	// This should return empty results since user has no access to that namespace
//	queryOut := runCLI(t, "query --namespace finance --name non-existent")
//
//	var resp v1.QuerySensorsResponse
//	err := protojson.Unmarshal([]byte(queryOut), &resp)
//	require.NoError(t, err)
//
//	// Should return empty list (no sensors found in unauthorized namespace)
//	assert.Empty(t, resp.Sensors, "Query should return no sensors for unauthorized namespace")
//}

func TestWorkflow_UnauthorizedNamespaceDelete(t *testing.T) {
	serverCmd := startTestServer(t)
	defer stopTestServer(t, serverCmd)

	// Register a sensor in an unauthorized namespace (this should fail)
	financeID, stderr := runCLIwithErr(t, "register --namespace finance --name secret-budget --description 'Secret budget' --graceful 1h --failure 2h")
	require.Empty(t, financeID, "Register in unauthorized namespace should fail and return empty")
	require.Contains(t, stderr, "code = PermissionDenied desc = user testuser cannot register sensor in namespace finance", "CLI should report an error for unauthorized namespace")
	// Even if somehow the sensor exists, try to delete it
	// This should also fail
	stdout, stderr := runCLIwithErr(t, "delete --id some-uuid")
	require.Empty(t, stdout, "Should not be able to delete a sensor in an unauthorized namespace")
	require.Contains(t, stderr, "code = NotFound desc = sensor some-uuid not found", "CLI should report an error for unauthorized namespace")
}

func TestWorkflow_CrossNamespaceAccess(t *testing.T) {
	serverCmd := startTestServer(t)
	defer stopTestServer(t, serverCmd)

	// Register sensors in different namespaces that user has access to
	infraID := strings.TrimSpace(runCLI(t, "register --namespace infra --name infra-sensor --description 'Infrastructure sensor' --graceful 1h --failure 2h"))
	require.NotEmpty(t, infraID)

	appID := strings.TrimSpace(runCLI(t, "register --namespace app --name app-sensor --description 'Application sensor' --graceful 1h --failure 2h"))
	require.NotEmpty(t, appID)

	// Query all sensors (should return both)
	queryOut := runCLI(t, "query")
	var resp v1.QuerySensorsResponse
	err := protojson.Unmarshal([]byte(queryOut), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Sensors, 2)

	//Verify both namespaces are accessible
	names := getNamesFromResponse(&resp)
	assert.Contains(t, names, "infra-sensor")
	assert.Contains(t, names, "app-sensor")

	// Verify each sensor's namespace is correct
	infraSensor := findSensorByID(resp.Sensors, infraID)
	require.NotNil(t, infraSensor)
	assert.Equal(t, "infra", infraSensor.Metadata.Namespace)

	appSensor := findSensorByID(resp.Sensors, appID)
	require.NotNil(t, appSensor)
	assert.Equal(t, "app", appSensor.Metadata.Namespace)
}

func TestWorkflow_SensorStateUnauthorized(t *testing.T) {
	serverCmd := startTestServer(t)
	defer stopTestServer(t, serverCmd)

	// Register sensor in authorized namespace
	id := strings.TrimSpace(runCLI(t, "register --namespace home --name test-sensor --description 'Test sensor' --graceful 1h --failure 2h"))
	require.NotEmpty(t, id)

	// Report data to set state to OK
	runCLI(t, fmt.Sprintf("report --id %s --data status=ok", id))

	// Verify state is OK
	queryOut := runCLI(t, fmt.Sprintf("query --namespace home --name test-sensor"))
	var resp v1.QuerySensorsResponse
	err := protojson.Unmarshal([]byte(queryOut), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Sensors, 1)

	assert.Equal(t, "OK", resp.Sensors[0].Status.State)
}
