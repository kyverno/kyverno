package compiler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/ext"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/cel/common"
	"k8s.io/apiserver/pkg/cel/library"
	"k8s.io/apiserver/pkg/cel/mutation"
	kjson "sigs.k8s.io/json"
)

const (
	MaxJSONDocumentBytes   = 1024 * 1024
	MaxJSONPatchOperations = 1024
)

type jsonEvaluationResult struct {
	Document         json.RawMessage
	Skipped          bool
	Exceptions       []*policiesv1beta1.PolicyException
	AuditAnnotations map[string]string
	Trace            *trace.Decision
}

type jsonPatchResolver struct{}

func (jsonPatchResolver) Resolve(name string) (common.ResolvedType, bool) {
	if name == mutation.JSONPatchTypeName {
		return &documentPatchType{}, true
	}
	return nil, false
}

type documentPatchType struct {
	mutation.JSONPatchType
}

func (t *documentPatchType) Field(name string) (*types.FieldType, bool) {
	switch name {
	case "op", "path", "from", "value":
		return t.JSONPatchType.Field(name)
	default:
		return nil, false
	}
}

func (t *documentPatchType) Val(fields map[string]ref.Val) ref.Val {
	for _, name := range []string{"op", "path"} {
		if _, exists := fields[name]; !exists {
			return types.NewErr("JSONPatch requires %s", name)
		}
	}
	if op, ok := fields["op"].(types.String); ok && (op == "move" || op == "copy") {
		if _, exists := fields["from"]; !exists {
			return types.NewErr("JSONPatch %s requires from", op)
		}
	}
	return t.JSONPatchType.Val(fields)
}

func (c *compilerImpl) compileForJSON(policy policiesv1beta1.MutatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList) {
	path := field.NewPath("spec")
	spec := policy.GetSpec()
	if spec.EvaluationConfiguration == nil || spec.EvaluationConfiguration.Mode != policieskyvernoio.EvaluationModeJSON {
		return nil, field.ErrorList{field.Invalid(path.Child("evaluation", "mode"), "", "JSON mode is required")}
	}
	var errs field.ErrorList
	for _, setting := range []struct {
		name    string
		enabled bool
	}{
		{"targetMatchConstraints", spec.TargetMatchConstraints != nil},
		{"targetMatchConditions", len(spec.TargetMatchConditions) > 0},
		{"autogen.podControllers.controllers", jsonAutogenPodControllers(spec)},
		{"autogen.mutatingAdmissionPolicy.enabled", spec.GenerateMutatingAdmissionPolicyEnabled()},
		{"evaluation.mutateExisting", spec.MutateExistingEnabled()},
		{"evaluation.useServerSideApply", spec.EvaluationConfiguration.UseServerSideApply},
		{"reinvocationPolicy", spec.GetReinvocationPolicy() != admissionregistrationv1.NeverReinvocationPolicy},
	} {
		if setting.enabled {
			names := strings.Split(setting.name, ".")
			errs = append(errs, field.Forbidden(path.Child(names[0], names[1:]...), "not supported for JSON documents"))
		}
	}
	if len(spec.Mutations) == 0 {
		errs = append(errs, field.Required(path.Child("mutations"), "at least one mutation is required"))
	}
	for i, m := range spec.Mutations {
		if m.PatchType != admissionregistrationv1alpha1.PatchTypeJSONPatch || m.JSONPatch == nil || m.ApplyConfiguration != nil {
			errs = append(errs, field.Invalid(path.Child("mutations").Index(i), m.PatchType, "only JSONPatch mutations are supported"))
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	opts := celcompiler.DynamicResourceEnvOptionsWithCompat()
	opts = append(opts,
		cel.Variable(celcompiler.ObjectKey, cel.DynType),
		cel.Variable(celcompiler.VariablesKey, celcompiler.VariablesType),
		cel.Types(jsonPatchType),
		common.ResolverEnvOption(jsonPatchResolver{}),
		library.JSONPatch(),
		ext.NativeTypes(reflect.TypeFor[libs.Exception](), ext.ParseStructTags(true)),
		cel.Variable(celcompiler.ExceptionsKey, types.NewObjectType("libs.Exception")),
	)
	env, err := celcompiler.NewEnv(opts...)
	if err != nil {
		return nil, field.ErrorList{field.InternalError(path, err)}
	}
	provider := celcompiler.NewVariablesProvider(env.CELTypeProvider())
	env, err = env.Extend(cel.CustomTypeProvider(provider))
	if err != nil {
		return nil, field.ErrorList{field.InternalError(path, err)}
	}
	result := &Policy{jsonMutations: make([]cel.Program, 0, len(spec.Mutations)), trace: c.trace}
	names := make(map[string]bool)
	for i, variable := range spec.Variables {
		if names[variable.Name] {
			return nil, field.ErrorList{field.Duplicate(path.Child("variables").Index(i).Child("name"), variable.Name)}
		}
		names[variable.Name] = true
	}
	var compileErrs field.ErrorList
	result.variables, result.tracedVariables, compileErrs = celcompiler.CompileVariablesWithTrace(path.Child("variables"), env, provider, c.trace, spec.Variables...)
	if len(compileErrs) > 0 {
		return nil, compileErrs
	}
	compile := func(path *field.Path, expression string, allowed ...*cel.Type) cel.Program {
		ast, issues := env.Compile(expression)
		if issues.Err() != nil {
			errs = append(errs, field.Invalid(path, expression, issues.Err().Error()))
			return nil
		}
		valid := false
		for _, t := range allowed {
			valid = valid || ast.OutputType().IsExactType(t)
		}
		if !valid {
			errs = append(errs, field.Invalid(path, expression, fmt.Sprintf("unexpected output type %s", ast.OutputType())))
			return nil
		}
		program, err := celcompiler.NewProgram(env, ast)
		if err != nil {
			errs = append(errs, field.InternalError(path, err))
		}
		return program
	}
	result.matchConditions, result.tracedMatchConditions, compileErrs = celcompiler.CompileMatchConditionsWithTrace(path.Child("matchConditions"), env, c.trace, spec.MatchConditions...)
	errs = append(errs, compileErrs...)
	for i, exception := range exceptions {
		if exception == nil {
			errs = append(errs, field.Required(field.NewPath("exceptions").Index(i), "exception is required"))
			continue
		}
		compiled := celcompiler.Exception{Exception: exception.DeepCopy()}
		for j, condition := range exception.Spec.MatchConditions {
			compiled.MatchConditions = append(compiled.MatchConditions, compile(field.NewPath("exceptions").Index(i).Child("matchConditions").Index(j).Child("expression"), condition.Expression, cel.BoolType))
		}
		result.exceptions = append(result.exceptions, compiled)
	}
	result.auditAnnotations = make(map[string]cel.Program)
	for i, annotation := range spec.AuditAnnotations {
		result.auditAnnotations[annotation.Key] = compile(path.Child("auditAnnotations").Index(i).Child("valueExpression"), annotation.ValueExpression, cel.StringType, cel.NullType)
	}
	for i, m := range spec.Mutations {
		compiled, compileErrs := celcompiler.CompileMutationWithTrace(path.Child("mutations").Index(i).Child("jsonPatch"), env, m.JSONPatch.Expression, cel.ListType(jsonPatchType), c.trace)
		errs = append(errs, compileErrs...)
		result.jsonMutations = append(result.jsonMutations, compiled.Program)
		if c.trace {
			compiled.Name = fmt.Sprintf("mutations[%d] (jsonPatch)", i)
			result.tracedMutations = append(result.tracedMutations, compiled)
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return result, nil
}

// EvaluateJSON evaluates an object-root payload without admission attributes.
func (p *Policy) EvaluateJSON(ctx context.Context, object *unstructured.Unstructured) *EvaluationResult {
	if p.jsonMutations == nil {
		return &EvaluationResult{Error: fmt.Errorf("cannot evaluate Kubernetes-mode policy against a JSON payload")}
	}
	if object == nil || object.Object == nil {
		return &EvaluationResult{Error: fmt.Errorf("JSON payload requires an object root")}
	}
	raw, err := object.MarshalJSON()
	if err != nil {
		return &EvaluationResult{Error: err}
	}
	result, err := p.evaluateJSONDocument(ctx, raw)
	if err != nil {
		return &EvaluationResult{Error: err, Trace: result.Trace}
	}
	var patched map[string]any
	if err := kjson.UnmarshalCaseSensitivePreserveInts(result.Document, &patched); err != nil || patched == nil {
		return &EvaluationResult{Error: fmt.Errorf("JSON mutation must preserve an object root"), Trace: result.Trace}
	}
	return &EvaluationResult{
		PatchedResource:  &unstructured.Unstructured{Object: patched},
		Skipped:          result.Skipped && len(result.Exceptions) == 0,
		Exceptions:       result.Exceptions,
		AuditAnnotations: result.AuditAnnotations,
		Trace:            result.Trace,
	}
}

func (p *Policy) evaluateJSONDocument(ctx context.Context, document json.RawMessage) (result jsonEvaluationResult, evaluationErr error) {
	var decision *trace.Decision
	if p.trace {
		decision = &trace.Decision{Verdict: trace.VerdictTrace{Status: trace.VerdictPass}}
		defer func() {
			if evaluationErr != nil {
				decision.Verdict.Status, decision.Verdict.Message = trace.VerdictError, evaluationErr.Error()
			} else if result.Skipped {
				decision.Verdict.Status = trace.VerdictSkip
				if decision.Verdict.Message == "" {
					decision.Verdict.Message = skipMessage(decision.Match)
				}
			}
			result.Trace = decision
		}()
	}
	if err := ctx.Err(); err != nil {
		return jsonEvaluationResult{}, err
	}
	object, err := decodeJSONDocument(document)
	if err != nil {
		return jsonEvaluationResult{}, err
	}
	data := map[string]any{celcompiler.ObjectKey: object, celcompiler.ExceptionsKey: libs.Exception{}}
	var recordMatch func(int, ref.Val, error)
	var recordVariable func(string, ref.Val, error)
	if p.trace {
		recordMatch = func(i int, out ref.Val, err error) {
			t := p.tracedMatchConditions[i]
			details := celcompiler.TraceDetails(ctx, t.Traced, data, err)
			decision.Match = append(decision.Match, trace.NamedExpressionTrace{
				Name: t.Name, ExpressionTrace: buildExpressionTrace(t.AST, out, details, err),
			})
		}
		recordVariable = func(name string, out ref.Val, err error) {
			t := p.tracedVariables[name]
			details := celcompiler.TraceDetails(ctx, t.Traced, data, err)
			decision.Variables = append(decision.Variables, trace.NamedExpressionTrace{
				Name: name, ExpressionTrace: buildExpressionTrace(t.AST, out, details, err),
			})
		}
	}
	p.appendVariables(ctx, data, nil)
	var exceptions []*policiesv1beta1.PolicyException
	scopes := libs.Exception{}
	fullExemption := false
	for _, exception := range p.exceptions {
		match, err := p.match(ctx, data, exception.MatchConditions, nil)
		if err != nil {
			if fullExemption {
				continue
			}
			return jsonEvaluationResult{}, fmt.Errorf("exception %s: %w", exception.Exception.Name, err)
		}
		if match {
			exceptions = append(exceptions, exception.Exception.DeepCopy())
			if len(exception.Exception.Spec.Images) == 0 && len(exception.Exception.Spec.AllowedValues) == 0 {
				fullExemption = true
			}
			scopes.AllowedImages = append(scopes.AllowedImages, exception.Exception.Spec.Images...)
			scopes.AllowedValues = append(scopes.AllowedValues, exception.Exception.Spec.AllowedValues...)
		}
	}
	if err := ctx.Err(); err != nil {
		return jsonEvaluationResult{}, err
	}
	if fullExemption {
		return jsonEvaluationResult{Document: bytes.Clone(document), Skipped: true, Exceptions: exceptions}, nil
	}
	data[celcompiler.ExceptionsKey] = scopes
	p.appendVariables(ctx, data, recordVariable)
	match, err := p.match(ctx, data, p.matchConditions, recordMatch)
	if err != nil {
		return jsonEvaluationResult{}, fmt.Errorf("match conditions: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return jsonEvaluationResult{}, err
	}
	if !match {
		return jsonEvaluationResult{Document: bytes.Clone(document), Skipped: true}, nil
	}
	patched := bytes.Clone(document)
	operations := 0
	for i, program := range p.jsonMutations {
		if err := ctx.Err(); err != nil {
			return jsonEvaluationResult{}, err
		}
		out, _, err := program.ContextEval(ctx, data)
		var skipped bool
		if err == nil {
			patched, skipped, err = applyDocumentPatch(ctx, patched, out, &operations)
		}
		if p.trace {
			t := p.tracedMutations[i]
			details := celcompiler.TraceDetails(ctx, t.Traced, data, err)
			mt := trace.MutationTrace{Name: t.Name, ExpressionTrace: buildExpressionTrace(t.AST, out, details, err)}
			if err != nil {
				mt.Error = err.Error()
			}
			if skipped {
				mt.NotApplied = "a test operation failed, so the object was left unchanged"
				decision.Verdict.Message = mt.NotApplied
			}
			decision.Mutations = append(decision.Mutations, mt)
		}
		if err != nil {
			return jsonEvaluationResult{}, fmt.Errorf("mutation %d: %w", i, err)
		}
		if err := ctx.Err(); err != nil {
			return jsonEvaluationResult{}, err
		}
		if skipped {
			return jsonEvaluationResult{Document: bytes.Clone(document), Skipped: true}, nil
		}
		data[celcompiler.ObjectKey], err = decodeJSONDocument(patched)
		if err != nil {
			return jsonEvaluationResult{}, fmt.Errorf("mutation %d output: %w", i, err)
		}
		// Variables are memoized per activation; rebind them after object changes.
		if _, ok := data[celcompiler.ObjectKey].(map[string]any); !ok {
			return jsonEvaluationResult{}, fmt.Errorf("JSON mutation must preserve an object root")
		}
		p.appendVariables(ctx, data, recordVariable)
	}
	annotations := make(map[string]string)
	for name, program := range p.auditAnnotations {
		out, _, err := program.ContextEval(ctx, data)
		if err != nil {
			return jsonEvaluationResult{}, fmt.Errorf("audit annotation %s: %w", name, err)
		}
		if out == types.NullValue {
			continue
		}
		value, ok := out.(types.String)
		if !ok {
			return jsonEvaluationResult{}, fmt.Errorf("audit annotation %s: expected string or null", name)
		}
		if value != "" {
			annotations[name] = string(value)
		}
	}
	if err := ctx.Err(); err != nil {
		return jsonEvaluationResult{}, err
	}
	return jsonEvaluationResult{Document: patched, AuditAnnotations: annotations}, nil
}

// jsonAutogenPodControllers reports whether the policy asks for Pod controller
// autogen. An empty autogen object (for example from a defaulted round-trip)
// is accepted; only configuration that would generate rules is forbidden.
func jsonAutogenPodControllers(spec *policiesv1beta1.MutatingPolicySpec) bool {
	return spec.AutogenConfiguration != nil &&
		spec.AutogenConfiguration.PodControllers != nil &&
		len(spec.AutogenConfiguration.PodControllers.Controllers) > 0
}

func decodeJSONDocument(document json.RawMessage) (any, error) {
	if len(document) > MaxJSONDocumentBytes {
		return nil, fmt.Errorf("JSON document exceeds %d bytes", MaxJSONDocumentBytes)
	}
	if !json.Valid(document) {
		return nil, fmt.Errorf("invalid JSON document")
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return jsonNumbers(value)
}

func jsonNumbers(value any) (any, error) {
	switch value := value.(type) {
	case json.Number:
		if !strings.ContainsAny(string(value), ".eE") {
			number, err := strconv.ParseInt(string(value), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("JSON integer %s is outside the CEL int64 range", value)
			}
			return number, nil
		}
		number, err := value.Float64()
		if err != nil || math.IsInf(number, 0) {
			return nil, fmt.Errorf("JSON number %s is outside the CEL double range", value)
		}
		return number, nil
	case []any:
		for i, element := range value {
			number, err := jsonNumbers(element)
			if err != nil {
				return nil, err
			}
			value[i] = number
		}
	case map[string]any:
		for key, element := range value {
			number, err := jsonNumbers(element)
			if err != nil {
				return nil, err
			}
			value[key] = number
		}
	}
	return value, nil
}

func jsonNativeValue(value ref.Val, remaining *int) (any, error) {
	*remaining--
	if *remaining < 0 {
		return nil, fmt.Errorf("patch value exceeds JSON document size budget")
	}
	switch value := value.(type) {
	case types.Null:
		return nil, nil
	case types.Bool:
		return bool(value), nil
	case types.String:
		*remaining -= len(value)
		if *remaining < 0 {
			return nil, fmt.Errorf("patch value exceeds JSON document size budget")
		}
		return string(value), nil
	case types.Int:
		return int64(value), nil
	case types.Uint:
		if uint64(value) > math.MaxInt64 {
			return nil, fmt.Errorf("unsigned integer %d is outside the int64 range", uint64(value))
		}
		return int64(value), nil //nolint:gosec // bounded by math.MaxInt64 above
	case types.Double:
		return float64(value), nil
	case traits.Lister:
		result := make([]any, 0)
		for iter := value.Iterator(); iter.HasNext() == types.True; {
			element, err := jsonNativeValue(iter.Next(), remaining)
			if err != nil {
				return nil, err
			}
			result = append(result, element)
		}
		return result, nil
	case traits.Mapper:
		result := make(map[string]any)
		for iter := value.Iterator(); iter.HasNext() == types.True; {
			key := iter.Next()
			name, ok := key.(types.String)
			if !ok {
				return nil, fmt.Errorf("JSON object keys must be strings")
			}
			*remaining -= len(name)
			element, err := jsonNativeValue(value.Get(key), remaining)
			if err != nil {
				return nil, err
			}
			result[string(name)] = element
		}
		return result, nil
	default:
		return nil, fmt.Errorf("CEL value of type %s is not JSON-compatible", value.Type())
	}
}
