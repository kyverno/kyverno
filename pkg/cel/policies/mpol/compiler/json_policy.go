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
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/cel/common"
	"k8s.io/apiserver/pkg/cel/library"
	"k8s.io/apiserver/pkg/cel/mutation"
)

const (
	MaxJSONDocumentBytes   = 1024 * 1024
	MaxJSONPatchOperations = 1024
)

// JSONPolicy is an immutable, document-native policy, independent of admission attributes.
type JSONPolicy struct {
	policy    Policy
	mutations []cel.Program
}

type JSONEvaluationResult struct {
	Document         json.RawMessage
	Skipped          bool
	Exceptions       []*policiesv1beta1.PolicyException
	AuditAnnotations map[string]string
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

// CompileJSON compiles only JSON-mode policies. It does not enable JSON policies
// in the Kubernetes compiler, provider, or admission pipeline.
func CompileJSON(policy policiesv1beta1.MutatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*JSONPolicy, field.ErrorList) {
	path := field.NewPath("spec")
	if policy == nil || (reflect.ValueOf(policy).Kind() == reflect.Pointer && reflect.ValueOf(policy).IsNil()) {
		return nil, field.ErrorList{field.Required(path, "policy is required")}
	}
	spec := policy.GetSpec()
	if spec == nil {
		return nil, field.ErrorList{field.Required(path, "spec is required")}
	}
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
	env, err := cel.NewEnv(opts...)
	if err != nil {
		return nil, field.ErrorList{field.InternalError(path, err)}
	}
	provider := celcompiler.NewVariablesProvider(env.CELTypeProvider())
	env, err = env.Extend(cel.CustomTypeProvider(provider))
	if err != nil {
		return nil, field.ErrorList{field.InternalError(path, err)}
	}
	result := &JSONPolicy{}
	result.policy.variables = make(map[string]cel.Program)
	for i, variable := range spec.Variables {
		vpath := path.Child("variables").Index(i)
		if _, exists := result.policy.variables[variable.Name]; exists {
			return nil, field.ErrorList{field.Duplicate(vpath.Child("name"), variable.Name)}
		}
		ast, issues := env.Compile(variable.Expression)
		if issues.Err() != nil {
			return nil, field.ErrorList{field.Invalid(vpath.Child("expression"), variable.Expression, issues.Err().Error())}
		}
		program, err := newJSONProgram(env, ast)
		if err != nil {
			return nil, field.ErrorList{field.InternalError(vpath, err)}
		}
		provider.RegisterField(variable.Name, ast.OutputType())
		result.policy.variables[variable.Name] = program
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
		program, err := newJSONProgram(env, ast)
		if err != nil {
			errs = append(errs, field.InternalError(path, err))
		}
		return program
	}
	for i, condition := range spec.MatchConditions {
		result.policy.matchConditions = append(result.policy.matchConditions, compile(path.Child("matchConditions").Index(i).Child("expression"), condition.Expression, cel.BoolType))
	}
	for i, exception := range exceptions {
		if exception == nil {
			errs = append(errs, field.Required(field.NewPath("exceptions").Index(i), "exception is required"))
			continue
		}
		compiled := celcompiler.Exception{Exception: exception.DeepCopy()}
		for j, condition := range exception.Spec.MatchConditions {
			compiled.MatchConditions = append(compiled.MatchConditions, compile(field.NewPath("exceptions").Index(i).Child("matchConditions").Index(j).Child("expression"), condition.Expression, cel.BoolType))
		}
		result.policy.exceptions = append(result.policy.exceptions, compiled)
	}
	result.policy.auditAnnotations = make(map[string]cel.Program)
	for i, annotation := range spec.AuditAnnotations {
		result.policy.auditAnnotations[annotation.Key] = compile(path.Child("auditAnnotations").Index(i).Child("valueExpression"), annotation.ValueExpression, cel.StringType, cel.NullType)
	}
	for i, m := range spec.Mutations {
		result.mutations = append(result.mutations, compile(path.Child("mutations").Index(i).Child("jsonPatch", "expression"), m.JSONPatch.Expression, cel.ListType(jsonPatchType)))
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return result, nil
}

func newJSONProgram(env *cel.Env, ast *cel.Ast) (cel.Program, error) {
	return env.Program(ast, cel.CostLimit(uint64(celconfig.RuntimeCELCostBudget)), cel.InterruptCheckFrequency(100))
}

// EvaluateJSON returns no document on error and never mutates the caller's input.
// A failed test operation rolls back the whole policy and marks it skipped.
func (p *JSONPolicy) EvaluateJSON(ctx context.Context, document json.RawMessage) (JSONEvaluationResult, error) {
	if err := ctx.Err(); err != nil {
		return JSONEvaluationResult{}, err
	}
	object, err := decodeJSONDocument(document)
	if err != nil {
		return JSONEvaluationResult{}, err
	}
	data := map[string]any{celcompiler.ObjectKey: object, celcompiler.ExceptionsKey: libs.Exception{}}
	p.policy.appendVariables(ctx, data)
	var exceptions []*policiesv1beta1.PolicyException
	scopes := libs.Exception{}
	fullExemption := false
	for _, exception := range p.policy.exceptions {
		match, err := p.policy.match(ctx, data, exception.MatchConditions)
		if err != nil {
			if fullExemption {
				continue
			}
			return JSONEvaluationResult{}, fmt.Errorf("exception %s: %w", exception.Exception.Name, err)
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
		return JSONEvaluationResult{}, err
	}
	if fullExemption {
		return JSONEvaluationResult{Document: bytes.Clone(document), Skipped: true, Exceptions: exceptions}, nil
	}
	data[celcompiler.ExceptionsKey] = scopes
	p.policy.appendVariables(ctx, data)
	match, err := p.policy.match(ctx, data, p.policy.matchConditions)
	if err != nil {
		return JSONEvaluationResult{}, fmt.Errorf("match conditions: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return JSONEvaluationResult{}, err
	}
	if !match {
		return JSONEvaluationResult{Document: bytes.Clone(document), Skipped: true}, nil
	}
	patched := bytes.Clone(document)
	operations := 0
	for i, program := range p.mutations {
		if err := ctx.Err(); err != nil {
			return JSONEvaluationResult{}, err
		}
		out, _, err := program.ContextEval(ctx, data)
		if err != nil {
			return JSONEvaluationResult{}, fmt.Errorf("mutation %d: %w", i, err)
		}
		var skipped bool
		patched, skipped, err = applyDocumentPatch(ctx, patched, out, &operations)
		if err != nil {
			return JSONEvaluationResult{}, fmt.Errorf("mutation %d: %w", i, err)
		}
		if err := ctx.Err(); err != nil {
			return JSONEvaluationResult{}, err
		}
		if skipped {
			return JSONEvaluationResult{Document: bytes.Clone(document), Skipped: true}, nil
		}
		data[celcompiler.ObjectKey], err = decodeJSONDocument(patched)
		if err != nil {
			return JSONEvaluationResult{}, fmt.Errorf("mutation %d output: %w", i, err)
		}
		// Variables are memoized per activation; rebind them after object changes.
		p.policy.appendVariables(ctx, data)
	}
	annotations := make(map[string]string)
	for name, program := range p.policy.auditAnnotations {
		out, _, err := program.ContextEval(ctx, data)
		if err != nil {
			return JSONEvaluationResult{}, fmt.Errorf("audit annotation %s: %w", name, err)
		}
		if out == types.NullValue {
			continue
		}
		value, ok := out.(types.String)
		if !ok {
			return JSONEvaluationResult{}, fmt.Errorf("audit annotation %s: expected string or null", name)
		}
		if value != "" {
			annotations[name] = string(value)
		}
	}
	if err := ctx.Err(); err != nil {
		return JSONEvaluationResult{}, err
	}
	return JSONEvaluationResult{Document: patched, AuditAnnotations: annotations}, nil
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
