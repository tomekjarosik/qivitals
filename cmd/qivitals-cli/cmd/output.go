package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v3"
)

// EmitOutput handles formatting the response based on the requested output format.
// Returns true if output was handled (skipping the default table print).
func EmitOutput(format string, response *v1.QuerySensorsResponse) error {
	if format == "" || format == "text" {
		return PrintQueryResultInTextFormat(response.Sensors)
	}

	if format == "json" {
		return PrintQueryResultInJSONFormat(response)
	}

	if format == "yaml" {
		return PrintQueryResultInYAMLFormat(response)
	}

	return fmt.Errorf("unsupported output format: %s", format)
}

func PrintQueryResultInYAMLFormat(response *v1.QuerySensorsResponse) error {
	marshaller := protojson.MarshalOptions{
		EmitUnpopulated: false,
		UseProtoNames:   true,
	}

	// Iterate through each sensor and emit a multi-document YAML stream
	for i, sensor := range response.Sensors {
		// Proto -> JSON bytes
		jsonBytes, err := marshaller.Marshal(sensor)
		if err != nil {
			return fmt.Errorf("failed to marshal sensor to JSON: %w", err)
		}

		// JSON bytes -> generic map
		var jsonObj map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &jsonObj); err != nil {
			return fmt.Errorf("failed to unmarshal JSON to map: %w", err)
		}

		// Generic map -> YAML bytes
		yamlBytes, err := yaml.Marshal(jsonObj)
		if err != nil {
			return fmt.Errorf("failed to marshal map to YAML: %w", err)
		}

		// Print with multi-document separator
		if i > 0 {
			fmt.Println("---")
		}
		fmt.Print(string(yamlBytes))
	}
	return nil
}

func PrintQueryResultInJSONFormat(response *v1.QuerySensorsResponse) error {
	marshaller := protojson.MarshalOptions{
		Multiline:       true,
		EmitUnpopulated: false, // Clean output: omits empty fields
		UseProtoNames:   true,  // Preserves snake_case field names from .proto
	}
	jsonStr, err := marshaller.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(jsonStr))
	return nil
}

func PrintQueryResultInTextFormat(sensors []*v1.Sensor) error {
	if len(sensors) == 0 {
		fmt.Println("No sensors found.")
		return nil
	}

	fmt.Printf("\nFound %d sensor(s):\n\n", len(sensors))
	fmt.Printf("%-35s%-45s%-12s%-25s\n", "NAMESPACE / NAME", "SENSOR ID", "STATUS", "LAST HEARTBEAT")
	fmt.Printf("%-35s%-45s%-12s%-25s\n", "----------------", "---------", "------", "--------------")
	for _, s := range sensors {
		state := "UNKNOWN"
		var lastUpdated int64

		if s.Status != nil {
			state = s.Status.State.String()
			lastUpdated = s.Status.LastReportedTimestamp
		}

		// Create a nice human-readable name string: namespace/name
		displayName := s.Metadata.Name
		if s.Metadata.Namespace != "" && s.Metadata.Namespace != "default" {
			displayName = s.Metadata.Namespace + "/" + s.Metadata.Name
		}

		// Truncate display name if it's too long for the column
		if len(displayName) > 33 {
			displayName = displayName[:30] + "..."
		}

		fmt.Printf("%-35s%-45s%-12s%-25s\n",
			displayName,
			s.Metadata.Id,
			state,
			timeString(lastUpdated))
	}
	fmt.Println()
	return nil
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
