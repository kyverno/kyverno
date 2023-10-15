package exception

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/logging"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"gotest.tools/assert"
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
				resource: []byte(`{"apiVersion":"kyverno.io/v2beta1","kind":"PolicyException","metadata":{"name":"enforce-label-exception","namespace":"delta"},"spec":{"exceptions":[{"policyName":"enforce-label","ruleNames":["enforce-label"]}],"match":{"any":[{"resources":{"kinds":["Pod"]}}]}}}`),
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
				resource: []byte(`{"apiVersion":"kyverno.io/v2beta1","kind":"PolicyException","metadata":{"name":"enforce-label-exception","namespace":"delta"},"spec":{"exceptions":[{"policyName":"enforce-label","ruleNames":["enforce-label"]}],"match":{"any":[{"resources":{"kinds":["Pod"]}}]}}}`),
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
				resource: []byte(`{"apiVersion":"kyverno.io/v2beta1","kind":"PolicyException","metadata":{"name":"enforce-label-exception","namespace":"kyverno"},"spec":{"exceptions":[{"policyName":"enforce-label","ruleNames":["enforce-label"]}],"match":{"any":[{"resources":{"kinds":["Pod"]}}]}}}`),
			},
			want: 0,
		},
		{
			name: "PolicyExceptions enabled. No namespace defined",
			args: args{
				opts: ValidationOptions{
					Enabled:   true,
					Namespace: "",
				},
				resource: []byte(`{"apiVersion":"kyverno.io/v2beta1","kind":"PolicyException","metadata":{"name":"enforce-label-exception","namespace":"kyverno"},"spec":{"exceptions":[{"policyName":"enforce-label","ruleNames":["enforce-label"]}],"match":{"any":[{"resources":{"kinds":["Pod"]}}]}}}`),
			},
			want: 0,
		},
	}
	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			polex, err := admissionutils.UnmarshalPolicyException(c.args.resource)
			assert.NilError(t, err)
			warnings, err := Validate(context.Background(), logging.GlobalLogger(), polex, c.args.opts)
			assert.NilError(t, err)
			assert.Assert(t, len(warnings) == c.want)
		})
	}
}

func Test_ValidateVariables(t *testing.T) {
	tc := []struct {
		name     string
		resource []byte
		error    bool
	}{
		{
			name:     "Variable used.",
			resource: []byte(`{"apiVersion":"kyverno.io/v2beta1","kind":"PolicyException","metadata":{"name":"enforce-label-polex"},"spec":{"background":true,"exceptions":[{"policyName":"enforce-label","ruleNames":["enforce-label"]}],"match":{"any":[{"resources":{"kinds":["Pod"],"namespaces":["{{request.object.name}}"],"names":["{{request.userInfo.username}}"]}}]}}}`),
			error:    true,
		},
		{
			name:     "Variable not used.",
			resource: []byte(`{"apiVersion":"kyverno.io/v2beta1","kind":"PolicyException","metadata":{"name":"enforce-label-polex"},"spec":{"background":true,"exceptions":[{"policyName":"enforce-label","ruleNames":["enforce-label"]}],"match":{"any":[{"resources":{"kinds":["Pod"]}}]}}}`),
			error:    false,
		},
	}
	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			polex, err := admissionutils.UnmarshalPolicyException(c.resource)
			assert.NilError(t, err)
			err = v2beta1.ValidateVariables(polex)
			if c.error {
				assert.Assert(t, err != nil)
			} else {
				assert.Assert(t, err)
			}
		})
	}
}
