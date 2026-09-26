package test

import (
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/runtime"
)

func printCheckResult(
	checks []v1alpha1.CheckResult,
	responses TestResponse,
	rc *resultCounts,
	resultsTable *table.Table,
) error {
	var engineResponses []engineapi.EngineResponse
	for _, responseList := range responses.Trigger {
		engineResponses = append(engineResponses, responseList...)
	}

	testCount := len(resultsTable.RawRows) + 1

	for _, check := range checks {
		matches := engineResponses

		if check.Match.Resource != nil {
			filtered := make([]engineapi.EngineResponse, 0, len(matches))
			for _, response := range matches {
				actual := response.Resource.UnstructuredContent()
				if checkMatches(check.Match.Resource.Value, addMetadataAliases(actual)) {
					filtered = append(filtered, response)
				}
			}
			matches = filtered
		}

		if check.Match.Policy != nil {
			filtered := make([]engineapi.EngineResponse, 0, len(matches))
			for _, response := range matches {
				policy := response.Policy()
				if policy == nil {
					continue
				}

				actual, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy.AsObject())
				if err != nil {
					return fmt.Errorf("convert policy response: %w", err)
				}

				if checkMatches(check.Match.Policy.Value, addMetadataAliases(actual)) {
					filtered = append(filtered, response)
				}
			}
			matches = filtered
		}

		for _, response := range matches {
			rules := response.PolicyResponse.Rules

			if check.Match.Rule != nil {
				filtered := make([]engineapi.RuleResponse, 0, len(rules))
				for _, rule := range rules {
					actual := map[string]any{
						"name":   rule.Name(),
						"type":   string(rule.RuleType()),
						"status": string(rule.Status()),
					}
					if checkMatches(check.Match.Rule.Value, actual) {
						filtered = append(filtered, rule)
					}
				}
				rules = filtered
			}

			for _, rule := range rules {
				activation, err := buildCheckActivation(response, rule)
				if err != nil {
					return fmt.Errorf("build check activation: %w", err)
				}

				if check.Assert != nil {
					pass, message, err := evaluateAssertions(check.Assert, activation, false)
					if err != nil {
						return fmt.Errorf("evaluate check assertion: %w", err)
					}

					addCheckResultRow(
						resultsTable,
						rc,
						testCount,
						response,
						rule,
						pass,
						message,
					)
					testCount++
				}

				if check.Error != nil {
					pass, message, err := evaluateAssertions(check.Error, activation, true)
					if err != nil {
						return fmt.Errorf("evaluate check error assertion: %w", err)
					}

					addCheckResultRow(
						resultsTable,
						rc,
						testCount,
						response,
						rule,
						pass,
						message,
					)
					testCount++
				}
			}
		}
	}

	return nil
}

func buildCheckActivation(
	response engineapi.EngineResponse,
	rule engineapi.RuleResponse,
) (map[string]any, error) {
	var policy map[string]any

	if response.Policy() != nil {
		converted, err := runtime.DefaultUnstructuredConverter.ToUnstructured(response.Policy().AsObject())
		if err != nil {
			return nil, err
		}
		policy = converted
	}

	result := map[string]any{
		"name":              rule.Name(),
		"ruleType":          string(rule.RuleType()),
		"message":           rule.Message(),
		"status":            string(rule.Status()),
		"podSecurityChecks": rule.PodSecurityChecks(),
		"exceptions":        rule.Exceptions(),
		"properties":        rule.Properties(),
	}

	ruleObject := map[string]any{
		"name":       rule.Name(),
		"type":       string(rule.RuleType()),
		"status":     string(rule.Status()),
		"message":    rule.Message(),
		"properties": rule.Properties(),
	}

	return map[string]any{
		"result":   result,
		"resource": response.Resource.UnstructuredContent(),
		"policy":   policy,
		"rule":     ruleObject,
	}, nil
}

func evaluateAssertions(
	assertions *v1alpha1.CheckAssertions,
	activation map[string]any,
	negative bool,
) (bool, string, error) {
	if assertions == nil || assertions.CEL == nil {
		return true, "", nil
	}

	if len(assertions.CEL.Expressions) == 0 {
		return false, "at least one CEL expression is required", nil
	}

	for _, expression := range assertions.CEL.Expressions {
		value, err := evaluateCEL(expression.Expression, activation)
		if err != nil {
			return false, "", err
		}

		passed := value
		if negative {
			passed = !value
		}

		if !passed {
			message := expression.Message
			if message == "" {
				if negative {
					message = fmt.Sprintf("CEL error assertion unexpectedly succeeded: %s", expression.Expression)
				} else {
					message = fmt.Sprintf("CEL assertion failed: %s", expression.Expression)
				}
			}
			return false, message, nil
		}
	}

	return true, "", nil
}

func evaluateCEL(expression string, activation map[string]any) (bool, error) {
	env, err := celcompiler.NewBaseEnv()
	if err != nil {
		return false, err
	}

	env, err = env.Extend(
		cel.Variable("result", cel.DynType),
		cel.Variable("resource", cel.DynType),
		cel.Variable("policy", cel.DynType),
		cel.Variable("rule", cel.DynType),
	)
	if err != nil {
		return false, err
	}

	ast, issues := env.Compile(expression)
	if issues.Err() != nil {
		return false, issues.Err()
	}

	program, err := env.Program(ast)
	if err != nil {
		return false, err
	}

	value, _, err := program.Eval(activation)
	if err != nil {
		return false, err
	}

	result, ok := value.Value().(bool)
	if !ok {
		return false, fmt.Errorf("assertion expression %q did not return bool", expression)
	}

	return result, nil
}

func addCheckResultRow(
	resultsTable *table.Table,
	rc *resultCounts,
	id int,
	response engineapi.EngineResponse,
	rule engineapi.RuleResponse,
	pass bool,
	failureReason string,
) {
	row := table.Row{
		RowCompact: table.RowCompact{
			ID:        id,
			Policy:    color.Policy("", response.Policy().GetName()),
			Rule:      color.Rule(rule.Name()),
			Resource:  color.Resource(response.Resource.GetKind(), response.Resource.GetNamespace(), response.Resource.GetName()),
			IsFailure: !pass,
		},
		Message: rule.Message(),
	}

	if pass {
		row.Result = color.ResultPass()
		row.Reason = "Ok"
		if rule.Status() == engineapi.RuleStatusSkip {
			rc.Skip++
		} else {
			rc.Pass++
		}
	} else {
		row.Result = color.ResultFail()
		if failureReason == "" {
			failureReason = "Assertion failed"
		}
		row.Reason = failureReason
		rc.Fail++
	}

	resultsTable.Add(row)
}

func checkMatches(expected, actual any) bool {
	switch expectedValue := expected.(type) {
	case map[string]any:
		actualValue, ok := actual.(map[string]any)
		if !ok {
			return false
		}

		for key, value := range expectedValue {
			actualField, exists := actualValue[key]
			if !exists || !checkMatches(value, actualField) {
				return false
			}
		}
		return true

	case []any:
		actualValue, ok := actual.([]any)
		if !ok || len(expectedValue) != len(actualValue) {
			return false
		}

		for i := range expectedValue {
			if !checkMatches(expectedValue[i], actualValue[i]) {
				return false
			}
		}
		return true

	default:
		return reflect.DeepEqual(expected, actual)
	}
}

func addMetadataAliases(data map[string]any) map[string]any {
	result := make(map[string]any, len(data)+2)

	for key, value := range data {
		result[key] = value
	}

	if metadata, ok := data["metadata"].(map[string]any); ok {
		if name, ok := metadata["name"]; ok {
			result["name"] = name
		}
		if namespace, ok := metadata["namespace"]; ok {
			result["namespace"] = namespace
		}
	}

	return result
}
