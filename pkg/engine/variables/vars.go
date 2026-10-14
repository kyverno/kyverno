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
	return jsonUtils.NewTraversal(rule, validateElementInForEach()).TraverseJSON()
}

func validateElementInForEach() jsonUtils.Action {
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

			variable, _ := replaceBracesAndTrimSpaces(v)
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

		for _, v := range regex.RegexReferences.FindAllString(value, -1) {
			initial := v[:2] == `$(`
			old := v

			if !initial {
				v = v[1:]
			}

			resolvedReference, err := resolveReference(data.Document, v, data.Path)
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

		for _, v := range regex.RegexEscpReferences.FindAllString(value, -1) {
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

// substSpan marks a byte range in the value under construction that must not
// be re-scanned for variable delimiters, because it was introduced by a
// resolved value rather than written by the policy author. An alias span is
// the one exception: it holds exactly one "{{ ... }}" token that a resolved
// value returned verbatim (a variable-to-variable indirection), and it stays
// eligible for one further resolution at its own starting offset, carrying
// the set of variable names already visited along that indirection chain so
// a cycle is caught instead of retried forever.
type substSpan struct {
	start, end int
	alias      bool
	visited    map[string]struct{}
}

// spanContaining returns the span (if any) covering byte offset pos.
// Spans are kept non-overlapping, so at most one can contain pos.
func spanContaining(spans []substSpan, pos int) *substSpan {
	for i := range spans {
		if spans[i].start <= pos && pos < spans[i].end {
			return &spans[i]
		}
	}
	return nil
}

// adjustSpans reflects a splice of value[start:end] with a replacement delta
// bytes longer (or shorter) into the existing span bookkeeping: spans wholly
// before the splice are untouched, spans at or after it shift by delta, and
// the portion of any span that falls inside the replaced range is consumed
// by the splice. No existing span can start inside [start, end) other than
// exactly at start, because that would have made the token ineligible for
// substitution in the first place.
func adjustSpans(spans []substSpan, start, end, delta int) []substSpan {
	adjusted := make([]substSpan, 0, len(spans))
	for _, s := range spans {
		switch {
		case s.end <= start:
			adjusted = append(adjusted, s)
		case s.start >= end:
			adjusted = append(adjusted, substSpan{start: s.start + delta, end: s.end + delta, alias: s.alias, visited: s.visited})
		case s.end <= end:
			// entirely consumed by the splice
		default:
			// straddles the splice boundary; the surviving tail shifts
			adjusted = append(adjusted, substSpan{start: end + delta, end: s.end + delta, alias: s.alias, visited: s.visited})
		}
	}
	return adjusted
}

// isAliasValue reports whether a resolved-and-spliced value is exactly one
// balanced "{{ ... }}" token and nothing else, the shape that makes it a
// variable-to-variable indirection rather than arbitrary reintroduced text.
func isAliasValue(v string) bool {
	return v != "" && regex.RegexVariableInit.FindString(v) == v
}

// maxTotalSubstitutions is a hard backstop on the number of substitutions a
// single document may require, independent of budget's alias-replenishment
// and the visited-set cycle detection (see the comment on totalSubs in
// substituteVariablesIfAny). It is counted across every leaf of one
// SubstituteAll traversal, not per leaf, so many small leaves cannot
// collectively exceed it. It is sized far above any legitimate nesting --
// real policies nest a handful of variables deep -- so it only fires as a
// defense-in-depth backstop, not a working-policy limit. Kept in the
// thousands rather than larger: each alias hop's visited-set carries its own
// copy of every name seen so far, so an n-hop chain costs O(n^2); a tighter
// cap here bounds that work too, not just the hop count.
const maxTotalSubstitutions = 5_000

// maxSubstitutedOutputBytes caps the total bytes variable substitution may
// produce across one SubstituteAll traversal. The count cap above bounds how
// many substitutions run, but not how large each result is: a few thousand
// substitutions each splicing in a multi-megabyte value stay within the count
// budget while expanding the document into gigabytes. This bounds the output
// itself. The anchor is the request size an admission webhook accepts: the
// apiserver default --max-request-body-bytes is 3 MiB and the etcd object
// limit is ~1.5 MiB, so 10 MiB is roughly 3x the largest legitimate single
// request while still refusing gigabyte-scale blowups.
const maxSubstitutedOutputBytes = 10 << 20

// substituteVariablesIfAny resolves "{{ ... }}" variable references in a
// leaf string.
//
// Within a substitution pass, variable delimiters are honored from the text
// the pass started with, and from a resolved value that is exactly one
// variable reference (an indirection, resolved further; cyclic references
// are errors). Any other text introduced by a resolved value is treated as
// data and is not re-scanned for delimiters in that pass.
func substituteVariablesIfAny(log logr.Logger, ctx context.EvalInterface, lookupVar VariableResolver) jsonUtils.Action {
	isDeleteRequest := isDeleteRequest(ctx)

	// totalSubs and totalOutputBytes are per-document backstops shared across
	// every leaf of this traversal. substituteVariablesIfAny is called once per
	// SubstituteAll and returns a fresh Action each time, so these never leak
	// across requests; the traversal visits leaves sequentially, so the shared
	// counters need no synchronization.
	totalSubs := 0
	totalOutputBytes := 0

	return jsonUtils.OnlyForLeafsAndKeys(func(data *jsonUtils.ActionData) (interface{}, error) {
		value, ok := data.Element.(string)
		if !ok {
			return data.Element, nil
		}

		var spans []substSpan
		// budget bounds the number of substitutions this leaf can require: it
		// starts at the number of "{{" the author literally wrote, decreases
		// by one per substitution performed, and increases by one whenever a
		// substitution introduces an alias span (a resolved value that is
		// itself exactly one token, which then needs its own resolution).
		// Because every other kind of introduced text is protected and never
		// re-scanned, this count is exact, not a heuristic cap.
		budget := strings.Count(value, "{{")

		// Each pass scans the string once and resolves every currently eligible
		// token, rather than resolving one token and rescanning. Resolving all
		// of a pass's tokens is equivalent to the one-at-a-time form -- a token's
		// resolved value depends only on the context and the variable name, never
		// on a sibling token -- but it turns an O(tokens x length) loop into one
		// bounded by alias-nesting depth, not token count. Tokens are resolved
		// and accounted left-to-right (so the budget and output-size errors name
		// the same token the one-at-a-time form did), then spliced right-to-left
		// so each splice leaves the earlier tokens' offsets untouched.
		type pending struct {
			g2start, g2end int
			repl           string
			alias          bool
			visited        map[string]struct{}
			variable       string
		}

		for {
			matches := regex.RegexVariables.FindAllStringSubmatchIndex(value, -1)

			var batch []pending
			for _, m := range matches {
				g2start, g2end := m[4], m[5]
				var visited map[string]struct{}
				s := spanContaining(spans, g2start)
				switch {
				case s == nil:
					visited = map[string]struct{}{}
				case s.alias && s.start == g2start:
					visited = s.visited
				default:
					continue
				}

				v := value[g2start:g2end]

				variable, shallowSubstitution := replaceBracesAndTrimSpaces(v)
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

				if _, cyclic := visited[variable]; cyclic {
					return nil, fmt.Errorf("cyclic variable reference: %s at path %s", variable, data.Path)
				}

				substitutedVar, err := lookupVar(ctx, variable)
				if err != nil {
					switch err.(type) {
					case context.InvalidVariableError, gojmespath.NotFoundError:
						return nil, err
					default:
						return nil, fmt.Errorf("failed to resolve %v at path %s: %v", variable, data.Path, err)
					}
				}

				log.V(3).Info("variable substituted", "variable", v, "value", substitutedVar, "path", data.Path)

				if g2start == 0 && g2end == len(value) {
					// A leaf that is exactly "{{ var }}" returns the resolved value
					// as-is to preserve its type, but it must still count against the
					// per-document budgets. Otherwise many such leaves, each resolving
					// a large value, expand the marshalled document without bound while
					// the embedded path below is capped.
					outBytes := 0
					if s, ok := substitutedVar.(string); ok {
						outBytes = len(s)
					} else if substitutedVar != nil {
						buffer, mErr := json.Marshal(substitutedVar)
						if mErr != nil {
							return nil, fmt.Errorf("failed to resolve %v at path %s: failed to marshal %T: %v", variable, data.Path, substitutedVar, substitutedVar)
						}
						outBytes = len(buffer)
					}

					totalSubs++
					if totalSubs > maxTotalSubstitutions {
						return nil, fmt.Errorf("failed to resolve %v at path %s: exceeded the hard limit on total variable substitutions", variable, data.Path)
					}

					totalOutputBytes += outBytes
					if totalOutputBytes > maxSubstitutedOutputBytes {
						return nil, fmt.Errorf("failed to resolve %v at path %s: exceeded the maximum substituted output size of %d bytes", variable, data.Path, maxSubstitutedOutputBytes)
					}

					return substitutedVar, nil
				}

				_, substitutedIsString := substitutedVar.(string)

				if shallowSubstitution && substitutedVar != nil {
					shallowStr, ok := substitutedVar.(string)
					if !ok {
						buffer, mErr := json.Marshal(substitutedVar)
						if mErr != nil {
							return nil, fmt.Errorf("failed to resolve %v at path %s: failed to marshal %T: %v", variable, data.Path, substitutedVar, substitutedVar)
						}
						shallowStr = string(buffer)
					}
					substitutedVar = strings.ReplaceAll(shallowStr, "{{", "\\{{")
				}

				var repl string
				if s, ok := substitutedVar.(string); ok {
					repl = s
				} else {
					buffer, mErr := json.Marshal(substitutedVar)
					if mErr != nil {
						return nil, fmt.Errorf("failed to resolve %v at path %s: failed to marshal %T: %v", variable, data.Path, substitutedVar, substitutedVar)
					}
					repl = string(buffer)
				}

				if budget == 0 {
					return nil, fmt.Errorf("failed to resolve %v at path %s: exceeded the maximum number of variable substitutions", variable, data.Path)
				}
				budget--

				totalSubs++
				if totalSubs > maxTotalSubstitutions {
					return nil, fmt.Errorf("failed to resolve %v at path %s: exceeded the hard limit on total variable substitutions", variable, data.Path)
				}

				// Bound the total output produced across the document, not just
				// the number of substitutions: one value spliced in can be huge.
				totalOutputBytes += len(repl)
				if totalOutputBytes > maxSubstitutedOutputBytes {
					return nil, fmt.Errorf("failed to resolve %v at path %s: exceeded the maximum substituted output size of %d bytes", variable, data.Path, maxSubstitutedOutputBytes)
				}

				p := pending{g2start: g2start, g2end: g2end, repl: repl, variable: variable}
				if substitutedIsString && !shallowSubstitution && isAliasValue(repl) {
					newVisited := make(map[string]struct{}, len(visited)+1)
					for k := range visited {
						newVisited[k] = struct{}{}
					}
					newVisited[variable] = struct{}{}
					p.alias = true
					p.visited = newVisited
					budget++
				}
				batch = append(batch, p)
			}

			if len(batch) == 0 {
				break
			}

			for i := len(batch) - 1; i >= 0; i-- {
				p := batch[i]
				delta := len(p.repl) - (p.g2end - p.g2start)
				spans = adjustSpans(spans, p.g2start, p.g2end, delta)

				newSpan := substSpan{start: p.g2start, end: p.g2start + len(p.repl)}
				if p.alias {
					newSpan.alias = true
					newSpan.visited = p.visited
				}
				spans = append(spans, newSpan)

				value = value[:p.g2start] + p.repl + value[p.g2end:]
			}
		}

		// Unescape escaped braces
		value = strings.ReplaceAll(value, "\\{{", "{{")
		return value, nil
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

func replaceBracesAndTrimSpaces(v string) (variable string, isShallow bool) {
	variable = strings.ReplaceAll(v, "{{", "")
	variable = strings.ReplaceAll(variable, "}}", "")
	variable = strings.TrimSpace(variable)
	if strings.HasPrefix(variable, "-") {
		variable = strings.TrimSpace(variable[1:])
		return variable, true
	}

	return variable, false
}

func resolveReference(fullDocument interface{}, reference, absolutePath string) (interface{}, error) {
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
		return nil, err
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
