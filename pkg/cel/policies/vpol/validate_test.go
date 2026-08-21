package vpol

import (
	"testing"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/autogen"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildValidatingPolicy(annotations map[string]string, validations []admissionregistrationv1.Validation) *v1beta1.ValidatingPolicy {
	return &v1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-policy",
			Annotations: annotations,
		},
		Spec: v1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			Validations: validations,
		},
	}
}

func TestValidate_Identifiers(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		validations []admissionregistrationv1.Validation
		wantErr     bool
	}{
		{
			name:        "no identifiers annotation",
			annotations: nil,
			validations: []admissionregistrationv1.Validation{
				{Expression: "object.spec.privileged == false"},
			},
			wantErr: false,
		},
		{
			name: "valid unique identifier",
			annotations: map[string]string{
				autogen.IdentifiersAnnotation: `{"object.spec.privileged == false":"check-privileged"}`,
			},
			validations: []admissionregistrationv1.Validation{
				{Expression: "object.spec.privileged == false"},
			},
			wantErr: false,
		},
		{
			name: "malformed annotation json",
			annotations: map[string]string{
				autogen.IdentifiersAnnotation: `{not valid json`,
			},
			validations: []admissionregistrationv1.Validation{
				{Expression: "object.spec.privileged == false"},
			},
			wantErr: true,
		},
		{
			name: "annotation key doesn't match any validation expression",
			annotations: map[string]string{
				autogen.IdentifiersAnnotation: `{"object.spec.nonexistent == true":"check-nonexistent"}`,
			},
			validations: []admissionregistrationv1.Validation{
				{Expression: "object.spec.privileged == false"},
			},
			wantErr: true,
		},
		{
			name: "duplicate identifiers assigned to different validations",
			annotations: map[string]string{
				autogen.IdentifiersAnnotation: `{
					"object.spec.privileged == false": "check-shared",
					"object.spec.hostNetwork == false": "check-shared"
				}`,
			},
			validations: []admissionregistrationv1.Validation{
				{Expression: "object.spec.privileged == false"},
				{Expression: "object.spec.hostNetwork == false"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pol := buildValidatingPolicy(tt.annotations, tt.validations)
			_, err := Validate(pol)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
