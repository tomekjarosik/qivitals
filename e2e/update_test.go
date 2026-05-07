package e2e

//func TestWorkflow_Update_Success(t *testing.T) {
//	startTestServer(t)
//
//	out, _, err := runCLI(t,
//		"register", "--namespace", "production", "--name", "web-api",
//		"--description", "Primary API server", "--graceful", "1h", "--failure", "2h",
//		"--label", "env=prod", "--label", "tier=backend")
//	require.NoError(t, err)
//	sensorID := strings.TrimSpace(strings.Split(out, "id: ")[1])
//
//	initialNamespace := "production"
//	initialName := "web-api"
//
//	_, stderr, err := runCLI(t, "update", "--id", sensorID,
//		"--description", "Updated API Description",
//		"--graceful", "1800s")
//	require.NoError(t, err, "CLI Update command failed. Stderr: %s", stderr)
//	require.Contains(t, out, "updated successfully")
//
//	out, _, err = runCLI(t, "query", "--namespace", initialNamespace, "--name", initialName)
//	require.NoError(t, err)
//	var resp v1.QuerySensorsResponse
//	require.NoError(t, protojson.Unmarshal([]byte(out), &resp))
//
//	require.Len(t, resp.Sensors, 1, "Sensor should still exist in the namespace")
//	updatedSensor := resp.Sensors[0]
//
//	assert.Equal(t, "Updated API Description", updatedSensor.Metadata.Description)
//	assert.Equal(t, int64(1800), updatedSensor.Spec.GracefulPeriodSeconds)
//
//	// Regression: unmentioned fields unchanged
//	assert.Equal(t, initialName, updatedSensor.Metadata.Name)
//	assert.Equal(t, initialNamespace, updatedSensor.Metadata.Namespace)
//	assert.Equal(t, "prod", updatedSensor.Metadata.Labels["env"])
//}
