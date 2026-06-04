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

// --------- Helpers specific to condition tests ---------

// queryByID fetches a sensor by ID and decodes the response, asserting exactly one match.
func queryByID(t *testing.T, sensorID string) *v1.Sensor {
	t.Helper()
	out := runCLI(t, fmt.Sprintf("query --id %s", sensorID))
	var resp v1.QuerySensorsResponse
	require.NoError(t, protojson.Unmarshal([]byte(out), &resp), "Failed to parse query response")
	require.Len(t, resp.Sensors, 1, "Expected exactly 1 sensor for id=%s", sensorID)
	return resp.Sensors[0]
}

// registerSensor registers a sensor and returns its ID.
func registerSensor(t *testing.T, namespace, name, description, graceful, failure string, extraFlags ...string) string {
	t.Helper()
	cmd := fmt.Sprintf("register --namespace %s --name %s --description '%s' --graceful %s --failure %s",
		namespace, name, description, graceful, failure)
	if len(extraFlags) > 0 {
		cmd += " " + strings.Join(extraFlags, " ")
	}
	id := strings.TrimSpace(runCLI(t, cmd))
	require.NotEmpty(t, id, "register should return sensor ID")
	return id
}

// findConditionByType returns the condition for the given rule type, or nil.
func findConditionByType(sensor *v1.Sensor, condType string) *v1.Condition {
	for _, c := range sensor.Status.Conditions {
		if c.Type == condType {
			return c
		}
	}
	return nil
}

// findRuleByName returns the rule with the given name, or nil.
func findRuleByName(sensor *v1.Sensor, name string) *v1.ConditionRule {
	for _, r := range sensor.Spec.Rules {
		if r.Name == name {
			return r
		}
	}
	return nil
}

// assertRuleEquals checks every field of a condition rule.
func assertRuleEquals(t *testing.T, rule *v1.ConditionRule, name, expression, targetState, messageTemplate string) {
	t.Helper()
	require.NotNil(t, rule, "expected rule %q to exist", name)
	assert.Equal(t, name, rule.Name)
	assert.Equal(t, expression, rule.Expression)
	assert.Equal(t, targetState, rule.TargetState)
	assert.Equal(t, messageTemplate, rule.MessageTemplate)
}

// assertConditionStatus checks that a condition of the given type has the expected status.
func assertConditionStatus(t *testing.T, sensor *v1.Sensor, condType, expectedStatus string) {
	t.Helper()
	cond := findConditionByType(sensor, condType)
	require.NotNil(t, cond, "expected condition %q to be evaluated", condType)
	assert.Equal(t, expectedStatus, cond.Status, "condition %q should be %s", condType, expectedStatus)
}

// addCondition runs an update command to add a condition rule.
func addCondition(t *testing.T, sensorID, ruleSpec string) {
	t.Helper()
	out := runCLI(t, fmt.Sprintf("update --id %s --add-condition '%s'", sensorID, ruleSpec))
	require.Contains(t, out, "updated successfully")
}

// reportData runs a report command with the given key=value data pairs.
func reportData(t *testing.T, sensorID string, data ...string) {
	t.Helper()
	cmd := fmt.Sprintf("report --id %s", sensorID)
	for _, d := range data {
		cmd += " --data " + d
	}
	runCLI(t, cmd)
}

// --------- Tests: Registration of Conditions ---------

// TestWorkflow_Conditions_Lifecycle_AddRemoveReplace exercises the full CRUD lifecycle
// for condition rules via the patch API, asserting persistence at every step.
func TestWorkflow_Conditions_Lifecycle_AddRemoveReplace(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	sensorID := registerSensor(t, "infra", "lifecycle-test", "Full lifecycle", "1h", "2h")

	// 1. Initial state: no rules
	sensor := queryByID(t, sensorID)
	assert.Empty(t, sensor.Spec.Rules, "newly registered sensor should have no rules")
	assert.Empty(t, sensor.Status.Conditions, "no rules means no conditions evaluated")

	// 2. Add first rule
	addCondition(t, sensorID, "HighCPU:double(reported_data[\"cpu\"]) > 90:DEGRADED:CPU usage is high")
	sensor = queryByID(t, sensorID)
	require.Len(t, sensor.Spec.Rules, 1)
	assertRuleEquals(t, sensor.Spec.Rules[0], "HighCPU", `double(reported_data["cpu"]) > 90`, "DEGRADED", "CPU usage is high")

	// 3. Add second rule — order preserved (appended)
	addCondition(t, sensorID, "HighMem:double(reported_data[\"mem\"]) > 85:DEGRADED:Memory usage is high")
	sensor = queryByID(t, sensorID)
	require.Len(t, sensor.Spec.Rules, 2)
	assert.Equal(t, "HighCPU", sensor.Spec.Rules[0].Name, "first rule should remain first")
	assert.Equal(t, "HighMem", sensor.Spec.Rules[1].Name, "new rule should be appended")

	// 4. Add third rule
	addCondition(t, sensorID, "HighDisk:double(reported_data[\"disk\"]) > 95:DEAD:Disk full")
	sensor = queryByID(t, sensorID)
	require.Len(t, sensor.Spec.Rules, 3)
	assert.Equal(t, "HighDisk", sensor.Spec.Rules[2].Name)
	assert.Equal(t, "DEAD", sensor.Spec.Rules[2].TargetState)

	// 5. Replace middle rule — only that one changes
	out := runCLI(t, fmt.Sprintf("update --id %s --replace-condition '1:HighMem:double(reported_data[\"mem\"]) > 95:DEAD:Memory critical'", sensorID))
	require.Contains(t, out, "updated successfully")
	sensor = queryByID(t, sensorID)
	require.Len(t, sensor.Spec.Rules, 3, "replace should not change rule count")
	assert.Equal(t, "HighCPU", sensor.Spec.Rules[0].Name, "first rule unaffected")
	assertRuleEquals(t, sensor.Spec.Rules[1], "HighMem", `double(reported_data["mem"]) > 95`, "DEAD", "Memory critical")
	assert.Equal(t, "HighDisk", sensor.Spec.Rules[2].Name, "last rule unaffected")

	// 6. Remove middle rule by index — others shift left
	out = runCLI(t, fmt.Sprintf("update --id %s --remove-condition 1", sensorID))
	require.Contains(t, out, "updated successfully")
	sensor = queryByID(t, sensorID)
	require.Len(t, sensor.Spec.Rules, 2)
	assert.Equal(t, "HighCPU", sensor.Spec.Rules[0].Name)
	assert.Equal(t, "HighDisk", sensor.Spec.Rules[1].Name, "HighDisk should have shifted left")

	// 7. Remove all rules at once
	out = runCLI(t, fmt.Sprintf("update --id %s --remove-condition all", sensorID))
	require.Contains(t, out, "updated successfully")
	sensor = queryByID(t, sensorID)
	assert.Empty(t, sensor.Spec.Rules, "all rules should be gone after 'remove all'")
	assert.Empty(t, sensor.Status.Conditions)
}

// TestWorkflow_Conditions_Evaluation_TrueFalse verifies that the same rule
// flips between True and False as reported data changes.
func TestWorkflow_Conditions_Evaluation_TrueFalse(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	sensorID := registerSensor(t, "infra", "cert-expiry", "TLS expiry monitor", "1h", "2h")
	addCondition(t, sensorID, "CertExpiringSoon:int(reported_data[\"days_remaining\"]) < 30:DEGRADED:Cert expires in {{ .reported_data.days_remaining }} days")

	// Healthy: 60 days remaining → False
	reportData(t, sensorID, "days_remaining=60")
	sensor := queryByID(t, sensorID)
	assert.Equal(t, "60", sensor.Status.ReportedData["days_remaining"])
	require.Len(t, sensor.Status.Conditions, 1)
	cond := sensor.Status.Conditions[0]
	assert.Equal(t, "CertExpiringSoon", cond.Type)
	assert.Equal(t, "False", cond.Status)
	assert.Equal(t, "RuleNotMatched", cond.Reason)
	assert.NotZero(t, cond.LastTransitionTime)

	// Warning: 15 days remaining → True
	reportData(t, sensorID, "days_remaining=15")
	sensor = queryByID(t, sensorID)
	assert.Equal(t, "15", sensor.Status.ReportedData["days_remaining"])
	require.Len(t, sensor.Status.Conditions, 1)
	cond = sensor.Status.Conditions[0]
	assert.Equal(t, "True", cond.Status)
	assert.Equal(t, "ThresholdExceeded", cond.Reason)
	assert.Contains(t, cond.Message, "15", "message template should be rendered with 15")
	assert.Contains(t, cond.Message, "days")

	// Edge: exactly at the threshold (30) → False (strict less than)
	reportData(t, sensorID, "days_remaining=30")
	sensor = queryByID(t, sensorID)
	cond = sensor.Status.Conditions[0]
	assert.Equal(t, "False", cond.Status, "30 should NOT be < 30")

	// Edge: just below threshold → True
	reportData(t, sensorID, "days_remaining=29")
	sensor = queryByID(t, sensorID)
	cond = sensor.Status.Conditions[0]
	assert.Equal(t, "True", cond.Status, "29 should be < 30")
}

// TestWorkflow_Conditions_MultiRuleEvaluation verifies that multiple rules
// are evaluated independently in a single ReportSensor call.
func TestWorkflow_Conditions_MultiRuleEvaluation(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	sensorID := registerSensor(t, "infra", "multi-rule-server", "Server with multiple thresholds", "1h", "2h")

	addCondition(t, sensorID, "HighCPU:int(reported_data[\"cpu\"]) > 90:DEGRADED:High CPU")
	addCondition(t, sensorID, "HighMemory:int(reported_data[\"mem\"]) > 85:DEGRADED:High memory")
	addCondition(t, sensorID, "HighDisk:int(reported_data[\"disk\"]) > 90:DEAD:Disk full")

	// Report: CPU & memory triggered, disk healthy
	reportData(t, sensorID, "cpu=95", "mem=90", "disk=50")
	sensor := queryByID(t, sensorID)

	require.Len(t, sensor.Status.Conditions, 3)
	require.Len(t, sensor.Spec.Rules, 3, "all 3 rules should still be present")

	assertConditionStatus(t, sensor, "HighCPU", "True")
	assertConditionStatus(t, sensor, "HighMemory", "True")
	assertConditionStatus(t, sensor, "HighDisk", "False")

	cpuCond := findConditionByType(sensor, "HighCPU")
	memCond := findConditionByType(sensor, "HighMemory")
	diskCond := findConditionByType(sensor, "HighDisk")

	require.NotNil(t, cpuCond)
	require.NotNil(t, memCond)
	require.NotNil(t, diskCond)

	assert.Equal(t, "ThresholdExceeded", cpuCond.Reason)
	assert.Equal(t, "ThresholdExceeded", memCond.Reason)
	assert.Equal(t, "RuleNotMatched", diskCond.Reason)

	// Report: everything healthy
	reportData(t, sensorID, "cpu=10", "mem=20", "disk=30")
	sensor = queryByID(t, sensorID)
	assertConditionStatus(t, sensor, "HighCPU", "False")
	assertConditionStatus(t, sensor, "HighMemory", "False")
	assertConditionStatus(t, sensor, "HighDisk", "False")

	// Report: catastrophic — all true
	reportData(t, sensorID, "cpu=99", "mem=99", "disk=99")
	sensor = queryByID(t, sensorID)
	assertConditionStatus(t, sensor, "HighCPU", "True")
	assertConditionStatus(t, sensor, "HighMemory", "True")
	assertConditionStatus(t, sensor, "HighDisk", "True")
}

// TestWorkflow_Conditions_TargetStateOverridesState confirms that a triggered
// condition's TargetState overrides the time-based OK/DEGRADED/DEAD computation.
func TestWorkflow_Conditions_TargetStateOverridesState(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	// Long grace period so the time-based state is always OK.
	sensorID := registerSensor(t, "infra", "state-override", "Test state override", "10h", "20h")
	addCondition(t, sensorID, "Critical:double(reported_data[\"err_count\"]) > 0.0:FAILED:Errors found")

	// Healthy reporting → state stays OK
	reportData(t, sensorID, "err_count=0")
	sensor := queryByID(t, sensorID)
	assert.Equal(t, "OK", sensor.Status.State.String(), "no errors → OK")
	assertConditionStatus(t, sensor, "Critical", "False")

	// Trigger condition → state should escalate to FAILED
	reportData(t, sensorID, "err_count=5")
	sensor = queryByID(t, sensorID)
	assert.Equal(t, "FAILED", sensor.Status.State.String(), "errors → DEAD (overridden by condition target_state)")
	assertConditionStatus(t, sensor, "Critical", "True")
}

// TestWorkflow_Conditions_LabelsInExpression verifies that labels are accessible
// inside CEL expressions alongside reported_data.
func TestWorkflow_Conditions_LabelsInExpression(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	sensorID := registerSensor(t, "infra", "env-aware", "Env-aware condition", "1h", "2h", "--label environment=production")
	addCondition(t, sensorID, "ProdHighCPU:int(reported_data[\"cpu\"]) > 50 && labels[\"environment\"] == \"production\":DEGRADED:Prod CPU is high")

	// Prod + high CPU → True
	reportData(t, sensorID, "cpu=60")
	sensor := queryByID(t, sensorID)
	assertConditionStatus(t, sensor, "ProdHighCPU", "True")

	// Prod + low CPU → False
	reportData(t, sensorID, "cpu=10")
	sensor = queryByID(t, sensorID)
	assertConditionStatus(t, sensor, "ProdHighCPU", "False")

	// Change label to staging, high CPU → False (env clause fails)
	out := runCLI(t, fmt.Sprintf("update --id %s --label environment=staging", sensorID))
	require.Contains(t, out, "updated successfully")
	reportData(t, sensorID, "cpu=60")
	sensor = queryByID(t, sensorID)
	assert.Equal(t, "staging", sensor.Metadata.Labels["environment"])
	assertConditionStatus(t, sensor, "ProdHighCPU", "False")
}

// TestWorkflow_Conditions_PersistenceAcrossPatches verifies that condition rules
// survive unrelated metadata, label, and spec patches.
func TestWorkflow_Conditions_PersistenceAcrossPatches(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	sensorID := registerSensor(t, "infra", "persistence-test", "Persistence test", "1h", "2h")

	// Establish baseline conditions
	addCondition(t, sensorID, "Rule1:double(reported_data[\"v1\"]) > 10:DEGRADED:V1 high")
	addCondition(t, sensorID, "Rule2:double(reported_data[\"v2\"]) > 20:DEAD:V2 critical")

	beforePatches := queryByID(t, sensorID)
	require.Len(t, beforePatches.Spec.Rules, 2)

	// Apply a series of unrelated patches
	runCLI(t, fmt.Sprintf("update --id %s --description 'Updated description'", sensorID))
	runCLI(t, fmt.Sprintf("update --id %s --label env=prod --label team=infra", sensorID))
	runCLI(t, fmt.Sprintf("update --id %s --graceful 30m", sensorID))
	runCLI(t, fmt.Sprintf("update --id %s --failure 90m", sensorID))

	// Re-query and verify rules untouched
	after := queryByID(t, sensorID)
	assert.Equal(t, "Updated description", after.Metadata.Description)
	assert.Equal(t, "prod", after.Metadata.Labels["env"])
	assert.Equal(t, "infra", after.Metadata.Labels["team"])
	assert.Equal(t, int64(1800), after.Spec.GracefulPeriodSeconds)
	assert.Equal(t, int64(5400), after.Spec.FailurePeriodSeconds)

	require.Len(t, after.Spec.Rules, 2, "condition rules should survive unrelated patches")
	assertRuleEquals(t, findRuleByName(after, "Rule1"),
		"Rule1", `double(reported_data["v1"]) > 10`, "DEGRADED", "V1 high")
	assertRuleEquals(t, findRuleByName(after, "Rule2"),
		"Rule2", `double(reported_data["v2"]) > 20`, "DEAD", "V2 critical")
}

// TestWorkflow_Conditions_BatchOperations verifies that multiple add/remove/replace
// flags can be combined in a single command.
func TestWorkflow_Conditions_BatchOperations(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	sensorID := registerSensor(t, "infra", "batch-ops", "Batch operations", "1h", "2h")

	// Add 3 rules in a single command
	out := runCLI(t, fmt.Sprintf(
		"update --id %s --add-condition 'A:double(reported_data[\"a\"]) > 1:DEGRADED:A' --add-condition 'B:double(reported_data[\"b\"]) > 2:DEGRADED:B' --add-condition 'C:double(reported_data[\"c\"]) > 3:DEAD:C'",
		sensorID))
	require.Contains(t, out, "updated successfully")

	sensor := queryByID(t, sensorID)
	require.Len(t, sensor.Spec.Rules, 3)
	assert.Equal(t, "A", sensor.Spec.Rules[0].Name)
	assert.Equal(t, "B", sensor.Spec.Rules[1].Name)
	assert.Equal(t, "C", sensor.Spec.Rules[2].Name)

	// Combined: replace one + remove one + add one
	out = runCLI(t, fmt.Sprintf(
		"update --id %s --replace-condition '0:A2:double(reported_data[\"a\"]) > 100:DEAD:A2' --remove-condition 2 --add-condition 'D:double(reported_data[\"d\"]) > 4:DEGRADED:D'",
		sensorID))
	require.Contains(t, out, "updated successfully")

	sensor = queryByID(t, sensorID)
	require.Len(t, sensor.Spec.Rules, 3, "1 replaced + 1 removed + 1 added = same count")

	// Find by name (order may depend on patch application order)
	assert.NotNil(t, findRuleByName(sensor, "A2"), "A should be replaced by A2")
	assert.Nil(t, findRuleByName(sensor, "A"), "A should no longer exist")
	assert.NotNil(t, findRuleByName(sensor, "B"), "B should still exist")
	assert.Nil(t, findRuleByName(sensor, "C"), "C should be removed")
	assert.NotNil(t, findRuleByName(sensor, "D"), "D should be added")
}

// TestWorkflow_Conditions_StringComparison ensures non-numeric CEL expressions
// (string equality, contains, startsWith) work end-to-end.
func TestWorkflow_Conditions_StringComparison(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	sensorID := registerSensor(t, "infra", "string-conditions", "String condition tests", "1h", "2h")

	addCondition(t, sensorID, "BackupFailed:reported_data[\"status\"] == \"failed\":DEAD:Backup failed")
	addCondition(t, sensorID, "ErrorInLog:reported_data[\"log\"].contains(\"ERROR\"):DEGRADED:Error in log")
	addCondition(t, sensorID, "LocalIP:reported_data[\"ip\"].startsWith(\"10.\"):OK:Local IP")

	// Trigger all three
	reportData(t, sensorID, "status=failed", "log=\"2025-01-01 ERROR: db down\"", "ip=10.0.0.1")
	sensor := queryByID(t, sensorID)
	assertConditionStatus(t, sensor, "BackupFailed", "True")
	assertConditionStatus(t, sensor, "ErrorInLog", "True")
	assertConditionStatus(t, sensor, "LocalIP", "True")

	// Trigger none
	reportData(t, sensorID, "status=success", "log=\"2025-01-01 INFO: db up\"", "ip=8.8.8.8")
	sensor = queryByID(t, sensorID)
	assertConditionStatus(t, sensor, "BackupFailed", "False")
	assertConditionStatus(t, sensor, "ErrorInLog", "False")
	assertConditionStatus(t, sensor, "LocalIP", "False")
}

// TestWorkflow_Conditions_MessageTemplating verifies that {{ .reported_data.KEY }}
// templates are rendered with actual values when conditions trigger.
func TestWorkflow_Conditions_MessageTemplating(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	sensorID := registerSensor(t, "home", "templated", "Templated message", "1h", "2h")
	addCondition(t, sensorID,
		"LowBattery:int(reported_data[\"battery\"]) < 20:DEGRADED:Battery at {{ .reported_data.battery }}% on device {{ .reported_data.device }}")

	reportData(t, sensorID, "battery=12", "device=phone")
	sensor := queryByID(t, sensorID)

	cond := findConditionByType(sensor, "LowBattery")
	require.NotNil(t, cond)
	assert.Equal(t, "True", cond.Status)
	assert.Contains(t, cond.Message, "12%", "template should substitute battery=12")
	assert.Contains(t, cond.Message, "phone", "template should substitute device=phone")

	// Re-report with different values — message should reflect new values
	reportData(t, sensorID, "battery=5", "device=watch")
	sensor = queryByID(t, sensorID)
	cond = findConditionByType(sensor, "LowBattery")
	require.NotNil(t, cond)
	assert.Contains(t, cond.Message, "5%")
	assert.Contains(t, cond.Message, "watch")
}

// TestWorkflow_Conditions_EvaluatedOnEveryReport confirms that condition status
// is re-evaluated on every ReportSensor call, not cached.
func TestWorkflow_Conditions_EvaluatedOnEveryReport(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	sensorID := registerSensor(t, "infra", "live-eval", "Live evaluation", "1h", "2h")
	addCondition(t, sensorID, "Triggered:int(reported_data[\"x\"]) > 10:DEGRADED:Triggered")

	// Sequence: oscillate true/false/true/false to ensure each evaluation is fresh
	expectations := []struct {
		value, expected string
	}{
		{"5", "False"},
		{"15", "True"},
		{"3", "False"},
		{"100", "True"},
		{"0", "False"},
	}
	for _, e := range expectations {
		reportData(t, sensorID, "x="+e.value)
		sensor := queryByID(t, sensorID)
		cond := findConditionByType(sensor, "Triggered")
		require.NotNil(t, cond)
		assert.Equal(t, e.expected, cond.Status, "after reporting x=%s, expected %s", e.value, e.expected)
	}
}

// TestWorkflow_Conditions_RemoveByIndex verifies index-based removal correctness,
// including the shift-left behavior after each removal.
func TestWorkflow_Conditions_RemoveByIndex(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	sensorID := registerSensor(t, "infra", "remove-idx", "Remove by index", "1h", "2h")

	for _, name := range []string{"A", "B", "C", "D", "E"} {
		addCondition(t, sensorID, fmt.Sprintf("%s:double(reported_data[\"x\"]) > 0:DEGRADED:%s", name, name))
	}
	sensor := queryByID(t, sensorID)
	require.Len(t, sensor.Spec.Rules, 5)

	// Remove index 2 ('C') → [A, B, D, E]
	out := runCLI(t, fmt.Sprintf("update --id %s --remove-condition 2", sensorID))
	require.Contains(t, out, "updated successfully")
	sensor = queryByID(t, sensorID)
	require.Len(t, sensor.Spec.Rules, 4)
	assert.Equal(t, []string{"A", "B", "D", "E"}, ruleNames(sensor))

	// Remove index 0 ('A') → [B, D, E]
	out = runCLI(t, fmt.Sprintf("update --id %s --remove-condition 0", sensorID))
	require.Contains(t, out, "updated successfully")
	sensor = queryByID(t, sensorID)
	require.Len(t, sensor.Spec.Rules, 3)
	assert.Equal(t, []string{"B", "D", "E"}, ruleNames(sensor))

	// Remove last index → [B, D]
	out = runCLI(t, fmt.Sprintf("update --id %s --remove-condition 2", sensorID))
	require.Contains(t, out, "updated successfully")
	sensor = queryByID(t, sensorID)
	require.Len(t, sensor.Spec.Rules, 2)
	assert.Equal(t, []string{"B", "D"}, ruleNames(sensor))
}

// ruleNames extracts rule names in order.
func ruleNames(sensor *v1.Sensor) []string {
	names := make([]string, len(sensor.Spec.Rules))
	for i, r := range sensor.Spec.Rules {
		names[i] = r.Name
	}
	return names
}
