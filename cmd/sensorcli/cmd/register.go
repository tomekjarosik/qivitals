package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
)

func NewCmdRegister() *cobra.Command {
	var (
		sensorID        string
		namespace       string
		sensorName      string
		description     string
		gracefulSeconds int64
		failureSeconds  int64
		labels          []string
	)

	cmd := &cobra.Command{
		Use:   "register [flags]",
		Short: "Register a new sensor with the status service",
		Long: `Register a new sensor and start monitoring its health status.

Sensors are uniquely identified by humans using a combination of their Namespace and Name. 
If you do not provide an explicit --id, the server will automatically generate a UUID for you.

Examples:
  sensorcli register --namespace db --name "Daily Backup" --label "env=production" --graceful 300 --failure 600
  sensorcli register -ns frontend -n "Payment-Service" -l "tier=1"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegister(cmd, args, sensorID, namespace, sensorName, description, gracefulSeconds, failureSeconds, labels)
		},
	}

	cmd.Flags().StringVarP(&sensorID, "id", "i", "", "Optional unique sensor UUID (server generates if empty)")
	cmd.Flags().StringVar(&namespace, "namespace", "default", "Logical grouping for the sensor")
	cmd.Flags().StringVarP(&sensorName, "name", "n", "", "Human-readable sensor name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Optional sensor description")
	cmd.Flags().Int64Var(&gracefulSeconds, "graceful", 300, "Seconds before showing DEGRADED status")
	cmd.Flags().Int64Var(&failureSeconds, "failure", 900, "Seconds before showing DEAD status")
	cmd.Flags().StringArrayVar(&labels, "label", []string{}, "Labels as key:value pairs (can be repeated)")

	cmd.MarkFlagRequired("name")

	return cmd
}

func runRegister(cmd *cobra.Command, _ []string, sensorID, namespace, sensorName, description string, gracefulSeconds, failureSeconds int64, labelStrings []string) error {
	parsedLabels, err := parseLabels(labelStrings)
	if err != nil {
		return fmt.Errorf("failed to parse labels: %w", err)
	}

	client, conn, err := NewStatusClient(cmd.Context()) // Assuming you have a helper for this now!
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer conn.Close()

	// Use the new KRM structure (Sensor -> Metadata + Spec)
	req := &v1.RegisterSensorRequest{
		Sensor: &v1.Sensor{
			Metadata: &v1.ObjectMeta{
				Id:          sensorID,
				Namespace:   namespace,
				Name:        sensorName,
				Description: description,
				Labels:      parsedLabels,
			},
			Spec: &v1.SensorSpec{
				GracefulPeriodSeconds: gracefulSeconds,
				FailurePeriodSeconds:  failureSeconds,
			},
		},
	}

	response, err := client.RegisterSensor(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to register sensor: %w", err)
	}

	// In gRPC, returning an object implies success (errors are thrown via err != nil)
	fmt.Printf("Sensor registered successfully. ID: %s\n", response.Sensor.Metadata.Id)

	return nil
}
