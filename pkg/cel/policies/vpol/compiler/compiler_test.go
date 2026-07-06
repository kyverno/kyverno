package compiler

import (
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
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
			name: "namespaced policy",
			policy: &v1beta1.NamespacedValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "namespaced", Namespace: "test"},
				Spec:       v1beta1.ValidatingPolicySpec{},
			},
			wantErr: false,
		},
		{
			name: "valid matchCondition",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-match"},
				Spec: v1beta1.ValidatingPolicySpec{
					MatchConditions: []admissionv1.MatchCondition{
						{
							Name:       "ns-is-default",
							Expression: `object.metadata.namespace == 'default'`,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid matchCondition",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-match"},
				Spec: v1beta1.ValidatingPolicySpec{
					MatchConditions: []admissionv1.MatchCondition{
						{
							Name:       "bad-match",
							Expression: "object.metadata.namespace ==",
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
			name: "valid validation",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-validation"},
				Spec: v1beta1.ValidatingPolicySpec{
					Validations: []admissionv1.Validation{
						{
							Expression: `object.metadata.name != ''`,
							Message:    "name must be set",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid validation",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-validation"},
				Spec: v1beta1.ValidatingPolicySpec{
					Validations: []admissionv1.Validation{
						{
							Expression: "object.metadata.name ==",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validation with messageExpression",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-message-expression"},
				Spec: v1beta1.ValidatingPolicySpec{
					Validations: []admissionv1.Validation{
						{
							Expression:        `object.metadata.name != ''`,
							Message:           "static message",
							MessageExpression: `"'dynamic: ' + object.metadata.name"`,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "validation with invalid messageExpression",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-message-expression"},
				Spec: v1beta1.ValidatingPolicySpec{
					Validations: []admissionv1.Validation{
						{
							Expression:        `object.metadata.name != ''`,
							MessageExpression: "not valid cel !!!",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid audit annotation",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-audit"},
				Spec: v1beta1.ValidatingPolicySpec{
					AuditAnnotations: []admissionv1.AuditAnnotation{
						{
							Key:             "owner",
							ValueExpression: `"team-a"`,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid audit annotation",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-audit"},
				Spec: v1beta1.ValidatingPolicySpec{
					AuditAnnotations: []admissionv1.AuditAnnotation{
						{
							Key:             "owner",
							ValueExpression: "object.metadata.labels[",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid policy exception",
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
								Expression: `object.metadata.namespace == 'default'`,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid policy exception",
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
		{
			name: "validation references variable",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "validation-refs-var"},
				Spec: v1beta1.ValidatingPolicySpec{
					Variables: []admissionv1.Variable{
						{
							Name:       "minNameLen",
							Expression: "1",
						},
					},
					Validations: []admissionv1.Validation{
						{
							Expression: `size(object.metadata.name) >= variables.minNameLen`,
							Message:    "name too short",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "json evaluation mode valid validation",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "json-valid"},
				Spec: v1beta1.ValidatingPolicySpec{
					EvaluationConfiguration: &v1beta1.EvaluationConfiguration{
						Mode: policieskyvernoio.EvaluationModeJSON,
					},
					Validations: []admissionv1.Validation{
						{
							Expression: `object.name != ''`,
							Message:    "name required",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "json evaluation mode invalid validation",
			policy: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "json-invalid"},
				Spec: v1beta1.ValidatingPolicySpec{
					EvaluationConfiguration: &v1beta1.EvaluationConfiguration{
						Mode: policieskyvernoio.EvaluationModeJSON,
					},
					Validations: []admissionv1.Validation{
						{
							Expression: "object.name ==",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	compiler := NewCompiler()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, errs := compiler.Compile(tt.policy, tt.exceptions)
			if tt.wantErr {
				assert.NotEmpty(t, errs, "expected compile errors")
			} else {
				assert.Empty(t, errs, "expected no compile errors, got: %v", errs)
				assert.NotNil(t, compiled, "expected compiled policy")
			}
		})
	}
}
