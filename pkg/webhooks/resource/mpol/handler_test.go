package mpol

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"
)

type mockURGenerator struct {
	called atomic.Int32
}

func (m *mockURGenerator) Apply(_ context.Context, _ kyvernov2.UpdateRequestSpec) error {
	m.called.Add(1)
	return nil
}

type mockReportsConfig struct{}

func (m *mockReportsConfig) ValidateReportsEnabled() bool                { return false }
func (m *mockReportsConfig) MutateReportsEnabled() bool                  { return false }
func (m *mockReportsConfig) MutateExistingReportsEnabled() bool          { return false }
func (m *mockReportsConfig) ImageVerificationReportsEnabled() bool       { return false }
func (m *mockReportsConfig) GenerateReportsEnabled() bool                { return false }
func (m *mockReportsConfig) IsStatusAllowed(_ engineapi.RuleStatus) bool { return false }

type mockEngine struct {
	matchedPolicies []string
	policies        map[string]mpolengine.Policy
	handleCalls     atomic.Int32
}

func TestJSONMutationAdmissionFailurePolicy(t *testing.T) {
	for _, failurePolicy := range []admissionregistrationv1.FailurePolicyType{admissionregistrationv1.Ignore, admissionregistrationv1.Fail} {
		for _, mutation := range []struct {
			name       string
			expression string
		}{
			{"evaluation error", `[JSONPatch{op:"add",path:"/data/value",value:object.missing}]`},
			{"patch error", `[JSONPatch{op:"remove",path:"/missing"}]`},
		} {
			t.Run(string(failurePolicy)+"/"+mutation.name, func(t *testing.T) {
				policy := &policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "json-policy"},
					Spec: policiesv1beta1.MutatingPolicySpec{
						FailurePolicy:           ptr.To(failurePolicy),
						EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
						Mutations: []admissionregistrationv1alpha1.Mutation{{
							PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
							JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: mutation.expression},
						}},
					},
				}
				provider, err := mpolengine.NewProvider(mpolcompiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{policy}, nil, nil)
				require.NoError(t, err)
				engine := mpolengine.NewEngine(provider, nil, matching.NewMatcher(), nil, nil)
				request := celengine.RequestFromAdmission(nil, admissionv1.AdmissionRequest{
					UID:       "test-uid",
					Name:      "input",
					Operation: admissionv1.Create,
					Resource:  metav1.GroupVersionResource{Version: "v1", Resource: "configmaps"},
					Kind:      metav1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
					Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"input"},"data":{}}`)},
				})
				result, err := engine.Handle(context.Background(), request, nil)
				require.NoError(t, err, "admission evaluation must return policy errors as rule results")
				require.Len(t, result.Policies, 1)
				require.Len(t, result.Policies[0].Rules, 1)
				require.Equal(t, engineapi.RuleStatusError, result.Policies[0].Rules[0].Status())
				require.Nil(t, result.PatchedResource)

				response, err := (&handler{}).admissionResponse(request, result)
				require.Equal(t, failurePolicy == admissionregistrationv1.Ignore, response.Allowed)
				require.Equal(t, failurePolicy == admissionregistrationv1.Fail, err != nil)
				require.Empty(t, response.Patch)
			})
		}
	}
}

func (m *mockEngine) Handle(_ context.Context, _ celengine.EngineRequest, _ mpolengine.Predicate) (mpolengine.EngineResponse, error) {
	m.handleCalls.Add(1)
	return mpolengine.EngineResponse{}, nil
}

func (m *mockEngine) Evaluate(_ context.Context, _ admission.Attributes, _ admissionv1.AdmissionRequest, _ mpolengine.Predicate) (mpolengine.EngineResponse, error) {
	return mpolengine.EngineResponse{}, nil
}

func (m *mockEngine) MatchedMutateExistingPolicies(_ context.Context, _ celengine.EngineRequest) []string {
	return m.matchedPolicies
}

func (m *mockEngine) GetCompiledPolicy(name string) (mpolengine.Policy, error) {
	if p, ok := m.policies[name]; ok {
		return p, nil
	}
	return mpolengine.Policy{}, fmt.Errorf("policy %s not found", name)
}

func (m *mockEngine) GetCompiledPolicies(names ...string) map[string]mpolengine.Policy {
	policies := make(map[string]mpolengine.Policy, len(names))
	for _, name := range names {
		if p, ok := m.policies[name]; ok {
			policies[name] = p
		}
	}
	return policies
}

func TestMutate_DryRunDoesNotFireMutateExistingURs(t *testing.T) {
	urMock := &mockURGenerator{}
	engineMock := &mockEngine{
		matchedPolicies: []string{"test-policy"},
		policies: map[string]mpolengine.Policy{
			"test-policy": {
				Policy: &policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
				},
			},
		},
	}
	h := New(nil, engineMock, nil, &mockReportsConfig{}, urMock, "system:serviceaccount:kyverno:kyverno-background-controller", nil)

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			DryRun:    ptr.To(true),
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test","namespace":"default"}}`)},
			UserInfo:  authenticationv1.UserInfo{Username: "test-user"},
		},
	}

	h.mutate(context.Background(), logr.Discard(), request, []string{"test-policy"}, mpolengine.MatchNames("test-policy"))

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), urMock.called.Load(), "dry-run request must not create mutate-existing UpdateRequests")
}

func TestMutate_BackgroundRequestSkippedByDefault(t *testing.T) {
	urMock := &mockURGenerator{}
	backgroundSA := "system:serviceaccount:kyverno:kyverno-background-controller"
	engineMock := &mockEngine{
		matchedPolicies: []string{"test-policy"},
		policies: map[string]mpolengine.Policy{
			"test-policy": {
				Policy: &policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
				},
			},
		},
	}
	h := New(nil, engineMock, nil, &mockReportsConfig{}, urMock, backgroundSA, nil)

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test","namespace":"default"}}`)},
			UserInfo:  authenticationv1.UserInfo{Username: backgroundSA},
		},
	}

	h.mutate(context.Background(), logr.Discard(), request, []string{"test-policy"}, mpolengine.MatchNames("test-policy"))

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), engineMock.handleCalls.Load(), "background request should skip policy by default")
	assert.Equal(t, int32(0), urMock.called.Load(), "background request should not create UR by default")
}

func TestMutate_BackgroundRequestAllowedWhenDisabled(t *testing.T) {
	urMock := &mockURGenerator{}
	backgroundSA := "system:serviceaccount:kyverno:kyverno-background-controller"
	engineMock := &mockEngine{
		matchedPolicies: []string{"test-policy"},
		policies: map[string]mpolengine.Policy{
			"test-policy": {
				Policy: &policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
					Spec: policiesv1beta1.MutatingPolicySpec{
						EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
							SkipBackgroundRequests: ptr.To(false),
						},
					},
				},
			},
		},
	}
	h := New(nil, engineMock, nil, &mockReportsConfig{}, urMock, backgroundSA, nil)

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test","namespace":"default"}}`)},
			UserInfo:  authenticationv1.UserInfo{Username: backgroundSA},
		},
	}

	h.mutate(context.Background(), logr.Discard(), request, []string{"test-policy"}, mpolengine.MatchNames("test-policy"))

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(1), engineMock.handleCalls.Load(), "background request should evaluate policy when explicitly disabled")
	assert.Equal(t, int32(1), urMock.called.Load(), "background request should create UR when explicitly disabled")
}

// Admission-disabled policies are excluded from admission-driven UR creation
// inside the engine's MatchesMutateExisting (see the reconciler/provider tests);
// the handler creates URs for whatever the engine returns.

func TestLogAppliedMutations(t *testing.T) {
	resource := func(labels map[string]string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap"}}
		u.SetLabels(labels)
		return u
	}
	policy := func(name string, patched *unstructured.Unstructured) mpolengine.MutatingPolicyResponse {
		return mpolengine.MutatingPolicyResponse{
			Policy:          &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: name}},
			PatchedResource: patched,
		}
	}
	afterFirst := resource(map[string]string{"team": "platform"})
	var log capturedLog
	logAppliedMutations(log.logger(2), mpolengine.EngineResponse{
		Resource: resource(nil),
		Policies: []mpolengine.MutatingPolicyResponse{
			policy("adds-label", afterFirst),
			policy("skipped", nil),
			policy("no-change", afterFirst.DeepCopy()),
			policy("adds-another-label", resource(map[string]string{"team": "platform", "env": "prod"})),
		},
	})
	var logged []any
	for _, line := range log.lines {
		if line["msg"] == "mutation rules from policy applied successfully" {
			logged = append(logged, line["policy"])
		}
	}
	assert.Equal(t, []any{"adds-label", "adds-another-label"}, logged)
}
