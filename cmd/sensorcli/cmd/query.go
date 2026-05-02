package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
)

func NewCmdQuery() *cobra.Command {
	var id string
	var name string
	var namespace string
	var status string
	var labels []string

	cmd := &cobra.Command{
		Use:   "query [flags]",
		Short: "Query sensors with optional filters",
		Long: `Find sensors matching the given criteria.

Examples:
  sensorcli query --namespace home
  sensorcli query --name "water-bill"
  sensorcli query --status ACTIVE
  sensorcli query --label "env=production" --label "region=us-east"
  sensorcli query --id 550e8400-e29b-41d4-a716-446655440000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuery(cmd, args, id, name, namespace, status, labels)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Filter by specific sensor UUID")
	cmd.Flags().StringVar(&name, "name", "", "Filter by exact sensor name")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Filter by namespace")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status: ACTIVE, DEGRADED, or DEAD")
	cmd.Flags().StringArrayVar(&labels, "label", []string{}, "Filter by label key:value pairs (can be repeated)")

	return cmd
}

func runQuery(cmd *cobra.Command, _ []string, id, name, namespace, status string, labelStrings []string) error {
	client, conn, err := NewStatusClient(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer conn.Close()

	parsedLabels, err := parseLabels(labelStrings)
	if err != nil {
		return fmt.Errorf("failed to parse labels: %w", err)
	}

	// Map CLI arguments to the exact fields available in our Proto schema
	req := &v1.QuerySensorsRequest{
		Id:        id,
		Name:      name,
		Namespace: namespace,
		Status:    status,
		Labels:    parsedLabels,
	}

	response, err := client.QuerySensors(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to query sensors: %w", err)
	}

	if emitJsonFromMessage(response) {
		return nil
	}

	printQueryResult(len(response.Sensors), response.Sensors)

	return nil
}

func printQueryResult(count int, sensors []*v1.Sensor) {
	if count == 0 {
		fmt.Println("No sensors found.")
		return
	}

	fmt.Printf("\nFound %d sensor(s):\n\n", count)
	fmt.Printf("%-20s%-12s%-34s%-34s\n", "SENSOR ID", "STATUS", "LAST HEARTBEAT", "LAST UPDATED")
	fmt.Printf("%-20s%-12s%-34s%-34s\n", "---------", "------", "------------", "------------")
	for _, s := range sensors {
		// Make sure to handle the case where Status might be nil, though the server should always return it
		state := "UNKNOWN"
		var lastOk, lastUpdated int64

		if s.Status != nil {
			state = s.Status.State
			lastOk = s.Status.LastOkTimestamp
			lastUpdated = s.Status.LastUpdatedTimestamp
		}

		fmt.Printf("%-20s%-12s%-34s%-34s\n",
			s.Id,
			state,
			timeString(lastOk),
			timeString(lastUpdated))
	}
	fmt.Println()
}

func timeString(ts int64) string {
	if ts == 0 {
		return "never"
	}
	return fmt.Sprintf("%s (%s ago)",
		time.Unix(ts, 0).Format("2006-01-02 15:04:05"),
		ageString(ts))
}

func ageString(ts int64) string {
	if ts == 0 {
		return "never"
	}
	return time.Since(time.Unix(ts, 0)).Round(time.Second).String()
}
