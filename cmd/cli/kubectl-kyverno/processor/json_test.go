package processor

import (
	"encoding/json"
	"io"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/payload"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func jsonMutationPolicy(t *testing.T, expression string) *policiesv1beta1.MutatingPolicy {
	t.Helper()
	expressionJSON, err := json.Marshal(expression)
	require.NoError(t, err)
	var policy policiesv1beta1.MutatingPolicy
	require.NoError(t, json.Unmarshal([]byte(`{
		"apiVersion":"policies.kyverno.io/v1","kind":"MutatingPolicy","metadata":{"name":"transform"},
		"spec":{"evaluation":{"mode":"JSON"},"mutations":[{"patchType":"JSONPatch","jsonPatch":{"expression":`+string(expressionJSON)+`}}]}
	}`), &policy))
	return &policy
}

func TestJSONMutationRoots(t *testing.T) {
	t.Parallel()
	for _, input := range []string{`{"id":9007199254740993}`, `[1,"a",null]`, `"hello"`, `true`, `9007199254740993`, `null`} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			document := &payload.Document{Name: "input.json", Raw: json.RawMessage(input)}
			rc := &ResultCounts{}
			processor := PolicyProcessor{
				JSONDocument: document, Rc: rc, Out: io.Discard,
				MutatingPolicies: []policiesv1beta1.MutatingPolicyLike{
					jsonMutationPolicy(t, `[JSONPatch{op: "replace", path: "", value: [object]}]`),
				},
			}
			responses, err := processor.ApplyPoliciesOnResource()
			require.NoError(t, err)
			require.Equal(t, "["+input+"]", string(document.Raw))
			require.Equal(t, 1, rc.Pass)
			require.Empty(t, responses[0].Resource.Object)
			require.Empty(t, responses[0].PatchedResource.Object)
		})
	}
}

func TestJSONMutationFailureAndModeMismatch(t *testing.T) {
	t.Parallel()
	document := &payload.Document{Name: "input.json", Raw: json.RawMessage(`{"id":9007199254740993}`)}
	policy := jsonMutationPolicy(t, `[JSONPatch{op: "replace", path: "/absent", value: true}]`)
	rc := &ResultCounts{}
	processor := PolicyProcessor{JSONDocument: document, Rc: rc, Out: io.Discard, MutatingPolicies: []policiesv1beta1.MutatingPolicyLike{policy}}
	responses, err := processor.ApplyPoliciesOnResource()
	require.Error(t, err)
	require.Equal(t, `{"id":9007199254740993}`, string(document.Raw))
	require.Equal(t, 1, rc.Error)
	require.Equal(t, engineapi.RuleStatusError, responses[0].PolicyResponse.Rules[0].Status())
	require.Empty(t, processor.JSONPatchedDocuments)

	policy.Spec.EvaluationConfiguration = nil
	rc = &ResultCounts{}
	processor.Rc = rc
	responses, err = processor.ApplyPoliciesOnResource()
	require.NoError(t, err)
	require.Equal(t, 1, rc.Skip)
	require.Equal(t, engineapi.RuleStatusSkip, responses[0].PolicyResponse.Rules[0].Status())
}

func TestJSONMutationTestRollback(t *testing.T) {
	t.Parallel()
	policy := jsonMutationPolicy(t, `[JSONPatch{op: "add", path: "/unwanted", value: true}, JSONPatch{op: "test", path: "/id", value: 0}]`)
	document := &payload.Document{Name: "input.json", Raw: json.RawMessage(`{"id":9007199254740993}`)}
	rc := &ResultCounts{}
	processor := PolicyProcessor{JSONDocument: document, Rc: rc, Out: io.Discard, MutatingPolicies: []policiesv1beta1.MutatingPolicyLike{policy}}
	responses, err := processor.ApplyPoliciesOnResource()
	require.NoError(t, err)
	require.Equal(t, `{"id":9007199254740993}`, string(document.Raw))
	require.Equal(t, 1, rc.Skip)
	require.Equal(t, engineapi.RuleStatusSkip, responses[0].PolicyResponse.Rules[0].Status())
	require.Equal(t, string(document.Raw), string(processor.JSONPatchedDocuments["transform"].Raw))
}

func TestJSONMutationValidationRequiresObjectRoot(t *testing.T) {
	t.Parallel()
	var validation policiesv1beta1.ValidatingPolicy
	require.NoError(t, json.Unmarshal([]byte(`{
		"apiVersion":"policies.kyverno.io/v1","kind":"ValidatingPolicy","metadata":{"name":"validate"},
		"spec":{"evaluation":{"mode":"JSON"},"validations":[{"expression":"true"}]}
	}`), &validation))
	document := &payload.Document{Name: "input.json", Raw: json.RawMessage(`{}`)}
	rc := &ResultCounts{}
	processor := PolicyProcessor{
		JSONDocument: document, Rc: rc, Out: io.Discard,
		MutatingPolicies:   []policiesv1beta1.MutatingPolicyLike{jsonMutationPolicy(t, `[JSONPatch{op: "replace", path: "", value: null}]`)},
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{&validation},
	}
	_, err := processor.ApplyPoliciesOnResource()
	require.ErrorContains(t, err, "requires an object root")
	require.Equal(t, 1, rc.Error)
}

func TestJSONMutationExceptionResult(t *testing.T) {
	t.Parallel()
	var exception policiesv1beta1.PolicyException
	require.NoError(t, json.Unmarshal([]byte(`{
		"apiVersion":"policies.kyverno.io/v1","kind":"PolicyException","metadata":{"name":"exempt"},
		"spec":{"policyRefs":[{"kind":"MutatingPolicy","name":"transform"}],"matchConditions":[{"name":"all","expression":"true"}]}
	}`), &exception))
	document := &payload.Document{Name: "input.json", Raw: json.RawMessage(`null`)}
	rc := &ResultCounts{}
	processor := PolicyProcessor{
		JSONDocument: document, Rc: rc,
		MutatingPolicies: []policiesv1beta1.MutatingPolicyLike{jsonMutationPolicy(t, `[JSONPatch{op: "replace", path: "", value: true}]`)},
		CELExceptions:    []*policiesv1beta1.PolicyException{&exception},
	}
	responses, err := processor.ApplyPoliciesOnResource()
	require.NoError(t, err)
	require.Equal(t, "null", string(document.Raw))
	require.Equal(t, 1, rc.Skip)
	require.Len(t, responses[0].PolicyResponse.Rules[0].Exceptions(), 1)
}

// Policy families other than CEL MutatingPolicies keep running through the
// existing object-root JSON payload pipeline after JSON mutation.
func TestJSONMutationKeepsOtherPolicyFamilies(t *testing.T) {
	t.Parallel()
	var vap admissionregistrationv1.ValidatingAdmissionPolicy
	require.NoError(t, json.Unmarshal([]byte(`{"metadata":{"name":"vap"},"spec":{"validations":[{"expression":"true"}]}}`), &vap))
	newProcessor := func(raw string) PolicyProcessor {
		return PolicyProcessor{
			Store: &store.Store{}, JSONDocument: &payload.Document{Name: "input.json", Raw: json.RawMessage(raw)}, Rc: &ResultCounts{}, Out: io.Discard,
			MutatingPolicies:            []policiesv1beta1.MutatingPolicyLike{jsonMutationPolicy(t, `[JSONPatch{op: "add", path: "/-", value: 1}]`)},
			ValidatingAdmissionPolicies: []admissionregistrationv1.ValidatingAdmissionPolicy{vap},
		}
	}
	// An object root reaches the VAP evaluation, which, as before JSON mutation
	// support, cannot resolve a GVR for a document without Kubernetes identity.
	processor := newProcessor(`{"list":[]}`)
	processor.MutatingPolicies = []policiesv1beta1.MutatingPolicyLike{jsonMutationPolicy(t, `[JSONPatch{op: "add", path: "/list/-", value: 1}]`)}
	_, err := processor.ApplyPoliciesOnResource()
	require.ErrorContains(t, err, "failed to map gvk to gvr")
	// These families require an object root, so a non-object result is an
	// explicit error rather than silently skipping them.
	processor = newProcessor(`[]`)
	_, err = processor.ApplyPoliciesOnResource()
	require.ErrorContains(t, err, "requires an object root")
	require.Equal(t, 1, processor.Rc.Error)
}
