package compiler

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// the exception env deliberately omits `variables` and `exceptions`; both must stay declared for
// the policy's own expressions, which share the builder that narrows them away.
func TestCompile_PolicyScopedIdentifiersStayDeclared(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{{
		name:       "exceptions",
		expression: "string(object.spec.image) in exceptions.allowedImages",
	}, {
		name:       "exceptions allowedValues",
		expression: "'CHOWN' in exceptions.allowedValues",
	}, {
		name:       "variables",
		expression: "'CHOWN' in variables.allowedCapabilities",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: policiesv1beta1.ValidatingPolicySpec{
					Variables: []admissionregistrationv1.Variable{
						{Name: "allowedCapabilities", Expression: "['CHOWN']"},
					},
					Validations: []admissionregistrationv1.Validation{{Expression: tt.expression}},
				},
			}
			_, errs := NewCompiler().Compile(policy, nil)
			assert.Nil(t, errs, "expected %q to compile, got %v", tt.expression, errs)
		})
	}
}

func TestCompile(t *testing.T) {
	tests := []struct {
		name       string
		policy     policiesv1beta1.ValidatingPolicyLike
		exceptions []*policiesv1beta1.PolicyException
		wantErr    bool
	}{
		{
			name: "empty policy",
			policy: &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "empty"},
				Spec:       policiesv1beta1.ValidatingPolicySpec{},
			},
			wantErr: false,
		},
		{
			name: "valid match condition",
			policy: &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-match-condition"},
				Spec: policiesv1beta1.ValidatingPolicySpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{
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
			policy: &policiesv1beta1.NamespacedValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "namespaced", Namespace: "default"},
				Spec:       policiesv1beta1.ValidatingPolicySpec{},
			},
			wantErr: false,
		},
		{
			name: "invalid match condition",
			policy: &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-match-condition"},
				Spec: policiesv1beta1.ValidatingPolicySpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{
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
			policy: &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-variable"},
				Spec: policiesv1beta1.ValidatingPolicySpec{
					Variables: []admissionregistrationv1.Variable{
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
			policy: &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-variable"},
				Spec: policiesv1beta1.ValidatingPolicySpec{
					Variables: []admissionregistrationv1.Variable{
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
			policy: &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-validation"},
				Spec: policiesv1beta1.ValidatingPolicySpec{
					Validations: []admissionregistrationv1.Validation{
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
			policy: &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-validation"},
				Spec: policiesv1beta1.ValidatingPolicySpec{
					Validations: []admissionregistrationv1.Validation{
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
			policy: &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-exception"},
				Spec:       policiesv1beta1.ValidatingPolicySpec{},
			},
			exceptions: []*policiesv1beta1.PolicyException{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "exc1"},
					Spec: policiesv1beta1.PolicyExceptionSpec{
						MatchConditions: []admissionregistrationv1.MatchCondition{
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
			policy: &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "validation-refs-var"},
				Spec: policiesv1beta1.ValidatingPolicySpec{
					Variables: []admissionregistrationv1.Variable{
						{
							Name:       "minReplicas",
							Expression: "1",
						},
					},
					Validations: []admissionregistrationv1.Validation{
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
			policy: &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-exception"},
				Spec:       policiesv1beta1.ValidatingPolicySpec{},
			},
			exceptions: []*policiesv1beta1.PolicyException{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "exc-bad"},
					Spec: policiesv1beta1.PolicyExceptionSpec{
						MatchConditions: []admissionregistrationv1.MatchCondition{
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
