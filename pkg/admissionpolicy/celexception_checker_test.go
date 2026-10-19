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
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := CanGenerateNativePolicy(tc.exceptions)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}


func Test_CanGenerateNativePolicy_PartialExceptionConditions(t *testing.T) {
	imageScoped := exception("default", "image-scoped")
	imageScoped.Spec.Images = []string{"ghcr.io/example/app:v1"}

	valueScoped := exception("default", "value-scoped")
	valueScoped.Spec.AllowedValues = []string{"allowed"}

	unconditionalImageScope := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "unconditional-image-scope"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			Images: []string{"ghcr.io/example/app:v1"},
		},
	}

	testCases := []struct {
		name       string
		exceptions []policiesv1beta1.PolicyException
		want       bool
		wantReason string
	}{
		{
			name:       "image-scoped exception with match conditions",
			exceptions: []policiesv1beta1.PolicyException{imageScoped},
			want:       false,
			wantReason: "policy exception(s) default/image-scoped combine matchConditions with image/value scopes, which a generated ValidatingAdmissionPolicy cannot evaluate",
		},
		{
			name:       "value-scoped exception with match conditions",
			exceptions: []policiesv1beta1.PolicyException{valueScoped},
			want:       false,
			wantReason: "policy exception(s) default/value-scoped combine matchConditions with image/value scopes, which a generated ValidatingAdmissionPolicy cannot evaluate",
		},
		{
			name:       "image scope without match conditions is still supported",
			exceptions: []policiesv1beta1.PolicyException{unconditionalImageScope},
			want:       true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := CanGenerateNativePolicy(tc.exceptions)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}
