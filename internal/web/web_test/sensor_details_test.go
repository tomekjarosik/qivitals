package web_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
)

func TestSensorDetailPage_Golden(t *testing.T) {
	env := newTestEnv(t)

	w := env.doRequest(t, "/sensors/test-sensor-default")
	require.Equal(t, http.StatusOK, w.Code)

	actualHTML := normalizeTimestamps(w.Body.Bytes())
	goldenPath := filepath.Join("testdata", "sensor_detail_page.golden.html")

	if *update {
		err := os.WriteFile(goldenPath, w.Body.Bytes(), 0644)
		require.NoError(t, err)
		t.Logf("Golden file updated at %s", goldenPath)
		return
	}

	expectedHTML, err := os.ReadFile(goldenPath)
	require.NoError(t, err)

	expectedHTML = normalizeTimestamps(expectedHTML)
	assert.Equal(t, string(expectedHTML), string(actualHTML), "HTML output does not match golden file")
}

func TestSensorDetailPage_WithLabels_Golden(t *testing.T) {
	env := newTestEnv(t,
		withID("test-sensor-labels"),
		withName("Sensor With Labels"),
		withLabels(map[string]string{
			"environment": "production",
			"team":        "backend",
			"cost-center": "engineering",
		}),
	)

	w := env.doRequest(t, "/sensors/test-sensor-labels")
	require.Equal(t, http.StatusOK, w.Code)

	actualHTML := normalizeTimestamps(w.Body.Bytes())
	goldenPath := filepath.Join("testdata", "sensor_detail_page_with_labels.golden.html")

	if *update {
		err := os.WriteFile(goldenPath, w.Body.Bytes(), 0644)
		require.NoError(t, err)
		t.Logf("Golden file updated at %s", goldenPath)
		return
	}

	expectedHTML, err := os.ReadFile(goldenPath)
	require.NoError(t, err)

	expectedHTML = normalizeTimestamps(expectedHTML)
	assert.Equal(t, string(expectedHTML), string(actualHTML), "HTML output does not match golden file")
}

func TestSensorDetailPage_WithConditions_Golden(t *testing.T) {
	env := newTestEnv(t,
		withID("test-sensor-conditions"),
		withName("Sensor With Conditions"),
		withConditionRules([]*v1.ConditionRule{
			{
				Name:            "high_cpu",
				Expression:      `int(reported_data['cpu']) > 90`,
				TargetState:     "CRITICAL",
				MessageTemplate: "CPU usage is critically high",
			},
			{
				Name:            "low_battery",
				Expression:      `int(reported_data['battery']) < 20`,
				TargetState:     "WARNING",
				MessageTemplate: "Battery level is low",
			},
		}),
		withData(map[string]string{
			"battery": "95",
			"cpu":     "95",
		}),
	)

	w := env.doRequest(t, "/sensors/test-sensor-conditions")
	require.Equal(t, http.StatusOK, w.Code)

	actualHTML := normalizeTimestamps(w.Body.Bytes())
	goldenPath := filepath.Join("testdata", "sensor_detail_page_with_conditions.golden.html")

	if *update {
		err := os.WriteFile(goldenPath, w.Body.Bytes(), 0644)
		require.NoError(t, err)
		t.Logf("Golden file updated at %s", goldenPath)
		return
	}

	expectedHTML, err := os.ReadFile(goldenPath)
	require.NoError(t, err)

	expectedHTML = normalizeTimestamps(expectedHTML)
	assert.Equal(t, string(expectedHTML), string(actualHTML), "HTML output does not match golden file")
}

func TestSensorDetailPage_WithBoth_Golden(t *testing.T) {
	env := newTestEnv(t,
		withID("test-sensor-both"),
		withName("Sensor With Both"),
		withLabels(map[string]string{
			"environment": "staging",
			"team":        "platform",
		}),
		withConditionRules([]*v1.ConditionRule{
			{
				Name:            "high_cpu",
				Expression:      `int(reported_data['cpu']) > 90`,
				TargetState:     "CRITICAL",
				MessageTemplate: "CPU usage is critically high",
			},
		}),
		withData(map[string]string{
			"battery": "50",
			"cpu":     "45",
		}),
	)

	w := env.doRequest(t, "/sensors/test-sensor-both")
	require.Equal(t, http.StatusOK, w.Code)

	actualHTML := normalizeTimestamps(w.Body.Bytes())
	goldenPath := filepath.Join("testdata", "sensor_detail_page_with_both.golden.html")

	if *update {
		err := os.WriteFile(goldenPath, w.Body.Bytes(), 0644)
		require.NoError(t, err)
		t.Logf("Golden file updated at %s", goldenPath)
		return
	}

	expectedHTML, err := os.ReadFile(goldenPath)
	require.NoError(t, err)

	expectedHTML = normalizeTimestamps(expectedHTML)
	assert.Equal(t, string(expectedHTML), string(actualHTML), "HTML output does not match golden file")
}

func TestSensorDetailPage_WithNoData_Golden(t *testing.T) {
	env := newTestEnv(t,
		withID("test-sensor-no-data"),
		withName("Dead Sensor"),
		withNoData,
	)

	w := env.doRequest(t, "/sensors/test-sensor-no-data")
	require.Equal(t, http.StatusOK, w.Code)

	actualHTML := normalizeTimestamps(w.Body.Bytes())
	goldenPath := filepath.Join("testdata", "sensor_detail_page_no_data.golden.html")

	if *update {
		err := os.WriteFile(goldenPath, w.Body.Bytes(), 0644)
		require.NoError(t, err)
		t.Logf("Golden file updated at %s", goldenPath)
		return
	}

	expectedHTML, err := os.ReadFile(goldenPath)
	require.NoError(t, err)

	expectedHTML = normalizeTimestamps(expectedHTML)
	assert.Equal(t, string(expectedHTML), string(actualHTML), "HTML output does not match golden file")
}
