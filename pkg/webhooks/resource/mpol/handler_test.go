package mpol

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
}

func (m *mockEngine) Handle(_ context.Context, _ celengine.EngineRequest, _ mpolengine.Predicate) (mpolengine.EngineResponse, error) {
	return mpolengine.EngineResponse{}, nil
}

func (m *mockEngine) Evaluate(_ context.Context, _ admission.Attributes, _ admissionv1.AdmissionRequest, _ mpolengine.Predicate) (mpolengine.EngineResponse, error) {
	return mpolengine.EngineResponse{}, nil
}

func (m *mockEngine) MatchedMutateExistingPolicies(_ context.Context, _ celengine.EngineRequest) []string {
	return m.matchedPolicies
}

func (m *mockEngine) GetCompiledPolicy(_ string) (mpolengine.Policy, error) {
	return mpolengine.Policy{}, nil
}

func TestMutate_DryRunDoesNotFireMutateExistingURs(t *testing.T) {
	urMock := &mockURGenerator{}
	engineMock := &mockEngine{matchedPolicies: []string{"test-policy"}}
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

type captureLogSink struct {
	messages []string
	levels   []int
	values   [][]interface{}
}

func (s *captureLogSink) Init(logr.RuntimeInfo) {}

func (s *captureLogSink) Enabled(int) bool {
	return true
}

func (s *captureLogSink) Info(level int, msg string, keysAndValues ...interface{}) {
	s.levels = append(s.levels, level)
	s.messages = append(s.messages, msg)
	s.values = append(s.values, append([]interface{}{}, keysAndValues...))
}

func (s *captureLogSink) Error(error, string, ...interface{}) {}

func (s *captureLogSink) WithValues(...interface{}) logr.LogSink {
	return s
}

func (s *captureLogSink) WithName(string) logr.LogSink {
	return s
}

func TestLogMutationRulesLogsOnlyRealMutations(t *testing.T) {
	sink := &captureLogSink{}
	logger := logr.New(sink)

	mutatedPolicy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "add-team-label"},
	}
	unmutatedPolicy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "no-change"},
	}
	skippedPolicy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "skipped-policy"},
	}

	logMutationRules(logger, []mpolengine.MutatingPolicyResponse{
		{
			Policy:  mutatedPolicy,
			Mutated: true,
			Rules: []engineapi.RuleResponse{
				*engineapi.RulePass("", engineapi.Mutation, "success", nil),
			},
		},
		{
			Policy:  unmutatedPolicy,
			Mutated: false,
			Rules: []engineapi.RuleResponse{
				*engineapi.RulePass("", engineapi.Mutation, "success", nil),
			},
		},
		{
			Policy:  skippedPolicy,
			Mutated: true,
			Rules: []engineapi.RuleResponse{
				*engineapi.RuleSkip("", engineapi.Mutation, "skip", nil),
			},
		},
	})

	assert.Len(t, sink.messages, 1)
	assert.Equal(t, 2, sink.levels[0])
	assert.Equal(t, "mutation rules from policy applied successfully", sink.messages[0])
	assert.Equal(t, "add-team-label", valueForKey(sink.values[0], "policy"))
	assert.Nil(t, valueForKey(sink.values[0], "rules"), "MutatingPolicy rules are unnamed, so the log should not add a fake empty rule")
}

func valueForKey(values []interface{}, key string) interface{} {
	for i := 0; i < len(values)-1; i += 2 {
		if values[i] == key {
			return values[i+1]
		}
	}
	return nil
}
