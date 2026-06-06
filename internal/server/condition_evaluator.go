package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
)

// ConditionEvaluator handles CEL expression evaluation for sensor conditions.
type ConditionEvaluator struct {
	mu           sync.RWMutex
	env          *cel.Env
	cached       map[string]cel.Program
	templates    map[string]*template.Template
	maxCacheSize int
}

// NewConditionEvaluator creates a new CEL evaluator with sensor-specific functions.
func NewConditionEvaluator() (*ConditionEvaluator, error) {
	celEnv, err := cel.NewEnv(
		cel.Variable("reported_data", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("labels", cel.MapType(cel.StringType, cel.StringType)),
		cel.Function("toDouble",
			cel.Overload("toDouble_string",
				[]*cel.Type{cel.StringType}, cel.DoubleType,
				cel.UnaryBinding(unaryDouble)),
		),
		cel.Function("toInt",
			cel.Overload("toInt_string",
				[]*cel.Type{cel.StringType}, cel.IntType,
				cel.UnaryBinding(unaryInt)),
		),
		cel.Function("toBool",
			cel.Overload("toBool_string",
				[]*cel.Type{cel.StringType}, cel.BoolType,
				cel.UnaryBinding(unaryToBool)),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return &ConditionEvaluator{
		env:          celEnv,
		cached:       make(map[string]cel.Program),
		templates:    make(map[string]*template.Template),
		maxCacheSize: 1000, // Prevent unbounded memory growth
	}, nil
}

// EvaluateConditions evaluates all condition rules against the given sensor data.
func (e *ConditionEvaluator) EvaluateConditions(
	ctx context.Context,
	rules []*v1.ConditionRule,
	reportedData map[string]string,
	labels map[string]string,
) []*v1.Condition {
	if len(rules) == 0 {
		return nil
	}

	conditions := make([]*v1.Condition, 0, len(rules))
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		conditions = append(conditions, e.evaluateSingleRule(rule, reportedData, labels))
	}
	return conditions
}

// evaluateSingleRule evaluates a single condition rule and returns the result.
func (e *ConditionEvaluator) evaluateSingleRule(
	rule *v1.ConditionRule,
	reportedData map[string]string,
	labels map[string]string,
) *v1.Condition {
	if rule == nil {
		return nil
	}

	prog, err := e.compileExpression(rule.Expression)
	if err != nil {
		return &v1.Condition{
			Type:               rule.Name,
			Status:             "Unknown",
			Reason:             "CompilationError",
			Message:            fmt.Sprintf("Failed to compile CEL expression: %v", err),
			LastTransitionTime: time.Now().Unix(),
		}
	}

	// CEL can accept native Go maps directly when the variable is declared
	// as map<string, string>, so no need to convert each value to ref.Val.
	out, _, err := prog.Eval(map[string]interface{}{
		"reported_data": reportedData,
		"labels":        labels,
	})
	if err != nil {
		return &v1.Condition{
			Type:               rule.Name,
			Status:             "Unknown",
			Reason:             "EvaluationError",
			Message:            fmt.Sprintf("CEL evaluation failed: %v", err),
			LastTransitionTime: time.Now().Unix(),
		}
	}

	// Check if the result is a boolean and evaluates to true.
	isTrue := false
	if b, ok := out.Value().(bool); ok {
		isTrue = b
	}

	var status, reason, message string
	if isTrue {
		status = "True"
		reason = "ThresholdExceeded"
		message = e.formatMessage(rule.MessageTemplate, reportedData)
		if message == "" {
			message = fmt.Sprintf("Condition '%s' triggered", rule.Name)
		}
	} else {
		status = "False"
		reason = "RuleNotMatched"
		message = fmt.Sprintf("Condition '%s' did not match", rule.Name)
	}

	return &v1.Condition{
		Type:               rule.Name,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: time.Now().Unix(),
	}
}

// getTemplate safely parses and caches Go templates to avoid repeated regex overhead.
func (e *ConditionEvaluator) getTemplate(tmplStr string) (*template.Template, error) {
	e.mu.RLock()
	if t, ok := e.templates[tmplStr]; ok {
		e.mu.RUnlock()
		return t, nil
	}
	e.mu.RUnlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	if t, ok := e.templates[tmplStr]; ok {
		return t, nil
	}

	// Evict oldest entry if cache is full (simple LRU-ish behavior by dropping random old keys)
	if len(e.templates) >= e.maxCacheSize {
		for k := range e.templates {
			delete(e.templates, k)
			break
		}
	}

	t, err := template.New("condition_msg").Parse(tmplStr)
	if err != nil {
		return nil, err
	}
	e.templates[tmplStr] = t
	return t, nil
}

// formatMessage formats the message template with the reported data.
func (e *ConditionEvaluator) formatMessage(templateStr string, reportedData map[string]string) string {
	if templateStr == "" {
		return ""
	}

	tmpl, err := e.getTemplate(templateStr)
	if err != nil {
		return fmt.Sprintf("Error formatting message: %v", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, map[string]map[string]string{
		"reported_data": reportedData,
	}); err != nil {
		return templateStr
	}
	return buf.String()
}

// compileExpression compiles a CEL expression and caches it by expression string.
func (e *ConditionEvaluator) compileExpression(expr string) (cel.Program, error) {
	e.mu.RLock()
	if prog, ok := e.cached[expr]; ok {
		e.mu.RUnlock()
		return prog, nil
	}
	e.mu.RUnlock()

	e.mu.Lock()
	defer e.mu.Unlock()

	// Double-check after acquiring the write lock.
	if prog, ok := e.cached[expr]; ok {
		return prog, nil
	}

	ast, iss := e.env.Compile(expr)
	if iss.Err() != nil {
		return nil, fmt.Errorf("CEL compile error for %q: %w", expr, iss.Err())
	}

	prog, err := e.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("CEL program creation error for %q: %w", expr, err)
	}

	e.cached[expr] = prog
	return prog, nil
}

// --- CEL helper functions ---

// unaryDouble converts a CEL string value to a CEL double via strconv.
func unaryDouble(val ref.Val) ref.Val {
	str, ok := val.Value().(string)
	if !ok {
		return types.NewErr("toDouble: expected string, got %T", val.Value())
	}
	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return types.NewErr("toDouble: cannot parse %q as double: %v", str, err)
	}
	return types.Double(f)
}

// unaryInt converts a CEL string value to a CEL int via strconv.
func unaryInt(val ref.Val) ref.Val {
	str, ok := val.Value().(string)
	if !ok {
		return types.NewErr("toInt: expected string, got %T", val.Value())
	}
	i, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return types.NewErr("toInt: cannot parse %q as int: %v", str, err)
	}
	return types.Int(i)
}

// unaryToBool converts a CEL string value to a CEL bool via strconv.
func unaryToBool(val ref.Val) ref.Val {
	str, ok := val.Value().(string)
	if !ok {
		return types.NewErr("toBool: expected string, got %T", val.Value())
	}
	b, err := strconv.ParseBool(str)
	if err != nil {
		return types.NewErr("toBool: cannot parse %q as bool: %v", str, err)
	}
	return types.Bool(b)
}
