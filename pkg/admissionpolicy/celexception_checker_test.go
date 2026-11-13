package admissionpolicy

import (
	"fmt"
	"strings"
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

func exceptionWithMatchConditions(namespace, name string, exprs ...string) policiesv1beta1.PolicyException {
	polex := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec:       policiesv1beta1.PolicyExceptionSpec{},
	}
	for i, expr := range exprs {
		polex.Spec.MatchConditions = append(polex.Spec.MatchConditions, admissionregistrationv1.MatchCondition{
			Name:       fmt.Sprintf("cond-%d", i),
			Expression: expr,
		})
	}
	return polex
}

func Test_CanGenerateNativePolicy(t *testing.T) {
	hugeCond := `"` + strings.Repeat("a", 55000) + `" != ""`
	mediumCond := `"` + strings.Repeat("a", 40000) + `" != ""`

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
		name: "individually valid matchConditions whose combined expression exceeds CEL limit",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithMatchConditions("default", "oversized", hugeCond, hugeCond),
		},
		want:       false,
		wantReason: "policy exception(s) default/oversized produce combined matchConditions exceeding the Kubernetes CEL code point limit (100000), which a generated native policy cannot evaluate",
	}, {
		name: "multiple matchConditions within CEL limit",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithMatchConditions("default", "fits-limit", mediumCond, mediumCond),
		},
		want: true,
	}, {
		name: "only size-offending exceptions are named",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithMatchConditions("default", "ok", "true", "false"),
			exceptionWithMatchConditions("default", "too-big", hugeCond, hugeCond),
			exceptionWithMatchConditions("prod", "also-ok", mediumCond),
		},
		want:       false,
		wantReason: "policy exception(s) default/too-big produce combined matchConditions exceeding the Kubernetes CEL code point limit (100000), which a generated native policy cannot evaluate",
	}, {
		name: "both compensating controls and size limit violations reported",
		exceptions: []policiesv1beta1.PolicyException{
			exception("default", "guarded", "true"),
			exceptionWithMatchConditions("default", "too-big", hugeCond, hugeCond),
		},
		want:       false,
		wantReason: "policy exception(s) default/guarded define compensating controls (spec.validations), which a generated ValidatingAdmissionPolicy cannot evaluate; policy exception(s) default/too-big produce combined matchConditions exceeding the Kubernetes CEL code point limit (100000), which a generated native policy cannot evaluate",
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := CanGenerateNativePolicy(tc.exceptions)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}
