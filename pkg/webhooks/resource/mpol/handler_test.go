package mpol

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
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

type mockEventGen struct{}

func (m *mockEventGen) Add(...event.Info) {}

type mockEngine struct {
	matchedPolicies []string
	policies        map[string]mpolengine.Policy
	handleCalls     atomic.Int32
	handleResponse  mpolengine.EngineResponse
	handleErr       error
}

func (m *mockEngine) Handle(_ context.Context, _ celengine.EngineRequest, _ mpolengine.Predicate) (mpolengine.EngineResponse, error) {
	m.handleCalls.Add(1)
	return m.handleResponse, m.handleErr
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

type logEntry struct {
	level         int
	msg           string
	keysAndValues []any
}

func (e *logEntry) hasKey(key string) bool {
	for i := 0; i < len(e.keysAndValues); i += 2 {
		if k, ok := e.keysAndValues[i].(string); ok && k == key {
			return true
		}
	}
	return false
}

func (e *logEntry) getValue(key string) any {
	for i := 0; i < len(e.keysAndValues)-1; i += 2 {
		if k, ok := e.keysAndValues[i].(string); ok && k == key {
			return e.keysAndValues[i+1]
		}
	}
	return nil
}

type captureSink struct {
	mu      sync.Mutex
	entries []logEntry
}

func (s *captureSink) Init(logr.RuntimeInfo) {}
func (s *captureSink) Enabled(int) bool      { return true }
func (s *captureSink) Info(level int, msg string, keysAndValues ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, logEntry{level: level, msg: msg, keysAndValues: keysAndValues})
}
func (s *captureSink) Error(error, string, ...any)    {}
func (s *captureSink) WithValues(...any) logr.LogSink { return s }
func (s *captureSink) WithName(string) logr.LogSink   { return s }

func (s *captureSink) findLog(msg string) *logEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.entries {
		if e.msg == msg {
			return &e
		}
	}
	return nil
}

func TestMutate_SuccessLogging_WhenMutationOccurs(t *testing.T) {
	origResource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test",
				"namespace": "default",
			},
		},
	}
	patchedResource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test",
				"namespace": "default",
				"labels": map[string]any{
					"mutated": "true",
				},
			},
		},
	}
	engineMock := &mockEngine{
		handleResponse: mpolengine.EngineResponse{
			Resource:        origResource,
			PatchedResource: patchedResource,
			Policies: []mpolengine.MutatingPolicyResponse{
				{
					Policy: &policiesv1beta1.MutatingPolicy{
						ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
					},
					Rules: []engineapi.RuleResponse{
						*engineapi.RulePass("", engineapi.Mutation, "success", nil),
					},
				},
			},
		},
	}

	h := New(nil, engineMock, nil, &mockReportsConfig{}, &mockURGenerator{}, "", &mockEventGen{})
	sink := &captureSink{}
	logger := logr.New(sink)

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test","namespace":"default"}}`)},
			UserInfo:  authenticationv1.UserInfo{Username: "test-user"},
		},
	}

	resp := h.mutate(context.Background(), logger, request, []string{"test-policy"}, mpolengine.MatchNames("test-policy"))
	assert.NotEmpty(t, resp.Patch, "expected mutation patch to be present")

	entry := sink.findLog("mutation rules from policy applied successfully")
	assert.NotNil(t, entry, "expected success log to be emitted when mutation occurs")
	assert.Equal(t, 2, entry.level)
	assert.Equal(t, "test-policy", entry.getValue("policy"))
	assert.False(t, entry.hasKey("rules"), "rules field should not be present when rule names are empty")
}

func TestMutate_NoSuccessLogging_WhenNoMutationOccurs(t *testing.T) {
	origResource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test",
				"namespace": "default",
			},
		},
	}
	engineMock := &mockEngine{
		handleResponse: mpolengine.EngineResponse{
			Resource:        origResource,
			PatchedResource: origResource,
			Policies: []mpolengine.MutatingPolicyResponse{
				{
					Policy: &policiesv1beta1.MutatingPolicy{
						ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
					},
					Rules: []engineapi.RuleResponse{
						*engineapi.RulePass("", engineapi.Mutation, "success", nil),
					},
				},
			},
		},
	}

	h := New(nil, engineMock, nil, &mockReportsConfig{}, &mockURGenerator{}, "", &mockEventGen{})
	sink := &captureSink{}
	logger := logr.New(sink)

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test","namespace":"default"}}`)},
			UserInfo:  authenticationv1.UserInfo{Username: "test-user"},
		},
	}

	resp := h.mutate(context.Background(), logger, request, []string{"test-policy"}, mpolengine.MatchNames("test-policy"))
	assert.Empty(t, resp.Patch, "expected no mutation patch when resource unchanged")

	entry := sink.findLog("mutation rules from policy applied successfully")
	assert.Nil(t, entry, "should not emit success log when no mutation patch occurs")
}

func TestMutate_NoSuccessLogging_WhenPatchedResourceNil(t *testing.T) {
	origResource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test",
				"namespace": "default",
			},
		},
	}
	engineMock := &mockEngine{
		handleResponse: mpolengine.EngineResponse{
			Resource:        origResource,
			PatchedResource: nil,
			Policies: []mpolengine.MutatingPolicyResponse{
				{
					Policy: &policiesv1beta1.MutatingPolicy{
						ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
					},
					Rules: []engineapi.RuleResponse{
						*engineapi.RulePass("", engineapi.Mutation, "success", nil),
					},
				},
			},
		},
	}

	h := New(nil, engineMock, nil, &mockReportsConfig{}, &mockURGenerator{}, "", &mockEventGen{})
	sink := &captureSink{}
	logger := logr.New(sink)

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test","namespace":"default"}}`)},
			UserInfo:  authenticationv1.UserInfo{Username: "test-user"},
		},
	}

	resp := h.mutate(context.Background(), logger, request, []string{"test-policy"}, mpolengine.MatchNames("test-policy"))
	assert.Empty(t, resp.Patch)

	entry := sink.findLog("mutation rules from policy applied successfully")
	assert.Nil(t, entry, "should not emit success log when PatchedResource is nil")
}

func TestMutate_NoSuccessLogging_WhenPolicyExceptionPasses(t *testing.T) {
	origResource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test",
				"namespace": "default",
			},
		},
	}
	patchedResource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test",
				"namespace": "default",
				"labels": map[string]any{
					"mutated": "true",
				},
			},
		},
	}
	engineMock := &mockEngine{
		handleResponse: mpolengine.EngineResponse{
			Resource:        origResource,
			PatchedResource: patchedResource,
			Policies: []mpolengine.MutatingPolicyResponse{
				{
					Policy: &policiesv1beta1.MutatingPolicy{
						ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
					},
					Rules: []engineapi.RuleResponse{
						*engineapi.RulePass("exception", engineapi.Mutation, "passed", nil).WithExceptions([]engineapi.GenericException{
							engineapi.NewCELPolicyException(&policiesv1beta1.PolicyException{
								ObjectMeta: metav1.ObjectMeta{Name: "test-exception"},
							}),
						}),
					},
				},
			},
		},
	}

	h := New(nil, engineMock, nil, &mockReportsConfig{}, &mockURGenerator{}, "", &mockEventGen{})
	sink := &captureSink{}
	logger := logr.New(sink)

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test","namespace":"default"}}`)},
			UserInfo:  authenticationv1.UserInfo{Username: "test-user"},
		},
	}

	h.mutate(context.Background(), logger, request, []string{"test-policy"}, mpolengine.MatchNames("test-policy"))

	entry := sink.findLog("mutation rules from policy applied successfully")
	assert.Nil(t, entry, "should not emit success log for policy exceptions")
}

func TestMutate_SuccessLogging_WithNamedRule(t *testing.T) {
	origResource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test",
				"namespace": "default",
			},
		},
	}
	patchedResource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test",
				"namespace": "default",
				"labels": map[string]any{
					"mutated": "true",
				},
			},
		},
	}
	engineMock := &mockEngine{
		handleResponse: mpolengine.EngineResponse{
			Resource:        origResource,
			PatchedResource: patchedResource,
			Policies: []mpolengine.MutatingPolicyResponse{
				{
					Policy: &policiesv1beta1.MutatingPolicy{
						ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
					},
					Rules: []engineapi.RuleResponse{
						*engineapi.RulePass("add-labels", engineapi.Mutation, "success", nil),
					},
				},
			},
		},
	}

	h := New(nil, engineMock, nil, &mockReportsConfig{}, &mockURGenerator{}, "", &mockEventGen{})
	sink := &captureSink{}
	logger := logr.New(sink)

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test","namespace":"default"}}`)},
			UserInfo:  authenticationv1.UserInfo{Username: "test-user"},
		},
	}

	resp := h.mutate(context.Background(), logger, request, []string{"test-policy"}, mpolengine.MatchNames("test-policy"))
	assert.NotEmpty(t, resp.Patch)

	entry := sink.findLog("mutation rules from policy applied successfully")
	assert.NotNil(t, entry)
	assert.Equal(t, 2, entry.level)
	assert.Equal(t, "test-policy", entry.getValue("policy"))
	assert.True(t, entry.hasKey("rules"), "rules field should be present when rule has a name")
	assert.Equal(t, []string{"add-labels"}, entry.getValue("rules"))
}
