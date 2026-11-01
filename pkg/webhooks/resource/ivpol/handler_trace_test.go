package ivpol

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
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/event"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

// traceTestPolicies are two image validating policies whose images never match
// matchImageReferences, so nothing is fetched from a registry and no signature is verified.
// require-team only audits; deployments-only denies, but never applies to a Pod.
const traceTestPolicies = `
- metadata:
    name: require-team
  spec:
    validationActions: [Audit]
    matchConstraints:
      resourceRules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE"]
        resources: ["pods"]
    matchConditions:
    - name: not-a-debug-pod
      expression: "!object.metadata.name.startsWith('debug-')"
    matchImageReferences:
    - glob: "registry.example.com/*"
    validationConfigurations:
      verifyDigest: false
      required: false
    attestors:
    - name: unused
      cosign:
        key:
          data: unused
    variables:
    - name: team
      expression: "object.metadata.?labels.?team.orValue('')"
    validations:
    - expression: "variables.team != ''"
      message: "pods need a team label"
- metadata:
    name: deployments-only
  spec:
    validationActions: [Deny]
    matchConstraints:
      resourceRules:
      - apiGroups: ["apps"]
        apiVersions: ["v1"]
        operations: ["CREATE"]
        resources: ["deployments"]
    matchImageReferences:
    - glob: "registry.example.com/*"
    validationConfigurations:
      verifyDigest: false
      required: false
    attestors:
    - name: unused
      cosign:
        key:
          data: unused
    validations:
    - expression: "true"
`

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

// decisions returns the policy and verdict of every line written under the decision-trace logger.
func (c *capturedLog) decisions() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	decisions := map[string]string{}
	for _, line := range c.lines {
		if line["logger"] == "decision-trace" {
			policy, _ := line["policy"].(string)
			verdict, _ := line["verdict"].(string)
			decisions[policy] = verdict
		}
	}
	return decisions
}

// recordedEvents keeps every event the handler emits.
type recordedEvents struct {
	mu     sync.Mutex
	events []event.Info
}

func (r *recordedEvents) Add(infos ...event.Info) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, infos...)
}

func (r *recordedEvents) all() []event.Info {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]event.Info(nil), r.events...)
}

// traceTestHandler returns the ivpol handler with the two test policies, and the events it emits.
// trace is what --enableDecisionTraces sets: whether the policies are compiled for tracing.
func traceTestHandler(t *testing.T, trace bool) (*handler, *recordedEvents) {
	t.Helper()
	// require-team only audits, so a request is not blocked and the handler asks whether it
	// needs a report, which reads the process-wide reporting configuration
	prevReportingCfg := reportutils.ReportingCfg
	reportutils.ReportingCfg = reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = prevReportingCfg })
	var policies []*policiesv1beta1.ImageValidatingPolicy
	require.NoError(t, yaml.Unmarshal([]byte(traceTestPolicies), &policies))
	likes := make([]policiesv1beta1.ImageValidatingPolicyLike, 0, len(policies))
	for _, p := range policies {
		likes = append(likes, p)
	}
	provider, err := ivpolengine.NewProvider(evaluator.NewCompilerWithTrace(nil, trace), likes, nil)
	require.NoError(t, err)
	namespaceResolver := func(string) *corev1.Namespace { return nil }
	engine := ivpolengine.NewEngine(provider, namespaceResolver, matching.NewMatcher(), nil, imageverifycache.DisabledImageVerifyCache(), config.NewDefaultConfiguration(false))
	events := &recordedEvents{}
	return New(engine, libs.NewFakeContextProvider(), nil, false, events), events
}

func tracePodRequest(name string, labels string) handlers.AdmissionRequest {
	return handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Namespace: "default",
			Name:      name,
			Object: runtime.RawExtension{Raw: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {"name": "` + name + `", "namespace": "default", "uid": "pod-uid", "labels": ` + labels + `},
				"spec": {"containers": [{"name": "web", "image": "docker.io/library/nginx:1.27"}]}
			}`)},
		},
	}
}

// TestValidate_DecisionTracesAreLogged runs admission requests through the ivpol handler with the
// policies compiled for tracing, as --enableDecisionTraces does, and reads the log. With tracing
// on, the engine also returns the policies that did not apply, or that a match condition skipped,
// for their traces: the handler must log those, and keep them out of the admission response and
// the events, which must be exactly what they are with tracing off.
func TestValidate_DecisionTracesAreLogged(t *testing.T) {
	tests := []struct {
		name   string
		pod    string
		labels string
		// want is the verdict logged per policy at verbosity 4, which logs every decision
		want map[string]string
	}{{
		name:   "a pod without a team label fails the audit",
		pod:    "web",
		labels: `{}`,
		want:   map[string]string{"require-team": "FAIL", "deployments-only": "SKIP"},
	}, {
		name:   "a pod with a team label passes",
		pod:    "web",
		labels: `{"team": "payments"}`,
		want:   map[string]string{"require-team": "PASS", "deployments-only": "SKIP"},
	}, {
		name:   "a match condition skips a debug pod",
		pod:    "debug-shell",
		labels: `{}`,
		want:   map[string]string{"require-team": "SKIP", "deployments-only": "SKIP"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			untracedHandler, untracedEvents := traceTestHandler(t, false)
			untraced := untracedHandler.ValidateClustered(context.Background(), logr.Discard(), tracePodRequest(tt.pod, tt.labels), "", time.Now())

			var log capturedLog
			tracedHandler, tracedEvents := traceTestHandler(t, true)
			traced := tracedHandler.ValidateClustered(context.Background(), log.logger(4), tracePodRequest(tt.pod, tt.labels), "", time.Now())

			require.Eventually(t, func() bool { return len(log.decisions()) == len(tt.want) }, 5*time.Second, 5*time.Millisecond)
			assert.Equal(t, tt.want, log.decisions(), "every policy's decision is logged, including the ones that did not apply")

			// the trace goes to the log only
			assert.Equal(t, untraced, traced, "the admission response is the one an untraced handler gives")
			assert.Equal(t, untracedEvents.all(), tracedEvents.all(), "the events are the ones an untraced handler emits")
		})
	}
}

// TestValidate_DecisionTracesAtTheDefaultVerbosity: at the default verbosity only the decisions
// that did not pass are logged, so a request whose policies pass, or do not apply, logs nothing.
func TestValidate_DecisionTracesAtTheDefaultVerbosity(t *testing.T) {
	var log capturedLog
	h, _ := traceTestHandler(t, true)
	h.ValidateClustered(context.Background(), log.logger(2), tracePodRequest("web", `{}`), "", time.Now())
	require.Eventually(t, func() bool { return len(log.decisions()) == 1 }, 5*time.Second, 5*time.Millisecond)
	assert.Equal(t, map[string]string{"require-team": "FAIL"}, log.decisions())

	var passing capturedLog
	h, _ = traceTestHandler(t, true)
	h.ValidateClustered(context.Background(), passing.logger(2), tracePodRequest("web", `{"team": "payments"}`), "", time.Now())
	assert.Never(t, func() bool { return len(passing.decisions()) > 0 }, 100*time.Millisecond, 10*time.Millisecond)
}

// TestValidate_NoDecisionTracesByDefault: without --enableDecisionTraces policies are compiled as
// before, carry no trace, and nothing is logged at any verbosity.
func TestValidate_NoDecisionTracesByDefault(t *testing.T) {
	var log capturedLog
	h, _ := traceTestHandler(t, false)
	h.ValidateClustered(context.Background(), log.logger(10), tracePodRequest("web", `{}`), "", time.Now())
	assert.Never(t, func() bool { return len(log.decisions()) > 0 }, 100*time.Millisecond, 10*time.Millisecond)
}
