package cmd

import (
	"fmt"
	"strings"

	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
)

func parseLabels(labelStrings []string) ([]*v1.Label, error) {
	var parsedLabels []*v1.Label // Initializes as a nil slice
	for _, label := range labelStrings {
		parts := strings.SplitN(label, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid label format: %s", label)
		}
		parsedLabels = append(parsedLabels, &v1.Label{
			Key:   parts[0],
			Value: parts[1],
		})
	}
	return parsedLabels, nil
}
