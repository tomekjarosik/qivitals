package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func NewCmdUpdate() *cobra.Command {
	var (
		sensorID        string
		namespace       string
		sensorName      string
		description     string
		gracefulSeconds int64
		failureSeconds  int64
		labelsToAdd     []string
		labelsToRemove  []string
	)

	cmd := &cobra.Command{
		Use:   "update [flags]",
		Short: "Partially update an existing sensor's configuration",
		Long: `Update specific properties or labels of an already registered sensor.

This command performs a partial update. Only the properties you explicitly 
provide will be modified.

When modifying labels, the CLI merges your changes with the existing labels.

Examples:
  # Update a property
  sensorcli update --id 550e8400-e29b --failure 3600

  # Add a new label and update an existing one
  sensorcli update --id 550e8400-e29b --label "tier:backend" --label "env:staging"

  # Remove a label
  sensorcli update --id 550e8400-e29b --remove-label "temporary-debug"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd, sensorID, namespace, sensorName, description, gracefulSeconds, failureSeconds, labelsToAdd, labelsToRemove)
		},
	}

	cmd.Flags().StringVarP(&sensorID, "id", "i", "", "Unique sensor UUID to update (required)")
	cmd.Flags().StringVar(&namespace, "namespace", "", "New namespace for the sensor")
	cmd.Flags().StringVarP(&sensorName, "name", "n", "", "New human-readable sensor name")
	cmd.Flags().StringVar(&description, "desc", "", "New sensor description")
	cmd.Flags().Int64Var(&gracefulSeconds, "graceful", 0, "New graceful period in seconds")
	cmd.Flags().Int64Var(&failureSeconds, "failure", 0, "New failure period in seconds")
	cmd.Flags().StringArrayVarP(&labelsToAdd, "label", "l", []string{}, "Labels to add/update as key:value (can be repeated)")
	cmd.Flags().StringArrayVar(&labelsToRemove, "remove-label", []string{}, "Label keys to remove (can be repeated)")

	cmd.MarkFlagRequired("id")

	return cmd
}

func runUpdate(cmd *cobra.Command, sensorID, namespace, sensorName, description string, gracefulSeconds, failureSeconds int64, labelsToAdd, labelsToRemove []string) error {
	client, conn, err := NewStatusClient(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer conn.Close()

	// Initialize the full Sensor structure required for the update payload
	sensorObj := &v1.Sensor{
		Metadata: &v1.ObjectMeta{
			Id: sensorID,
		},
		Spec: &v1.SensorSpec{},
	}

	var updatePaths []string

	// Check metadata updates
	if cmd.Flags().Changed("name") {
		sensorObj.Metadata.Name = sensorName
		updatePaths = append(updatePaths, "metadata.name")
	}
	if cmd.Flags().Changed("namespace") {
		sensorObj.Metadata.Namespace = namespace
		updatePaths = append(updatePaths, "metadata.namespace")
	}
	if cmd.Flags().Changed("desc") {
		sensorObj.Metadata.Description = description
		updatePaths = append(updatePaths, "metadata.description")
	}

	// Check spec updates
	if cmd.Flags().Changed("graceful") {
		sensorObj.Spec.GracefulPeriodSeconds = gracefulSeconds
		updatePaths = append(updatePaths, "spec.graceful_period_seconds")
	}
	if cmd.Flags().Changed("failure") {
		sensorObj.Spec.FailurePeriodSeconds = failureSeconds
		updatePaths = append(updatePaths, "spec.failure_period_seconds")
	}

	// Handle Label modifications
	labelsChanged := len(labelsToAdd) > 0 || len(labelsToRemove) > 0
	if labelsChanged {
		// 1. Fetch current labels
		queryResp, err := client.QuerySensors(cmd.Context(), &v1.QuerySensorsRequest{Id: sensorID})
		if err != nil {
			return fmt.Errorf("failed to fetch current sensor state for label update: %w", err)
		}
		if len(queryResp.Sensors) == 0 {
			return fmt.Errorf("sensor '%s' not found", sensorID)
		}

		existingSensor := queryResp.Sensors[0]
		if existingSensor.Metadata == nil {
			return fmt.Errorf("server returned invalid sensor metadata")
		}

		// Initialize local map with existing labels
		currentLabels := existingSensor.Metadata.Labels
		if currentLabels == nil {
			currentLabels = make(map[string]string)
		}

		// 2. Remove specified labels
		for _, keyToRemove := range labelsToRemove {
			delete(currentLabels, keyToRemove)
		}

		// 3. Add/Update specified labels
		for _, lblStr := range labelsToAdd {
			// Labels are expected as key=value or key:value based on your parsing pref.
			// We'll split on = or : depending on what your CLI docs say (your docs use :)
			delimiter := ":"
			if strings.Contains(lblStr, "=") {
				delimiter = "="
			}
			parts := strings.SplitN(lblStr, delimiter, 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid label format '%s', expected key:value or key=value", lblStr)
			}
			currentLabels[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}

		// 4. Assign the map back to the update payload
		sensorObj.Metadata.Labels = currentLabels
		updatePaths = append(updatePaths, "metadata.labels")
	}

	if len(updatePaths) == 0 {
		return fmt.Errorf("no update fields provided. specify at least one field to update (e.g., --desc, --label)")
	}

	// Create the FieldMask targeting the Sensor object
	mask, err := fieldmaskpb.New(sensorObj, updatePaths...)
	if err != nil {
		return fmt.Errorf("failed to create field mask: %w", err)
	}

	req := &v1.UpdateSensorRequest{
		Sensor:     sensorObj,
		UpdateMask: mask,
	}

	response, err := client.UpdateSensor(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to update sensor: %w", err)
	}

	// gRPC returns an error if it fails, so if we reach here it was a success.
	fmt.Printf("Sensor '%s' updated successfully. Updated fields: %v\n", response.Sensor.Metadata.Id, updatePaths)

	return nil
}
