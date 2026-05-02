package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
)

func NewCmdStatus() *cobra.Command {
	var sensorID string
	var sensorName string
	var namespace string

	cmd := &cobra.Command{
		Use:   "status [flags]",
		Short: "Get the detailed status of a specific sensor",
		Long: `Show the full health details, configuration (spec), and latest telemetry data for a single sensor.

You can look up a sensor by its unique UUID, or by its human-readable Name and Namespace.

Examples:
  sensorcli status --name "Daily Backup" --namespace db
  sensorcli status --id 550e8400-e29b-41d4-a716-446655440000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sensorID == "" && sensorName == "" {
				return fmt.Errorf("must provide either --id or --name to identify the sensor")
			}
			return runStatus(cmd, args, sensorID, sensorName, namespace)
		},
	}

	cmd.Flags().StringVarP(&sensorID, "id", "i", "", "Sensor ID to check")
	cmd.Flags().StringVarP(&sensorName, "name", "n", "", "Sensor Name to check")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Namespace of the sensor")

	return cmd
}

func runStatus(cmd *cobra.Command, _ []string, sensorID, sensorName, namespace string) error {
	client, conn, err := NewStatusClient(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer conn.Close()

	req := &v1.QuerySensorsRequest{
		Id:        sensorID,
		Name:      sensorName,
		Namespace: namespace,
	}

	// Execute the gRPC call
	response, err := client.QuerySensors(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to query sensor: %w", err)
	}

	if len(response.Sensors) == 0 {
		return fmt.Errorf("sensor not found")
	}

	s := response.Sensors[0]

	// Support machine-readable output if requested globally
	if emitJsonFromMessage(s) {
		return nil
	}

	fmt.Printf("\n--- Sensor Details ---\n")
	fmt.Printf("ID:             %s\n", s.Metadata.Id)
	fmt.Printf("Name:           %s\n", s.Metadata.Name)
	fmt.Printf("Namespace:      %s\n", s.Metadata.Namespace)
	if s.Metadata.Description != "" {
		fmt.Printf("Description:    %s\n", s.Metadata.Description)
	}

	fmt.Printf("\n--- Status ---\n")
	fmt.Printf("State:          %s\n", s.Status.State)
	fmt.Printf("Last Heartbeat: %s\n", timeString(s.Status.LastOkTimestamp))
	fmt.Printf("Last Updated:   %s\n", timeString(s.Status.LastUpdatedTimestamp))
	fmt.Printf("Grace Period:   %d seconds\n", s.Spec.GracefulPeriodSeconds)
	fmt.Printf("Failure Period: %d seconds\n", s.Spec.FailurePeriodSeconds)

	if len(s.Metadata.Labels) > 0 {
		fmt.Printf("\n--- Labels ---\n")
		for k, v := range s.Metadata.Labels {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	if len(s.Status.ReportedData) > 0 {
		fmt.Printf("\n--- Latest Telemetry Data ---\n")
		for k, v := range s.Status.ReportedData {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}
	fmt.Println()

	return nil
}
