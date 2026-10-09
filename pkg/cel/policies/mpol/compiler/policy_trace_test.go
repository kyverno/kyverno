package compiler

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/authentication/user"
)

// mutTraceAttrs is a parameterized admission.Attributes, unlike the fixed mockAttributes in
// policy_test.go: these tests need a real *unstructured.Unstructured Pod (matching what the real
// mutation-application pipeline -- ApplyStructuredMergeDiff -- actually produces, as verified
// live via `kyverno apply`), with a namespace that varies per test case.
type mutTraceAttrs struct {
	obj *unstructured.Unstructured
}

func (m *mutTraceAttrs) GetName() string      { return m.obj.GetName() }
func (m *mutTraceAttrs) GetNamespace() string { return m.obj.GetNamespace() }
func (m *mutTraceAttrs) GetResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{Version: "v1", Resource: "pods"}
}
func (m *mutTraceAttrs) GetSubresource() string              { return "" }
func (m *mutTraceAttrs) GetOperation() admission.Operation   { return admission.Create }
func (m *mutTraceAttrs) GetOperationOptions() runtime.Object { return nil }
func (m *mutTraceAttrs) IsDryRun() bool                      { return false }
func (m *mutTraceAttrs) GetObject() runtime.Object           { return m.obj }
func (m *mutTraceAttrs) GetOldObject() runtime.Object        { return nil }
func (m *mutTraceAttrs) GetKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Version: "v1", Kind: "Pod"}
}
func (m *mutTraceAttrs) GetUserInfo() user.Info                { return &user.DefaultInfo{} }
func (m *mutTraceAttrs) AddAnnotation(key, value string) error { return nil }
func (m *mutTraceAttrs) AddAnnotationWithLevel(key, value string, level auditinternal.Level) error {
	return nil
}
func (m *mutTraceAttrs) GetReinvocationContext() admission.ReinvocationContext { return nil }

// podObject builds a fresh Pod each call -- callers that compare a traced and an untraced run
// against "the same" object must call this twice, not share one pointer, since the real merge
// pipeline may not leave its input untouched.
func podObject(namespace string, labels map[string]any) *unstructured.Unstructured {
	metadata := map[string]any{"name": "nginx", "namespace": namespace}
	if labels != nil {
		metadata["labels"] = labels
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   metadata,
		"spec": map[string]any{
			"containers": []any{map[string]any{"name": "web", "image": "nginx"}},
		},
	}}
}

func applyConfigMutation(expr string) admissionregistrationv1alpha1.Mutation {
	return admissionregistrationv1alpha1.Mutation{
		PatchType:          admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
		ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: expr},
	}
}

func buildMutationTracePolicy(mutations ...admissionregistrationv1alpha1.Mutation) *policiesv1beta1.MutatingPolicy {
	return &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "trace-test"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "not-kube-system",
				Expression: "object.metadata.namespace != 'kube-system'",
			}},
			Variables: []admissionregistrationv1.Variable{{
				Name:       "teamLabel",
				Expression: "'platform'",
			}},
			Mutations: mutations,
		},
	}
}

func compileMutAndEvaluate(t *testing.T, traced bool, policy *policiesv1beta1.MutatingPolicy, obj *unstructured.Unstructured) *EvaluationResult {
	t.Helper()
	p, errs := NewCompilerWithTrace(traced).Compile(policy, nil)
	require.Empty(t, errs)
	return p.Evaluate(context.Background(), &mutTraceAttrs{obj: obj}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, nil, &libs.FakeContextProvider{})
}

const addTeamLabelExpr = `Object{metadata: Object.metadata{labels: {"team": variables.teamLabel}}}`

func TestEvaluate_TracingOff_NoTrace(t *testing.T) {
	policy := buildMutationTracePolicy(applyConfigMutation(addTeamLabelExpr))

	res := compileMutAndEvaluate(t, false, policy, podObject("prod", nil))
	require.NotNil(t, res)
	require.NoError(t, res.Error)
	require.NotNil(t, res.PatchedResource)
	assert.Nil(t, res.Trace, "no trace when the policy was compiled without tracing")
}

func TestEvaluate_TracingOn_Pass(t *testing.T) {
	policy := buildMutationTracePolicy(applyConfigMutation(addTeamLabelExpr))

	res := compileMutAndEvaluate(t, true, policy, podObject("prod", nil))
	require.NotNil(t, res)
	require.NoError(t, res.Error)
	require.NotNil(t, res.Trace)

	require.Len(t, res.Trace.Match, 1)
	assert.Equal(t, "not-kube-system", res.Trace.Match[0].Name)
	assert.Equal(t, "true", res.Trace.Match[0].Result)

	require.Len(t, res.Trace.Variables, 1)
	assert.Equal(t, "teamLabel", res.Trace.Variables[0].Name)
	assert.Equal(t, "platform", res.Trace.Variables[0].Result)

	require.Len(t, res.Trace.Mutations, 1)
	assert.Equal(t, "mutations[0] (applyConfiguration)", res.Trace.Mutations[0].Name)
	assert.Empty(t, res.Trace.Mutations[0].Error)
	assert.Contains(t, res.Trace.Mutations[0].Result, "platform")

	assert.Equal(t, trace.VerdictPass, res.Trace.Verdict.Status)

	// tracing must not change the real outcome: the mutation actually applied
	labels, _, err := unstructured.NestedStringMap(res.PatchedResource.Object, "metadata", "labels")
	require.NoError(t, err)
	assert.Equal(t, "platform", labels["team"])
}

func TestEvaluate_TracingOn_MatchConditionFalseSkipsPolicy(t *testing.T) {
	policy := buildMutationTracePolicy(applyConfigMutation(addTeamLabelExpr))

	traced := compileMutAndEvaluate(t, true, policy, podObject("kube-system", nil))
	require.NotNil(t, traced)
	assert.True(t, traced.Skipped)
	assert.Nil(t, traced.PatchedResource)
	require.NotNil(t, traced.Trace)
	assert.Equal(t, trace.VerdictSkip, traced.Trace.Verdict.Status)
	assert.Contains(t, traced.Trace.Verdict.Message, "not-kube-system")
	require.Len(t, traced.Trace.Match, 1)
	assert.Equal(t, "false", traced.Trace.Match[0].Result)
	assert.Empty(t, traced.Trace.Variables, "no variable is read once a match condition excludes the resource")
	assert.Empty(t, traced.Trace.Mutations, "no mutation runs once a match condition excludes the resource")

	// with tracing off the behavior is unchanged: a skip is a nil result
	assert.Nil(t, compileMutAndEvaluate(t, false, policy, podObject("kube-system", nil)))
}

func TestEvaluate_TracingOn_MutationErrorIsCapturedOnTheMutation(t *testing.T) {
	// references a label that doesn't exist at all -- a genuine CEL runtime error, not a Go
	// compile-time error, the same distinction TestBuild_ErrorNodesNeverCarryAValue exercises
	// for validations in pkg/cel/trace.
	policy := buildMutationTracePolicy(applyConfigMutation(
		`Object{metadata: Object.metadata{labels: {"copied": object.metadata.labels.owner}}}`,
	))

	res := compileMutAndEvaluate(t, true, policy, podObject("prod", nil)) // no labels at all
	require.NotNil(t, res)
	require.Error(t, res.Error)
	require.NotNil(t, res.Trace)
	assert.Equal(t, trace.VerdictError, res.Trace.Verdict.Status)

	require.Len(t, res.Trace.Mutations, 1)
	mt := res.Trace.Mutations[0]
	assert.NotEmpty(t, mt.Error, "the mutation-level Error must be set, not just buried in a node")
	require.NotEmpty(t, mt.Nodes, "the failing mutation should carry its per-node breakdown")

	var sawErrorNode bool
	for _, n := range mt.Nodes {
		if n.Error != "" {
			sawErrorNode = true
			assert.Empty(t, n.Value, "an errored node must not also carry a value")
		}
	}
	assert.True(t, sawErrorNode, "expected at least one node with Error populated")
}

func TestEvaluate_TracingOn_MatchesTracingOffOutcome(t *testing.T) {
	tests := []struct {
		name      string
		mutation  string
		namespace string
		want      string // "mutated", "skipped" or "error"
	}{
		{name: "mutation applies", mutation: addTeamLabelExpr, namespace: "prod", want: "mutated"},
		{name: "match condition skips", mutation: addTeamLabelExpr, namespace: "kube-system", want: "skipped"},
		{
			// the runtime-error policy from TestEvaluate_TracingOn_MutationErrorIsCapturedOnTheMutation
			name:      "mutation errors",
			mutation:  `Object{metadata: Object.metadata{labels: {"copied": object.metadata.labels.owner}}}`,
			namespace: "prod",
			want:      "error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := buildMutationTracePolicy(applyConfigMutation(tt.mutation))
			// separate podObject() calls per run: the real merge pipeline is not guaranteed to
			// leave its input object untouched, so sharing one pointer between the traced and
			// untraced run could make one run see the other's mutation.
			off := compileMutAndEvaluate(t, false, policy, podObject(tt.namespace, nil))
			on := compileMutAndEvaluate(t, true, policy, podObject(tt.namespace, nil))
			require.NotNil(t, on)

			switch tt.want {
			case "skipped":
				// without tracing a skip is a nil result; with it, a result flagged Skipped
				assert.Nil(t, off)
				assert.True(t, on.Skipped)
				assert.Nil(t, on.PatchedResource)
			case "error":
				require.NotNil(t, off)
				require.Error(t, off.Error)
				require.Error(t, on.Error)
				assert.Equal(t, off.Error.Error(), on.Error.Error(), "tracing must not change the error")
				assert.Nil(t, off.PatchedResource)
				assert.Nil(t, on.PatchedResource, "a failed mutation must not hand back a patched resource")
			case "mutated":
				require.NotNil(t, off)
				require.NoError(t, off.Error)
				require.NoError(t, on.Error)
				require.NotNil(t, off.PatchedResource)
				require.NotNil(t, on.PatchedResource)
				assert.Equal(t, off.PatchedResource.Object, on.PatchedResource.Object)
			}
		})
	}
}

// TestEvaluate_TracingKeepsTheCostLimit: in cel-go v0.31 a program that tracks state does not
// enforce the per-call cost limit, so the outcome must come from the normal program and the
// tracking twin is only re-run to explain. An expression over the limit is an error with tracing
// off, and must stay the same error with it on, wherever it appears in the policy.
func TestEvaluate_TracingKeepsTheCostLimit(t *testing.T) {
	const items = "[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20]"
	costly := items + ".all(a, " + items + ".all(b, " + items + ".all(c, " + items + ".all(d, " + items + ".all(e, a+b+c+d+e > 0)))))"
	tests := map[string]func(p *policiesv1beta1.MutatingPolicy){
		"in an applyConfiguration mutation": func(p *policiesv1beta1.MutatingPolicy) {
			p.Spec.Mutations = []admissionregistrationv1alpha1.Mutation{applyConfigMutation(
				`Object{metadata: Object.metadata{labels: {"checked": ` + costly + ` ? "yes" : "no"}}}`,
			)}
		},
		"in a jsonPatch mutation": func(p *policiesv1beta1.MutatingPolicy) {
			p.Spec.Mutations = []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels", value: {"checked": ` + costly + ` ? "yes" : "no"}}]`,
				},
			}}
		},
		"in a variable": func(p *policiesv1beta1.MutatingPolicy) {
			p.Spec.Variables = []admissionregistrationv1.Variable{{Name: "teamLabel", Expression: costly + ` ? "platform" : "other"`}}
			p.Spec.Mutations = []admissionregistrationv1alpha1.Mutation{applyConfigMutation(addTeamLabelExpr)}
		},
		"in a match condition": func(p *policiesv1beta1.MutatingPolicy) {
			p.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "expensive", Expression: costly}}
			p.Spec.Mutations = []admissionregistrationv1alpha1.Mutation{applyConfigMutation(addTeamLabelExpr)}
		},
	}
	for name, configure := range tests {
		t.Run(name, func(t *testing.T) {
			policy := buildMutationTracePolicy()
			configure(policy)

			untraced := compileMutAndEvaluate(t, false, policy, podObject("prod", nil))
			traced := compileMutAndEvaluate(t, true, policy, podObject("prod", nil))
			require.NotNil(t, untraced)
			require.NotNil(t, traced)

			require.Error(t, untraced.Error, "the cost limit must stop the expression")
			require.Error(t, traced.Error, "tracing must not lift the cost limit")
			assert.Equal(t, untraced.Error.Error(), traced.Error.Error())
			assert.Contains(t, traced.Error.Error(), "cost limit exceeded")
			assert.Nil(t, traced.PatchedResource, "nothing may be patched once the limit is hit")

			require.NotNil(t, traced.Trace)
			assert.Equal(t, trace.VerdictError, traced.Trace.Verdict.Status)
			assert.Contains(t, traced.Trace.Verdict.Message, "cost limit exceeded")
		})
	}
}

// TestEvaluate_TracingOn_SkipShowsTheVariablesAMatchConditionRead: a MutatingPolicy binds its
// variables before its match conditions, so a condition such as !variables.isSystem can skip the
// policy. The skip trace must then show the variable the condition read, not just the condition,
// for the trigger's match conditions and for the target ones alike.
func TestEvaluate_TracingOn_SkipShowsTheVariablesAMatchConditionRead(t *testing.T) {
	isSystem := admissionregistrationv1.Variable{Name: "isSystem", Expression: "object.metadata.namespace == 'kube-system'"}
	condition := admissionregistrationv1.MatchCondition{Name: "not-system", Expression: "!variables.isSystem"}
	tests := map[string]struct {
		configure func(p *policiesv1beta1.MutatingPolicy)
		evaluate  func(p *Policy, obj *unstructured.Unstructured) *EvaluationResult
	}{
		"match conditions": {
			configure: func(p *policiesv1beta1.MutatingPolicy) {
				p.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{condition}
			},
			evaluate: func(p *Policy, obj *unstructured.Unstructured) *EvaluationResult {
				return p.Evaluate(context.Background(), &mutTraceAttrs{obj: obj}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, nil, &libs.FakeContextProvider{})
			},
		},
		"target match conditions": {
			configure: func(p *policiesv1beta1.MutatingPolicy) {
				p.Spec.TargetMatchConditions = []admissionregistrationv1.MatchCondition{condition}
			},
			evaluate: func(p *Policy, obj *unstructured.Unstructured) *EvaluationResult {
				return p.EvaluateTarget(context.Background(), &mutTraceAttrs{obj: obj}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, nil, &libs.FakeContextProvider{})
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			policy := buildMutationTracePolicy(applyConfigMutation(addTeamLabelExpr))
			policy.Spec.Variables = append(policy.Spec.Variables, isSystem)
			tt.configure(policy)
			compiled, errs := NewCompilerWithTrace(true).Compile(policy, nil)
			require.Empty(t, errs)

			res := tt.evaluate(compiled, podObject("kube-system", nil))
			require.NotNil(t, res)
			assert.True(t, res.Skipped)
			require.NotNil(t, res.Trace)
			assert.Equal(t, trace.VerdictSkip, res.Trace.Verdict.Status)
			var read []string
			for _, v := range res.Trace.Variables {
				read = append(read, v.Name+"="+v.Result)
			}
			assert.Equal(t, []string{"isSystem=true"}, read, "the skip shows the variable the condition read, and only that one")
		})
	}
}
