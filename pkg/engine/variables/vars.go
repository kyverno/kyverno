package variables

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/anchor"
	"github.com/kyverno/kyverno/pkg/engine/context"
	jsonUtils "github.com/kyverno/kyverno/pkg/engine/jsonutils"
	"github.com/kyverno/kyverno/pkg/engine/operator"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/utils/jsonpointer"
)

var RegexVariables = regexp.MustCompile(`(^|[^\\])(\{\{(?:\{[^{}]*\}|[^{}])*\}\})`)

var RegexEscpVariables = regexp.MustCompile(`\\\{\{(\{[^{}]*\}|[^{}])*\}\}`)

// RegexReferences is the Regex for '$(...)' at the beginning of the string, and 'x$(...)' where 'x' is not '\'
var RegexReferences = regexp.MustCompile(`^\$\(.[^\ ]*\)|[^\\]\$\(.[^\ ]*\)`)

// RegexEscpReferences is the Regex for '\$(...)'
var RegexEscpReferences = regexp.MustCompile(`\\\$\(.[^\ ]*\)`)

var regexVariableInit = regexp.MustCompile(`^\{\{(\{[^{}]*\}|[^{}])*\}\}`)

var regexElementIndex = regexp.MustCompile(`{{\s*elementIndex\d*\s*}}`)

// IsVariable returns true if the element contains a 'valid' variable {{}}
func IsVariable(value string) bool {
	groups := RegexVariables.FindAllStringSubmatch(value, -1)
	return len(groups) != 0
}

// IsReference returns true if the element contains a 'valid' reference $()
func IsReference(value string) bool {
	groups := RegexReferences.FindAllStringSubmatch(value, -1)
	return len(groups) != 0
}

// ReplaceAllVars replaces all variables with the value defined in the replacement function
// This is used to avoid validation errors
func ReplaceAllVars(src string, repl func(string) string) string {
	wrapper := func(s string) string {
		initial := len(regexVariableInit.FindAllString(s, -1)) > 0
		prefix := ""

		if !initial {
			prefix = string(s[0])
			s = s[1:]
		}

		return prefix + repl(s)
	}

	return RegexVariables.ReplaceAllStringFunc(src, wrapper)
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
	// We must convert all incoming conditions to JSON data i.e.
	// string, []interface{}, map[string]interface{}
	// we cannot use structs otherwise json traverse doesn't work
	untypedDoc, err := DocumentToUntyped(document)
	if err != nil {
		return nil, err
	}
	return substituteAll(log, ctx, untypedDoc, newPreconditionsVariableResolver(log))
}

func SubstituteAllInType[T any](log logr.Logger, ctx context.EvalInterface, t *T) (*T, error) {
	untyped, err := DocumentToUntyped(t)
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

func SubstituteAllInRule(log logr.Logger, ctx context.EvalInterface, rule kyvernov1.Rule) (_ kyvernov1.Rule, err error) {
	result, err := SubstituteAllInType(log, ctx, &rule)
	if err != nil {
		return kyvernov1.Rule{}, err
	}

	return *result, nil
}

func DocumentToUntyped(doc interface{}) (interface{}, error) {
	jsonDoc, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	var untyped interface{}
	err = json.Unmarshal(jsonDoc, &untyped)
	if err != nil {
		return nil, err
	}

	return untyped, nil
}

func UntypedToRule(untyped interface{}) (kyvernov1.Rule, error) {
	jsonRule, err := json.Marshal(untyped)
	if err != nil {
		return kyvernov1.Rule{}, err
	}

	var rule kyvernov1.Rule
	err = json.Unmarshal(jsonRule, &rule)
	if err != nil {
		return kyvernov1.Rule{}, err
	}

	return rule, nil
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

	rule, err = DocumentToUntyped(typedRule)
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

	return UntypedToRule(rule)
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
		vars := RegexVariables.FindAllString(value, -1)
		for _, v := range vars {
			initial := len(regexVariableInit.FindAllString(v, -1)) > 0

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

func substituteReferencesIfAny(log logr.Logger) jsonUtils.Action {
	return jsonUtils.OnlyForLeafsAndKeys(func(data *jsonUtils.ActionData) (interface{}, error) {
		value, ok := data.Element.(string)
		if !ok {
			return data.Element, nil
		}

		for _, v := range RegexReferences.FindAllString(value, -1) {
			initial := v[:2] == `$(`
			old := v

			if !initial {
				v = v[1:]
			}

			resolvedReference, err := resolveReference(log, data.Document, v, data.Path)
			if err != nil {
				switch err.(type) {
				case context.InvalidVariableError:
					return nil, err
				default:
					return nil, fmt.Errorf("failed to resolve %v at path %s: %v", v, data.Path, err)
				}
			}

			if resolvedReference == nil {
				return data.Element, fmt.Errorf("got nil resolved variable %v at path %s: %v", v, data.Path, err)
			}

			log.V(3).Info("reference resolved", "reference", v, "value", resolvedReference, "path", data.Path)

			if val, ok := resolvedReference.(string); ok {
				replacement := ""

				if !initial {
					replacement = string(old[0])
				}

				replacement += val

				value = strings.Replace(value, old, replacement, 1)
				continue
			}

			return data.Element, NotResolvedReferenceError{
				reference: v,
				path:      data.Path,
			}
		}

		for _, v := range RegexEscpReferences.FindAllString(value, -1) {
			value = strings.Replace(value, v, v[1:], -1)
		}

		return value, nil
	})
}

// VariableResolver defines the handler function for variable substitution
type VariableResolver = func(ctx context.EvalInterface, variable string) (interface{}, error)

// DefaultVariableResolver is used in all variable substitutions except preconditions
func DefaultVariableResolver(ctx context.EvalInterface, variable string) (interface{}, error) {
	return ctx.Query(variable)
}

func substituteVariablesIfAny(log logr.Logger, ctx context.EvalInterface, vr VariableResolver) jsonUtils.Action {
	return jsonUtils.OnlyForLeafsAndKeys(func(data *jsonUtils.ActionData) (interface{}, error) {
		value, ok := data.Element.(string)
		if !ok {
			return data.Element, nil
		}

		isDeleteRequest := IsDeleteRequest(ctx)

		vars := RegexVariables.FindAllString(value, -1)
		for len(vars) > 0 {
			originalPattern := value
			for _, v := range vars {
				initial := len(regexVariableInit.FindAllString(v, -1)) > 0
				old := v

				if !initial {
					v = v[1:]
				}

				variable := replaceBracesAndTrimSpaces(v)

				if variable == "@" {
					pathPrefix := "target"
					if _, err := ctx.Query("target"); err != nil {
						pathPrefix = "request.object"
					}

					// Convert path to JMESPath for current identifier.
					// Skip 2 elements (e.g. mutate.overlay | validate.pattern) plus "foreach" if it is part of the pointer.
					// Prefix the pointer with pathPrefix.
					val := jsonpointer.ParsePath(data.Path).SkipPast("foreach").SkipN(2).Prepend(strings.Split(pathPrefix, ".")...).JMESPath()

					variable = strings.Replace(variable, "@", val, -1)
				}

				if isDeleteRequest {
					variable = strings.ReplaceAll(variable, "request.object", "request.oldObject")
				}

				substitutedVar, err := vr(ctx, variable)
				if err != nil {
					switch err.(type) {
					case context.InvalidVariableError, gojmespath.NotFoundError:
						return nil, err
					default:
						return nil, fmt.Errorf("failed to resolve %v at path %s: %v", variable, data.Path, err)
					}
				}

				log.V(3).Info("variable substituted", "variable", v, "value", substitutedVar, "path", data.Path)

				if originalPattern == v {
					return substitutedVar, nil
				}

				prefix := ""

				if !initial {
					prefix = string(old[0])
				}

				if value, err = substituteVarInPattern(prefix, originalPattern, v, substitutedVar); err != nil {
					return nil, fmt.Errorf("failed to resolve %v at path %s: %s", variable, data.Path, err.Error())
				}

				continue
			}

			// check for nested variables in strings
			vars = RegexVariables.FindAllString(value, -1)
		}

		for _, v := range RegexEscpVariables.FindAllString(value, -1) {
			value = strings.Replace(value, v, v[1:], -1)
		}

		return value, nil
	})
}

func IsDeleteRequest(ctx context.EvalInterface) bool {
	if ctx == nil {
		return false
	}

	operation, err := ctx.Query("request.operation")
	if err == nil && operation == "DELETE" {
		return true
	}

	return false
}

func substituteVarInPattern(prefix, pattern, variable string, value interface{}) (string, error) {
	var stringToSubstitute string

	if s, ok := value.(string); ok {
		stringToSubstitute = s
	} else {
		buffer, err := json.Marshal(value)
		if err != nil {
			return "", fmt.Errorf("failed to marshal %T: %v", value, value)
		}
		stringToSubstitute = string(buffer)
	}

	stringToSubstitute = prefix + stringToSubstitute
	variable = prefix + variable

	return strings.Replace(pattern, variable, stringToSubstitute, 1), nil
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
	for _, reference := range RegexReferences.FindAllString(value, -1) {
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
		if len(regexElementIndex.FindAllSubmatch(rawDocument, -1)) == 0 {
			break
		}

		rawDocument = regexElementIndex.ReplaceAll(rawDocument, []byte(`0`))
	}

	for {
		if len(RegexVariables.FindAllSubmatch(rawDocument, -1)) == 0 {
			break
		}

		rawDocument = RegexVariables.ReplaceAll(rawDocument, []byte(`${1}placeholderValue`))
	}

	var output interface{}
	err = json.Unmarshal(rawDocument, &output)
	if err != nil {
		logging.Error(err, "failed to unmarshall JSON", "document", string(rawDocument))
		return document
	}

	return output
}
