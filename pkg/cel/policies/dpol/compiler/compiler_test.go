package compiler

import (
	"testing"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCompile(t *testing.T) {
	tests := []struct {
		name       string
		policy     *v1alpha1.DeletingPolicy
		exceptions []*v1alpha1.PolicyException
		wantErr    bool
	}{
		{
			name: "empty policy",
			policy: &v1alpha1.DeletingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "empty"},
				Spec: v1alpha1.DeletingPolicySpec{
					Schedule: "* * * * *",
				},
			},
			wantErr: false,
		},
		{
			name: "valid condition",
			policy: &v1alpha1.DeletingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-condition"},
				Spec: v1alpha1.DeletingPolicySpec{
					Schedule: "* * * * *",
					Conditions: []admissionv1.MatchCondition{
						{
							Name:       "cond1",
							Expression: "object.metadata.name == 'foo'",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid condition",
			policy: &v1alpha1.DeletingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-condition"},
				Spec: v1alpha1.DeletingPolicySpec{
					Schedule: "* * * * *",
					Conditions: []admissionv1.MatchCondition{
						{
							Name:       "bad-cond",
							Expression: "object.metadata.name == ",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid variable",
			policy: &v1alpha1.DeletingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-variable"},
				Spec: v1alpha1.DeletingPolicySpec{
					Schedule: "* * * * *",
					Variables: []admissionv1.Variable{
						{
							Name:       "foo",
							Expression: "'bar'",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid variable",
			policy: &v1alpha1.DeletingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-variable"},
				Spec: v1alpha1.DeletingPolicySpec{
					Schedule: "* * * * *",
					Variables: []admissionv1.Variable{
						{
							Name:       "foo",
							Expression: "???",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid exception",
			policy: &v1alpha1.DeletingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-exception"},
				Spec: v1alpha1.DeletingPolicySpec{
					Schedule: "* * * * *",
				},
			},
			exceptions: []*v1alpha1.PolicyException{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "exc1"},
					Spec: v1alpha1.PolicyExceptionSpec{
						MatchConditions: []admissionv1.MatchCondition{
							{
								Name:       "exc-cond",
								Expression: "object.metadata.namespace == 'default'",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid exception",
			policy: &v1alpha1.DeletingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-exception"},
				Spec: v1alpha1.DeletingPolicySpec{
					Schedule: "* * * * *",
				},
			},
			exceptions: []*v1alpha1.PolicyException{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "exc-bad"},
					Spec: v1alpha1.PolicyExceptionSpec{
						MatchConditions: []admissionv1.MatchCondition{
							{
								Name:       "exc-bad-cond",
								Expression: "object.metadata.namespace =",
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCompiler()
			p, errs := c.Compile(tt.policy, tt.exceptions)
			if tt.wantErr {
				assert.Nil(t, p)
				assert.NotEmpty(t, errs)
			} else {
				assert.NotNil(t, p)
				assert.Empty(t, errs)
			}
		})
	}
}
