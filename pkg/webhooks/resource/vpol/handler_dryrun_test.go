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

type recordingEventGenerator struct {
	mu     sync.Mutex
	events []event.Info
}

func (r *recordingEventGenerator) Add(infoList ...event.Info) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, infoList...)
}

func (r *recordingEventGenerator) all() []event.Info {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]event.Info, len(r.events))
	copy(cp, r.events)
	return cp
}

func testVpolHandler(t *testing.T, action admissionregistrationv1.ValidationAction, recorder event.Interface) *handler {
	t.Helper()
	prevReportingCfg := reportutils.ReportingCfg
	reportutils.ReportingCfg = reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = prevReportingCfg })
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-platform-team"},
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
			Variables: []admissionregistrationv1.Variable{{
				Name:       "team",
				Expression: "object.metadata.?labels.?team.orValue('')",
			}},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "variables.team == 'platform'",
				Message:    "the team label must be platform",
			}},
		},
	}
	provider, err := vpolengine.NewProvider(vpolcompiler.NewCompiler(), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	engine := vpolengine.NewEngine(provider, func(string) *corev1.Namespace { return nil }, matching.NewMatcher())
	return New(engine, libs.NewFakeContextProvider(), fakekyvernoclient.NewSimpleClientset(), false, recorder)
}

func TestValidate_DryRunDoesNotEmitEvents(t *testing.T) {
	tests := []struct {
		name        string
		action      admissionregistrationv1.ValidationAction
		dryRun      *bool
		team        string
		wantAllowed bool
		wantEvents  int
	}{
		{
			name:        "audit policy with dryRun=true does not emit events on violation",
			action:      admissionregistrationv1.Audit,
			dryRun:      ptr.To(true),
			team:        "payments",
			wantAllowed: true,
			wantEvents:  0,
		},
		{
			name:        "audit policy with dryRun=false emits policy violation events",
			action:      admissionregistrationv1.Audit,
			dryRun:      ptr.To(false),
			team:        "payments",
			wantAllowed: true,
			wantEvents:  2,
		},
		{
			name:        "audit policy with dryRun=nil emits policy violation events",
			action:      admissionregistrationv1.Audit,
			dryRun:      nil,
			team:        "payments",
			wantAllowed: true,
			wantEvents:  2,
		},
		{
			name:        "deny policy with dryRun=true blocks and does not emit events",
			action:      admissionregistrationv1.Deny,
			dryRun:      ptr.To(true),
			team:        "payments",
			wantAllowed: false,
			wantEvents:  0,
		},
		{
			name:        "deny policy with dryRun=false blocks and emits policy fail event",
			action:      admissionregistrationv1.Deny,
			dryRun:      ptr.To(false),
			team:        "payments",
			wantAllowed: false,
			wantEvents:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &recordingEventGenerator{}
			h := testVpolHandler(t, tt.action, recorder)

			req := podRequest(tt.team)
			req.DryRun = tt.dryRun

			resp := h.ValidateClustered(context.Background(), logr.Discard(), req, "", time.Now())
			assert.Equal(t, tt.wantAllowed, resp.Allowed)

			events := recorder.all()
			assert.Len(t, events, tt.wantEvents, "unexpected event count")
		})
	}
}
