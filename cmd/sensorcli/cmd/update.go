package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
)

func NewCmdUpdate() *cobra.Command {
	var (
		sensorID        string
		namespace       string
		sensorName      string
		newName         string
		newNamespace    string
		description     string
		gracefulSeconds int64
		failureSeconds  int64
		labelsToAdd     []string
		labelsToRemove  []string
	)

	cmd := &cobra.Command{
		Use:   "update [flags]",
		Short: "Partially update an existing sensor's configuration",
		Long: `Patch specific properties or labels of an already registered sensor.

Identify the sensor using EITHER its --id OR its --namespace and --name.

Examples:
  # Patch using unique ID
  sensorcli update --id 550e8400-e29b --failure 3600

  # Patch using human-readable Namespace & Name
  sensorcli update --namespace db --name "Daily Backup" --graceful 1800

  # Rename a sensor
  sensorcli update --namespace infra --name "old-job" --new-name "new-job"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sensorID == "" && sensorName == "" {
				return fmt.Errorf("must provide either --id or --name to identify the sensor to update")
			}
			return runUpdate(cmd, sensorID, namespace, sensorName, newName, newNamespace, description, gracefulSeconds, failureSeconds, labelsToAdd, labelsToRemove)
		},
	}

	cmd.Flags().StringVarP(&sensorID, "id", "i", "", "Unique sensor UUID to update")
	cmd.Flags().StringVar(&namespace, "namespace", "default", "Current namespace (used with --name)")
	cmd.Flags().StringVarP(&sensorName, "name", "n", "", "Current sensor name")

	cmd.Flags().StringVar(&newName, "new-name", "", "Rename the sensor")
	cmd.Flags().StringVar(&newNamespace, "new-namespace", "", "Move sensor to a new namespace")
	cmd.Flags().StringVar(&description, "desc", "", "New sensor description")
	cmd.Flags().Int64Var(&gracefulSeconds, "graceful", 0, "New graceful period in seconds")
	cmd.Flags().Int64Var(&failureSeconds, "failure", 0, "New failure period in seconds")
	cmd.Flags().StringArrayVarP(&labelsToAdd, "label", "l", []string{}, "Labels to add/update as key:value")
	cmd.Flags().StringArrayVar(&labelsToRemove, "remove-label", []string{}, "Label keys to remove")

	return cmd
}

func runUpdate(cmd *cobra.Command, sensorID, namespace, sensorName, newName, newNamespace, description string, gracefulSeconds, failureSeconds int64, labelsToAdd, labelsToRemove []string) error {
	client, conn, err := NewStatusClient(cmd.Context())
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
	if cmd.Flags().Changed("desc") {
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
			Value: strconv.FormatInt(gracefulSeconds, 10),
		})
	}
	if cmd.Flags().Changed("failure") {
		patches = append(patches, &v1.PatchOperation{
			Op:    "replace",
			Path:  "/spec/failure_period_seconds",
			Value: strconv.FormatInt(failureSeconds, 10),
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
