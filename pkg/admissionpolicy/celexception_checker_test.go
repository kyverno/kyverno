package admissionpolicy

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func exception(namespace, name string, validations ...string) policiesv1beta1.PolicyException {
	polex := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "match",
				Expression: "true",
			}},
		},
	}
	for _, expression := range validations {
		polex.Spec.Validations = append(polex.Spec.Validations, admissionregistrationv1.Validation{
			Expression: expression,
		})
	}
	return polex
}

func exceptionWithCondition(namespace, name, expression string) policiesv1beta1.PolicyException {
	polex := exception(namespace, name)
	polex.Spec.MatchConditions[0].Expression = expression
	return polex
}

func Test_CanGenerateNativePolicy(t *testing.T) {
	testCases := []struct {
		name       string
		exceptions []policiesv1beta1.PolicyException
		want       bool
		wantReason string
	}{{
		name:       "no exceptions",
		exceptions: nil,
		want:       true,
	}, {
		name:       "exception without compensating controls",
		exceptions: []policiesv1beta1.PolicyException{exception("default", "polex-1")},
		want:       true,
	}, {
		name:       "exception with compensating controls",
		exceptions: []policiesv1beta1.PolicyException{exception("default", "polex-1", "object.metadata.name != ''")},
		want:       false,
		wantReason: "policy exception(s) default/polex-1 define compensating controls (spec.validations), which a generated ValidatingAdmissionPolicy cannot evaluate",
	}, {
		name: "only the offending exceptions are named",
		exceptions: []policiesv1beta1.PolicyException{
			exception("default", "plain"),
			exception("default", "guarded", "true"),
			exception("other", "also-guarded", "true"),
		},
		want:       false,
		wantReason: "policy exception(s) default/guarded, other/also-guarded define compensating controls (spec.validations), which a generated ValidatingAdmissionPolicy cannot evaluate",
	}, {
		name: "exception with native match conditions",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithCondition("default", "native", `namespaceObject.metadata.labels["team"] == "infra" && request.userInfo.username != ""`),
		},
		want: true,
	}, {
		name: "exception with a match condition using a Kyverno CEL library",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithCondition("default", "native", "true"),
			exceptionWithCondition("kyverno", "infra", `resource.Get("v1", "namespaces", "", object.metadata.namespace).metadata.labels["team"] == "infra"`),
		},
		want:       false,
		wantReason: "policy exception(s) kyverno/infra use matchConditions that the API server cannot compile in a generated ValidatingAdmissionPolicy, such as calls to the Kyverno CEL libraries",
	}, {
		name: "both reasons are reported",
		exceptions: []policiesv1beta1.PolicyException{
			exception("default", "guarded", "true"),
			exceptionWithCondition("kyverno", "infra", `resource.Get("v1", "namespaces", "", "default").metadata.name == "default"`),
		},
		want:       false,
		wantReason: "policy exception(s) default/guarded define compensating controls (spec.validations), which a generated ValidatingAdmissionPolicy cannot evaluate; policy exception(s) kyverno/infra use matchConditions that the API server cannot compile in a generated ValidatingAdmissionPolicy, such as calls to the Kyverno CEL libraries",
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := CanGenerateNativePolicy(tc.exceptions)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}
