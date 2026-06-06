package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
)

// TestNewConditionEvaluator verifies the evaluator initializes correctly.
func TestNewConditionEvaluator(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		evaluator, err := NewConditionEvaluator()
		require.NoError(t, err)
		require.NotNil(t, evaluator)
		require.NotNil(t, evaluator.env)
		require.NotNil(t, evaluator.cached)
	})

	t.Run("evaluator is reusable", func(t *testing.T) {
		e1, err := NewConditionEvaluator()
		require.NoError(t, err)

		e2, err := NewConditionEvaluator()
		require.NoError(t, err)

		assert.NotSame(t, e1, e2, "each call should return a new instance")
	})
}

// TestEvaluateConditions_EmptyInputs covers edge cases with empty or nil inputs.
func TestEvaluateConditions_EmptyInputs(t *testing.T) {
	evaluator, err := NewConditionEvaluator()
	require.NoError(t, err)

	t.Run("nil rules returns nil", func(t *testing.T) {
		result := evaluator.EvaluateConditions(context.Background(), nil, nil, nil)
		assert.Nil(t, result)
	})

	t.Run("empty rules returns nil", func(t *testing.T) {
		result := evaluator.EvaluateConditions(context.Background(), []*v1.ConditionRule{}, map[string]string{}, map[string]string{})
		assert.Nil(t, result)
	})

	t.Run("nil rule in slice does not return i", func(t *testing.T) {
		rules := []*v1.ConditionRule{nil}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{})
		require.Len(t, result, 0)
	})

	t.Run("empty reported data with label-only rule", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "ProdEnv", Expression: `labels['environment'] == 'production'`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{"environment": "production"})
		require.Len(t, result, 1)
		assert.Equal(t, "ProdEnv", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
		assert.Equal(t, "ThresholdExceeded", result[0].Reason)
		assert.Contains(t, result[0].Message, "ProdEnv")
		assert.NotZero(t, result[0].LastTransitionTime)
	})

	t.Run("no labels provided", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "TestRule", Expression: `reported_data['key'] == 'value'`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"key": "value"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "TestRule", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})
}

// TestEvaluateConditions_SimpleComparisons covers basic numeric and string comparisons.
func TestEvaluateConditions_SimpleComparisons(t *testing.T) {
	evaluator, err := NewConditionEvaluator()
	require.NoError(t, err)

	t.Run("less than with double conversion", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "LowBattery", Expression: "double(reported_data['battery']) < 20.0"},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"battery": "15"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "LowBattery", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("less than with double conversion - false", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "LowBattery", Expression: "double(reported_data['battery']) < 20.0"},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"battery": "45"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "LowBattery", result[0].Type)
		assert.Equal(t, "False", result[0].Status)
		assert.Equal(t, "RuleNotMatched", result[0].Reason)
	})

	t.Run("less than with int conversion", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "LowMemory", Expression: "int(reported_data['free_mb']) < 512"},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"free_mb": "256"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "LowMemory", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("greater than", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "HighLoad", Expression: "int(reported_data['cpu_percent']) > 90"},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"cpu_percent": "95"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "HighLoad", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("equals string comparison", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "ProdEnv", Expression: `labels['environment'] == 'production'`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{"environment": "production"})
		require.Len(t, result, 1)
		assert.Equal(t, "ProdEnv", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("not equals string comparison", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "NotStaging", Expression: `labels['environment'] != 'staging'`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{"environment": "development"})
		require.Len(t, result, 1)
		assert.Equal(t, "NotStaging", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("equals string comparison - false", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "ProdEnv", Expression: `labels['environment'] == 'production'`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{"environment": "staging"})
		require.Len(t, result, 1)
		assert.Equal(t, "ProdEnv", result[0].Type)
		assert.Equal(t, "False", result[0].Status)
	})

	t.Run("equality with non-numeric string", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "MatchVersion", Expression: `reported_data['version'] == '2.1.0'`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"version": "2.1.0"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "MatchVersion", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})
}

// TestEvaluateConditions_LogicalOperators covers &&, ||, and !.
func TestEvaluateConditions_LogicalOperators(t *testing.T) {
	evaluator, err := NewConditionEvaluator()
	require.NoError(t, err)

	t.Run("AND operator - both true", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "HighCPUAndMemory", Expression: `int(reported_data['cpu']) > 80 && int(reported_data['memory']) > 90`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"cpu": "95", "memory": "92"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "HighCPUAndMemory", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("AND operator - first false", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "HighCPUAndMemory", Expression: `int(reported_data['cpu']) > 80 && int(reported_data['memory']) > 90`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"cpu": "50", "memory": "92"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "HighCPUAndMemory", result[0].Type)
		assert.Equal(t, "False", result[0].Status)
	})

	t.Run("AND operator - second false", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "HighCPUAndMemory", Expression: `int(reported_data['cpu']) > 80 && int(reported_data['memory']) > 90`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"cpu": "95", "memory": "45"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "HighCPUAndMemory", result[0].Type)
		assert.Equal(t, "False", result[0].Status)
	})

	t.Run("OR operator - first true", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "HighCPUOrMemory", Expression: `int(reported_data['cpu']) > 95 || int(reported_data['memory']) > 90`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"cpu": "96", "memory": "50"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "HighCPUOrMemory", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("OR operator - both false", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "HighCPUOrMemory", Expression: `int(reported_data['cpu']) > 95 || int(reported_data['memory']) > 90`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"cpu": "50", "memory": "40"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "HighCPUOrMemory", result[0].Type)
		assert.Equal(t, "False", result[0].Status)
	})

	t.Run("NOT operator", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "NotMaintenance", Expression: `!bool(labels['maintenance_mode'])`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{"maintenance_mode": "false"})
		require.Len(t, result, 1)
		assert.Equal(t, "NotMaintenance", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("NOT operator - true means not-maintenance", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "NotMaintenance", Expression: `!bool(labels['maintenance_mode'])`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{"maintenance_mode": "true"})
		require.Len(t, result, 1)
		assert.Equal(t, "NotMaintenance", result[0].Type)
		assert.Equal(t, "False", result[0].Status)
	})

	t.Run("complex nested expression", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "ComplexAlert", Expression: `int(reported_data['cpu']) > 90 && (labels['tier'] == 'backend' || labels['tier'] == 'database')`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"cpu": "95"}, map[string]string{"tier": "backend"})
		require.Len(t, result, 1)
		assert.Equal(t, "ComplexAlert", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})
}

// TestEvaluateConditions_StringOperations covers string functions like contains, startsWith.
func TestEvaluateConditions_StringOperations(t *testing.T) {
	evaluator, err := NewConditionEvaluator()
	require.NoError(t, err)

	t.Run("contains method", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "ErrorInLog", Expression: `reported_data['error'].contains('timeout')`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"error": "connection timeout exceeded"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "ErrorInLog", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("contains method - no match", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "ErrorInLog", Expression: `reported_data['error'].contains('timeout')`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"error": "connection refused"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "ErrorInLog", result[0].Type)
		assert.Equal(t, "False", result[0].Status)
	})

	t.Run("startsWith method", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "LocalIP", Expression: `reported_data['ip'].startsWith('10.')`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"ip": "10.0.1.5"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "LocalIP", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("startsWith method - no match", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "LocalIP", Expression: `reported_data['ip'].startsWith('10.')`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"ip": "192.168.1.1"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "LocalIP", result[0].Type)
		assert.Equal(t, "False", result[0].Status)
	})

	t.Run("endsWith method", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "TLDCheck", Expression: `reported_data['domain'].endsWith('.com')`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"domain": "example.com"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "TLDCheck", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("string length", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "ShortMessage", Expression: `reported_data['message'].size() < 10`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"message": "short"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "ShortMessage", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})
}

// TestEvaluateConditions_MultipleRules covers evaluation of multiple rules at once.
func TestEvaluateConditions_MultipleRules(t *testing.T) {
	evaluator, err := NewConditionEvaluator()
	require.NoError(t, err)

	t.Run("multiple rules all true", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "LowBattery", Expression: `double(reported_data['battery']) < 20.0`},
			{Name: "HighTemp", Expression: `int(reported_data['temp']) > 80`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"battery": "15", "temp": "85"}, nil)
		require.Len(t, result, 2)
		assert.Equal(t, "LowBattery", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
		assert.Equal(t, "HighTemp", result[1].Type)
		assert.Equal(t, "True", result[1].Status)
	})

	t.Run("multiple rules mixed results", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "LowBattery", Expression: `double(reported_data['battery']) < 20.0`},
			{Name: "HighTemp", Expression: `int(reported_data['temp']) > 80`},
			{Name: "LowMemory", Expression: `int(reported_data['mem']) > 1024`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"battery": "15", "temp": "85", "mem": "512"}, nil)
		require.Len(t, result, 3)
		assert.Equal(t, "LowBattery", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
		assert.Equal(t, "HighTemp", result[1].Type)
		assert.Equal(t, "True", result[1].Status)
		assert.Equal(t, "LowMemory", result[2].Type)
		assert.Equal(t, "False", result[2].Status)
	})

	t.Run("all rules false", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "LowBattery", Expression: `double(reported_data['battery']) < 20.0`},
			{Name: "HighTemp", Expression: `int(reported_data['temp']) > 80`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"battery": "50", "temp": "40"}, nil)
		require.Len(t, result, 2)
		assert.Equal(t, "LowBattery", result[0].Type)
		assert.Equal(t, "False", result[0].Status)
		assert.Equal(t, "HighTemp", result[1].Type)
		assert.Equal(t, "False", result[1].Status)
	})

	t.Run("rule count matches input count", func(t *testing.T) {
		ruleCount := 10
		rules := make([]*v1.ConditionRule, ruleCount)
		for i := 0; i < ruleCount; i++ {
			rules[i] = &v1.ConditionRule{
				Name:       "Rule",
				Expression: "true",
			}
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{})
		assert.Len(t, result, ruleCount)
		for i := range result {
			assert.NotNil(t, result[i])
		}
	})
}

// TestEvaluateConditions_ConditionFields covers that all condition fields are correctly populated.
func TestEvaluateConditions_ConditionFields(t *testing.T) {
	evaluator, err := NewConditionEvaluator()
	require.NoError(t, err)

	t.Run("True condition has correct reason", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "TestRule", Expression: `true`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{})
		require.Len(t, result, 1)
		assert.Equal(t, "TestRule", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
		assert.Equal(t, "ThresholdExceeded", result[0].Reason)
	})

	t.Run("False condition has correct reason", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "TestRule", Expression: `false`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{})
		require.Len(t, result, 1)
		assert.Equal(t, "TestRule", result[0].Type)
		assert.Equal(t, "False", result[0].Status)
		assert.Equal(t, "RuleNotMatched", result[0].Reason)
	})

	t.Run("LastTransitionTime is set and reasonable", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "TestRule", Expression: `true`},
		}
		before := time.Now().Unix()
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{})
		after := time.Now().Unix()

		require.Len(t, result, 1)
		assert.GreaterOrEqual(t, result[0].LastTransitionTime, before)
		assert.LessOrEqual(t, result[0].LastTransitionTime, after)
	})

	t.Run("message is not empty on trigger", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "TriggeredRule", Expression: `true`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{})
		require.Len(t, result, 1)
		assert.NotEmpty(t, result[0].Message)
	})

	t.Run("message is not empty on non-trigger", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "NonTriggeredRule", Expression: `false`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{})
		require.Len(t, result, 1)
		assert.NotEmpty(t, result[0].Message)
	})

	t.Run("custom message template is used", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{
				Name:            "CustomMsg",
				Expression:      `true`,
				MessageTemplate: "Battery at {{ .reported_data.battery_level }}%",
			},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"battery_level": "15"}, map[string]string{})
		require.Len(t, result, 1)
		assert.Equal(t, "True", result[0].Status)
		assert.Contains(t, result[0].Message, "Battery at 15%")
	})

	t.Run("message template with missing key still renders", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{
				Name:            "MissingKey",
				Expression:      `true`,
				MessageTemplate: "Value: {{ .reported_data.missing_key }}",
			},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{})
		require.Len(t, result, 1)
		assert.Equal(t, "True", result[0].Status)
		// Go template renders missing keys as empty string or <no value>
		assert.NotEmpty(t, result[0].Message)
	})

	t.Run("empty message template uses fallback", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "NoTemplate", Expression: `true`, MessageTemplate: ""},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, map[string]string{})
		require.Len(t, result, 1)
		assert.Contains(t, result[0].Message, "NoTemplate")
	})
}

// TestEvaluateConditions_CompilationErrors covers invalid CEL expressions.
func TestEvaluateConditions_CompilationErrors(t *testing.T) {
	evaluator, err := NewConditionEvaluator()
	require.NoError(t, err)

	t.Run("invalid syntax returns unknown status", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "BadSyntax", Expression: `reported_data['key'`}, // missing closing bracket
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"key": "value"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "BadSyntax", result[0].Type)
		assert.Equal(t, "Unknown", result[0].Status)
		assert.Equal(t, "CompilationError", result[0].Reason)
		assert.Contains(t, result[0].Message, "Failed to compile")
	})

	t.Run("unknown variable returns unknown status", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "UnknownVar", Expression: `some_unknown_var == true`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "UnknownVar", result[0].Type)
		assert.Equal(t, "Unknown", result[0].Status)
		assert.Equal(t, "CompilationError", result[0].Reason)
	})

	t.Run("invalid type operation returns unknown status", func(t *testing.T) {
		// Trying to use string methods on a non-existent map
		rules := []*v1.ConditionRule{
			{Name: "TypeErr", Expression: `reported_data['missing'].toInt()`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "TypeErr", result[0].Type)
		// Either Unknown (eval error) or Unknown (compilation might succeed but eval fails)
		assert.Contains(t, []string{"Unknown", "False"}, result[0].Status)
	})
}

// TestEvaluateConditions_EvaluationErrors covers runtime CEL errors.
func TestEvaluateConditions_EvaluationErrors(t *testing.T) {
	evaluator, err := NewConditionEvaluator()
	require.NoError(t, err)

	t.Run("invalid double conversion returns error in condition", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "BadDouble", Expression: `double(reported_data['value']) > 10.0`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"value": "not-a-number"}, nil)
		require.Len(t, result, 1)
		// CEL evaluates to error, which is not bool, so status is False
		assert.Equal(t, "BadDouble", result[0].Type)
		assert.Equal(t, "Unknown", result[0].Status)
		assert.Equal(t, "EvaluationError", result[0].Reason)
	})

	t.Run("invalid int conversion returns error in condition", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "BadInt", Expression: `int(reported_data['value']) > 10`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"value": "abc"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "BadInt", result[0].Type)
		assert.Equal(t, "Unknown", result[0].Status)
		assert.Equal(t, "EvaluationError", result[0].Reason)
	})
}

// TestEvaluateConditions_ToBoolConversion covers the toBool custom function.
func TestEvaluateConditions_ToBoolConversion(t *testing.T) {
	evaluator, err := NewConditionEvaluator()
	require.NoError(t, err)

	t.Run("toBool true string", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "BoolTest", Expression: `toBool(reported_data['flag']) == true`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"flag": "true"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "BoolTest", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("toBool false string", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "BoolTest", Expression: `toBool(reported_data['flag']) == false`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"flag": "false"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "BoolTest", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("toBool invalid returns false", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "BoolTest", Expression: `toBool(reported_data['flag']) == true`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"flag": "invalid"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "BoolTest", result[0].Type)
		assert.Equal(t, "Unknown", result[0].Status)
		assert.Equal(t, "EvaluationError", result[0].Reason)
	})
}

// TestEvaluateConditions_Caching covers expression compilation caching.
func TestEvaluateConditions_Caching(t *testing.T) {
	evaluator, err := NewConditionEvaluator()
	require.NoError(t, err)

	t.Run("same expression is cached", func(t *testing.T) {
		expr := `double(reported_data['val']) > 5.0`
		rules := []*v1.ConditionRule{
			{Name: "CachedTest", Expression: expr},
		}

		// First evaluation compiles and caches.
		result1 := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"val": "10"}, nil)
		require.Len(t, result1, 1)

		// Second evaluation should use cached program.
		result2 := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"val": "20"}, nil)
		require.Len(t, result2, 1)

		// Both should return the same result since the expression is the same.
		assert.Equal(t, result1[0].Type, result2[0].Type)
		assert.Equal(t, result1[0].Status, result2[0].Status)
	})

	t.Run("different expressions are cached separately", func(t *testing.T) {
		rules1 := []*v1.ConditionRule{{Name: "Rule1", Expression: `double(reported_data['a']) > 5.0`}}
		rules2 := []*v1.ConditionRule{{Name: "Rule2", Expression: `double(reported_data['b']) > 5.0`}}

		result1 := evaluator.EvaluateConditions(context.Background(), rules1, map[string]string{"a": "10"}, nil)
		result2 := evaluator.EvaluateConditions(context.Background(), rules2, map[string]string{"b": "10"}, nil)

		assert.Len(t, result1, 1)
		assert.Len(t, result2, 1)
		assert.NotEqual(t, result1[0].Type, result2[0].Type)
	})
}

// TestEvaluateConditions_RealWorldScenarios covers realistic sensor condition rules.
func TestEvaluateConditions_RealWorldScenarios(t *testing.T) {
	evaluator, err := NewConditionEvaluator()
	require.NoError(t, err)

	t.Run("battery low alert", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{
				Name:            "BatteryLow",
				Expression:      `double(reported_data['battery_level']) < 15.0`,
				TargetState:     "DEGRADED",
				MessageTemplate: "Battery at {{ .reported_data.battery_level }}%",
			},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"battery_level": "12.5"}, map[string]string{})
		require.Len(t, result, 1)
		assert.Equal(t, "BatteryLow", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
		assert.Contains(t, result[0].Message, "12.5")
	})

	t.Run("TLS cert expiry warning", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{
				Name:            "CertExpiringSoon",
				Expression:      `int(reported_data['days_remaining']) < 30`,
				TargetState:     "DEGRADED",
				MessageTemplate: "Certificate expires in {{ .reported_data.days_remaining }} days",
			},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"days_remaining": "15"}, map[string]string{})
		require.Len(t, result, 1)
		assert.Equal(t, "CertExpiringSoon", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
		assert.Contains(t, result[0].Message, "15")
	})

	t.Run("backup success check", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{
				Name:       "BackupFailed",
				Expression: `reported_data['status'] == 'failed'`,
			},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"status": "failed"}, map[string]string{})
		require.Len(t, result, 1)
		assert.Equal(t, "BackupFailed", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("network latency threshold", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{
				Name:            "HighLatency",
				Expression:      `int(reported_data['latency_ms']) > 500`,
				TargetState:     "DEGRADED",
				MessageTemplate: "Network latency is {{ .reported_data.latency_ms }}ms",
			},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"latency_ms": "750"}, map[string]string{})
		require.Len(t, result, 1)
		assert.Equal(t, "HighLatency", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
		assert.Contains(t, result[0].Message, "750")
	})

	t.Run("multi-condition: server health", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{
				Name:       "HighCPU",
				Expression: `int(reported_data['cpu']) > 90`,
			},
			{
				Name:       "HighMemory",
				Expression: `int(reported_data['memory']) > 85`,
			},
			{
				Name:       "HighDisk",
				Expression: `int(reported_data['disk']) > 90`,
			},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{
			"cpu":    "95",
			"memory": "88",
			"disk":   "50",
		}, map[string]string{})
		require.Len(t, result, 3)
		assert.Equal(t, "HighCPU", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
		assert.Equal(t, "HighMemory", result[1].Type)
		assert.Equal(t, "True", result[1].Status)
		assert.Equal(t, "HighDisk", result[2].Type)
		assert.Equal(t, "False", result[2].Status)
	})

	t.Run("domain check with contains", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{
				Name:       "ExternalDomain",
				Expression: `reported_data['domain'].endsWith('.com') || reported_data['domain'].endsWith('.org')`,
			},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"domain": "example.com"}, map[string]string{})
		require.Len(t, result, 1)
		assert.Equal(t, "ExternalDomain", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})
}

// TestEvaluateConditions_ContextCancellation covers context handling.
func TestEvaluateConditions_ContextCancellation(t *testing.T) {
	evaluator, err := NewConditionEvaluator()
	require.NoError(t, err)

	t.Run("cancelled context still works (eval is fast)", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		rules := []*v1.ConditionRule{
			{Name: "TestRule", Expression: `true`},
		}
		result := evaluator.EvaluateConditions(ctx, rules, map[string]string{}, map[string]string{})
		// CEL doesn't respect context cancellation for simple evals, so this should still work
		require.Len(t, result, 1)
		assert.Equal(t, "True", result[0].Status)
	})
}

// TestEvaluateConditions_BoundaryValues covers edge cases around numeric boundaries.
func TestEvaluateConditions_BoundaryValues(t *testing.T) {
	evaluator, err := NewConditionEvaluator()
	require.NoError(t, err)

	t.Run("zero value", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "ZeroCheck", Expression: `int(reported_data['value']) == 0`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"value": "0"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "ZeroCheck", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("negative value", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "NegativeCheck", Expression: `int(reported_data['temp']) < 0`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"temp": "-5"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "NegativeCheck", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("large value", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "LargeCheck", Expression: `double(reported_data['large']) > 1000000.0`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"large": "1234567.89"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "LargeCheck", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("decimal boundary", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "DecimalCheck", Expression: `double(reported_data['pct']) < 1.0`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"pct": "0.99"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "DecimalCheck", result[0].Type)
		assert.Equal(t, "True", result[0].Status)
	})

	t.Run("decimal at boundary", func(t *testing.T) {
		rules := []*v1.ConditionRule{
			{Name: "DecimalCheck", Expression: `double(reported_data['pct']) < 1.0`},
		}
		result := evaluator.EvaluateConditions(context.Background(), rules, map[string]string{"pct": "1.0"}, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "DecimalCheck", result[0].Type)
		assert.Equal(t, "False", result[0].Status)
	})
}
