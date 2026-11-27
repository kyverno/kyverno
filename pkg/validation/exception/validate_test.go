package exception

import (
	"context"
	"strings"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/logging"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Validate(t *testing.T) {
	type args struct {
		opts     ValidationOptions
		resource []byte
	}
	tc := []struct {
		name string
		args args
		want int
	}{
		{
			name: "PolicyExceptions disabled.",
			args: args{
				opts: ValidationOptions{
					Enabled:   false,
					Namespace: "kyverno",
				},
				resource: []byte(`{"apiVersion":"kyverno.io/v2","kind":"PolicyException","metadata":{"name":"enforce-label-exception","namespace":"delta"},"spec":{"exceptions":[{"policyName":"enforce-label","ruleNames":["enforce-label"]}],"match":{"any":[{"resources":{"kinds":["Pod"]}}]}}}`),
			},
			want: 1,
		},
		{
			name: "PolicyExceptions enabled. Defined namespace doesn't match namespace passed.",
			args: args{
				opts: ValidationOptions{
					Enabled:   true,
					Namespace: "kyverno",
				},
				resource: []byte(`{"apiVersion":"kyverno.io/v2","kind":"PolicyException","metadata":{"name":"enforce-label-exception","namespace":"delta"},"spec":{"exceptions":[{"policyName":"enforce-label","ruleNames":["enforce-label"]}],"match":{"any":[{"resources":{"kinds":["Pod"]}}]}}}`),
			},
			want: 1,
		},
		{
			name: "PolicyExceptions enabled. Defined namespace matches namespace passed",
			args: args{
				opts: ValidationOptions{
					Enabled:   true,
					Namespace: "kyverno",
				},
				resource: []byte(`{"apiVersion":"kyverno.io/v2","kind":"PolicyException","metadata":{"name":"enforce-label-exception","namespace":"kyverno"},"spec":{"exceptions":[{"policyName":"enforce-label","ruleNames":["enforce-label"]}],"match":{"any":[{"resources":{"kinds":["Pod"]}}]}}}`),
			},
			want: 0,
		},
		{
			name: "PolicyExceptions enabled. All namespaces are enabled",
			args: args{
				opts: ValidationOptions{
					Enabled:   true,
					Namespace: "*",
				},
				resource: []byte(`{"apiVersion":"kyverno.io/v2","kind":"PolicyException","metadata":{"name":"enforce-label-exception","namespace":"kyverno"},"spec":{"exceptions":[{"policyName":"enforce-label","ruleNames":["enforce-label"]}],"match":{"any":[{"resources":{"kinds":["Pod"]}}]}}}`),
			},
			want: 0,
		},
	}
	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			polex, err := admissionutils.UnmarshalPolicyException(c.args.resource)
			assert.NilError(t, err)
			warnings := ValidateNamespace(context.Background(), logging.GlobalLogger(), polex.GetNamespace(), c.args.opts)
			assert.Assert(t, len(warnings) == c.want)
		})
	}
}

func TestValidateGlobLengths(t *testing.T) {
	overLimit := strings.Repeat("a", 2049)

	t.Run("over limit name in match.any is rejected", func(t *testing.T) {
		raw := []byte(`{"apiVersion":"kyverno.io/v2","kind":"PolicyException","metadata":{"name":"e","namespace":"ns"},"spec":{"exceptions":[{"policyName":"p","ruleNames":["r"]}],"match":{"any":[{"resources":{"kinds":["Pod"],"names":["` + overLimit + `"]}}]}}}`)
		polex, err := admissionutils.UnmarshalPolicyException(raw)
		assert.NilError(t, err)

		err = ValidateGlobLengths(polex)
		assert.Assert(t, err != nil)
		assert.ErrorContains(t, err, "exceeds the maximum allowed length")
	})

	t.Run("at limit name in match.all is accepted", func(t *testing.T) {
		atLimit := strings.Repeat("a", 2048)
		raw := []byte(`{"apiVersion":"kyverno.io/v2","kind":"PolicyException","metadata":{"name":"e","namespace":"ns"},"spec":{"exceptions":[{"policyName":"p","ruleNames":["r"]}],"match":{"all":[{"resources":{"kinds":["Pod"],"names":["` + atLimit + `"]}}]}}}`)
		polex, err := admissionutils.UnmarshalPolicyException(raw)
		assert.NilError(t, err)

		err = ValidateGlobLengths(polex)
		assert.NilError(t, err)
	})
}

func TestValidateGlobLengthsRejectsOversizedRuleName(t *testing.T) {
	overLimit := strings.Repeat("a", 2049)
	raw := []byte(`{"apiVersion":"kyverno.io/v2","kind":"PolicyException","metadata":{"name":"e","namespace":"ns"},"spec":{"exceptions":[{"policyName":"p","ruleNames":["` + overLimit + `"]}],"match":{"any":[{"resources":{"kinds":["Pod"]}}]}}}`)
	polex, err := admissionutils.UnmarshalPolicyException(raw)
	assert.NilError(t, err)

	err = ValidateGlobLengths(polex)
	assert.Assert(t, err != nil)
	assert.ErrorContains(t, err, "exceeds the maximum allowed length")
}

func TestValidateGlobLengthsAcceptsRuleNameAtLimit(t *testing.T) {
	atLimit := strings.Repeat("a", 2048)
	raw := []byte(`{"apiVersion":"kyverno.io/v2","kind":"PolicyException","metadata":{"name":"e","namespace":"ns"},"spec":{"exceptions":[{"policyName":"p","ruleNames":["` + atLimit + `"]}],"match":{"any":[{"resources":{"kinds":["Pod"]}}]}}}`)
	polex, err := admissionutils.UnmarshalPolicyException(raw)
	assert.NilError(t, err)

	err = ValidateGlobLengths(polex)
	assert.NilError(t, err)
}

func Test_ValidateScopeReach(t *testing.T) {
	namespacedRef := []policiesv1beta1.PolicyRef{{Name: "require-team-label", Kind: "NamespacedValidatingPolicy"}}
	clusterRef := []policiesv1beta1.PolicyRef{{Name: "cluster-require-owner-label", Kind: "ValidatingPolicy"}}

	testCases := []struct {
		name string
		opts ValidationOptions
		refs []policiesv1beta1.PolicyRef
		want int
	}{
		{
			name: "disabled: no warning regardless of scope",
			opts: ValidationOptions{Enabled: false, Namespace: "*"},
			refs: namespacedRef,
			want: 0,
		},
		{
			name: "concrete scope: no warning, central namespace covers the exception",
			opts: ValidationOptions{Enabled: true, Namespace: "kyverno"},
			refs: namespacedRef,
			want: 0,
		},
		{
			name: "unset scope: no warning here, ValidateNamespace already covers it",
			opts: ValidationOptions{Enabled: true, Namespace: ""},
			refs: namespacedRef,
			want: 0,
		},
		{
			name: "all-namespaces scope targeting a namespaced policy: warns, no central namespace exists",
			opts: ValidationOptions{Enabled: true, Namespace: "*"},
			refs: namespacedRef,
			want: 1,
		},
		{
			name: "all-namespaces scope targeting a cluster-scoped policy: no warning, documented reach",
			opts: ValidationOptions{Enabled: true, Namespace: "*"},
			refs: clusterRef,
			want: 0,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			polex := &policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{Namespace: "tenant-b"},
				Spec:       policiesv1beta1.PolicyExceptionSpec{PolicyRefs: tc.refs},
			}
			warnings := ValidateScopeReach(polex, tc.opts)
			assert.Equal(t, tc.want, len(warnings))
		})
	}
}
