package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
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

func parseConditionRule(s string) (*v1.ConditionRule, error) {
	parts := strings.SplitN(s, ":", 4)
	if len(parts) < 2 {
		return nil, fmt.Errorf("condition rule must have at least 'name:expression', got %q", s)
	}
	rule := &v1.ConditionRule{
		Name:       parts[0],
		Expression: parts[1],
	}
	if len(parts) >= 3 {
		rule.TargetState = parts[2]
	}
	if len(parts) == 4 {
		rule.MessageTemplate = parts[3]
	}
	return rule, nil
}

// parseConditionRules parses a slice of condition rule strings.
func parseConditionRules(rules []string) ([]*v1.ConditionRule, error) {
	result := make([]*v1.ConditionRule, 0, len(rules))
	for _, s := range rules {
		rule, err := parseConditionRule(s)
		if err != nil {
			return nil, fmt.Errorf("invalid condition rule %q: %w", s, err)
		}
		result = append(result, rule)
	}
	return result, nil
}

func parseStates(states []string) ([]v1.SensorState, error) {
	if len(states) == 0 {
		return nil, nil
	}

	result := make([]v1.SensorState, 0, len(states))
	for _, s := range states {
		key := strings.ToUpper(strings.TrimSpace(s))
		val, ok := v1.SensorState_value[key]
		if !ok {
			return nil, fmt.Errorf("unknown sensor state %q", s)
		}
		result = append(result, v1.SensorState(val))
	}

	return result, nil
}
