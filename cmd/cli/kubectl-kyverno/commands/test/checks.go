package test

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
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

	testCount := 1

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
				actual, err := runtime.DefaultUnstructuredConverter.ToUnstructured(response.Policy().AsObject())
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
					actual := map[string]any{"name": rule.Name()}
					if checkMatches(check.Match.Rule.Value, actual) {
						filtered = append(filtered, rule)
					}
				}
				rules = filtered
			}

			for _, rule := range rules {
				actual := map[string]any{
					"name":              rule.Name(),
					"ruleType":          rule.RuleType(),
					"message":           rule.Message(),
					"status":            string(rule.Status()),
					"podSecurityChecks": rule.PodSecurityChecks(),
					"exceptions":        rule.Exceptions(),
				}

				if check.Assert.Value != nil {
					ok, err := evaluateCheck(context.Background(), check.Assert.Value, actual)
					if err != nil {
						return fmt.Errorf("evaluate check assertion: %w", err)
					}
					addCheckResultRow(resultsTable, rc, testCount, response, rule, ok, "Assertion failed")
					testCount++
				}

				if check.Error.Value != nil {
					ok, err := evaluateCheck(context.Background(), check.Error.Value, actual)
					if err != nil {
						return fmt.Errorf("evaluate check error assertion: %w", err)
					}
					addCheckResultRow(resultsTable, rc, testCount, response, rule, !ok, "The assertion succeeded but was expected to fail")
					testCount++
				}
			}
		}
	}

	return nil
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
		row.Reason = failureReason
		rc.Fail++
	}

	resultsTable.Add(row)
}

func evaluateCheck(ctx context.Context, value any, actual map[string]any) (bool, error) {
	expected, ok := value.(map[string]any)
	if !ok {
		return false, fmt.Errorf("assertion must be an object")
	}

	for key, expectedValue := range expected {
		if strings.HasPrefix(key, "(") && strings.HasSuffix(key, ")") {
			expression := strings.TrimSpace(key[1 : len(key)-1])

			result, err := evaluateCEL(ctx, expression, actual)
			if err != nil {
				return false, err
			}

			want, ok := expectedValue.(bool)
			if !ok {
				return false, fmt.Errorf("assertion expression %q must have a boolean expected value", expression)
			}

			if result != want {
				return false, nil
			}
			continue
		}

		actualValue, exists := actual[key]
		if !exists || !checkMatches(expectedValue, actualValue) {
			return false, nil
		}
	}

	return true, nil
}

func evaluateCEL(_ context.Context, expression string, activation map[string]any) (bool, error) {
	options := make([]cel.EnvOption, 0, len(activation))
	for name := range activation {
		options = append(options, cel.Variable(name, cel.DynType))
	}

	env, err := cel.NewEnv(options...)
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
