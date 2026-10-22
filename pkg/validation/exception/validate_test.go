package exception

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/logging"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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

func Test_ValidateCompensatingControls(t *testing.T) {
	validations := []admissionregistrationv1.Validation{{Expression: "true"}}
	testCases := []struct {
		name string
		spec policiesv1beta1.PolicyExceptionSpec
		want []string
	}{{
		name: "no validations means no warning",
		spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "mpol-1", Kind: "MutatingPolicy"}},
		},
		want: nil,
	}, {
		name: "validations on a ValidatingPolicy are supported",
		spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs:  []policiesv1beta1.PolicyRef{{Name: "vpol-1", Kind: "ValidatingPolicy"}},
			Validations: validations,
		},
		want: nil,
	}, {
		name: "validations on a NamespacedValidatingPolicy are supported",
		spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs:  []policiesv1beta1.PolicyRef{{Name: "vpol-1", Kind: "NamespacedValidatingPolicy"}},
			Validations: validations,
		},
		want: nil,
	}, {
		name: "validations on a MutatingPolicy warn",
		spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs:  []policiesv1beta1.PolicyRef{{Name: "mpol-1", Kind: "MutatingPolicy"}},
			Validations: validations,
		},
		want: []string{"spec.validations is only evaluated for ValidatingPolicy and NamespacedValidatingPolicy; it will be ignored for the referenced MutatingPolicy."},
	}, {
		name: "unsupported kinds are deduplicated and the supported one is omitted",
		spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{
				{Name: "vpol-1", Kind: "ValidatingPolicy"},
				{Name: "gpol-1", Kind: "GeneratingPolicy"},
				{Name: "gpol-2", Kind: "GeneratingPolicy"},
				{Name: "dpol-1", Kind: "DeletingPolicy"},
			},
			Validations: validations,
		},
		want: []string{"spec.validations is only evaluated for ValidatingPolicy and NamespacedValidatingPolicy; it will be ignored for the referenced GeneratingPolicy, DeletingPolicy."},
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateCompensatingControls(&tc.spec)
			assert.DeepEqual(t, tc.want, got)
		})
	}
}

func Test_ValidateCompensatingControlExpressions(t *testing.T) {
	vpolRef := []policiesv1beta1.PolicyRef{{Name: "vpol-1", Kind: "ValidatingPolicy"}}
	testCases := []struct {
		name      string
		spec      policiesv1beta1.PolicyExceptionSpec
		wantErr   bool
		wantField string
	}{{
		name: "no validations",
		spec: policiesv1beta1.PolicyExceptionSpec{PolicyRefs: vpolRef},
	}, {
		name: "valid expression",
		spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs:  vpolRef,
			Validations: []admissionregistrationv1.Validation{{Expression: "has(object.metadata.name)"}},
		},
	}, {
		name: "syntax error is rejected before the exception can break its policy",
		spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs:  vpolRef,
			Validations: []admissionregistrationv1.Validation{{Expression: "object.metadata.name =="}},
		},
		wantErr:   true,
		wantField: "spec.validations[0].expression",
	}, {
		name: "non-boolean expression is rejected",
		spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs:  vpolRef,
			Validations: []admissionregistrationv1.Validation{{Expression: "object.metadata.name"}},
		},
		wantErr:   true,
		wantField: "spec.validations[0].expression",
	}, {
		name: "bad messageExpression is rejected",
		spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: vpolRef,
			Validations: []admissionregistrationv1.Validation{{
				Expression:        "true",
				MessageExpression: "1 + 1",
			}},
		},
		wantErr:   true,
		wantField: "spec.validations[0].messageExpression",
	}, {
		name: "policy-scoped identifiers are out of scope for an exception",
		spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs:  vpolRef,
			Validations: []admissionregistrationv1.Validation{{Expression: "variables.env == 'prod'"}},
		},
		wantErr:   true,
		wantField: "spec.validations[0].expression",
	}, {
		// the ValidatingPolicy environment says nothing about what a MutatingPolicy accepts, and
		// the field is ignored there anyway, so it is only warned about, never rejected
		name: "kinds that ignore the field are not compiled",
		spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs:  []policiesv1beta1.PolicyRef{{Name: "mpol-1", Kind: "MutatingPolicy"}},
			Validations: []admissionregistrationv1.Validation{{Expression: "object.metadata.name =="}},
		},
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateCompensatingControlExpressions(&policiesv1beta1.PolicyException{Spec: tc.spec})
			if !tc.wantErr {
				assert.Equal(t, 0, len(errs))
				return
			}
			assert.Equal(t, 1, len(errs))
			assert.Equal(t, tc.wantField, errs[0].Field)
		})
	}
}
