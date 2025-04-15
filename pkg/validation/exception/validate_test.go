package exception

import (
	"context"
	"testing"

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
		{
			name: "CELPolicyExceptions disabled.",
			args: args{
				opts: ValidationOptions{
					Enabled:   false,
					Namespace: "kyverno",
				},
				resource: []byte(`{
    "apiVersion": "policies.kyverno.io/v1alpha1",
    "kind": "PolicyException",
    "metadata": {
        "name": "pod-security-exception",
        "namespace": "delta"
    },
    "spec": {
        "policyRefs": [
            {
                "name": "require-run-as-nonroot"
            }
        ],
        "matchConditions": [
            {
                "name": "check-namespace",
                "expression": "object.metadata.namespace == 'test-ns'"
            }
        ]
    }
}`),
			},
			want: 1,
		},
		{
			name: "CELPolicyExceptions enabled. Defined namespace doesn't match namespace passed.",
			args: args{
				opts: ValidationOptions{
					Enabled:   true,
					Namespace: "kyverno",
				},
				resource: []byte(`{
    "apiVersion": "policies.kyverno.io/v1alpha1",
    "kind": "PolicyException",
    "metadata": {
        "name": "pod-security-exception",
        "namespace": "delta"
    },
    "spec": {
        "policyRefs": [
            {
                "name": "require-run-as-nonroot"
            }
        ],
        "matchConditions": [
            {
                "name": "check-namespace",
                "expression": "object.metadata.namespace == 'test-ns'"
            }
        ]
    }
}`),
			},
			want: 1,
		},
		{
			name: "CELPolicyExceptions enabled. Defined namespace matches namespace passed",
			args: args{
				opts: ValidationOptions{
					Enabled:   true,
					Namespace: "delta",
				},
				resource: []byte(`{
    "apiVersion": "policies.kyverno.io/v1alpha1",
    "kind": "PolicyException",
    "metadata": {
        "name": "pod-security-exception",
        "namespace": "delta"
    },
    "spec": {
        "policyRefs": [
            {
                "name": "require-run-as-nonroot"
            }
        ],
        "matchConditions": [
            {
                "name": "check-namespace",
                "expression": "object.metadata.namespace == 'test-ns'"
            }
        ]
    }
}`),
			},
			want: 0,
		},
		{
			name: "CELPolicyExceptions enabled. All namespaces are enabled",
			args: args{
				opts: ValidationOptions{
					Enabled:   true,
					Namespace: "*",
				},
				resource: []byte(`{
    "apiVersion": "policies.kyverno.io/v1alpha1",
    "kind": "PolicyException",
    "metadata": {
        "name": "pod-security-exception",
        "namespace": "delta"
    },
    "spec": {
        "policyRefs": [
            {
                "name": "require-run-as-nonroot"
            }
        ],
        "matchConditions": [
            {
                "name": "check-namespace",
                "expression": "object.metadata.namespace == 'test-ns'"
            }
        ]
    }
}`),
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
