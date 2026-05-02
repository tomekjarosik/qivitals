package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
)

func NewCmdReport() *cobra.Command {
	var sensorID string
	var data map[string]string

	cmd := &cobra.Command{
		Use:   "report [flags]",
		Short: "Send a health check signal from a registered sensor",
		Long: `Send a heartbeat signal to confirm a sensor is operating normally.

The timestamp of your signal keeps the sensor status as ACTIVE. If you stop sending signals for too long, the sensor will transition to DEGRADED and then DEAD based on the grace and failure periods you defined when registering it.

Examples:
  sensorcli report --id my-backup --data="status=ok" --data="records=1524"
  sensorcli report -i sensor-001 -d="cpu=45" -d="memory=72"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReport(cmd, args, sensorID, data)
		},
	}

	cmd.Flags().StringVarP(&sensorID, "id", "i", "", "Sensor identifier to report for (required)")
	cmd.Flags().StringToStringVar(&data, "data", map[string]string{}, "Key=value data pairs to send with the report (can be repeated)")
	cmd.MarkFlagRequired("id")

	return cmd
}

func runReport(cmd *cobra.Command, _ []string, sensorID string, data map[string]string) error {
	client, conn, err := NewStatusClient(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer conn.Close()

	req := &v1.ReportSensorRequest{
		Id:   sensorID,
		Data: data,
	}

	response, err := client.ReportSensor(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to report sensor data: %w", err)
	}

	fmt.Printf("Report sent successfully. Sensor: %s, Timestamp: %s\n",
		response.Sensor.Metadata.Id,
		time.Unix(response.Sensor.Status.LastOkTimestamp, 0).Format("2006-01-02 15:04:05"))

	return nil
}
