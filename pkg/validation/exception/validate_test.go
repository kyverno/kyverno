package exception

import (
	"context"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/pkg/logging"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"gotest.tools/v3/assert"
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
