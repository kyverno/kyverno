package mpol

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
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	fakekyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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

const labelPolicy = "label-deployments"

// labelHandler returns a handler with one policy that applies the given applyConfiguration
// expression to deployments. trace is what --enableDecisionTraces sets: whether the policy is
// compiled for tracing.
func labelHandler(t *testing.T, trace bool, expression string) *handler {
	t.Helper()
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: labelPolicy},
		Spec: policiesv1beta1.MutatingPolicySpec{
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
	provider, err := mpolengine.NewProvider(mpolcompiler.NewCompilerWithTrace(trace), []policiesv1beta1.MutatingPolicyLike{policy}, nil, libs.NewFakeContextProvider())
	require.NoError(t, err)
	engine := mpolengine.NewEngine(
		provider,
		func(ns string) *corev1.Namespace { return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}} },
		matching.NewMatcher(),
		&gateBenchTypeConverter{},
		&libs.FakeContextProvider{},
	)
	return New(nil, engine, fakekyvernoclient.NewSimpleClientset(), &mockReportsConfig{}, &mockURGenerator{}, "", event.NewFake())
}

func deploymentRequest() handlers.AdmissionRequest {
	return handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			Kind:      metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Resource:  metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			Namespace: "default",
			Name:      "web",
			// a dry run, so the handler does not start the mutate-existing goroutine
			DryRun: ptr.To(true),
			Object: runtime.RawExtension{Raw: []byte(`{
				"apiVersion": "apps/v1",
				"kind": "Deployment",
				"metadata": {"name": "web", "namespace": "default"},
				"spec": {
					"selector": {"matchLabels": {"app": "web"}},
					"template": {
						"metadata": {"labels": {"app": "web"}},
						"spec": {"containers": [{"name": "web", "image": "nginx:1.27"}]}
					}
				}
			}`)},
		},
	}
}

// TestMutate_DecisionTracesAreLogged runs admission requests through the handler with the policy
// compiled for tracing, as --enableDecisionTraces does, and reads the log.
func TestMutate_DecisionTracesAreLogged(t *testing.T) {
	tests := []struct {
		name        string
		expression  string
		verbosity   int
		wantVerdict string
		wantError   string
	}{{
		name:        "a mutation that ran is logged at verbosity 3",
		expression:  `Object{metadata: Object.metadata{labels: {"team": "platform"}}}`,
		verbosity:   3,
		wantVerdict: "PASS",
	}, {
		// the deployment has no labels to read the owner from
		name:        "a mutation that failed is logged at the default verbosity",
		expression:  `Object{metadata: Object.metadata{labels: {"owner": object.metadata.labels.owner}}}`,
		verbosity:   2,
		wantVerdict: "ERROR",
		wantError:   "no such key: labels",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutate := func(trace bool, logger logr.Logger) handlers.AdmissionResponse {
				return labelHandler(t, trace, tt.expression).mutate(context.Background(), logger, deploymentRequest(), []string{labelPolicy}, mpolengine.MatchNames(labelPolicy))
			}
			var log capturedLog
			response := mutate(true, log.logger(tt.verbosity))

			require.Eventually(t, func() bool { return len(log.decisions()) == 1 }, 5*time.Second, 5*time.Millisecond, "one policy decided, so one trace is logged")
			line := log.decisions()[0]
			assert.Equal(t, "policy decision", line["msg"])
			assert.Equal(t, labelPolicy, line["policy"])
			assert.Equal(t, "MutatingPolicy", line["policyKind"])
			assert.Equal(t, tt.wantVerdict, line["verdict"])

			logged, ok := line["trace"].(map[string]any)
			require.True(t, ok)
			mutations, ok := logged["mutations"].([]any)
			require.True(t, ok)
			require.Len(t, mutations, 1)
			mutation, ok := mutations[0].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "mutations[0] (applyConfiguration)", mutation["name"])
			assert.Equal(t, tt.expression, mutation["expression"])
			if tt.wantError == "" {
				assert.Nil(t, mutation["error"])
				assert.Nil(t, mutation["evaluated"], "a mutation that ran has no breakdown")
			} else {
				assert.Contains(t, mutation["error"], tt.wantError)
				assert.Contains(t, mutation["evaluated"], map[string]any{"expression": "object.metadata.labels", "error": tt.wantError}, "the breakdown shows the read that failed")
			}

			// the trace goes to the log only: the answer to the API server is the one an
			// untraced handler gives
			var untracedLog capturedLog
			assert.Equal(t, mutate(false, untracedLog.logger(tt.verbosity)), response)
		})
	}
}

// TestMutate_NoDecisionTracesByDefault: without --enableDecisionTraces policies are compiled as
// before, carry no trace, and nothing is logged at any verbosity.
func TestMutate_NoDecisionTracesByDefault(t *testing.T) {
	var log capturedLog
	labelHandler(t, false, `Object{metadata: Object.metadata{labels: {"team": "platform"}}}`).mutate(context.Background(), log.logger(10), deploymentRequest(), []string{labelPolicy}, mpolengine.MatchNames(labelPolicy))
	assert.Never(t, func() bool { return len(log.decisions()) > 0 }, 100*time.Millisecond, 10*time.Millisecond)
}
