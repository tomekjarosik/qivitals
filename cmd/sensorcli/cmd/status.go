package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
)

func NewCmdStatus() *cobra.Command {
	var sensorID string

	cmd := &cobra.Command{
		Use:   "status [flags]",
		Short: "Get the detailed status of a specific sensor",
		Long: `Show the full health details for a single sensor.

Examples:
  sensorcli status --id my-backup
  sensorcli status -i sensor-001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, args, sensorID)
		},
	}

	cmd.Flags().StringVarP(&sensorID, "id", "i", "", "Sensor ID to check (required)")
	cmd.MarkFlagRequired("id")

	return cmd
}
func runStatus(cmd *cobra.Command, _ []string, sensorID string) error {
	// Connect to the gRPC server.
	client, conn, err := NewStatusClient(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer conn.Close()

	req := &v1.QuerySensorsRequest{Id: sensorID}

	// Execute the gRPC call
	response, err := client.QuerySensors(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to query sensor: %w", err)
	}

	if len(response.Sensors) == 0 {
		return fmt.Errorf("sensor '%s' not found", sensorID)
	}

	s := response.Sensors[0]

	fmt.Printf("\nSensor: %s\n", s.Id)
	fmt.Printf("Status: %s\n", s.Status)
	fmt.Printf("Last Heartbeat: %s (%s ago)\n",
		timeString(s.Status.LastOkTimestamp),
		ageString(s.Status.LastOkTimestamp))
	fmt.Printf("Last Updated: %s\n",
		timeString(s.Status.LastUpdatedTimestamp))

	return nil
}
