package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
)

func NewCmdUpdate() *cobra.Command {
	var (
		sensorID       string
		namespace      string
		sensorName     string
		newName        string
		newNamespace   string
		description    string
		graceful       string
		failure        string
		labelsToAdd    []string
		labelsToRemove []string

		// Condition rule flags
		addCondition     []string // format: "name:expression:target_state:message_template"
		removeCondition  []string // index (0, 1, 2, ...) or "all"
		replaceCondition []string // format: "index:name:expression:target_state:message_template"
	)

	cmd := &cobra.Command{
		Use:   "update [flags]",
		Short: "Partially update an existing sensor's configuration",
		Long: `Patch specific properties or labels of an already registered sensor.

Identify the sensor using EITHER its --id OR its --namespace and --name.

Examples:
  # Patch using unique ID
  qivitals-cli update --id 550e8400-e29b --failure 1h

  # Patch using human-readable Namespace & Name
  qivitals-cli update --namespace db --name "Daily Backup" --graceful 30m

  # Rename a sensor
  qivitals-cli update --namespace infra --name "old-job" --new-name "new-job"

  # Add a condition rule
  qivitals-cli update --id 550e8400-e29b --add-condition "LowBattery:double(reported_data['battery']) < 15.0:DEGRADED:Battery low"

  # Remove a condition by index
  qivitals-cli update --id 550e8400-e29b --remove-condition 0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sensorID == "" && sensorName == "" {
				return fmt.Errorf("must provide either --id or --name to identify the sensor to update")
			}
			gracefulDuration, err := ParseExtendedDuration(graceful)
			if err != nil {
				return fmt.Errorf("invalid value for --graceful: %w", err)
			}
			failureDuration, err := ParseExtendedDuration(failure)
			if err != nil {
				return fmt.Errorf("invalid value for --failure: %w", err)
			}
			return runUpdate(cmd, sensorID, namespace, sensorName, newName, newNamespace, description, gracefulDuration, failureDuration, labelsToAdd, labelsToRemove, addCondition, removeCondition, replaceCondition)
		},
	}

	cmd.Flags().StringVarP(&sensorID, "id", "i", "", "Unique sensor UUID to update")
	cmd.Flags().StringVar(&namespace, "namespace", "default", "Current namespace (used with --name)")
	cmd.Flags().StringVarP(&sensorName, "name", "n", "", "Current sensor name")

	cmd.Flags().StringVar(&newName, "new-name", "", "Rename the sensor")
	cmd.Flags().StringVar(&newNamespace, "new-namespace", "", "Move sensor to a new namespace")
	cmd.Flags().StringVar(&description, "description", "", "New sensor description")
	cmd.Flags().StringVarP(&graceful, "graceful", "g", "300s", "Duration before showing DEGRADED status")
	cmd.Flags().StringVarP(&failure, "failure", "f", "900s", "Duration before showing DEAD status")
	cmd.Flags().StringArrayVarP(&labelsToAdd, "label", "l", []string{}, "Labels to add/update as key:value")
	cmd.Flags().StringArrayVar(&labelsToRemove, "remove-label", []string{}, "Label keys to remove")

	cmd.Flags().StringArrayVar(&addCondition, "add-condition", []string{}, "Add a condition rule (format: 'name:expression:target_state:message_template')")
	cmd.Flags().StringArrayVar(&removeCondition, "remove-condition", []string{}, "Remove a condition rule by index (e.g., '0', '1', or 'all')")
	cmd.Flags().StringArrayVar(&replaceCondition, "replace-condition", []string{}, "Replace a condition rule by index (format: 'index:name:expression:target_state:message_template')")

	return cmd
}

func runUpdate(cmd *cobra.Command, sensorID, namespace, sensorName, newName, newNamespace, description string, gracefulDuration, failureDuration time.Duration, labelsToAdd, labelsToRemove []string, addCondition, removeCondition, replaceCondition []string) error {
	client, conn, err := NewQiVitalsClient(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer conn.Close()

	queryReq := &v1.QuerySensorsRequest{}
	if sensorID != "" {
		queryReq.Id = sensorID
	} else {
		queryReq.Namespace = namespace
		queryReq.Name = sensorName
	}
	queryResp, err := client.QuerySensors(cmd.Context(), queryReq)
	if err != nil || len(queryResp.Sensors) == 0 {
		return fmt.Errorf("sensor '%s/%s' not found", namespace, sensorName)
	}
	originalSensorData := queryResp.Sensors[0]

	var patches []*v1.PatchOperation

	if cmd.Flags().Changed("new-name") {
		patches = append(patches, &v1.PatchOperation{
			Op:    "replace",
			Path:  "/metadata/name",
			Value: fmt.Sprintf("\"%s\"", newName),
		})
	}
	if cmd.Flags().Changed("new-namespace") {
		patches = append(patches, &v1.PatchOperation{
			Op:    "replace",
			Path:  "/metadata/namespace",
			Value: fmt.Sprintf("\"%s\"", newNamespace),
		})
	}
	if cmd.Flags().Changed("description") {
		patches = append(patches, &v1.PatchOperation{
			Op:    "replace",
			Path:  "/metadata/description",
			Value: fmt.Sprintf("\"%s\"", description),
		})
	}

	if cmd.Flags().Changed("graceful") {
		patches = append(patches, &v1.PatchOperation{
			Op:    "replace",
			Path:  "/spec/graceful_period_seconds",
			Value: strconv.FormatInt(int64(math.Round(gracefulDuration.Seconds())), 10),
		})
	}
	if cmd.Flags().Changed("failure") {
		patches = append(patches, &v1.PatchOperation{
			Op:    "replace",
			Path:  "/spec/failure_period_seconds",
			Value: strconv.FormatInt(int64(math.Round(failureDuration.Seconds())), 10),
		})
	}

	for _, label := range labelsToRemove {
		patches = append(patches, &v1.PatchOperation{
			Op:   "remove",
			Path: fmt.Sprintf("/metadata/labels/%s", label),
		})
	}
	labelsToAddMap, err := parseLabels(labelsToAdd)
	if err != nil {
		return fmt.Errorf("failed to parse labels to add: %w", err)
	}
	for key, value := range labelsToAddMap {
		patches = append(patches, &v1.PatchOperation{
			Op:    "add",
			Path:  fmt.Sprintf("/metadata/labels/%s", key),
			Value: fmt.Sprintf("\"%s\"", value),
		})
	}

	conditionPatches, err := calculatePatchConditions(originalSensorData, addCondition, removeCondition, replaceCondition)
	if err != nil {
		return fmt.Errorf("failed to process condition rules: %w", err)
	}
	patches = append(patches, conditionPatches...)

	req := &v1.PatchSensorRequest{
		Id:         originalSensorData.Metadata.Id,
		Version:    originalSensorData.Metadata.ResourceVersion,
		Operations: patches,
	}

	response, err := client.PatchSensor(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to update sensor: %w", err)
	}

	fmt.Printf("Sensor updated successfully. New Sensor data: %v\n", response.Sensor)
	return nil
}

// makeAddOp builds a JSON Patch "add" op that appends a rule to /spec/rules.
func makeAddRuleOp(rule *v1.ConditionRule) (*v1.PatchOperation, error) {
	data, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("marshal rule: %w", err)
	}
	return &v1.PatchOperation{
		Op:    "add",
		Path:  "/spec/rules/-",
		Value: string(data),
	}, nil
}

// calculatePatchConditions builds JSON Patch operations for condition rules.
func calculatePatchConditions(sensor *v1.Sensor, addRules, removeRules, replaceRules []string) ([]*v1.PatchOperation, error) {
	var patches []*v1.PatchOperation
	ruleCount := len(sensor.Spec.Rules)

	// Add operations
	for _, s := range addRules {
		rule, err := parseConditionRule(s)
		if err != nil {
			return nil, fmt.Errorf("invalid add-condition %q: %w", s, err)
		}
		op, err := makeAddRuleOp(rule)
		if err != nil {
			return nil, err
		}
		patches = append(patches, op)
	}

	// Remove operations
	for _, s := range removeRules {
		if s == "all" {
			patches = append(patches, &v1.PatchOperation{
				Op:    "replace",
				Path:  "/spec/rules",
				Value: "[]",
			})
			ruleCount = 0
			continue
		}
		idx, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("invalid remove-condition index %q: %w", s, err)
		}
		if idx < 0 || idx >= ruleCount {
			return nil, fmt.Errorf("remove-condition index %d out of range [0, %d)", idx, ruleCount)
		}
		patches = append(patches, &v1.PatchOperation{
			Op:   "remove",
			Path: fmt.Sprintf("/spec/rules/%d", idx),
		})
		ruleCount--
	}

	// Replace operations: format is "index:name:expression:target_state:message_template"
	for _, s := range replaceRules {
		idxStr, ruleStr, ok := strings.Cut(s, ":")
		if !ok {
			return nil, fmt.Errorf("invalid replace-condition %q: expected 'index:rule'", s)
		}
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			return nil, fmt.Errorf("invalid replace-condition index %q: %w", idxStr, err)
		}
		if idx < 0 || idx >= ruleCount {
			return nil, fmt.Errorf("replace-condition index %d out of range [0, %d)", idx, ruleCount)
		}
		rule, err := parseConditionRule(ruleStr)
		if err != nil {
			return nil, fmt.Errorf("invalid replace-condition rule %q: %w", ruleStr, err)
		}
		data, err := json.Marshal(rule)
		if err != nil {
			return nil, fmt.Errorf("marshal rule: %w", err)
		}
		patches = append(patches, &v1.PatchOperation{
			Op:    "replace",
			Path:  fmt.Sprintf("/spec/rules/%d", idx),
			Value: string(data),
		})
	}

	return patches, nil
}
