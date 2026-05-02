package cmd

import (
	"fmt"
	"strings"
)

// parseLabels takes a slice of "key=value" strings from the CLI flags
// and converts them into a standard Go map to match the protobuf schema.
func parseLabels(labelStrings []string) (map[string]string, error) {
	// If no labels were provided, return an empty map instead of nil,
	// so the caller doesn't have to worry about nil-pointer panics.
	if len(labelStrings) == 0 {
		return make(map[string]string), nil
	}

	parsedLabels := make(map[string]string)

	for _, label := range labelStrings {
		parts := strings.SplitN(label, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid label format: %s. Expected format is key=value", label)
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if key == "" {
			return nil, fmt.Errorf("invalid label format: %s. Key cannot be empty", label)
		}

		parsedLabels[key] = val
	}

	return parsedLabels, nil
}
