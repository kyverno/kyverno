package variables

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/go-logr/logr"
	jsoniter "github.com/json-iterator/go"
	gojmespath "github.com/kyverno/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/anchor"
	"github.com/kyverno/kyverno/pkg/engine/context"
	jsonUtils "github.com/kyverno/kyverno/pkg/engine/jsonutils"
	"github.com/kyverno/kyverno/pkg/engine/operator"
	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/utils/jsonpointer"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// ReplaceAllVars replaces all variables with the value defined in the replacement function
// This is used to avoid validation errors
func ReplaceAllVars(src string, repl func(string) string) string {
	wrapper := func(s string) string {
		initial := len(regex.RegexVariableInit.FindAllString(s, -1)) > 0
		prefix := ""

		if !initial {
			prefix = string(s[0])
			s = s[1:]
		}

		return prefix + repl(s)
	}

	return regex.RegexVariables.ReplaceAllStringFunc(src, wrapper)
}

func newPreconditionsVariableResolver(log logr.Logger) VariableResolver {
	// PreconditionsVariableResolver is used to substitute vars in preconditions.
	// It returns an empty string if an error occurs during the substitution.
	return func(ctx context.EvalInterface, variable string) (interface{}, error) {
		value, err := DefaultVariableResolver(ctx, variable)
		if err != nil {
			log.V(4).Info(fmt.Sprintf("Variable substitution failed in preconditions, therefore nil value assigned to variable,  \"%s\" ", variable))
			return value, err
		}

		return value, nil
	}
}

// SubstituteAll substitutes variables and references in the document. The document must be JSON data
// i.e. string, []interface{}, map[string]interface{}
func SubstituteAll(log logr.Logger, ctx context.EvalInterface, document interface{}) (interface{}, error) {
	return substituteAll(log, ctx, document, DefaultVariableResolver)
}

func SubstituteAllInPreconditions(log logr.Logger, ctx context.EvalInterface, document interface{}) (interface{}, error) {
	untypedDoc, err := jsonUtils.DocumentToUntyped(document)
	if err != nil {
		return nil, err
	}
	return substituteAll(log, ctx, untypedDoc, newPreconditionsVariableResolver(log))
}

func SubstituteAllInType[T any](log logr.Logger, ctx context.EvalInterface, t *T) (*T, error) {
	untyped, err := jsonUtils.DocumentToUntyped(t)
	if err != nil {
		return nil, err
	}

	untypedResults, err := SubstituteAll(log, ctx, untyped)
	if err != nil {
		return nil, err
	}

	jsonBytes, err := json.Marshal(untypedResults)
	if err != nil {
		return nil, err
	}

	var result T
	err = json.Unmarshal(jsonBytes, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func SubstituteAllInRule(log logr.Logger, ctx context.EvalInterface, rule kyvernov1.Rule) (kyvernov1.Rule, error) {
	result, err := SubstituteAllInType(log, ctx, &rule)
	if err != nil {
		return kyvernov1.Rule{}, err
	}

	return *result, nil
}

func untypedToTyped[T any](untyped interface{}) (*T, error) {
	jsonRule, err := json.Marshal(untyped)
	if err != nil {
		return nil, err
	}

	var t T
	err = json.Unmarshal(jsonRule, &t)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

func SubstituteAllInConditions(log logr.Logger, ctx context.EvalInterface, conditions []kyvernov1.AnyAllConditions) ([]kyvernov1.AnyAllConditions, error) {
	c, err := ConditionsToJSONObject(conditions)
	if err != nil {
		return nil, err
	}

	i, err := SubstituteAll(log, ctx, c)
	if err != nil {
		return nil, err
	}

	return JSONObjectToConditions(i)
}

func ConditionsToJSONObject(conditions []kyvernov1.AnyAllConditions) ([]map[string]interface{}, error) {
	bytes, err := json.Marshal(conditions)
	if err != nil {
		return nil, err
	}

	m := []map[string]interface{}{}
	if err := json.Unmarshal(bytes, &m); err != nil {
		return nil, err
	}

	return m, nil
}

func JSONObjectToConditions(data interface{}) ([]kyvernov1.AnyAllConditions, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var c []kyvernov1.AnyAllConditions
	if err := json.Unmarshal(bytes, &c); err != nil {
		return nil, err
	}

	return c, nil
}

func substituteAll(log logr.Logger, ctx context.EvalInterface, document interface{}, resolver VariableResolver) (interface{}, error) {
	document, err := substituteReferences(log, document)
	if err != nil {
		return nil, err
	}
	return substituteVars(log, ctx, document, resolver)
}

func SubstituteAllForceMutate(log logr.Logger, ctx context.Interface, typedRule kyvernov1.Rule) (_ kyvernov1.Rule, err error) {
	var rule interface{}

	rule, err = jsonUtils.DocumentToUntyped(typedRule)
	if err != nil {
		return kyvernov1.Rule{}, err
	}

	rule, err = substituteReferences(log, rule)
	if err != nil {
		return kyvernov1.Rule{}, err
	}

	if ctx == nil {
		rule = replaceSubstituteVariables(rule)
	} else {
		rule, err = substituteVars(log, ctx, rule, DefaultVariableResolver)
		if err != nil {
			return kyvernov1.Rule{}, err
		}
	}

	result, err := untypedToTyped[kyvernov1.Rule](rule)
	if err != nil {
		return kyvernov1.Rule{}, err
	}

	return *result, nil
}

func substituteVars(log logr.Logger, ctx context.EvalInterface, rule interface{}, vr VariableResolver) (interface{}, error) {
	return jsonUtils.NewTraversal(rule, substituteVariablesIfAny(log, ctx, vr)).TraverseJSON()
}

func substituteReferences(log logr.Logger, rule interface{}) (interface{}, error) {
	return jsonUtils.NewTraversal(rule, substituteReferencesIfAny(log)).TraverseJSON()
}

func ValidateElementInForEach(log logr.Logger, rule interface{}) (interface{}, error) {
	return jsonUtils.NewTraversal(rule, validateElementInForEach(log)).TraverseJSON()
}

func validateElementInForEach(log logr.Logger) jsonUtils.Action {
	return jsonUtils.OnlyForLeafsAndKeys(func(data *jsonUtils.ActionData) (interface{}, error) {
		value, ok := data.Element.(string)
		if !ok {
			return data.Element, nil
		}
		vars := regex.RegexVariables.FindAllString(value, -1)
		for _, v := range vars {
			initial := len(regex.RegexVariableInit.FindAllString(v, -1)) > 0

			if !initial {
				v = v[1:]
			}

			variable := replaceBracesAndTrimSpaces(v)
			isElementVar := strings.HasPrefix(variable, "element") || variable == "elementIndex"
			if isElementVar && !strings.Contains(data.Path, "/foreach/") {
				return nil, fmt.Errorf("variable '%v' present outside of foreach at path %s", variable, data.Path)
			}
		}
		return nil, nil
	})
}

// NotResolvedReferenceError is returned when it is impossible to resolve the variable
type NotResolvedReferenceError struct {
	reference string
	path      string
}

func (n NotResolvedReferenceError) Error() string {
	return fmt.Sprintf("NotResolvedReferenceErr,reference %s not resolved at path %s", n.reference, n.path)
}

type refSubState int

const (
	stateRefText refSubState = iota
	stateRefStart
	stateRefName
)

func substituteReferencesIfAny(log logr.Logger) jsonUtils.Action {
	return jsonUtils.OnlyForLeafsAndKeys(func(data *jsonUtils.ActionData) (interface{}, error) {
		value, ok := data.Element.(string)
		if !ok {
			return data.Element, nil
		}

		var newValue strings.Builder
		var refNameBuilder strings.Builder
		state := stateRefText

		for _, r := range value {
			switch state {
			case stateRefText:
				if r == '$' {
					state = stateRefStart
				} else {
					newValue.WriteRune(r)
				}
			case stateRefStart:
				if r == '(' {
					state = stateRefName
				} else {
					newValue.WriteRune('$')
					newValue.WriteRune('(')
					state = stateRefText
				}
			case stateRefName:
				if r == ')' {
					refName := refNameBuilder.String()
					refName = strings.TrimSpace(refName)
					refNameBuilder.Reset()

					// ----------------- Reference Resolution -----------------
					resolvedReference, err := resolveReference(log, data.Document, refName, data.Path)
					if err != nil {
						switch err.(type) {
						case context.InvalidVariableError:
							return nil, err
						default:
							return nil, fmt.Errorf("failed to resolve %v at path %s: %v", refName, data.Path, err)
						}
					}

					if resolvedReference == nil {
						return data.Element, fmt.Errorf("got nil resolved variable %v at path %s: %v", refName, data.Path, err)
					}

					if val, ok := resolvedReference.(string); ok {
						newValue.WriteString(val)
					} else {
						return data.Element, NotResolvedReferenceError{
							reference: refName,
							path:      data.Path,
						}
					}

					// ----------------- Reference Resolution -----------------
					state = stateRefText
				} else {
					refNameBuilder.WriteRune(r)
				}
			}
		}

		return newValue.String(), nil
	})
}

// VariableResolver defines the handler function for variable substitution
type VariableResolver = func(ctx context.EvalInterface, variable string) (interface{}, error)

// DefaultVariableResolver is used in all variable substitutions except preconditions
func DefaultVariableResolver(ctx context.EvalInterface, variable string) (interface{}, error) {
	return ctx.Query(variable)
}

type varSubState int

const (
	stateVarText varSubState = iota
	stateVarStart
	stateVarStartNested
	stateVarName
	stateVarEnd
)

func substituteVariablesIfAny(log logr.Logger, ctx context.EvalInterface, vr VariableResolver) jsonUtils.Action {
	isDeleteRequest := isDeleteRequest(ctx)
	return jsonUtils.OnlyForLeafsAndKeys(func(data *jsonUtils.ActionData) (interface{}, error) {
		value, ok := data.Element.(string)
		if !ok {
			return data.Element, nil
		}

		var newValue strings.Builder
		nestedStack := make([]*strings.Builder, 0, 2)
		state := stateVarText
		dirty := false

		for i, r := range value {
			switch state {
			case stateVarText:
				if r == '{' {
					state = stateVarStart
				} else {
					newValue.WriteRune(r)
					dirty = true
				}
			case stateVarStart:
				if r == '{' {
					nestedStack = append(nestedStack, &strings.Builder{})
					state = stateVarName
				} else {
					newValue.WriteRune('{')
					newValue.WriteRune(r)
					state = stateVarText
					dirty = true
				}
			case stateVarName:
				if r == '{' {
					state = stateVarStartNested
				} else if r == '}' {
					state = stateVarEnd
				} else {
					nestedStack[len(nestedStack)-1].WriteRune(r)
				}
			case stateVarStartNested:
				if r == '{' {
					nestedStack = append(nestedStack, &strings.Builder{})
				} else {
					nestedStack[len(nestedStack)-1].WriteRune('{')
					nestedStack[len(nestedStack)-1].WriteRune(r)
				}
				state = stateVarName
			case stateVarEnd:
				if r == '}' {
					varName := nestedStack[len(nestedStack)-1].String()
					varName = strings.TrimSpace(varName)

					// ------------------ Variable Resolution ------------------
					if varName == "@" {
						pathPrefix := "target"
						if _, err := ctx.Query("target"); err != nil {
							pathPrefix = "request.object"
						}

						// Convert path to JMESPath for current identifier.
						// Skip 2 elements (e.g. mutate.overlay | validate.pattern) plus "foreach" if it is part of the pointer.
						// Prefix the pointer with pathPrefix.
						val := jsonpointer.ParsePath(data.Path).SkipPast("foreach").SkipN(2).Prepend(strings.Split(pathPrefix, ".")...).JMESPath()

						varName = strings.Replace(varName, "@", val, -1)
					}

					if isDeleteRequest {
						varName = strings.ReplaceAll(varName, "request.object", "request.oldObject")
					}

					substitutedVar, err := vr(ctx, varName)
					if err != nil {
						switch err.(type) {
						case context.InvalidVariableError, gojmespath.NotFoundError:
							return nil, err
						default:
							return nil, fmt.Errorf("failed to resolve %v at path %s: %v", varName, data.Path, err)
						}
					}

					// ------------------ Variable Resolution ------------------

					if len(nestedStack) > 1 { // Nested variable
						varValue := fmt.Sprintf("%v", substitutedVar)
						nestedStack = nestedStack[:len(nestedStack)-1]
						nestedStack[len(nestedStack)-1].WriteString(varValue)
						state = stateVarName
					} else {
						if !dirty && i == len(value)-1 {
							return substitutedVar, nil
						}

						varValue := fmt.Sprintf("%v", substitutedVar)
						newValue.WriteString(varValue)
						state = stateVarText
					}
				} else {
					nestedStack[len(nestedStack)-1].WriteRune('}')
					nestedStack[len(nestedStack)-1].WriteRune(r)
					state = stateVarName
				}
			}
		}

		// TODO: Handle any remaining characters or unterminated variables

		return newValue.String(), nil
	})
}

func isDeleteRequest(ctx context.EvalInterface) bool {
	if ctx == nil {
		return false
	}

	if op := ctx.QueryOperation(); op != "" {
		return op == "DELETE"
	}

	return false
}

func replaceBracesAndTrimSpaces(v string) string {
	variable := strings.ReplaceAll(v, "{{", "")
	variable = strings.ReplaceAll(variable, "}}", "")
	variable = strings.TrimSpace(variable)
	return variable
}

func resolveReference(log logr.Logger, fullDocument interface{}, reference, absolutePath string) (interface{}, error) {
	var foundValue interface{}

	path := strings.Trim(reference, "$()")

	operation := operator.GetOperatorFromStringPattern(path)
	path = path[len(operation):]

	if len(path) == 0 {
		return nil, errors.New("expected path, found empty reference")
	}

	path = formAbsolutePath(path, absolutePath)

	valFromReference, err := getValueFromReference(fullDocument, path)
	if err != nil {
		return err, nil
	}

	if operation == operator.Equal { // if operator does not exist return raw value
		return valFromReference, nil
	}

	foundValue, err = valFromReferenceToString(valFromReference, string(operation))
	if err != nil {
		return "", err
	}

	return string(operation) + foundValue.(string), nil
}

// Parse value to string
func valFromReferenceToString(value interface{}, operator string) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case int, int64:
		return fmt.Sprintf("%d", value), nil
	case float64:
		return fmt.Sprintf("%f", value), nil
	default:
		return "", fmt.Errorf("incorrect expression: operator %s does not match with value %v", operator, value)
	}
}

func FindAndShiftReferences(log logr.Logger, value, shift, pivot string) string {
	for _, reference := range regex.RegexReferences.FindAllString(value, -1) {
		initial := reference[:2] == `$(`
		oldReference := reference

		if !initial {
			reference = reference[1:]
		}

		index := strings.Index(reference, pivot)
		if index == -1 {
			log.Error(fmt.Errorf(`failed to shift reference: pivot value "%s" was not found`, pivot), "pivot search failed")
		}

		// try to get rule index from the reference
		if pivot == "anyPattern" {
			ruleIndex := strings.Split(reference[index+len(pivot)+1:], "/")[0]
			pivot = pivot + "/" + ruleIndex
		}

		shiftedReference := strings.Replace(reference, pivot, pivot+"/"+shift, -1)
		replacement := ""

		if !initial {
			replacement = string(oldReference[0])
		}

		replacement += shiftedReference

		value = strings.Replace(value, oldReference, replacement, 1)
	}

	return value
}

func formAbsolutePath(referencePath, absolutePath string) string {
	if path.IsAbs(referencePath) {
		return referencePath
	}

	return path.Join(absolutePath, referencePath)
}

func getValueFromReference(fullDocument interface{}, path string) (interface{}, error) {
	var element interface{}

	if _, err := jsonUtils.NewTraversal(fullDocument, jsonUtils.OnlyForLeafsAndKeys(
		func(data *jsonUtils.ActionData) (interface{}, error) {
			if anchor.RemoveAnchorsFromPath(data.Path) == path {
				element = data.Element
			}

			return data.Element, nil
		})).TraverseJSON(); err != nil {
		return nil, err
	}

	return element, nil
}

func replaceSubstituteVariables(document interface{}) interface{} {
	rawDocument, err := json.Marshal(document)
	if err != nil {
		return document
	}

	for {
		if len(regex.RegexElementIndex.FindAllSubmatch(rawDocument, -1)) == 0 {
			break
		}

		rawDocument = regex.RegexElementIndex.ReplaceAll(rawDocument, []byte(`0`))
	}

	for {
		if len(regex.RegexVariables.FindAllSubmatch(rawDocument, -1)) == 0 {
			break
		}

		rawDocument = regex.RegexVariables.ReplaceAll(rawDocument, []byte(`${1}placeholderValue`))
	}

	var output interface{}
	err = json.Unmarshal(rawDocument, &output)
	if err != nil {
		logging.Error(err, "failed to unmarshall JSON", "document", string(rawDocument))
		return document
	}

	return output
}
