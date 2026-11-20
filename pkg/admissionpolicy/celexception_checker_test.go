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

func exceptionWithImages(namespace, name string, images []string, matchConds ...string) policiesv1beta1.PolicyException {
	polex := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			Images: images,
		},
	}
	for i, expr := range matchConds {
		polex.Spec.MatchConditions = append(polex.Spec.MatchConditions, admissionregistrationv1.MatchCondition{
			Name:       fmt.Sprintf("cond-%d", i),
			Expression: expr,
		})
	}
	return polex
}

func exceptionWithAllowedValues(namespace, name string, values []string, matchConds ...string) policiesv1beta1.PolicyException {
	polex := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			AllowedValues: values,
		},
	}
	for i, expr := range matchConds {
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
	}, {
		name: "unscoped exception with images is allowed",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithImages("default", "unscoped-images", []string{"nginx:latest", "redis:alpine"}),
		},
		want: true,
	}, {
		name: "unscoped exception with allowedValues is allowed",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithAllowedValues("default", "unscoped-values", []string{"FOO", "BAR"}),
		},
		want: true,
	}, {
		name: "scoped exception with images and matchConditions is rejected",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithImages("dev", "scoped-images", []string{"debug:latest"}, "request.namespace == 'dev'"),
		},
		want:       false,
		wantReason: "policy exception(s) dev/scoped-images define fine-grained exceptions (spec.images / spec.allowedValues) scoped with spec.matchConditions, which a generated ValidatingAdmissionPolicy cannot evaluate safely",
	}, {
		name: "scoped exception with allowedValues and matchConditions is rejected",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithAllowedValues("dev", "scoped-values", []string{"ALLOW_ALL"}, "request.namespace == 'dev'"),
		},
		want:       false,
		wantReason: "policy exception(s) dev/scoped-values define fine-grained exceptions (spec.images / spec.allowedValues) scoped with spec.matchConditions, which a generated ValidatingAdmissionPolicy cannot evaluate safely",
	}, {
		name: "mixed scoped and unscoped exceptions only names scoped offender",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithImages("default", "unscoped-images", []string{"nginx:latest"}),
			exceptionWithImages("dev", "scoped-images", []string{"debug:latest"}, "request.namespace == 'dev'"),
		},
		want:       false,
		wantReason: "policy exception(s) dev/scoped-images define fine-grained exceptions (spec.images / spec.allowedValues) scoped with spec.matchConditions, which a generated ValidatingAdmissionPolicy cannot evaluate safely",
	}, {
		name: "unscoped full exemption without matchConditions is rejected",
		exceptions: []policiesv1beta1.PolicyException{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "full-exemption"},
				Spec:       policiesv1beta1.PolicyExceptionSpec{},
			},
		},
		want:       false,
		wantReason: "policy exception(s) default/full-exemption define full exemptions without matchConditions, which a generated ValidatingAdmissionPolicy cannot evaluate",
	}, {
		name: "unscoped images exceeding CEL code point limit",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithImages("default", "huge-images", []string{strings.Repeat("a", 100005)}),
		},
		want:       false,
		wantReason: "combined fine-grained list(s) allowedImages exceed the Kubernetes CEL code point limit (100000), which a generated native policy cannot evaluate",
	}, {
		name: "unscoped allowedValues exceeding CEL code point limit",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithAllowedValues("default", "huge-values", []string{strings.Repeat("v", 100005)}),
		},
		want:       false,
		wantReason: "combined fine-grained list(s) allowedValues exceed the Kubernetes CEL code point limit (100000), which a generated native policy cannot evaluate",
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := CanGenerateNativePolicy(tc.exceptions)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}

func Test_CanGenerateNativeMutatingPolicy(t *testing.T) {
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
		name:       "exception with compensating controls is allowed for MAP (size-only check)",
		exceptions: []policiesv1beta1.PolicyException{exception("default", "polex-1", "object.metadata.name != ''")},
		want:       true,
	}, {
		name: "individually valid matchConditions whose combined expression exceeds CEL limit",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithMatchConditions("default", "oversized", hugeCond, hugeCond),
		},
		want:       false,
		wantReason: "policy exception(s) default/oversized produce combined matchConditions exceeding the Kubernetes CEL code point limit (100000), which a generated MutatingAdmissionPolicy cannot evaluate",
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
		wantReason: "policy exception(s) default/too-big produce combined matchConditions exceeding the Kubernetes CEL code point limit (100000), which a generated MutatingAdmissionPolicy cannot evaluate",
	}, {
		name: "exception with images is rejected for mutating admission policy",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithImages("default", "mpol-images", []string{"nginx:latest"}),
		},
		want:       false,
		wantReason: "policy exception(s) default/mpol-images define fine-grained exceptions (spec.images / spec.allowedValues), which a generated MutatingAdmissionPolicy cannot evaluate",
	}, {
		name: "exception with allowedValues is rejected for mutating admission policy",
		exceptions: []policiesv1beta1.PolicyException{
			exceptionWithAllowedValues("default", "mpol-values", []string{"VALUE"}),
		},
		want:       false,
		wantReason: "policy exception(s) default/mpol-values define fine-grained exceptions (spec.images / spec.allowedValues), which a generated MutatingAdmissionPolicy cannot evaluate",
	}, {
		name: "unscoped full exemption without matchConditions is rejected for mutating admission policy",
		exceptions: []policiesv1beta1.PolicyException{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mpol-full-exemption"},
				Spec:       policiesv1beta1.PolicyExceptionSpec{},
			},
		},
		want:       false,
		wantReason: "policy exception(s) default/mpol-full-exemption define full exemptions without matchConditions, which a generated MutatingAdmissionPolicy cannot evaluate",
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := CanGenerateNativeMutatingPolicy(tc.exceptions)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}

func Test_CanGenerateNativeValidatingPolicy(t *testing.T) {
	testCases := []struct {
		name       string
		policy     *policiesv1beta1.ValidatingPolicy
		exceptions []policiesv1beta1.PolicyException
		want       bool
		wantReason string
	}{
		{
			name: "policy matchConditions referencing exceptions.allowedImages is rejected",
			policy: &policiesv1beta1.ValidatingPolicy{
				Spec: policiesv1beta1.ValidatingPolicySpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "check-img",
						Expression: "request.image in exceptions.allowedImages",
					}},
				},
			},
			want:       false,
			wantReason: "policy spec.matchConditions reference exception variables (allowedImages/allowedValues), which cannot be evaluated in native ValidatingAdmissionPolicy matchConditions",
		},
		{
			name: "policy matchConditions referencing exceptions.allowedValues is rejected",
			policy: &policiesv1beta1.ValidatingPolicy{
				Spec: policiesv1beta1.ValidatingPolicySpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "check-val",
						Expression: "val in exceptions.allowedValues",
					}},
				},
			},
			want:       false,
			wantReason: "policy spec.matchConditions reference exception variables (allowedImages/allowedValues), which cannot be evaluated in native ValidatingAdmissionPolicy matchConditions",
		},
		{
			name: "policy matchConditions referencing variables is rejected",
			policy: &policiesv1beta1.ValidatingPolicy{
				Spec: policiesv1beta1.ValidatingPolicySpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "check-var",
						Expression: "variables.isProduction == true",
					}},
				},
			},
			want:       false,
			wantReason: "policy spec.matchConditions reference variables, which cannot be evaluated in native ValidatingAdmissionPolicy matchConditions",
		},
		{
			name: "valid policy delegates to exception eligibility checks",
			policy: &policiesv1beta1.ValidatingPolicy{
				Spec: policiesv1beta1.ValidatingPolicySpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "normal-cond",
						Expression: "request.operation == 'CREATE'",
					}},
				},
			},
			exceptions: []policiesv1beta1.PolicyException{
				exception("default", "polex-compensating", "object.metadata.name != ''"),
			},
			want:       false,
			wantReason: "policy exception(s) default/polex-compensating define compensating controls (spec.validations), which a generated ValidatingAdmissionPolicy cannot evaluate",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := CanGenerateNativeValidatingPolicy(tc.policy, tc.exceptions)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}
