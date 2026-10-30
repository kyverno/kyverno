package compiler

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/common"
	"k8s.io/apiserver/pkg/cel/library"
	"k8s.io/apiserver/pkg/cel/mutation"
)

func (c *compilerImpl) compileForJSON(policy policiesv1beta1.MutatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList) {
	spec := policy.GetSpec()
	path := field.NewPath("spec")
	if errs := validateJSONSpec(spec); len(errs) > 0 {
		return nil, errs
	}

	env, variablesProvider, err := newJSONEnv()
	if err != nil {
		return nil, field.ErrorList{field.InternalError(path, fmt.Errorf(compileExtendedError, err))}
	}

	result := &Policy{jsonMode: true, trace: c.trace}
	var errs field.ErrorList
	result.variables, result.tracedVariables, errs = celcompiler.CompileVariablesWithTrace(path.Child("variables"), env, variablesProvider, c.trace, spec.Variables...)
	if len(errs) > 0 {
		return nil, errs
	}
	result.matchConditions, result.tracedMatchConditions, errs = celcompiler.CompileMatchConditionsWithTrace(path.Child("matchConditions"), env, c.trace, spec.MatchConditions...)
	if len(errs) > 0 {
		return nil, errs
	}
	for i, exception := range exceptions {
		if exception == nil {
			return nil, field.ErrorList{field.Required(field.NewPath("exceptions").Index(i), "exception is required")}
		}
		matchConditions, errs := celcompiler.CompileMatchConditions(field.NewPath("exceptions").Index(i).Child("matchConditions"), env, exception.Spec.MatchConditions...)
		if len(errs) > 0 {
			return nil, errs
		}
		result.exceptions = append(result.exceptions, celcompiler.Exception{
			Exception:       exception.DeepCopy(),
			MatchConditions: matchConditions,
		})
	}
	result.auditAnnotations, errs = celcompiler.CompileAuditAnnotations(path.Child("auditAnnotations"), env, spec.AuditAnnotations...)
	if len(errs) > 0 {
		return nil, errs
	}

	for i, mutation := range spec.Mutations {
		switch mutation.PatchType {
		case admissionregistrationv1alpha1.PatchTypeJSONPatch:
			if mutation.JSONPatch == nil {
				continue
			}
			compiled, errs := celcompiler.CompileMutationWithTrace(
				path.Child("mutations").Index(i).Child("jsonPatch"),
				env,
				mutation.JSONPatch.Expression,
				cel.ListType(jsonPatchType),
				c.trace,
			)
			if len(errs) > 0 {
				return nil, errs
			}
			result.patchers = append(result.patchers, newJSONPatcher(compiled.Program))
			if c.trace {
				compiled.Name = fmt.Sprintf("mutations[%d] (jsonPatch)", i)
				result.tracedMutations = append(result.tracedMutations, compiled)
			}
		case admissionregistrationv1alpha1.PatchTypeApplyConfiguration:
			if mutation.ApplyConfiguration == nil {
				continue
			}
			compiled, errs := celcompiler.CompileMutationWithTrace(
				path.Child("mutations").Index(i).Child("applyConfiguration"),
				env,
				mutation.ApplyConfiguration.Expression,
				applyConfigObjectType,
				c.trace,
			)
			if len(errs) > 0 {
				return nil, errs
			}
			useServerSideApply := spec.EvaluationConfiguration != nil && spec.EvaluationConfiguration.UseServerSideApply
			result.patchers = append(result.patchers, newApplyConfigPatcher(compiled.Program, useServerSideApply))
			if c.trace {
				compiled.Name = fmt.Sprintf("mutations[%d] (applyConfiguration)", i)
				result.tracedMutations = append(result.tracedMutations, compiled)
			}
		}
	}
	return result, nil
}

func validateJSONSpec(spec *policiesv1beta1.MutatingPolicySpec) field.ErrorList {
	path := field.NewPath("spec")
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
	for i, mutation := range spec.Mutations {
		switch mutation.PatchType {
		case admissionregistrationv1alpha1.PatchTypeJSONPatch:
			if mutation.JSONPatch == nil || mutation.ApplyConfiguration != nil {
				errs = append(errs, field.Invalid(path.Child("mutations").Index(i), mutation.PatchType, "JSONPatch mutation requires jsonPatch and cannot include applyConfiguration"))
			}
		case admissionregistrationv1alpha1.PatchTypeApplyConfiguration:
			if mutation.ApplyConfiguration == nil || mutation.JSONPatch != nil {
				errs = append(errs, field.Invalid(path.Child("mutations").Index(i), mutation.PatchType, "ApplyConfiguration mutation requires applyConfiguration and cannot include jsonPatch"))
			}
		default:
			errs = append(errs, field.Invalid(path.Child("mutations").Index(i), mutation.PatchType, "only JSONPatch and ApplyConfiguration mutations are supported"))
		}
	}
	return errs
}

// EvaluateJSON evaluates an object-root payload without admission attributes.
func (p *Policy) EvaluateJSON(ctx context.Context, object *unstructured.Unstructured) *EvaluationResult {
	if !p.jsonMode {
		return &EvaluationResult{Error: fmt.Errorf("cannot evaluate Kubernetes-mode policy against a JSON payload")}
	}
	if object == nil || object.Object == nil {
		return &EvaluationResult{Error: fmt.Errorf("JSON payload requires an object root")}
	}
	if err := ctx.Err(); err != nil {
		return &EvaluationResult{Error: err}
	}
	attr := admission.NewAttributesRecord(
		object,
		nil,
		schema.GroupVersionKind{},
		object.GetNamespace(),
		object.GetName(),
		schema.GroupVersionResource{},
		"",
		admission.Create,
		nil,
		false,
		nil,
	)
	return p.evaluate(ctx, attr, nil, admissionv1.AdmissionRequest{}, nil, nil, nil, false)
}

func jsonAutogenPodControllers(spec *policiesv1beta1.MutatingPolicySpec) bool {
	return spec.AutogenConfiguration != nil &&
		spec.AutogenConfiguration.PodControllers != nil &&
		len(spec.AutogenConfiguration.PodControllers.Controllers) > 0
}

type jsonPatchResolver struct{}

func (jsonPatchResolver) Resolve(name string) (common.ResolvedType, bool) {
	if name == mutation.JSONPatchTypeName {
		return &documentPatchType{}, true
	}
	return (&mutation.DynamicTypeResolver{}).Resolve(name)
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

func newJSONEnv() (*cel.Env, *celcompiler.VariablesProvider, error) {
	opts := celcompiler.DynamicResourceEnvOptionsWithCompat()
	opts = append(opts,
		cel.Variable(celcompiler.ObjectKey, cel.DynType),
		cel.Variable(celcompiler.VariablesKey, celcompiler.VariablesType),
		cel.Types(jsonPatchType, applyConfigObjectType),
		common.ResolverEnvOption(jsonPatchResolver{}),
		library.JSONPatch(),
		ext.NativeTypes(reflect.TypeFor[libs.Exception](), ext.ParseStructTags(true)),
		cel.Variable(celcompiler.ExceptionsKey, types.NewObjectType("libs.Exception")),
	)
	env, err := celcompiler.NewEnv(opts...)
	if err != nil {
		return nil, nil, err
	}
	provider := celcompiler.NewVariablesProvider(env.CELTypeProvider())
	env, err = env.Extend(cel.CustomTypeProvider(provider))
	if err != nil {
		return nil, nil, err
	}
	return env, provider, nil
}
