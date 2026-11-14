package apol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	authorizationv1 "k8s.io/api/authorization/v1"
	authorizationv1alpha1 "k8s.io/api/authorization/v1alpha1"
)

func sarBody(verb string, handled ...string) string {
	opts := ""
	if handled != nil {
		raw, _ := json.Marshal(handled)
		opts = fmt.Sprintf(`,"authorizationOptions":{"handledDecisionTypes":%s}`, raw)
	}
	return fmt.Sprintf(`{"apiVersion":"authorization.k8s.io/v1","kind":"SubjectAccessReview","spec":{"user":"alice","groups":["dev"],
	  "resourceAttributes":{"namespace":"team-a","verb":%q,"version":"v1","resource":"configmaps","name":"cm"}%s}}`, verb, opts)
}

func reviewSAR(t *testing.T, h *Handler, body string) authorizationv1.SubjectAccessReviewStatus {
	t.Helper()
	raw, status, err := h.Review(context.Background(), []byte(body), "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	var out authorizationv1.SubjectAccessReview
	require.NoError(t, json.Unmarshal(raw, &out))
	return out.Status
}

// TestDecisionTypeNegotiation checks that a conditional decision is returned
// only when handledDecisionTypes is a superset of {ConditionsMap, Union}, and
// that every other caller gets the fail-closed unconditional decision.
func TestDecisionTypeNegotiation(t *testing.T) {
	t.Parallel()
	h := newHandler(t, ownerPolicy())
	tests := []struct {
		name        string
		handled     []string
		conditional bool
	}{
		{"no authorizationOptions", nil, false},
		{"empty handledDecisionTypes", []string{}, false},
		{"unconditional only", []string{"Allow", "Deny", "NoOpinion"}, false},
		{"ConditionsMap without Union", []string{"Allow", "Deny", "NoOpinion", "ConditionsMap"}, false},
		{"Union without ConditionsMap", []string{"Allow", "Deny", "NoOpinion", "Union"}, false},
		{"all decision types", []string{"Allow", "Deny", "NoOpinion", "ConditionsMap", "Union"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			st := reviewSAR(t, h, sarBody("update", tt.handled...))
			if tt.conditional {
				require.NotNil(t, st.ConditionalDecision)
				require.Equal(t, authorizationv1.ConditionsAwareDecisionTypeConditionsMap, st.ConditionalDecision.Type)
				require.False(t, st.Allowed)
				require.False(t, st.Denied)
				require.Empty(t, st.Reason)
				require.Empty(t, st.EvaluationError)
				return
			}
			// The owner policy has a residual Deny on writes, so an unaware
			// caller must be denied, never allowed.
			require.Nil(t, st.ConditionalDecision)
			require.True(t, st.Denied, "%+v", st)
			require.False(t, st.Allowed)
		})
	}
}

// TestReadsAreNeverConditional checks that reads, which carry no object at
// admission, get an unconditional decision even from conditions-aware callers.
func TestReadsAreNeverConditional(t *testing.T) {
	t.Parallel()
	h := newHandler(t, ownerPolicy())
	for _, verb := range []string{"get", "list", "watch"} {
		st := reviewSAR(t, h, sarBody(verb, "Allow", "Deny", "NoOpinion", "ConditionsMap", "Union"))
		require.Nil(t, st.ConditionalDecision, verb)
		require.True(t, st.Allowed, "%s: %+v", verb, st)
	}
}

func TestMalformedWireInput(t *testing.T) {
	t.Parallel()
	h := newHandler(t, ownerPolicy())
	tests := []struct {
		name string
		body string
	}{
		{"empty body", ``},
		{"json array", `[]`},
		{"json string", `"SubjectAccessReview"`},
		{"missing TypeMeta", `{"spec":{"user":"alice"}}`},
		{"kind without apiVersion", `{"kind":"SubjectAccessReview","spec":{"user":"alice"}}`},
		{"wrong group", `{"apiVersion":"authorization.example.com/v1","kind":"SubjectAccessReview"}`},
		{"kind is case sensitive", `{"apiVersion":"authorization.k8s.io/v1","kind":"subjectaccessreview"}`},
		{"v1beta1 conditions review", `{"apiVersion":"authorization.k8s.io/v1beta1","kind":"AuthorizationConditionsReview"}`},
		{"groups is not a list", `{"apiVersion":"authorization.k8s.io/v1","kind":"SubjectAccessReview","spec":{"user":"alice","groups":"dev"}}`},
		{"resourceAttributes is not an object", `{"apiVersion":"authorization.k8s.io/v1","kind":"SubjectAccessReview","spec":{"user":"alice","resourceAttributes":"pods"}}`},
		{"handledDecisionTypes is not a list", `{"apiVersion":"authorization.k8s.io/v1","kind":"SubjectAccessReview","spec":{"user":"alice","authorizationOptions":{"handledDecisionTypes":"ConditionsMap"}}}`},
		{"truncated SubjectAccessReview", sarBody("create")[:60]},
		{"decision is not an object", `{"apiVersion":"authorization.k8s.io/v1alpha1","kind":"AuthorizationConditionsReview","request":{"decision":"Allow","admissionRequest":{"uid":"u"}}}`},
		{"conditions is not a list", `{"apiVersion":"authorization.k8s.io/v1alpha1","kind":"AuthorizationConditionsReview","request":{"decision":{"type":"ConditionsMap","conditionsMap":{"allowConditions":{}}},"admissionRequest":{"uid":"u"}}}`},
		{"uid is not a string", `{"apiVersion":"authorization.k8s.io/v1alpha1","kind":"AuthorizationConditionsReview","request":{"decision":{"type":"Allow","allow":{}},"admissionRequest":{"uid":1}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			raw, status, err := h.Review(context.Background(), []byte(tt.body), "")
			require.Error(t, err)
			require.Equal(t, http.StatusBadRequest, status)
			require.Nil(t, raw)
		})
	}
}

// TestInvalidSubjectAccessReviewSpecsAreDenied checks that well-formed JSON
// that fails upstream SubjectAccessReview validation is denied, never allowed.
func TestInvalidSubjectAccessReviewSpecsAreDenied(t *testing.T) {
	t.Parallel()
	h := newHandler(t, ownerPolicy())
	tests := []struct {
		name string
		spec string
	}{
		{"no attributes", `{"user":"alice","groups":["dev"]}`},
		{"no user or groups", `{"resourceAttributes":{"verb":"get","version":"v1","resource":"configmaps"}}`},
		{"unknown handled decision type", `{"user":"alice","groups":["dev"],"resourceAttributes":{"verb":"get","version":"v1","resource":"configmaps"},"authorizationOptions":{"handledDecisionTypes":["Bogus"]}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			st := reviewSAR(t, h, `{"apiVersion":"authorization.k8s.io/v1","kind":"SubjectAccessReview","spec":`+tt.spec+`}`)
			require.False(t, st.Allowed, "%+v", st)
			require.Nil(t, st.ConditionalDecision)
			if st.Denied {
				require.NotEmpty(t, st.EvaluationError)
			}
		})
	}
}

// TestConditionsReviewEchoesEachUID checks the UID echo for distinct requests
// and that the callback answers from the decision it is given.
func TestConditionsReviewEchoesEachUID(t *testing.T) {
	t.Parallel()
	h := newHandler(t, ownerPolicy())
	st := reviewSAR(t, h, sarBody("create", "Allow", "Deny", "NoOpinion", "ConditionsMap", "Union"))
	require.NotNil(t, st.ConditionalDecision)
	tests := []struct {
		uid    string
		object string
		want   authorizationv1.ConditionsAwareDecisionType
	}{
		{"uid-owner", `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm","labels":{"owner":"alice"}}}`, authorizationv1.ConditionsAwareDecisionTypeAllow},
		{"uid-protected", `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm","labels":{"owner":"alice","protected":"x"}}}`, authorizationv1.ConditionsAwareDecisionTypeDeny},
		{"uid-other", `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm","labels":{"owner":"bob"}}}`, authorizationv1.ConditionsAwareDecisionTypeNoOpinion},
		// A non-object payload makes the residual Deny error, and an erroring Deny applies.
		{"uid-array", `[1,2]`, authorizationv1.ConditionsAwareDecisionTypeDeny},
		{"uid-scalar", `"cm"`, authorizationv1.ConditionsAwareDecisionTypeDeny},
	}
	for _, tt := range tests {
		decision, err := json.Marshal(st.ConditionalDecision)
		require.NoError(t, err)
		body := fmt.Sprintf(`{"apiVersion":"authorization.k8s.io/v1alpha1","kind":"AuthorizationConditionsReview","request":{"decision":%s,
		  "admissionRequest":{"uid":%q,"kind":{"group":"","version":"v1","kind":"ConfigMap"},"resource":{"group":"","version":"v1","resource":"configmaps"},
		  "operation":"CREATE","userInfo":{"username":"alice"},"object":%s}}}`, decision, tt.uid, tt.object)
		raw, status, err := h.Review(context.Background(), []byte(body), AuthorizationConditionsReview)
		require.NoError(t, err, tt.uid)
		require.Equal(t, http.StatusOK, status)
		var out authorizationv1alpha1.AuthorizationConditionsReview
		require.NoError(t, json.Unmarshal(raw, &out))
		require.Equal(t, tt.uid, string(out.Response.UID))
		require.Equal(t, tt.want, out.Response.Decision.Type, "%s: %+v", tt.uid, out.Response.Decision)
	}
}

func TestHTTPEndpoints(t *testing.T) {
	t.Parallel()
	h := newHandler(t, ownerPolicy())
	acr := `{"apiVersion":"authorization.k8s.io/v1alpha1","kind":"AuthorizationConditionsReview","request":{"decision":{"type":"Allow","allow":{}},"admissionRequest":{"uid":"u"}}}`
	tests := []struct {
		name    string
		handler http.HandlerFunc
		body    string
		status  int
	}{
		{"SAR endpoint serves SubjectAccessReview", h.HandleSubjectAccessReview, sarBody("get"), http.StatusOK},
		{"SAR endpoint rejects conditions review", h.HandleSubjectAccessReview, acr, http.StatusBadRequest},
		{"conditions endpoint serves conditions review", h.HandleConditionsReview, acr, http.StatusOK},
		{"conditions endpoint rejects SubjectAccessReview", h.HandleConditionsReview, sarBody("get"), http.StatusBadRequest},
		{"combined endpoint serves SubjectAccessReview", h.HandleCombined, sarBody("get"), http.StatusOK},
		{"combined endpoint serves conditions review", h.HandleCombined, acr, http.StatusOK},
		{"oversized body", h.HandleCombined, `{"apiVersion":"authorization.k8s.io/v1","kind":"SubjectAccessReview","spec":{"user":"` + strings.Repeat("a", MaxRequestBytes) + `"}}`, http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			tt.handler(rec, httptest.NewRequest(http.MethodPost, CombinedPath, bytes.NewBufferString(tt.body)))
			require.Equal(t, tt.status, rec.Code, rec.Body.String())
			if tt.status == http.StatusOK {
				require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			}
		})
	}
}
