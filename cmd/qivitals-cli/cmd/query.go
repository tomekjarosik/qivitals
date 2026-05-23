package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
)

func NewCmdQuery() *cobra.Command {
	var id string
	var name string
	var namespace string
	var search string
	var statuses []string
	var labels []string
	var hasLabelKeys []string

	cmd := &cobra.Command{
		Use:   "query [flags]",
		Short: "Query sensors with advanced filtering",
		Long: `Find sensors matching the given criteria. Supports advanced 
filtering like free-text search and multiple statuses.

Examples:
  # Basic filtering
  sensorcli query --namespace home
  sensorcli query --name "water-bill"
  
  # Multiple statuses
  sensorcli query --status DEGRADED --status DEAD
  
  # Free-text search
  sensorcli query --search "backup"
  
  # Label filtering
  sensorcli query --label "env:production" --has-label "critical"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuery(cmd, args, id, name, namespace, search, statuses, labels, hasLabelKeys)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Filter by specific sensor UUID")
	cmd.Flags().StringVar(&name, "name", "", "Filter by exact sensor name")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Filter by namespace")
	cmd.Flags().StringVar(&search, "search", "", "Free-text search across name and description")

	// Changed to StringArrayVar to allow multiple statuses
	cmd.Flags().StringArrayVar(&statuses, "status", []string{}, "Filter by status: ACTIVE, DEGRADED, DEAD (can be repeated)")

	cmd.Flags().StringArrayVar(&labels, "label", []string{}, "Filter by exact label key:value pairs (can be repeated)")
	cmd.Flags().StringArrayVar(&hasLabelKeys, "has-label", []string{}, "Filter sensors that have this label key, regardless of value (can be repeated)")

	return cmd
}

func runQuery(cmd *cobra.Command, _ []string, id, name, namespace, search string, statuses, labelStrings, hasLabelKeys []string) error {
	client, conn, err := NewQiVitalsClient(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer conn.Close()

	parsedLabels, err := parseLabels(labelStrings)
	if err != nil {
		return fmt.Errorf("failed to parse labels: %w", err)
	}

	req := &v1.QuerySensorsRequest{
		Id:           id,
		Name:         name,
		Namespace:    namespace,
		Search:       search,
		Statuses:     statuses,
		Labels:       parsedLabels,
		HasLabelKeys: hasLabelKeys,
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
	fmt.Printf("%-35s%-25s%-12s%-25s\n", "NAMESPACE / NAME", "SENSOR ID", "STATUS", "LAST HEARTBEAT")
	fmt.Printf("%-35s%-25s%-12s%-25s\n", "----------------", "---------", "------", "--------------")
	for _, s := range sensors {
		state := "UNKNOWN"
		var lastUpdated int64

		if s.Status != nil {
			state = s.Status.State
			lastUpdated = s.Status.LastReportedTimestamp
		}

		// Create a nice human-readable name string: namespace/name
		displayName := s.Metadata.Name
		if s.Metadata.Namespace != "" && s.Metadata.Namespace != "default" {
			displayName = s.Metadata.Namespace + "/" + s.Metadata.Name
		}

		// Truncate UUID for cleaner table view (first 8 chars)
		shortID := s.Metadata.Id
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		// Truncate display name if it's too long for the column
		if len(displayName) > 33 {
			displayName = displayName[:30] + "..."
		}

		fmt.Printf("%-35s%-25s%-12s%-25s\n",
			displayName,
			shortID,
			state,
			timeString(lastUpdated))
	}
	fmt.Println()
}

func timeString(ts int64) string {
	if ts == 0 {
		return "never"
	}
	return ageString(ts) + " ago"
}

func ageString(ts int64) string {
	if ts == 0 {
		return "never"
	}
	return time.Since(time.Unix(ts, 0)).Round(time.Second).String()
}
