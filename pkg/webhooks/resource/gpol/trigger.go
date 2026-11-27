package gpol

import (
	"context"
	"slices"
	"sync"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	celutils "github.com/kyverno/sdk/extensions/cel/utils"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
)

// admissionPlan is the UpdateRequest the admission handler should write for one
// generating policy. queue is false when the object is neither a trigger nor a
// trigger that just stopped matching.
type admissionPlan struct {
	queue            bool
	deleteDownstream bool
	synchronize      bool
}

// planGeneratingAdmission decides whether this admission is a trigger for policy.
//
// The webhook lets UPDATE and DELETE through for every resource of the kind so
// synchronize can delete downstream objects when a trigger stops matching
// (kyverno/kyverno#16832). resourceNames cannot be expressed on a webhook rule,
// and those match conditions are wrapped so they only apply to CREATE. Both
// checks happen here, before an UpdateRequest is written.
//
// A check that cannot be evaluated (a Kyverno-only CEL function, a namespace
// selector) does not exclude the request. The background engine still matches
// before it generates.
func planGeneratingAdmission(
	ctx context.Context,
	policy policiesv1beta1.GeneratingPolicyLike,
	request admissionv1.AdmissionRequest,
	object unstructured.Unstructured,
	oldObject unstructured.Unstructured,
) admissionPlan {
	spec := policy.GetSpec()
	if spec == nil {
		return admissionPlan{queue: true}
	}
	sync := spec.SynchronizationEnabled()
	constraints := policy.GetMatchConstraints()
	conditions := policy.GetMatchConditions()

	switch request.Operation {
	case admissionv1.Create:
		if triggerMatches(ctx, constraints, conditions, request, &object, &oldObject, true, false) {
			return admissionPlan{queue: true}
		}
	case admissionv1.Update:
		if triggerMatches(ctx, constraints, conditions, request, &object, &oldObject, true, false) {
			return admissionPlan{queue: true, synchronize: sync}
		}
		// The live object no longer matches. If the previous object did, synchronize
		// still has to delete the downstream resources it created for this trigger.
		if sync && triggerMatches(ctx, constraints, conditions, request, &oldObject, &oldObject, false, true) {
			return admissionPlan{queue: true, synchronize: true}
		}
	case admissionv1.Delete:
		// DELETE carries the trigger in oldObject. Match that object, and do not
		// require the policy to have listed DELETE: synchronize observes deletion
		// of a trigger whose rule only names CREATE or UPDATE.
		if !triggerMatches(ctx, constraints, conditions, request, &oldObject, &oldObject, false, true) {
			return admissionPlan{}
		}
		if operationListed(constraints, admissionregistrationv1.Delete) {
			return admissionPlan{queue: true}
		}
		if sync {
			return admissionPlan{queue: true, deleteDownstream: true}
		}
	default:
		if triggerMatches(ctx, constraints, conditions, request, &object, &oldObject, true, false) {
			return admissionPlan{queue: true}
		}
	}
	return admissionPlan{}
}

// triggerMatches reports whether candidate is a resource this policy generates
// from. enforceOperation requires the admission operation to be one the rule
// lists; synchronize passes false so a trigger deleted or updated out of its
// rule is still recognized. cleanup is the synchronize path that looks at the
// previous object while the admission operation is UPDATE or DELETE.
func triggerMatches(
	ctx context.Context,
	constraints admissionregistrationv1.MatchResources,
	conditions []admissionregistrationv1.MatchCondition,
	request admissionv1.AdmissionRequest,
	candidate *unstructured.Unstructured,
	oldObject *unstructured.Unstructured,
	enforceOperation bool,
	cleanup bool,
) bool {
	if candidate == nil {
		candidate = &unstructured.Unstructured{}
	}
	if oldObject == nil {
		oldObject = &unstructured.Unstructured{}
	}
	matchConstraints := constraints
	if !enforceOperation {
		matchConstraints = widenOperations(constraints, admissionregistrationv1.OperationType(request.Operation))
	}
	matched, err := constraintsMatch(&matchConstraints, request, candidate, oldObject)
	if err != nil {
		// A matcher failure is not proof that the resource is not a trigger.
		matched = true
	}
	if !matched {
		return false
	}
	return conditionsMatch(ctx, conditions, request, candidate, oldObject, cleanup)
}

func operationListed(constraints admissionregistrationv1.MatchResources, operation admissionregistrationv1.OperationType) bool {
	for _, rule := range constraints.ResourceRules {
		if slices.Contains(rule.Operations, admissionregistrationv1.OperationAll) || slices.Contains(rule.Operations, operation) {
			return true
		}
	}
	return false
}

// widenOperations adds operation to every resource rule so a name and selector
// check still runs when synchronize observes an operation the policy did not list.
func widenOperations(in admissionregistrationv1.MatchResources, operation admissionregistrationv1.OperationType) admissionregistrationv1.MatchResources {
	out := in.DeepCopy()
	for i := range out.ResourceRules {
		ops := out.ResourceRules[i].Operations
		if slices.Contains(ops, admissionregistrationv1.OperationAll) || slices.Contains(ops, operation) {
			continue
		}
		out.ResourceRules[i].Operations = append(append([]admissionregistrationv1.OperationType{}, ops...), operation)
	}
	return *out
}

func constraintsMatch(
	constraints *admissionregistrationv1.MatchResources,
	request admissionv1.AdmissionRequest,
	object *unstructured.Unstructured,
	oldObject *unstructured.Unstructured,
) (bool, error) {
	// The admission handler does not have namespace objects. Leave namespace
	// selectors to the background engine instead of rejecting triggers whose
	// namespace labels are simply unknown here.
	if constraints != nil && constraints.NamespaceSelector != nil {
		copied := constraints.DeepCopy()
		copied.NamespaceSelector = nil
		constraints = copied
	}
	name := request.Name
	if name == "" {
		name = object.GetName()
	}
	namespace := request.Namespace
	if namespace == "" {
		namespace = object.GetNamespace()
	}
	attr := admission.NewAttributesRecord(
		object,
		oldObject,
		schema.GroupVersionKind(request.Kind),
		namespace,
		name,
		schema.GroupVersionResource(request.Resource),
		request.SubResource,
		admission.Operation(request.Operation),
		nil,
		false,
		admissionpolicy.NewUser(request.UserInfo),
	)
	return triggerMatcher.Match(&matching.MatchCriteria{Constraints: constraints}, attr, nil)
}

var triggerMatcher = matching.NewMatcher()

// conditionsMatch is false only when a match condition evaluates to false.
// A condition that cannot be compiled or evaluated here is ignored so a
// Kyverno-only function cannot hide a real trigger from synchronize.
//
// On the cleanup path the admission operation is no longer the one that
// created the trigger. A condition that reads request is inconclusive there:
// request.operation == "CREATE" was true when the downstream resources were
// generated and is false on the delete that must remove them.
func conditionsMatch(
	ctx context.Context,
	conditions []admissionregistrationv1.MatchCondition,
	request admissionv1.AdmissionRequest,
	object *unstructured.Unstructured,
	oldObject *unstructured.Unstructured,
	cleanup bool,
) bool {
	if len(conditions) == 0 {
		return true
	}
	activation, err := conditionActivation(request, object, oldObject)
	if err != nil {
		return true
	}
	// The worker outlives the HTTP request. Drop cancellation and keep the
	// request cost budget that admission already attached to ctx.
	evalCtx := context.WithoutCancel(ctx)
	for _, condition := range conditions {
		if cleanup && referencesRequest(condition.Expression) {
			continue
		}
		program, ok := compileMatchCondition(condition.Expression)
		if !ok {
			continue
		}
		out, _, err := program.ContextEval(evalCtx, activation)
		if err != nil {
			continue
		}
		result, err := celutils.ConvertToNative[bool](out)
		if err != nil {
			continue
		}
		if !result {
			return false
		}
	}
	return true
}

func conditionActivation(request admissionv1.AdmissionRequest, object, oldObject runtime.Object) (map[string]any, error) {
	name := request.Name
	namespace := request.Namespace
	if u, ok := object.(*unstructured.Unstructured); ok && u != nil {
		if name == "" {
			name = u.GetName()
		}
		if namespace == "" {
			namespace = u.GetNamespace()
		}
	}
	attr := admission.NewAttributesRecord(
		object,
		oldObject,
		schema.GroupVersionKind(request.Kind),
		namespace,
		name,
		schema.GroupVersionResource(request.Resource),
		request.SubResource,
		admission.Operation(request.Operation),
		nil,
		false,
		admissionpolicy.NewUser(request.UserInfo),
	)
	activation, err := compiler.PrepareK8sActivation(attr, &request, nil, nil, compiler.BuildRawRequestMap)
	if err != nil {
		return nil, err
	}
	return activation.ToMap(), nil
}

var (
	conditionEnvOnce sync.Once
	conditionEnv     *cel.Env
	errConditionEnv  error

	conditionPrograms sync.Mutex
	conditionCache    = map[string]cel.Program{}
	conditionMisses   = map[string]struct{}{}
)

func matchConditionEnv() (*cel.Env, error) {
	conditionEnvOnce.Do(func() {
		conditionEnv, errConditionEnv = compiler.NewEnv(append(
			compiler.DefaultEnvOptionsWithCompat(),
			cel.Variable(compiler.NamespaceObjectKey, cel.DynType),
			cel.Variable(compiler.ObjectKey, cel.DynType),
			cel.Variable(compiler.OldObjectKey, cel.DynType),
			cel.Variable(compiler.RequestKey, cel.DynType),
		)...)
	})
	return conditionEnv, errConditionEnv
}

// referencesRequest reports whether expression reads the admission request.
// An expression that cannot be parsed is treated as if it does, so cleanup
// does not drop a trigger on a condition this pre-filter cannot understand.
func referencesRequest(expression string) bool {
	env, err := getRequestIdentParseEnv()
	if err != nil {
		return true
	}
	parsed, iss := env.Parse(expression)
	if iss != nil && iss.Err() != nil {
		return true
	}
	found := false
	celast.PreOrderVisit(parsed.NativeRep().Expr(), celast.NewExprVisitor(func(node celast.Expr) {
		if found || node == nil {
			return
		}
		if node.Kind() == celast.IdentKind && node.AsIdent() == compiler.RequestKey {
			found = true
		}
	}))
	return found
}

var (
	parsedRequestEnv     *cel.Env
	errParsedRequestEnv  error
	parsedRequestEnvOnce sync.Once
)

func getRequestIdentParseEnv() (*cel.Env, error) {
	parsedRequestEnvOnce.Do(func() {
		parsedRequestEnv, errParsedRequestEnv = cel.NewEnv(cel.OptionalTypes())
	})
	return parsedRequestEnv, errParsedRequestEnv
}

func compileMatchCondition(expression string) (cel.Program, bool) {
	conditionPrograms.Lock()
	defer conditionPrograms.Unlock()
	if program, ok := conditionCache[expression]; ok {
		return program, true
	}
	if _, ok := conditionMisses[expression]; ok {
		return nil, false
	}
	env, err := matchConditionEnv()
	if err != nil {
		conditionMisses[expression] = struct{}{}
		return nil, false
	}
	program, errs := compiler.CompileMatchCondition(field.NewPath("spec").Child("matchConditions"), env, admissionregistrationv1.MatchCondition{
		Name:       "match",
		Expression: expression,
	})
	if len(errs) > 0 || program == nil {
		conditionMisses[expression] = struct{}{}
		return nil, false
	}
	conditionCache[expression] = program
	return program, true
}
