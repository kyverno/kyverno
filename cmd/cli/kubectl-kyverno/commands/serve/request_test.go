package serve

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/apply"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func postRequest(t *testing.T, h http.Handler, req any) (int, *EvaluationResponse, Status) {
	t.Helper()
	body, err := json.Marshal(req)
	require.NoError(t, err)
	code, raw := do(t, h, http.MethodPost, "/v1alpha1/evaluate", body)
	var resp EvaluationResponse
	var st Status
	if code == http.StatusOK {
		require.NoError(t, json.Unmarshal(raw, &resp), string(raw))
	} else {
		require.NoError(t, json.Unmarshal(raw, &st), string(raw))
	}
	return code, &resp, st
}

func applySummary(t *testing.T, args ...string) Summary {
	t.Helper()
	var out bytes.Buffer
	cmd := apply.Command()
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(append(args, "--remove-color"))
	_ = cmd.Execute()
	m := summaryRegexp.FindStringSubmatch(out.String())
	require.NotNil(t, m, out.String())
	return summaryFromMatch(m)
}

// TestEvaluateRequest covers POST /v1alpha1/evaluate. Every case is also run through the apply
// command with the equivalent flags, and the summaries must match.
//
//nolint:paralleltest // apply and newServerFromPaths set process-global state
func TestEvaluateRequest(t *testing.T) {
	s, err := newServerFromPaths([]string{fixture("policies", "kubernetes")}, Config{AllowRequestPolicies: true})
	require.NoError(t, err)
	h := s.Handler()
	k8sPolicies := fixture("policies", "kubernetes")
	labelsPolicy := fixture("policies", "kubernetes", "check-deployment-labels.yaml")
	tests := []struct {
		name      string
		req       EvaluationRequest
		applyArgs []string
		decision  Decision
		ownDigest bool
	}{{
		name:      "server policies",
		req:       EvaluationRequest{Mode: evaluator.ModeKubernetes, Payload: read(t, fixture("payloads", "kubernetes", "deployment-fail.yaml"))},
		applyArgs: []string{k8sPolicies, "-r", fixture("payloads", "kubernetes", "deployment-fail.yaml")},
		decision:  DecisionDeny,
	}, {
		name:      "request policies",
		req:       EvaluationRequest{APIVersion: APIVersion, Kind: "EvaluationRequest", Mode: "kubernetes", Policies: read(t, labelsPolicy), Payload: read(t, fixture("payloads", "kubernetes", "deployment-fail.yaml"))},
		applyArgs: []string{labelsPolicy, "-r", fixture("payloads", "kubernetes", "deployment-fail.yaml")},
		decision:  DecisionDeny,
		ownDigest: true,
	}, {
		name:      "operation DELETE does not match a CREATE/UPDATE policy",
		req:       EvaluationRequest{Mode: evaluator.ModeKubernetes, Policies: read(t, labelsPolicy), Payload: read(t, fixture("payloads", "kubernetes", "deployment-fail.yaml")), Operation: "delete"},
		applyArgs: []string{labelsPolicy, "-r", fixture("payloads", "kubernetes", "deployment-fail.yaml"), "-f", fixture("requests", "values-delete.yaml")},
		decision:  DecisionNoMatch,
		ownDigest: true,
	}, {
		name:      "operation UPDATE",
		req:       EvaluationRequest{Mode: evaluator.ModeKubernetes, Policies: read(t, labelsPolicy), Payload: read(t, fixture("payloads", "kubernetes", "deployment-fail.yaml")), Operation: "UPDATE"},
		applyArgs: []string{labelsPolicy, "-r", fixture("payloads", "kubernetes", "deployment-fail.yaml"), "-f", fixture("requests", "values-update.yaml")},
		decision:  DecisionDeny,
		ownDigest: true,
	}, {
		name:      "exceptions",
		req:       EvaluationRequest{Mode: evaluator.ModeKubernetes, Policies: read(t, labelsPolicy), Payload: read(t, fixture("payloads", "kubernetes", "deployment-fail.yaml")), Exceptions: read(t, fixture("requests", "exempt-bad-deployment.yaml"))},
		applyArgs: []string{labelsPolicy, "-r", fixture("payloads", "kubernetes", "deployment-fail.yaml"), "--exception", fixture("requests", "exempt-bad-deployment.yaml")},
		decision:  DecisionNoMatch,
		ownDigest: true,
	}, {
		name:      "context lookups",
		req:       EvaluationRequest{Mode: evaluator.ModeKubernetes, Policies: read(t, fixture("requests", "require-allowed-registry.yaml")), Payload: read(t, fixture("payloads", "kubernetes", "pod.yaml")), Context: read(t, fixture("requests", "allowed-registry-context.yaml"))},
		applyArgs: []string{fixture("requests", "require-allowed-registry.yaml"), "-r", fixture("payloads", "kubernetes", "pod.yaml"), "--context-file", fixture("requests", "allowed-registry-context.yaml")},
		decision:  DecisionDeny,
		ownDigest: true,
	}, {
		name:      "VAP parameter resources",
		req:       EvaluationRequest{Mode: evaluator.ModeKubernetes, Policies: read(t, fixture("requests", "vap-with-params.yaml")), Payload: read(t, fixture("requests", "deployment-10-replicas.yaml")), ParameterResources: read(t, fixture("requests", "vap-params.yaml"))},
		applyArgs: []string{fixture("requests", "vap-with-params.yaml"), "-r", fixture("requests", "deployment-10-replicas.yaml"), "--parameter-resource", fixture("requests", "vap-params.yaml")},
		decision:  DecisionDeny,
		ownDigest: true,
	}, {
		name: "mutateExisting targets with CRDs",
		req: EvaluationRequest{
			Mode: evaluator.ModeKubernetes, Policies: read(t, fixture("requests", "targets", "policy.yaml")), Payload: read(t, fixture("requests", "targets", "route.yaml")),
			TargetResources: read(t, fixture("requests", "targets", "gateways.yaml")), CustomResourceDefinitions: read(t, fixture("requests", "targets", "crds.yaml")),
		},
		applyArgs: []string{fixture("requests", "targets", "policy.yaml"), "-r", fixture("requests", "targets", "route.yaml"), "--target-resource", fixture("requests", "targets", "gateways.yaml"), "--crd-paths", fixture("requests", "targets", "crds.yaml")},
		decision:  DecisionAllow,
		ownDigest: true,
	}, {
		name:      "request policies in another mode",
		req:       EvaluationRequest{Mode: evaluator.ModeEnvoy, Policies: read(t, fixture("policies", "envoy", "forbid-admin-path.yaml")), Payload: read(t, fixture("payloads", "envoy", "admin.json"))},
		applyArgs: []string{fixture("policies", "envoy", "forbid-admin-path.yaml"), "--envoy-payload", fixture("payloads", "envoy", "admin.json")},
		decision:  DecisionDeny,
		ownDigest: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := applySummary(t, tt.applyArgs...)
			code, resp, st := postRequest(t, h, tt.req)
			require.Equal(t, http.StatusOK, code, st.Message)
			assert.Equal(t, want, resp.Summary)
			assert.Equal(t, tt.decision, resp.Decision)
			if tt.ownDigest {
				assert.NotEqual(t, s.digest, resp.PolicySetDigest)
				assert.Regexp(t, `^sha256:`, resp.PolicySetDigest)
			} else {
				assert.Equal(t, s.digest, resp.PolicySetDigest)
			}
		})
	}
}

//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestEvaluateRequestErrors(t *testing.T) {
	locked, err := newServerFromPaths([]string{fixture("policies", "kubernetes")}, Config{Modes: []evaluator.Mode{evaluator.ModeKubernetes}})
	require.NoError(t, err)
	open, err := newServerFromPaths(nil, Config{AllowRequestPolicies: true})
	require.NoError(t, err)
	pod := read(t, fixture("payloads", "kubernetes", "pod.yaml"))
	labels := read(t, fixture("policies", "kubernetes", "check-deployment-labels.yaml"))
	tests := []struct {
		name   string
		server *Server
		body   string
		code   int
		msg    string
	}{
		{"not json", locked, `{`, http.StatusBadRequest, "failed to decode EvaluationRequest"},
		{"unknown field", locked, `{"mode":"Kubernetes","payload":"x","previousObject":"y"}`, http.StatusBadRequest, "unknown field"},
		{"trailing data", locked, `{"mode":"Kubernetes","payload":"x"}{}`, http.StatusBadRequest, "unexpected data after the request object"},
		{"wrong kind", locked, `{"kind":"AdmissionReview","mode":"Kubernetes","payload":"x"}`, http.StatusBadRequest, "unsupported apiVersion/kind"},
		{"bad mode", locked, `{"mode":"SOAP","payload":"x"}`, http.StatusBadRequest, "unsupported mode"},
		{"disabled mode", locked, `{"mode":"Envoy","payload":"{}"}`, http.StatusBadRequest, "not enabled"},
		{"empty payload", locked, `{"mode":"Kubernetes"}`, http.StatusBadRequest, "payload: request body is empty"},
		{"bad operation", locked, mustJSON(t, EvaluationRequest{Mode: "Kubernetes", Payload: pod, Operation: "PATCH"}), http.StatusBadRequest, "invalid operation"},
		{"request policies disabled", locked, mustJSON(t, EvaluationRequest{Mode: "Kubernetes", Payload: pod, Policies: labels}), http.StatusForbidden, "--allow-request-policies"},
		{"no policies anywhere", open, mustJSON(t, EvaluationRequest{Mode: "Kubernetes", Payload: pod}), http.StatusBadRequest, "has none"},
		{"invalid request policies", open, mustJSON(t, EvaluationRequest{Mode: "Kubernetes", Payload: pod, Policies: read(t, fixture("policies", "invalid", "broken.yaml"))}), http.StatusBadRequest, "policies: failed to load"},
		{"legacy request policy", open, mustJSON(t, EvaluationRequest{Mode: "Kubernetes", Payload: pod, Policies: "apiVersion: kyverno.io/v1\nkind: ClusterPolicy\nmetadata:\n  name: x\nspec:\n  rules: []\n"}), http.StatusBadRequest, "policies:"},
		{"non-exception in exceptions", open, mustJSON(t, EvaluationRequest{Mode: "Kubernetes", Payload: pod, Policies: labels, Exceptions: pod}), http.StatusBadRequest, "CEL PolicyException"},
		{"bad context", open, mustJSON(t, EvaluationRequest{Mode: "Kubernetes", Payload: pod, Policies: labels, Context: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n"}), http.StatusBadRequest, "context:"},
		{"non-CRD in customResourceDefinitions", open, mustJSON(t, EvaluationRequest{Mode: "Kubernetes", Payload: pod, Policies: labels, CustomResourceDefinitions: pod}), http.StatusBadRequest, "must be a CustomResourceDefinition"},
		{"bad cluster resources", open, mustJSON(t, EvaluationRequest{Mode: "Kubernetes", Payload: pod, Policies: labels, ClusterResources: "{"}), http.StatusBadRequest, "clusterResources:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, raw := do(t, tt.server.Handler(), http.MethodPost, "/v1alpha1/evaluate", []byte(tt.body))
			assert.Equal(t, tt.code, code, string(raw))
			assert.Contains(t, string(raw), tt.msg)
		})
	}
	t.Run("raw endpoint without server policies", func(t *testing.T) {
		code, raw := do(t, open.Handler(), http.MethodPost, "/v1alpha1/evaluate/Kubernetes", []byte(pod))
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Contains(t, string(raw), "no policies loaded")
		code, raw = do(t, open.Handler(), http.MethodGet, "/v1alpha1/policies", nil)
		assert.Equal(t, http.StatusOK, code)
		assert.Contains(t, string(raw), `"policies": []`)
	})
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

// TestRequestCRDsDoNotLeak sends the same request with, without, then with CRDs: the CRDs of one
// request must not make the custom kinds resolvable for the next.
//
//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestRequestCRDsDoNotLeak(t *testing.T) {
	s, err := newServerFromPaths(nil, Config{AllowRequestPolicies: true})
	require.NoError(t, err)
	h := s.Handler()
	req := EvaluationRequest{
		Mode: evaluator.ModeKubernetes, Policies: read(t, fixture("requests", "targets", "policy.yaml")), Payload: read(t, fixture("requests", "targets", "route.yaml")),
		TargetResources: read(t, fixture("requests", "targets", "gateways.yaml")),
	}
	withCRDs := req
	withCRDs.CustomResourceDefinitions = read(t, fixture("requests", "targets", "crds.yaml"))

	code, resp, st := postRequest(t, h, withCRDs)
	require.Equal(t, http.StatusOK, code, st.Message)
	require.Len(t, resp.Results, 2)
	names := map[string]string{}
	for _, r := range resp.Results {
		require.NotNil(t, r.Resource)
		require.NotNil(t, r.PatchedTarget, "patchedTarget for %s", r.Resource.Name)
		names[r.Resource.Name] = r.PatchedTarget["metadata"].(map[string]any)["annotations"].(map[string]any)["route"].(string)
	}
	assert.Equal(t, map[string]string{"gateway-a": "route-a", "gateway-b": "route-a"}, names)

	code, _, st = postRequest(t, h, req)
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Contains(t, st.Message, `no matches for kind "Gateway"`)

	code, _, st = postRequest(t, h, withCRDs)
	assert.Equal(t, http.StatusOK, code, st.Message)
}

// TestRequestSameObjectsAsClusterAndTargets sends the same objects as clusterResources and
// targetResources (what the Playground does); the in-memory cluster must deduplicate them.
//
//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestRequestSameObjectsAsClusterAndTargets(t *testing.T) {
	s, err := newServerFromPaths(nil, Config{AllowRequestPolicies: true})
	require.NoError(t, err)
	gateways := read(t, fixture("requests", "targets", "gateways.yaml"))
	code, resp, st := postRequest(t, s.Handler(), EvaluationRequest{
		Mode: evaluator.ModeKubernetes, Policies: read(t, fixture("requests", "targets", "policy.yaml")), Payload: read(t, fixture("requests", "targets", "route.yaml")),
		ClusterResources: gateways, TargetResources: gateways, CustomResourceDefinitions: read(t, fixture("requests", "targets", "crds.yaml")),
	})
	require.Equal(t, http.StatusOK, code, st.Message)
	assert.Equal(t, 2, resp.Summary.Pass)
}

const immutableEnvPolicy = `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: immutable-env
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: ["apps"]
      apiVersions: ["v1"]
      operations: ["UPDATE"]
      resources: ["deployments"]
  validations:
  - expression: "object.metadata.labels['env'] == oldObject.metadata.labels['env']"
    message: env label is immutable
`

const prodNamespacesOnlyPolicy = `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: no-pods-in-prod
spec:
  matchConstraints:
    namespaceSelector:
      matchLabels:
        env: prod
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: ["CREATE"]
      resources: ["pods"]
  validations:
  - expression: "false"
    message: no pods in prod namespaces
`

//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestRequestOldPayloadAndNamespaces(t *testing.T) {
	s, err := newServerFromPaths(nil, Config{AllowRequestPolicies: true})
	require.NoError(t, err)
	h := s.Handler()
	current := read(t, fixture("payloads", "kubernetes", "deployment-fail.yaml")) // env: testing
	previous := strings.ReplaceAll(current, "env: testing", "env: prod")

	// without an old object the resource is its own old state, so the label did not change
	code, resp, st := postRequest(t, h, EvaluationRequest{Mode: "Kubernetes", Policies: immutableEnvPolicy, Payload: current, Operation: "UPDATE"})
	require.Equal(t, http.StatusOK, code, st.Message)
	assert.Equal(t, DecisionAllow, resp.Decision)
	// with the previous state, the change is caught
	code, resp, st = postRequest(t, h, EvaluationRequest{Mode: "Kubernetes", Policies: immutableEnvPolicy, Payload: current, OldPayload: previous, Operation: "UPDATE"})
	require.Equal(t, http.StatusOK, code, st.Message)
	assert.Equal(t, DecisionDeny, resp.Decision)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "env label is immutable", resp.Results[0].Message)
	// oldPayload is Kubernetes-only
	code, _, st = postRequest(t, h, EvaluationRequest{Mode: "JSON", Policies: read(t, fixture("policies", "json", "check-dockerfile.yaml")), Payload: "{}", OldPayload: "{}"})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, st.Message, "only supported in Kubernetes mode")

	// namespaceSelector reads Namespace objects sent as cluster resources
	pod := read(t, fixture("payloads", "kubernetes", "pod.yaml"))
	code, resp, st = postRequest(t, h, EvaluationRequest{Mode: "Kubernetes", Policies: prodNamespacesOnlyPolicy, Payload: pod})
	require.Equal(t, http.StatusOK, code, st.Message)
	assert.Equal(t, DecisionNoMatch, resp.Decision)
	code, resp, st = postRequest(t, h, EvaluationRequest{Mode: "Kubernetes", Policies: prodNamespacesOnlyPolicy, Payload: pod, ClusterResources: "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: default\n  labels:\n    env: prod\n"})
	require.Equal(t, http.StatusOK, code, st.Message)
	assert.Equal(t, DecisionDeny, resp.Decision)
}
