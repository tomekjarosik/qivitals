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
			return runUpdate(cmd, sensorID, sensorName, description, gracefulSeconds, failureSeconds, labelsToAdd, labelsToRemove)
		},
	}

	cmd.Flags().StringVarP(&sensorID, "id", "i", "", "Unique sensor UUID to update (required)")
	cmd.Flags().StringVarP(&sensorName, "name", "n", "", "New human-readable sensor name")
	cmd.Flags().StringVar(&description, "desc", "", "New sensor description")
	cmd.Flags().Int64Var(&gracefulSeconds, "graceful", 0, "New graceful period in seconds")
	cmd.Flags().Int64Var(&failureSeconds, "failure", 0, "New failure period in seconds")
	cmd.Flags().StringArrayVarP(&labelsToAdd, "label", "l", []string{}, "Labels to add/update as key:value (can be repeated)")
	cmd.Flags().StringArrayVar(&labelsToRemove, "remove-label", []string{}, "Label keys to remove (can be repeated)")

	cmd.MarkFlagRequired("id")

	return cmd
}

func runUpdate(cmd *cobra.Command, sensorID, sensorName, description string, gracefulSeconds, failureSeconds int64, labelsToAdd, labelsToRemove []string) error {
	client, conn, err := NewStatusClient(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer conn.Close()

	// Use SensorSpec instead of SensorInfo
	sensorSpec := &v1.SensorSpec{
		Id: sensorID,
	}

	var updatePaths []string

	if cmd.Flags().Changed("name") {
		sensorSpec.Name = sensorName
		updatePaths = append(updatePaths, "name")
	}
	if cmd.Flags().Changed("desc") {
		sensorSpec.Description = description
		updatePaths = append(updatePaths, "description")
	}
	if cmd.Flags().Changed("graceful") {
		sensorSpec.GracefulPeriodSeconds = gracefulSeconds
		updatePaths = append(updatePaths, "graceful_period_seconds")
	}
	if cmd.Flags().Changed("failure") {
		sensorSpec.FailurePeriodSeconds = failureSeconds
		updatePaths = append(updatePaths, "failure_period_seconds")
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

		sensor := queryResp.Sensors[0]
		if sensor.Spec == nil {
			return fmt.Errorf("server returned invalid sensor spec")
		}

		// Build a map of existing labels from the Spec
		currentLabels := make(map[string]string)
		for _, lbl := range sensor.Spec.Labels {
			currentLabels[lbl.Key] = lbl.Value
		}

		// 2. Remove specified labels
		for _, keyToRemove := range labelsToRemove {
			delete(currentLabels, keyToRemove)
		}

		// 3. Add/Update specified labels
		for _, lblStr := range labelsToAdd {
			parts := strings.SplitN(lblStr, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid label format '%s', expected key:value", lblStr)
			}
			currentLabels[parts[0]] = parts[1]
		}

		// 4. Convert back to protobuf format
		var updatedLabels []*v1.Label
		for k, v := range currentLabels {
			updatedLabels = append(updatedLabels, &v1.Label{Key: k, Value: v})
		}

		sensorSpec.Labels = updatedLabels
		updatePaths = append(updatePaths, "labels")
	}

	if len(updatePaths) == 0 {
		return fmt.Errorf("no update fields provided. specify at least one field to update (e.g., --desc, --label)")
	}

	// Create the FieldMask using the SensorSpec
	mask, err := fieldmaskpb.New(sensorSpec, updatePaths...)
	if err != nil {
		return fmt.Errorf("failed to create field mask: %w", err)
	}

	req := &v1.UpdateSensorRequest{
		Spec:       sensorSpec, // Updated from Sensor to Spec
		UpdateMask: mask,
	}

	response, err := client.UpdateSensor(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to update sensor: %w", err)
	}

	if response.Success {
		fmt.Printf("Sensor '%s' updated successfully. Updated fields: %v\n", response.Spec.Id, updatePaths) // Updated from response.Sensor.Id
	} else {
		fmt.Printf("Failed to update sensor '%s'.\n", sensorID)
	}

	return nil
}
