package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"github.com/tomekjarosik/qivitals/internal/sensors"
)

func NewCmdReport() *cobra.Command {
	var sensorID string
	var data map[string]string
	var forceGeneric bool

	cmd := &cobra.Command{
		Use:   "report [flags] [sensor_type [sensor_args...]]",
		Short: "Send a health check signal from a registered sensor",
		Long: `Send a heartbeat signal to confirm a sensor is operating normally.

The timestamp of your signal keeps the sensor status as ACTIVE. If you stop sending signals for too long, the sensor will transition to DEGRADED and then DEAD based on the grace and failure periods you defined when registering it.

Examples:
  # Generic report (manual data entry)
  qivitals-cli report --id my-backup --data="status=ok" --data="records=1524"

  # Built-in sensor
  qivitals-cli report --id my-backup zpool tank

  # Dry run - collect data without sending
  qivitals-cli --dry-run report --id my-backup zpool tank

Available sensor types: ` + getAvailableSensorsHint() + `.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReport(cmd.Context(), args, sensorID, data, forceGeneric, viper.GetBool("dry-run"))
		},
	}

	cmd.Flags().StringVarP(&sensorID, "id", "i", "", "Sensor identifier to report for (required)")
	cmd.Flags().StringToStringVar(&data, "data", map[string]string{}, "Key=value data pairs to send with the report (can be repeated)")
	cmd.Flags().BoolVarP(&forceGeneric, "generic", "g", false, "Force generic mode, even if a sensor type is specified")
	cmd.MarkFlagRequired("id")

	return cmd
}

func getAvailableSensorsHint() string {
	return strings.Join(sensors.DefaultRegistry().AvailableTypes(), ", ")
}

func runReport(ctx context.Context, args []string, sensorID string, data map[string]string, forceGeneric bool, dryRun bool) error {
	results, err := collectResults(ctx, args, data, forceGeneric)
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Println("Dry run - data collected (not sent):")
		for k, v := range results {
			fmt.Printf("  %s=%s\n", k, v)
		}
		return nil
	}

	client, conn, err := NewQiVitalsClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer conn.Close()

	req := &v1.ReportSensorRequest{Id: sensorID, Data: results}

	response, err := client.ReportSensor(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to report sensor data: %w", err)
	}

	fmt.Printf("Report sent successfully. Sensor: %s\n", response.Sensor.Metadata.Id)
	return nil
}

func collectResults(ctx context.Context, args []string, data map[string]string, forceGeneric bool) (map[string]string, error) {
	if forceGeneric || len(args) == 0 || args[0] == "generic" {
		return collectGeneric(data)
	}
	return collectSensor(ctx, args[0], args[1:])
}

func collectGeneric(data map[string]string) (map[string]string, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("generic mode requires at least one --data flag")
	}
	return data, nil
}

func collectSensor(ctx context.Context, sensorType string, args []string) (map[string]string, error) {
	sensor, err := sensors.DefaultRegistry().Create(sensorType, args...)
	if err != nil {
		return nil, fmt.Errorf("unknown sensor type %q (available: %s)",
			sensorType, strings.Join(sensors.DefaultRegistry().AvailableTypes(), ", "))
	}

	if err := sensor.Execute(ctx, args); err != nil {
		return nil, fmt.Errorf("sensor %q failed: %w", sensorType, err)
	}

	results := sensor.Results()
	if results == nil {
		results = map[string]string{}
	}
	return results, nil
}
