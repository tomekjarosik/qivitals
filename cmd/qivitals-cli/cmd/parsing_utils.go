package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
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

func ParseExtendedDuration(s string) (time.Duration, error) {
	re := regexp.MustCompile(`^(\d+)([wd])$`)
	matches := re.FindStringSubmatch(s)

	if len(matches) == 3 {
		value, _ := strconv.Atoi(matches[1])
		unit := matches[2]

		switch unit {
		case "w":
			// 1 week = 168 hours
			s = fmt.Sprintf("%dh", value*168)
		case "d":
			// 1 day = 24 hours
			s = fmt.Sprintf("%dh", value*24)
		}
	}

	return time.ParseDuration(s)
}
