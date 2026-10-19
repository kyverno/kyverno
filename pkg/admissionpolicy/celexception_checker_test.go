package admissionpolicy

import (
	"fmt"
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

func Test_CanGenerateNativeMutatingPolicy(t *testing.T) {
	fullException := exception("default", "full")
	imageScoped := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "image-scoped"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			Images: []string{"ghcr.io/example/app:v1"},
		},
	}
	valueScoped := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "value-scoped"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			AllowedValues: []string{"allowed"},
		},
	}
	guarded := exception("default", "guarded", "object.metadata.name != ''")

	testCases := []struct {
		name       string
		exceptions []policiesv1beta1.PolicyException
		want       bool
		wantReason string
	}{
		{
			name:       "full exception with match conditions is representable",
			exceptions: []policiesv1beta1.PolicyException{fullException},
			want:       true,
		},
		{
			name:       "image scope is not natively represented",
			exceptions: []policiesv1beta1.PolicyException{imageScoped},
			want:       false,
			wantReason: "policy exception(s) default/image-scoped use image/value scopes or compensating controls, which a generated MutatingAdmissionPolicy cannot evaluate",
		},
		{
			name:       "value scope is not natively represented",
			exceptions: []policiesv1beta1.PolicyException{valueScoped},
			want:       false,
			wantReason: "policy exception(s) default/value-scoped use image/value scopes or compensating controls, which a generated MutatingAdmissionPolicy cannot evaluate",
		},
		{
			name:       "compensating controls are not natively represented",
			exceptions: []policiesv1beta1.PolicyException{guarded},
			want:       false,
			wantReason: "policy exception(s) default/guarded use image/value scopes or compensating controls, which a generated MutatingAdmissionPolicy cannot evaluate",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := CanGenerateNativeMutatingPolicy(tc.exceptions)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}

func Test_CanGenerateNativePolicies_MatchConditionLimit(t *testing.T) {
	globalExceptions := []policiesv1beta1.PolicyException{
		{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "full-one"}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "full-two"}},
	}
	conditionedExceptions := make([]policiesv1beta1.PolicyException, 65)
	for i := range conditionedExceptions {
		conditionedExceptions[i] = policiesv1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: fmt.Sprintf("conditioned-%d", i)},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{{
					Name:       fmt.Sprintf("condition-%d", i),
					Expression: "true",
				}},
			},
		}
	}

	tests := []struct {
		name          string
		exceptions    []policiesv1beta1.PolicyException
		existingCount int
		want          bool
		wantReason    string
	}{
		{
			name:          "multiple global exceptions collapse to one match condition",
			exceptions:    globalExceptions,
			existingCount: 63,
			want:          true,
		},
		{
			name:          "policy and exception conditions cannot exceed limit",
			exceptions:    globalExceptions[:1],
			existingCount: 64,
			want:          false,
			wantReason:    "generated ValidatingAdmissionPolicy would contain 65 matchConditions; Kubernetes allows at most 64",
		},
		{
			name:          "too many exception conditions skip native generation",
			exceptions:    conditionedExceptions,
			existingCount: 0,
			want:          false,
			wantReason:    "generated ValidatingAdmissionPolicy would contain 65 matchConditions; Kubernetes allows at most 64",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := CanGenerateNativePolicyWithMatchConditions(tc.exceptions, tc.existingCount)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantReason, reason)
		})
	}

	got, reason := CanGenerateNativeMutatingPolicyWithMatchConditions(globalExceptions, 63)
	assert.Equal(t, true, got)
	assert.Equal(t, "", reason)
	got, reason = CanGenerateNativeMutatingPolicyWithMatchConditions(globalExceptions[:1], 64)
	assert.Equal(t, false, got)
	assert.Equal(t, "generated MutatingAdmissionPolicy would contain 65 matchConditions; Kubernetes allows at most 64", reason)
}
