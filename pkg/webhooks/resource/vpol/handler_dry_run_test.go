package vpol

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	fakekyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/event"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// recordingEventGen captures the events admissionEvent would emit.
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
	out := make([]event.Info, len(r.events))
	copy(out, r.events)
	return out
}

// alwaysFailHandler returns a handler whose only policy fails every request.
func alwaysFailHandler(t *testing.T, action admissionregistrationv1.ValidationAction, eventGen event.Interface) *handler {
	t.Helper()
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "audit-pods"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			ValidationAction: []admissionregistrationv1.ValidationAction{action},
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
			Validations: []admissionregistrationv1.Validation{{
				Expression: "false",
				Message:    "pod changes are audited",
			}},
		},
	}
	provider, err := vpolengine.NewProvider(vpolcompiler.NewCompiler(), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	engine := vpolengine.NewEngine(provider, func(string) *corev1.Namespace { return nil }, matching.NewMatcher())
	return New(engine, libs.NewFakeContextProvider(), fakekyvernoclient.NewSimpleClientset(), false, eventGen)
}

// TestValidate_DryRunEmitsNoEvents: a server-side dry run must not record
// PolicyViolation events. The webhook declares sideEffects: NoneOnDryRun, and
// the request changes nothing. A real request still records them, and a dry
// run still returns the decision the real request would get.
func TestValidate_DryRunEmitsNoEvents(t *testing.T) {
	// audit() consults the process-wide reporting config before it emits
	// events. Unit tests do not start the controller that installs it.
	reportutils.ReportingCfg = reportutils.NewReportingConfig(nil)

	tests := []struct {
		name        string
		action      admissionregistrationv1.ValidationAction
		dryRun      bool
		wantAllowed bool
		wantEvents  bool
	}{{
		name:        "audit dry-run records nothing",
		action:      admissionregistrationv1.Audit,
		dryRun:      true,
		wantAllowed: true,
	}, {
		name:        "audit records a violation",
		action:      admissionregistrationv1.Audit,
		wantAllowed: true,
		wantEvents:  true,
	}, {
		name:   "deny dry-run still denies and records nothing",
		action: admissionregistrationv1.Deny,
		dryRun: true,
	}, {
		name:       "deny records a violation",
		action:     admissionregistrationv1.Deny,
		wantEvents: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &recordingEventGen{}
			request := podRequest("payments")
			if tt.dryRun {
				request.DryRun = ptr.To(true)
			}
			response := alwaysFailHandler(t, tt.action, gen).ValidateClustered(context.Background(), logr.Discard(), request, "", time.Now())
			assert.Equal(t, tt.wantAllowed, response.Allowed)

			events := gen.get()
			if !tt.wantEvents {
				assert.Empty(t, events)
				return
			}
			require.NotEmpty(t, events)
			for _, got := range events {
				assert.Equal(t, event.PolicyViolation, got.Reason)
			}
		})
	}
}
