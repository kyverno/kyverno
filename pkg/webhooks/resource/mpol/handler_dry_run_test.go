package mpol

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	fakekyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// recordingEventGen captures the events the handler emits. audit() runs in its
// own goroutine, so it is read and written under a lock.
type recordingEventGen struct {
	mu     sync.Mutex
	events []event.Info
}

func (r *recordingEventGen) Add(infos ...event.Info) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, infos...)
}

func (r *recordingEventGen) get() []event.Info {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]event.Info(nil), r.events...)
}

// eventTestHandler returns a handler with one Ignore policy that applies the
// given applyConfiguration expression to deployments, and the events it emits.
func eventTestHandler(t *testing.T, expression string) (*handler, *recordingEventGen) {
	t.Helper()
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: labelPolicy},
		Spec: policiesv1beta1.MutatingPolicySpec{
			FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Resources:   []string{"deployments"},
						},
					},
				}},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType:          admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: expression},
			}},
		},
	}
	provider, err := mpolengine.NewProvider(mpolcompiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{policy}, nil, libs.NewFakeContextProvider())
	require.NoError(t, err)
	engine := mpolengine.NewEngine(
		provider,
		func(ns string) *corev1.Namespace { return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}} },
		matching.NewMatcher(),
		&gateBenchTypeConverter{},
		&libs.FakeContextProvider{},
	)
	events := &recordingEventGen{}
	return New(nil, engine, fakekyvernoclient.NewSimpleClientset(), &mockReportsConfig{}, &mockURGenerator{}, "", events), events
}

// TestMutate_DryRunEmitsNoEvents: a server-side dry run must not record events.
// The webhook declares sideEffects: NoneOnDryRun, and the request changes
// nothing. A real request still records them, and a dry run still returns the
// patch the real request would get.
func TestMutate_DryRunEmitsNoEvents(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		dryRun     bool
		wantPatch  bool
		wantReason event.Reason
	}{{
		name:       "applied mutation records PolicyApplied",
		expression: `Object{metadata: Object.metadata{labels: {"team": "platform"}}}`,
		wantPatch:  true,
		wantReason: event.PolicyApplied,
	}, {
		name:       "applied mutation in a dry run records nothing",
		expression: `Object{metadata: Object.metadata{labels: {"team": "platform"}}}`,
		dryRun:     true,
		wantPatch:  true,
	}, {
		name:       "ignored mutation error records PolicyViolation",
		expression: `Object{metadata: Object.metadata{labels: {"team": object.missing}}}`,
		wantReason: event.PolicyViolation,
	}, {
		name:       "ignored mutation error in a dry run records nothing",
		expression: `Object{metadata: Object.metadata{labels: {"team": object.missing}}}`,
		dryRun:     true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, events := eventTestHandler(t, tt.expression)
			request := deploymentRequest()
			if !tt.dryRun {
				request.DryRun = nil
			}
			response := h.mutate(context.Background(), logr.Discard(), request, []string{labelPolicy}, mpolengine.MatchNames(labelPolicy))
			assert.True(t, response.Allowed)
			assert.Equal(t, tt.wantPatch, len(response.Patch) > 0)

			if tt.dryRun {
				// audit() would run in a goroutine, so give it time to emit
				assert.Never(t, func() bool { return len(events.get()) > 0 }, 200*time.Millisecond, 10*time.Millisecond)
				return
			}
			require.Eventually(t, func() bool { return len(events.get()) > 0 }, 5*time.Second, 10*time.Millisecond)
			for _, got := range events.get() {
				assert.Equal(t, tt.wantReason, got.Reason)
			}
		})
	}
}
