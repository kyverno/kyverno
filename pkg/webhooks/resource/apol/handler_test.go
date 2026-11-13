package apol

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	apolengine "github.com/kyverno/kyverno/pkg/cel/policies/apol/engine"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	authorizationv1alpha1 "k8s.io/api/authorization/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	genericfeatures "k8s.io/apiserver/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/apiserver/plugin/pkg/authorizer/webhook"
	webhookmetrics "k8s.io/apiserver/plugin/pkg/authorizer/webhook/metrics"
	"k8s.io/client-go/rest"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
)

// ownerPolicy allows writes to configmaps owned by the caller, and denies
// writes to configmaps labelled protected.
func ownerPolicy() *policiesv1alpha1.AuthorizingPolicy {
	return &policiesv1alpha1.AuthorizingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "owner"},
		Spec: policiesv1alpha1.AuthorizingPolicySpec{
			MatchSubjects: []policiesv1alpha1.SubjectSelectorTerm{{
				MatchExpressions: []policiesv1alpha1.SubjectSelectorRequirement{{Key: "groups", Operator: "In", Values: []string{"dev"}}},
			}},
			MatchResources: policiesv1alpha1.AuthorizingMatchResources{
				ResourceRules: []policiesv1alpha1.AuthorizingResourceRule{{
					APIGroups: []string{""}, APIVersions: []string{"*"}, Resources: []string{"configmaps"}, Verbs: []string{"*"},
				}},
			},
			Rules: []policiesv1alpha1.AuthorizingRule{{
				Name:        "owner-writes",
				Effect:      policiesv1alpha1.AuthorizingRuleEffectAllow,
				Expression:  "has(object.metadata.labels) && 'owner' in object.metadata.labels && object.metadata.labels.owner == request.user",
				Description: "owners may write",
			}, {
				Name:   "protected",
				Effect: policiesv1alpha1.AuthorizingRuleEffectDeny,
				// Deny rules that read object must be scoped to writes: on reads the
				// object is never known, so an object-dependent Deny fails closed.
				Expression: "request.resourceAttributes.verb != 'get' && request.resourceAttributes.verb != 'list' && request.resourceAttributes.verb != 'watch' && has(object.metadata.labels) && 'protected' in object.metadata.labels",
			}, {
				Name:       "reads",
				Effect:     policiesv1alpha1.AuthorizingRuleEffectAllow,
				Expression: "request.resourceAttributes.verb in ['get', 'list', 'watch']",
			}},
		},
	}
}

func newHandler(t *testing.T, pols ...*policiesv1alpha1.AuthorizingPolicy) *Handler {
	t.Helper()
	c := apolcompiler.NewCompiler()
	var compiled []apolengine.CompiledPolicy
	for _, p := range pols {
		cp := apolengine.NewCompiledPolicy(c, p)
		require.NoError(t, cp.CompileError())
		compiled = append(compiled, cp)
	}
	e, err := apolengine.NewEngine(apolengine.ProviderFunc(func(context.Context) ([]apolengine.CompiledPolicy, error) { return compiled, nil }))
	require.NoError(t, err)
	return New(e)
}

// upstreamAuthorizer builds the real kube-apiserver v0.38.0-beta.0 webhook
// authorizer pointed at the Kyverno handler, so requests and responses go
// through upstream encoding, validation, and UID checks.
func upstreamAuthorizer(t *testing.T, withConditionsReview bool) *webhook.WebhookAuthorizer {
	t.Helper()
	h := newHandler(t, ownerPolicy())
	mux := http.NewServeMux()
	mux.HandleFunc(CombinedPath, h.HandleCombined)
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	cfg := &rest.Config{Host: srv.URL + CombinedPath, TLSClientConfig: rest.TLSClientConfig{Insecure: true}}
	var acrCfg *rest.Config
	var acrVersion string
	if withConditionsReview {
		acrCfg, acrVersion = cfg, "v1alpha1"
	}
	a, err := webhook.New(cfg, "v1", 0, 0, wait.Backoff{Steps: 1, Duration: time.Millisecond}, authorizer.DecisionDeny, nil, "kyverno",
		webhookmetrics.NoopAuthorizerMetrics{}, acrCfg, acrVersion)
	require.NoError(t, err)
	return a
}

func attrs(verb string) authorizer.AttributesRecord {
	return authorizer.AttributesRecord{
		User:            &user.DefaultInfo{Name: "alice", Groups: []string{"dev"}},
		Verb:            verb,
		Namespace:       "team-a",
		APIVersion:      "v1",
		Resource:        "configmaps",
		Name:            "cm",
		ResourceRequest: true,
	}
}

type conditionsData struct {
	object, oldObject runtime.Object
}

func (d conditionsData) GetName() string      { return "cm" }
func (d conditionsData) GetNamespace() string { return "team-a" }
func (d conditionsData) GetResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
}
func (d conditionsData) GetSubresource() string                      { return "" }
func (d conditionsData) GetOperation() authorizer.AdmissionOperation { return "CREATE" }
func (d conditionsData) GetOperationOptions() runtime.Object         { return nil }
func (d conditionsData) IsDryRun() bool                              { return false }
func (d conditionsData) GetObject() runtime.Object                   { return d.object }
func (d conditionsData) GetOldObject() runtime.Object                { return d.oldObject }
func (d conditionsData) GetKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
}
func (d conditionsData) GetUserInfo() user.Info {
	return &user.DefaultInfo{Name: "alice", Groups: []string{"dev"}}
}

func configMap(labels map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": "cm", "namespace": "team-a", "labels": labels},
	}}
}

//nolint:paralleltest // mutates the global feature gate
func TestUpstreamRoundTripConditional(t *testing.T) {
	featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, genericfeatures.ConditionalAuthorization, true)
	a := upstreamAuthorizer(t, true)
	ctx := context.Background()

	// Reads are decided unconditionally.
	rd := a.ConditionsAwareAuthorize(ctx, attrs("get"))
	require.True(t, rd.IsAllow(), "%s %v", rd.String(), rd.Error())

	d := a.ConditionsAwareAuthorize(ctx, attrs("create"))
	require.True(t, d.IsConditionsMap(), "%s", d.String())
	require.Equal(t, 1, countSeq(d.ConditionsMap().DenyConditions()))
	require.Equal(t, 1, countSeq(d.ConditionsMap().AllowConditions()))

	tests := []struct {
		name   string
		labels map[string]any
		want   authorizer.Decision
	}{
		{"owner", map[string]any{"owner": "alice"}, authorizer.DecisionAllow},
		{"not owner", map[string]any{"owner": "bob"}, authorizer.DecisionNoOpinion},
		{"protected", map[string]any{"owner": "alice", "protected": "yes"}, authorizer.DecisionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason, err := a.EvaluateConditions(ctx, d, conditionsData{object: configMap(tt.labels)})
			require.Equal(t, tt.want, got, "reason=%q err=%v", reason, err)
			if tt.want != authorizer.DecisionDeny {
				// Upstream rejects mismatched UIDs and malformed responses with an error.
				require.NoError(t, err)
			}
		})
	}
}

//nolint:paralleltest // mutates the global feature gate
func TestUpstreamFeatureGateOff(t *testing.T) {
	featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, genericfeatures.ConditionalAuthorization, false)
	a := upstreamAuthorizer(t, true)
	// The caller does not opt in: a Deny residual exists, so Kyverno fails closed to Deny.
	d := a.ConditionsAwareAuthorize(context.Background(), attrs("create"))
	require.True(t, d.IsDeny(), "%s", d.String())
	require.True(t, a.ConditionsAwareAuthorize(context.Background(), attrs("get")).IsAllow())
}

//nolint:paralleltest // mutates the global feature gate
func TestUpstreamNoConditionsReviewConfig(t *testing.T) {
	featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, genericfeatures.ConditionalAuthorization, true)
	a := upstreamAuthorizer(t, false)
	d := a.ConditionsAwareAuthorize(context.Background(), attrs("create"))
	require.True(t, d.IsDeny(), "%s", d.String())
}

func countSeq[T any](seq func(func(T) bool)) int {
	n := 0
	for range seq {
		n++
	}
	return n
}

const goldenSARRequest = `{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "user": "alice",
    "groups": ["dev"],
    "resourceAttributes": {"namespace": "team-a", "verb": "create", "version": "v1", "resource": "configmaps", "name": "cm"},
    "authorizationOptions": {"handledDecisionTypes": ["Allow", "Deny", "NoOpinion", "ConditionsMap", "Union"]}
  }
}`

const goldenSARResponseStatus = `{
  "allowed": false,
  "conditionalDecision": {
    "type": "ConditionsMap",
    "conditionsMap": {
      "denyConditions": [
        {"id": "owner/protected", "condition": "has(object.metadata.labels) && \"protected\" in object.metadata.labels", "type": "policies.kyverno.io/cel"}
      ],
      "allowConditions": [
        {"id": "owner/owner-writes", "condition": "has(object.metadata.labels) && \"owner\" in object.metadata.labels && object.metadata.labels.owner == \"alice\"", "type": "policies.kyverno.io/cel", "description": "owners may write"}
      ]
    }
  }
}`

func TestGoldenSubjectAccessReview(t *testing.T) {
	t.Parallel()
	h := newHandler(t, ownerPolicy())
	raw, status, err := h.Review(context.Background(), []byte(goldenSARRequest), "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	var out authorizationv1.SubjectAccessReview
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, "authorization.k8s.io/v1", out.APIVersion)
	require.Equal(t, "SubjectAccessReview", out.Kind)
	gotStatus, err := json.Marshal(out.Status)
	require.NoError(t, err)
	require.JSONEq(t, goldenSARResponseStatus, string(gotStatus))
	// Union invariant: conditional responses keep allowed/denied/reason/evaluationError zero.
	require.False(t, out.Status.Allowed)
	require.False(t, out.Status.Denied)
	require.Empty(t, out.Status.Reason)
	require.Empty(t, out.Status.EvaluationError)
}

func TestGoldenConditionsReview(t *testing.T) {
	t.Parallel()
	h := newHandler(t, ownerPolicy())
	var sar authorizationv1.SubjectAccessReview
	raw, _, err := h.Review(context.Background(), []byte(goldenSARRequest), "")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &sar))
	acr := map[string]any{
		"apiVersion": "authorization.k8s.io/v1alpha1",
		"kind":       "AuthorizationConditionsReview",
		"request": map[string]any{
			"decision": sar.Status.ConditionalDecision,
			"admissionRequest": map[string]any{
				"uid":       "7d0a2b54-0000-4000-8000-000000000001",
				"kind":      map[string]any{"group": "", "version": "v1", "kind": "ConfigMap"},
				"resource":  map[string]any{"group": "", "version": "v1", "resource": "configmaps"},
				"operation": "CREATE",
				"userInfo":  map[string]any{"username": "alice"},
				"object":    configMap(map[string]any{"owner": "alice"}).Object,
			},
		},
	}
	body, err := json.Marshal(acr)
	require.NoError(t, err)
	raw, status, err := h.Review(context.Background(), body, AuthorizationConditionsReview)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.JSONEq(t, `{
	  "apiVersion": "authorization.k8s.io/v1alpha1",
	  "kind": "AuthorizationConditionsReview",
	  "metadata": {},
	  "response": {
	    "uid": "7d0a2b54-0000-4000-8000-000000000001",
	    "decision": {"type": "Allow", "allow": {"reason": "condition \"owner/owner-writes\" allowed the request with description \"owners may write\""}}
	  }
	}`, string(raw))
}

// AuthorizationConditionsReview is the kind filter used by the dedicated endpoint.
const AuthorizationConditionsReview = "AuthorizationConditionsReview"

func TestMalformedRequests(t *testing.T) {
	t.Parallel()
	h := newHandler(t, ownerPolicy())
	tests := []struct {
		name     string
		body     string
		onlyKind string
		status   int
	}{
		{"not json", `{`, "", http.StatusBadRequest},
		{"unknown kind", `{"apiVersion":"v1","kind":"Pod"}`, "", http.StatusBadRequest},
		{"v1beta1 SAR unsupported", `{"apiVersion":"authorization.k8s.io/v1beta1","kind":"SubjectAccessReview"}`, "", http.StatusBadRequest},
		{"wrong kind for endpoint", goldenSARRequest, AuthorizationConditionsReview, http.StatusBadRequest},
		{"conditions review without request", `{"apiVersion":"authorization.k8s.io/v1alpha1","kind":"AuthorizationConditionsReview"}`, "", http.StatusBadRequest},
		{"conditions review without admission request", `{"apiVersion":"authorization.k8s.io/v1alpha1","kind":"AuthorizationConditionsReview","request":{"decision":{"type":"Allow","allow":{}}}}`, "", http.StatusBadRequest},
		{"conditions review without uid", `{"apiVersion":"authorization.k8s.io/v1alpha1","kind":"AuthorizationConditionsReview","request":{"decision":{"type":"Allow","allow":{}},"admissionRequest":{}}}`, "", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, status, err := h.Review(context.Background(), []byte(tt.body), tt.onlyKind)
			require.Error(t, err)
			require.Equal(t, tt.status, status)
		})
	}
}

func TestInvalidSubjectAccessReviewIsDenied(t *testing.T) {
	t.Parallel()
	h := newHandler(t, ownerPolicy())
	body := `{"apiVersion":"authorization.k8s.io/v1","kind":"SubjectAccessReview","spec":{"user":"alice",
	  "resourceAttributes":{"verb":"get","resource":"configmaps"},"nonResourceAttributes":{"path":"/x","verb":"get"}}}`
	raw, _, err := h.Review(context.Background(), []byte(body), "")
	require.NoError(t, err)
	var out authorizationv1.SubjectAccessReview
	require.NoError(t, json.Unmarshal(raw, &out))
	require.True(t, out.Status.Denied)
	require.NotEmpty(t, out.Status.EvaluationError)
}

func TestConditionsReviewEchoesUIDAndFailsClosed(t *testing.T) {
	t.Parallel()
	h := newHandler(t)
	in := &authorizationv1alpha1.AuthorizationConditionsReview{
		TypeMeta: metav1.TypeMeta{APIVersion: "authorization.k8s.io/v1alpha1", Kind: AuthorizationConditionsReview},
		Request: &authorizationv1alpha1.AuthorizationConditionsRequest{
			Decision: authorizationv1.ConditionsAwareDecision{Type: authorizationv1.ConditionsAwareDecisionTypeUnion},
		},
	}
	in.Request.AdmissionRequest = admissionRequest("uid-1")
	body, err := json.Marshal(in)
	require.NoError(t, err)
	raw, _, err := h.Review(context.Background(), body, "")
	require.NoError(t, err)
	var out authorizationv1alpha1.AuthorizationConditionsReview
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, "uid-1", string(out.Response.UID))
	// An empty Union is not a Kyverno-authored ConditionsMap; with no possible Deny it fails closed to NoOpinion.
	require.Equal(t, authorizationv1.ConditionsAwareDecisionTypeNoOpinion, out.Response.Decision.Type)
}

func TestToStatusFailsClosedForUnawareCallers(t *testing.T) {
	t.Parallel()
	cm := authorizationv1.ConditionsAwareDecision{Type: authorizationv1.ConditionsAwareDecisionTypeConditionsMap, ConditionsMap: &authorizationv1.ConditionsMap{
		AllowConditions: []authorizationv1.Condition{{ID: "p/a", Condition: "true"}},
	}}
	require.Equal(t, authorizationv1.SubjectAccessReviewStatus{Reason: "failed closed: caller does not handle conditional decisions"}, ToStatus(cm, false))
	cm.ConditionsMap.DenyConditions = []authorizationv1.Condition{{ID: "p/d", Condition: "true"}}
	require.True(t, ToStatus(cm, false).Denied)
	require.NotNil(t, ToStatus(cm, true).ConditionalDecision)
}

func admissionRequest(uid string) *admissionv1.AdmissionRequest {
	return &admissionv1.AdmissionRequest{UID: types.UID(uid)}
}
