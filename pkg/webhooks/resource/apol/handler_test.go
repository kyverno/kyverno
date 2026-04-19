package apol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies/v1alpha1"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type fakeProvider struct {
	policies []*apolcompiler.Policy
	err      error
}

func (f fakeProvider) Fetch(context.Context) ([]*apolcompiler.Policy, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.policies, nil
}

func compilePolicy(t *testing.T, pol *policiesv1alpha1.AuthorizingPolicy) *apolcompiler.Policy {
	t.Helper()
	compiled, errs := apolcompiler.NewCompiler().Compile(pol)
	if errs != nil {
		t.Fatalf("compile errors: %v", errs)
	}
	return compiled
}

func sarBody(t *testing.T) []byte {
	t.Helper()
	sar := &authorizationv1.SubjectAccessReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "authorization.k8s.io/v1",
			Kind:       "SubjectAccessReview",
		},
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:   "alice",
			UID:    "abc123",
			Groups: []string{"developers"},
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:      "create",
				Namespace: "authorize-test",
				Resource:  "pods",
			},
		},
	}
	body, err := json.Marshal(sar)
	if err != nil {
		t.Fatalf("failed to marshal SAR: %v", err)
	}
	return body
}

func sarBodyWithConditionsMode(t *testing.T, mode string) []byte {
	t.Helper()
	return []byte(`{
	"apiVersion": "authorization.k8s.io/v1",
	"kind": "SubjectAccessReview",
	"spec": {
		"conditionalAuthorization": {
			"mode": "` + mode + `"
		},
		"user": "alice",
		"groups": ["developers"],
		"resourceAttributes": {
			"verb": "create",
			"resource": "pods",
			"namespace": "authorize-test"
		}
	}
}`)
}

func TestHandleSubjectAccessReview_ConditionalResponse(t *testing.T) {
	pol := &policiesv1alpha1.AuthorizingPolicy{}
	pol.Name = "conditional"
	pol.Spec.Rules = []policiesv1alpha1.AuthorizingRule{
		{
			Name:       "with-conds",
			Effect:     policiesv1alpha1.AuthorizingRuleEffectConditional,
			Expression: "true",
			Conditions: []policiesv1alpha1.AuthorizingCondition{
				{
					ID:          "require-approved-label",
					Expression:  "request.namespace == 'authorize-test'",
					Effect:      policiesv1alpha1.AuthorizingConditionEffectAllow,
					Description: "requires approved label",
				},
			},
		},
	}

	handler := New(fakeProvider{policies: []*apolcompiler.Policy{compilePolicy(t, pol)}})
	req := httptest.NewRequest("POST", "/authz/subjectaccessreview", bytes.NewReader(sarBodyWithConditionsMode(t, "HumanReadable")))
	w := httptest.NewRecorder()

	handler.HandleSubjectAccessReview(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	status, ok := response["status"].(map[string]any)
	if !ok {
		t.Fatalf("missing status object in response")
	}
	if status["allowed"] != false {
		t.Fatalf("expected allowed=false for conditional response")
	}
	if status["denied"] != false {
		t.Fatalf("expected denied=false for conditional response")
	}
	chain, ok := status["conditionSetChain"].([]any)
	if !ok || len(chain) != 1 {
		t.Fatalf("expected one condition set in conditionSetChain")
	}
	set, ok := chain[0].(map[string]any)
	if !ok {
		t.Fatalf("expected conditionSetChain entry to be an object")
	}
	if set["authorizerName"] != "kyverno" {
		t.Fatalf("expected authorizerName=kyverno, got %v", set["authorizerName"])
	}
	conditions, ok := set["conditions"].([]any)
	if !ok || len(conditions) != 1 {
		t.Fatalf("expected one condition in condition set")
	}
	first, ok := conditions[0].(map[string]any)
	if !ok {
		t.Fatalf("expected condition to be an object")
	}
	if first["id"] != "require-approved-label" {
		t.Fatalf("unexpected condition id: %v", first["id"])
	}
	if first["effect"] != "Allow" {
		t.Fatalf("unexpected condition effect: %v", first["effect"])
	}
}

func TestHandleSubjectAccessReview_ConditionalResponseWithoutModeFolds(t *testing.T) {
	pol := &policiesv1alpha1.AuthorizingPolicy{}
	pol.Name = "conditional-no-mode"
	pol.Spec.Rules = []policiesv1alpha1.AuthorizingRule{
		{
			Name:       "with-conds",
			Effect:     policiesv1alpha1.AuthorizingRuleEffectConditional,
			Expression: "true",
			Conditions: []policiesv1alpha1.AuthorizingCondition{
				{
					ID:         "allow-cond",
					Expression: "true",
					Effect:     policiesv1alpha1.AuthorizingConditionEffectAllow,
				},
			},
		},
	}

	handler := New(fakeProvider{policies: []*apolcompiler.Policy{compilePolicy(t, pol)}})
	req := httptest.NewRequest("POST", "/authz/subjectaccessreview", bytes.NewReader(sarBody(t)))
	w := httptest.NewRecorder()

	handler.HandleSubjectAccessReview(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	status := response["status"].(map[string]any)
	if _, hasChain := status["conditionSetChain"]; hasChain {
		t.Fatalf("expected no conditionSetChain when conditions mode is unset")
	}
}

func TestHandleSubjectAccessReview_UnrecognisedModeFolds(t *testing.T) {
	pol := &policiesv1alpha1.AuthorizingPolicy{}
	pol.Name = "conditional-unknown-mode"
	pol.Spec.Rules = []policiesv1alpha1.AuthorizingRule{
		{
			Name:       "with-conds",
			Effect:     policiesv1alpha1.AuthorizingRuleEffectConditional,
			Expression: "true",
			Conditions: []policiesv1alpha1.AuthorizingCondition{
				{
					ID:         "allow-cond",
					Expression: "true",
					Effect:     policiesv1alpha1.AuthorizingConditionEffectAllow,
				},
			},
		},
	}

	handler := New(fakeProvider{policies: []*apolcompiler.Policy{compilePolicy(t, pol)}})
	// Send a mode value that is not in the known KEP-5681 enum.
	req := httptest.NewRequest("POST", "/authz/subjectaccessreview", bytes.NewReader(sarBodyWithConditionsMode(t, "UnknownFutureMode")))
	w := httptest.NewRecorder()

	handler.HandleSubjectAccessReview(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	status := response["status"].(map[string]any)
	// An unrecognised mode must be folded to a concrete decision – no conditionSetChain.
	if _, hasChain := status["conditionSetChain"]; hasChain {
		t.Fatalf("expected no conditionSetChain for unrecognised conditions mode")
	}
}

func TestHandleSubjectAccessReview_InvalidJSON(t *testing.T) {
	handler := New(fakeProvider{})
	req := httptest.NewRequest("POST", "/authz/subjectaccessreview", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()

	handler.HandleSubjectAccessReview(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestHandleSubjectAccessReview_BodyTooLarge(t *testing.T) {
	handler := New(fakeProvider{})
	req := httptest.NewRequest(
		"POST",
		"/authz/subjectaccessreview",
		bytes.NewReader(bytes.Repeat([]byte("a"), 1024*1024+1)),
	)
	w := httptest.NewRecorder()

	handler.HandleSubjectAccessReview(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "failed to read request body") {
		t.Fatalf("expected oversized body error, got %q", w.Body.String())
	}
}

func TestHandleSubjectAccessReview_FetchError(t *testing.T) {
	handler := New(fakeProvider{err: errors.New("provider unavailable")})
	req := httptest.NewRequest("POST", "/authz/subjectaccessreview", bytes.NewReader(sarBody(t)))
	w := httptest.NewRecorder()

	handler.HandleSubjectAccessReview(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	status := response["status"].(map[string]any)
	if status["allowed"] != false || status["denied"] != false {
		t.Fatalf("expected no-opinion response for provider fetch error")
	}
}

func TestHandleSubjectAccessReview_EngineErrorReturnsEvaluationError(t *testing.T) {
	pol := &policiesv1alpha1.AuthorizingPolicy{}
	pol.Name = "engine-error-policy"
	pol.Spec.Variables = []policiesv1alpha1.AuthorizingVariable{
		{
			Name:       "bad",
			Expression: "1 / 0",
		},
	}
	pol.Spec.Rules = []policiesv1alpha1.AuthorizingRule{
		{
			Name:       "runtime-error",
			Effect:     policiesv1alpha1.AuthorizingRuleEffectAllow,
			Expression: "variables.bad == 1",
		},
	}

	handler := New(fakeProvider{policies: []*apolcompiler.Policy{compilePolicy(t, pol)}})
	req := httptest.NewRequest("POST", "/authz/subjectaccessreview", bytes.NewReader(sarBody(t)))
	w := httptest.NewRecorder()

	handler.HandleSubjectAccessReview(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	status := response["status"].(map[string]any)
	if status["allowed"] != false || status["denied"] != false {
		t.Fatalf("expected no-opinion response for engine error")
	}
	evalErr, ok := status["evaluationError"].(string)
	if !ok || evalErr == "" {
		t.Fatalf("expected non-empty evaluationError in response status")
	}
	if !strings.Contains(evalErr, "engine-error-policy") {
		t.Fatalf("expected evaluationError to include policy name, got %q", evalErr)
	}
}

func TestHandleConditionsReview_WithObjectLabels(t *testing.T) {
	handler := New(fakeProvider{})
	payload := map[string]any{
		"apiVersion": "authorization.k8s.io/v1alpha1",
		"kind":       "AuthorizationConditionsReview",
		"request": map[string]any{
			"conditionSetChain": []map[string]any{
				{
					"authorizerName": "kyverno",
					"conditionsType": "k8s.io/cel",
					"conditions": []map[string]any{
						{"id": "allow-prod", "effect": "Allow", "condition": "request.resourceLabels['env'] == 'prod'"},
						{"id": "deny-non-prod", "effect": "Deny", "condition": "request.resourceLabels['env'] != 'prod'"},
					},
				},
			},
			"spec": map[string]any{
				"user":   "alice",
				"groups": []string{"developers"},
				"resourceAttributes": map[string]any{
					"verb":      "update",
					"resource":  "deployments",
					"namespace": "tenant-a-prod",
				},
			},
			"object": map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "api",
					"namespace": "tenant-a-prod",
					"labels": map[string]any{
						"env": "prod",
					},
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest("POST", "/authz/conditions", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleConditionsReview(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	respObj, ok := response["response"].(map[string]any)
	if !ok {
		t.Fatalf("expected response object in AuthorizationConditionsReview")
	}
	if respObj["allowed"] != true {
		t.Fatalf("expected allowed=true, got %v", respObj["allowed"])
	}
	if denied, ok := respObj["denied"].(bool); ok && denied {
		t.Fatalf("expected denied=false")
	}
}

func TestHandleConditionsReview_RequiresConditionSetChain(t *testing.T) {
	handler := New(fakeProvider{})
	req := httptest.NewRequest("POST", "/authz/conditions", bytes.NewReader([]byte(`{"apiVersion":"authorization.k8s.io/v1alpha1","kind":"AuthorizationConditionsReview","request":{"spec":{"user":"alice"}}}`)))
	w := httptest.NewRecorder()

	handler.HandleConditionsReview(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestHandleConditionsReview_BodyTooLarge(t *testing.T) {
	handler := New(fakeProvider{})
	req := httptest.NewRequest(
		"POST",
		"/authz/conditions",
		bytes.NewReader(bytes.Repeat([]byte("a"), 1024*1024+1)),
	)
	w := httptest.NewRecorder()

	handler.HandleConditionsReview(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "failed to read request body") {
		t.Fatalf("expected oversized body error, got %q", w.Body.String())
	}
}
