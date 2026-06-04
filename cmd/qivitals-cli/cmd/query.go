package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
			return runQuery(cmd, args, id, name, namespace, search, statuses, labels, hasLabelKeys, viper.GetString("output"))
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

func runQuery(cmd *cobra.Command, _ []string, id, name, namespace, search string, states, labelStrings, hasLabelKeys []string, outputFormat string) error {
	client, conn, err := NewQiVitalsClient(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer conn.Close()

	parsedLabels, err := parseLabels(labelStrings)
	if err != nil {
		return fmt.Errorf("failed to parse labels: %w", err)
	}

	protoStates, err := parseStates(states)
	if err != nil {
		return fmt.Errorf("failed to parse states: %w", err)
	}

	req := &v1.QuerySensorsRequest{
		Id:           id,
		Name:         name,
		Namespace:    namespace,
		Search:       search,
		States:       protoStates,
		Labels:       parsedLabels,
		HasLabelKeys: hasLabelKeys,
	}

	response, err := client.QuerySensors(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to query sensors: %w", err)
	}

	return EmitOutput(outputFormat, response)
}
