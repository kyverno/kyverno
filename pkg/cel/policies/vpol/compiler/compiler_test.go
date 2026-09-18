package compiler

import (
	"testing"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCompile(t *testing.T) {
	tests := []struct {
		name       string
		policy     v1beta1.ValidatingPolicyLike
		exceptions []*v1beta1.PolicyException
		wantErr    bool
	}{
		{
			name: "empty policy",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "empty"},
				Spec:       v1beta1.ValidatingPolicySpec{},
			},
			wantErr: false,
		},
		{
			name: "valid match condition",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-match-condition"},
				Spec: v1beta1.ValidatingPolicySpec{
					MatchConditions: []admissionv1.MatchCondition{
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
			name: "namespaced policy",
			policy: &v1beta1.NamespacedValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "namespaced"},
				Spec:       v1beta1.ValidatingPolicySpec{},
			},
			wantErr: false,
		},
		{
			name: "invalid match condition",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-match-condition"},
				Spec: v1beta1.ValidatingPolicySpec{
					MatchConditions: []admissionv1.MatchCondition{
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
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-variable"},
				Spec: v1beta1.ValidatingPolicySpec{
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
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-variable"},
				Spec: v1beta1.ValidatingPolicySpec{
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
			name: "valid validation expression",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-validation"},
				Spec: v1beta1.ValidatingPolicySpec{
					Validations: []admissionv1.Validation{
						{
							Expression: "object.spec.replicas > 0",
							Message:    "replicas must be positive",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid validation expression",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-validation"},
				Spec: v1beta1.ValidatingPolicySpec{
					Validations: []admissionv1.Validation{
						{
							Expression: "this is not CEL",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid exception",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-exception"},
				Spec:       v1beta1.ValidatingPolicySpec{},
			},
			exceptions: []*v1beta1.PolicyException{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "exc1"},
					Spec: v1beta1.PolicyExceptionSpec{
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
			name: "validation references variable",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "validation-refs-var"},
				Spec: v1beta1.ValidatingPolicySpec{
					Variables: []admissionv1.Variable{
						{
							Name:       "minReplicas",
							Expression: "1",
						},
					},
					Validations: []admissionv1.Validation{
						{
							Expression: "object.spec.replicas >= variables.minReplicas",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid exception",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-exception"},
				Spec:       v1beta1.ValidatingPolicySpec{},
			},
			exceptions: []*v1beta1.PolicyException{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "exc-bad"},
					Spec: v1beta1.PolicyExceptionSpec{
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
