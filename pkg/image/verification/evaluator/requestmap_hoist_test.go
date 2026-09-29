package evaluator

import (
	"context"
	"fmt"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engine "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

// buildRequestMapHoistPolicy returns a Kubernetes-mode ImageValidatingPolicy
// matching v1/pods, with an always-true matchCondition and numExceptions
// PolicyExceptions whose matchConditions never match (so match() is
// exercised for the policy and for every exception without ever hitting the
// full-exemption early return). No ImageExtractors are declared, so
// ExtractImages sees zero images and Evaluate/MutateDigest never touch the
// network -- these tests only exist to count requestMapFn invocations.
func buildRequestMapHoistPolicy(name string, numExceptions int) (*policiesv1beta1.ImageValidatingPolicy, []*policiesv1beta1.PolicyException) {
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "always-true", Expression: "true"},
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "true"},
			},
		},
	}
	exceptions := make([]*policiesv1beta1.PolicyException, 0, numExceptions)
	for i := 0; i < numExceptions; i++ {
		exceptions = append(exceptions, &policiesv1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-polex-%d", name, i)},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				PolicyRefs: []policiesv1beta1.PolicyRef{
					{Name: policy.Name, Kind: "ImageValidatingPolicy"},
				},
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{Name: "never", Expression: "false"},
				},
			},
		})
	}
	return policy, exceptions
}

func buildRequestMapHoistRequestAndAttr(t *testing.T, op admissionv1.Operation) (*admissionv1.AdmissionRequest, admission.Attributes, unstructured.Unstructured) {
	t.Helper()
	pod := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": "nginx", "namespace": "default"},
		"spec":       map[string]any{"containers": []any{}},
	}}
	objRaw, err := pod.MarshalJSON()
	require.NoError(t, err)

	request := &admissionv1.AdmissionRequest{
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Name:      "nginx",
		Namespace: "default",
		Operation: op,
		UserInfo:  authenticationv1.UserInfo{Username: "alice"},
		Object:    runtime.RawExtension{Raw: objRaw},
	}
	attr := admission.NewAttributesRecord(
		&pod,
		nil,
		schema.GroupVersionKind(request.Kind),
		request.Namespace,
		request.Name,
		schema.GroupVersionResource(request.Resource),
		request.SubResource,
		admission.Operation(request.Operation),
		nil,
		false,
		nil,
	)
	return request, attr, pod
}

func compileRequestMapHoistPolicy(t *testing.T, policy *policiesv1beta1.ImageValidatingPolicy, exceptions []*policiesv1beta1.PolicyException) CompiledPolicy {
	t.Helper()
	compiled, errs := NewCompiler(nil).Compile(policy, exceptions)
	require.Empty(t, errs)
	return compiled
}

// countingRequestMapFn returns a requestMapFn that records how many times it
// is invoked, alongside the returned map.
func countingRequestMapFn(request *admissionv1.AdmissionRequest) (func() (map[string]any, error), *int) {
	calls := 0
	return func() (map[string]any, error) {
		calls++
		m, err := utils.ConvertObjectToUnstructured(request)
		if err != nil {
			return nil, err
		}
		return m.Object, nil
	}, &calls
}

// TestEvaluate_RequestMapBuiltOnceRegardlessOfExceptions proves Evaluate
// resolves requestMapFn exactly once per call -- through prepareK8sData --
// no matter how many exceptions the policy carries, closing the multiplication
// bug the design exists to fix (previously: once for matchConditions, once
// per exception).
func TestEvaluate_RequestMapBuiltOnceRegardlessOfExceptions(t *testing.T) {
	const numExceptions = 3
	policy, exceptions := buildRequestMapHoistPolicy("evaluate-once", numExceptions)
	compiled := compileRequestMapHoistPolicy(t, policy, exceptions)
	request, attr, _ := buildRequestMapHoistRequestAndAttr(t, admissionv1.Create)
	ictx, err := imagedataloader.NewImageContext(nil, nil, nil)
	require.NoError(t, err)

	requestMapFn, calls := countingRequestMapFn(request)
	result, err := compiled.Evaluate(context.Background(), &imageverify.Runtime{ImageContext: ictx}, attr, request, nil, true, requestMapFn, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, *calls, "Evaluate must call requestMapFn exactly once per invocation, regardless of matchConditions/exception count")
}

// TestMutateDigest_RequestMapBuiltOnceRegardlessOfExceptions is the
// MutateDigest counterpart of the Evaluate test above.
func TestMutateDigest_RequestMapBuiltOnceRegardlessOfExceptions(t *testing.T) {
	const numExceptions = 3
	policy, exceptions := buildRequestMapHoistPolicy("mutate-once", numExceptions)
	compiled := compileRequestMapHoistPolicy(t, policy, exceptions)
	request, attr, pod := buildRequestMapHoistRequestAndAttr(t, admissionv1.Create)
	ictx, err := imagedataloader.NewImageContext(nil, nil, nil)
	require.NoError(t, err)

	requestMapFn, calls := countingRequestMapFn(request)
	_, err = compiled.MutateDigest(context.Background(), &imageverify.Runtime{ImageContext: ictx}, attr, request, nil, pod, requestMapFn, config.NewDefaultConfiguration(false), nil)
	require.NoError(t, err)
	assert.Equal(t, 1, *calls, "MutateDigest must call requestMapFn exactly once per invocation, regardless of matchConditions/exception count")
}

// buildRequestMapConsumptionPolicy returns a Kubernetes-mode
// ImageValidatingPolicy whose matchCondition, every exception's
// matchCondition, and its single validation all branch on request.operation
// -- built so the branches disagree depending on whether the *hoisted* map
// (requestMapFn's return value) or a locally rebuilt map is what `data`
// actually holds:
//   - matchCondition:            request.operation == 'CONNECT'  (true only under the hoisted map)
//   - every exception's matchCondition: request.operation != 'CONNECT'  (false under the hoisted map, true under a rebuild)
//   - validation:                request.operation == 'CONNECT'  (true only under the hoisted map)
//
// This is deliberately not just a call counter: it proves matchConditions,
// every exception, and the validation all observe the *same* map object
// returned by requestMapFn, not independently rebuilt copies of it.
func buildRequestMapConsumptionPolicy(name string, numExceptions int) (*policiesv1beta1.ImageValidatingPolicy, []*policiesv1beta1.PolicyException) {
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "hoisted-map-only", Expression: "request.operation == 'CONNECT'"},
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "request.operation == 'CONNECT'"},
			},
		},
	}
	exceptions := make([]*policiesv1beta1.PolicyException, 0, numExceptions)
	for i := 0; i < numExceptions; i++ {
		exceptions = append(exceptions, &policiesv1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-polex-%d", name, i)},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				PolicyRefs: []policiesv1beta1.PolicyRef{
					{Name: policy.Name, Kind: "ImageValidatingPolicy"},
				},
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{Name: "rebuild-only", Expression: "request.operation != 'CONNECT'"},
				},
			},
		})
	}
	return policy, exceptions
}

// sentinelRequestMapFn returns a requestMapFn built from request (like
// countingRequestMapFn), except the returned map's "operation" is
// overwritten to the sentinel "CONNECT" -- a value the real
// *admissionv1.AdmissionRequest (built with a different operation, e.g.
// CREATE) never carries. Any consumer that reads this exact map sees the
// sentinel; any consumer that instead falls back to rebuilding its own map
// from request sees the request's real operation.
func sentinelRequestMapFn(request *admissionv1.AdmissionRequest) (func() (map[string]any, error), *int) {
	calls := 0
	return func() (map[string]any, error) {
		calls++
		m, err := utils.ConvertObjectToUnstructured(request)
		if err != nil {
			return nil, err
		}
		m.Object["operation"] = "CONNECT"
		return m.Object, nil
	}, &calls
}

// TestEvaluate_MatchConditionsExceptionsAndValidationShareOneHoistedMap is
// the discriminating counterpart of
// TestEvaluate_RequestMapBuiltOnceRegardlessOfExceptions: it does not just
// count requestMapFn invocations, it proves the policy's matchCondition,
// every exception's matchCondition, and the validation all observe the same
// hoisted map object -- not independently rebuilt copies -- for a policy
// with matchConditions and multiple exceptions. A regression that
// reintroduced per-call-site rebuilding inside match would still call
// requestMapFn once (so the plain call-count assertion would stay green) but
// would fail every branch here, since match's internally rebuilt map would
// carry the real request.operation instead of the sentinel.
//
// The nil-requestMapFn fallback path (compiler.BuildRawRequestMap, used for
// the ExtractionMode synthetic-request carve-out and the JSON evaluation
// mode) is covered separately by TestPrepareK8sData_GoldenEquality, which
// calls prepareK8sData(..., nil) directly.
func TestEvaluate_MatchConditionsExceptionsAndValidationShareOneHoistedMap(t *testing.T) {
	const numExceptions = 3
	policy, exceptions := buildRequestMapConsumptionPolicy("shared-hoisted-map", numExceptions)
	compiled := compileRequestMapHoistPolicy(t, policy, exceptions)
	// The real request's operation is CREATE, never CONNECT -- only the
	// sentinel map (if actually consumed everywhere) can make the CONNECT
	// checks above pass.
	request, attr, _ := buildRequestMapHoistRequestAndAttr(t, admissionv1.Create)
	ictx, err := imagedataloader.NewImageContext(nil, nil, nil)
	require.NoError(t, err)

	requestMapFn, calls := sentinelRequestMapFn(request)
	result, err := compiled.Evaluate(context.Background(), &imageverify.Runtime{ImageContext: ictx}, attr, request, nil, true, requestMapFn, nil)
	require.NoError(t, err)
	require.NotNil(t, result, "matchCondition must have read the hoisted (sentinel) map, or Evaluate would have short-circuited as non-matching")
	assert.Empty(t, result.Exceptions, "every exception's matchCondition must have read the hoisted (sentinel) map, or a rebuilt map would satisfy 'operation != CONNECT' and grant a full exemption")
	assert.True(t, result.Result, "the validation must have read the hoisted (sentinel) map, or 'operation == CONNECT' would be false")
	assert.Equal(t, 1, *calls, "requestMapFn must be called exactly once per Evaluate invocation, regardless of matchConditions/exception count")
}

// TestPrepareK8sData_GoldenEquality proves ivpol's request.* activation value
// -- produced by prepareK8sData through the hoisted compiler.BuildRawRequestMap
// path -- is byte-identical to the pre-hoist base formula
// utils.ConvertObjectToUnstructured(request).Object, across the operation
// matrix, including DELETE and CONNECT where object/oldObject are empty and
// must stay CEL null (nil), not a normalized empty map.
func TestPrepareK8sData_GoldenEquality(t *testing.T) {
	for _, op := range []admissionv1.Operation{
		admissionv1.Create,
		admissionv1.Update,
		admissionv1.Delete,
		admissionv1.Connect,
	} {
		t.Run(string(op), func(t *testing.T) {
			request, attr, _ := buildRequestMapHoistRequestAndAttr(t, op)
			if op == admissionv1.Delete || op == admissionv1.Connect {
				// DELETE/CONNECT admit with an empty (or absent) object payload;
				// reproduce that so both formulas see the same empty RawExtension.
				request.Object = runtime.RawExtension{}
			}

			data, err := prepareK8sData(attr, request, nil, true, nil)
			require.NoError(t, err)

			base, err := utils.ConvertObjectToUnstructured(request)
			require.NoError(t, err)

			assert.Equal(t, base.Object, data[engine.RequestKey],
				"prepareK8sData's request map must be byte-identical to the pre-hoist base formula")

			if op == admissionv1.Delete || op == admissionv1.Connect {
				requestMap, ok := data[engine.RequestKey].(map[string]any)
				require.True(t, ok)
				assert.Nil(t, requestMap["object"],
					"an empty admitted object on %s must surface as CEL null (a nil map entry), not a normalized empty map", op)
			}
		})
	}
}
