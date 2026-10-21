package vpol

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	fakekyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// capturedLog collects what a logger writes. The decision traces are logged from a goroutine,
// so it is read and written under a lock.
type capturedLog struct {
	mu    sync.Mutex
	lines []map[string]any
}

func (c *capturedLog) logger(verbosity int) logr.Logger {
	return funcr.NewJSON(func(obj string) {
		var line map[string]any
		if err := json.Unmarshal([]byte(obj), &line); err != nil {
			line = map[string]any{"undecodable": obj}
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		c.lines = append(c.lines, line)
	}, funcr.Options{Verbosity: verbosity})
}

// decisions returns the lines written under the decision-trace logger.
func (c *capturedLog) decisions() []map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	var decisions []map[string]any
	for _, line := range c.lines {
		if line["logger"] == "decision-trace" {
			decisions = append(decisions, line)
		}
	}
	return decisions
}

// teamLabelHandler returns a handler enforcing one policy: a pod's team label must be platform.
// trace is what --enableDecisionTraces sets: whether the policy is compiled for tracing.
func teamLabelHandler(t *testing.T, trace bool) *handler {
	t.Helper()
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "require-platform-team"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
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
	provider, err := vpolengine.NewProvider(vpolcompiler.NewCompilerWithTrace(trace), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	engine := vpolengine.NewEngine(provider, func(string) *corev1.Namespace { return nil }, matching.NewMatcher())
	return New(engine, libs.NewFakeContextProvider(), fakekyvernoclient.NewSimpleClientset(), false, event.NewFake())
}

func podRequest(team string) handlers.AdmissionRequest {
	return handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Namespace: "default",
			Name:      "web",
			Object: runtime.RawExtension{Raw: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {"name": "web", "namespace": "default", "labels": {"team": "` + team + `"}},
				"spec": {"containers": [{"name": "web", "image": "nginx:1.27"}]}
			}`)},
		},
	}
}

// TestValidate_DecisionTracesAreLogged runs admission requests through the handler with the
// policy compiled for tracing, as --enableDecisionTraces does, and reads the log.
func TestValidate_DecisionTracesAreLogged(t *testing.T) {
	tests := []struct {
		name        string
		team        string
		verbosity   int
		wantAllowed bool
		wantVerdict string
	}{{
		name:        "a denied request is logged at the default verbosity",
		team:        "payments",
		verbosity:   2,
		wantVerdict: "FAIL",
	}, {
		name:        "an allowed request is logged at verbosity 3",
		team:        "platform",
		verbosity:   3,
		wantAllowed: true,
		wantVerdict: "PASS",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var log capturedLog
			response := teamLabelHandler(t, true).ValidateClustered(context.Background(), log.logger(tt.verbosity), podRequest(tt.team), "", time.Now())
			assert.Equal(t, tt.wantAllowed, response.Allowed)

			require.Eventually(t, func() bool { return len(log.decisions()) == 1 }, 5*time.Second, 5*time.Millisecond, "one policy decided, so one trace is logged")
			line := log.decisions()[0]
			assert.Equal(t, "policy decision", line["msg"])
			assert.Equal(t, "require-platform-team", line["policy"])
			assert.Equal(t, "ValidatingPolicy", line["policyKind"])
			assert.Equal(t, tt.wantVerdict, line["verdict"])

			logged, ok := line["trace"].(map[string]any)
			require.True(t, ok)
			assert.NotEmpty(t, logged["scope"])
			assert.Equal(t, []any{map[string]any{"name": "team", "expression": "object.metadata.?labels.?team.orValue('')", "result": tt.team}}, logged["variables"])
			verdict, ok := logged["verdict"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "variables.team == 'platform'", verdict["expression"])
			if tt.wantAllowed {
				assert.Nil(t, verdict["evaluated"], "a pass has no breakdown")
			} else {
				assert.Equal(t, "the team label must be platform", verdict["message"])
				assert.Contains(t, verdict["evaluated"], map[string]any{"expression": "variables.team", "value": tt.team}, "the breakdown shows the value that failed the check")
			}

			// the trace goes to the log only: the answer to the API server is the one an
			// untraced handler gives
			var untracedLog capturedLog
			untraced := teamLabelHandler(t, false).ValidateClustered(context.Background(), untracedLog.logger(tt.verbosity), podRequest(tt.team), "", time.Now())
			assert.Equal(t, untraced, response)
		})
	}
}

// TestValidate_NoDecisionTracesByDefault: without --enableDecisionTraces policies are compiled
// as before, carry no trace, and nothing is logged at any verbosity.
func TestValidate_NoDecisionTracesByDefault(t *testing.T) {
	var log capturedLog
	response := teamLabelHandler(t, false).ValidateClustered(context.Background(), log.logger(10), podRequest("payments"), "", time.Now())
	assert.False(t, response.Allowed)
	assert.Never(t, func() bool { return len(log.decisions()) > 0 }, 100*time.Millisecond, 10*time.Millisecond)
}
